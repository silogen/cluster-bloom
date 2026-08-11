// +build linux

package runtime

import (
	"strings"
	"testing"
)

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
