//go:build linux

package runtime

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// CleanupLonghornMounts performs cleanup of Longhorn PVCs and mounts
func CleanupLonghornMounts() error {
	fmt.Println("💾 Cleaning Longhorn mounts and PVCs...")

	// Stop Longhorn services first if they exist
	fmt.Println("   Stopping Longhorn services...")
	cmd := exec.Command("sudo", "systemctl", "stop", "longhorn-*")
	if err := cmd.Run(); err != nil {
		fmt.Printf("   Warning: Failed to stop Longhorn services: %v\n", err)
	}

	// Find and unmount all Longhorn-related mounts (3 retries like Bloom v1)
	fmt.Println("   Unmounting Longhorn volumes...")
	for attempt := 1; attempt <= 3; attempt++ {
		fmt.Printf("   Attempt %d/3...\n", attempt)

		// Unmount Longhorn device files
		exec.Command("sudo", "umount", "-lf", "/dev/longhorn/pvc-*").Run()

		// Find and unmount CSI volume mounts
		exec.Command("bash", "-c", "sudo umount -Af /var/lib/kubelet/pods/*/volumes/kubernetes.io~csi/pvc-* 2>/dev/null || true").Run()
		exec.Command("bash", "-c", "sudo umount -Af /var/lib/kubelet/pods/*/volumes/kubernetes.io~csi/*/mount 2>/dev/null || true").Run()

		// Find and unmount CSI plugin mounts
		exec.Command("bash", "-c", "mount | grep 'driver.longhorn.io' | awk '{print $3}' | xargs -r sudo umount -lf 2>/dev/null || true").Run()
		exec.Command("bash", "-c", "sudo umount -Af /var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/* 2>/dev/null || true").Run()
	}

	// Force kill any processes using Longhorn mounts
	fmt.Println("   Killing processes using Longhorn mounts...")
	exec.Command("sudo", "fuser", "-km", "/dev/longhorn/").Run()

	// Clean up device files
	fmt.Println("   Cleaning up device files...")
	exec.Command("sudo", "rm", "-rf", "/dev/longhorn/pvc-*").Run()

	// Clean up kubelet CSI mounts
	exec.Command("sudo", "rm", "-rf", "/var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/*").Run()

	fmt.Println("   Longhorn cleanup completed")
	return nil
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
			// Longhorn cleanup tasks (based on CleanupLonghornMounts)
			{
				"name": "Stop and disable RKE2 services",
				"systemd": map[string]any{
					"name":    "{{ item }}",
					"state":   "stopped",
					"enabled": false,
				},
				"loop": []string{"rke2-server", "rke2-agent"},
				"failed_when": false,
			},
			{
				"name": "Clean Longhorn mounts and processes",
				"shell": "pkill -f longhorn || true; for mount in $(mount | grep longhorn | awk '{print $3}'); do umount \"$mount\" 2>/dev/null || true; done; rm -rf /var/lib/longhorn/* 2>/dev/null || true; echo 'Longhorn cleanup completed'",
				"register": "longhorn_cleanup",
				"failed_when": false,
			},
			// RKE2 cleanup tasks (based on UninstallRKE2)
			{
				"name": "Run RKE2 uninstall script",
				"shell": "/usr/local/bin/rke2-uninstall.sh",
				"register": "rke2_uninstall",
				"failed_when": false,
			},
			{
				"name": "Clean RKE2 directories and files",
				"shell": "rm -rf /var/lib/rancher/rke2 /etc/rancher/rke2 /var/lib/kubelet /var/log/pods /var/log/containers; rm -f /usr/local/bin/rke2* /usr/local/bin/kubectl /usr/local/bin/crictl /usr/local/bin/ctr; echo 'RKE2 cleanup completed'",
				"register": "rke2_cleanup",
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
						"cluster_disks_cleanup_list": fmt.Sprintf("{{ '%s'.split(',') }}", clusterDisks),
					},
				},
				{
					"name": "Unmount cluster disks",
					"shell": "umount {{ item }} 2>/dev/null || true",
					"loop": "{{ cluster_disks_cleanup_list }}",
					"failed_when": false,
				},
				{
					"name": "Remove fstab entries for cluster disks",
					"lineinfile": map[string]any{
						"path":   "/etc/fstab",
						"regexp": "{{ item | regex_escape }}",
						"state":  "absent",
					},
					"loop": "{{ cluster_disks_cleanup_list }}",
					"failed_when": false,
				},
				{
					"name": "Wipe filesystem signatures from cluster disks",
					"shell": "wipefs -a {{ item }} 2>/dev/null || true",
					"loop": "{{ cluster_disks_cleanup_list }}",
					"failed_when": false,
				},
				{
					"name": "Remove mount point directories",
					"file": map[string]any{
						"path":  "/mnt/disk{{ ansible_loop.index0 }}",
						"state": "absent",
					},
					"loop": "{{ cluster_disks_cleanup_list }}",
					"loop_control": map[string]any{
						"extended": true,
					},
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
					"shell":       "rm -rf {{ item }}/pvc-* {{ item }}/longhorn-disk.cfg {{ item }}/longhorn-disk.cfg.tmp 2>/dev/null; echo 'cleaned {{ item }}'",
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
		// Remove PVC dirs and Longhorn disk state; keep the ext4 filesystem intact
		patterns := []string{
			mp + "/pvc-*",
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
		if strings.Contains(line, "# managed by cluster-bloom") {
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
