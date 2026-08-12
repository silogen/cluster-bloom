//go:build linux

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

type deviceProtection struct {
	mountPoints map[string]struct{}
	swapReason  string
}

func (protection deviceProtection) formatReason() string {
	var parts []string
	if len(protection.mountPoints) > 0 {
		mountPoints := make([]string, 0, len(protection.mountPoints))
		for mountPoint := range protection.mountPoints {
			mountPoints = append(mountPoints, mountPoint)
		}
		sort.Strings(mountPoints)
		parts = append(parts, "system mounts "+strings.Join(mountPoints, ", "))
	}
	if protection.swapReason != "" {
		parts = append(parts, protection.swapReason)
	}
	return strings.Join(parts, "; ")
}

func blockSourcesForExactMountPoint(mountPoint string) ([]string, error) {
	runFindmnt := func(column string) ([]byte, error) {
		return exec.Command(
			"findmnt",
			"--raw",
			"--noheadings",
			"--output", column,
			"--mountpoint", mountPoint,
		).Output()
	}

	out, err := runFindmnt("SOURCES")
	if findmntReportsUnknownColumn(err, "SOURCES") {
		out, err = runFindmnt("SOURCE")
	}
	if err != nil {
		if detail := commandStderr(err); detail != "" {
			return nil, fmt.Errorf("%w: %s", err, detail)
		}
		return nil, err
	}
	sources := strings.FieldsFunc(strings.TrimSpace(string(out)), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	var blockSources []string
	for _, source := range sources {
		if strings.HasPrefix(source, "/dev/") {
			blockSources = append(blockSources, source)
		}
	}
	if len(blockSources) == 0 {
		return nil, fmt.Errorf("mount point %s has no block-device sources", mountPoint)
	}
	return blockSources, nil
}

func findmntReportsUnknownColumn(err error, column string) bool {
	detail := strings.ToLower(commandStderr(err))
	return strings.Contains(detail, "unknown column") &&
		strings.Contains(detail, strings.ToLower(column))
}

func commandStderr(err error) string {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return ""
	}
	return strings.TrimSpace(string(exitErr.Stderr))
}

// protectedSystemDevices identifies every block device backing a critical
// system mount or active swap, including its lower-level dependencies.
func protectedSystemDevices() (map[blockDeviceID]string, error) {
	protected := map[blockDeviceID]deviceProtection{}
	rootFound := false

	addMountSource := func(source, mountPoint string) error {
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
			entry := protected[id]
			if entry.mountPoints == nil {
				entry.mountPoints = map[string]struct{}{}
			}
			entry.mountPoints[mountPoint] = struct{}{}
			protected[id] = entry
		}
		return nil
	}
	addSwapSource := func(source, swapReason string) error {
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
			entry := protected[id]
			if entry.swapReason == "" {
				entry.swapReason = swapReason
			}
			protected[id] = entry
		}
		return nil
	}

	for _, mountPoint := range criticalSystemMounts {
		sources, err := blockSourcesForExactMountPoint(mountPoint)
		if err != nil {
			if mountPoint == "/" {
				return nil, fmt.Errorf("determine root filesystem source: %w", err)
			}
			continue
		}
		for _, source := range sources {
			if err := addMountSource(source, mountPoint); err != nil {
				return nil, err
			}
		}
		if mountPoint == "/" {
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
				if err := addSwapSource(fields[0], "active swap"); err != nil {
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
				if err := addSwapSource(source, "active swapfile "+fields[0]); err != nil {
					return nil, err
				}
			}
		}
	}

	reasons := make(map[blockDeviceID]string, len(protected))
	for id, protection := range protected {
		reasons[id] = protection.formatReason()
	}
	return reasons, nil
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
	return mountedDeviceTreeError(target, mounts)
}

func mountedDeviceTreeError(target string, mounts map[string]string) error {
	if len(mounts) == 0 {
		return nil
	}

	devices := make([]string, 0, len(mounts))
	for device := range mounts {
		devices = append(devices, device)
	}
	sort.Strings(devices)
	details := make([]string, 0, len(devices))
	for _, device := range devices {
		details = append(details, fmt.Sprintf("%s at %s", device, mounts[device]))
	}
	return fmt.Errorf("refusing to wipe %s: mounted device tree entries: %s",
		target, strings.Join(details, "; "))
}
