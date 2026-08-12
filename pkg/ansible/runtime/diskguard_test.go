//go:build linux

package runtime

import (
	"strings"
	"testing"
)

func TestAssertDeviceTreeUnmountedMessageIncludesAllMounts(t *testing.T) {
	mounts := map[string]string{
		"/dev/sda2": "/var/lib/data",
		"/dev/sda1": "/boot",
	}

	err := mountedDeviceTreeError("/dev/sda", mounts)
	if err == nil {
		t.Fatal("mountedDeviceTreeError() error = nil, want mounted-device error")
	}
	got := err.Error()
	for _, want := range []string{"/dev/sda1 at /boot", "/dev/sda2 at /var/lib/data"} {
		if !strings.Contains(got, want) {
			t.Fatalf("mountedDeviceTreeError() = %q, want %q", got, want)
		}
	}
	if strings.Index(got, "/dev/sda1") > strings.Index(got, "/dev/sda2") {
		t.Fatalf("mountedDeviceTreeError() = %q, want deterministic device order", got)
	}
}
