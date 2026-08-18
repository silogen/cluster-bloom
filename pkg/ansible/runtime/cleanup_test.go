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

func TestBloomFstabParseRemediationForRancherMountWithClusterTag(t *testing.T) {
	content := `/dev/sdc /var/lib/rancher ext4 defaults 0 2 # managed by cluster-bloom`
	hints := bloomFstabParseRemediation(content)
	if len(hints) != 1 || !strings.Contains(hints[0], fstabRancherTag) {
		t.Fatalf("bloomFstabParseRemediation() = %#v, want rancher-disk tag guidance", hints)
	}
}

func TestInvalidBloomFstabErrorIncludesRemediation(t *testing.T) {
	content := `/dev/sdc /var/lib/rancher ext4 defaults 0 2 # managed by cluster-bloom`
	_, errs := parseBloomFstab(content)
	if len(errs) != 1 {
		t.Fatalf("parseBloomFstab() errors = %v, want one validation error", errs)
	}
	err := invalidBloomFstabError(content, errs)
	if !strings.Contains(err.Error(), "Remediation:") || !strings.Contains(err.Error(), fstabRancherTag) {
		t.Fatalf("invalidBloomFstabError() = %v, want remediation with rancher tag", err)
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

func TestFindActiveFstabMountIgnoresCommentedEntries(t *testing.T) {
	input := `# /dev/sdb /var/lib/rancher ext4 defaults 0 2
/dev/sdc /var/lib/rancher ext4 defaults 0 2`
	entry, found, err := findActiveFstabMount(input, "/var/lib/rancher")
	if err != nil || !found || entry.source != "/dev/sdc" || entry.tag != bloomFstabNone {
		t.Fatalf("findActiveFstabMount() = %#v, %v, %v; want active untagged /dev/sdc", entry, found, err)
	}
}

func TestFindActiveFstabMountReturnsNoneForCommentOnly(t *testing.T) {
	input := `# /dev/sdb /var/lib/rancher ext4 defaults 0 2`
	if entry, found, err := findActiveFstabMount(input, "/var/lib/rancher"); err != nil || found {
		t.Fatalf("findActiveFstabMount() = %#v, %v, %v; want no active entry", entry, found, err)
	}
}

func TestRancherMisconfigHintsDetectsClusterDiskMappedToRancher(t *testing.T) {
	ctx := rancherStorageContext{
		liveSource:   "/dev/sdc",
		activeSource: "/dev/sdc",
	}
	hints := rancherMisconfigHints([]string{"/dev/sdc"}, "", ctx)
	if len(hints) != 1 || !strings.Contains(hints[0], "set RANCHER_DISK") {
		t.Fatalf("rancherMisconfigHints() = %#v, want RANCHER_DISK remediation", hints)
	}
}

func TestRancherMisconfigHintsMatchesAbsentDeviceReferences(t *testing.T) {
	const absentDevice = "/dev/sdz"
	ctx := rancherStorageContext{activeSource: absentDevice}
	hints := rancherMisconfigHints([]string{absentDevice}, "", ctx)
	if len(hints) != 1 || !strings.Contains(hints[0], "set RANCHER_DISK") {
		t.Fatalf("rancherMisconfigHints() = %#v, want RANCHER_DISK remediation for absent device", hints)
	}
}

func TestRancherMisconfigHintsDetectsStaleRancherMapping(t *testing.T) {
	ctx := rancherStorageContext{liveSource: "/dev/sdc"}
	hints := rancherMisconfigHints([]string{"/dev/sdc"}, "/dev/sdb", ctx)
	if len(hints) != 1 || !strings.Contains(hints[0], "RANCHER_DISK is /dev/sdb") {
		t.Fatalf("rancherMisconfigHints() = %#v, want stale mapping remediation", hints)
	}
}

func TestClusterDisksMissingFstabHintsPreferRancherRemediation(t *testing.T) {
	hints := clusterDisksMissingFstabHints(
		[]string{"/dev/sdc"},
		"",
		"/dev/sdc /var/lib/rancher ext4 defaults 0 2",
		"",
	)
	if len(hints) != 1 || !strings.Contains(hints[0], "set RANCHER_DISK") {
		t.Fatalf("clusterDisksMissingFstabHints() = %#v, want rancher-specific hint", hints)
	}
}

func TestClusterDisksMissingFstabHintsFallbackToGenericGuidance(t *testing.T) {
	hints := clusterDisksMissingFstabHints(
		[]string{"/dev/sdd"},
		"",
		"",
		"",
	)
	if len(hints) != 2 || !strings.Contains(hints[0], "/mnt/diskN") {
		t.Fatalf("clusterDisksMissingFstabHints() = %#v, want generic guidance", hints)
	}
}

func TestFormatCleanupRemediationIncludesBullets(t *testing.T) {
	got := formatCleanupRemediation([]string{"first hint", "second hint"})
	if !strings.Contains(got, "Remediation:") || !strings.Contains(got, "• first hint") {
		t.Fatalf("formatCleanupRemediation() = %q", got)
	}
}

func TestFormatDeviceProtectionReasonListsExactMountPoints(t *testing.T) {
	reason := deviceProtection{
		mountPoints: map[string]struct{}{
			"/":         {},
			"/boot":     {},
			"/boot/efi": {},
		},
	}.formatReason()
	if reason != "system mounts /, /boot, /boot/efi" {
		t.Fatalf("formatReason() = %q", reason)
	}
}

func TestRejectProtectedCleanupTargetRejectsRootDeviceChain(t *testing.T) {
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
		err := rejectProtectedCleanupTarget(dependency, "CLUSTER_DISKS")
		if err == nil {
			t.Errorf("rejectProtectedCleanupTarget(%q) allowed a device in the root dependency chain", dependency)
			continue
		}
		if !strings.Contains(err.Error(), "system mounts") || !strings.Contains(err.Error(), "/") {
			t.Errorf("rejectProtectedCleanupTarget(%q) = %v, want root protection error", dependency, err)
		}
		if !strings.Contains(err.Error(), "Remediation:") {
			t.Errorf("rejectProtectedCleanupTarget(%q) = %v, want remediation hints", dependency, err)
		}
	}
}

func TestResolveCleanupStorageDoesNotAutoDiscoverForExplicitEmptyConfig(t *testing.T) {
	storage, err := ResolveCleanupStorage("", "", "", true, false)
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

func TestHasStableDeviceIdentity(t *testing.T) {
	for _, reference := range []string{
		"/dev/disk/by-id/wwn-0x5000c500a1b2c3d4",
		"/dev/disk/by-uuid/7c4a8d09-1234-4f2e-9abc-1234567890ab",
		"UUID=7c4a8d09-1234-4f2e-9abc-1234567890ab",
		"uuid=7c4a8d09",
		"LABEL=data",
		"PARTUUID=0001-0002",
		" UUID=7c4a8d09 ",
		"/dev/mapper/vg--data-longhorn",
	} {
		if !hasStableDeviceIdentity(reference) {
			t.Errorf("hasStableDeviceIdentity(%q) = false, want true", reference)
		}
	}
	for _, reference := range []string{
		"", "   ", "/dev/sdb", "/dev/sdb1", "/dev/nvme0n1", "/dev/vda", "/dev/md0",
		"UUID=", "sdb", "no-equals",
	} {
		if hasStableDeviceIdentity(reference) {
			t.Errorf("hasStableDeviceIdentity(%q) = true, want false", reference)
		}
	}
}

func TestUnverifiableIdentityErrorNamesTargetAndRemediation(t *testing.T) {
	err := unverifiableIdentityError("RANCHER_DISK", "/dev/sdb", []string{"/dev/sdb", "/dev/sdb", ""})
	if err == nil {
		t.Fatal("unverifiableIdentityError() = nil, want error")
	}
	got := err.Error()
	for _, want := range []string{"RANCHER_DISK", "/dev/sdb", "/dev/disk/by-id/", "UUID="} {
		if !strings.Contains(got, want) {
			t.Errorf("unverifiableIdentityError() = %q, want it to mention %q", got, want)
		}
	}
	if strings.Count(got, "/dev/sdb,") != 0 {
		t.Errorf("unverifiableIdentityError() = %q, want duplicate references collapsed", got)
	}
}

func TestDeployDeviceReferenceKeepsStableOperatorPaths(t *testing.T) {
	storage := CleanupStorage{
		ClusterDisks:           "/dev/sdb,/dev/sdc",
		ClusterDisksConfigured: "/dev/disk/by-id/wwn-0xaaa,/dev/sdc",
		RancherDisk:            "/dev/sdd",
		RancherDiskConfigured:  "UUID=7c4a8d09",
	}
	// The by-id path survives; the kernel name has nothing better to fall back to.
	if got, want := storage.DeployClusterDisks(), "/dev/disk/by-id/wwn-0xaaa,/dev/sdc"; got != want {
		t.Errorf("DeployClusterDisks() = %q, want %q", got, want)
	}
	// A tag form cannot be handed to wipefs, so the resolved path is used.
	if got, want := storage.DeployRancherDisk(), "/dev/sdd"; got != want {
		t.Errorf("DeployRancherDisk() = %q, want %q", got, want)
	}
}

func TestDeployClusterDisksFallsBackWhenListsDisagree(t *testing.T) {
	storage := CleanupStorage{
		ClusterDisks:           "/dev/sdb",
		ClusterDisksConfigured: "/dev/disk/by-id/wwn-0xaaa,/dev/disk/by-id/wwn-0xbbb",
	}
	if got, want := storage.DeployClusterDisks(), "/dev/sdb"; got != want {
		t.Errorf("DeployClusterDisks() = %q, want %q", got, want)
	}
}
