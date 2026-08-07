//go:build linux

package runtime

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestParseLonghornISCSISessions(t *testing.T) {
	input := `tcp: [1] 169.254.2.2:3260,1 iqn.2015-02.oracle.boot:uefi
tcp: [3] 10.43.11.22:3260,1 iqn.2019-10.io.longhorn:pvc-abc (non-flash)
tcp: [4] 10.43.11.23:3260,1 iqn.2019-10.io.longhorn:pvc-def
malformed
tcp: [5] 10.43.11.24:3260,1 iqn.2000-01.example:other`

	want := []iscsiSession{
		{sid: "3", portal: "10.43.11.22:3260", target: "iqn.2019-10.io.longhorn:pvc-abc"},
		{sid: "4", portal: "10.43.11.23:3260", target: "iqn.2019-10.io.longhorn:pvc-def"},
	}

	if got := parseLonghornISCSISessions(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLonghornISCSISessions() = %#v, want %#v", got, want)
	}
}

func TestParseLonghornISCSISessionsPreservesDuplicateTargetSessions(t *testing.T) {
	input := `tcp: [7] 10.43.11.22:3260,1 iqn.2019-10.io.longhorn:pvc-abc
tcp: [8] 10.43.11.22:3260,1 iqn.2019-10.io.longhorn:pvc-abc`

	got := parseLonghornISCSISessions(input)
	if len(got) != 2 {
		t.Fatalf("parseLonghornISCSISessions() returned %d sessions, want 2", len(got))
	}
	if got[0].sid != "7" || got[1].sid != "8" {
		t.Fatalf("session IDs = %q, %q; want 7, 8", got[0].sid, got[1].sid)
	}
}

func TestParseLonghornISCSISessionsNoMatches(t *testing.T) {
	input := `tcp: [1] 169.254.2.2:3260,1 iqn.2015-02.oracle.boot:uefi`
	if got := parseLonghornISCSISessions(input); len(got) != 0 {
		t.Fatalf("parseLonghornISCSISessions() = %#v, want no sessions", got)
	}
}

func TestParseLonghornISCSISessionsDoesNotDependOnTargetFieldPosition(t *testing.T) {
	input := `transport tcp: [9] [2001:db8::10]:3260,1 extra-field iqn.2019-10.io.longhorn:pvc-ipv6 (non-flash)`
	want := []iscsiSession{{
		sid:    "9",
		portal: "[2001:db8::10]:3260",
		target: "iqn.2019-10.io.longhorn:pvc-ipv6",
	}}
	if got := parseLonghornISCSISessions(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("parseLonghornISCSISessions() = %#v, want %#v", got, want)
	}
}

func TestParseBloomFstabUsesExactTagsAndMountShapes(t *testing.T) {
	input := `UUID=cluster /mnt/disk0 ext4 defaults 0 2 # managed by cluster-bloom
UUID=premounted /mnt/disk12 ext4 defaults 0 2 # premounted by cluster-bloom
UUID=rancher /var/lib/rancher ext4 defaults 0 2 # managed by cluster-bloom rancher-disk
/dev/sdz /data ext4 defaults 0 2 # old entry, managed by cluster-bloom team`

	entries, errs := parseBloomFstab(input)
	if len(errs) != 0 {
		t.Fatalf("parseBloomFstab() errors = %v, want none", errs)
	}
	if len(entries) != 3 {
		t.Fatalf("parseBloomFstab() returned %d entries, want 3", len(entries))
	}
	if entries[0].tag != bloomFstabManaged || entries[1].tag != bloomFstabPremounted ||
		entries[2].tag != bloomFstabRancher {
		t.Fatalf("parseBloomFstab() tags = %v, %v, %v", entries[0].tag, entries[1].tag, entries[2].tag)
	}
}

func TestParseBloomFstabRejectsTagOnCriticalMount(t *testing.T) {
	input := `/dev/sda1 / ext4 defaults 0 1 # managed by cluster-bloom`
	entries, errs := parseBloomFstab(input)
	if len(entries) != 0 || len(errs) != 1 {
		t.Fatalf("parseBloomFstab() entries = %#v, errors = %v; want one validation error", entries, errs)
	}
}

func TestParseBloomFstabRejectsIncompleteRecord(t *testing.T) {
	input := `/dev/sdb /mnt/disk0 # managed by cluster-bloom`
	entries, errs := parseBloomFstab(input)
	if len(entries) != 0 || len(errs) != 1 {
		t.Fatalf("parseBloomFstab() entries = %#v, errors = %v; want one validation error", entries, errs)
	}
}

func TestParseBloomFstabAllowsArbitrarySafePremountedPath(t *testing.T) {
	input := `UUID=premounted /data/storage-01 ext4 defaults 0 2 # premounted by cluster-bloom`
	entries, errs := parseBloomFstab(input)
	if len(errs) != 0 || len(entries) != 1 || entries[0].tag != bloomFstabPremounted {
		t.Fatalf("parseBloomFstab() entries = %#v, errors = %v; want one premounted entry", entries, errs)
	}
}

func TestParseBloomFstabRejectsPremountedCriticalPath(t *testing.T) {
	input := `UUID=premounted /var ext4 defaults 0 2 # premounted by cluster-bloom`
	entries, errs := parseBloomFstab(input)
	if len(entries) != 0 || len(errs) != 1 {
		t.Fatalf("parseBloomFstab() entries = %#v, errors = %v; want one validation error", entries, errs)
	}
}

func TestParseBloomFstabRejectsExtraFields(t *testing.T) {
	input := `UUID=cluster /mnt/disk0 ext4 defaults 0 2 unexpected # managed by cluster-bloom`
	entries, errs := parseBloomFstab(input)
	if len(entries) != 0 || len(errs) != 1 {
		t.Fatalf("parseBloomFstab() entries = %#v, errors = %v; want one validation error", entries, errs)
	}
}

func TestParseBloomFstabRejectsDuplicateMountPoint(t *testing.T) {
	input := `UUID=first /mnt/disk0 ext4 defaults 0 2 # managed by cluster-bloom
UUID=second /mnt/disk0 ext4 defaults 0 2 # managed by cluster-bloom`
	entries, errs := parseBloomFstab(input)
	if len(entries) != 1 || len(errs) != 1 {
		t.Fatalf("parseBloomFstab() entries = %#v, errors = %v; want one entry and one duplicate error", entries, errs)
	}
}

func TestResolveCleanupStorageDoesNotAutoDiscoverForExplicitEmptyConfig(t *testing.T) {
	storage, err := ResolveCleanupStorage("", "", "", true)
	if err != nil {
		t.Fatalf("ResolveCleanupStorage() error = %v", err)
	}
	if !storage.ConfigWasPresent || storage.ClusterDisks != "" ||
		storage.PremountedDisks != "" || storage.RancherDisk != "" {
		t.Fatalf("ResolveCleanupStorage() = %#v, want explicit empty storage", storage)
	}
}

func TestAssertSafeToWipeRejectsRootDeviceChain(t *testing.T) {
	out, err := exec.Command("findmnt", "--noheadings", "--output", "SOURCE", "--target", "/").Output()
	if err != nil {
		t.Skipf("cannot determine root source: %v", err)
	}
	rootSource := strings.TrimSpace(string(out))
	if !strings.HasPrefix(rootSource, "/dev/") {
		t.Skipf("root source %q is not a block device", rootSource)
	}
	dependencies, err := blockDeviceDependencies(rootSource)
	if err != nil {
		t.Fatalf("blockDeviceDependencies(%q): %v", rootSource, err)
	}
	for _, dependency := range dependencies {
		if err := assertSafeToWipe(dependency); err == nil {
			t.Errorf("assertSafeToWipe(%q) allowed a device in the root dependency chain", dependency)
		}
	}
}

func TestParseBloomDiskMountPointBounds(t *testing.T) {
	for _, mountPoint := range []string{"/mnt/disk0", "/mnt/disk4095"} {
		if _, ok := parseBloomDiskMountPoint(mountPoint); !ok {
			t.Errorf("parseBloomDiskMountPoint(%q) rejected valid mount", mountPoint)
		}
	}
	for _, mountPoint := range []string{
		"/", "/mnt/disk", "/mnt/disk-1", "/mnt/disk4096", "/mnt/disk1/child", "/mnt/disk01x",
	} {
		if _, ok := parseBloomDiskMountPoint(mountPoint); ok {
			t.Errorf("parseBloomDiskMountPoint(%q) accepted invalid mount", mountPoint)
		}
	}
}
