package fileutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAtomicallyReplacesCompleteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fstab")
	if err := os.WriteFile(path, []byte("old content"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := WriteAtomically(path, []byte("new complete content"), 0644); err != nil {
		t.Fatalf("WriteAtomically() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "new complete content" {
		t.Fatalf("WriteAtomically() content = %q, want complete replacement", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Fatalf("WriteAtomically() mode = %o, want 0644", info.Mode().Perm())
	}
}

func TestWriteAtomicallyCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authorized_keys")

	if err := WriteAtomically(path, []byte("ssh-ed25519 AAAA"), 0600); err != nil {
		t.Fatalf("WriteAtomically() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "ssh-ed25519 AAAA" {
		t.Fatalf("WriteAtomically() content = %q, want the written content", got)
	}
}

// CreateTemp opens the temporary file 0600, so a mode narrower than that must
// still be applied rather than left at the default.
func TestWriteAtomicallyAppliesMode(t *testing.T) {
	for _, mode := range []os.FileMode{0600, 0640, 0644, 0400} {
		t.Run(fmt.Sprintf("%o", mode), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "target")
			if err := WriteAtomically(path, []byte("x"), mode); err != nil {
				t.Fatalf("WriteAtomically() error = %v", err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("Stat() error = %v", err)
			}
			if info.Mode().Perm() != mode {
				t.Fatalf("mode = %o, want %o", info.Mode().Perm(), mode)
			}
		})
	}
}

// The guarantee the package exists for, asserted without racing the writer: a
// reader that opened the file before the write still reads the complete old
// content afterwards. An in-place truncate would empty the file under that
// reader, which is what made an empty authorized_keys permanent in the field.
func TestWriteAtomicallyLeavesOpenReaderOnCompleteOldContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authorized_keys")
	const old = "ssh-ed25519 AAAAold operator@host\n"
	if err := os.WriteFile(path, []byte(old), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	reader, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()

	if err := WriteAtomically(path, []byte("ssh-ed25519 AAAAnew operator@host\n"), 0600); err != nil {
		t.Fatalf("WriteAtomically() error = %v", err)
	}

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(got) != old {
		t.Errorf("reader opened before the write saw %q, want the complete old content %q", got, old)
	}
}

// The mechanism behind the guarantee above: the target is only ever reached by
// renaming a new file over it, never opened for writing, so it is a different
// inode afterwards. Replacing the body with os.WriteFile would keep the inode.
func TestWriteAtomicallyReplacesInodeRatherThanWritingInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fstab")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}

	if err := WriteAtomically(path, []byte("new"), 0644); err != nil {
		t.Fatalf("WriteAtomically() error = %v", err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if os.SameFile(before, after) {
		t.Error("target is the same file as before the write, so it was modified in place rather than replaced by rename")
	}
}

func TestWriteAtomicallyLeavesNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fstab")

	if err := WriteAtomically(path, []byte("content"), 0644); err != nil {
		t.Fatalf("WriteAtomically() error = %v", err)
	}

	if leftovers := temporaryFilesIn(t, dir, "fstab"); len(leftovers) > 0 {
		t.Errorf("temporary files left behind after a successful write: %v", leftovers)
	}
}

// A failed write must leave the original exactly as it was. This is the field
// scenario: the operation does not complete, and the file must survive it.
func TestWriteAtomicallyLeavesOriginalIntactWhenWriteFails(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores the directory permissions this test relies on")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "authorized_keys")
	const original = "ssh-ed25519 AAAAoriginal operator@host\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	// Denying write on the directory fails the temporary file creation, so the
	// write cannot get as far as the rename.
	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0700) })

	if err := WriteAtomically(path, []byte("replacement"), 0600); err == nil {
		t.Fatal("WriteAtomically() error = nil in a directory that denies writes, want an error")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != original {
		t.Errorf("after a failed write the file holds %q, want the untouched original %q", got, original)
	}
}

// A failed rename must not litter the directory either. Renaming onto an
// existing directory fails after the temporary file already exists.
func TestWriteAtomicallyCleansUpWhenRenameFails(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "occupied")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatalf("Mkdir() error = %v", err)
	}

	if err := WriteAtomically(target, []byte("content"), 0644); err == nil {
		t.Fatal("WriteAtomically() error = nil when renaming onto a directory, want an error")
	}

	if leftovers := temporaryFilesIn(t, parent, "occupied"); len(leftovers) > 0 {
		t.Errorf("temporary files left behind after a failed write: %v", leftovers)
	}
}

// Exercises the chown branch, which WriteAtomically skips by passing -1. A real
// ownership change needs root, so this only confirms that a caller-supplied
// uid/gid is accepted and does not break the write.
func TestWriteAtomicallyOwnedAcceptsCurrentOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "authorized_keys")

	if err := WriteAtomicallyOwned(path, []byte("content"), 0600, os.Getuid(), os.Getgid()); err != nil {
		t.Fatalf("WriteAtomicallyOwned() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(got) != "content" {
		t.Fatalf("content = %q, want the written content", got)
	}
}

func temporaryFilesIn(t *testing.T, dir, base string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	var found []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "."+base+".tmp-") {
			found = append(found, entry.Name())
		}
	}
	return found
}
