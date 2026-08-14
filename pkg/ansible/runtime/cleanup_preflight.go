//go:build linux

package runtime

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type CleanupStorage struct {
	ClusterDisks     string
	PremountedDisks  string
	RancherDisk      string
	ConfigWasPresent bool
	RancherExplicit  bool

	// Spellings as supplied by the operator or /etc/fstab, before canonicalization
	// rewrote them to kernel names. Identity checks need these because a stable
	// reference is exactly the information canonicalization discards.
	ClusterDisksConfigured string
	RancherDiskConfigured  string
}

func splitCleanupValues(value string) []string {
	var values []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}

// ResolveCleanupStorage uses strict fstab discovery only when no config file
// was supplied. An explicitly provided config remains authoritative even when
// its storage values are empty and is cross-checked by RunCleanupPreflight.
func ResolveCleanupStorage(
	clusterDisks, premountedDisks, rancherDisk string,
	configWasProvided, rancherExplicit bool,
) (CleanupStorage, error) {
	storage := CleanupStorage{
		ClusterDisks:     clusterDisks,
		PremountedDisks:  premountedDisks,
		RancherDisk:      rancherDisk,
		ConfigWasPresent: configWasProvided,
		RancherExplicit:  rancherExplicit,
	}
	if storage.ConfigWasPresent {
		return canonicalizeCleanupStorage(storage)
	}

	_, entries, err := readBloomFstab()
	if err != nil {
		return CleanupStorage{}, err
	}
	var clusterSources []string
	var premountedMounts []string
	for _, entry := range entries {
		switch entry.tag {
		case bloomFstabManaged:
			if _, _, err := resolveBlockDevice(entry.source); err != nil {
				return CleanupStorage{}, staleFstabEntryError(entry, err)
			}
			clusterSources = append(clusterSources, entry.source)
		case bloomFstabPremounted:
			premountedMounts = append(premountedMounts, entry.mountPoint)
		case bloomFstabRancher:
			if _, _, err := resolveBlockDevice(entry.source); err != nil {
				return CleanupStorage{}, staleFstabEntryError(entry, err)
			}
			storage.RancherDisk = entry.source
		}
	}
	storage.ClusterDisks = strings.Join(clusterSources, ",")
	storage.PremountedDisks = strings.Join(premountedMounts, ",")
	return canonicalizeCleanupStorage(storage)
}

func canonicalizeCleanupStorage(storage CleanupStorage) (CleanupStorage, error) {
	storage.ClusterDisksConfigured = storage.ClusterDisks
	storage.RancherDiskConfigured = storage.RancherDisk

	var canonicalCluster []string
	for _, device := range splitCleanupValues(storage.ClusterDisks) {
		canonical, _, err := resolveBlockDevice(device)
		if err != nil {
			return CleanupStorage{}, fmt.Errorf("resolve CLUSTER_DISK %s: %w", device, err)
		}
		canonicalCluster = append(canonicalCluster, canonical)
	}
	storage.ClusterDisks = strings.Join(canonicalCluster, ",")
	if strings.TrimSpace(storage.RancherDisk) != "" {
		canonical, _, err := resolveBlockDevice(storage.RancherDisk)
		if err != nil {
			return CleanupStorage{}, fmt.Errorf("resolve RANCHER_DISK %s: %w", storage.RancherDisk, err)
		}
		storage.RancherDisk = canonical
	}
	return storage, nil
}

// hasStableDeviceIdentity reports whether reference names a disk in a way that
// survives detach and reattach. A bare kernel name such as /dev/sdb does not:
// the kernel reassigns it, so it cannot prove a device is still the one Bloom
// recorded. UUID=, LABEL=, PARTUUID=, /dev/disk/by-*, and device-mapper names can,
// the last because they come from on-disk LVM metadata rather than probe order.
// UUID= is what Bloom itself writes into /etc/fstab.
func hasStableDeviceIdentity(reference string) bool {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return false
	}
	if strings.HasPrefix(reference, "/dev/disk/by-") || strings.HasPrefix(reference, "/dev/mapper/") {
		return true
	}
	if strings.HasPrefix(reference, "/dev/") {
		return false
	}
	name, value, separated := strings.Cut(reference, "=")
	if !separated || strings.TrimSpace(value) == "" {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "UUID", "LABEL", "PARTUUID", "PARTLABEL", "ID":
		return true
	}
	return false
}

// unverifiableIdentityError refuses a wipe whose target is only ever named by a
// kernel device name. This is the failure this guard exists for: after a disk is
// detached and reattached, /dev/sdb can point at a different disk, and comparing
// one kernel name against another cannot detect it.
func unverifiableIdentityError(configKey, target string, references []string) error {
	seen := map[string]struct{}{}
	var known []string
	for _, reference := range references {
		reference = strings.TrimSpace(reference)
		if reference == "" {
			continue
		}
		if _, duplicate := seen[reference]; duplicate {
			continue
		}
		seen[reference] = struct{}{}
		known = append(known, reference)
	}
	return cleanupPreflightError(
		fmt.Sprintf(
			"refusing to wipe %s %s: every reference to this disk is a kernel device name (%s), "+
				"which the kernel reassigns when a disk is detached and reattached, so Bloom cannot "+
				"confirm it is still the disk that was recorded",
			configKey, target, strings.Join(known, ", ")),
		[]string{
			fmt.Sprintf("Point %s at a stable path instead, for example /dev/disk/by-id/... (run 'ls -l /dev/disk/by-id')", configKey),
			"Or give the /etc/fstab entry for this mount a UUID= source, which is what Bloom writes for storage it provisions",
		},
	)
}

// deployDeviceReference picks the spelling the deploy playbook should receive.
// The Ansible tasks hand these values straight to wipefs, blkid, and mkfs, so
// only real device paths survive the round trip: a /dev/disk/by-id path is kept
// because it stays correct across renumbering, while a tag form such as UUID=
// has to be passed already resolved.
func deployDeviceReference(configured, canonical string) string {
	configured = strings.TrimSpace(configured)
	if strings.HasPrefix(configured, "/dev/") {
		return configured
	}
	return canonical
}

// DeployClusterDisks returns CLUSTER_DISKS as the deploy playbook should see it,
// preserving the operator's stable device paths where they were supplied.
func (storage CleanupStorage) DeployClusterDisks() string {
	configured := splitCleanupValues(storage.ClusterDisksConfigured)
	canonical := splitCleanupValues(storage.ClusterDisks)
	if len(configured) != len(canonical) {
		return storage.ClusterDisks
	}
	references := make([]string, 0, len(canonical))
	for index := range canonical {
		references = append(references, deployDeviceReference(configured[index], canonical[index]))
	}
	return strings.Join(references, ",")
}

// DeployRancherDisk returns RANCHER_DISK as the deploy playbook should see it.
func (storage CleanupStorage) DeployRancherDisk() string {
	return deployDeviceReference(storage.RancherDiskConfigured, storage.RancherDisk)
}

func deviceIDs(devices []string) (map[blockDeviceID]string, error) {
	ids := map[blockDeviceID]string{}
	for _, device := range devices {
		canonical, id, err := resolveBlockDevice(device)
		if err != nil {
			return nil, err
		}
		if previous, duplicate := ids[id]; duplicate {
			return nil, fmt.Errorf("%s and %s resolve to the same block device %s", previous, device, canonical)
		}
		ids[id] = device
	}
	return ids, nil
}

func compareDeviceSets(configured, recorded []string) error {
	configuredIDs, err := deviceIDs(configured)
	if err != nil {
		return err
	}
	recordedIDs, err := deviceIDs(recorded)
	if err != nil {
		return err
	}
	var drift []string
	for id, configuredPath := range configuredIDs {
		if _, exists := recordedIDs[id]; !exists {
			drift = append(drift, fmt.Sprintf("config-only %s", configuredPath))
		}
	}
	for id, recordedPath := range recordedIDs {
		if _, exists := configuredIDs[id]; !exists {
			drift = append(drift, fmt.Sprintf("fstab-only %s", recordedPath))
		}
	}
	if len(drift) > 0 {
		sort.Strings(drift)
		return fmt.Errorf("bloom.yaml and fstab identify different CLUSTER_DISKS: %s", strings.Join(drift, ", "))
	}
	return nil
}

type rancherStorageContext struct {
	liveSource   string
	activeSource string
	taggedSource string
}

func rancherStorageContextFrom(fstabContent, fstabRancher string) (rancherStorageContext, error) {
	ctx := rancherStorageContext{taggedSource: fstabRancher}
	liveSource, err := exactMountSource("/var/lib/rancher")
	if err != nil {
		return rancherStorageContext{}, err
	}
	ctx.liveSource = liveSource
	if active, found, _ := findActiveFstabMount(fstabContent, "/var/lib/rancher"); found {
		ctx.activeSource = active.source
	}
	return ctx, nil
}

// sameDevice reports whether two sources name the same disk. Identical paths
// match without touching the host, so a detached or renamed device still
// resolves instead of failing the comparison.
func sameDevice(a, b string) bool {
	if filepath.Clean(strings.TrimSpace(a)) == filepath.Clean(strings.TrimSpace(b)) {
		return true
	}
	return compareDeviceSets([]string{a}, []string{b}) == nil
}

func deviceAuthorizedAsRancher(device string, ctx rancherStorageContext) bool {
	for _, source := range []string{ctx.liveSource, ctx.activeSource, ctx.taggedSource} {
		if source == "" {
			continue
		}
		if sameDevice(device, source) {
			return true
		}
	}
	return false
}

func deviceUsedForRancher(device, fstabContent, fstabRancher string) bool {
	ctx, err := rancherStorageContextFrom(fstabContent, fstabRancher)
	if err != nil {
		return false
	}
	if deviceAuthorizedAsRancher(device, ctx) {
		return true
	}
	mounts, err := blockDeviceTreeMounts(device)
	if err != nil {
		return false
	}
	for _, mountPoints := range mounts {
		for _, mountPoint := range mountPoints {
			if mountPoint == "/var/lib/rancher" {
				return true
			}
		}
	}
	return false
}

func rancherMisconfigHints(devices []string, configuredRancher string, ctx rancherStorageContext) []string {
	var hints []string
	for _, device := range devices {
		if !deviceAuthorizedAsRancher(device, ctx) {
			continue
		}
		switch {
		case strings.TrimSpace(configuredRancher) == "":
			hints = append(hints, fmt.Sprintf(
				"%s is the /var/lib/rancher disk; set RANCHER_DISK and remove it from CLUSTER_DISKS",
				device))
		case !sameDevice(device, configuredRancher):
			hints = append(hints, fmt.Sprintf(
				"%s is mounted at /var/lib/rancher but RANCHER_DISK is %s; fix the stale bloom.yaml mapping",
				device, configuredRancher))
		default:
			hints = append(hints, fmt.Sprintf(
				"%s belongs under RANCHER_DISK, not CLUSTER_DISKS", device))
		}
	}
	return hints
}

func clusterDisksMissingFstabHints(devices []string, configuredRancher, fstabContent, fstabRancher string) []string {
	ctx, err := rancherStorageContextFrom(fstabContent, fstabRancher)
	if err != nil {
		return []string{fmt.Sprintf("Could not read the live mount table: %v", err)}
	}
	hints := rancherMisconfigHints(devices, configuredRancher, ctx)
	for _, device := range devices {
		if deviceAuthorizedAsRancher(device, ctx) {
			continue
		}
		if deviceUsedForRancher(device, fstabContent, fstabRancher) {
			hints = append(hints, fmt.Sprintf(
				"%s is mounted at /var/lib/rancher; move it to RANCHER_DISK instead of CLUSTER_DISKS",
				device))
		}
	}
	if len(hints) > 0 {
		return hints
	}
	return []string{
		"CLUSTER_DISKS expects Bloom-managed /mnt/diskN fstab entries tagged '# managed by cluster-bloom', or '# managed by cluster-bloom rancher-disk' for RANCHER_DISK",
		"Verify bloom.yaml matches the cluster that created those tags, or run configless cleanup to auto-discover tagged storage",
	}
}

func formatCleanupRemediation(hints []string) string {
	if len(hints) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("\n\nRemediation:")
	for _, hint := range hints {
		builder.WriteString("\n  • ")
		builder.WriteString(hint)
	}
	return builder.String()
}

func cleanupPreflightError(message string, hints []string) error {
	return fmt.Errorf("%s%s", message, formatCleanupRemediation(hints))
}

func protectedDeviceRemediationHints(device, configKey string) []string {
	return []string{
		fmt.Sprintf("Remove %s from %s in bloom.yaml", device, configKey),
		"Bloom never wipes devices backing /, /boot, /boot/efi, swap, or other critical system mounts",
	}
}

func rejectProtectedCleanupTarget(device, configKey string) error {
	canonical, reason, err := protectedConflictForDevice(device)
	if err != nil {
		return err
	}
	if reason == "" {
		return nil
	}
	return cleanupPreflightError(
		fmt.Sprintf("refusing to wipe %s (%s): it backs %s", device, canonical, reason),
		protectedDeviceRemediationHints(device, configKey),
	)
}

func validateConfiguredCleanupTargets(clusterDevices []string, rancherDisk string) error {
	for _, device := range clusterDevices {
		if err := rejectProtectedCleanupTarget(device, "CLUSTER_DISKS"); err != nil {
			return err
		}
	}
	if strings.TrimSpace(rancherDisk) != "" {
		if err := rejectProtectedCleanupTarget(rancherDisk, "RANCHER_DISK"); err != nil {
			return err
		}
	}
	return nil
}

func rejectOverlappingCleanupTargets(targets map[string]string) error {
	var names []string
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)

	targetIDs := map[string]blockDeviceID{}
	dependencyIDs := map[string]map[blockDeviceID]string{}
	for _, name := range names {
		_, targetID, err := resolveBlockDevice(targets[name])
		if err != nil {
			return err
		}
		targetIDs[name] = targetID
		dependencyIDs[name] = map[blockDeviceID]string{}
		dependencies, err := blockDeviceDependencies(targets[name])
		if err != nil {
			return err
		}
		for _, dependency := range dependencies {
			_, id, err := resolveBlockDevice(dependency)
			if err != nil {
				return err
			}
			dependencyIDs[name][id] = dependency
		}
	}
	for i, name := range names {
		for _, other := range names[i+1:] {
			if dependency, overlap := dependencyIDs[name][targetIDs[other]]; overlap {
				return fmt.Errorf("cleanup storage target %s contains %s at %s", name, other, dependency)
			}
			if dependency, overlap := dependencyIDs[other][targetIDs[name]]; overlap {
				return fmt.Errorf("cleanup storage target %s contains %s at %s", other, name, dependency)
			}
		}
	}
	return nil
}

// exactMountSource returns the block device mounted exactly at mountPoint, or
// an empty string when nothing is mounted there. It reports a read failure as
// an error rather than as "not mounted", because callers use an empty result to
// skip drift checks that must not be skipped on uncertainty.
func exactMountSource(mountPoint string) (string, error) {
	table, err := readMountTable()
	if err != nil {
		return "", err
	}
	sources := table.blockSourcesForMountPoint(mountPoint)
	if len(sources) == 0 {
		return "", nil
	}
	// The last mount at a path is the one currently visible there.
	return sources[len(sources)-1], nil
}

func validateClusterDiskScope(clusterDisks string) error {
	configured := splitCleanupValues(clusterDisks)
	_, entries, err := readBloomFstab()
	if err != nil {
		return err
	}
	var recorded []string
	for _, entry := range entries {
		if entry.tag != bloomFstabManaged {
			continue
		}
		recorded = append(recorded, entry.source)
		liveSource, err := exactMountSource(entry.mountPoint)
		if err != nil {
			return err
		}
		if liveSource != "" {
			if err := compareDeviceSets([]string{entry.source}, []string{liveSource}); err != nil {
				return fmt.Errorf("fstab/live mount mismatch at %s: %w", entry.mountPoint, err)
			}
		}
	}
	if len(configured) == 0 && len(recorded) == 0 {
		return nil
	}
	if len(configured) == 0 || len(recorded) == 0 {
		return fmt.Errorf("validated CLUSTER_DISKS scope no longer matches strict Bloom fstab entries")
	}
	if err := compareDeviceSets(configured, recorded); err != nil {
		return err
	}
	for _, device := range configured {
		if err := rejectProtectedCleanupTarget(device, "CLUSTER_DISKS"); err != nil {
			return err
		}
	}
	return nil
}

// RunCleanupPreflight validates every destructive target against fstab and the
// running host. It performs no mutations and fails closed on uncertainty.
func RunCleanupPreflight(storage CleanupStorage) error {
	fmt.Println("🔎 Running destructive cleanup preflight...")

	fstabContent, fstabEntries, err := readBloomFstab()
	if err != nil {
		return err
	}
	var fstabCluster []string
	var fstabPremounted []string
	var fstabRancher string
	for _, entry := range fstabEntries {
		switch entry.tag {
		case bloomFstabManaged:
			fstabCluster = append(fstabCluster, entry.source)
		case bloomFstabPremounted:
			fstabPremounted = append(fstabPremounted, entry.mountPoint)
		case bloomFstabRancher:
			if fstabRancher != "" {
				return fmt.Errorf("multiple Bloom-managed RANCHER_DISK entries in fstab")
			}
			fstabRancher = entry.source
		}
		liveSource, err := exactMountSource(entry.mountPoint)
		if err != nil {
			return err
		}
		if liveSource != "" {
			if err := compareDeviceSets([]string{entry.source}, []string{liveSource}); err != nil {
				return fmt.Errorf("fstab/live mount mismatch at %s: %w", entry.mountPoint, err)
			}
		}
	}

	clusterDevices := splitCleanupValues(storage.ClusterDisks)
	if err := validateConfiguredCleanupTargets(clusterDevices, storage.RancherDisk); err != nil {
		return err
	}
	destructiveTargets := map[string]string{}
	for index, device := range clusterDevices {
		destructiveTargets[fmt.Sprintf("CLUSTER_DISKS[%d]", index)] = device
	}
	if storage.RancherDisk != "" {
		destructiveTargets["RANCHER_DISK"] = storage.RancherDisk
	}
	for index, entry := range fstabEntries {
		if entry.tag != bloomFstabPremounted {
			continue
		}
		canonical, _, err := resolveBlockDevice(entry.source)
		if err != nil {
			return staleFstabEntryError(entry, err)
		}
		destructiveTargets[fmt.Sprintf("CLUSTER_PREMOUNTED_DISKS[%d]", index)] = canonical
	}
	if err := rejectOverlappingCleanupTargets(destructiveTargets); err != nil {
		return err
	}
	if storage.ConfigWasPresent && len(clusterDevices) > 0 && len(fstabCluster) > 0 {
		if err := compareDeviceSets(clusterDevices, fstabCluster); err != nil {
			ctx, ctxErr := rancherStorageContextFrom(fstabContent, fstabRancher)
			if ctxErr != nil {
				return ctxErr
			}
			hints := rancherMisconfigHints(clusterDevices, storage.RancherDisk, ctx)
			if len(hints) == 0 {
				hints = []string{
					"Update CLUSTER_DISKS in bloom.yaml to match the strictly tagged /mnt/diskN fstab entries",
				}
			}
			return fmt.Errorf("%w%s", err, formatCleanupRemediation(hints))
		}
	} else if storage.ConfigWasPresent && len(clusterDevices) > 0 && len(fstabCluster) == 0 {
		return cleanupPreflightError(
			"CLUSTER_DISKS are configured but no strict Bloom-managed fstab entries exist",
			clusterDisksMissingFstabHints(clusterDevices, storage.RancherDisk, fstabContent, fstabRancher),
		)
	} else if storage.ConfigWasPresent && len(clusterDevices) == 0 && len(fstabCluster) > 0 {
		return cleanupPreflightError(
			"fstab contains Bloom-managed disks but bloom.yaml CLUSTER_DISKS is empty",
			[]string{
				fmt.Sprintf("Add the fstab devices to CLUSTER_DISKS in bloom.yaml: %s", strings.Join(fstabCluster, ", ")),
				"Or run configless cleanup to auto-discover strictly tagged storage",
			},
		)
	}

	clusterIDs, err := deviceIDs(clusterDevices)
	if err != nil {
		return fmt.Errorf("validate CLUSTER_DISKS: %w", err)
	}
	configuredClusterIDs, err := deviceIDs(splitCleanupValues(storage.ClusterDisksConfigured))
	if err != nil {
		return fmt.Errorf("validate CLUSTER_DISKS: %w", err)
	}
	allowedClusterMounts := map[blockDeviceID]map[string]struct{}{}
	fstabClusterSources := map[blockDeviceID]string{}
	for _, entry := range fstabEntries {
		if entry.tag != bloomFstabManaged {
			continue
		}
		_, id, err := resolveBlockDevice(entry.source)
		if err != nil {
			return staleFstabEntryError(entry, err)
		}
		if allowedClusterMounts[id] == nil {
			allowedClusterMounts[id] = map[string]struct{}{}
		}
		allowedClusterMounts[id][entry.mountPoint] = struct{}{}
		fstabClusterSources[id] = entry.source
	}
	for id, device := range clusterIDs {
		if err := rejectProtectedCleanupTarget(device, "CLUSTER_DISKS"); err != nil {
			return err
		}
		if !hasStableDeviceIdentity(configuredClusterIDs[id]) &&
			!hasStableDeviceIdentity(fstabClusterSources[id]) {
			return unverifiableIdentityError("CLUSTER_DISKS", device,
				[]string{configuredClusterIDs[id], fstabClusterSources[id]})
		}
		mounts, err := blockDeviceTreeMounts(device)
		if err != nil {
			return err
		}
		for mountedDevice, mountPoints := range mounts {
			for _, mountPoint := range mountPoints {
				if _, allowed := allowedClusterMounts[id][mountPoint]; !allowed {
					message := fmt.Sprintf(
						"refusing to clean %s: %s is mounted at non-Bloom mount point %s",
						device, mountedDevice, mountPoint)
					var hints []string
					if mountPoint == "/var/lib/rancher" {
						ctx, ctxErr := rancherStorageContextFrom(fstabContent, fstabRancher)
						if ctxErr != nil {
							return ctxErr
						}
						hints = rancherMisconfigHints([]string{device}, storage.RancherDisk, ctx)
					}
					if len(hints) == 0 {
						hints = []string{
							"CLUSTER_DISKS only covers Bloom-managed /mnt/diskN mounts",
							"Move rancher storage to RANCHER_DISK or fix the stale bloom.yaml mapping",
						}
					}
					return cleanupPreflightError(message, hints)
				}
			}
		}
		canonical, _, _ := resolveBlockDevice(device)
		fmt.Printf("   ✓ CLUSTER_DISK %-24s -> %s (safe to wipe)\n", device, canonical)
		clusterIDs[id] = canonical
	}

	configuredPremounted := splitCleanupValues(storage.PremountedDisks)
	configuredPremountedSet := map[string]struct{}{}
	for _, mountPoint := range configuredPremounted {
		if !isSafePremountedMountPoint(mountPoint) {
			return fmt.Errorf("invalid CLUSTER_PREMOUNTED_DISKS mount point %q", mountPoint)
		}
		configuredPremountedSet[mountPoint] = struct{}{}
	}
	if storage.ConfigWasPresent && len(configuredPremounted) > 0 && len(fstabPremounted) > 0 {
		for _, mountPoint := range fstabPremounted {
			if _, exists := configuredPremountedSet[mountPoint]; !exists {
				return fmt.Errorf("fstab premounted disk %s is absent from CLUSTER_PREMOUNTED_DISKS", mountPoint)
			}
		}
		if len(configuredPremountedSet) != len(fstabPremounted) {
			return fmt.Errorf("CLUSTER_PREMOUNTED_DISKS and fstab contain different mount points")
		}
	} else if storage.ConfigWasPresent && len(configuredPremounted) > 0 && len(fstabPremounted) == 0 {
		return fmt.Errorf("CLUSTER_PREMOUNTED_DISKS are configured but no strict premounted Bloom fstab entries exist")
	} else if storage.ConfigWasPresent && len(configuredPremounted) == 0 && len(fstabPremounted) > 0 {
		return fmt.Errorf("fstab contains premounted Bloom disks but bloom.yaml CLUSTER_PREMOUNTED_DISKS is empty")
	}

	if storage.RancherDisk != "" {
		canonical, rancherID, err := resolveBlockDevice(storage.RancherDisk)
		if err != nil {
			return fmt.Errorf("validate RANCHER_DISK: %w", err)
		}
		if clusterPath, collision := clusterIDs[rancherID]; collision {
			return fmt.Errorf("RANCHER_DISK %s is also configured as CLUSTER_DISK %s", storage.RancherDisk, clusterPath)
		}
		recordedRancher := fstabRancher
		liveRancher, err := exactMountSource("/var/lib/rancher")
		if err != nil {
			return err
		}
		activeRancher, hasActiveRancher, err := findActiveFstabMount(fstabContent, "/var/lib/rancher")
		if err != nil {
			return err
		}
		activeRancherSource := ""
		if hasActiveRancher {
			activeRancherSource = activeRancher.source
		}
		if recordedRancher == "" && !storage.RancherExplicit {
			return fmt.Errorf("configless cleanup requires a strictly tagged Bloom RANCHER_DISK fstab entry")
		}
		if recordedRancher == "" && liveRancher == "" {
			return fmt.Errorf("explicit RANCHER_DISK has no matching live /var/lib/rancher mount or managed fstab entry")
		}
		if recordedRancher != "" {
			if err := compareDeviceSets([]string{storage.RancherDisk}, []string{recordedRancher}); err != nil {
				return fmt.Errorf("RANCHER_DISK config/fstab mismatch: %w", err)
			}
		}
		if hasActiveRancher {
			if err := compareDeviceSets([]string{storage.RancherDisk}, []string{activeRancherSource}); err != nil {
				return fmt.Errorf("RANCHER_DISK config/active fstab mismatch: %w", err)
			}
		}
		if liveRancher != "" {
			if err := compareDeviceSets([]string{storage.RancherDisk}, []string{liveRancher}); err != nil {
				return fmt.Errorf("RANCHER_DISK config/live mount mismatch: %w", err)
			}
		}
		// The comparisons above all resolve through the live kernel state, so they
		// agree by construction when every reference is a kernel name that was
		// reassigned together. At least one stable reference is required to make
		// them meaningful.
		if !hasStableDeviceIdentity(storage.RancherDiskConfigured) &&
			!hasStableDeviceIdentity(recordedRancher) &&
			!hasStableDeviceIdentity(activeRancherSource) {
			return unverifiableIdentityError("RANCHER_DISK", storage.RancherDisk,
				[]string{storage.RancherDiskConfigured, recordedRancher, activeRancherSource, liveRancher})
		}
		if recordedRancher == "" {
			fmt.Printf("   ✓ Explicit RANCHER_DISK authorizes matching legacy mount %s\n", liveRancher)
		}
		mounts, err := blockDeviceTreeMounts(storage.RancherDisk)
		if err != nil {
			return err
		}
		for mountedDevice, mountPoints := range mounts {
			for _, mountPoint := range mountPoints {
				if mountPoint != "/var/lib/rancher" {
					return fmt.Errorf(
						"refusing to clean RANCHER_DISK %s: %s is mounted at %s",
						storage.RancherDisk, mountedDevice, mountPoint)
				}
			}
		}
		fmt.Printf("   ✓ RANCHER_DISK %-24s -> %s (safe to wipe)\n", storage.RancherDisk, canonical)
	} else if fstabRancher != "" {
		return cleanupPreflightError(
			"RANCHER_DISK exists in fstab or live mounts but is absent from cleanup configuration",
			[]string{
				fmt.Sprintf("Add RANCHER_DISK: %s to bloom.yaml", fstabRancher),
				"Or run configless cleanup if you only want strictly tagged storage wiped",
			},
		)
	} else {
		liveRancher, err := exactMountSource("/var/lib/rancher")
		if err != nil {
			return err
		}
		if getDeviceFromFstabEntry("/var/lib/rancher") != "" || liveRancher != "" {
			fmt.Println("   ⚠️  Preserving untagged /var/lib/rancher storage (configless cleanup)")
			fmt.Println("   ℹ️  To wipe it, rerun cleanup with bloom.yaml containing a matching RANCHER_DISK entry")
		}
	}

	fmt.Println("   ✅ Cleanup preflight passed")
	return nil
}
