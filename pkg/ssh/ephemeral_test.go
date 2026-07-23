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
	"os"
	"path/filepath"
	"testing"
)

// TestValidateAuthorizedKeysNormalizesPermissions verifies that a too-loose
// ~/.ssh (0755) and authorized_keys (0664) — which sshd's StrictModes would
// reject — are auto-tightened to 0700/0600 instead of hard-failing on a manual
// chmod.
func TestValidateAuthorizedKeysNormalizesPermissions(t *testing.T) {
	sshDir := t.TempDir()
	authKeys := filepath.Join(sshDir, "authorized_keys")

	if err := os.WriteFile(authKeys, []byte("ssh-ed25519 AAAA... user@host\n"), 0600); err != nil {
		t.Fatalf("write authorized_keys: %v", err)
	}
	// chmod explicitly (not via WriteFile mode, which the umask would trim) so
	// this reproduces the exact group-writable 0664 case sshd rejects.
	if err := os.Chmod(authKeys, 0664); err != nil {
		t.Fatalf("chmod authorized_keys: %v", err)
	}
	// t.TempDir() is 0700 by default; loosen it to exercise the dir fix.
	if err := os.Chmod(sshDir, 0755); err != nil {
		t.Fatalf("chmod ssh dir: %v", err)
	}

	e := &EphemeralSSHManager{Username: "tester", AuthorizedKeysPath: authKeys}

	if err := e.validateAuthorizedKeys(); err != nil {
		t.Fatalf("validateAuthorizedKeys should self-heal permissions, got error: %v", err)
	}

	if perm := statPerm(t, sshDir); perm != 0700 {
		t.Errorf(".ssh dir perms = %o, want 700", perm)
	}
	if perm := statPerm(t, authKeys); perm != 0600 {
		t.Errorf("authorized_keys perms = %o, want 600", perm)
	}
}

// TestValidateAuthorizedKeysAcceptsCorrectPermissions confirms the common,
// already-correct case is a no-op that leaves perms untouched.
func TestValidateAuthorizedKeysAcceptsCorrectPermissions(t *testing.T) {
	sshDir := t.TempDir()
	authKeys := filepath.Join(sshDir, "authorized_keys")

	if err := os.WriteFile(authKeys, []byte("ssh-ed25519 AAAA... user@host\n"), 0600); err != nil {
		t.Fatalf("write authorized_keys: %v", err)
	}
	if err := os.Chmod(sshDir, 0700); err != nil {
		t.Fatalf("chmod ssh dir: %v", err)
	}

	e := &EphemeralSSHManager{Username: "tester", AuthorizedKeysPath: authKeys}

	if err := e.validateAuthorizedKeys(); err != nil {
		t.Fatalf("unexpected error on correctly-permissioned files: %v", err)
	}
	if perm := statPerm(t, authKeys); perm != 0600 {
		t.Errorf("authorized_keys perms = %o, want 600", perm)
	}
}

// TestValidateAuthorizedKeysCreatesMissingFile verifies a missing
// authorized_keys is created empty with 0600 rather than hard-failing.
func TestValidateAuthorizedKeysCreatesMissingFile(t *testing.T) {
	sshDir := t.TempDir() // exists, 0700
	authKeys := filepath.Join(sshDir, "authorized_keys")

	e := &EphemeralSSHManager{Username: "tester", AuthorizedKeysPath: authKeys}

	if err := e.validateAuthorizedKeys(); err != nil {
		t.Fatalf("validateAuthorizedKeys should create a missing authorized_keys, got error: %v", err)
	}
	if !e.createdAuthKeys {
		t.Errorf("createdAuthKeys = false, want true")
	}
	if e.createdSSHDir {
		t.Errorf("createdSSHDir = true, want false (dir already existed)")
	}
	if perm := statPerm(t, authKeys); perm != 0600 {
		t.Errorf("created authorized_keys perms = %o, want 600", perm)
	}
}

// TestValidateAuthorizedKeysCreatesMissingDir verifies a missing ~/.ssh is
// created 0700 along with the authorized_keys file.
func TestValidateAuthorizedKeysCreatesMissingDir(t *testing.T) {
	sshDir := filepath.Join(t.TempDir(), ".ssh") // does not exist yet
	authKeys := filepath.Join(sshDir, "authorized_keys")

	e := &EphemeralSSHManager{Username: "tester", AuthorizedKeysPath: authKeys}

	if err := e.validateAuthorizedKeys(); err != nil {
		t.Fatalf("validateAuthorizedKeys should create a missing ~/.ssh, got error: %v", err)
	}
	if !e.createdSSHDir || !e.createdAuthKeys {
		t.Errorf("createdSSHDir=%v createdAuthKeys=%v, want both true", e.createdSSHDir, e.createdAuthKeys)
	}
	if perm := statPerm(t, sshDir); perm != 0700 {
		t.Errorf("created .ssh dir perms = %o, want 700", perm)
	}
	if perm := statPerm(t, authKeys); perm != 0600 {
		t.Errorf("created authorized_keys perms = %o, want 600", perm)
	}
}

// TestRemoveCreatedIfEmpty verifies cleanup removes a bloom-created empty file
// (and dir) but never removes one with real content.
func TestRemoveCreatedIfEmpty(t *testing.T) {
	t.Run("removes empty created file and dir", func(t *testing.T) {
		sshDir := filepath.Join(t.TempDir(), ".ssh")
		authKeys := filepath.Join(sshDir, "authorized_keys")
		e := &EphemeralSSHManager{Username: "tester", AuthorizedKeysPath: authKeys}
		if err := e.validateAuthorizedKeys(); err != nil {
			t.Fatalf("setup: %v", err)
		}

		e.removeCreatedIfEmpty()

		if _, err := os.Stat(authKeys); !os.IsNotExist(err) {
			t.Errorf("authorized_keys should have been removed, stat err = %v", err)
		}
		if _, err := os.Stat(sshDir); !os.IsNotExist(err) {
			t.Errorf(".ssh dir should have been removed, stat err = %v", err)
		}
	})

	t.Run("keeps file bloom did not create", func(t *testing.T) {
		sshDir := t.TempDir()
		authKeys := filepath.Join(sshDir, "authorized_keys")
		if err := os.WriteFile(authKeys, []byte(""), 0600); err != nil {
			t.Fatalf("write: %v", err)
		}
		// createdAuthKeys stays false because the file already existed.
		e := &EphemeralSSHManager{Username: "tester", AuthorizedKeysPath: authKeys}
		if err := e.validateAuthorizedKeys(); err != nil {
			t.Fatalf("setup: %v", err)
		}

		e.removeCreatedIfEmpty()

		if _, err := os.Stat(authKeys); err != nil {
			t.Errorf("pre-existing authorized_keys must not be removed, got %v", err)
		}
	})

	t.Run("keeps created file that gained real content", func(t *testing.T) {
		sshDir := t.TempDir()
		authKeys := filepath.Join(sshDir, "authorized_keys")
		e := &EphemeralSSHManager{Username: "tester", AuthorizedKeysPath: authKeys}
		if err := e.validateAuthorizedKeys(); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := os.WriteFile(authKeys, []byte("ssh-ed25519 AAAA... real@key\n"), 0600); err != nil {
			t.Fatalf("write: %v", err)
		}

		e.removeCreatedIfEmpty()

		if _, err := os.Stat(authKeys); err != nil {
			t.Errorf("authorized_keys with real content must not be removed, got %v", err)
		}
	})
}

func statPerm(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}
