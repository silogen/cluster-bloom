//go:build linux

package runtime

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type CleanupStorage struct {
	ClusterDisks     string
	PremountedDisks  string
	RancherDisk      string
	ConfigWasPresent bool
	RancherExplicit  bool
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
			clusterSources = append(clusterSources, entry.source)
		case bloomFstabPremounted:
			premountedMounts = append(premountedMounts, entry.mountPoint)
		case bloomFstabRancher:
			storage.RancherDisk = entry.source
		}
	}
	storage.ClusterDisks = strings.Join(clusterSources, ",")
	storage.PremountedDisks = strings.Join(premountedMounts, ",")
	return canonicalizeCleanupStorage(storage)
}

func canonicalizeCleanupStorage(storage CleanupStorage) (CleanupStorage, error) {
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

func exactMountSource(mountPoint string) string {
	out, err := exec.Command(
		"findmnt",
		"--mountpoint", mountPoint,
		"--noheadings",
		"--output", "SOURCE",
	).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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
		if liveSource := exactMountSource(entry.mountPoint); liveSource != "" {
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
		if err := assertSafeToWipe(device); err != nil {
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
		if liveSource := exactMountSource(entry.mountPoint); liveSource != "" {
			if err := compareDeviceSets([]string{entry.source}, []string{liveSource}); err != nil {
				return fmt.Errorf("fstab/live mount mismatch at %s: %w", entry.mountPoint, err)
			}
		}
	}

	clusterDevices := splitCleanupValues(storage.ClusterDisks)
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
			return fmt.Errorf("resolve premounted fstab source %s: %w", entry.source, err)
		}
		destructiveTargets[fmt.Sprintf("CLUSTER_PREMOUNTED_DISKS[%d]", index)] = canonical
	}
	if err := rejectOverlappingCleanupTargets(destructiveTargets); err != nil {
		return err
	}
	if storage.ConfigWasPresent && len(clusterDevices) > 0 && len(fstabCluster) > 0 {
		if err := compareDeviceSets(clusterDevices, fstabCluster); err != nil {
			return err
		}
	} else if storage.ConfigWasPresent && len(clusterDevices) > 0 && len(fstabCluster) == 0 {
		return fmt.Errorf("CLUSTER_DISKS are configured but no strict Bloom-managed fstab entries exist")
	} else if storage.ConfigWasPresent && len(clusterDevices) == 0 && len(fstabCluster) > 0 {
		return fmt.Errorf("fstab contains Bloom-managed disks but bloom.yaml CLUSTER_DISKS is empty")
	}

	clusterIDs, err := deviceIDs(clusterDevices)
	if err != nil {
		return fmt.Errorf("validate CLUSTER_DISKS: %w", err)
	}
	allowedClusterMounts := map[blockDeviceID]map[string]struct{}{}
	for _, entry := range fstabEntries {
		if entry.tag != bloomFstabManaged {
			continue
		}
		_, id, err := resolveBlockDevice(entry.source)
		if err != nil {
			return fmt.Errorf("resolve Bloom-managed fstab source %s: %w", entry.source, err)
		}
		if allowedClusterMounts[id] == nil {
			allowedClusterMounts[id] = map[string]struct{}{}
		}
		allowedClusterMounts[id][entry.mountPoint] = struct{}{}
	}
	for id, device := range clusterIDs {
		if err := assertSafeToWipe(device); err != nil {
			return err
		}
		mounts, err := blockDeviceTreeMounts(device)
		if err != nil {
			return err
		}
		for mountedDevice, mountPoints := range mounts {
			for _, mountPoint := range strings.Fields(mountPoints) {
				if _, allowed := allowedClusterMounts[id][mountPoint]; !allowed {
					return fmt.Errorf(
						"refusing to clean %s: %s is mounted at non-Bloom mount point %s",
						device, mountedDevice, mountPoint)
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
		if err := assertSafeToWipe(storage.RancherDisk); err != nil {
			return err
		}
		if clusterPath, collision := clusterIDs[rancherID]; collision {
			return fmt.Errorf("RANCHER_DISK %s is also configured as CLUSTER_DISK %s", storage.RancherDisk, clusterPath)
		}
		recordedRancher := fstabRancher
		liveRancher := exactMountSource("/var/lib/rancher")
		activeRancher, hasActiveRancher, err := findActiveFstabMount(fstabContent, "/var/lib/rancher")
		if err != nil {
			return err
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
			if err := compareDeviceSets([]string{storage.RancherDisk}, []string{activeRancher.source}); err != nil {
				return fmt.Errorf("RANCHER_DISK config/active fstab mismatch: %w", err)
			}
		}
		if liveRancher != "" {
			if err := compareDeviceSets([]string{storage.RancherDisk}, []string{liveRancher}); err != nil {
				return fmt.Errorf("RANCHER_DISK config/live mount mismatch: %w", err)
			}
		}
		if recordedRancher == "" {
			fmt.Printf("   ✓ Explicit RANCHER_DISK authorizes matching legacy mount %s\n", liveRancher)
		}
		mounts, err := blockDeviceTreeMounts(storage.RancherDisk)
		if err != nil {
			return err
		}
		for mountedDevice, mountPoints := range mounts {
			for _, mountPoint := range strings.Fields(mountPoints) {
				if mountPoint != "/var/lib/rancher" {
					return fmt.Errorf(
						"refusing to clean RANCHER_DISK %s: %s is mounted at %s",
						storage.RancherDisk, mountedDevice, mountPoint)
				}
			}
		}
		fmt.Printf("   ✓ RANCHER_DISK %-24s -> %s (safe to wipe)\n", storage.RancherDisk, canonical)
	} else if fstabRancher != "" {
		return fmt.Errorf("RANCHER_DISK exists in fstab or live mounts but is absent from cleanup configuration")
	} else if getDeviceFromFstabEntry("/var/lib/rancher") != "" || exactMountSource("/var/lib/rancher") != "" {
		fmt.Println("   ⚠️  Preserving untagged /var/lib/rancher storage; Bloom only wipes strictly tagged RANCHER_DISK entries")
	}

	fmt.Println("   ✅ Cleanup preflight passed")
	return nil
}
