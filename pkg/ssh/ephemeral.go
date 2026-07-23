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

package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
)

// EphemeralSSHManager manages temporary SSH keys for single-node Ansible deployments
type EphemeralSSHManager struct {
	WorkDir              string // Bloom work directory
	Username             string // Target user (from SUDO_USER/USER)
	PrivateKeyPath       string // {workdir}/ssh/id_ephemeral
	PublicKeyPath        string // {workdir}/ssh/id_ephemeral.pub
	AuthorizedKeysPath   string // /home/{username}/.ssh/authorized_keys
	AuthorizedKeysBackup string // {workdir}/ssh/authorized_keys.backup
	isInstalled          bool   // Track installation state for cleanup
	createdSSHDir        bool   // ~/.ssh did not exist and was created by bloom
	createdAuthKeys      bool   // authorized_keys did not exist and was created by bloom
}

// NewEphemeralSSHManager creates a new ephemeral SSH key manager for single-node deployment
func NewEphemeralSSHManager(workDir, username string) (*EphemeralSSHManager, error) {
	sshDir := filepath.Join(workDir, "ssh")

	// Get the user's actual home directory
	userSSHDir, err := getUserSSHDir(username)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH manager: %w", err)
	}

	// Generate timestamped backup filename in the same directory as authorized_keys
	timestamp := time.Now().Format("20060102_150405")
	backupFilename := fmt.Sprintf("authorized_keys.backup.%s", timestamp)
	authKeysBackupPath := filepath.Join(userSSHDir, backupFilename)

	return &EphemeralSSHManager{
		WorkDir:              workDir,
		Username:             username,
		PrivateKeyPath:       filepath.Join(sshDir, "id_ephemeral"),
		PublicKeyPath:        filepath.Join(sshDir, "id_ephemeral.pub"),
		AuthorizedKeysPath:   filepath.Join(userSSHDir, "authorized_keys"),
		AuthorizedKeysBackup: authKeysBackupPath,
		isInstalled:          false,
	}, nil
}
func getUserSSHDir(username string) (string, error) {
	// Look up the actual user to get their home directory
	// Don't rely on HOME env var as it may point to /root when using sudo
	userInfo, err := user.Lookup(username)
	if err != nil {
		return "", fmt.Errorf("failed to lookup user %s: %w", username, err)
	}
	return filepath.Join(userInfo.HomeDir, ".ssh"), nil
}

// Setup generates ephemeral SSH keys and installs the public key for localhost access
func (e *EphemeralSSHManager) Setup() error {
	// Generate ephemeral key pair
	if err := e.generateKey(); err != nil {
		// Best-effort: remove any partially-written key material so a failed
		// Setup does not leak the ephemeral private key on disk.
		e.removeKeyFiles()
		return fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	// Install public key to authorized_keys
	if err := e.installPublicKey(); err != nil {
		// The defer'd Cleanup() in the caller is only registered after Setup()
		// returns nil, so on failure here nothing else removes the generated
		// key files. Clean them up now to avoid leaving the ephemeral key (and
		// its work dir) behind.
		e.removeKeyFiles()
		return fmt.Errorf("failed to install public key: %w", err)
	}

	return nil
}

// Cleanup removes the ephemeral public key and restores original authorized_keys
func (e *EphemeralSSHManager) Cleanup() error {
	// Remove public key from authorized_keys
	if err := e.removePublicKey(); err != nil {
		return fmt.Errorf("failed to remove SSH key for user %s: %w", e.Username, err)
	}

	// Remove ephemeral key files
	if err := e.removeKeyFiles(); err != nil {
		// Don't fail on key file cleanup, just continue
	}

	return nil
}

// verifyBackup checks if the backup file exists and is readable
func (e *EphemeralSSHManager) verifyBackup() bool {
	if stat, err := os.Stat(e.AuthorizedKeysBackup); err != nil {
		return false
	} else if stat.Size() == 0 {
		// Empty backup might be valid (no existing keys), but log it
		fmt.Printf("      ⚠️ Backup file is empty (may be valid if no keys existed)\n")
		return true
	}
	return true
}

// generateKey creates an ED25519 key pair for ephemeral use
func (e *EphemeralSSHManager) generateKey() error {
	// Ensure SSH directory exists
	sshDir := filepath.Dir(e.PrivateKeyPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create SSH directory: %w", err)
	}

	// Generate ED25519 key pair
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate ED25519 key: %w", err)
	}

	// Convert to PKCS8 format for PEM encoding
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	// Create PEM block for private key
	privateKeyPEM := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	// Write private key file
	privateKeyFile, err := os.OpenFile(e.PrivateKeyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create private key file: %w", err)
	}
	defer privateKeyFile.Close()

	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Generate SSH public key
	publicKey, err := ssh.NewPublicKey(privateKey.Public())
	if err != nil {
		return fmt.Errorf("failed to create SSH public key: %w", err)
	}

	// Format public key for OpenSSH
	publicKeyBytes := ssh.MarshalAuthorizedKey(publicKey)
	publicKeyString := fmt.Sprintf("%s bloom-ephemeral@localhost", strings.TrimSpace(string(publicKeyBytes)))

	// Write public key file
	if err := os.WriteFile(e.PublicKeyPath, []byte(publicKeyString), 0644); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	return nil
}

// validateAuthorizedKeys checks that .ssh directory and authorized_keys file exist with correct permissions
func (e *EphemeralSSHManager) validateAuthorizedKeys() error {
	userSSHDir := filepath.Dir(e.AuthorizedKeysPath)

	// Check if .ssh directory exists and has 700 permissions. bloom needs SSH to
	// localhost for the single-node Ansible run and owns the ephemeral key
	// lifecycle, so a missing ~/.ssh is self-healed (created 0700) rather than
	// hard-failing and forcing a manual mkdir. Ownership is corrected to the
	// target user by runAsUser after the operation.
	sshInfo, err := os.Stat(userSSHDir)
	if os.IsNotExist(err) {
		fmt.Printf("⚠️ SSH directory %s does not exist; creating it (0700).\n", userSSHDir)
		if mkErr := os.MkdirAll(userSSHDir, 0700); mkErr != nil {
			fmt.Printf("❌ ERROR: failed to create SSH directory: %v\n", mkErr)
			fmt.Printf("   Please create it manually: mkdir -p %s && chmod 700 %s\n", userSSHDir, userSSHDir)
			return fmt.Errorf("failed to create SSH directory %s: %w", userSSHDir, mkErr)
		}
		e.createdSSHDir = true
		if sshInfo, err = os.Stat(userSSHDir); err != nil {
			return fmt.Errorf("failed to stat SSH directory after creating it: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check SSH directory: %w", err)
	}

	sshPerms := sshInfo.Mode().Perm()
	if sshPerms != 0700 {
		// bloom owns the lifecycle of the ephemeral key in this dir, and sshd's
		// StrictModes rejects a group/other-accessible ~/.ssh anyway, so
		// normalize to 0700 rather than hard-failing and forcing a manual
		// chmod (matching the leftover-key self-heal below). This only ever
		// tightens permissions.
		fmt.Printf("⚠️ SSH directory %s has permissions %o (expected 700); fixing to 700.\n", userSSHDir, sshPerms)
		if err := os.Chmod(userSSHDir, 0700); err != nil {
			fmt.Printf("❌ ERROR: failed to fix SSH directory permissions: %v\n", err)
			fmt.Printf("   Please fix permissions manually: chmod 700 %s\n", userSSHDir)
			return fmt.Errorf("SSH directory has incorrect permissions %o and could not be fixed: %w", sshPerms, err)
		}
	}

	// Check if authorized_keys file exists and has 600 permissions. A missing
	// file is self-healed by creating an empty one (0600) — this is exactly what
	// ssh-copy-id does and is less invasive than the permission fix-ups below,
	// which already run against an existing file. createdAuthKeys is tracked so
	// cleanup can remove the file (if still empty) to leave no trace.
	authKeysInfo, err := os.Stat(e.AuthorizedKeysPath)
	if os.IsNotExist(err) {
		fmt.Printf("⚠️ authorized_keys %s does not exist; creating an empty one (0600).\n", e.AuthorizedKeysPath)
		if wErr := os.WriteFile(e.AuthorizedKeysPath, []byte{}, 0600); wErr != nil {
			fmt.Printf("❌ ERROR: failed to create authorized_keys: %v\n", wErr)
			fmt.Printf("   Please create it manually: touch %s && chmod 600 %s\n", e.AuthorizedKeysPath, e.AuthorizedKeysPath)
			return fmt.Errorf("failed to create authorized_keys %s: %w", e.AuthorizedKeysPath, wErr)
		}
		e.createdAuthKeys = true
		if authKeysInfo, err = os.Stat(e.AuthorizedKeysPath); err != nil {
			return fmt.Errorf("failed to stat authorized_keys after creating it: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check authorized_keys file: %w", err)
	}

	authKeysPerms := authKeysInfo.Mode().Perm()
	if authKeysPerms != 0600 {
		// Same rationale as the .ssh dir above: bloom writes to this file, and
		// sshd ignores an authorized_keys that is group/other-writable, so
		// normalize to 0600 instead of aborting on a manual chmod.
		fmt.Printf("⚠️ authorized_keys %s has permissions %o (expected 600); fixing to 600.\n", e.AuthorizedKeysPath, authKeysPerms)
		if err := os.Chmod(e.AuthorizedKeysPath, 0600); err != nil {
			fmt.Printf("❌ ERROR: failed to fix authorized_keys permissions: %v\n", err)
			fmt.Printf("   Please fix permissions manually: chmod 600 %s\n", e.AuthorizedKeysPath)
			return fmt.Errorf("authorized_keys file has incorrect permissions %o and could not be fixed: %w", authKeysPerms, err)
		}
	}

	// Check if bloom ephemeral key already exists
	authKeysContent, err := os.ReadFile(e.AuthorizedKeysPath)
	if err != nil {
		return fmt.Errorf("failed to read authorized_keys for validation: %w", err)
	}

	// Check for either bloom marker (comment or hostname)
	hasBloomComment := strings.Contains(string(authKeysContent), "# bloom-ephemeral-key")
	hasBloomHostname := strings.Contains(string(authKeysContent), "bloom-ephemeral@localhost")

	if hasBloomComment || hasBloomHostname {
		// A leftover key means a previous bloom run did not clean up (e.g. it was
		// killed with SIGKILL, the terminal was closed, or the box lost power
		// before the signal/defer cleanup could run). Rather than hard-failing and
		// forcing a manual edit, self-heal by stripping the stale bloom line(s) so
		// this run can install a fresh key and proceed.
		fmt.Printf("⚠️ Found a leftover bloom ephemeral key in %s (a previous run was not cleaned up); removing it before continuing.\n", e.AuthorizedKeysPath)
		removed, err := e.removeStaleEphemeralKeys()
		if err != nil {
			fmt.Printf("❌ ERROR: failed to auto-remove the stale bloom ephemeral key: %v\n", err)
			fmt.Printf("   Manual removal:\n")
			fmt.Printf("   1. Edit the file: nano %s\n", e.AuthorizedKeysPath)
			fmt.Printf("   2. Delete any line containing '# bloom-ephemeral-key' or 'bloom-ephemeral@localhost'\n")
			fmt.Printf("   3. Save and re-run bloom\n")
			return fmt.Errorf("failed to remove stale bloom ephemeral key from authorized_keys: %w", err)
		}
		fmt.Printf("   Removed %d stale bloom key line(s); continuing.\n", removed)
	}

	return nil
}

// removeStaleEphemeralKeys strips any bloom ephemeral key lines from
// authorized_keys (lines carrying the "# bloom-ephemeral-key" comment marker or
// the "bloom-ephemeral@localhost" hostname marker). It is used both to self-heal
// a leftover key on startup and as a cleanup fallback when a backup restore is
// not possible. The file is rewritten atomically via a temp file with the
// original owner and permissions preserved. Returns the number of lines removed.
func (e *EphemeralSSHManager) removeStaleEphemeralKeys() (int, error) {
	content, err := os.ReadFile(e.AuthorizedKeysPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read authorized_keys: %w", err)
	}

	uid, gid, mode, err := getFileInfo(e.AuthorizedKeysPath)
	if err != nil {
		return 0, fmt.Errorf("failed to get authorized_keys file info: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	kept := make([]string, 0, len(lines))
	removed := 0
	for _, line := range lines {
		if strings.Contains(line, "# bloom-ephemeral-key") || strings.Contains(line, "bloom-ephemeral@localhost") {
			removed++
			continue
		}
		kept = append(kept, line)
	}

	if removed == 0 {
		return 0, nil
	}

	tmpPath := e.AuthorizedKeysPath + ".heal.tmp"
	if err := os.WriteFile(tmpPath, []byte(strings.Join(kept, "\n")), mode.Perm()); err != nil {
		return 0, fmt.Errorf("failed to write cleaned authorized_keys: %w", err)
	}
	if err := safelyOverwriteFile(tmpPath, e.AuthorizedKeysPath, uid, gid, mode); err != nil {
		os.Remove(tmpPath)
		return 0, fmt.Errorf("failed to overwrite authorized_keys: %w", err)
	}
	os.Remove(tmpPath)

	return removed, nil
}

// installPublicKey backs up original authorized_keys and adds ephemeral public key
// Process: 1) validate prerequisites, 2) backup, 3) temp copy, 4) add key, 5) fix ownership, 6) overwrite original
func (e *EphemeralSSHManager) installPublicKey() error {
	return e.runAsUser(func() error {
		// Validate .ssh directory and authorized_keys file exist with correct permissions
		if err := e.validateAuthorizedKeys(); err != nil {
			return err
		}

		// Step 1: Get original file info and create backup
		// (we know the file exists because validateAuthorizedKeys() passed)
		uid, gid, mode, err := getFileInfo(e.AuthorizedKeysPath)
		if err != nil {
			return fmt.Errorf("failed to get original file info: %w", err)
		}
		originalUID, originalGID, originalMode := uid, gid, mode

		// Create backup — skipped when bloom created authorized_keys this run,
		// since there is no prior content to preserve. Cleanup for that case
		// strips the ephemeral line and removes the file if it is left empty.
		if !e.createdAuthKeys {
			if err := copyFile(e.AuthorizedKeysPath, e.AuthorizedKeysBackup); err != nil {
				return fmt.Errorf("failed to backup authorized_keys: %w", err)
			}
		}

		// Step 2: Make a temporary copy of authorized_keys
		tmpPath := e.AuthorizedKeysPath + ".tmp"
		if err := copyFile(e.AuthorizedKeysPath, tmpPath); err != nil {
			return fmt.Errorf("failed to copy existing file: %w", err)
		}

		// Step 3: Add ephemeral key to temporary copy
		pubKeyContent, err := os.ReadFile(e.PublicKeyPath)
		if err != nil {
			os.Remove(tmpPath) // Clean up temp file
			return fmt.Errorf("failed to read public key: %w", err)
		}

		comment := "# bloom-ephemeral-key"
		keyEntry := fmt.Sprintf("\n%s %s\n", strings.TrimSpace(string(pubKeyContent)), comment)

		// Append to temporary file
		tmpFile, err := os.OpenFile(tmpPath, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to open temporary file: %w", err)
		}

		if _, err := tmpFile.WriteString(keyEntry); err != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("failed to write to temporary file: %w", err)
		}
		tmpFile.Close()

		// Step 4: Change ownership of temp file to match original
		if err := os.Chown(tmpPath, originalUID, originalGID); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to set ownership on temporary file: %w", err)
		}

		// Step 5: Overwrite original with temp file while preserving permissions
		if err := safelyOverwriteFile(tmpPath, e.AuthorizedKeysPath, originalUID, originalGID, originalMode); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("failed to overwrite original file: %w", err)
		}

		// Clean up temporary file
		os.Remove(tmpPath)

		e.isInstalled = true
		return nil
	})
}

// removePublicKey restores original authorized_keys from backup
// All operations run in the target user's context for proper file permissions
func (e *EphemeralSSHManager) removePublicKey() error {
	if !e.isInstalled {
		return nil // Nothing to remove
	}

	return e.runAsUser(func() error {
		// Preferred path: restore the pre-run backup (exact prior state).
		if e.verifyBackup() {
			if copyErr := copyFile(e.AuthorizedKeysBackup, e.AuthorizedKeysPath); copyErr == nil {
				e.isInstalled = false
				return nil
			}
		}
		// Fallback: strip the bloom-marked line(s) directly, so the ephemeral key
		// is removed even when the backup is missing or unreadable. Previously
		// this returned nil and reported success without removing anything, which
		// left the key behind and blocked the next run. This is also the normal
		// path when bloom created authorized_keys this run (no backup is taken).
		if _, err := e.removeStaleEphemeralKeys(); err != nil {
			return fmt.Errorf("failed to remove ephemeral key (backup restore unavailable): %w", err)
		}
		e.isInstalled = false
		e.removeCreatedIfEmpty()
		return nil
	})
}

// removeCreatedIfEmpty restores the pre-run state when bloom created the SSH
// resources itself: if bloom created authorized_keys and it is now empty (only
// the ephemeral line was ever in it), the file is removed; if bloom also created
// ~/.ssh and it is now empty, the directory is removed too. It never touches a
// file/dir that bloom did not create, and never removes a non-empty file.
func (e *EphemeralSSHManager) removeCreatedIfEmpty() {
	if !e.createdAuthKeys {
		return
	}

	content, err := os.ReadFile(e.AuthorizedKeysPath)
	if err != nil || strings.TrimSpace(string(content)) != "" {
		// File is gone, unreadable, or a user added real keys during the run —
		// leave it alone.
		return
	}
	if err := os.Remove(e.AuthorizedKeysPath); err != nil {
		return
	}
	fmt.Printf("   Removed the empty authorized_keys bloom created for this run.\n")

	if !e.createdSSHDir {
		return
	}
	userSSHDir := filepath.Dir(e.AuthorizedKeysPath)
	if entries, err := os.ReadDir(userSSHDir); err == nil && len(entries) == 0 {
		if err := os.Remove(userSSHDir); err == nil {
			fmt.Printf("   Removed the empty %s directory bloom created for this run.\n", userSSHDir)
		}
	}
}

// removeKeyFiles deletes the ephemeral key files (preserves backup)
func (e *EphemeralSSHManager) removeKeyFiles() error {
	files := []string{
		e.PrivateKeyPath,
		e.PublicKeyPath,
		// Note: AuthorizedKeysBackup is intentionally preserved
	}

	for _, file := range files {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			fmt.Printf("   Warning: Failed to remove %s: %v\n", file, err)
		}
	}

	// Try to remove SSH directory if empty
	sshDir := filepath.Dir(e.PrivateKeyPath)
	os.Remove(sshDir) // Ignore error - directory might not be empty

	return nil
}

// copyFile copies a file from src to dst, preserving permissions
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// getUserInfo gets the target user's uid and gid for context switching
func getUserInfo(username string) (uint32, uint32, error) {
	userInfo, err := user.Lookup(username)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to lookup user %s: %w", username, err)
	}

	uid, err := strconv.ParseUint(userInfo.Uid, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid uid for user %s: %w", username, err)
	}

	gid, err := strconv.ParseUint(userInfo.Gid, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid gid for user %s: %w", username, err)
	}

	return uint32(uid), uint32(gid), nil
}

// getFileInfo gets the ownership, permissions, and other info for a file
func getFileInfo(filePath string) (uid int, gid int, mode os.FileMode, err error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, 0, 0, err
	}

	// Get the underlying system-specific file info
	sys := fileInfo.Sys()
	if sys == nil {
		return 0, 0, fileInfo.Mode(), fmt.Errorf("unable to get system file info")
	}

	// Cast to unix-specific stat structure
	stat, ok := sys.(*syscall.Stat_t)
	if !ok {
		return 0, 0, fileInfo.Mode(), fmt.Errorf("unable to get unix stat info")
	}

	return int(stat.Uid), int(stat.Gid), fileInfo.Mode(), nil
}

// safelyOverwriteFile overwrites dst with src while preserving ownership and permissions
func safelyOverwriteFile(src, dst string, uid, gid int, mode os.FileMode) error {
	// Read source file content
	content, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	// Write to destination with preserved permissions
	if err := os.WriteFile(dst, content, mode.Perm()); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	// Set correct ownership
	if err := os.Chown(dst, uid, gid); err != nil {
		return fmt.Errorf("failed to set ownership: %w", err)
	}

	return nil
}

// runAsUser executes a function and ensures proper file ownership
// Instead of changing process UID, runs as root and sets correct ownership after operations
// This is more reliable and works around privilege switching limitations
func (e *EphemeralSSHManager) runAsUser(operation func() error) error {
	// Get target user information for ownership setting
	targetUID, targetGID, err := getUserInfo(e.Username)
	if err != nil {
		return fmt.Errorf("failed to get user info for ownership setting: %w", err)
	}

	// Execute the operation as root (current context)
	operationErr := operation()
	if operationErr != nil {
		return operationErr
	}

	// Set correct ownership on SSH files after operation
	if err := e.setSSHFileOwnership(int(targetUID), int(targetGID)); err != nil {
		return fmt.Errorf("failed to set correct ownership on SSH files: %w", err)
	}

	return nil
}

// setSSHFileOwnership ensures SSH directory and files have correct ownership
func (e *EphemeralSSHManager) setSSHFileOwnership(uid, gid int) error {
	// Set ownership on .ssh directory
	userSSHDir := filepath.Dir(e.AuthorizedKeysPath)
	if err := os.Chown(userSSHDir, uid, gid); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to set ownership on SSH directory %s: %w", userSSHDir, err)
	}

	// Set ownership on authorized_keys file if it exists
	if _, err := os.Stat(e.AuthorizedKeysPath); err == nil {
		if err := os.Chown(e.AuthorizedKeysPath, uid, gid); err != nil {
			return fmt.Errorf("failed to set ownership on authorized_keys: %w", err)
		}
	}

	// Set ownership on backup file if it exists
	if _, err := os.Stat(e.AuthorizedKeysBackup); err == nil {
		if err := os.Chown(e.AuthorizedKeysBackup, uid, gid); err != nil {
			return fmt.Errorf("failed to set ownership on backup file: %w", err)
		}
	}

	return nil
}
