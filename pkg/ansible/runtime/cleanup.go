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

	// Check if uninstall script exists
	if _, err := os.Stat("/usr/local/bin/rke2-uninstall.sh"); os.IsNotExist(err) {
		fmt.Println("   RKE2 uninstall script not found, skipping")
		return nil
	}

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

	// Sync filesystem to ensure all writes are flushed
	fmt.Println("   Syncing filesystems...")
	exec.Command("sync").Run()

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

	fmt.Println("   Unmounting CLUSTER_DISKS devices...")
	devices := strings.Split(clusterDisks, ",")
	for _, device := range devices {
		device = strings.TrimSpace(device)
		if device == "" {
			continue
		}

		// Force unmount the device
		fmt.Printf("   Unmounting %s...\n", device)
		cmd := exec.Command("sudo", "umount", "-lf", device)
		if output, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("   Warning: Failed to unmount %s: %v (output: %s)\n", device, err, string(output))
		} else {
			fmt.Printf("   Successfully unmounted %s\n", device)
		}

		// Also try to unmount any mount points using this device
		cmd = exec.Command("bash", "-c", fmt.Sprintf("mount | grep '^%s' | awk '{print $3}' | xargs -r sudo umount -lf 2>/dev/null || true", device))
		cmd.Run()
	}

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
