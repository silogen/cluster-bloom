//go:build linux

package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	fstabManagedTag    = "# managed by cluster-bloom"
	fstabPremountedTag = "# premounted by cluster-bloom"
	fstabRancherTag    = "# managed by cluster-bloom rancher-disk"
	maxBloomDiskIndex  = 4095
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

func readBloomFstab() (string, []bloomFstabEntry, error) {
	data, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return "", nil, fmt.Errorf("read /etc/fstab: %w", err)
	}
	entries, parseErrors := parseBloomFstab(string(data))
	if len(parseErrors) > 0 {
		var messages []string
		for _, parseErr := range parseErrors {
			messages = append(messages, parseErr.Error())
		}
		return "", nil, fmt.Errorf("invalid Bloom fstab entries: %s", strings.Join(messages, "; "))
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

func backupAndWriteFstab(original string, retainedLines []string) error {
	timestamp := time.Now().Format("20060102-150405.000000000")
	backupPath := fmt.Sprintf("/etc/fstab.bak-%s", timestamp)
	if err := os.WriteFile(backupPath, []byte(original), 0644); err != nil {
		return fmt.Errorf("back up fstab to %s: %w", backupPath, err)
	}
	if err := os.WriteFile("/etc/fstab", []byte(strings.Join(retainedLines, "\n")), 0644); err != nil {
		return fmt.Errorf("update /etc/fstab (backup: %s): %w", backupPath, err)
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
