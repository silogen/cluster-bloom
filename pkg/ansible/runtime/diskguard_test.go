//go:build linux

package runtime

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBlockSourcesForExactMountPointFallsBackWhenSourcesColumnIsUnsupported(t *testing.T) {
	installFakeFindmnt(t, `#!/bin/sh
case "$*" in
  *"--output SOURCES"*)
    printf '%s\n' 'findmnt: unknown column: SOURCES' >&2
    exit 1
    ;;
  *"--output SOURCE"*)
    printf '%s\n' '/dev/sda1'
    ;;
  *)
    exit 2
    ;;
esac
`)

	got, err := blockSourcesForExactMountPoint("/")
	if err != nil {
		t.Fatalf("blockSourcesForExactMountPoint() error = %v", err)
	}
	want := []string{"/dev/sda1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("blockSourcesForExactMountPoint() = %v, want %v", got, want)
	}
}

func TestBlockSourcesForExactMountPointFailsClosedForOtherFindmntErrors(t *testing.T) {
	installFakeFindmnt(t, `#!/bin/sh
case "$*" in
  *"--output SOURCES"*)
    printf '%s\n' 'findmnt: permission denied' >&2
    exit 1
    ;;
  *"--output SOURCE"*)
    printf '%s\n' '/dev/sda1'
    ;;
esac
`)

	_, err := blockSourcesForExactMountPoint("/")
	if err == nil {
		t.Fatal("blockSourcesForExactMountPoint() error = nil, want findmnt failure")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("blockSourcesForExactMountPoint() error = %v, want permission details", err)
	}
}

func installFakeFindmnt(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "findmnt")
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake findmnt: %v", err)
	}
	t.Setenv("PATH", dir)
}

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
