//go:build linux

package runtime

import (
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
