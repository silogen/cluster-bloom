/**
 * Copyright 2025 Advanced Micro Devices, Inc.  All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
**/
package pkg

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/viper"
)

func CleanDisks() error {
	LogMessage(Info, "Disks cleanup started.")
	
	// First, try to clean prior Longhorn target disks
	disks, err := GetPriorLonghornDisks()
	if err != nil {
		LogMessage(Warn, fmt.Sprintf("Failed to get prior Longhorn disks: %v", err))
	} else if disks != nil && len(disks) > 0 {
		LogMessage(Info, "Cleaning prior Longhorn target disks...")
		if err := CleanTargetDisks(disks); err != nil {
			LogMessage(Warn, fmt.Sprintf("Failed to clean target disks: %v", err))
		}
	} else {
		LogMessage(Info, "No prior Longhorn disks found to clean.")
	}
	
	// Continue with existing cleanup logic
	cmd := exec.Command("mount")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("mount command failed: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 2 && strings.Contains(fields[2], "kubernetes.io/csi/driver.longhorn.io") {
			_, err := runCommand("sudo", "umount", "-lf", fields[2])
			if err != nil {
				LogMessage(Warn, fmt.Sprintf("Failed to unmount %s", fields[2]))
			} else {
				LogMessage(Info, fmt.Sprintf("Unmounted %s", fields[2]))
			}
		}
	}

	cmd = exec.Command("lsblk", "-o", "NAME,MOUNTPOINT")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("lsblk command failed: %w", err)
	}

	scanner = bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 1 && strings.Contains(fields[1], "kubernetes.io/csi/driver.longhorn.io") {
			dev := "/dev/" + fields[0]
			_, err := runCommand("sudo", "wipefs", "-a", dev)
			if err != nil {
				LogMessage(Warn, fmt.Sprintf("Failed to wipe %s", dev))
			} else {
				LogMessage(Info, fmt.Sprintf("Wiped %s", dev))
			}
		}
	}

	_, err = runCommand("sudo", "rm", "-rf", "/var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/*")
	if err != nil {
		LogMessage(Warn, fmt.Sprintf("Failed to remove longhorn plugins directory: %v", err))
	}

	cmd = exec.Command("lsblk", "-nd", "-o", "NAME,TYPE,MOUNTPOINT")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("lsblk command failed: %w", err)
	}

	scanner = bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 3 && strings.HasPrefix(fields[0], "sd") && fields[1] == "disk" && fields[2] == "" {
			echoCmd := exec.Command("sudo", "tee", "/sys/block/"+fields[0]+"/device/delete")
			echoCmd.Stdin = strings.NewReader("1\n")
			if err := echoCmd.Run(); err != nil {
				LogMessage(Warn, fmt.Sprintf("Failed to delete /dev/%s", fields[0]))
			} else {
				LogMessage(Info, fmt.Sprintf("Deleted /dev/%s", fields[0]))
			}
		}
	}
	LogMessage(Info, "Disk cleanup completed.")
	return nil
}

var nodeLabelTemplate = `
node-label:
  - cluster-bloom/gpu-node=%t
`
var longhornDiskTemplate = `
  - node.longhorn.io/create-default-disk=config
  - node.longhorn.io/instance-manager=true
  - silogen.ai/longhorndisks=%s
`

func ParseLonghornDiskConfig() string {
	disks := strings.Split(viper.GetString("LONGHORN_DISKS"), ",")
	diskList := strings.Join(disks, "xxx")
	return diskList
}

func GenerateNodeLabels() error {
	rke2ConfigPath := "/etc/rancher/rke2/config.yaml"
	// Fill the template with the GPU_NODE setting, leave longhor for later
	nodeLabels := fmt.Sprintf(nodeLabelTemplate, viper.GetBool("GPU_NODE"))
	if err := appendToFile(rke2ConfigPath, nodeLabels); err != nil {
		return fmt.Errorf("failed to append Longhorn configuration to %s: %w", rke2ConfigPath, err)
	}

	if viper.IsSet("LONGHORN_DISKS") && viper.GetString("LONGHORN_DISKS") != "" {
		LogMessage(Info, "Using LONGHORN_DISKS for Longhorn configuration.")
		diskList := ParseLonghornDiskConfig()
		configContent := fmt.Sprintf(longhornDiskTemplate, diskList)
		if err := appendToFile(rke2ConfigPath, configContent); err != nil {
			return fmt.Errorf("failed to append Longhorn configuration to %s: %w", rke2ConfigPath, err)
		}
		LogMessage(Info, "Appended Longhorn disk configuration to RKE2 config.")
		return nil
	}
	if viper.GetBool("SKIP_DISK_CHECK") == true {
		LogMessage(Info, "Skipping GenerateLonghornDiskString as SKIP_DISK_CHECK is set.")
		return nil
	}
	selectedDisks := viper.GetStringSlice("selected_disks")
	if len(selectedDisks) == 0 {
		LogMessage(Info, "No disks selected for mounting, skipping")
		return nil
	}

	cmd := exec.Command("sh", "-c", "mount | grep -oP '/mnt/disk\\d+'")
	output, err := cmd.CombinedOutput()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to list mounted disks: %v", err))
		return fmt.Errorf("failed to list mounted disks: %w", err)
	}

	disks := strings.Fields(string(output))
	if len(disks) == 0 {
		LogMessage(Info, "No /mnt/disk{x} drives found.")
		return nil
	}
	diskNames := []string{}
	// # Check if GPU_NODE is set or no disks are selected
	// if viper.GetBool("GPU_NODE") || !selectedDisks {
	// 	for _, disk := range disks {
	// 		cmd := exec.Command("sh", "-c", fmt.Sprintf("lsblk -no NAME,MOUNTPOINT | grep '%s' | grep 'nvme'", disk))
	// 		if err := cmd.Run(); err == nil {
	// 			diskNames = append(diskNames, strings.TrimPrefix(disk, "/mnt/"))
	// 		}
	// 	}
	// } else {
	for _, disk := range disks {
		diskNames = append(diskNames, strings.TrimPrefix(disk, "/mnt/"))
	}
	// }

	if len(diskNames) > 0 {
		diskList := strings.Join(diskNames, "xxx")

		configContent := fmt.Sprintf(longhornDiskTemplate, diskList)

		if err := appendToFile(rke2ConfigPath, configContent); err != nil {
			return fmt.Errorf("failed to append Longhorn configuration to %s: %w", rke2ConfigPath, err)
		}
		LogMessage(Info, "Appended Longhorn disk configuration to RKE2 config.")
	}
	return nil
}

func appendToFile(filePath, content string) error {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("failed to write to file %s: %w", filePath, err)
	}
	return nil
}

func isVirtualDisk(udevOut []byte) bool {
	virtualMarkers := []string{
		"ID_VENDOR=QEMU",
		"ID_VENDOR=Virtio",
		"ID_VENDOR=VMware",
		"ID_VENDOR=Virtual",
		"ID_VENDOR=Microsoft",
		"ID_MODEL=VIRTUAL-DISK",
		"SCSI_MODEL=VIRTUAL-DISK",
	}

	for _, marker := range virtualMarkers {
		if bytes.Contains(udevOut, []byte(marker)) {
			return true
		}
	}
	return false
}

func GetUnmountedPhysicalDisks() ([]string, error) {
	if viper.GetBool("SKIP_DISK_CHECK") == true {
		LogMessage(Info, "Skipping disk check as SKIP_DISK_CHECK is set.")
		return nil, nil
	}
	var result []string

	lsblkCmd := exec.Command("lsblk", "-dn", "-o", "NAME,TYPE")
	out, err := lsblkCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("lsblk failed: %w", err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || fields[1] != "disk" {
			continue
		}
		name := fields[0]
		if !strings.HasPrefix(name, "nvme") && !strings.HasPrefix(name, "sd") {
			continue
		}
		devPath := "/dev/" + name
		mountCheck := exec.Command("lsblk", "-no", "MOUNTPOINT", devPath)
		mountOut, err := mountCheck.Output()
		if err != nil {
			continue
		}
		if strings.Contains(string(mountOut), "/") {
			continue
		}
		if strings.HasPrefix(name, "sd") {
			udevCmd := exec.Command("udevadm", "info", "--query=property", "--name", devPath)
			udevOut, err := udevCmd.Output()
			if err != nil {
				continue
			}
			if isVirtualDisk(udevOut) {
				continue
			}
		}

		result = append(result, devPath)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner error: %w", err)
	}
	return result, nil
}
func MountDrives(drives []string) error {
	if viper.IsSet("LONGHORN_DISKS") && viper.GetString("LONGHORN_DISKS") != "" {
		LogMessage(Info, "Skipping drive mounting as LONGHORN_DISKS is set.")
		return nil
	}
	if viper.GetBool("SKIP_DISK_CHECK") == true {
		LogMessage(Info, "Skipping drive mounting as SKIP_DISK_CHECK is set.")
		return nil
	}

	usedMountPoints := make(map[string]bool)
	i := 0
	cmd := exec.Command("sh", "-c", "mount | awk '/\\/mnt\\/disk[0-9]+/ {print $3}'")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list existing mount points: %w", err)
	}
	existingMountPoints := strings.Fields(string(output))
	for _, mountPoint := range existingMountPoints {
		usedMountPoints[mountPoint] = true
	}
	fstabContent, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return fmt.Errorf("failed to read /etc/fstab: %w", err)
	}

	for _, drive := range drives {
		cmd = exec.Command("lsblk", "-f", drive)
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to check filesystem type for %s: %w", drive, err)
		}
		if strings.Contains(string(output), "ext4") {
			LogMessage(Info, fmt.Sprintf("Disk %s is already formatted as ext4. Skipping format.", drive))
		} else {
			cmd = exec.Command("lsblk", "-no", "PARTTYPE", drive)
			output, err = cmd.Output()
			if err != nil {
				return fmt.Errorf("failed to check partition type for %s: %w", drive, err)
			}
			if strings.TrimSpace(string(output)) != "" {
				LogMessage(Info, fmt.Sprintf("Disk %s has existing partitions. Removing partitions...", drive))
				cmd = exec.Command("sudo", "wipefs", "-a", drive)
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("failed to wipe partitions on %s: %w", drive, err)
				}
			}

			LogMessage(Info, fmt.Sprintf("Disk %s is not partitioned. Formatting with ext4...", drive))
			cmd = exec.Command("mkfs.ext4", "-F", "-F", drive)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to format %s: %w", drive, err)
			}
		}
		cmd = exec.Command("blkid", "-s", "UUID", "-o", "value", drive)
		uuidOutput, err := cmd.Output()
		uuid := ""
		if err == nil {
			uuid = strings.TrimSpace(string(uuidOutput))
		}
		if uuid != "" && strings.Contains(string(fstabContent), fmt.Sprintf("UUID=%s", uuid)) {
			LogMessage(Info, fmt.Sprintf("%s is in /etc/fstab, automounting.", drive))
			cmd := exec.Command("mount", "-a", drive)
			_, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("failed to automount %s: %w", drive, err)
			}
			continue
		}
		mountPoint := fmt.Sprintf("/mnt/disk%d", i)
		for usedMountPoints[mountPoint] || strings.Contains(string(fstabContent), mountPoint) {
			i++
			mountPoint = fmt.Sprintf("/mnt/disk%d", i)
		}
		usedMountPoints[mountPoint] = true

		if err := os.MkdirAll(mountPoint, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", mountPoint, err)
		}

		cmd = exec.Command("mount", drive, mountPoint)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to mount %s at %s: %w", drive, mountPoint, err)
		}

		LogMessage(Info, fmt.Sprintf("Mounted %s at %s", drive, mountPoint))
		i++
	}
	return nil
}

func PersistMountedDisks() error {
	if viper.IsSet("LONGHORN_DISKS") && viper.GetString("LONGHORN_DISKS") != "" {
		LogMessage(Info, "Skipping drive mounting as LONGHORN_DISKS is set.")
		return nil
	}
	if viper.GetBool("SKIP_DISK_CHECK") == true {
		LogMessage(Info, "Skipping drive mounting as SKIP_DISK_CHECK is set.")
		return nil
	}
	cmd := exec.Command("sh", "-c", "mount | awk '/\\/mnt\\/disk[0-9]+/ {print $1, $3}'")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list mounted disks: %w", err)
	}

	mountedDisks := strings.TrimSpace(string(output))
	if mountedDisks == "" {
		return nil
	}

	fstabFile := "/etc/fstab"
	backupFile := "/etc/fstab.bak"
	if err := exec.Command("sudo", "cp", fstabFile, backupFile).Run(); err != nil {
		return fmt.Errorf("failed to backup fstab file: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(mountedDisks))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		device, mountPoint := fields[0], fields[1]

		cmd := exec.Command("blkid", "-s", "UUID", "-o", "value", device)
		uuidOutput, err := cmd.Output()
		if err != nil {
			LogMessage(Info, fmt.Sprintf("Could not retrieve UUID for %s. Skipping...", device))
			continue
		}
		uuid := strings.TrimSpace(string(uuidOutput))
		if uuid == "" {
			LogMessage(Info, fmt.Sprintf("Could not retrieve UUID for %s. Skipping...", device))
			continue
		}
		fstabContent, err := os.ReadFile(fstabFile)
		if err != nil {
			return fmt.Errorf("failed to read fstab file: %w", err)
		}
		if strings.Contains(string(fstabContent), fmt.Sprintf("UUID=%s", uuid)) {
			LogMessage(Debug, fmt.Sprintf("%s is already in /etc/fstab.", mountPoint))
			continue
		}
		entry := fmt.Sprintf("UUID=%s %s ext4 defaults,nofail 0 2\n", uuid, mountPoint)
		cmd = exec.Command("sudo", "tee", "-a", fstabFile)
		cmd.Stdin = strings.NewReader(entry)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add entry to fstab: %w", err)
		}
		LogMessage(Debug, fmt.Sprintf("Added %s to /etc/fstab.", mountPoint))
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}
	if err := exec.Command("sudo", "mount", "-a").Run(); err != nil {
		return fmt.Errorf("failed to remount filesystems: %w", err)
	}

	return nil
}

func CleanTargetDisks(targetDisks []string) error {
	if len(targetDisks) == 0 {
		LogMessage(Info, "No target disks provided for cleanup.")
		return nil
	}

	LogMessage(Info, fmt.Sprintf("Starting cleanup process for %d disks: %v", len(targetDisks), targetDisks))

	// Step 1: Clean fstab entries (reverse of PersistMountedDisks)
	LogMessage(Info, "Step 1: Cleaning fstab entries")
	if err := CleanFstab(targetDisks); err != nil {
		return fmt.Errorf("failed to clean fstab: %w", err)
	}

	// Step 2: Get mount points before unmounting (for later cleanup)
	LogMessage(Info, "Step 2: Getting mount points for cleanup")
	mountPointsToRemove, err := GetMountPoints(targetDisks)
	if err != nil {
		return fmt.Errorf("failed to get mount points: %w", err)
	}
	LogMessage(Debug, fmt.Sprintf("Found %d mount points to remove later: %v", len(mountPointsToRemove), mountPointsToRemove))

	// Step 3: Unmount target disks
	LogMessage(Info, "Step 3: Unmounting target disks")
	var successfulUnmounts []string
	for _, disk := range targetDisks {
		if err := UnmountTargetDisks([]string{disk}); err != nil {
			LogMessage(Info, fmt.Sprintf("Skipping disk %s: failed to unmount (%v)", disk, err))
			continue
		}
		successfulUnmounts = append(successfulUnmounts, disk)
	}
	if len(successfulUnmounts) == 0 {
		LogMessage(Error, "No disks could be successfully unmounted. Aborting cleanup.")
		return fmt.Errorf("all target disks failed to unmount")
	}


	// Step 4: Wipe target disks (wipe and format - reverse of MountDrive)
	LogMessage(Info, "Step 4: Wiping and formatting target disks")
	var successfullyWiped []string
	for _, disk := range successfulUnmounts {
		if err := WipeTargetDisks([]string{disk}); err != nil {
			LogMessage(Info, fmt.Sprintf("Skipping disk %s: failed to wipe (%v)", disk, err))
			continue
		}
		successfullyWiped = append(successfullyWiped, disk)
	}

	if len(successfullyWiped) == 0 {
		LogMessage(Error, "No disks could be successfully wiped. Aborting cleanup.")
		return fmt.Errorf("all target disks failed to wipe")
	}

	// Step 5: Remove mount point directories
	LogMessage(Info, "Step 5: Removing mount point directories")
	for _, mountPoint := range mountPointsToRemove {
		if err := RemoveMountPointDirectories([]string{mountPoint}); err != nil {
			LogMessage(Info, fmt.Sprintf("Failed to remove mount point %s: %v", mountPoint, err))
		}
	}
	LogMessage(Info, fmt.Sprintf("Successfully completed cleanup process for %d disks (some may have been skipped)", len(successfullyWiped)))
	return nil
}

func CleanFstab(targetDisks []string) error {
	if len(targetDisks) == 0 {
		LogMessage(Info, "No target disks provided for fstab cleanup.")
		return nil
	}

	// Create timestamped backup of fstab
	backupTimestamp := time.Now().Format("060102-15:04")
	backupFile := fmt.Sprintf("/etc/fstab.bak-%s", backupTimestamp)

	if err := exec.Command("sudo", "cp", "/etc/fstab", backupFile).Run(); err != nil {
		return fmt.Errorf("failed to backup fstab file: %w", err)
	}
	LogMessage(Info, fmt.Sprintf("Created fstab backup: %s", backupFile))

	// Get UUIDs from target disks
	var targetUUIDs []string
	for _, disk := range targetDisks {
		cmd := exec.Command("blkid", "-s", "UUID", "-o", "value", disk)
		uuidOutput, err := cmd.Output()
		if err != nil {
			LogMessage(Warn, fmt.Sprintf("Could not retrieve UUID for %s: %v", disk, err))
			continue
		}
		uuid := strings.TrimSpace(string(uuidOutput))
		if uuid != "" {
			targetUUIDs = append(targetUUIDs, uuid)
			LogMessage(Debug, fmt.Sprintf("Found UUID %s for disk %s", uuid, disk))
		}
	}

	if len(targetUUIDs) == 0 {
		LogMessage(Info, "No UUIDs found for target disks.")
		return nil
	}

	// Read current fstab content
	fstabContent, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return fmt.Errorf("failed to read fstab file: %w", err)
	}

	// Clean fstab entries
	lines := strings.Split(string(fstabContent), "\n")
	var cleanedLines []string
	removedCount := 0

	for _, line := range lines {
		shouldRemove := false
		for _, uuid := range targetUUIDs {
			if strings.Contains(line, fmt.Sprintf("UUID=%s", uuid)) {
				LogMessage(Info, fmt.Sprintf("Removing fstab entry: %s", strings.TrimSpace(line)))
				shouldRemove = true
				removedCount++
				break
			}
		}
		if !shouldRemove {
			cleanedLines = append(cleanedLines, line)
		}
	}

	// Write cleaned fstab back
	cleanedContent := strings.Join(cleanedLines, "\n")
	tempFile := "/tmp/fstab.clean"
	if err := os.WriteFile(tempFile, []byte(cleanedContent), 0644); err != nil {
		return fmt.Errorf("failed to write temporary fstab: %w", err)
	}

	if err := exec.Command("sudo", "cp", tempFile, "/etc/fstab").Run(); err != nil {
		return fmt.Errorf("failed to update fstab: %w", err)
	}

	if err := os.Remove(tempFile); err != nil {
		LogMessage(Warn, fmt.Sprintf("Failed to remove temporary file %s: %v", tempFile, err))
	}

	LogMessage(Info, fmt.Sprintf("Cleaned %d entries from /etc/fstab", removedCount))
	return nil
}

func GetMountPoints(targetDisks []string) ([]string, error) {
	if len(targetDisks) == 0 {
		LogMessage(Info, "No target disks provided for getting mount points.")
		return nil, nil
	}

	// Get mount points before unmounting
	var mountPointsToRemove []string
	for _, disk := range targetDisks {
		cmd := exec.Command("lsblk", "-no", "MOUNTPOINT", disk)
		output, err := cmd.Output()
		if err != nil {
			LogMessage(Warn, fmt.Sprintf("Could not get mount point for %s: %v", disk, err))
			continue
		}
		mountPoint := strings.TrimSpace(string(output))
		if mountPoint != "" && mountPoint != "/" {
			mountPointsToRemove = append(mountPointsToRemove, mountPoint)
			LogMessage(Debug, fmt.Sprintf("Found mount point %s for disk %s", mountPoint, disk))
		}
	}

	LogMessage(Info, fmt.Sprintf("Found %d mount points to track for removal", len(mountPointsToRemove)))
	return mountPointsToRemove, nil
}

func UnmountTargetDisks(targetDisks []string) error {
	if len(targetDisks) == 0 {
		LogMessage(Info, "No target disks provided for unmounting.")
		return nil
	}

	// Unmount target disks
	for _, disk := range targetDisks {
		cmd := exec.Command("lsblk", "-no", "MOUNTPOINT", disk)
		output, err := cmd.Output()
		if err != nil {
			LogMessage(Warn, fmt.Sprintf("Could not check mount status for %s: %v", disk, err))
			continue
		}
		mountPoint := strings.TrimSpace(string(output))
		if mountPoint != "" && mountPoint != "/" {
			LogMessage(Info, fmt.Sprintf("Unmounting %s from %s", disk, mountPoint))
			cmd := exec.Command("sudo", "umount", mountPoint)
			if err := cmd.Run(); err != nil {
				LogMessage(Warn, fmt.Sprintf("Failed to unmount %s: %v", disk, err))
				// Try force unmount
				cmd := exec.Command("sudo", "umount", "-f", mountPoint)
				if err := cmd.Run(); err != nil {
					LogMessage(Error, fmt.Sprintf("Failed to force unmount %s: %v", disk, err))
					continue
				}
				LogMessage(Info, fmt.Sprintf("Force unmounted %s", disk))
			} else {
				LogMessage(Info, fmt.Sprintf("Successfully unmounted %s", disk))
			}
		} else {
			LogMessage(Debug, fmt.Sprintf("Disk %s is not mounted or mounted at root", disk))
		}
	}

	return nil
}

func WipeTargetDisks(targetDisks []string) error {
	if len(targetDisks) == 0 {
		LogMessage(Info, "No target disks provided for cleaning.")
		return nil
	}

	// Clean target disks
	for _, disk := range targetDisks {
		cmd := exec.Command("lsblk", "-no", "MOUNTPOINT", disk)
		output, _ := cmd.Output()
		if strings.TrimSpace(string(output)) != "" {
			LogMessage(Warn, fmt.Sprintf("Disk %s appears to still be mounted", disk))
		}
		LogMessage(Info, fmt.Sprintf("Wiping filesystem signatures on %s", disk))
		cmd = exec.Command("sudo", "wipefs", "-a", disk) //
		if err := cmd.Run(); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to wipe %s: %v", disk, err))
			continue
		}

		LogMessage(Info, fmt.Sprintf("Formatting %s with ext4", disk))
		cmd = exec.Command("sudo", "mkfs.ext4", "-F", "-F", disk)
		if err := cmd.Run(); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to format %s: %v", disk, err))
			continue
		}

		LogMessage(Info, fmt.Sprintf("Successfully cleaned and formatted %s", disk))
	}

	return nil
}

func RemoveMountPointDirectories(mountPointsToRemove []string) error {
	if len(mountPointsToRemove) == 0 {
		LogMessage(Info, "No mount point directories to remove.")
		return nil
	}

	// Remove mount point directories
	for _, mountPoint := range mountPointsToRemove {
		if strings.HasPrefix(mountPoint, "/mnt/disk") {
			LogMessage(Info, fmt.Sprintf("Removing mount point directory %s", mountPoint))
			if err := os.RemoveAll(mountPoint); err != nil {
				LogMessage(Warn, fmt.Sprintf("Failed to remove mount point %s: %v", mountPoint, err))
			} else {
				LogMessage(Info, fmt.Sprintf("Successfully removed mount point %s", mountPoint))
			}
		} else {
			LogMessage(Warn, fmt.Sprintf("Skipping removal of non-standard mount point %s", mountPoint))
		}
	}

	return nil
}
