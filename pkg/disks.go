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

	"github.com/silogen/cluster-bloom/pkg/mockablecmd"
	"github.com/spf13/viper"
)

const bloomFstabTag = "# managed by cluster-bloom"

func UnmountPriorLonghornDisks() error {

	// Create backup first
	backupTimestamp := time.Now().Format("060102-15:04")
	backupFile := fmt.Sprintf("/etc/fstab.bak-%s", backupTimestamp)
	if err := exec.Command("sudo", "cp", "/etc/fstab", backupFile).Run(); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to backup fstab: %v", err))
		return fmt.Errorf("failed to backup fstab: %w", err)
	}
	LogMessage(Info, fmt.Sprintf("Created fstab backup: %s", backupFile))

	mountPoints := make(map[string]string)

	// Read /etc/fstab and look for entries tagged with bloomFstabTag
	fstabContent, err := os.ReadFile("/etc/fstab")
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to read /etc/fstab: %v", err))
		return fmt.Errorf("failed to read /etc/fstab: %w", err)
	}

	// Open temp file for writing cleaned fstab
	tempFile := "/tmp/fstab.clean"
	cleanFile, err := os.OpenFile(tempFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create temporary fstab: %w", err)
	}
	defer cleanFile.Close()

	// Parse fstab in a single pass: unmount bloom entries and write non-bloom lines
	lines := strings.Split(string(fstabContent), "\n")
	removedCount := 0

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check if this is a bloom entry
		if strings.HasSuffix(trimmedLine, bloomFstabTag) {
			// Extract mount point for unmounting
			fields := strings.Fields(trimmedLine)
			if len(fields) >= 2 {
				mountPoint := fields[1]
				if mountPoint != "" {
					mountPoints[mountPoint] = mountPoint
					LogMessage(Info, fmt.Sprintf("Force unmounting %s", mountPoint))
					cmd := exec.Command("sudo", "umount", "-lf", mountPoint)
					if err := cmd.Run(); err != nil {
						LogMessage(Warn, fmt.Sprintf("Failed to force unmount %s: %v", mountPoint, err))
					}
					LogMessage(Info, fmt.Sprintf("Successfully unmounted %s", mountPoint))
				}
			}
			LogMessage(Info, fmt.Sprintf("Removing fstab entry: %s", trimmedLine))
			removedCount++
			continue
		}

		if _, err := cleanFile.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("failed to write to temporary fstab: %w", err)
		}
	}

	if len(mountPoints) == 0 {
		LogMessage(Info, "No bloom-tagged mount points found in fstab")
		return nil
	}

	LogMessage(Info, fmt.Sprintf("Successfully unmounted and removed %d mount points from fstab", len(mountPoints)))

	// Close file before moving
	if err := cleanFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary fstab: %w", err)
	}

	// Write cleaned fstab
	LogMessage(Info, "Writing cleaned /etc/fstab")

	if err := exec.Command("sudo", "mv", tempFile, "/etc/fstab").Run(); err != nil {
		return fmt.Errorf("failed to update fstab: %w", err)
	}

	LogMessage(Info, fmt.Sprintf("Removed %d bloom entries from /etc/fstab", removedCount))

	return nil
}

func CleanDisks() error {
	LogMessage(Info, "Disks cleanup started.")

	err := UnmountPriorLonghornDisks()
	if err != nil {
		LogMessage(Warn, fmt.Sprintf("Failed to unmount prior Longhorn disks: %v", err))
	}

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
`

func GenerateNodeLabels(mountedDiskMap map[string]string) error {
	rke2ConfigPath := "/etc/rancher/rke2/config.yaml"
	// Fill the template with the GPU_NODE setting, leave longhor for later
	nodeLabels := fmt.Sprintf(nodeLabelTemplate, viper.GetBool("GPU_NODE"))
	if err := appendToFile(rke2ConfigPath, nodeLabels); err != nil {
		return fmt.Errorf("failed to append Longhorn configuration to %s: %w", rke2ConfigPath, err)
	}

	if viper.GetBool("NO_DISKS_FOR_CLUSTER") {
		LogMessage(Info, "Skipping GenerateLonghornDiskString as NO_DISKS_FOR_CLUSTER is set.")
		return nil
	}

	if len(mountedDiskMap) == 0 {
		LogMessage(Info, "No mounted disks found in mountedDiskMap, skipping")
		return nil
	}

	if err := appendToFile(rke2ConfigPath, longhornDiskTemplate); err != nil {
		return fmt.Errorf("failed to append Longhorn template to %s: %w", rke2ConfigPath, err)
	}

	for mountPoint, device := range mountedDiskMap {
		// Replace slashes with underscores in device name for label
		diskLabel := fmt.Sprintf("  - bloom.disk%s=disk%s\n", mountPoint, device)
		diskLabel = strings.ReplaceAll(diskLabel, "/", "___")
		if err := appendToFile(rke2ConfigPath, diskLabel); err != nil {
			return fmt.Errorf("failed to append label to %s: %w", rke2ConfigPath, err)
		}
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

func MountDrives(drives []string) (map[string]string, error) {
	if viper.IsSet("CLUSTER_PREMOUNTED_DISKS") && viper.GetString("CLUSTER_PREMOUNTED_DISKS") != "" {
		LogMessage(Info, "Skipping drive mounting as CLUSTER_PREMOUNTED_DISKS is set.")
		return nil, nil
	}
	if viper.GetBool("NO_DISKS_FOR_CLUSTER") == true {
		LogMessage(Info, "Skipping drive mounting as NO_DISKS_FOR_CLUSTER is set.")
		return nil, nil
	}

	mountedMap := make(map[string]string)
	usedMountPoints := make(map[string]bool)
	i := 0
	output, err := mockablecmd.Run("PrepareLonghornDisksStep.ListMounts", "sh", "-c", "mount | awk '/\\/mnt\\/disk[0-9]+/ {print $3}'")
	if err != nil {
		return nil, fmt.Errorf("failed to list existing mount points: %w", err)
	}
	existingMountPoints := strings.Fields(string(output))
	for _, mountPoint := range existingMountPoints {
		usedMountPoints[mountPoint] = true
	}
	fstabContent, err := os.ReadFile("/etc/fstab")
	if err != nil {
		return nil, fmt.Errorf("failed to read /etc/fstab: %w", err)
	}

	for _, drive := range drives {
		mockID := fmt.Sprintf("PrepareLonghornDisksStep.CheckFilesystem.%s", drive)
		output, err := mockablecmd.Run(mockID, "lsblk", "-f", drive)
		if err != nil {
			return mountedMap, fmt.Errorf("failed to check filesystem type for %s: %w", drive, err)
		}
		if strings.Contains(string(output), "ext4") {
			LogMessage(Info, fmt.Sprintf("Disk %s is already formatted as ext4. Skipping format.", drive))
		} else {
			mockID = fmt.Sprintf("PrepareLonghornDisksStep.CheckPartitionType.%s", drive)
			output, err = mockablecmd.Run(mockID, "lsblk", "-no", "PARTTYPE", drive)
			if err != nil {
				return mountedMap, fmt.Errorf("failed to check partition type for %s: %w", drive, err)
			}
			if strings.TrimSpace(string(output)) != "" {
				LogMessage(Info, fmt.Sprintf("Disk %s has existing partitions. Removing partitions...", drive))
				mockID := fmt.Sprintf("PrepareLonghornDisksStep.WipePartitions.%s", drive)
				_, err := mockablecmd.Run(mockID, "sudo", "wipefs", "-a", drive)
				if err != nil {
					return mountedMap, fmt.Errorf("failed to wipe partitions on %s: %w", drive, err)
				}
			}

			LogMessage(Info, fmt.Sprintf("Disk %s is not partitioned. Formatting with ext4...", drive))
			mockID := fmt.Sprintf("PrepareLonghornDisksStep.FormatDisk.%s", drive)
			_, err := mockablecmd.Run(mockID, "mkfs.ext4", "-F", "-F", drive)
			if err != nil {
				return mountedMap, fmt.Errorf("failed to format %s: %w", drive, err)
			}
		}
		mockID = fmt.Sprintf("PrepareLonghornDisksStep.GetUUID.%s", drive)
		uuidOutput, err := mockablecmd.Run(mockID, "blkid", "-s", "UUID", "-o", "value", drive)
		uuid := ""
		if err == nil {
			uuid = strings.TrimSpace(string(uuidOutput))
		}
		if uuid != "" && strings.Contains(string(fstabContent), fmt.Sprintf("UUID=%s", uuid)) {
			return mountedMap, fmt.Errorf("disk %s is already in /etc/fstab - please remove it first", drive)
		}
		mountPoint := fmt.Sprintf("/mnt/disk%d", i)
		for usedMountPoints[mountPoint] || strings.Contains(string(fstabContent), mountPoint) {
			i++
			mountPoint = fmt.Sprintf("/mnt/disk%d", i)
		}
		usedMountPoints[mountPoint] = true

		if err := os.MkdirAll(mountPoint, 0755); err != nil {
			return mountedMap, fmt.Errorf("failed to create mount point %s: %w", mountPoint, err)
		}

		mockID = fmt.Sprintf("PrepareLonghornDisksStep.MountDrive.%s", drive)
		_, err = mockablecmd.Run(mockID, "mount", drive, mountPoint)
		if err != nil {
			return mountedMap, fmt.Errorf("failed to mount %s at %s: %w", drive, mountPoint, err)
		}

		LogMessage(Info, fmt.Sprintf("Mounted %s at %s", drive, mountPoint))
		mountedMap[mountPoint] = fmt.Sprintf("%s-%s", drive, uuid)

		i++
	}
	return mountedMap, nil
}

func PersistMountedDisks(mountedMap map[string]string) error {
	if viper.IsSet("CLUSTER_PREMOUNTED_DISKS") && viper.GetString("CLUSTER_PREMOUNTED_DISKS") != "" {
		LogMessage(Info, "Skipping drive mounting as CLUSTER_PREMOUNTED_DISKS is set.")
		return nil
	}
	if viper.GetBool("NO_DISKS_FOR_CLUSTER") == true {
		LogMessage(Info, "Skipping drive mounting as NO_DISKS_FOR_CLUSTER is set.")
		return nil
	}

	if len(mountedMap) == 0 {
		LogMessage(Info, "No mounted directories to persist")
		return nil
	}

	fstabFile := "/etc/fstab"
	backupFile := "/etc/fstab.bak"
	if _, err := mockablecmd.Run("PersistMountedDisks.BackupFstab", "sudo", "cp", fstabFile, backupFile); err != nil {
		return fmt.Errorf("failed to backup fstab file: %w", err)
	}

	for mountPoint, device := range mountedMap {
		mockID := fmt.Sprintf("PersistMountedDisks.GetUUID.%s", device)
		uuidOutput, err := mockablecmd.Run(mockID, "blkid", "-s", "UUID", "-o", "value", device)
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
		entry := fmt.Sprintf("UUID=%s %s ext4 defaults,nofail 0 2 %s\n", uuid, mountPoint, bloomFstabTag)
		cmd := exec.Command("sudo", "tee", "-a", fstabFile)
		cmd.Stdin = strings.NewReader(entry)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add entry to fstab: %w", err)
		}
		LogMessage(Debug, fmt.Sprintf("Added %s to /etc/fstab.", mountPoint))
	}

	if _, err := mockablecmd.Run("PersistMountedDisks.RemountAll", "sudo", "mount", "-a"); err != nil {
		return fmt.Errorf("failed to remount filesystems: %w", err)
	}

	return nil
}

