//go:build linux

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
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

// mountTablePath is the kernel's authoritative list of mounts. Reading it
// directly replaces per-device findmnt calls, which signal "nothing matched"
// and a genuine failure through the same exit status: a device that could not
// be inspected must never be mistaken for an unmounted one.
var mountTablePath = "/proc/self/mountinfo"

type mountEntry struct {
	mountPoint string
	deviceID   blockDeviceID
	source     string
}

type mountTable []mountEntry

// unescapeMountField decodes the octal escapes that mountinfo uses for space,
// tab, newline, and backslash inside path fields.
func unescapeMountField(field string) string {
	if !strings.Contains(field, `\`) {
		return field
	}
	var builder strings.Builder
	for index := 0; index < len(field); index++ {
		if field[index] == '\\' && index+3 < len(field) {
			if value, err := strconv.ParseUint(field[index+1:index+4], 8, 8); err == nil {
				builder.WriteByte(byte(value))
				index += 3
				continue
			}
		}
		builder.WriteByte(field[index])
	}
	return builder.String()
}

func parseMountDeviceID(field string) (blockDeviceID, error) {
	majorText, minorText, separated := strings.Cut(field, ":")
	if !separated {
		return 0, fmt.Errorf("malformed device number %q", field)
	}
	major, err := strconv.ParseUint(majorText, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("malformed device major in %q", field)
	}
	minor, err := strconv.ParseUint(minorText, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("malformed device minor in %q", field)
	}
	return blockDeviceID(unix.Mkdev(uint32(major), uint32(minor))), nil
}

// readMountTable parses every mount the kernel currently reports. It returns
// either the complete table or an error, so no caller can act on a partial
// result that looks like "nothing is mounted".
func readMountTable() (mountTable, error) {
	data, err := os.ReadFile(mountTablePath)
	if err != nil {
		return nil, fmt.Errorf("read mount table %s: %w", mountTablePath, err)
	}
	var table mountTable
	for index, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		separator := -1
		for position, field := range fields {
			if field == "-" {
				separator = position
				break
			}
		}
		// Layout: id parent major:minor root mountPoint options [optional...] - fstype source superOptions
		if separator < 6 || len(fields) < separator+3 {
			return nil, fmt.Errorf("mount table %s line %d is malformed", mountTablePath, index+1)
		}
		deviceID, err := parseMountDeviceID(fields[2])
		if err != nil {
			return nil, fmt.Errorf("mount table %s line %d: %w", mountTablePath, index+1, err)
		}
		table = append(table, mountEntry{
			mountPoint: unescapeMountField(fields[4]),
			deviceID:   deviceID,
			source:     unescapeMountField(fields[separator+2]),
		})
	}
	return table, nil
}

// blockSourcesForMountPoint returns the block-device sources mounted exactly at
// mountPoint. An empty result means nothing is mounted there, which is normal:
// on most hosts paths such as /usr and /var live on / rather than on their own
// filesystem. Callers distinguish that from a failure because an unreadable
// mount table is reported by readMountTable instead.
func (table mountTable) blockSourcesForMountPoint(mountPoint string) []string {
	var sources []string
	for _, entry := range table {
		if entry.mountPoint != mountPoint {
			continue
		}
		if strings.HasPrefix(entry.source, "/dev/") {
			sources = append(sources, entry.source)
		}
	}
	return sources
}

// blockSourcesContainingPath returns the block-device sources of the mount that
// holds path, which for a swapfile is the filesystem it was allocated on.
func (table mountTable) blockSourcesContainingPath(path string) []string {
	holder := ""
	for _, entry := range table {
		underMount := entry.mountPoint == "/" ||
			path == entry.mountPoint ||
			strings.HasPrefix(path, strings.TrimSuffix(entry.mountPoint, "/")+"/")
		if underMount && len(entry.mountPoint) >= len(holder) {
			holder = entry.mountPoint
		}
	}
	if holder == "" {
		return nil
	}
	return table.blockSourcesForMountPoint(holder)
}

// mountPointsForDeviceIDs maps each mounted device in ids to the mount points
// it serves. Both the mountinfo device number and its source path are matched,
// because multi-device filesystems such as btrfs report an anonymous device
// number and identify the real block device only through the source.
func (table mountTable) mountPointsForDeviceIDs(ids map[blockDeviceID]string) map[string][]string {
	mounts := map[string][]string{}
	for _, entry := range table {
		device, matched := ids[entry.deviceID]
		if !matched && strings.HasPrefix(entry.source, "/dev/") {
			// A /dev path that no longer resolves cannot be one of ids, whose
			// members all resolved successfully, so skipping it is safe.
			if _, sourceID, err := resolveBlockDevice(entry.source); err == nil {
				device, matched = ids[sourceID]
			}
		}
		if !matched {
			continue
		}
		mounts[device] = append(mounts[device], entry.mountPoint)
	}
	for device, mountPoints := range mounts {
		sort.Strings(mountPoints)
		mounts[device] = mountPoints
	}
	return mounts
}

// protectedSystemDevices identifies every block device backing a critical
// system mount or active swap, including its lower-level dependencies.
func protectedSystemDevices() (map[blockDeviceID]string, error) {
	table, err := readMountTable()
	if err != nil {
		return nil, err
	}
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
		// An empty result here means the path is not a separate filesystem on
		// this host, so it is already covered by whichever device backs its
		// parent. It never means the lookup failed: readMountTable already
		// returned the whole table or an error.
		sources := table.blockSourcesForMountPoint(mountPoint)
		if len(sources) == 0 {
			if mountPoint == "/" {
				return nil, fmt.Errorf("mount table reports no block device backing /")
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

	// A kernel built without swap support has no /proc/swaps. Any other read
	// failure would leave active swap devices unprotected, so it fails closed.
	data, err := os.ReadFile("/proc/swaps")
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read active swap devices: %w", err)
	}
	if err == nil {
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

			swapSources := table.blockSourcesContainingPath(fields[0])
			if len(swapSources) == 0 {
				return nil, fmt.Errorf("swapfile %s has no block-device source", fields[0])
			}
			for _, source := range swapSources {
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

// blockDeviceTreeMounts reports every mount served by target or by any of its
// partitions and layered children. Inspecting only the whole disk would miss
// mounted partitions such as /dev/sda1 when the candidate is /dev/sda.
func blockDeviceTreeMounts(target string) (map[string][]string, error) {
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
	treeIDs := map[blockDeviceID]string{}
	for _, device := range strings.Fields(string(out)) {
		if !strings.HasPrefix(device, "/dev/") {
			continue
		}
		canonicalDevice, id, err := resolveBlockDevice(device)
		if err != nil {
			return nil, fmt.Errorf("inspect mounts below %s: %w", target, err)
		}
		treeIDs[id] = canonicalDevice
	}
	table, err := readMountTable()
	if err != nil {
		return nil, err
	}
	return table.mountPointsForDeviceIDs(treeIDs), nil
}

func assertDeviceTreeUnmounted(target string) error {
	mounts, err := blockDeviceTreeMounts(target)
	if err != nil {
		return err
	}
	return mountedDeviceTreeError(target, mounts)
}

func mountedDeviceTreeError(target string, mounts map[string][]string) error {
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
		details = append(details, fmt.Sprintf("%s at %s", device, strings.Join(mounts[device], " ")))
	}
	return fmt.Errorf("refusing to wipe %s: mounted device tree entries: %s",
		target, strings.Join(details, "; "))
}
