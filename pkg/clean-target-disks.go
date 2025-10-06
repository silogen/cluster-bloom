package pkg 

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

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
