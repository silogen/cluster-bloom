//go:build linux

package runtime

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func installMountTable(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write mount table fixture: %v", err)
	}
	previous := mountTablePath
	mountTablePath = path
	t.Cleanup(func() { mountTablePath = previous })
}

func TestReadMountTableParsesOptionalFieldsAndEscapedPaths(t *testing.T) {
	installMountTable(t, `21 1 8:2 / / rw,relatime shared:1 - ext4 /dev/sda2 rw
22 21 8:1 / /boot rw,relatime - ext4 /dev/sda1 rw
23 21 0:24 / /mnt/my\040disk rw shared:3 master:2 - tmpfs tmpfs rw
`)

	table, err := readMountTable()
	if err != nil {
		t.Fatalf("readMountTable() error = %v", err)
	}
	if len(table) != 3 {
		t.Fatalf("readMountTable() returned %d entries, want 3", len(table))
	}
	if table[0].deviceID != blockDeviceID(unix.Mkdev(8, 2)) {
		t.Errorf("root deviceID = %d, want %d", table[0].deviceID, unix.Mkdev(8, 2))
	}
	if table[0].source != "/dev/sda2" {
		t.Errorf("root source = %q, want /dev/sda2", table[0].source)
	}
	// The second entry has no optional field, so the separator shifts left.
	if table[1].mountPoint != "/boot" || table[1].source != "/dev/sda1" {
		t.Errorf("boot entry = %+v, want /boot from /dev/sda1", table[1])
	}
	if table[2].mountPoint != "/mnt/my disk" {
		t.Errorf("escaped mount point = %q, want %q", table[2].mountPoint, "/mnt/my disk")
	}
}

func TestReadMountTableFailsClosedWhenUnreadable(t *testing.T) {
	previous := mountTablePath
	mountTablePath = filepath.Join(t.TempDir(), "absent")
	t.Cleanup(func() { mountTablePath = previous })

	if _, err := readMountTable(); err == nil {
		t.Fatal("readMountTable() error = nil, want failure for an unreadable table")
	}
}

func TestReadMountTableRejectsMalformedLines(t *testing.T) {
	installMountTable(t, "21 1 not-a-device / / rw - ext4 /dev/sda2 rw\n")

	if _, err := readMountTable(); err == nil {
		t.Fatal("readMountTable() error = nil, want failure for a malformed device number")
	}
}

func TestBlockSourcesForMountPointDistinguishesAbsentFromFailed(t *testing.T) {
	installMountTable(t, `21 1 8:2 / / rw,relatime shared:1 - ext4 /dev/sda2 rw
22 21 0:24 / /tmp rw shared:2 - tmpfs tmpfs rw
`)

	table, err := readMountTable()
	if err != nil {
		t.Fatalf("readMountTable() error = %v", err)
	}
	if got := table.blockSourcesForMountPoint("/"); !reflect.DeepEqual(got, []string{"/dev/sda2"}) {
		t.Errorf("blockSourcesForMountPoint(/) = %v, want [/dev/sda2]", got)
	}
	// /usr is not a separate filesystem here, which is normal and must not look
	// like a lookup failure.
	if got := table.blockSourcesForMountPoint("/usr"); len(got) != 0 {
		t.Errorf("blockSourcesForMountPoint(/usr) = %v, want empty", got)
	}
	// A tmpfs has no block-device source.
	if got := table.blockSourcesForMountPoint("/tmp"); len(got) != 0 {
		t.Errorf("blockSourcesForMountPoint(/tmp) = %v, want empty", got)
	}
}

func TestBlockSourcesContainingPathPicksLongestMatch(t *testing.T) {
	installMountTable(t, `21 1 8:2 / / rw,relatime shared:1 - ext4 /dev/sda2 rw
22 21 8:17 / /var rw,relatime shared:2 - ext4 /dev/sdb1 rw
23 21 8:33 / /var/lib rw,relatime shared:3 - ext4 /dev/sdc1 rw
`)

	table, err := readMountTable()
	if err != nil {
		t.Fatalf("readMountTable() error = %v", err)
	}
	got := table.blockSourcesContainingPath("/var/lib/swapfile")
	if !reflect.DeepEqual(got, []string{"/dev/sdc1"}) {
		t.Errorf("blockSourcesContainingPath() = %v, want [/dev/sdc1]", got)
	}
	if got := table.blockSourcesContainingPath("/swapfile"); !reflect.DeepEqual(got, []string{"/dev/sda2"}) {
		t.Errorf("blockSourcesContainingPath(/swapfile) = %v, want [/dev/sda2]", got)
	}
	// A path under /var but not /var/lib must not be attributed to /var/lib.
	if got := table.blockSourcesContainingPath("/var/libexec/swapfile"); !reflect.DeepEqual(got, []string{"/dev/sdb1"}) {
		t.Errorf("blockSourcesContainingPath(/var/libexec/...) = %v, want [/dev/sdb1]", got)
	}
}

func TestMountPointsForDeviceIDsReportsEveryMountOfADevice(t *testing.T) {
	installMountTable(t, `21 1 8:2 / / rw,relatime shared:1 - ext4 /dev/sda2 rw
22 21 8:16 / /mnt/disk0 rw,relatime shared:2 - ext4 /dev/sdb rw
23 21 8:16 /replicas /var/lib/longhorn rw,relatime shared:3 - ext4 /dev/sdb rw
24 21 0:24 / /tmp rw shared:4 - tmpfs tmpfs rw
`)

	table, err := readMountTable()
	if err != nil {
		t.Fatalf("readMountTable() error = %v", err)
	}
	mounts := table.mountPointsForDeviceIDs(map[blockDeviceID]string{
		blockDeviceID(unix.Mkdev(8, 16)): "/dev/sdb",
	})
	want := map[string][]string{"/dev/sdb": {"/mnt/disk0", "/var/lib/longhorn"}}
	if !reflect.DeepEqual(mounts, want) {
		t.Fatalf("mountPointsForDeviceIDs() = %v, want %v", mounts, want)
	}
}

func TestAssertDeviceTreeUnmountedMessageIncludesAllMounts(t *testing.T) {
	mounts := map[string][]string{
		"/dev/sda2": {"/var/lib/data"},
		"/dev/sda1": {"/boot"},
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
