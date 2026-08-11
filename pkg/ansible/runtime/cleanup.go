//go:build linux

package runtime

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// longhornIQNPrefix is the iSCSI target prefix for Longhorn v1 volumes attached via
// the tgt + open-iscsi stack (CSI driver.longhorn.io). Upstream defines this in
// go-iscsi-helper iscsidev.GetTargetName():
//   return "iqn.2019-10.io.longhorn:" + Volume2ISCSIName(volumeName)
// See https://github.com/longhorn/go-iscsi-helper/blob/master/iscsidev/iscsi.go
// The "2019-10" segment is the IQN registration date, not a Longhorn release tag.
// Longhorn v2/SPDK volumes do not use iSCSI; see docs/storage-management.md.
const longhornIQNPrefix = "iqn.2019-10.io.longhorn:"

type iscsiSession struct {
	sid    string
	portal string
	target string
}

var longhornProcessNames = []string{"longhorn-engine", "longhorn-instance-manager", "longhorn-manager"}

// longhornCleanupNeeded reports whether any Longhorn artifacts remain on the node.
// Medium/small clusters never deploy Longhorn; when nothing is present this avoids
// noisy kubectl drain and no-op umount retries during bloom cleanup.
func longhornCleanupNeeded() bool {
	if entries, err := os.ReadDir("/dev/longhorn"); err == nil && len(entries) > 0 {
		return true
	}
	for _, proc := range longhornProcessNames {
		if exec.Command("pgrep", "-x", proc).Run() == nil {
			return true
		}
	}
	if out, err := exec.Command("bash", "-c",
		`mount | grep -E 'longhorn|driver[.]longhorn[.]io'`).Output(); err == nil &&
		strings.TrimSpace(string(out)) != "" {
		return true
	}
	if entries, err := os.ReadDir("/var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io"); err == nil && len(entries) > 0 {
		return true
	}
	if entries, err := os.ReadDir("/var/lib/longhorn"); err == nil && len(entries) > 0 {
		return true
	}
	out, err := exec.Command("iscsiadm", "-m", "session").Output()
	if err == nil && len(parseLonghornISCSISessions(string(out))) > 0 {
		return true
	}
	return false
}

// CleanupLonghornMounts performs cleanup of Longhorn PVCs and mounts.
// Sequences: graceful kubectl drain → iSCSI logout → TERM/KILL processes → force umount.
// This ordering is required because Longhorn uses iSCSI sessions that remain
// in the kernel even after the Longhorn process is killed; skipping the iSCSI
// logout leaves the device busy and causes rm/umount to block or silently fail.
func CleanupLonghornMounts() error {
	fmt.Println("💾 Cleaning Longhorn mounts and PVCs...")
	if !longhornCleanupNeeded() {
		fmt.Println("   ℹ️  No Longhorn detected — skipping cleanup")
		return nil
	}

	// Step 1: Graceful kubectl drain (best-effort, cluster may already be down)
	fmt.Println("   🚰 Attempting graceful node drain via kubectl...")
	nodeNameOut, _ := exec.Command("hostname").Output()
	nodeName := strings.TrimSpace(string(nodeNameOut))
	kubeconfig := "/etc/rancher/rke2/rke2.yaml"
	_, kubeconfigErr := os.Stat(kubeconfig)
	apiReachable := false
	if kubeconfigErr == nil {
		fmt.Print("      🔍 Checking Kubernetes API server reachability... ")
		apiReachable = isKubeAPIReachable()
		if apiReachable {
			fmt.Println("✓ reachable")
		} else {
			fmt.Println("✗ unreachable")
		}
	}
	nodeInCluster := false
	if apiReachable && nodeName != "" {
		fmt.Printf("      🔍 Checking if %s is a member of the cluster... ", nodeName)
		nodeInCluster = isNodeInCluster(kubeconfig, nodeName)
		if nodeInCluster {
			fmt.Println("✓ yes")
		} else {
			fmt.Println("✗ no")
		}
	}
	if kubeconfigErr == nil && nodeName != "" && apiReachable && nodeInCluster {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig,
			"cordon", nodeName).Run()
		drainCtx, drainCancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer drainCancel()
		fmt.Println("      💧 Draining node (best-effort, ~30s timeout)...")
		exec.CommandContext(drainCtx, "kubectl", "--kubeconfig", kubeconfig,
			"drain", nodeName,
			"--delete-emptydir-data", "--ignore-daemonsets",
			"--force", "--disable-eviction",
			"--grace-period=10", "--timeout=30s").Run()
		// Only wait for Longhorn volumes if /dev/longhorn/ exists and has volumes
		out, _ := exec.Command("bash", "-c", "ls /dev/longhorn/ 2>/dev/null | wc -l").Output()
		initialCount := strings.TrimSpace(string(out))
		if initialCount != "" && initialCount != "0" {
			fmt.Println("      ⏳ Waiting for Longhorn volumes to detach...")
			for i := 0; i < 30; i++ {
				out, _ := exec.Command("bash", "-c", "ls /dev/longhorn/ 2>/dev/null | wc -l").Output()
				if strings.TrimSpace(string(out)) == "0" {
					break
				}
				time.Sleep(2 * time.Second)
			}
		} else {
			fmt.Println("      ℹ️  No Longhorn volumes detected — skipping volume detach wait")
		}
	} else if kubeconfigErr != nil {
		fmt.Println("      ℹ️  kubectl/kubeconfig not available — skipping drain")
	} else if !apiReachable {
		fmt.Println("      ℹ️  Kubernetes API server unreachable — skipping drain")
	} else {
		fmt.Printf("      ℹ️  Node %s is not a member of this cluster — skipping drain\n", nodeName)
	}

	// Step 2: iSCSI logout — releases kernel block device mappings for Longhorn volumes.
	// Must happen before umount; without this the device remains busy regardless of
	// whether the Longhorn process is alive. Never use an unscoped logout here:
	// cloud boot volumes can also use iSCSI, and logging out every session detaches
	// the live root disk.
	fmt.Println("   🔌 Logging out Longhorn iSCSI sessions...")
	logoutLonghornISCSISessions()

	// Step 3: Graceful TERM then KILL of Longhorn processes in dependency order
	// Use exact binary name matching to avoid killing unrelated processes.
	// Longhorn binaries have these exact names in /proc/*/comm.
	anyRunning := false
	for _, proc := range longhornProcessNames {
		if exec.Command("pgrep", "-x", proc).Run() == nil {
			anyRunning = true
			break
		}
	}
	if anyRunning {
		fmt.Println("   🛑 Stopping Longhorn processes (TERM)...")
		for _, proc := range longhornProcessNames {
			exec.Command("pkill", "-TERM", "-x", proc).Run()
		}
		time.Sleep(5 * time.Second)
		// Only KILL if some processes survived TERM
		stillRunning := false
		for _, proc := range longhornProcessNames {
			if exec.Command("pgrep", "-x", proc).Run() == nil {
				stillRunning = true
				break
			}
		}
		if stillRunning {
			fmt.Println("      ⚡ Force killing remaining Longhorn processes (KILL)...")
			for _, proc := range longhornProcessNames {
				exec.Command("pkill", "-KILL", "-x", proc).Run()
			}
		}
	} else {
		fmt.Println("   ℹ️  No Longhorn processes running — skipping")
	}

	// Step 4: Force umount everything Longhorn-related
	fmt.Println("   ⏏️  Unmounting Longhorn volumes...")
	for attempt := 1; attempt <= 3; attempt++ {
		fmt.Printf("      🔄 Attempt %d/3...\n", attempt)
		exec.Command("bash", "-c", "umount -lf /dev/longhorn/pvc-* 2>/dev/null || true").Run()
		exec.Command("bash", "-c", `mount | grep -E 'longhorn|driver[.]longhorn[.]io' | awk '{print $3}' | xargs -r umount -lf 2>/dev/null || true`).Run()
		exec.Command("bash", "-c", "umount -Af /var/lib/kubelet/pods/*/volumes/kubernetes.io~csi/pvc-* 2>/dev/null || true").Run()
		exec.Command("bash", "-c", "umount -Af /var/lib/kubelet/pods/*/volumes/kubernetes.io~csi/*/mount 2>/dev/null || true").Run()
		exec.Command("bash", "-c", "umount -Af /var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/*/globalmount 2>/dev/null || true").Run()
		exec.Command("bash", "-c", "umount -Af /var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/* 2>/dev/null || true").Run()
		// Unmount kubelet volume-subpath bind mounts (reverse-sorted so nested paths are released first)
		exec.Command("bash", "-c", `mount | grep '/var/lib/kubelet/pods/.*/volume-subpaths' | awk '{print $3}' | sort -r | xargs -r umount -lf 2>/dev/null || true`).Run()
		time.Sleep(1 * time.Second)
	}

	// Step 5: fuser as last resort for anything still held open
	exec.Command("fuser", "-km", "/dev/longhorn/").Run()

	exec.Command("rm", "-rf", "/dev/longhorn/pvc-*").Run()
	exec.Command("rm", "-rf", "/var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/*").Run()

	fmt.Println("   ✅ Longhorn cleanup completed")
	return nil
}

// parseLonghornISCSISessions extracts the session ID, portal, and target from
// `iscsiadm -m session` output for targets matching longhornIQNPrefix only.
// If Longhorn upstream ever changes GetTargetName(), update longhornIQNPrefix
// and the matching awk in GenerateCleanupTasks to stay in sync.
// Longhorn v2/SPDK uses block devices directly and is outside this iSCSI path.
func parseLonghornISCSISessions(out string) []iscsiSession {
	var sessions []iscsiSession
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		targetIndex := -1
		for i, field := range fields {
			if strings.HasPrefix(field, longhornIQNPrefix) {
				targetIndex = i
				break
			}
		}
		if targetIndex < 0 {
			continue
		}
		sidIndex := -1
		for i := 0; i < targetIndex; i++ {
			if strings.HasPrefix(fields[i], "[") && strings.HasSuffix(fields[i], "]") {
				sidIndex = i
				break
			}
		}
		if sidIndex < 0 || sidIndex+1 >= targetIndex {
			continue
		}
		sessions = append(sessions, iscsiSession{
			sid:    strings.Trim(fields[sidIndex], "[]"),
			portal: strings.SplitN(fields[sidIndex+1], ",", 2)[0],
			target: fields[targetIndex],
		})
	}
	return sessions
}

// logoutLonghornISCSISessions logs out each Longhorn session by session ID,
// then removes only its matching target/portal node record. Other sessions,
// including cloud boot volumes, remain untouched.
func logoutLonghornISCSISessions() {
	out, err := exec.Command("iscsiadm", "-m", "session").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 21 {
			fmt.Println("      ℹ️  No active iSCSI sessions detected")
		} else {
			fmt.Printf("      ⚠️  Warning: Could not list iSCSI sessions: %v\n", err)
		}
		return
	}

	sessions := parseLonghornISCSISessions(string(out))
	if len(sessions) == 0 {
		fmt.Println("      ℹ️  No Longhorn iSCSI sessions detected")
		return
	}

	loggedOut := make([]iscsiSession, 0, len(sessions))
	for _, session := range sessions {
		if err := exec.Command("iscsiadm", "-m", "session", "-r", session.sid, "--logout").Run(); err != nil {
			fmt.Printf("      ⚠️  Warning: Failed to log out Longhorn session %s (%s): %v\n",
				session.sid, session.target, err)
			continue
		}
		loggedOut = append(loggedOut, session)
		fmt.Printf("      ✓ Logged out Longhorn session %s (%s)\n", session.sid, session.target)
	}

	seenNodes := map[string]struct{}{}
	for _, session := range loggedOut {
		key := session.target + "\x00" + session.portal
		if _, seen := seenNodes[key]; seen {
			continue
		}
		seenNodes[key] = struct{}{}
		if err := exec.Command("iscsiadm", "-m", "node",
			"-T", session.target, "-p", session.portal, "--op=delete").Run(); err != nil {
			fmt.Printf("      ⚠️  Warning: Failed to remove Longhorn node record %s at %s: %v\n",
				session.target, session.portal, err)
		}
	}
}

// isKubeAPIReachable checks that the RKE2 API server is both reachable and
// responsive at the HTTP level. A plain TCP dial is not sufficient — a degraded
// or starting-up API server can accept the connection then stall on the request.
func isKubeAPIReachable() bool {
	// Quick TCP probe first — avoids the process-spawn cost when port is closed
	conn, err := net.DialTimeout("tcp", "127.0.0.1:6443", 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	// HTTP-level check: kubectl version with a short request timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = exec.CommandContext(ctx, "kubectl",
		"--kubeconfig", "/etc/rancher/rke2/rke2.yaml",
		"version", "--request-timeout=4s").Run()
	return err == nil
}

// isNodeInCluster checks whether nodeName appears in the cluster node list.
// Uses a short timeout so it doesn't block cleanup if the API is sluggish.
func isNodeInCluster(kubeconfig, nodeName string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "kubectl",
		"--kubeconfig", kubeconfig,
		"get", "node", nodeName,
		"--no-headers", "--ignore-not-found",
		"--request-timeout=4s",
		"-o", "name").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// UninstallRKE2 executes the RKE2 uninstall script if it exists
func UninstallRKE2() error {
	fmt.Println("🔧 Uninstalling RKE2...")

	// Run uninstall script if it exists
	if _, err := os.Stat("/usr/local/bin/rke2-uninstall.sh"); err == nil {
		fmt.Println("   ⏳ Executing RKE2 uninstall script (may take a couple minutes)...")
		cmd := exec.Command("/usr/local/bin/rke2-uninstall.sh")
		output, err := cmd.CombinedOutput()

		// Log output regardless of error (matching Bloom v1 behavior)
		if len(output) > 0 {
			fmt.Printf("      📋 RKE2 uninstall script output: %s\n", string(output))
		}

		if err != nil {
			fmt.Printf("      ⚠️  RKE2 uninstall script encountered warnings: %v\n", err)
			// Don't return error - Bloom v1 continues on uninstall script errors
		} else {
			fmt.Println("      ✓ RKE2 uninstall script executed successfully")
		}
	} else {
		fmt.Println("   ℹ️  RKE2 uninstall script not found")
	}

	// Always force-remove RKE2 directories to ensure clean state
	// This handles cases where the uninstall script doesn't exist, fails, or leaves remnants
	fmt.Println("   🗑️  Removing RKE2 directories and data...")
	directories := []string{
		"/etc/rancher/rke2",
		"/var/lib/rancher/rke2",
		"/var/lib/kubelet",
	}

	for _, dir := range directories {
		if _, err := os.Stat(dir); err == nil {
			cmd := exec.Command("rm", "-rf", dir)
			if err := cmd.Run(); err != nil {
				fmt.Printf("      ⚠️  Warning: Failed to remove %s: %v\n", dir, err)
			} else {
				fmt.Printf("      ✓ Removed %s\n", dir)
			}
		}
	}

	fmt.Println("   ✅ RKE2 uninstall completed")
	return nil
}


// CleanupBloomDisks removes bloom-managed disks and cleans up disk state
func CleanupBloomDisks(clusterDisks string) error {
	fmt.Println("💽 Cleaning bloom-managed disks...")

	// Lock destructive scope to the exact devices validated against fstab.
	if err := validateClusterDiskScope(clusterDisks); err != nil {
		return fmt.Errorf("CLUSTER_DISKS scope changed before wipe: %w", err)
	}

	// Enter critical section for disk operations
	EnterCriticalSection("disk cleanup and fstab modification")
	defer func() {
		ExitCriticalSection()
	}()

	// First unmount prior Longhorn disks (equivalent to UnmountPriorLonghornDisks)
	if err := unmountPriorLonghornDisks(clusterDisks); err != nil {
		return fmt.Errorf("unmount Bloom-managed disks and update fstab: %w", err)
	}

	// Directly unmount all CLUSTER_DISKS devices if they're mounted
	if err := unmountClusterDisks(clusterDisks); err != nil {
		return fmt.Errorf("unmount CLUSTER_DISKS: %w", err)
	}

	// Parse mount output to find and unmount CSI driver mounts
	fmt.Println("   🔍 Checking for CSI driver mounts...")
	cmd := exec.Command("mount")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("mount command failed: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 2 && strings.Contains(fields[2], "kubernetes.io/csi/driver.longhorn.io") {
			_, err := exec.Command("sudo", "umount", "-lf", fields[2]).CombinedOutput()
			if err != nil {
				fmt.Printf("      ⚠️  Warning: Failed to unmount %s\n", fields[2])
			} else {
				fmt.Printf("      ✓ Unmounted %s\n", fields[2])
			}
		}
	}

	// Force unmount all CLUSTER_DISKS devices again to ensure they're released
	// This is critical before wipefs to prevent I/O errors
	fmt.Println("   ⏏️  Final unmount pass for cluster disks...")
	for device := range strings.SplitSeq(clusterDisks, ",") {
		device = strings.TrimSpace(device)
		if device == "" {
			continue
		}
		// Attempt multiple unmount passes to ensure device is fully released
		for i := 0; i < 3; i++ {
			exec.Command("umount", "-lf", device).Run()
			exec.Command("umount", "-f", device).Run()
		}
	}

	// CRITICAL: Wait for kernel to fully flush pending I/O and release devices
	// Without this delay, wipefs can cause catastrophic I/O errors on busy systems
	fmt.Println("   ⏳ Waiting for kernel to release devices (10s)...")
	time.Sleep(10 * time.Second)

	// Wipe and reformat every CLUSTER_DISKS device
	// Only proceed if device is confirmed unmounted to prevent system corruption
	fmt.Println("   🧹 Wiping and formatting cluster disks...")
	for device := range strings.SplitSeq(clusterDisks, ",") {
		device = strings.TrimSpace(device)
		if device == "" {
			continue
		}

		// Re-run the protected-device guard immediately before destruction.
		// Preflight already checks this, but live storage topology can change
		// between confirmation and the wipe operation.
		if err := assertSafeToWipe(device); err != nil {
			return err
		}

		if err := assertDeviceTreeUnmounted(device); err != nil {
			return err
		}

		if out, err := exec.Command("wipefs", "-a", device).CombinedOutput(); err != nil {
			return fmt.Errorf("wipefs failed on %s: %w (%s)", device, err, strings.TrimSpace(string(out)))
		}
		fmt.Printf("      ✓ Wiped %s\n", device)
		if out, err := exec.Command("mkfs.ext4", "-F", device).CombinedOutput(); err != nil {
			return fmt.Errorf("mkfs.ext4 failed on %s: %w (%s)", device, err, strings.TrimSpace(string(out)))
		}
		fmt.Printf("      ✓ Formatted %s as ext4\n", device)
	}

	// Remove longhorn plugins directory
	_, err = exec.Command("sudo", "rm", "-rf", "/var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/*").CombinedOutput()
	if err != nil {
		fmt.Printf("   ⚠️  Warning: Failed to remove longhorn plugins directory: %v\n", err)
	}

	// Keep cleaned block devices attached to the kernel. Bloom v1 hot-removed all
	// unmounted bare sd* devices via /sys/block/<device>/device/delete to discard
	// stale Longhorn iSCSI LUNs. Longhorn sessions and node records are now cleaned
	// explicitly by CleanupLonghornMounts, so that sweep is unnecessary and cannot
	// distinguish an orphaned LUN from an unmounted virtio-scsi cloud volume.
	// Cleanup may wipe and reformat configured disks, but it must never detach them.

	// Skip filesystem sync as it commonly hangs on systems with I/O issues
	// The 500ms delay below is sufficient for kernel to release mounts
	fmt.Println("   ⏳ Allowing kernel to flush pending I/O...")

	// Brief delay to allow kernel to fully release mounts
	time.Sleep(500 * time.Millisecond)

	fmt.Println("   ✅ Disk cleanup completed")
	return nil
}

// unmountClusterDisks directly unmounts all devices found in CLUSTER_DISKS
func unmountClusterDisks(clusterDisks string) error {
	if clusterDisks == "" {
		return nil
	}

	devices := strings.Split(clusterDisks, ",")
	fmt.Printf("   ⏏️  Unmounting cluster disks: %s\n", clusterDisks)

	for _, device := range devices {
		device = strings.TrimSpace(device)
		if device == "" {
			continue
		}

		// Skip silently if the device is not currently mounted
		out, _ := exec.Command("findmnt", "--source", device, "--noheadings").Output()
		if strings.TrimSpace(string(out)) == "" {
			continue
		}
		// Unmount the device
		cmd := exec.Command("umount", device)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("unmount %s: %w", device, err)
		} else {
			fmt.Printf("      ✓ Successfully unmounted %s\n", device)
		}
	}

	return nil
}

// GenerateCleanupTasks is retained for source compatibility only. Destructive
// cleanup must run through the Go implementation so strict fstab parsing,
// config/live preflight, and protected-device guards cannot be bypassed.
func GenerateCleanupTasks(clusterDisks string, premountedDisks string, rancherDisk string) []map[string]any {
	return nil
}

// generateLegacyCleanupTasks is kept temporarily as migration context for the
// removed task generator. It is unexported and has no callers.
func generateLegacyCleanupTasks(clusterDisks string, premountedDisks string, rancherDisk string) []map[string]any {
	var cleanupTasks []map[string]any

	// Main cleanup block task
	cleanupBlock := map[string]any{
		"name": "⚠️ DESTRUCTIVE CLEANUP: Remove existing Bloom cluster installation",
		"tags": []string{"cleanup", "destroy-data"},
		"block": []map[string]any{
			{
				"name": "Display destructive operation warning",
				"debug": map[string]any{
					"msg": []string{
						"⚠️  DANGER: DESTRUCTIVE OPERATION IN PROGRESS ⚠️",
						"",
						"This playbook will PERMANENTLY DESTROY:",
						"• Entire Kubernetes cluster (RKE2 uninstall)",
						"• ALL Longhorn storage volumes and data",
						"• ALL managed disk devices (wipefs + reformat)",
						fmt.Sprintf("• All data on storage devices: %s", clusterDisks),
						"",
						"This action cannot be undone.",
					},
				},
			},
			// Step 1: Graceful kubectl drain while the cluster is still up
			{
				"name":         "Get node hostname for kubectl drain",
				"shell":        "hostname",
				"register":     "cleanup_hostname",
				"changed_when": false,
				"failed_when":  false,
			},
			{
				"name":     "Check if kubeconfig is available (cluster running)",
				"stat":     map[string]any{"path": "/etc/rancher/rke2/rke2.yaml"},
				"register": "cleanup_kubeconfig",
			},
			{
				"name":        "Cordon node to prevent new scheduling",
				"shell":       "/var/lib/rancher/rke2/bin/kubectl --kubeconfig /etc/rancher/rke2/rke2.yaml cordon {{ cleanup_hostname.stdout }} 2>/dev/null || true",
				"when":        "cleanup_kubeconfig.stat.exists",
				"failed_when": false,
			},
			{
				"name":        "Drain node (best-effort, allows Longhorn to detach volumes)",
				"shell":       "/var/lib/rancher/rke2/bin/kubectl --kubeconfig /etc/rancher/rke2/rke2.yaml drain {{ cleanup_hostname.stdout }} --delete-emptydir-data --ignore-daemonsets --force --disable-eviction --grace-period=10 --timeout=30s 2>/dev/null || true",
				"when":        "cleanup_kubeconfig.stat.exists",
				"failed_when": false,
			},
			{
				"name":         "Wait for Longhorn volumes to detach after drain",
				"shell":        "for i in $(seq 1 30); do [ -z \"$(ls /dev/longhorn/ 2>/dev/null)\" ] && exit 0; sleep 2; done; echo \"timeout\"",
				"when":         "cleanup_kubeconfig.stat.exists",
				"failed_when":  false,
				"changed_when": false,
			},
			// Step 2: iSCSI logout — must happen before umount or the block device stays busy
			{
				"name":        "Logout Longhorn iSCSI sessions (preserve non-Longhorn devices)",
				"shell":       "iscsiadm -m session 2>/dev/null | awk -v prefix='" + longhornIQNPrefix + "' '{ target=\"\"; sid=\"\"; portal=\"\"; for (i=1; i<=NF; i++) { if (index($i,prefix)==1) target=$i; if ($i ~ /^\\[[0-9]+\\]$/) { sid=$i; portal=$(i+1) } } if (target!=\"\" && sid!=\"\") { gsub(/[][]/,\"\",sid); split(portal,p,\",\"); print sid,p[1],target } }' | while read -r sid portal target; do iscsiadm -m session -r \"$sid\" --logout 2>/dev/null || continue; iscsiadm -m node -T \"$target\" -p \"$portal\" --op=delete 2>/dev/null || true; done",
				"failed_when": false,
			},
			// Step 3: Stop Longhorn processes gracefully in dependency order
			// Use -x for exact binary name matching to avoid killing unrelated processes
			{
				"name":        "Gracefully stop Longhorn processes (TERM)",
				"shell":       "pkill -TERM -x longhorn-engine 2>/dev/null || true; pkill -TERM -x longhorn-instance-manager 2>/dev/null || true; pkill -TERM -x longhorn-manager 2>/dev/null || true",
				"failed_when": false,
			},
			{
				"name":         "Wait for Longhorn processes to stop",
				"shell":        "sleep 5",
				"changed_when": false,
			},
			{
				"name":        "Force kill remaining Longhorn processes (KILL)",
				"shell":       "pkill -KILL -x longhorn-engine 2>/dev/null || true; pkill -KILL -x longhorn-instance-manager 2>/dev/null || true; pkill -KILL -x longhorn-manager 2>/dev/null || true",
				"failed_when": false,
			},
			// Step 4: Stop RKE2 so no new mounts are created
			{
				"name": "Stop and disable RKE2 services",
				"systemd": map[string]any{
					"name":    "{{ item }}",
					"state":   "stopped",
					"enabled": false,
				},
				"loop":        []string{"rke2-server", "rke2-agent"},
				"failed_when": false,
			},
			// Step 5: Force umount all remaining Longhorn mounts
			{
				"name":        "Force umount all Longhorn-related mounts",
				"shell":       "mount | grep -E 'longhorn|driver\\.longhorn\\.io' | awk '{print $3}' | xargs -r umount -lf 2>/dev/null || true; umount -lf /dev/longhorn/pvc-* 2>/dev/null || true; umount -Af /var/lib/kubelet/pods/*/volumes/kubernetes.io~csi/pvc-* 2>/dev/null || true; umount -Af /var/lib/kubelet/pods/*/volumes/kubernetes.io~csi/*/mount 2>/dev/null || true; umount -Af /var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/*/globalmount 2>/dev/null || true; umount -Af /var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/* 2>/dev/null || true; mount | grep '/var/lib/kubelet/pods/.*/volume-subpaths' | awk '{print $3}' | sort -r | xargs -r umount -lf 2>/dev/null || true",
				"failed_when": false,
			},
			{
				"name":        "Remove Longhorn device files and kubelet CSI state",
				"shell":       "rm -rf /dev/longhorn/pvc-* /var/lib/longhorn/* /var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/* 2>/dev/null || true",
				"failed_when": false,
			},
			// Step 6: Uninstall RKE2
			{
				"name":        "Run RKE2 uninstall script",
				"shell":       "/usr/local/bin/rke2-uninstall.sh",
				"register":    "rke2_uninstall",
				"failed_when": false,
			},
			{
				"name":        "Clean RKE2 directories and files",
				"shell":       "rm -rf /var/lib/rancher/rke2 /etc/rancher/rke2 /var/lib/kubelet /var/log/pods /var/log/containers; rm -f /usr/local/bin/rke2* /usr/local/bin/kubectl /usr/local/bin/crictl /usr/local/bin/ctr; echo 'RKE2 cleanup completed'",
				"register":    "rke2_cleanup",
				"failed_when": false,
			},
		},
	}

	cleanupTasks = append(cleanupTasks, cleanupBlock)

	// Disk cleanup tasks (based on CleanupBloomDisks)
	if clusterDisks != "" {
		diskCleanupTask := map[string]any{
			"name": "Clean and wipe cluster disks",
			"tags": []string{"cleanup", "destroy-data", "storage"},
			"block": []map[string]any{
				{
					"name": "Convert CLUSTER_DISKS to list for cleanup",
					"set_fact": map[string]any{
						"cluster_disks_cleanup_list": fmt.Sprintf("{{ '%s'.split(',') | map('trim') | select('!=', '') | list }}", clusterDisks),
					},
				},
				{
					"name":        "Unmount cluster disks (skip if not mounted)",
					"shell":       "findmnt --source {{ item }} --noheadings -o TARGET | xargs -r umount -lf 2>/dev/null || true",
					"loop":        "{{ cluster_disks_cleanup_list }}",
					"failed_when": false,
				},
				{
					"name":        "Remove bloom-managed fstab entries (preserve premounted entries)",
					"shell":       "sed -i '/# managed by cluster-bloom/{/# premounted by cluster-bloom/!d}' /etc/fstab",
					"failed_when": false,
				},
				{
					"name":        "Wipe and reformat cluster disks",
					"shell":       "wipefs -a {{ item }} && mkfs.ext4 -F {{ item }}",
					"loop":        "{{ cluster_disks_cleanup_list }}",
					"failed_when": false,
				},
				{
					"name":        "Pre-clean bloom artifacts from future mount point dirs (preserve user files)",
					"shell":       fmt.Sprintf(`n=$(echo '%s' | tr ',' $'\n' | grep -c '.'); reserved=$({ grep '# premounted by cluster-bloom' /etc/fstab 2>/dev/null | awk '{print $2}' | sed 's|.*/disk||'; printf '%%s' '{{ CLUSTER_PREMOUNTED_DISKS }}' | tr ',' $'\n' | sed 's/[[:space:]]//g;s|.*/disk||'; } | grep -E '^[0-9]+$' | sort -un | tr $'\n' ' '); start=0; while true; do conflict=0; i=0; while [ $i -lt $n ]; do idx=$((start+i)); for r in $reserved; do [ "$idx" = "$r" ] && conflict=1 && break; done; [ $conflict -eq 1 ] && break; i=$((i+1)); done; [ $conflict -eq 0 ] && break; start=$((start+1)); done; i=0; while [ $i -lt $n ]; do mp="/mnt/disk$((start+i))"; [ -d "$mp" ] && rm -rf "$mp"/pvc-* "$mp"/replicas "$mp"/longhorn-disk.cfg "$mp"/longhorn-disk.cfg.tmp 2>/dev/null || true; i=$((i+1)); done`, clusterDisks),
					"failed_when": false,
				},
			},
		}
		cleanupTasks = append(cleanupTasks, diskCleanupTask)
	}

	// Completion task
	// Premounted disk cleanup — wipe contents only, keep filesystem + fstab entry
	if premountedDisks != "" {
		premountedCleanupTask := map[string]any{
			"name": "Clean premounted disk contents (preserve filesystem)",
			"tags": []string{"cleanup", "destroy-data", "storage"},
			"block": []map[string]any{
				{
					"name": "Parse premounted disks list for cleanup",
					"set_fact": map[string]any{
						"premounted_cleanup_list": "{{ CLUSTER_PREMOUNTED_DISKS.split(',') | map('trim') | reject('equalto', '') | list }}",
					},
				},
				{
					"name":  "Ensure premounted disks are mounted for cleanup",
					"shell": "mountpoint -q {{ item }} || mount {{ item }} 2>/dev/null || true",
					"loop":  "{{ premounted_cleanup_list }}",
				},
				{
					"name":        "Remove PVC directories and Longhorn state from premounted disks",
					"shell":       "rm -rf {{ item }}/pvc-* {{ item }}/replicas {{ item }}/longhorn-disk.cfg {{ item }}/longhorn-disk.cfg.tmp 2>/dev/null; echo 'cleaned {{ item }}'",
					"loop":        "{{ premounted_cleanup_list }}",
					"failed_when": false,
				},
				{
					"name":  "Verify premounted disks are still mounted after cleanup",
					"shell": "mountpoint -q {{ item }}",
					"loop":  "{{ premounted_cleanup_list }}",
				},
			},
		}
		cleanupTasks = append(cleanupTasks, premountedCleanupTask)
	}

	// RANCHER_DISK cleanup — unmount bind mount and clean fstab entry
	if rancherDisk != "" {
		rancherDiskCleanupTask := map[string]any{
			"name": "Clean RANCHER_DISK configuration",
			"tags": []string{"cleanup", "destroy-data", "storage"},
			"block": []map[string]any{
				{
					"name":        "Unmount /var/lib/rancher bind mount",
					"shell":       "umount /var/lib/rancher 2>/dev/null || true",
					"failed_when": false,
				},
				{
					"name":        "Remove RANCHER_DISK fstab entry",
					"shell":       "sed -i '/# managed by cluster-bloom rancher-disk/d' /etc/fstab",
					"failed_when": false,
				},
				{
					"name":        "Remove RANCHER_DISK fstab entry",
					"shell":       "sed -i '/UUID=.*\\/var\\/lib\\/rancher.*# managed by cluster-bloom rancher-disk/d' /etc/fstab",
					"failed_when": false,
				},
				{
					"name":        "Create clean /var/lib/rancher directory", 
					"shell":       "rm -rf /var/lib/rancher && mkdir -p /var/lib/rancher",
					"failed_when": false,
				},
			},
		}
		cleanupTasks = append(cleanupTasks, rancherDiskCleanupTask)
	}

	finalTask := map[string]any{
		"name": "Cleanup completion summary",
		"debug": map[string]any{
			"msg": []string{
				"✅ Destructive cleanup completed",
				"• RKE2 services stopped and uninstalled",
				"• Longhorn storage cleaned",
				"• Disk devices wiped and unmounted",
				"• System ready for fresh installation",
				"",
				"If this node had k3s paused automatically, resume it with:",
				"  sudo systemctl start k3s-server   # or: systemctl start k3s",
				"If networking fails after resume:",
				"  sudo ip link delete cni0 2>/dev/null; sudo ip link delete flannel.1 2>/dev/null",
				"",
				"Proceeding with normal cluster deployment...",
			},
		},
		"tags": []string{"cleanup", "destroy-data"},
	}
	cleanupTasks = append(cleanupTasks, finalTask)

	return cleanupTasks
}

// CleanupPremountedDisks clears PVC data and Longhorn state from premounted disks
// without wiping the filesystem — the disks remain mounted and ext4-formatted.
func CleanupPremountedDisks(premountedDisks string) error {
	if premountedDisks == "" {
		fmt.Println("   No premounted disks configured - skipping")
		return nil
	}
	fmt.Println("💾 Cleaning premounted disk contents (preserving filesystems)...")
	_, entries, err := readBloomFstab()
	if err != nil {
		return err
	}
	recordedSources := map[string]string{}
	for _, entry := range entries {
		if entry.tag == bloomFstabPremounted {
			recordedSources[entry.mountPoint] = entry.source
		}
	}
	mountPoints := strings.Split(premountedDisks, ",")
	for _, mp := range mountPoints {
		mp = strings.TrimSpace(mp)
		if mp == "" {
			continue
		}
		recordedSource, exists := recordedSources[mp]
		if !exists {
			return fmt.Errorf("premounted cleanup path %s has no strictly tagged fstab entry", mp)
		}
		// Ensure it is mounted before we try to clean it
		if _, err := exec.Command("mountpoint", "-q", mp).CombinedOutput(); err != nil {
			fmt.Printf("   📌 Mounting %s before cleanup...\n", mp)
			if _, err2 := exec.Command("mount", mp).CombinedOutput(); err2 != nil {
				return fmt.Errorf("mount premounted disk %s: %w", mp, err2)
			}
		}
		liveSource := exactMountSource(mp)
		if liveSource == "" {
			return fmt.Errorf("premounted cleanup path %s is not an exact mount point", mp)
		}
		if err := compareDeviceSets([]string{recordedSource}, []string{liveSource}); err != nil {
			return fmt.Errorf("premounted cleanup source changed at %s: %w", mp, err)
		}
		// Verify no iSCSI sessions are still holding pvc-* devices within this mountpoint.
		// If any remain the remove will block; force-unmount the sub-paths first.
		pvcPaths, err := filepath.Glob(filepath.Join(mp, "pvc-*"))
		if err != nil {
			return fmt.Errorf("list PVC paths under %s: %w", mp, err)
		}
		for _, pvcPath := range pvcPaths {
			exec.Command("umount", "-lf", pvcPath).Run()
		}
		if err := compareDeviceSets([]string{recordedSource}, []string{exactMountSource(mp)}); err != nil {
			return fmt.Errorf("premounted cleanup source changed at %s: %w", mp, err)
		}
		// Remove PVC dirs and Longhorn disk state; keep the ext4 filesystem intact
		removePaths := append(pvcPaths,
			filepath.Join(mp, "replicas"),
			filepath.Join(mp, "longhorn-disk.cfg"),
			filepath.Join(mp, "longhorn-disk.cfg.tmp"),
		)
		for _, path := range removePaths {
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("remove Bloom artifact %s: %w", path, err)
			}
		}
		fmt.Printf("   ✓ Cleaned contents of %s\n", mp)
	}
	fmt.Println("   ✅ Premounted disk cleanup completed")
	return nil
}

// CleanupRancherDisk unmounts the RANCHER_DISK bind mount and cleans the data directory
func CleanupRancherDisk(rancherDisk string, explicitlyConfigured bool) error {
	if strings.TrimSpace(rancherDisk) == "" {
		fmt.Println("   No strictly managed RANCHER_DISK configured - skipping")
		return nil
	}

	liveDevice := getDeviceFromCurrentMount("/var/lib/rancher")
	fstabContent, entries, err := readBloomFstab()
	if err != nil {
		return err
	}
	activeRancher, hasActiveRancher, err := findActiveFstabMount(fstabContent, "/var/lib/rancher")
	if err != nil {
		return err
	}
	fstabDevice := ""
	for _, entry := range entries {
		if entry.tag == bloomFstabRancher {
			if fstabDevice != "" {
				return fmt.Errorf("multiple strictly tagged RANCHER_DISK fstab entries")
			}
			canonical, _, err := resolveBlockDevice(entry.source)
			if err != nil {
				return fmt.Errorf("resolve RANCHER_DISK fstab source %s: %w", entry.source, err)
			}
			fstabDevice = canonical
		}
	}
	if fstabDevice == "" {
		if !explicitlyConfigured {
			return fmt.Errorf("configless RANCHER_DISK cleanup requires a strictly tagged fstab entry")
		}
		if liveDevice == "" {
			return fmt.Errorf("explicit RANCHER_DISK has no matching live /var/lib/rancher mount")
		}
	}

	devicePath := strings.TrimSpace(rancherDisk)
	if liveDevice != "" {
		if err := compareDeviceSets([]string{devicePath}, []string{liveDevice}); err != nil {
			return fmt.Errorf("RANCHER_DISK does not match live /var/lib/rancher mount: %w", err)
		}
	}
	if fstabDevice != "" {
		if err := compareDeviceSets([]string{devicePath}, []string{fstabDevice}); err != nil {
			return fmt.Errorf("RANCHER_DISK does not match /etc/fstab: %w", err)
		}
	}
	if hasActiveRancher {
		if err := compareDeviceSets([]string{devicePath}, []string{activeRancher.source}); err != nil {
			return fmt.Errorf("RANCHER_DISK does not match active fstab entry: %w", err)
		}
	}
	if err := assertSafeToWipe(devicePath); err != nil {
		return err
	}

	fmt.Printf("💾 Cleaning RANCHER_DISK %s...\n", devicePath)
	EnterCriticalSection("RANCHER_DISK cleanup and fstab modification")
	defer ExitCriticalSection()

	if isVarLibRancherMounted() {
		if lsofOutput, err := exec.Command("lsof", "+D", "/var/lib/rancher").CombinedOutput(); err == nil &&
			len(strings.TrimSpace(string(lsofOutput))) > 0 {
			fmt.Printf("      👀 Processes using /var/lib/rancher:\n%s\n", string(lsofOutput))
		}
		umountOutput, err := exec.Command("sudo", "umount", "-lf", "/var/lib/rancher").CombinedOutput()
		if err != nil {
			return fmt.Errorf("unmount /var/lib/rancher: %w (%s)", err, strings.TrimSpace(string(umountOutput)))
		}
		if isVarLibRancherMounted() {
			return fmt.Errorf("/var/lib/rancher is still mounted after unmount")
		}
		fmt.Println("      ✓ Unmounted /var/lib/rancher")
	}

	// Re-evaluate system topology immediately before destructive operations.
	if err := assertSafeToWipe(devicePath); err != nil {
		return err
	}
	if err := assertDeviceTreeUnmounted(devicePath); err != nil {
		return err
	}
	fmt.Printf("      🗑️  Wiping filesystem signatures on %s\n", devicePath)
	if output, err := exec.Command("wipefs", "-a", devicePath).CombinedOutput(); err != nil {
		return fmt.Errorf("wipe %s: %w (%s)", devicePath, err, strings.TrimSpace(string(output)))
	}
	if output, err := exec.Command("mkfs.ext4", "-F", devicePath).CombinedOutput(); err != nil {
		return fmt.Errorf("format %s: %w (%s)", devicePath, err, strings.TrimSpace(string(output)))
	}
	fmt.Printf("      ✓ Wiped and formatted %s as ext4\n", devicePath)

	if fstabDevice != "" {
		fmt.Println("   📝 Removing strictly tagged /var/lib/rancher fstab entry...")
		if err := removeBloomFstabEntries(bloomFstabRancher); err != nil {
			return err
		}
	} else if hasActiveRancher {
		fmt.Println("   📝 Removing validated legacy /var/lib/rancher fstab entry...")
		if err := removeExactFstabLine(activeRancher.raw); err != nil {
			return err
		}
	}

	// Create clean /var/lib/rancher directory
	fmt.Println("   📁 Creating clean /var/lib/rancher directory...")
	exec.Command("rm", "-rf", "/var/lib/rancher").Run()
	if _, err := exec.Command("mkdir", "-p", "/var/lib/rancher").CombinedOutput(); err != nil {
		fmt.Printf("      ⚠️  Warning: Failed to create /var/lib/rancher: %v\n", err)
	} else {
		fmt.Printf("      ✓ Created clean /var/lib/rancher directory\n")
	}

	fmt.Println("   ✅ RANCHER_DISK cleanup completed")
	return nil
}

// --- Disk index helpers ---

// extractDiskIndex extracts the integer N from a /mnt/diskN path.
func extractDiskIndex(mountPoint string) (int, error) {
	mp := strings.TrimSpace(mountPoint)
	index, ok := parseBloomDiskMountPoint(mp)
	if !ok {
		return 0, fmt.Errorf("not a valid /mnt/diskN path (N must be 0-%d): %s", maxBloomDiskIndex, mp)
	}
	return index, nil
}

// isDiskBloomArtifact reports whether the named entry is a Longhorn/bloom artifact.
func isDiskBloomArtifact(name string) bool {
return strings.HasPrefix(name, "pvc-") ||
name == "replicas" ||
name == "longhorn-disk.cfg" ||
name == "longhorn-disk.cfg.tmp"
}

// inspectDirContents returns bloom artifacts and user files found directly inside dir.
// Excludes lost+found as it's an automatically created ext4 filesystem folder, not user data.
func inspectDirContents(dir string) (bloom []string, user []string) {
entries, err := os.ReadDir(dir)
if err != nil {
return
}
for _, e := range entries {
if isDiskBloomArtifact(e.Name()) {
bloom = append(bloom, e.Name())
} else if e.Name() != "lost+found" {
user = append(user, e.Name())
}
// lost+found is silently excluded - it's an ext4 system folder, not user data
}
return
}

// countClusterDisksStr counts non-empty entries in a comma-separated CLUSTER_DISKS string.
func countClusterDisksStr(clusterDisks string) int {
if clusterDisks == "" {
return 0
}
count := 0
for _, d := range strings.Split(clusterDisks, ",") {
if strings.TrimSpace(d) != "" {
count++
}
}
return count
}

// calculateFutureDiskStart returns the lowest start index S such that the sequential
// range [S, S+diskCount) does not overlap reserved indexes.
//
// Reserved = every currently-mounted /mnt/diskN path EXCEPT those whose block device
// is explicitly listed in clusterDisks (those will be unmounted and wiped by cleanup).
// Fstab tags are intentionally not used as the authority here: a disk may carry a stale
// bloom tag from a previous run while no longer being in the current CLUSTER_DISKS
// config, meaning bloom will NOT wipe it and its mount point remains occupied.
func calculateFutureDiskStart(clusterDisks, premountedDisks string, diskCount int) int {
reserved := map[int]bool{}

// Build a set of the block devices in CLUSTER_DISKS — their current mount points
// will be freed by cleanup and are therefore NOT reserved.
clusterDevSet := map[string]bool{}
for _, dev := range strings.Split(clusterDisks, ",") {
if d := strings.TrimSpace(dev); d != "" {
clusterDevSet[d] = true
}
}

// Find the current mount point of each CLUSTER_DISKS device via /proc/mounts.
clusterDiskMounts := map[string]bool{}
if data, err := os.ReadFile("/proc/mounts"); err == nil {
for _, line := range strings.Split(string(data), "\n") {
fields := strings.Fields(line)
if len(fields) >= 2 && clusterDevSet[fields[0]] {
clusterDiskMounts[fields[1]] = true
}
}
}

// From CLUSTER_PREMOUNTED_DISKS config string
for _, p := range strings.Split(premountedDisks, ",") {
if idx, err := extractDiskIndex(strings.TrimSpace(p)); err == nil {
reserved[idx] = true
}
}

// From strictly parsed /etc/fstab premounted-by-cluster-bloom entries.
if _, entries, err := readBloomFstab(); err == nil {
for _, entry := range entries {
if entry.tag == bloomFstabPremounted {
if idx, err2 := extractDiskIndex(entry.mountPoint); err2 == nil {
reserved[idx] = true
}
}
}
}

// Reserve every currently-mounted /mnt/diskN that is NOT a CLUSTER_DISKS mount.
// This covers premounted disks, user disks, and any stale bloom-tagged disks that
// are no longer in CLUSTER_DISKS — all of which survive cleanup.
if data, err := os.ReadFile("/proc/mounts"); err == nil {
for _, line := range strings.Split(string(data), "\n") {
fields := strings.Fields(line)
if len(fields) < 2 {
continue
}
mp := fields[1]
if clusterDiskMounts[mp] {
continue // CLUSTER_DISKS mount — will be freed
}
if idx, err2 := extractDiskIndex(mp); err2 == nil {
reserved[idx] = true
}
}
}

// Find lowest S such that {S, S+1, ..., S+diskCount-1} is free of reserved indexes
for start := 0; ; start++ {
ok := true
for i := 0; i < diskCount; i++ {
if reserved[start+i] {
ok = false
break
}
}
if ok {
return start
}
}
}

// parseManagedFstabMounts returns mount points of bloom-managed (non-premounted) fstab entries.
func parseManagedFstabMounts() []string {
	_, entries, err := readBloomFstab()
	if err != nil {
		return nil
	}
	var mounts []string
	for _, entry := range entries {
		if entry.tag == bloomFstabManaged {
			mounts = append(mounts, entry.mountPoint)
		}
	}
	return mounts
}

// discoverAllBloomStorage auto-discovers all bloom-managed storage from fstab
// Returns comma-separated device paths for clusterDisks, premountedDisks, and rancherDisk
func discoverAllBloomStorage() (clusterDisks, premountedDisks, rancherDisk string) {
	_, entries, err := readBloomFstab()
	if err != nil {
		return "", "", ""
	}
	
	var clusterDiskPaths []string
	var premountedPaths []string
	
	for _, entry := range entries {
		switch entry.tag {
		case bloomFstabRancher:
			rancherDisk = resolveSourceToBlockDevice(entry.source)
		case bloomFstabPremounted:
			premountedPaths = append(premountedPaths, entry.mountPoint)
		case bloomFstabManaged:
			if devicePath := resolveSourceToBlockDevice(entry.source); devicePath != "" {
				clusterDiskPaths = append(clusterDiskPaths, devicePath)
			}
		}
	}
	
	clusterDisks = strings.Join(clusterDiskPaths, ",")
	premountedDisks = strings.Join(premountedPaths, ",")
	
	// NEW: Legacy detection if no bloom-managed RANCHER_DISK found
	if rancherDisk == "" {
		if legacyDevice, isLegacy := detectLegacyRancherMount(); isLegacy {
			rancherDisk = resolveDevicePathFromSource(legacyDevice)
			
			// Show warning about legacy mount detection
			fmt.Println("⚠️  LEGACY MOUNT DETECTED:")
			fmt.Printf("   Found manually-mounted /var/lib/rancher → %s\n", legacyDevice)
			fmt.Println("   This mount was created outside of bloom management.")
			fmt.Println("   It will be included in cleanup preview.")
			fmt.Println()
			fmt.Println("💡 Future Guidance:")
			fmt.Printf("   To manage this disk with bloom, add to your bloom.yaml:\n")
			fmt.Printf("   RANCHER_DISK: %s\n", rancherDisk)
			fmt.Println()
		}
	}
	
	return
}

// extractDeviceFromFstabLine extracts the device path from a fstab line
// Handles both UUID= format and direct device paths
func extractDeviceFromFstabLine(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	
	device := fields[0]
	
	// Handle UUID=... format
	if strings.HasPrefix(device, "UUID=") {
		uuid := strings.TrimPrefix(device, "UUID=")
		if devicePath := resolveUUIDToDevice(uuid); devicePath != "" {
			return devicePath
		}
	}
	
	// Return device path directly if it starts with /dev/
	if strings.HasPrefix(device, "/dev/") {
		return device
	}
	
	return ""
}

// resolveUUIDToDevice resolves a UUID to its corresponding device path
func resolveUUIDToDevice(uuid string) string {
	// Use blkid to resolve UUID to device path
	if output, err := exec.Command("blkid", "-U", uuid).Output(); err == nil {
		return strings.TrimSpace(string(output))
	}
	return ""
}

// detectLegacyRancherMount detects manually mounted /var/lib/rancher without bloom tags
// Returns device source and whether it's a legacy manual mount
func detectLegacyRancherMount() (device string, isLegacy bool) {
	// Step 1: Check if /var/lib/rancher is mounted to a device
	output, err := exec.Command("findmnt", "-n", "-o", "SOURCE", "/var/lib/rancher").Output()
	if err != nil {
		return "", false // Not mounted or error
	}
	
	source := strings.TrimSpace(string(output))
	
	// Step 2: Only consider block device mounts (not bind mounts, NFS, etc.)
	if !strings.HasPrefix(source, "/dev/") && !strings.HasPrefix(source, "UUID=") {
		return "", false // Not a block device mount
	}
	
	// Step 3: Check fstab for this mount point to determine if it's bloom-managed
	data, err := os.ReadFile("/etc/fstab")
	if err != nil {
		// Fail closed: inability to inspect fstab must never authorize a wipe.
		return "", false
	}
	
	// Step 4: Check if there's an existing bloom-managed entry
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "/var/lib/rancher" {
			// Check if this entry IS bloom-managed
			if exactBloomFstabTag(line) == bloomFstabRancher {
				return "", false // Found bloom-managed entry, not legacy
			}
		}
	}
	
	// Step 5: If we reach here, we have an active mount but no bloom-managed fstab entry
	// This is either a legacy manual mount or an orphaned mount from previous cleanup
	return source, true // Consider any unmanaged mount as legacy
}

// resolveDevicePathFromSource converts mount source (UUID= or /dev/) to consistent device path
func resolveDevicePathFromSource(source string) string {
	if strings.HasPrefix(source, "UUID=") {
		uuid := strings.TrimPrefix(source, "UUID=")
		if devicePath := resolveUUIDToDevice(uuid); devicePath != "" {
			return devicePath
		}
		fmt.Printf("⚠️  Warning: Could not resolve UUID %s to device path\n", uuid)
		return source // Keep UUID format if resolution fails
	}
	return source // Already a device path
}

// isVarLibRancherMounted checks if /var/lib/rancher is currently mounted
func isVarLibRancherMounted() bool {
	output, err := exec.Command("findmnt", "--mountpoint", "/var/lib/rancher", "--noheadings").Output()
	return err == nil && len(strings.TrimSpace(string(output))) > 0
}

// detectFstabEntryType determines what type of fstab entry exists for /var/lib/rancher
func detectFstabEntryType(mountPoint string) string {
	data, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return "none"
	}
	
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == mountPoint {
			if exactBloomFstabTag(line) == bloomFstabRancher {
				return "bloom-managed"
			}
			return "legacy"
		}
	}
	
	return "none"
}

// getDeviceFromCurrentMount gets device currently/recently mounted at path
func getDeviceFromCurrentMount(mountPath string) string {
	output, err := exec.Command("findmnt", "-n", "-o", "SOURCE", mountPath).Output()
	if err != nil {
		return "" // Not currently mounted
	}
	
	source := strings.TrimSpace(string(output))
	return resolveSourceToBlockDevice(source) // Handle UUID= format
}

// getDeviceFromFstabEntry gets device from fstab entry for mount point
func getDeviceFromFstabEntry(mountPath string) string {
	data, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return ""
	}
	
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == mountPath {
			return resolveSourceToBlockDevice(fields[0]) // Handle UUID= format
		}
	}
	
	return ""
}

// resolveSourceToBlockDevice converts UUID=/dev/ to actual block device path
func resolveSourceToBlockDevice(source string) string {
	if strings.HasPrefix(source, "UUID=") {
		uuid := strings.TrimPrefix(source, "UUID=")
		// Resolve UUID to /dev/sdX device path
		output, err := exec.Command("blkid", "-U", uuid).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(output))
	}
	
	// Already a device path (/dev/sdb2)
	if strings.HasPrefix(source, "/dev/") {
		return source
	}
	
	return ""
}

// isLegacyRancherMount checks if the given device is a legacy manual mount
func isLegacyRancherMount(device string) bool {
	legacyDevice, isLegacy := detectLegacyRancherMount()
	if !isLegacy {
		return false
	}
	// Resolve both to device paths for comparison
	resolvedLegacy := resolveDevicePathFromSource(legacyDevice)
	resolvedDevice := resolveDevicePathFromSource(device)
	return resolvedLegacy == resolvedDevice
}

// shouldAutoDiscover determines if we should auto-discover storage parameters
// Returns true if all storage parameters are empty (no config provided)
func shouldAutoDiscover(clusterDisks, premountedDisks, rancherDisk string) bool {
	return clusterDisks == "" && premountedDisks == "" && rancherDisk == ""
}

// PrintDiskWipePreview prints a preview of bloom-managed mounts to be wiped and
// the future mount range to be pre-cleaned, before the user confirms cleanup.
func PrintDiskWipePreview(clusterDisks, premountedDisks, rancherDisk string) {
	// Storage scope is resolved and validated by ResolveCleanupStorage before
	// preview. Never rediscover here: doing so could add unvalidated legacy
	// mounts or change the scope shown to the operator.
	managed := parseManagedFstabMounts()

	var future []string
	n := countClusterDisksStr(clusterDisks)
	if n > 0 {
		start := calculateFutureDiskStart(clusterDisks, premountedDisks, n)
		for i := 0; i < n; i++ {
			future = append(future, fmt.Sprintf("/mnt/disk%d", start+i))
		}
	}

	if len(managed) == 0 && len(future) == 0 &&
		strings.TrimSpace(premountedDisks) == "" && strings.TrimSpace(rancherDisk) == "" {
		fmt.Println("ℹ️  No strictly Bloom-managed storage discovered; no disks will be wiped or reformatted")
		return
	}

sep := strings.Repeat("─", 62)
fmt.Printf("\n%s\n", sep)
fmt.Println("  ⚠️   DISK CLEANUP PREVIEW")
fmt.Printf("%s\n", sep)

if len(managed) > 0 {
fmt.Println("  Bloom-managed mounts to be WIPED:")
for _, mp := range managed {
bloom, user := inspectDirContents(mp)
switch {
case len(user) > 0:
if len(user) > 5 {
fmt.Printf("    ⚠️  %-18s — %d bloom item(s), ⚠️  %d user file(s) will be LOST\n",
mp, len(bloom), len(user))
} else {
fmt.Printf("    ⚠️  %-18s — %d bloom item(s), ⚠️  %d user file(s) will be LOST: %s\n",
mp, len(bloom), len(user), strings.Join(user, ", "))
}
case len(bloom) > 0:
fmt.Printf("    ✓  %-18s — bloom state only (%d item(s))\n", mp, len(bloom))
default:
fmt.Printf("    ✓  %-18s — empty\n", mp)
}
}
}

if premountedDisks != "" {
var premountedLines []string
for p := range strings.SplitSeq(premountedDisks, ",") {
mp := strings.TrimSpace(p)
if mp == "" {
continue
}
bloom, _ := inspectDirContents(mp)
if len(bloom) > 0 {
premountedLines = append(premountedLines, fmt.Sprintf("    ✓  %-18s — %d bloom artifact(s) removed (filesystem preserved)", mp, len(bloom)))
}
	}
	if len(premountedLines) > 0 {
		fmt.Println("\n  Premounted disks — bloom artifacts cleaned, filesystem kept:")
		for _, line := range premountedLines {
			fmt.Println(line)
		}
	}
}

if rancherDisk != "" {
	// Check if this is a legacy mount for different preview display
	if legacyDevice, isLegacy := detectLegacyRancherMount(); isLegacy && legacyDevice != "" {
		// Legacy-specific preview
		if _, err := os.Stat("/var/lib/rancher"); err == nil {
			bloom, user := inspectDirContents("/var/lib/rancher")
			fmt.Println("\n  RANCHER_DISK — /var/lib/rancher (LEGACY MANUAL MOUNT):")
			switch {
			case len(user) > 0:
				fmt.Printf("    ⚠️  %-18s — manually mounted, will be unmounted\n", "/var/lib/rancher")
				fmt.Printf("    ⚠️  %-18s — fstab entry will be removed\n", rancherDisk)
				if len(user) > 5 {
					fmt.Printf("    ⚠️  %-18s — %d rancher + %d user file(s) will be LOST\n", "RKE2 data", len(bloom), len(user))
				} else {
					fmt.Printf("    ⚠️  %-18s — %d rancher + %d user file(s) will be LOST: %s\n", "RKE2 data", len(bloom), len(user), strings.Join(user, ", "))
				}
			case len(bloom) > 0:
				fmt.Printf("    ⚠️  %-18s — manually mounted, will be unmounted\n", "/var/lib/rancher")
				fmt.Printf("    ⚠️  %-18s — fstab entry will be removed\n", rancherDisk)
				fmt.Printf("    ⚠️  %-18s — %d rancher artifact(s) will be LOST\n", "RKE2 data", len(bloom))
			default:
				fmt.Printf("    ⚠️  %-18s — manually mounted, will be unmounted\n", "/var/lib/rancher")
				fmt.Printf("    ⚠️  %-18s — fstab entry will be removed\n", rancherDisk)
				fmt.Printf("    ✓  %-18s — empty\n", "RKE2 data")
			}
		} else {
			fmt.Println("\n  RANCHER_DISK — /var/lib/rancher (LEGACY MANUAL MOUNT):")
			fmt.Printf("    ⚠️  %-18s — manually mounted, will be unmounted\n", "/var/lib/rancher")
			fmt.Printf("    ⚠️  %-18s — fstab entry will be removed\n", rancherDisk)
		}
	} else {
		// Regular bloom-managed RANCHER_DISK preview
		if _, err := os.Stat("/var/lib/rancher"); err == nil {
			bloom, user := inspectDirContents("/var/lib/rancher")
			fmt.Println("\n  RANCHER_DISK — /var/lib/rancher will be wiped clean:")
			switch {
			case len(user) > 0:
				if len(user) > 5 {
					fmt.Printf("    ⚠️  %-18s — %d rancher item(s), ⚠️  %d user file(s) will be LOST\n",
						"/var/lib/rancher", len(bloom), len(user))
				} else {
					fmt.Printf("    ⚠️  %-18s — %d rancher item(s), ⚠️  %d user file(s) will be LOST: %s\n",
						"/var/lib/rancher", len(bloom), len(user), strings.Join(user, ", "))
				}
			case len(bloom) > 0:
				fmt.Printf("    ✓  %-18s — rancher state only (%d item(s))\n", "/var/lib/rancher", len(bloom))
			default:
				fmt.Printf("    ✓  %-18s — empty\n", "/var/lib/rancher")
			}
		} else {
			fmt.Println("\n  RANCHER_DISK — /var/lib/rancher will be wiped clean:")
			fmt.Printf("    ✓  %-18s — directory will be created\n", "/var/lib/rancher")
		}
	}
}

if len(future) > 0 {
first := future[0]
last := future[len(future)-1]
fmt.Printf("\n  Future mount range (%s – %s): bloom artifacts pre-cleaned, user files preserved\n", first, last)
for _, mp := range future {
bloom, user := inspectDirContents(mp)
if _, err := os.Stat(mp); os.IsNotExist(err) {
fmt.Printf("    ✓  %-18s — will be created\n", mp)
continue
}
if len(bloom) == 0 && len(user) == 0 {
fmt.Printf("    ✓  %-18s — empty\n", mp)
continue
}
parts := []string{}
if len(bloom) > 0 {
parts = append(parts, fmt.Sprintf("%d bloom artifact(s) removed", len(bloom)))
}
if len(user) > 0 {
if len(user) > 5 {
parts = append(parts, fmt.Sprintf("%d user file(s) kept", len(user)))
} else {
parts = append(parts, fmt.Sprintf("%d user file(s) kept: %s", len(user), strings.Join(user, ", ")))
}
}
flag := "✓ "
if len(user) > 0 {
flag = "ℹ️ "
}
fmt.Printf("    %s %-16s — %s\n", flag, mp, strings.Join(parts, "; "))
}
}
fmt.Printf("%s\n\n", sep)
}

// PrecleanFutureMountPoints removes bloom artifacts from directories that will be
// used in the next deployment, preserving all non-bloom user files intact.
func PrecleanFutureMountPoints(clusterDisks, premountedDisks string) error {
n := countClusterDisksStr(clusterDisks)
if n == 0 {
return nil
}
start := calculateFutureDiskStart(clusterDisks, premountedDisks, n)
fmt.Printf("🗂️  Pre-cleaning future mount range /mnt/disk%d–/mnt/disk%d (bloom artifacts only)...\n", start, start+n-1)
blooPatterns := []string{"pvc-*", "replicas", "longhorn-disk.cfg", "longhorn-disk.cfg.tmp"}
for i := 0; i < n; i++ {
mp := fmt.Sprintf("/mnt/disk%d", start+i)
if _, err := os.Stat(mp); os.IsNotExist(err) {
continue
}
for _, pattern := range blooPatterns {
exec.Command("bash", "-c", "rm -rf "+mp+"/"+pattern+" 2>/dev/null").Run()
}
}
fmt.Println("   ✅ Pre-clean complete")
return nil
}


// unmountPriorLonghornDisks helper function to handle fstab cleanup
func unmountPriorLonghornDisks(clusterDisks string) error {
	fstabContent, entries, err := readBloomFstab()
	if err != nil {
		return err
	}

	configuredIDs, err := deviceIDs(splitCleanupValues(clusterDisks))
	if err != nil {
		return err
	}
	managedLines := map[string]string{}
	for _, entry := range entries {
		if entry.tag == bloomFstabManaged {
			_, id, err := resolveBlockDevice(entry.source)
			if err != nil {
				return err
			}
			if _, validated := configuredIDs[id]; validated {
				managedLines[entry.raw] = entry.mountPoint
			}
		}
	}

	for _, entry := range entries {
		if entry.tag != bloomFstabManaged {
			continue
		}
		if _, validated := managedLines[entry.raw]; !validated {
			continue
		}
		if exactMountSource(entry.mountPoint) != "" {
			fmt.Printf("      ⏏️  Unmounting bloom-managed mount: %s\n", entry.mountPoint)
			if output, err := exec.Command("sudo", "umount", "-lf", entry.mountPoint).CombinedOutput(); err != nil {
				return fmt.Errorf("unmount %s before fstab update: %w (%s)",
					entry.mountPoint, err, strings.TrimSpace(string(output)))
			}
		}
	}

	var retainedLines []string
	for _, line := range strings.Split(fstabContent, "\n") {
		if _, remove := managedLines[line]; !remove {
			retainedLines = append(retainedLines, line)
		}
	}
	return backupAndWriteFstab(fstabContent, retainedLines)
}
