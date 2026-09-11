//go:build linux

package ssh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
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

// authorizedKeysFixture returns a plausible authorized_keys body.
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

	backup = authorizedKeysFixture(50)
	live = append(append([]byte{}, backup...), []byte(testEphemeralKeyLine)...)

	if err := os.WriteFile(m.AuthorizedKeysBackup, backup, 0600); err != nil {
		t.Fatalf("seed backup: %v", err)
	}
	if err := os.WriteFile(m.AuthorizedKeysPath, live, 0600); err != nil {
		t.Fatalf("seed authorized_keys: %v", err)
	}
	return backup, live
}

// A reader must never see authorized_keys in a state other than "before
// cleanup" or "restored", because an interrupt landing inside that window
// leaves the truncated content on disk permanently.
//
// An in-place restore truncates the existing file, so a descriptor opened
// before Cleanup would read back the truncated content. Holding one across the
// call proves atomicity without depending on how long the write happens to
// take.
func TestCleanupRestoresAuthorizedKeysAtomically(t *testing.T) {
	m := newTestManager(t)
	backup, live := seedInstalledState(t, m)

	reader, err := os.Open(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("open authorized_keys before Cleanup: %v", err)
	}
	defer reader.Close()

	if err := m.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	seen, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read from the descriptor opened before Cleanup: %v", err)
	}
	if !bytes.Equal(seen, live) {
		t.Errorf("a reader that opened authorized_keys before Cleanup() saw %d bytes, want the complete %d-byte pre-cleanup content", len(seen), len(live))
	}

	got, err := os.ReadFile(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("read authorized_keys: %v", err)
	}
	if !bytes.Equal(got, backup) {
		t.Errorf("authorized_keys after Cleanup() = %d bytes, want the %d-byte backup restored", len(got), len(backup))
	}
}

// The corollary of the descriptor check: the restored path must be a different
// inode, since only a rename can swap the content without the old one ever
// being incomplete.
func TestCleanupReplacesAuthorizedKeysInodeRatherThanWritingInPlace(t *testing.T) {
	m := newTestManager(t)
	seedInstalledState(t, m)

	before, err := os.Stat(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("stat authorized_keys before Cleanup: %v", err)
	}

	if err := m.Cleanup(); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	after, err := os.Stat(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("stat authorized_keys after Cleanup: %v", err)
	}
	if os.SameFile(before, after) {
		t.Error("Cleanup() modified authorized_keys in place; want it replaced by a rename so the file is never observably truncated")
	}
}

// Going through Cleanup() would mask a regression here: runAsUser's post-hoc
// setSSHFileOwnership chowns authorized_keys to the target user regardless of
// what the restore itself did, so asserting on Cleanup()'s end state can't
// tell a correct restore from one that ignored the live file's owner
// entirely. Call restoreAuthorizedKeysFromBackup directly so the assertion
// reflects what it told fileutil.WriteAtomicallyOwned to do, and catches a
// regression back to a hardcoded mode or a skipped stat call.
func TestRestorePreservesLiveFileModeAndOwner(t *testing.T) {
	m := newTestManager(t)
	backup, _ := seedInstalledState(t, m)

	const nonDefaultMode = os.FileMode(0644)
	if err := os.Chmod(m.AuthorizedKeysPath, nonDefaultMode); err != nil {
		t.Fatalf("chmod authorized_keys: %v", err)
	}

	wantUID, wantGID, _, err := getFileInfo(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("getFileInfo before restore: %v", err)
	}

	if err := m.restoreAuthorizedKeysFromBackup(); err != nil {
		t.Fatalf("restoreAuthorizedKeysFromBackup() error = %v", err)
	}

	gotUID, gotGID, gotMode, err := getFileInfo(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("getFileInfo after restore: %v", err)
	}
	if gotMode.Perm() != nonDefaultMode.Perm() {
		t.Errorf("authorized_keys mode after restore = %v, want %v", gotMode.Perm(), nonDefaultMode.Perm())
	}
	if gotUID != wantUID || gotGID != wantGID {
		t.Errorf("authorized_keys owner after restore = %d:%d, want %d:%d", gotUID, gotGID, wantUID, wantGID)
	}

	got, err := os.ReadFile(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("read authorized_keys: %v", err)
	}
	if !bytes.Equal(got, backup) {
		t.Errorf("authorized_keys after restore = %d bytes, want the %d-byte backup restored", len(got), len(backup))
	}
}

// When the live file is gone entirely, getFileInfo can't supply a mode/owner
// to preserve. The fallback must still target the intended user rather than
// -1,-1: skipping the chown would publish a process-owned file (root, under
// sudo) via the rename, and a signal landing before the later chown fixup
// runs would leave it that way — this harness can only run unprivileged, so
// it can't reproduce the root-vs-target-user mismatch directly, but it does
// exercise the getUserInfo fallback path end to end.
func TestRestoreFallsBackToTargetUserWhenLiveFileIsGone(t *testing.T) {
	m := newTestManager(t)
	backup, _ := seedInstalledState(t, m)

	if err := os.Remove(m.AuthorizedKeysPath); err != nil {
		t.Fatalf("remove authorized_keys: %v", err)
	}

	wantUID, wantGID, err := getUserInfo(m.Username)
	if err != nil {
		t.Fatalf("getUserInfo: %v", err)
	}

	if err := m.restoreAuthorizedKeysFromBackup(); err != nil {
		t.Fatalf("restoreAuthorizedKeysFromBackup() error = %v", err)
	}

	gotUID, gotGID, gotMode, err := getFileInfo(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("getFileInfo after restore: %v", err)
	}
	if gotMode.Perm() != os.FileMode(0600) {
		t.Errorf("authorized_keys mode after restore = %v, want 0600 fallback", gotMode.Perm())
	}
	if gotUID != int(wantUID) || gotGID != int(wantGID) {
		t.Errorf("authorized_keys owner after restore = %d:%d, want target user %d:%d", gotUID, gotGID, wantUID, wantGID)
	}

	got, err := os.ReadFile(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("read authorized_keys: %v", err)
	}
	if !bytes.Equal(got, backup) {
		t.Errorf("authorized_keys after restore = %d bytes, want the %d-byte backup restored", len(got), len(backup))
	}
}

// Two separate paths call Cleanup on interrupt (setupHostSSHSignalHandling's
// signal handler and the deferred cleanup in the executor), so concurrent
// invocation is the real-world case, not a synthetic one.
func TestConcurrentCleanupRestoresAuthorizedKeysExactlyOnce(t *testing.T) {
	m := newTestManager(t)
	backup, live := seedInstalledState(t, m)

	reader, err := os.Open(m.AuthorizedKeysPath)
	if err != nil {
		t.Fatalf("open authorized_keys before Cleanup: %v", err)
	}
	defer reader.Close()

	const callers = 3
	errs := make(chan error, callers)

	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- m.Cleanup()
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("concurrent Cleanup() error = %v", err)
		}
	}

	seen, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read from the descriptor opened before Cleanup: %v", err)
	}
	if !bytes.Equal(seen, live) {
		t.Errorf("a reader that opened authorized_keys before concurrent Cleanup() saw %d bytes, want the complete %d-byte pre-cleanup content", len(seen), len(live))
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
