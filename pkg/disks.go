package pkg

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func CleanDisks() error {
	cmd := exec.Command("mount")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("mount command failed")
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 2 && strings.Contains(fields[2], "kubernetes.io/csi/driver.longhorn.io") {
			if err := exec.Command("sudo", "umount", "-lf", fields[2]).Run(); err != nil {
				LogMessage(Warn, fmt.Sprintf("Failed to unmount %s", fields[2]))
			} else {
				LogMessage(Info, fmt.Sprintf("Unmounted %s", fields[2]))
			}

		}
	}

	cmd = exec.Command("lsblk", "-o", "NAME,MOUNTPOINT")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("lsblk command failed")
	}

	scanner = bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 1 && strings.Contains(fields[1], "kubernetes.io/csi/driver.longhorn.io") {
			dev := "/dev/" + fields[0]
			if err := exec.Command("sudo", "wipefs", "-a", dev).Run(); err != nil {
				LogMessage(Warn, fmt.Sprintf("Failed to wipe %s", dev))
			} else {
				LogMessage(Info, fmt.Sprintf("Wiped %s", dev))
			}
		}
	}

	exec.Command("sudo", "rm", "-rf", "/var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/*").Run()

	cmd = exec.Command("lsblk", "-nd", "-o", "NAME,TYPE,MOUNTPOINT")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("lsblk command failed")
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

func MountAndPersistNVMeDrives() error {
	cmd := exec.Command("sh", "-c", "lsblk -nd -o NAME | grep nvme")
	output, err := cmd.CombinedOutput()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to list NVMe drives: %v", err))
		return fmt.Errorf("failed to list NVMe drives: %w", err)
	}

	availableDisks := strings.Fields(string(output))
	if len(availableDisks) == 0 {
		LogMessage(Info, "No NVMe drives found.")
		return nil
	}

	fmt.Println("The following NVMe drives will be mounted:")
	for _, disk := range availableDisks {
		fmt.Printf("/dev/%s\n", disk)
	}
	fmt.Print("Do you want to continue? (yes/no): ")
	var response string
	fmt.Scanln(&response)
	if strings.ToLower(response) != "yes" {
		LogMessage(Info, "Mounting aborted by user.")
		return nil
	}

	for i, disk := range availableDisks {
		mountPoint := fmt.Sprintf("/mnt/disk%d", i+1)
		if err := runCommand("sudo", "mkdir", "-p", mountPoint); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to create mount point %s: %v", mountPoint, err))
			return fmt.Errorf("failed to create mount point %s: %w", mountPoint, err)
		}
		if err := runCommand("sudo", "chmod", "755", mountPoint); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to set permissions on mount point %s: %v", mountPoint, err))
			return fmt.Errorf("failed to set permissions on mount point %s: %w", mountPoint, err)
		}
		if err := runCommand("sudo", "mkfs.ext4", "-F", fmt.Sprintf("/dev/%s", disk)); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to format /dev/%s: %v", disk, err))
			return fmt.Errorf("failed to format /dev/%s: %w", disk, err)
		}
		if err := runCommand("sudo", "mount", fmt.Sprintf("/dev/%s", disk), mountPoint); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to mount /dev/%s at %s: %v", disk, mountPoint, err))
			return fmt.Errorf("failed to mount /dev/%s at %s: %w", disk, mountPoint, err)
		}
		LogMessage(Info, fmt.Sprintf("Mounted /dev/%s at %s", disk, mountPoint))

		uuidCmd := exec.Command("blkid", "-s", "UUID", "-o", "value", fmt.Sprintf("/dev/%s", disk))
		uuidOutput, err := uuidCmd.Output()
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to get UUID for /dev/%s: %v", disk, err))
			return fmt.Errorf("failed to get UUID for /dev/%s: %w", disk, err)
		}
		uuid := strings.TrimSpace(string(uuidOutput))
		fstabEntry := fmt.Sprintf("UUID=%s %s ext4 defaults,nofail 0 2\n", uuid, mountPoint)

		fstabFile, err := os.OpenFile("/etc/fstab", os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to open /etc/fstab: %v", err))
			return fmt.Errorf("failed to open /etc/fstab: %w", err)
		}
		defer fstabFile.Close()

		if _, err := fstabFile.WriteString(fstabEntry); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to write to /etc/fstab: %v", err))
			return fmt.Errorf("failed to write to /etc/fstab: %w", err)
		}
		LogMessage(Info, fmt.Sprintf("Added /dev/%s to /etc/fstab", disk))
	}

	if err := runCommand("sudo", "mount", "-a"); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to remount filesystems: %v", err))
		return fmt.Errorf("failed to remount filesystems: %w", err)
	}

	LogMessage(Info, "Mounted and persisted NVMe drives successfully.")
	return nil
}

func GenerateLonghornDiskString() (string, error) {
	cmd := exec.Command("sh", "-c", "mount | grep -oP '/mnt/disk\\d+'")
	output, err := cmd.CombinedOutput()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to list mounted disks: %v", err))
		return "", fmt.Errorf("failed to list mounted disks: %w", err)
	}

	disks := strings.Fields(string(output))
	if len(disks) == 0 {
		LogMessage(Info, "No /mnt/disk{x} drives found.")
		return "", nil
	}

	nvmeDisks := []string{}
	for _, disk := range disks {
		cmd := exec.Command("sh", "-c", fmt.Sprintf("lsblk -no NAME,MOUNTPOINT | grep '%s' | grep 'nvme'", disk))
		if err := cmd.Run(); err == nil {
			nvmeDisks = append(nvmeDisks, disk)
		}
	}

	if len(nvmeDisks) == 0 {
		LogMessage(Info, "No NVMe drives found among the mounted disks.")
		return "", nil
	}

	var jsonBuilder strings.Builder
	jsonBuilder.WriteString("[")
	for _, disk := range nvmeDisks {
		jsonBuilder.WriteString(fmt.Sprintf("{\\\"path\\\":\\\"%s\\\",\\\"allowScheduling\\\":true},", disk))
	}
	jsonString := strings.TrimRight(jsonBuilder.String(), ",") + "]"

	LogMessage(Info, "Generated Longhorn disk configuration string.")
	return jsonString, nil
}
