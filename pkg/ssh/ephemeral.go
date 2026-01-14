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
	"strings"

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

	return &EphemeralSSHManager{
		WorkDir:              workDir,
		Username:             username,
		PrivateKeyPath:       filepath.Join(sshDir, "id_ephemeral"),
		PublicKeyPath:        filepath.Join(sshDir, "id_ephemeral.pub"),
		AuthorizedKeysPath:   filepath.Join(userSSHDir, "authorized_keys"),
		AuthorizedKeysBackup: filepath.Join(sshDir, "authorized_keys.backup"),
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
	fmt.Println("🔑 Setting up ephemeral SSH key for Ansible connections...")

	// Generate ephemeral SSH key pair
	if err := e.generateKey(); err != nil {
		return fmt.Errorf("failed to generate SSH key: %w", err)
	}

	// Install public key to authorized_keys
	if err := e.installPublicKey(); err != nil {
		return fmt.Errorf("failed to install public key: %w", err)
	}

	// Log all files created/edited
	e.logFileOperations()

	fmt.Println("   ✓ Ephemeral SSH key setup completed")
	return nil
}

// Cleanup removes the ephemeral public key and restores original authorized_keys
func (e *EphemeralSSHManager) Cleanup() error {
	fmt.Println("🧹 Cleaning up ephemeral SSH key...")

	// Remove public key from authorized_keys
	if err := e.removePublicKey(); err != nil {
		fmt.Printf("   Warning: Failed to remove public key: %v\n", err)
	}

	// Remove ephemeral key files
	if err := e.removeKeyFiles(); err != nil {
		fmt.Printf("   Warning: Failed to remove key files: %v\n", err)
	}

	fmt.Println("   ✓ Ephemeral SSH key cleanup completed")
	return nil
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

// installPublicKey backs up original authorized_keys and adds ephemeral public key
func (e *EphemeralSSHManager) installPublicKey() error {
	// Ensure user's .ssh directory exists
	userSSHDir := filepath.Dir(e.AuthorizedKeysPath)
	if err := os.MkdirAll(userSSHDir, 0700); err != nil {
		return fmt.Errorf("failed to create user SSH directory: %w", err)
	}

	// Backup existing authorized_keys if it exists
	if _, err := os.Stat(e.AuthorizedKeysPath); err == nil {
		if err := copyFile(e.AuthorizedKeysPath, e.AuthorizedKeysBackup); err != nil {
			return fmt.Errorf("failed to backup authorized_keys: %w", err)
		}
	}

	// Read ephemeral public key
	pubKeyContent, err := os.ReadFile(e.PublicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read public key: %w", err)
	}

	// Append ephemeral public key with comment
	comment := "# bloom-ephemeral-key"
	keyEntry := fmt.Sprintf("\n%s %s\n", strings.TrimSpace(string(pubKeyContent)), comment)

	// Append to authorized_keys
	authKeysFile, err := os.OpenFile(e.AuthorizedKeysPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open authorized_keys: %w", err)
	}
	defer authKeysFile.Close()

	if _, err := authKeysFile.WriteString(keyEntry); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	e.isInstalled = true
	return nil
}

// removePublicKey restores original authorized_keys from backup
func (e *EphemeralSSHManager) removePublicKey() error {
	if !e.isInstalled {
		return nil // Nothing to remove
	}

	// Check if backup exists
	if _, err := os.Stat(e.AuthorizedKeysBackup); err == nil {
		// Restore from backup
		return copyFile(e.AuthorizedKeysBackup, e.AuthorizedKeysPath)
	} else {
		// No backup existed, remove the authorized_keys file entirely
		if err := os.Remove(e.AuthorizedKeysPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove authorized_keys: %w", err)
		}
	}

	e.isInstalled = false
	return nil
}

// removeKeyFiles deletes the ephemeral key files and backup
func (e *EphemeralSSHManager) removeKeyFiles() error {
	files := []string{
		e.PrivateKeyPath,
		e.PublicKeyPath,
		e.AuthorizedKeysBackup,
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

// logFileOperations logs all files created/edited during SSH setup
func (e *EphemeralSSHManager) logFileOperations() {
	fmt.Println("   📁 Files created/edited for SSH:")

	// Log generated key files
	if _, err := os.Stat(e.PrivateKeyPath); err == nil {
		fmt.Printf("   • Created: %s (private key)\n", e.PrivateKeyPath)
	}

	if _, err := os.Stat(e.PublicKeyPath); err == nil {
		fmt.Printf("   • Created: %s (public key)\n", e.PublicKeyPath)
	}

	// Log backup file if created
	if _, err := os.Stat(e.AuthorizedKeysBackup); err == nil {
		fmt.Printf("   • Created: %s (authorized_keys backup)\n", e.AuthorizedKeysBackup)
	}

	// Log modified authorized_keys
	if e.isInstalled {
		fmt.Printf("   • Modified: %s (added ephemeral public key)\n", e.AuthorizedKeysPath)
	}

	// Log SSH directory
	sshDir := filepath.Dir(e.PrivateKeyPath)
	if _, err := os.Stat(sshDir); err == nil {
		fmt.Printf("   • Created: %s/ (SSH working directory)\n", sshDir)
	}
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
