//go:build linux
// +build linux

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
)

var (
	exitFunc      = os.Exit
	preExitHookMu sync.Mutex
	preExitHook   func() error
	preExitOnce   sync.Once
)

// SetPreExitHook registers a function to run exactly once before this
// package's signal handler terminates the process. Intended for cleanup that
// must complete even when interrupted mid-run (e.g. restoring authorized_keys).
// A non-nil return value causes the process to exit with status 1 instead of
// the signal-derived exit code, matching the old exit-on-cleanup-failure behavior.
func SetPreExitHook(hook func() error) {
	preExitHookMu.Lock()
	defer preExitHookMu.Unlock()
	preExitHook = hook
}

// runPreExitHook runs the registered pre-exit hook exactly once and reports
// whether it failed.
func runPreExitHook() (failed bool) {
	preExitOnce.Do(func() {
		preExitHookMu.Lock()
		hook := preExitHook
		preExitHookMu.Unlock()
		if hook != nil {
			failed = hook() != nil
		}
	})
	return failed
}

// exitCodeAfterHook returns 1 if the pre-exit hook failed, else the
// signal-derived exit code.
func exitCodeAfterHook(hookFailed bool, code int) int {
	if hookFailed {
		return 1
	}
	return code
}

// CriticalSection tracks whether we're in a critical operation that shouldn't be interrupted
type CriticalSection struct {
	mu          sync.Mutex
	inCritical  bool
	description string
	signalChan  chan os.Signal
	pendingExit bool
	exitCode    int
	exiting     bool
}

var globalCriticalSection = &CriticalSection{
	signalChan: make(chan os.Signal, 1),
}

// InitSignalHandling sets up global signal handling for graceful shutdown
func InitSignalHandling() {
	signal.Notify(globalCriticalSection.signalChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)

	go func() {
		for sig := range globalCriticalSection.signalChan {
			handleSignal(sig)
		}
	}()
}

// handleSignal processes received signals
func handleSignal(sig os.Signal) {
	globalCriticalSection.mu.Lock()

	if globalCriticalSection.exiting {
		// Signal received while pre-exit cleanup is running - force exit
		fmt.Fprintf(os.Stderr, "\n🔥 FORCE EXIT - Interrupted during exit cleanup!\n")
		code := getSignalExitCode(sig)
		globalCriticalSection.mu.Unlock()
		exitFunc(code)
		return
	}

	if globalCriticalSection.inCritical {
		// We're in a critical section - mark for exit but don't exit yet
		if !globalCriticalSection.pendingExit {
			fmt.Fprintf(os.Stderr, "\n⚠️  Interrupt received during critical operation: %s\n", globalCriticalSection.description)
			fmt.Fprintf(os.Stderr, "⏳ Waiting for operation to complete safely... (this may take a moment)\n")
			fmt.Fprintf(os.Stderr, "💡 Press Ctrl+C again to force exit (may leave system in inconsistent state)\n")
			globalCriticalSection.pendingExit = true
			globalCriticalSection.exitCode = getSignalExitCode(sig)
			globalCriticalSection.mu.Unlock()
		} else {
			// Second signal - force exit
			fmt.Fprintf(os.Stderr, "\n🔥 FORCE EXIT - System may be in inconsistent state!\n")
			code := getSignalExitCode(sig)
			globalCriticalSection.mu.Unlock()
			exitFunc(code)
		}
	} else {
		// Not in critical section - exit immediately
		fmt.Fprintf(os.Stderr, "\n✋ Interrupted - exiting...\n")
		code := getSignalExitCode(sig)
		globalCriticalSection.exiting = true
		globalCriticalSection.mu.Unlock()
		hookFailed := runPreExitHook()
		exitFunc(exitCodeAfterHook(hookFailed, code))
	}
}

// EnterCriticalSection marks the start of a critical operation
func EnterCriticalSection(description string) {
	globalCriticalSection.mu.Lock()
	defer globalCriticalSection.mu.Unlock()

	globalCriticalSection.inCritical = true
	globalCriticalSection.description = description
	globalCriticalSection.pendingExit = false
}

// ExitCriticalSection marks the end of a critical operation
// Returns true if we should exit (signal was received during critical section)
func ExitCriticalSection() bool {
	globalCriticalSection.mu.Lock()

	globalCriticalSection.inCritical = false
	globalCriticalSection.description = ""

	if globalCriticalSection.pendingExit {
		fmt.Fprintf(os.Stderr, "✅ Critical operation completed safely\n")
		fmt.Fprintf(os.Stderr, "👋 Exiting as requested...\n")
		code := globalCriticalSection.exitCode
		globalCriticalSection.exiting = true
		globalCriticalSection.mu.Unlock()
		hookFailed := runPreExitHook()
		exitFunc(exitCodeAfterHook(hookFailed, code))
		return true
	}

	globalCriticalSection.mu.Unlock()
	return false
}

// CheckPendingExit checks if there's a pending exit request
// This can be called periodically in long-running operations
func CheckPendingExit() bool {
	globalCriticalSection.mu.Lock()
	defer globalCriticalSection.mu.Unlock()
	return globalCriticalSection.pendingExit
}

// runProtectedCommand runs a destructive, slow command (e.g. wipefs, mkfs)
// in its own process group so a terminal-delivered interrupt (Ctrl-C, which
// the kernel sends to every process in the foreground process group) cannot
// kill it directly. Without this, EnterCriticalSection's deferred-exit
// guarantee only protects the Go process itself; the child process would
// still receive and act on the same signal, potentially leaving a wipe or
// format operation partially applied.
//
// A caller can still force-kill the whole run via a second Ctrl-C
// (see handleSignal); this only prevents a single, well-intentioned
// interrupt from silently corrupting an in-progress destructive operation.
func runProtectedCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANGUAGE=")
	return cmd.CombinedOutput()
}

// getSignalExitCode returns the appropriate exit code for a signal
func getSignalExitCode(sig os.Signal) int {
	switch sig {
	case os.Interrupt:
		return 130 // 128 + SIGINT
	case syscall.SIGTERM:
		return 143 // 128 + SIGTERM
	case syscall.SIGHUP:
		return 129 // 128 + SIGHUP
	case syscall.SIGQUIT:
		return 131 // 128 + SIGQUIT
	default:
		return 1
	}
}
