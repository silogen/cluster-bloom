//go:build linux

package ssh

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

const testEphemeralKeyLine = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBloomEphemeralTestKeyPlaceholder00000 bloom-ephemeral@localhost # bloom-ephemeral-key\n"

// newTestManager builds a manager whose paths all live under t.TempDir() so a
// test can never touch the real ~/.ssh/authorized_keys. Username is the current
// user because runAsUser chowns the files it touches, and a non-root process may
// only chown to its own uid.
func newTestManager(t *testing.T) *EphemeralSSHManager {
	t.Helper()

	current, err := user.Current()
	if err != nil {
		t.Skipf("cannot determine current user: %v", err)
	}
	if _, err := user.Lookup(current.Username); err != nil {
		t.Skipf("cannot look up current user %q: %v", current.Username, err)
	}

	root := t.TempDir()
	homeSSHDir := filepath.Join(root, "home", ".ssh")
	workSSHDir := filepath.Join(root, "work", "ssh")
	for _, dir := range []string{homeSSHDir, workSSHDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatalf("MkdirAll(%s) = %v", dir, err)
		}
	}

	return &EphemeralSSHManager{
		WorkDir:              filepath.Join(root, "work"),
		Username:             current.Username,
		PrivateKeyPath:       filepath.Join(workSSHDir, "id_ephemeral"),
		PublicKeyPath:        filepath.Join(workSSHDir, "id_ephemeral.pub"),
		AuthorizedKeysPath:   filepath.Join(homeSSHDir, "authorized_keys"),
		AuthorizedKeysBackup: filepath.Join(homeSSHDir, "authorized_keys.backup.test"),
		isInstalled:          true,
	}
}

// authorizedKeysFixture returns a plausible authorized_keys body. It is made
// large on purpose: the truncate-then-write window in a non-atomic restore is
// proportional to the file size, and a realistic 200-byte file would make the
// torn-write tests below miss the bug most of the time.
func authorizedKeysFixture(lines int) []byte {
	var b bytes.Buffer
	for i := 0; i < lines; i++ {
		fmt.Fprintf(&b, "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI%040d operator-%d@example.com\n", i, i)
	}
	return b.Bytes()
}

// seedInstalledState writes the on-disk state left by a successful
// installPublicKey: a backup of the user's keys, and a live authorized_keys with
// the ephemeral key appended. It returns both bodies.
func seedInstalledState(t *testing.T, m *EphemeralSSHManager) (backup, live []byte) {
	t.Helper()

	backup = authorizedKeysFixture(20000)
	live = append(append([]byte{}, backup...), []byte(testEphemeralKeyLine)...)

	if err := os.WriteFile(m.AuthorizedKeysBackup, backup, 0600); err != nil {
		t.Fatalf("seed backup: %v", err)
	}
	if err := os.WriteFile(m.AuthorizedKeysPath, live, 0600); err != nil {
		t.Fatalf("seed authorized_keys: %v", err)
	}
	return backup, live
}

// watchTornWrites polls path and records every observation that is not one of
// the complete states a concurrent reader is allowed to see. sshd reads
// authorized_keys at arbitrary times, so any other observation is a window in
// which an incoming connection is authenticated against a truncated key list.
//
// The returned func stops the watcher and returns the violations.
func watchTornWrites(path string, allowed [][]byte) func() []string {
	done := make(chan struct{})
	finished := make(chan struct{})
	var violations []string

	go func() {
		defer close(finished)
		for {
			select {
			case <-done:
				return
			default:
			}

			got, err := os.ReadFile(path)
			switch {
			case err != nil:
				if len(violations) < 10 {
					violations = append(violations, fmt.Sprintf("unreadable: %v", err))
				}
			case !containsExact(allowed, got):
				if len(violations) < 10 {
					violations = append(violations, fmt.Sprintf("observed %d bytes, which is neither complete state", len(got)))
				}
			}
			runtime.Gosched()
		}
	}()

	return func() []string {
		close(done)
		<-finished
		return violations
	}
}

func containsExact(candidates [][]byte, got []byte) bool {
	for _, want := range candidates {
		if bytes.Equal(got, want) {
			return true
		}
	}
	return false
}

// A reader must never see authorized_keys in a state other than "before
// cleanup" or "restored", because an interrupt landing inside that window
// leaves the truncated content on disk permanently.
func TestCleanupRestoresAuthorizedKeysAtomically(t *testing.T) {
	m := newTestManager(t)
	backup, live := seedInstalledState(t, m)

	stop := watchTornWrites(m.AuthorizedKeysPath, [][]byte{live, backup})
	err := m.Cleanup()
	violations := stop()

	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("authorized_keys was observable in an incomplete state during Cleanup(); first observation: %s", violations[0])
	}

	got, err := os.ReadFile(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("read authorized_keys: %v", err)
	}
	if !bytes.Equal(got, backup) {
		t.Errorf("authorized_keys after Cleanup() = %d bytes, want the %d-byte backup restored", len(got), len(backup))
	}
}

// removePublicKey previously passed -1, -1 to writeFileAtomically, discarding the
// original file's owner and leaving the restored file root-owned whenever bloom
// runs as root restoring a non-root user's authorized_keys. This harness can only
// run unprivileged (newTestManager targets the current user), so it can't
// reproduce a root-vs-target-user mismatch directly, but it exercises the
// getFileInfo-derived mode/uid/gid path end to end and catches a regression back
// to a hardcoded mode or a skipped stat call.
func TestCleanupPreservesFileModeAndOwner(t *testing.T) {
	m := newTestManager(t)
	backup, _ := seedInstalledState(t, m)

	const nonDefaultMode = os.FileMode(0644)
	if err := os.Chmod(m.AuthorizedKeysPath, nonDefaultMode); err != nil {
		t.Fatalf("chmod authorized_keys: %v", err)
	}

	wantUID, wantGID, _, err := getFileInfo(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("getFileInfo before Cleanup: %v", err)
	}

	if err := m.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	gotUID, gotGID, gotMode, err := getFileInfo(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("getFileInfo after Cleanup: %v", err)
	}
	if gotMode.Perm() != nonDefaultMode.Perm() {
		t.Errorf("authorized_keys mode after Cleanup() = %v, want %v", gotMode.Perm(), nonDefaultMode.Perm())
	}
	if gotUID != wantUID || gotGID != wantGID {
		t.Errorf("authorized_keys owner after Cleanup() = %d:%d, want %d:%d", gotUID, gotGID, wantUID, wantGID)
	}

	got, err := os.ReadFile(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("read authorized_keys: %v", err)
	}
	if !bytes.Equal(got, backup) {
		t.Errorf("authorized_keys after Cleanup() = %d bytes, want the %d-byte backup restored", len(got), len(backup))
	}
}

// Three separate paths call Cleanup on interrupt (the global signal handler, the
// host SSH signal handler, and the deferred cleanup in the executor), so
// concurrent invocation is the real-world case, not a synthetic one.
func TestConcurrentCleanupRestoresAuthorizedKeysExactlyOnce(t *testing.T) {
	m := newTestManager(t)
	backup, live := seedInstalledState(t, m)

	const callers = 3
	errs := make(chan error, callers)

	stop := watchTornWrites(m.AuthorizedKeysPath, [][]byte{live, backup})
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- m.Cleanup()
		}()
	}
	wg.Wait()
	violations := stop()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("concurrent Cleanup() error = %v", err)
		}
	}
	if len(violations) > 0 {
		t.Errorf("authorized_keys was observable in an incomplete state during concurrent Cleanup(); first observation: %s", violations[0])
	}

	got, err := os.ReadFile(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("read authorized_keys: %v", err)
	}
	if !bytes.Equal(got, backup) {
		t.Errorf("authorized_keys after concurrent Cleanup() = %d bytes, want the %d-byte backup restored", len(got), len(backup))
	}
}

// A failed restore currently returns nil, so bloom prints its "original
// authorized_keys restored!" success line over a file it did not restore.
func TestCleanupReportsUnrestorableBackup(t *testing.T) {
	m := newTestManager(t)
	_, live := seedInstalledState(t, m)

	if err := os.Remove(m.AuthorizedKeysBackup); err != nil {
		t.Fatalf("remove backup: %v", err)
	}

	if err := m.Cleanup(); err == nil {
		t.Error("Cleanup() error = nil after the backup went missing, want a non-nil error so the caller does not report success")
	}

	got, err := os.ReadFile(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("read authorized_keys: %v", err)
	}
	if !bytes.Equal(got, live) {
		t.Errorf("authorized_keys after a failed Cleanup() = %d bytes, want the %d-byte pre-cleanup content left intact", len(got), len(live))
	}
}
