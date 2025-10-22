/**
 * Copyright 2025 Advanced Micro Devices, Inc.  All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
**/

package fsops

import (
	"io/fs"
	"os"

	"github.com/silogen/cluster-bloom/pkg/dryrun"
	log "github.com/sirupsen/logrus"
)

// WriteFile writes data to a file named by filename.
// If dry-run mode is enabled, it logs the operation instead.
func WriteFile(name string, data []byte, perm fs.FileMode) error {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] WRITE: %s (%d bytes, perm: %o)", name, len(data), perm)
		return nil
	}
	return os.WriteFile(name, data, perm)
}

// Create creates or truncates the named file.
// If dry-run mode is enabled, it logs the operation instead.
func Create(name string) (*os.File, error) {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] CREATE: %s", name)
		return nil, nil
	}
	return os.Create(name)
}

// Remove removes the named file or (empty) directory.
// If dry-run mode is enabled, it logs the operation instead.
func Remove(name string) error {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] REMOVE: %s", name)
		return nil
	}
	return os.Remove(name)
}

// RemoveAll removes path and any children it contains.
// If dry-run mode is enabled, it logs the operation instead.
func RemoveAll(path string) error {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] REMOVE_ALL: %s", path)
		return nil
	}
	return os.RemoveAll(path)
}

// Mkdir creates a new directory with the specified name and permission bits.
// If dry-run mode is enabled, it logs the operation instead.
func Mkdir(name string, perm fs.FileMode) error {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] MKDIR: %s (perm: %o)", name, perm)
		return nil
	}
	return os.Mkdir(name, perm)
}

// MkdirAll creates a directory named path, along with any necessary parents.
// If dry-run mode is enabled, it logs the operation instead.
func MkdirAll(path string, perm fs.FileMode) error {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] MKDIR_ALL: %s (perm: %o)", path, perm)
		return nil
	}
	return os.MkdirAll(path, perm)
}

// Rename renames (moves) oldpath to newpath.
// If dry-run mode is enabled, it logs the operation instead.
func Rename(oldpath, newpath string) error {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] RENAME: %s -> %s", oldpath, newpath)
		return nil
	}
	return os.Rename(oldpath, newpath)
}

// Chmod changes the mode of the named file to mode.
// If dry-run mode is enabled, it logs the operation instead.
func Chmod(name string, mode fs.FileMode) error {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] CHMOD: %s (mode: %o)", name, mode)
		return nil
	}
	return os.Chmod(name, mode)
}

// Chown changes the numeric uid and gid of the named file.
// If dry-run mode is enabled, it logs the operation instead.
func Chown(name string, uid, gid int) error {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] CHOWN: %s (uid: %d, gid: %d)", name, uid, gid)
		return nil
	}
	return os.Chown(name, uid, gid)
}

// OpenFile is the generalized open call.
// If dry-run mode is enabled, it logs the operation and returns nil for write-mode operations.
func OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	if dryrun.IsDryRun() {
		// Determine operation type from flags
		operation := "OPEN"
		if flag&os.O_CREATE != 0 {
			operation = "CREATE"
		} else if flag&os.O_APPEND != 0 {
			operation = "APPEND"
		} else if flag&os.O_WRONLY != 0 || flag&os.O_RDWR != 0 {
			operation = "WRITE"
		}
		log.Infof("[DRY-RUN] OPEN_FILE %s: %s (flag: %d, perm: %o)", operation, name, flag, perm)

		// For write operations, return nil (don't actually open)
		if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_APPEND) != 0 {
			return nil, nil
		}

		// For read-only operations, allow actual open (needed for reading config, etc.)
		return os.OpenFile(name, flag, perm)
	}
	return os.OpenFile(name, flag, perm)
}

// Symlink creates newname as a symbolic link to oldname.
// If dry-run mode is enabled, it logs the operation instead.
func Symlink(oldname, newname string) error {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] SYMLINK: %s -> %s", newname, oldname)
		return nil
	}
	return os.Symlink(oldname, newname)
}

// Link creates newname as a hard link to the oldname file.
// If dry-run mode is enabled, it logs the operation instead.
func Link(oldname, newname string) error {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] LINK: %s -> %s", newname, oldname)
		return nil
	}
	return os.Link(oldname, newname)
}

// logDryRun logs a filesystem operation in dry-run mode with consistent formatting
func logDryRun(operation, details string) {
	log.Infof("[DRY-RUN] %s: %s", operation, details)
}

// AppendToFile is a helper that opens a file in append mode and writes data to it.
// If dry-run mode is enabled, it logs the operation instead.
func AppendToFile(name string, data []byte) error {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] APPEND: %s (%d bytes)", name, len(data))
		return nil
	}

	file, err := os.OpenFile(name, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

// WriteString is a helper that writes a string to a file.
// If dry-run mode is enabled, it logs the operation instead.
func WriteString(name string, data string) error {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] WRITE_STRING: %s (%d chars)", name, len(data))
		return nil
	}

	return os.WriteFile(name, []byte(data), 0644)
}

// CreateTemp creates a new temporary file in the directory dir.
// If dry-run mode is enabled, it logs the operation and returns a nil file.
func CreateTemp(dir, pattern string) (*os.File, error) {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] CREATE_TEMP: dir=%s, pattern=%s", dir, pattern)
		return nil, nil
	}
	return os.CreateTemp(dir, pattern)
}

// MkdirTemp creates a new temporary directory in the directory dir.
// If dry-run mode is enabled, it logs the operation and returns empty string.
func MkdirTemp(dir, pattern string) (string, error) {
	if dryrun.IsDryRun() {
		log.Infof("[DRY-RUN] MKDIR_TEMP: dir=%s, pattern=%s", dir, pattern)
		return "", nil
	}
	return os.MkdirTemp(dir, pattern)
}
