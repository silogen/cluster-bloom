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
}

// NewEphemeralSSHManager creates a new ephemeral SSH key manager for single-node deployment
func NewEphemeralSSHManager(workDir, username string) *EphemeralSSHManager {
	sshDir := filepath.Join(workDir, "ssh")

	// Get the user's actual home directory
	userSSHDir := getUserSSHDir(username)

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
	}
}

// getUserSSHDir returns the .ssh directory path for the given username
// Handles various scenarios including sudo, root, and different home directory layouts
//
// This function properly resolves user home directories by:
// 1. Using os/user.Lookup() to get actual home directory (works for any user layout)
// 2. Falling back to well-known paths if lookup fails
// 3. Handling root user specially (/root vs /home/root)
//
// When running with sudo, the caller should pass SUDO_USER (original user) not root,
// so SSH keys get installed in the original user's home directory for proper access.
func getUserSSHDir(username string) string {
	// Try to look up the user to get their actual home directory
	if userInfo, err := user.Lookup(username); err == nil {
		return filepath.Join(userInfo.HomeDir, ".ssh")
	}

	// Fallback: construct path based on username
	if username == "root" {
		return "/root/.ssh"
	}

	// Default fallback for regular users
	return filepath.Join("/home", username, ".ssh")
}

// Setup generates ephemeral SSH keys and installs the public key for localhost access
func (e *EphemeralSSHManager) Setup() error {
	// Generate ephemeral key pair
	if err := e.generateKey(); err != nil {
		return fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	// Install public key to authorized_keys
	if err := e.installPublicKey(); err != nil {
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

	// Check if .ssh directory exists and has 700 permissions
	sshInfo, err := os.Stat(userSSHDir)
	if os.IsNotExist(err) {
		fmt.Printf("❌ ERROR: SSH directory does not exist: %s\n", userSSHDir)
		fmt.Printf("   Please ensure SSH is properly configured for user %s\n", e.Username)
		return fmt.Errorf("SSH directory missing: %s", userSSHDir)
	}
	if err != nil {
		return fmt.Errorf("failed to check SSH directory: %w", err)
	}

	sshPerms := sshInfo.Mode().Perm()
	if sshPerms != 0700 {
		fmt.Printf("❌ ERROR: SSH directory has incorrect permissions: %o (expected 700)\n", sshPerms)
		fmt.Printf("   Please fix permissions: chmod 700 %s\n", userSSHDir)
		return fmt.Errorf("SSH directory has incorrect permissions: %o", sshPerms)
	}

	// Check if authorized_keys file exists and has 600 permissions
	authKeysInfo, err := os.Stat(e.AuthorizedKeysPath)
	if os.IsNotExist(err) {
		fmt.Printf("❌ ERROR: authorized_keys file does not exist: %s\n", e.AuthorizedKeysPath)
		fmt.Printf("   Please ensure authorized_keys is properly configured for user %s\n", e.Username)
		return fmt.Errorf("authorized_keys file missing: %s", e.AuthorizedKeysPath)
	}
	if err != nil {
		return fmt.Errorf("failed to check authorized_keys file: %w", err)
	}

	authKeysPerms := authKeysInfo.Mode().Perm()
	if authKeysPerms != 0600 {
		fmt.Printf("❌ ERROR: authorized_keys file has incorrect permissions: %o (expected 600)\n", authKeysPerms)
		fmt.Printf("   Please fix permissions: chmod 600 %s\n", e.AuthorizedKeysPath)
		return fmt.Errorf("authorized_keys file has incorrect permissions: %o", authKeysPerms)
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
		fmt.Printf("❌ ERROR: Bloom ephemeral key already exists in authorized_keys\n")
		fmt.Printf("   This indicates a previous bloom run was not cleaned up properly.\n")
		fmt.Printf("   RECOMMENDED: Restore from backup (if available):\n")
		fmt.Printf("   1. Check for backup files: ls %s*.backup.*\n", e.AuthorizedKeysPath)
		fmt.Printf("   2. Restore the most recent backup:\n")
		fmt.Printf("      cp %s.backup.YYYYMMDD_HHMMSS %s\n", e.AuthorizedKeysPath, e.AuthorizedKeysPath)
		fmt.Printf("   3. Re-run bloom\n")
		fmt.Printf("   ALTERNATIVE: Manual removal (if no backup available):\n")
		fmt.Printf("   1. Edit the file: nano %s\n", e.AuthorizedKeysPath)
		fmt.Printf("   2. Look for and remove lines containing:\n")
		fmt.Printf("      - '# bloom-ephemeral-key' (comment marker)\n")
		fmt.Printf("      - 'bloom-ephemeral@localhost' (hostname marker)\n")
		fmt.Printf("   3. The line will look like:\n")
		fmt.Printf("      ssh-ed25519 AAAAC3NzaC... bloom-ephemeral@localhost # bloom-ephemeral-key\n")
		fmt.Printf("   4. Delete the entire line(s) with bloom markers\n")
		fmt.Printf("   5. Save and re-run bloom\n")
		return fmt.Errorf("bloom ephemeral key already exists in authorized_keys")
	}

	return nil
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

		// Create backup
		if err := copyFile(e.AuthorizedKeysPath, e.AuthorizedKeysBackup); err != nil {
			return fmt.Errorf("failed to backup authorized_keys: %w", err)
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
		if e.verifyBackup() {
			if copyErr := copyFile(e.AuthorizedKeysBackup, e.AuthorizedKeysPath); copyErr == nil {
				e.isInstalled = false
				return nil
			}
		}
		return nil
	})
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
