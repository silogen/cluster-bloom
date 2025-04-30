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

	"github.com/spf13/viper"
)

func CleanDisks() error {
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

var longhornConfigTemplate = `
node-label:
  - node.longhorn.io/create-default-disk=config
  - node.longhorn.io/instance-manager=true
  - silogen.ai/longhorndisks=%s
`

func GenerateLonghornDiskString() error {
	rke2ConfigPath := "/etc/rancher/rke2/config.yaml"

	if viper.IsSet("LONGHORN_DISKS") && viper.GetString("LONGHORN_DISKS") != "" {
		LogMessage(Info, "Using LONGHORN_DISKS for Longhorn configuration.")
		disks := strings.Split(viper.GetString("LONGHORN_DISKS"), ",")
		diskList := strings.Join(disks, "xxx")

		configContent := fmt.Sprintf(longhornConfigTemplate, diskList)

		if err := appendToFile(rke2ConfigPath, configContent); err != nil {
			return fmt.Errorf("failed to append Longhorn configuration to %s: %w", rke2ConfigPath, err)
		}
		LogMessage(Info, "Appended Longhorn disk configuration to RKE2 config.")
		return nil
	}
	if viper.IsSet("SKIP_DISK_CHECK") {
		LogMessage(Info, "Skipping GenerateLonghornDiskString as SKIP_DISK_CHECK is set.")
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

	nvmeDisks := []string{}
	for _, disk := range disks {
		cmd := exec.Command("sh", "-c", fmt.Sprintf("lsblk -no NAME,MOUNTPOINT | grep '%s' | grep 'nvme'", disk))
		if err := cmd.Run(); err == nil {
			nvmeDisks = append(nvmeDisks, strings.TrimPrefix(disk, "/mnt/"))
		}
	}

	if len(nvmeDisks) > 0 {
		diskList := strings.Join(nvmeDisks, "xxx")

		configContent := fmt.Sprintf(longhornConfigTemplate, diskList)

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
	if viper.IsSet("SKIP_DISK_CHECK") {
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
	if viper.IsSet("SKIP_DISK_CHECK") {
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
		mountPoint := fmt.Sprintf("/mnt/disk%d", i)
		for usedMountPoints[mountPoint] || strings.Contains(string(fstabContent), mountPoint) {
			i++
			mountPoint = fmt.Sprintf("/mnt/disk%d", i)
		}
		usedMountPoints[mountPoint] = true

		if err := os.MkdirAll(mountPoint, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", mountPoint, err)
		}

		cmd := exec.Command("lsblk", "-f", drive)
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
	if viper.IsSet("SKIP_DISK_CHECK") {
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
