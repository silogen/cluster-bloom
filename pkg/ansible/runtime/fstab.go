//go:build linux

package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/silogen/cluster-bloom/pkg/fileutil"
)

const (
	fstabManagedTag    = "# managed by cluster-bloom"
	fstabPremountedTag = "# premounted by cluster-bloom"
	fstabRancherTag    = "# managed by cluster-bloom rancher-disk"
	maxBloomDiskIndex  = 4095

	fstabSectionHeader = "# # # this section is managed by AMD Enterprise AI tool cluster-bloom"
	fstabSectionFooter = "# # # end of AMD Enterprise AI cluster-bloom"

	fstabBackupDirectory    = "/var/backups/cluster-bloom/fstab"
	maxRetainedFstabBackups = 5
	fstabBackupFilePrefix   = "fstab-"
)

type bloomFstabTag int

const (
	bloomFstabNone bloomFstabTag = iota
	bloomFstabManaged
	bloomFstabPremounted
	bloomFstabRancher
)

type bloomFstabEntry struct {
	source     string
	mountPoint string
	tag        bloomFstabTag
	raw        string
}

type activeFstabMount struct {
	source string
	raw    string
	tag    bloomFstabTag
}

func exactBloomFstabTag(line string) bloomFstabTag {
	commentIndex := strings.IndexByte(line, '#')
	if commentIndex < 0 {
		return bloomFstabNone
	}
	comment := "# " + strings.TrimSpace(line[commentIndex+1:])
	switch comment {
	case fstabManagedTag:
		return bloomFstabManaged
	case fstabPremountedTag:
		return bloomFstabPremounted
	case fstabRancherTag:
		return bloomFstabRancher
	default:
		return bloomFstabNone
	}
}

func parseBloomDiskMountPoint(mountPoint string) (int, bool) {
	const prefix = "/mnt/disk"
	if !strings.HasPrefix(mountPoint, prefix) {
		return 0, false
	}
	suffix := strings.TrimPrefix(mountPoint, prefix)
	if suffix == "" {
		return 0, false
	}
	for _, r := range suffix {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	index, err := strconv.Atoi(suffix)
	if err != nil || index < 0 || index > maxBloomDiskIndex {
		return 0, false
	}
	return index, true
}

func isSafePremountedMountPoint(mountPoint string) bool {
	if !filepath.IsAbs(mountPoint) || filepath.Clean(mountPoint) != mountPoint || mountPoint == "/" {
		return false
	}
	for _, protectedMount := range criticalSystemMounts {
		if mountPoint == protectedMount {
			return false
		}
	}
	for _, protectedPrefix := range []string{
		"/boot/", "/dev/", "/etc/", "/proc/", "/run/", "/sys/", "/usr/",
	} {
		if strings.HasPrefix(mountPoint, protectedPrefix) {
			return false
		}
	}
	for _, r := range mountPoint {
		isAlphaNumeric := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
		if !isAlphaNumeric && r != '/' && r != '.' && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

func parseBloomFstab(content string) ([]bloomFstabEntry, []error) {
	var entries []bloomFstabEntry
	var parseErrors []error
	seenMountPoints := map[string]int{}

	for lineNumber, line := range strings.Split(content, "\n") {
		tag := exactBloomFstabTag(line)
		if tag == bloomFstabNone {
			continue
		}
		beforeComment := line
		if commentIndex := strings.IndexByte(line, '#'); commentIndex >= 0 {
			beforeComment = line[:commentIndex]
		}
		fields := strings.Fields(beforeComment)
		if len(fields) != 6 {
			parseErrors = append(parseErrors, fmt.Errorf(
				"fstab line %d has a Bloom tag but does not contain exactly six fstab fields", lineNumber+1))
			continue
		}
		if fields[2] == "" || fields[3] == "" {
			parseErrors = append(parseErrors, fmt.Errorf(
				"fstab line %d has an empty filesystem type or mount options field", lineNumber+1))
			continue
		}
		if _, err := strconv.Atoi(fields[4]); err != nil {
			parseErrors = append(parseErrors, fmt.Errorf(
				"fstab line %d has invalid dump field %q", lineNumber+1, fields[4]))
			continue
		}
		if _, err := strconv.Atoi(fields[5]); err != nil {
			parseErrors = append(parseErrors, fmt.Errorf(
				"fstab line %d has invalid fsck pass field %q", lineNumber+1, fields[5]))
			continue
		}
		entry := bloomFstabEntry{
			source:     fields[0],
			mountPoint: fields[1],
			tag:        tag,
			raw:        line,
		}

		switch tag {
		case bloomFstabManaged:
			if entry.mountPoint == "/var/lib/rancher" {
				parseErrors = append(parseErrors, fmt.Errorf(
					"fstab line %d: %q is not valid on /var/lib/rancher",
					lineNumber+1, fstabManagedTag))
				continue
			}
			if _, ok := parseBloomDiskMountPoint(entry.mountPoint); !ok {
				parseErrors = append(parseErrors, fmt.Errorf(
					"fstab line %d: Bloom disk tag is not allowed on mount point %q",
					lineNumber+1, entry.mountPoint))
				continue
			}
		case bloomFstabPremounted:
			if !isSafePremountedMountPoint(entry.mountPoint) {
				parseErrors = append(parseErrors, fmt.Errorf(
					"fstab line %d: invalid premounted Bloom path %q",
					lineNumber+1, entry.mountPoint))
				continue
			}
		case bloomFstabRancher:
			if entry.mountPoint != "/var/lib/rancher" {
				parseErrors = append(parseErrors, fmt.Errorf(
					"fstab line %d: Bloom rancher-disk tag is not allowed on mount point %q",
					lineNumber+1, entry.mountPoint))
				continue
			}
		}
		if previousLine, duplicate := seenMountPoints[entry.mountPoint]; duplicate {
			parseErrors = append(parseErrors, fmt.Errorf(
				"fstab line %d duplicates Bloom-managed mount point %q from line %d",
				lineNumber+1, entry.mountPoint, previousLine))
			continue
		}
		seenMountPoints[entry.mountPoint] = lineNumber + 1
		entries = append(entries, entry)
	}

	return entries, parseErrors
}

func bloomTaggedMountPoint(line string) (string, bool) {
	if exactBloomFstabTag(line) == bloomFstabNone {
		return "", false
	}
	beforeComment := line
	if commentIndex := strings.IndexByte(line, '#'); commentIndex >= 0 {
		beforeComment = line[:commentIndex]
	}
	fields := strings.Fields(beforeComment)
	if len(fields) < 2 {
		return "", false
	}
	return fields[1], true
}

func bloomFstabParseRemediation(content string) []string {
	var hints []string
	for lineNumber, line := range strings.Split(content, "\n") {
		tag := exactBloomFstabTag(line)
		if tag == bloomFstabNone {
			continue
		}
		mountPoint, ok := bloomTaggedMountPoint(line)
		if !ok {
			continue
		}
		switch {
		case tag == bloomFstabManaged && mountPoint == "/var/lib/rancher":
			hints = append(hints, fmt.Sprintf(
				"Line %d: /var/lib/rancher must use %q, not %q",
				lineNumber+1, fstabRancherTag, fstabManagedTag))
		case tag == bloomFstabRancher && mountPoint != "/var/lib/rancher":
			hints = append(hints, fmt.Sprintf(
				"Line %d: %q is only valid on /var/lib/rancher; use %q on /mnt/diskN cluster mounts",
				lineNumber+1, fstabRancherTag, fstabManagedTag))
		case tag == bloomFstabManaged:
			if _, ok := parseBloomDiskMountPoint(mountPoint); !ok {
				hints = append(hints, fmt.Sprintf(
					"Line %d: %q is only valid on /mnt/diskN mounts such as /mnt/disk0",
					lineNumber+1, fstabManagedTag))
			}
		}
	}
	if len(hints) > 0 {
		return hints
	}
	return []string{
		fmt.Sprintf("Cluster disks: /mnt/diskN with %q", fstabManagedTag),
		fmt.Sprintf("Rancher disk: /var/lib/rancher with %q", fstabRancherTag),
		fmt.Sprintf("Premounted disks: safe absolute path with %q", fstabPremountedTag),
	}
}

// staleFstabEntryError explains a Bloom-managed /etc/fstab line whose source
// (UUID=, LABEL=, or device path) no longer resolves to any device on this
// host. This most commonly happens when a previous `bloom cleanup` run was
// interrupted after a device was wiped/reformatted (changing its identity)
// but before the now-stale fstab line was removed.
func staleFstabEntryError(entry bloomFstabEntry, resolveErr error) error {
	tagLabel := fstabManagedTag
	switch entry.tag {
	case bloomFstabRancher:
		tagLabel = fstabRancherTag
	case bloomFstabPremounted:
		tagLabel = fstabPremountedTag
	}
	return cleanupPreflightError(
		fmt.Sprintf(
			"/etc/fstab has a Bloom-managed entry for %s (tagged %q) that references %s, "+
				"but no device on this system currently has that identity (%v)",
			entry.mountPoint, tagLabel, entry.source, resolveErr,
		),
		[]string{
			"This usually means a previous 'bloom cleanup' run was interrupted after the device was " +
				"wiped/reformatted (which changes its UUID) but before the now-stale /etc/fstab line was removed",
			fmt.Sprintf("Line to remove if the device was already wiped and is no longer needed: %s", entry.raw),
			"Run 'lsblk -f' or 'blkid' to see which devices currently exist and their UUIDs",
			"If the device still exists under a new identity, update /etc/fstab (or point the matching " +
				"config field at it) to match, then re-run 'bloom cleanup'",
		},
	)
}

func invalidBloomFstabError(content string, parseErrors []error) error {
	var messages []string
	for _, parseErr := range parseErrors {
		messages = append(messages, parseErr.Error())
	}
	return cleanupPreflightError(
		"invalid Bloom fstab entries: "+strings.Join(messages, "; "),
		bloomFstabParseRemediation(content),
	)
}

func readBloomFstab() (string, []bloomFstabEntry, error) {
	data, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return "", nil, fmt.Errorf("read /etc/fstab: %w", err)
	}
	entries, parseErrors := parseBloomFstab(string(data))
	if len(parseErrors) > 0 {
		return "", nil, invalidBloomFstabError(string(data), parseErrors)
	}
	return string(data), entries, nil
}

func findActiveFstabMount(content, mountPoint string) (activeFstabMount, bool, error) {
	var found activeFstabMount
	for lineNumber, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		beforeComment := line
		if commentIndex := strings.IndexByte(line, '#'); commentIndex >= 0 {
			beforeComment = line[:commentIndex]
		}
		fields := strings.Fields(beforeComment)
		if len(fields) < 2 || fields[1] != mountPoint {
			continue
		}
		if found.raw != "" {
			return activeFstabMount{}, false, fmt.Errorf(
				"multiple active fstab entries target %s (including line %d)", mountPoint, lineNumber+1)
		}
		found = activeFstabMount{
			source: fields[0],
			raw:    line,
			tag:    exactBloomFstabTag(line),
		}
	}
	return found, found.raw != "", nil
}

func fstabBackupPath(timestamp string) string {
	return filepath.Join(fstabBackupDirectory, fstabBackupFilePrefix+timestamp)
}

func pruneFstabBackupsIn(directory string, maxRetained int) error {
	if maxRetained <= 0 {
		return nil
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("list fstab backups in %s: %w", directory, err)
	}
	var backups []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), fstabBackupFilePrefix) {
			continue
		}
		backups = append(backups, entry.Name())
	}
	if len(backups) <= maxRetained {
		return nil
	}
	sort.Strings(backups)
	for _, name := range backups[:len(backups)-maxRetained] {
		if err := os.Remove(filepath.Join(directory, name)); err != nil {
			return fmt.Errorf("remove old fstab backup %s: %w", name, err)
		}
	}
	return nil
}

func isBloomFstabSectionMarker(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == fstabSectionHeader || trimmed == fstabSectionFooter
}

func hasBloomTaggedFstabLine(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if exactBloomFstabTag(line) != bloomFstabNone {
			return true
		}
	}
	return false
}

// pruneEmptyBloomFstabSection removes the Ansible deployment section header and
// footer when no strictly tagged Bloom fstab entries remain. Premounted entries
// count as tagged and keep the section intact.
func pruneEmptyBloomFstabSection(lines []string) []string {
	if hasBloomTaggedFstabLine(strings.Join(lines, "\n")) {
		return lines
	}
	var pruned []string
	for _, line := range lines {
		if !isBloomFstabSectionMarker(line) {
			pruned = append(pruned, line)
		}
	}
	return pruned
}

// joinFstabLines joins the retained lines and makes sure the result ends with
// exactly one newline. A file that lost its final newline earlier would
// otherwise keep the defect, and the next appended entry joins the last line.
func joinFstabLines(lines []string) string {
	content := strings.Join(lines, "\n")
	if content == "" || strings.HasSuffix(content, "\n") {
		return content
	}
	return content + "\n"
}

func backupAndWriteFstab(original string, retainedLines []string) error {
	beforePrune := len(retainedLines)
	retainedLines = pruneEmptyBloomFstabSection(retainedLines)
	if len(retainedLines) < beforePrune {
		fmt.Println("      ✓ Removed empty Bloom fstab section markers")
	}

	timestamp := time.Now().Format("20060102-150405.000000000")
	if err := os.MkdirAll(fstabBackupDirectory, 0755); err != nil {
		return fmt.Errorf("create fstab backup directory %s: %w", fstabBackupDirectory, err)
	}
	backupPath := fstabBackupPath(timestamp)
	// The backup must reach disk before /etc/fstab is replaced. A crash between
	// the two would otherwise leave neither the original nor a recoverable copy.
	if err := fileutil.WriteAtomically(backupPath, []byte(original), 0644); err != nil {
		return fmt.Errorf("back up fstab to %s: %w", backupPath, err)
	}
	if err := fileutil.WriteAtomically("/etc/fstab", []byte(joinFstabLines(retainedLines)), 0644); err != nil {
		return fmt.Errorf("update /etc/fstab (backup: %s): %w", backupPath, err)
	}
	if err := pruneFstabBackupsIn(fstabBackupDirectory, maxRetainedFstabBackups); err != nil {
		return err
	}
	fmt.Printf("      ✓ Backed up fstab to %s\n", backupPath)
	return nil
}

func removeBloomFstabEntries(tag bloomFstabTag) error {
	original, entries, err := readBloomFstab()
	if err != nil {
		return err
	}
	removeLines := map[string]struct{}{}
	for _, entry := range entries {
		if entry.tag == tag {
			removeLines[entry.raw] = struct{}{}
		}
	}
	var retained []string
	for _, line := range strings.Split(original, "\n") {
		if _, remove := removeLines[line]; !remove {
			retained = append(retained, line)
		}
	}
	return backupAndWriteFstab(original, retained)
}

func removeExactFstabLine(rawLine string) error {
	original, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return fmt.Errorf("read /etc/fstab: %w", err)
	}
	removed := false
	var retained []string
	for _, line := range strings.Split(string(original), "\n") {
		if !removed && line == rawLine {
			removed = true
			continue
		}
		retained = append(retained, line)
	}
	if !removed {
		return fmt.Errorf("validated fstab entry changed before removal")
	}
	return backupAndWriteFstab(string(original), retained)
}
