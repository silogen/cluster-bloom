//go:build linux

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

var criticalSystemMounts = []string{
	"/",
	"/boot",
	"/boot/efi",
	"/usr",
	"/var",
	"/etc",
	"/home",
	"/opt",
	"/srv",
}

type blockDeviceID uint64

// resolveBlockDevice canonicalizes device aliases such as UUID= and
// /dev/disk/by-id paths, then verifies that the result is a block device.
func resolveBlockDevice(source string) (string, blockDeviceID, error) {
	source = strings.TrimSpace(source)
	if bracket := strings.IndexByte(source, '['); bracket >= 0 {
		source = source[:bracket]
	}
	if source == "" {
		return "", 0, fmt.Errorf("empty device source")
	}

	if !strings.HasPrefix(source, "/dev/") {
		if !strings.Contains(source, "=") {
			return "", 0, fmt.Errorf("%q is not a block-device source", source)
		}
		out, err := exec.Command("findfs", source).Output()
		if err != nil {
			return "", 0, fmt.Errorf("resolve %s with findfs: %w", source, err)
		}
		source = strings.TrimSpace(string(out))
	}

	canonical, err := filepath.EvalSymlinks(source)
	if err != nil {
		return "", 0, fmt.Errorf("resolve device path %s: %w", source, err)
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", 0, fmt.Errorf("stat device %s: %w", canonical, err)
	}
	if info.Mode()&os.ModeDevice == 0 || info.Mode()&os.ModeCharDevice != 0 {
		return "", 0, fmt.Errorf("%s is not a block device", canonical)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", 0, fmt.Errorf("read device identity for %s", canonical)
	}
	return canonical, blockDeviceID(stat.Rdev), nil
}

// blockDeviceDependencies returns a device and every lower-level block device
// it depends on. This covers partitions and common LVM, LUKS, and md-raid
// layouts without relying on device-name prefixes.
func blockDeviceDependencies(device string) ([]string, error) {
	out, err := exec.Command(
		"lsblk",
		"--inverse",
		"--list",
		"--paths",
		"--noheadings",
		"--output", "PATH",
		device,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("resolve block-device dependencies for %s: %w", device, err)
	}

	seen := map[string]struct{}{}
	var dependencies []string
	for _, field := range strings.Fields(string(out)) {
		if !strings.HasPrefix(field, "/dev/") {
			continue
		}
		canonical, _, err := resolveBlockDevice(field)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		dependencies = append(dependencies, canonical)
	}
	if len(dependencies) == 0 {
		return nil, fmt.Errorf("no block-device dependencies found for %s", device)
	}
	return dependencies, nil
}

// protectedSystemDevices identifies every block device backing a critical
// system mount or active swap, including its lower-level dependencies.
func protectedSystemDevices() (map[blockDeviceID]string, error) {
	protected := map[blockDeviceID]string{}
	rootFound := false

	addSource := func(source, reason string) error {
		canonical, _, err := resolveBlockDevice(source)
		if err != nil {
			return err
		}
		dependencies, err := blockDeviceDependencies(canonical)
		if err != nil {
			return err
		}
		for _, dependency := range dependencies {
			_, id, err := resolveBlockDevice(dependency)
			if err != nil {
				return err
			}
			protected[id] = reason
		}
		return nil
	}

	for _, mountPoint := range criticalSystemMounts {
		out, err := exec.Command(
			"findmnt",
			"--raw",
			"--noheadings",
			"--output", "SOURCES",
			"--target", mountPoint,
		).Output()
		if err != nil {
			if mountPoint == "/" {
				return nil, fmt.Errorf("determine root filesystem source: %w", err)
			}
			continue
		}
		sources := strings.FieldsFunc(strings.TrimSpace(string(out)), func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t' || r == '\n'
		})
		if len(sources) == 0 {
			if mountPoint == "/" {
				return nil, fmt.Errorf("root filesystem has no block-device sources")
			}
			continue
		}
		addedSource := false
		for _, source := range sources {
			if !strings.HasPrefix(source, "/dev/") {
				continue
			}
			if err := addSource(source, "system mount "+mountPoint); err != nil {
				return nil, err
			}
			addedSource = true
		}
		if mountPoint == "/" && addedSource {
			rootFound = true
		}
	}
	if !rootFound {
		return nil, fmt.Errorf("could not identify the block device backing /")
	}

	if data, err := os.ReadFile("/proc/swaps"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines[1:] {
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			if strings.HasPrefix(fields[0], "/dev/") {
				if err := addSource(fields[0], "active swap"); err != nil {
					return nil, err
				}
				continue
			}

			out, err := exec.Command(
				"findmnt",
				"--raw",
				"--noheadings",
				"--output", "SOURCES",
				"--target", fields[0],
			).Output()
			if err != nil {
				return nil, fmt.Errorf("resolve filesystem backing swapfile %s: %w", fields[0], err)
			}
			swapSources := strings.FieldsFunc(strings.TrimSpace(string(out)), func(r rune) bool {
				return r == ',' || r == ' ' || r == '\t' || r == '\n'
			})
			if len(swapSources) == 0 {
				return nil, fmt.Errorf("swapfile %s has no block-device source", fields[0])
			}
			for _, source := range swapSources {
				if !strings.HasPrefix(source, "/dev/") {
					return nil, fmt.Errorf("swapfile %s source %q is not a block device", fields[0], source)
				}
				if err := addSource(source, "active swapfile "+fields[0]); err != nil {
					return nil, err
				}
			}
		}
	}

	return protected, nil
}

func protectedConflictForDevice(target string) (canonical, reason string, err error) {
	canonical, id, err := resolveBlockDevice(target)
	if err != nil {
		return "", "", err
	}
	protected, err := protectedSystemDevices()
	if err != nil {
		return "", "", fmt.Errorf("build protected-device set: %w", err)
	}
	if conflictReason, exists := protected[id]; exists {
		return canonical, conflictReason, nil
	}
	return canonical, "", nil
}

// assertSafeToWipe fails closed if target is not a block device or if it is
// anywhere in the dependency chain of a critical system mount or active swap.
func assertSafeToWipe(target string) error {
	canonical, reason, err := protectedConflictForDevice(target)
	if err != nil {
		return err
	}
	if reason != "" {
		return fmt.Errorf("refusing to wipe %s (%s): it backs %s", target, canonical, reason)
	}
	return nil
}

// assertDeviceTreeUnmounted checks the target and all of its partitions and
// layered children. Checking only findmnt --source <whole-disk> misses mounted
// partitions such as /dev/sda1 when the candidate is /dev/sda.
func blockDeviceTreeMounts(target string) (map[string]string, error) {
	canonical, _, err := resolveBlockDevice(target)
	if err != nil {
		return nil, err
	}
	out, err := exec.Command(
		"lsblk",
		"--list",
		"--paths",
		"--noheadings",
		"--output", "PATH",
		canonical,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("inspect mounts below %s: %w", target, err)
	}
	mounts := map[string]string{}
	for _, device := range strings.Fields(string(out)) {
		mountOutput, err := exec.Command(
			"findmnt",
			"--source", device,
			"--noheadings",
			"--output", "TARGET",
		).Output()
		if err != nil {
			continue
		}
		if mountPoints := strings.Fields(string(mountOutput)); len(mountPoints) > 0 {
			mounts[device] = strings.Join(mountPoints, " ")
		}
	}
	return mounts, nil
}

func assertDeviceTreeUnmounted(target string) error {
	mounts, err := blockDeviceTreeMounts(target)
	if err != nil {
		return err
	}
	for device, mountPoints := range mounts {
		return fmt.Errorf("refusing to wipe %s: %s is still mounted at %s",
			target, device, mountPoints)
	}
	return nil
}
