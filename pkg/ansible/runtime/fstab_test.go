//go:build linux

package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFstabBackupPathUsesDedicatedDirectory(t *testing.T) {
	got := fstabBackupPath("20260810-103231.834580706")
	want := filepath.Join(fstabBackupDirectory, "fstab-20260810-103231.834580706")
	if got != want {
		t.Fatalf("fstabBackupPath() = %q, want %q", got, want)
	}
}

func TestPruneFstabBackupsRetainsLatestFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"fstab-20260810-100000.000000001",
		"fstab-20260810-100000.000000002",
		"fstab-20260810-100000.000000003",
		"other-file",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatalf("WriteFile(%q): %v", name, err)
		}
	}
	if err := pruneFstabBackupsIn(dir, 2); err != nil {
		t.Fatalf("pruneFstabBackupsIn() error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	var backups []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), fstabBackupFilePrefix) {
			backups = append(backups, entry.Name())
		}
	}
	if len(backups) != 2 {
		t.Fatalf("remaining fstab backups = %v, want 2 newest files", backups)
	}
	if backups[0] != "fstab-20260810-100000.000000002" || backups[1] != "fstab-20260810-100000.000000003" {
		t.Fatalf("remaining fstab backups = %v, want two newest timestamped files", backups)
	}
}

func TestStaleFstabEntryErrorExplainsInterruptedCleanup(t *testing.T) {
	entry := bloomFstabEntry{
		source:     "UUID=6d051afa-5188-4f47-aa58-e8ed490208b7",
		mountPoint: "/var/lib/rancher",
		tag:        bloomFstabRancher,
		raw:        "UUID=6d051afa-5188-4f47-aa58-e8ed490208b7 /var/lib/rancher ext4 defaults 0 2 # managed by cluster-bloom rancher-disk",
	}
	err := staleFstabEntryError(entry, errors.New("resolve UUID=6d051afa-5188-4f47-aa58-e8ed490208b7 with findfs: exit status 1"))

	got := err.Error()
	for _, want := range []string{
		"/var/lib/rancher",
		fstabRancherTag,
		"UUID=6d051afa-5188-4f47-aa58-e8ed490208b7",
		"interrupted",
		"Remediation:",
		entry.raw,
		"lsblk",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("staleFstabEntryError() = %q, want it to contain %q", got, want)
		}
	}
}

func TestPruneEmptyBloomFstabSectionRemovesMarkersWhenNoTags(t *testing.T) {
	input := []string{
		"/dev/sda1 / ext4 defaults 0 1",
		fstabSectionHeader,
		fstabSectionFooter,
	}
	got := pruneEmptyBloomFstabSection(input)
	if len(got) != 1 || got[0] != "/dev/sda1 / ext4 defaults 0 1" {
		t.Fatalf("pruneEmptyBloomFstabSection() = %#v, want root line only", got)
	}
}

func TestPruneEmptyBloomFstabSectionKeepsMarkersWhenTaggedEntriesRemain(t *testing.T) {
	input := []string{
		fstabSectionHeader,
		"UUID=abc /mnt/disk0 ext4 defaults 0 2 # managed by cluster-bloom",
		fstabSectionFooter,
	}
	got := pruneEmptyBloomFstabSection(input)
	if len(got) != len(input) {
		t.Fatalf("pruneEmptyBloomFstabSection() = %#v, want markers preserved", got)
	}
}

func TestPruneEmptyBloomFstabSectionKeepsMarkersForPremountedEntries(t *testing.T) {
	input := []string{
		fstabSectionHeader,
		"UUID=abc /data/storage ext4 defaults 0 2 # premounted by cluster-bloom",
		fstabSectionFooter,
	}
	got := pruneEmptyBloomFstabSection(input)
	if len(got) != len(input) {
		t.Fatalf("pruneEmptyBloomFstabSection() = %#v, want premounted entry to retain section", got)
	}
}
