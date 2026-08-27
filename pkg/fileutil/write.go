package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteAtomically writes data to path through a temporary file in the same
// directory and renames it into place, so a reader only ever observes the old
// or the new content. Writing in place would truncate path for the duration of
// the write, and an interrupt landing in that window makes the truncation
// permanent.
//
// The rename is followed by a directory fsync, so a caller may rely on the new
// content being durable once this returns. Callers that write a backup before
// replacing the original depend on that ordering.
func WriteAtomically(path string, data []byte, mode os.FileMode) error {
	return WriteAtomicallyOwned(path, data, mode, -1, -1)
}

// WriteAtomicallyOwned is WriteAtomically with ownership. A uid or gid of -1
// leaves that value unchanged, matching os.Chown.
func WriteAtomicallyOwned(path string, data []byte, mode os.FileMode, uid, gid int) (err error) {
	directory := filepath.Dir(path)
	temp, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", path, err)
	}
	tempPath := temp.Name()
	renamed := false
	defer func() {
		temp.Close()
		if !renamed {
			os.Remove(tempPath)
		}
	}()

	if _, err = temp.Write(data); err != nil {
		return fmt.Errorf("write temporary file for %s: %w", path, err)
	}
	// CreateTemp opens 0600, so widening the mode only after the content is
	// written means it is never briefly readable to more than the final mode
	// allows.
	if err = temp.Chmod(mode.Perm()); err != nil {
		return fmt.Errorf("set permissions on temporary file for %s: %w", path, err)
	}
	if uid >= 0 || gid >= 0 {
		if err = temp.Chown(uid, gid); err != nil {
			return fmt.Errorf("set ownership on temporary file for %s: %w", path, err)
		}
	}
	if err = temp.Sync(); err != nil {
		return fmt.Errorf("sync temporary file for %s: %w", path, err)
	}
	if err = temp.Close(); err != nil {
		return fmt.Errorf("close temporary file for %s: %w", path, err)
	}
	if err = os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace %s atomically: %w", path, err)
	}
	renamed = true

	directoryHandle, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("open directory %s after replacing %s: %w", directory, path, err)
	}
	defer directoryHandle.Close()
	if err = directoryHandle.Sync(); err != nil {
		return fmt.Errorf("sync directory %s after replacing %s: %w", directory, path, err)
	}
	return nil
}
