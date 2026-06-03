// +build linux

package runtime

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// CriticalSection tracks whether we're in a critical operation that shouldn't be interrupted
type CriticalSection struct {
	mu           sync.Mutex
	inCritical   bool
	description  string
	signalChan   chan os.Signal
	pendingExit  bool
	exitCode     int
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
	defer globalCriticalSection.mu.Unlock()

	if globalCriticalSection.inCritical {
		// We're in a critical section - mark for exit but don't exit yet
		if !globalCriticalSection.pendingExit {
			fmt.Fprintf(os.Stderr, "\n⚠️  Interrupt received during critical operation: %s\n", globalCriticalSection.description)
			fmt.Fprintf(os.Stderr, "⏳ Waiting for operation to complete safely... (this may take a moment)\n")
			fmt.Fprintf(os.Stderr, "💡 Press Ctrl+C again to force exit (may leave system in inconsistent state)\n")
			globalCriticalSection.pendingExit = true
			globalCriticalSection.exitCode = getSignalExitCode(sig)
		} else {
			// Second signal - force exit
			fmt.Fprintf(os.Stderr, "\n🔥 FORCE EXIT - System may be in inconsistent state!\n")
			os.Exit(getSignalExitCode(sig))
		}
	} else {
		// Not in critical section - exit immediately
		fmt.Fprintf(os.Stderr, "\n✋ Interrupted - exiting...\n")
		os.Exit(getSignalExitCode(sig))
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
	defer globalCriticalSection.mu.Unlock()

	globalCriticalSection.inCritical = false
	globalCriticalSection.description = ""

	if globalCriticalSection.pendingExit {
		fmt.Fprintf(os.Stderr, "✅ Critical operation completed safely\n")
		fmt.Fprintf(os.Stderr, "👋 Exiting as requested...\n")
		os.Exit(globalCriticalSection.exitCode)
		return true
	}

	return false
}

// CheckPendingExit checks if there's a pending exit request
// This can be called periodically in long-running operations
func CheckPendingExit() bool {
	globalCriticalSection.mu.Lock()
	defer globalCriticalSection.mu.Unlock()
	return globalCriticalSection.pendingExit
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
