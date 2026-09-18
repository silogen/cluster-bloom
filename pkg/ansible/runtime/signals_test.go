//go:build linux
// +build linux

package runtime

import (
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
)

func resetSignalState(t *testing.T) {
	t.Helper()
	preExitHookMu.Lock()
	preExitHook = nil
	preExitOnce = sync.Once{}
	preExitHookMu.Unlock()

	globalCriticalSection.mu.Lock()
	globalCriticalSection.inCritical = false
	globalCriticalSection.pendingExit = false
	globalCriticalSection.description = ""
	globalCriticalSection.exitCode = 0
	globalCriticalSection.exiting = false
	globalCriticalSection.mu.Unlock()

	exitFunc = os.Exit
}

func TestRunProtectedCommandReturnsOutput(t *testing.T) {
	out, err := runProtectedCommand("echo", "hello-from-protected-command")
	if err != nil {
		t.Fatalf("runProtectedCommand() error = %v", err)
	}
	if !strings.Contains(string(out), "hello-from-protected-command") {
		t.Fatalf("runProtectedCommand() output = %q, want it to contain expected text", out)
	}
}

func TestRunProtectedCommandSurfacesFailureOutput(t *testing.T) {
	_, err := runProtectedCommand("false")
	if err == nil {
		t.Fatal("runProtectedCommand() error = nil, want non-nil for a failing command")
	}
}

func TestPreExitHookRunsBeforeExitOnSignal(t *testing.T) {
	resetSignalState(t)
	defer resetSignalState(t)

	var (
		hookRan  bool
		order    []string
		exited   bool
		exitCode int
		testMu   sync.Mutex
	)

	SetPreExitHook(func() error {
		testMu.Lock()
		defer testMu.Unlock()
		hookRan = true
		order = append(order, "hook")
		return nil
	})

	exitFunc = func(code int) {
		testMu.Lock()
		defer testMu.Unlock()
		exited = true
		exitCode = code
		order = append(order, "exit")
	}

	handleSignal(os.Interrupt)

	testMu.Lock()
	defer testMu.Unlock()

	if !hookRan {
		t.Fatal("pre-exit hook did not run")
	}
	if !exited {
		t.Fatal("exitFunc was not called")
	}
	if exitCode != 130 {
		t.Fatalf("exitCode = %d, want 130", exitCode)
	}
	if len(order) != 2 || order[0] != "hook" || order[1] != "exit" {
		t.Fatalf("order = %v, want [hook, exit]", order)
	}
}

func TestPreExitHookRunsOnlyOnce(t *testing.T) {
	resetSignalState(t)
	defer resetSignalState(t)

	var count int32
	SetPreExitHook(func() error {
		atomic.AddInt32(&count, 1)
		return nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runPreExitHook()
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("hook ran %d times, want exactly 1", got)
	}
}

func TestPreExitHookRunsOnExitCriticalSection(t *testing.T) {
	resetSignalState(t)
	defer resetSignalState(t)

	var (
		hookRan  bool
		exited   bool
		exitCode int
	)

	SetPreExitHook(func() error {
		hookRan = true
		return nil
	})

	exitFunc = func(code int) {
		exited = true
		exitCode = code
	}

	EnterCriticalSection("test-critical-op")
	handleSignal(syscall.SIGTERM)

	if hookRan {
		t.Fatal("hook ran while still inside critical section")
	}
	if exited {
		t.Fatal("exited while still inside critical section")
	}

	didExit := ExitCriticalSection()
	if !didExit {
		t.Fatal("ExitCriticalSection returned false, want true when pending exit")
	}
	if !hookRan {
		t.Fatal("hook did not run after ExitCriticalSection")
	}
	if !exited || exitCode != 143 {
		t.Fatalf("exitFunc called = %v, exitCode = %d, want true, 143", exited, exitCode)
	}
}

func TestFailedPreExitHookForcesExitCodeOne(t *testing.T) {
	resetSignalState(t)
	defer resetSignalState(t)

	SetPreExitHook(func() error {
		return errors.New("cleanup failed")
	})

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	handleSignal(os.Interrupt)

	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1 when pre-exit hook fails", exitCode)
	}
}

func TestSuccessfulPreExitHookPreservesSignalExitCode(t *testing.T) {
	resetSignalState(t)
	defer resetSignalState(t)

	SetPreExitHook(func() error {
		return nil
	})

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	handleSignal(os.Interrupt)

	if exitCode != 130 {
		t.Fatalf("exitCode = %d, want 130 when pre-exit hook succeeds", exitCode)
	}
}

func TestNilPreExitHookIsSafeNoOp(t *testing.T) {
	resetSignalState(t)
	defer resetSignalState(t)

	runPreExitHook()
}

func TestForceExitDoesNotRunHook(t *testing.T) {
	resetSignalState(t)
	defer resetSignalState(t)

	var hookRan bool
	SetPreExitHook(func() error {
		hookRan = true
		return nil
	})

	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	EnterCriticalSection("test-op")
	handleSignal(os.Interrupt)
	handleSignal(os.Interrupt)

	if hookRan {
		t.Fatal("force exit should not run pre-exit hook")
	}
	if exitCode != 130 {
		t.Fatalf("exitCode = %d, want 130", exitCode)
	}
}

func TestSecondSignalDuringExitCleanupForcesImmediateExit(t *testing.T) {
	resetSignalState(t)
	defer resetSignalState(t)

	hookStarted := make(chan struct{})
	hookBlock := make(chan struct{})

	SetPreExitHook(func() error {
		close(hookStarted)
		<-hookBlock
		return nil
	})

	var (
		exitCodes []int
		testMu    sync.Mutex
		wg        sync.WaitGroup
	)
	exitFunc = func(code int) {
		testMu.Lock()
		defer testMu.Unlock()
		exitCodes = append(exitCodes, code)
	}

	// First signal triggers non-critical exit and runs hook
	wg.Add(1)
	go func() {
		defer wg.Done()
		handleSignal(os.Interrupt)
	}()

	<-hookStarted

	// Second signal lands while hook is blocked
	handleSignal(syscall.SIGTERM)

	testMu.Lock()
	count := len(exitCodes)
	var secondExitCode int
	if count > 0 {
		secondExitCode = exitCodes[0]
	}
	testMu.Unlock()

	// Release hook to clean up goroutine
	close(hookBlock)
	wg.Wait()

	if count == 0 {
		t.Fatal("second signal during exit cleanup did not force immediate exit")
	}
	if secondExitCode != 143 {
		t.Fatalf("force exit code = %d, want 143", secondExitCode)
	}
}
