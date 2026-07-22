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

func statPerm(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}
