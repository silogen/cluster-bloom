//go:build linux

package runtime

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// CleanupLonghornMounts performs cleanup of Longhorn PVCs and mounts.
// Sequences: graceful kubectl drain → iSCSI logout → TERM/KILL processes → force umount.
// This ordering is required because Longhorn uses iSCSI sessions that remain
// in the kernel even after the Longhorn process is killed; skipping the iSCSI
// logout leaves the device busy and causes rm/umount to block or silently fail.
func CleanupLonghornMounts() error {
	fmt.Println("💾 Cleaning Longhorn mounts and PVCs...")

	// Step 1: Graceful kubectl drain (best-effort, cluster may already be down)
	fmt.Println("   Attempting graceful node drain via kubectl...")
	nodeNameOut, _ := exec.Command("hostname").Output()
	nodeName := strings.TrimSpace(string(nodeNameOut))
	kubeconfig := "/etc/rancher/rke2/rke2.yaml"
	_, kubeconfigErr := os.Stat(kubeconfig)
	apiReachable := false
	if kubeconfigErr == nil {
		fmt.Print("   Checking Kubernetes API server reachability... ")
		apiReachable = isKubeAPIReachable()
		if apiReachable {
			fmt.Println("reachable")
		} else {
			fmt.Println("unreachable")
		}
	}
	nodeInCluster := false
	if apiReachable && nodeName != "" {
		fmt.Printf("   Checking if %s is a member of the cluster... ", nodeName)
		nodeInCluster = isNodeInCluster(kubeconfig, nodeName)
		if nodeInCluster {
			fmt.Println("yes")
		} else {
			fmt.Println("no")
		}
	}
	if kubeconfigErr == nil && nodeName != "" && apiReachable && nodeInCluster {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig,
			"cordon", nodeName).Run()
		drainCtx, drainCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer drainCancel()
		exec.CommandContext(drainCtx, "kubectl", "--kubeconfig", kubeconfig,
			"drain", nodeName,
			"--delete-emptydir-data", "--ignore-daemonsets",
			"--grace-period=15", "--timeout=45s").Run()
		// Wait briefly for Longhorn to detach volumes
		fmt.Println("   Waiting for Longhorn volumes to detach...")
		for i := 0; i < 30; i++ {
			out, _ := exec.Command("bash", "-c", "ls /dev/longhorn/ 2>/dev/null | wc -l").Output()
			if strings.TrimSpace(string(out)) == "0" {
				break
			}
			time.Sleep(2 * time.Second)
		}
	} else if kubeconfigErr != nil {
		fmt.Println("   kubectl/kubeconfig not available — skipping drain")
	} else if !apiReachable {
		fmt.Println("   Kubernetes API server unreachable — skipping drain")
	} else {
		fmt.Printf("   Node %s is not a member of this cluster — skipping drain\n", nodeName)
	}

	// Step 2: iSCSI logout — releases kernel block device mappings for Longhorn volumes.
	// Must happen before umount; without this the device remains busy regardless of
	// whether the Longhorn process is alive.
	fmt.Println("   Logging out iSCSI sessions...")
	exec.Command("iscsiadm", "-m", "session", "--logout").Run()
	exec.Command("iscsiadm", "-m", "node", "--op=delete").Run()

	// Step 3: Graceful TERM then KILL of Longhorn processes in dependency order
	longhornProcs := []string{"longhorn-engine", "longhorn-instance-manager", "longhorn-manager"}
	anyRunning := false
	for _, proc := range longhornProcs {
		if exec.Command("pgrep", "-f", proc).Run() == nil {
			anyRunning = true
			break
		}
	}
	if anyRunning {
		fmt.Println("   Stopping Longhorn processes (TERM)...")
		for _, proc := range longhornProcs {
			exec.Command("pkill", "-TERM", "-f", proc).Run()
		}
		time.Sleep(5 * time.Second)
		// Only KILL if some processes survived TERM
		stillRunning := false
		for _, proc := range longhornProcs {
			if exec.Command("pgrep", "-f", proc).Run() == nil {
				stillRunning = true
				break
			}
		}
		if stillRunning {
			fmt.Println("   Force killing remaining Longhorn processes (KILL)...")
			for _, proc := range longhornProcs {
				exec.Command("pkill", "-KILL", "-f", proc).Run()
			}
		}
	} else {
		fmt.Println("   No Longhorn processes running — skipping")
	}

	// Step 4: Force umount everything Longhorn-related
	fmt.Println("   Unmounting Longhorn volumes...")
	for attempt := 1; attempt <= 3; attempt++ {
		fmt.Printf("   Attempt %d/3...\n", attempt)
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

	fmt.Println("   Longhorn cleanup completed")
	return nil
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
		fmt.Println("   Executing RKE2 uninstall script (may take a couple minutes)...")
		cmd := exec.Command("/usr/local/bin/rke2-uninstall.sh")
		output, err := cmd.CombinedOutput()

		// Log output regardless of error (matching Bloom v1 behavior)
		if len(output) > 0 {
			fmt.Printf("   RKE2 uninstall script output: %s\n", string(output))
		}

		if err != nil {
			fmt.Printf("   RKE2 uninstall script encountered warnings: %v\n", err)
			// Don't return error - Bloom v1 continues on uninstall script errors
		} else {
			fmt.Println("   RKE2 uninstall script executed successfully")
		}
	} else {
		fmt.Println("   RKE2 uninstall script not found")
	}

	// Always force-remove RKE2 directories to ensure clean state
	// This handles cases where the uninstall script doesn't exist, fails, or leaves remnants
	fmt.Println("   Removing RKE2 directories and data...")
	directories := []string{
		"/etc/rancher/rke2",
		"/var/lib/rancher/rke2",
		"/var/lib/kubelet",
	}

	for _, dir := range directories {
		if _, err := os.Stat(dir); err == nil {
			cmd := exec.Command("rm", "-rf", dir)
			if err := cmd.Run(); err != nil {
				fmt.Printf("   Warning: Failed to remove %s: %v\n", dir, err)
			} else {
				fmt.Printf("   Removed %s\n", dir)
			}
		}
	}

	return nil
}

// CleanupBloomDisks removes bloom-managed disks and cleans up disk state
func CleanupBloomDisks(clusterDisks string) error {
	fmt.Println("💽 Cleaning bloom-managed disks...")

	// First unmount prior Longhorn disks (equivalent to UnmountPriorLonghornDisks)
	if err := unmountPriorLonghornDisks(); err != nil {
		fmt.Printf("   Warning: Failed to unmount prior Longhorn disks: %v\n", err)
	}

	// Directly unmount all CLUSTER_DISKS devices if they're mounted
	if err := unmountClusterDisks(clusterDisks); err != nil {
		fmt.Printf("   Warning: Failed to unmount CLUSTER_DISKS: %v\n", err)
	}

	// Parse mount output to find and unmount CSI driver mounts
	fmt.Println("   Checking for CSI driver mounts...")
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
				fmt.Printf("   Warning: Failed to unmount %s\n", fields[2])
			} else {
				fmt.Printf("   Unmounted %s\n", fields[2])
			}
		}
	}

	// Use lsblk to find and wipe devices with Longhorn CSI mounts
	fmt.Println("   Checking for devices to wipe...")
	cmd = exec.Command("lsblk", "-o", "NAME,MOUNTPOINT")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("lsblk command failed: %w", err)
	}

	scanner = bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 1 && strings.Contains(fields[1], "kubernetes.io/csi/driver.longhorn.io") {
			device := "/dev/" + fields[0]
			_, err := exec.Command("sudo", "wipefs", "-a", device).CombinedOutput()
			if err != nil {
				fmt.Printf("   Warning: Failed to wipe %s\n", device)
			} else {
				fmt.Printf("   Wiped %s\n", device)
			}
		}
	}

	// Remove longhorn plugins directory
	_, err = exec.Command("sudo", "rm", "-rf", "/var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/*").CombinedOutput()
	if err != nil {
		fmt.Printf("   Warning: Failed to remove longhorn plugins directory: %v\n", err)
	}

	// Delete unmounted disk devices (matching Bloom v1 logic)
	fmt.Println("   Checking for unmounted disks to delete...")
	cmd = exec.Command("lsblk", "-nd", "-o", "NAME,TYPE,MOUNTPOINT")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("lsblk command failed: %w", err)
	}

	scanner = bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 3 && strings.HasPrefix(fields[0], "sd") && fields[1] == "disk" && fields[2] == "" {
			deleteCmd := exec.Command("sudo", "tee", "/sys/block/"+fields[0]+"/device/delete")
			deleteCmd.Stdin = strings.NewReader("1\n")
			if err := deleteCmd.Run(); err != nil {
				fmt.Printf("   Warning: Failed to delete /dev/%s\n", fields[0])
			} else {
				fmt.Printf("   Deleted /dev/%s\n", fields[0])
			}
		}
	}

	// Skip filesystem sync as it commonly hangs on systems with I/O issues
	// The 500ms delay below is sufficient for kernel to release mounts
	fmt.Println("   Allowing kernel to flush pending I/O...")

	// Brief delay to allow kernel to fully release mounts
	time.Sleep(500 * time.Millisecond)

	fmt.Println("   Disk cleanup completed")
	return nil
}

// unmountClusterDisks directly unmounts all devices found in CLUSTER_DISKS
func unmountClusterDisks(clusterDisks string) error {
	if clusterDisks == "" {
		return nil
	}

	devices := strings.Split(clusterDisks, ",")
	fmt.Printf("   Unmounting cluster disks: %s\n", clusterDisks)

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
			fmt.Printf("   Warning: Failed to unmount %s: %v\n", device, err)
		} else {
			fmt.Printf("   Successfully unmounted %s\n", device)
		}
	}

	return nil
}

// GenerateCleanupTasks creates Ansible tasks equivalent to the cleanup functions above
func GenerateCleanupTasks(clusterDisks string, premountedDisks string) []map[string]any {
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
						"• ALL managed disk devices (wipefs + deletion)",
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
				"name":        "Drain node (evicts pods so Longhorn detaches volumes gracefully)",
				"shell":       "/var/lib/rancher/rke2/bin/kubectl --kubeconfig /etc/rancher/rke2/rke2.yaml drain {{ cleanup_hostname.stdout }} --delete-emptydir-data --ignore-daemonsets --grace-period=30 --timeout=90s 2>/dev/null || true",
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
				"name":        "Logout iSCSI sessions (releases Longhorn kernel block devices)",
				"shell":       "iscsiadm -m session --logout 2>/dev/null || true; iscsiadm -m node --op=delete 2>/dev/null || true",
				"failed_when": false,
			},
			// Step 3: Stop Longhorn processes gracefully in dependency order
			{
				"name":        "Gracefully stop Longhorn processes (TERM)",
				"shell":       "pkill -TERM -f longhorn-engine 2>/dev/null || true; pkill -TERM -f longhorn-instance-manager 2>/dev/null || true; pkill -TERM -f longhorn-manager 2>/dev/null || true",
				"failed_when": false,
			},
			{
				"name":         "Wait for Longhorn processes to stop",
				"shell":        "sleep 5",
				"changed_when": false,
			},
			{
				"name":        "Force kill remaining Longhorn processes (KILL)",
				"shell":       "pkill -KILL -f longhorn-engine 2>/dev/null || true; pkill -KILL -f longhorn-instance-manager 2>/dev/null || true; pkill -KILL -f longhorn-manager 2>/dev/null || true",
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
					"name":        "Wipe filesystem signatures from cluster disks",
					"shell":       "wipefs -a {{ item }} 2>/dev/null || true",
					"loop":        "{{ cluster_disks_cleanup_list }}",
					"failed_when": false,
				},
				{
					"name":        "Pre-clean bloom artifacts from future mount point dirs (preserve user files)",
					"shell":       fmt.Sprintf(`n=$(echo '%s' | tr ',' '
' | grep -c '.'); reserved=$({ grep '# premounted by cluster-bloom' /etc/fstab 2>/dev/null | awk '{print $2}' | sed 's|.*/disk||'; printf '%%s' '{{ CLUSTER_PREMOUNTED_DISKS }}' | tr ',' '
' | sed 's/[[:space:]]//g;s|.*/disk||'; } | grep -E '^[0-9]+$' | sort -un | tr '
' ' '); start=0; while true; do conflict=0; i=0; while [ $i -lt $n ]; do idx=$((start+i)); for r in $reserved; do [ "$idx" = "$r" ] && conflict=1 && break; done; [ $conflict -eq 1 ] && break; i=$((i+1)); done; [ $conflict -eq 0 ] && break; start=$((start+1)); done; i=0; while [ $i -lt $n ]; do mp="/mnt/disk$((start+i))"; [ -d "$mp" ] && rm -rf "$mp"/pvc-* "$mp"/replicas "$mp"/longhorn-disk.cfg "$mp"/longhorn-disk.cfg.tmp 2>/dev/null || true; i=$((i+1)); done`, clusterDisks),
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
	mountPoints := strings.Split(premountedDisks, ",")
	for _, mp := range mountPoints {
		mp = strings.TrimSpace(mp)
		if mp == "" {
			continue
		}
		// Ensure it is mounted before we try to clean it
		if _, err := exec.Command("mountpoint", "-q", mp).CombinedOutput(); err != nil {
			fmt.Printf("   Mounting %s before cleanup...\n", mp)
			if _, err2 := exec.Command("mount", mp).CombinedOutput(); err2 != nil {
				fmt.Printf("   Warning: Could not mount %s (skipping): %v\n", mp, err2)
				continue
			}
		}
		// Verify no iSCSI sessions are still holding pvc-* devices within this mountpoint.
		// If any remain the remove will block; force-unmount the sub-paths first.
		exec.Command("bash", "-c",
			fmt.Sprintf(`for d in %s/pvc-*; do umount -lf "$d" 2>/dev/null || true; done`, mp)).Run()
		// Remove PVC dirs and Longhorn disk state; keep the ext4 filesystem intact
		patterns := []string{
			mp + "/pvc-*",
			mp + "/replicas",
			mp + "/longhorn-disk.cfg",
			mp + "/longhorn-disk.cfg.tmp",
		}
		for _, pattern := range patterns {
			exec.Command("bash", "-c", "rm -rf "+pattern+" 2>/dev/null").Run()
		}
		fmt.Printf("   Cleaned contents of %s\n", mp)
	}
	fmt.Println("   Premounted disk cleanup completed")
	return nil
}

// --- Disk index helpers ---

// extractDiskIndex extracts the integer N from a /mnt/diskN path.
func extractDiskIndex(mountPoint string) (int, error) {
mp := strings.TrimSpace(mountPoint)
const prefix = "/mnt/disk"
if !strings.HasPrefix(mp, prefix) {
return 0, fmt.Errorf("not a /mnt/diskN path: %s", mp)
}
return strconv.Atoi(mp[len(prefix):])
}

// isDiskBloomArtifact reports whether the named entry is a Longhorn/bloom artifact.
func isDiskBloomArtifact(name string) bool {
return strings.HasPrefix(name, "pvc-") ||
name == "replicas" ||
name == "longhorn-disk.cfg" ||
name == "longhorn-disk.cfg.tmp"
}

// inspectDirContents returns bloom artifacts and user files found directly inside dir.
func inspectDirContents(dir string) (bloom []string, user []string) {
entries, err := os.ReadDir(dir)
if err != nil {
return
}
for _, e := range entries {
if isDiskBloomArtifact(e.Name()) {
bloom = append(bloom, e.Name())
} else {
user = append(user, e.Name())
}
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
// range [S, S+diskCount) does not overlap reserved indexes. Reserved indexes come from
// /etc/fstab premounted-by-cluster-bloom entries and the CLUSTER_PREMOUNTED_DISKS string.
func calculateFutureDiskStart(premountedDisks string, diskCount int) int {
reserved := map[int]bool{}

// From /etc/fstab premounted-by-cluster-bloom entries
if data, err := os.ReadFile("/etc/fstab"); err == nil {
for _, line := range strings.Split(string(data), "\n") {
if strings.Contains(line, "# premounted by cluster-bloom") {
fields := strings.Fields(line)
if len(fields) >= 2 {
if idx, err2 := extractDiskIndex(fields[1]); err2 == nil {
reserved[idx] = true
}
}
}
}
}
// From CLUSTER_PREMOUNTED_DISKS config string
for _, p := range strings.Split(premountedDisks, ",") {
if idx, err := extractDiskIndex(strings.TrimSpace(p)); err == nil {
reserved[idx] = true
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
data, err := os.ReadFile("/etc/fstab")
if err != nil {
return nil
}
var mounts []string
for _, line := range strings.Split(string(data), "\n") {
if strings.Contains(line, "# managed by cluster-bloom") &&
!strings.Contains(line, "# premounted by cluster-bloom") {
fields := strings.Fields(line)
if len(fields) >= 2 {
mounts = append(mounts, fields[1])
}
}
}
return mounts
}

// PrintDiskWipePreview prints a preview of bloom-managed mounts to be wiped and
// the future mount range to be pre-cleaned, before the user confirms cleanup.
func PrintDiskWipePreview(clusterDisks, premountedDisks string) {
managed := parseManagedFstabMounts()

var future []string
n := countClusterDisksStr(clusterDisks)
if n > 0 {
start := calculateFutureDiskStart(premountedDisks, n)
for i := 0; i < n; i++ {
future = append(future, fmt.Sprintf("/mnt/disk%d", start+i))
}
}

if len(managed) == 0 && len(future) == 0 {
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
start := calculateFutureDiskStart(premountedDisks, n)
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
fmt.Println("   Pre-clean complete")
return nil
}


// unmountPriorLonghornDisks helper function to handle fstab cleanup
func unmountPriorLonghornDisks() error {
	// Read fstab to find bloom-managed entries
	fstabContent, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return fmt.Errorf("failed to read fstab: %w", err)
	}

	// Create backup
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("/etc/fstab.bak-%s", timestamp)
	if err := os.WriteFile(backupPath, fstabContent, 0644); err != nil {
		fmt.Printf("   Warning: Failed to backup fstab: %v\n", err)
	} else {
		fmt.Printf("   Created fstab backup: %s\n", backupPath)
	}

	// Process fstab lines
	lines := strings.Split(string(fstabContent), "\n")
	var cleanLines []string

	for _, line := range lines {
		// Only remove entries tagged "# managed by cluster-bloom" (CLUSTER_DISKS).
		// Entries tagged "# premounted by cluster-bloom" (CLUSTER_PREMOUNTED_DISKS) are
		// intentionally skipped — premounted disks survive cleanup with filesystem intact.
		if strings.Contains(line, "# managed by cluster-bloom") && !strings.Contains(line, "# premounted by cluster-bloom") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				mountPoint := fields[1]
				fmt.Printf("   Unmounting bloom-managed mount: %s\n", mountPoint)
				exec.Command("sudo", "umount", "-lf", mountPoint).Run()
			}
			// Don't add this line to cleanLines (removes it from fstab)
		} else {
			cleanLines = append(cleanLines, line)
		}
	}

	// Write cleaned fstab
	cleanFstab := strings.Join(cleanLines, "\n")
	if err := os.WriteFile("/etc/fstab", []byte(cleanFstab), 0644); err != nil {
		return fmt.Errorf("failed to update fstab: %w", err)
	}

	return nil
}
