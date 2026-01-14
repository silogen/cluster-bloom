//go:build linux

package runtime

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/silogen/cluster-bloom/pkg/ssh"
)

// PreExecutionCleanup performs comprehensive cleanup before running Ansible playbooks
// to prevent issues with stuck gathering facts from previous interrupted runs
func PreExecutionCleanup() error {
	fmt.Println("🧹 Performing pre-execution cleanup...")

	var errors []string

	// Clean up SSH processes targeting localhost
	if err := cleanupSSHProcesses(); err != nil {
		errors = append(errors, fmt.Sprintf("SSH cleanup: %v", err))
	}

	// Clean up Ansible temporary files and control sockets
	if err := cleanupAnsibleTempFiles(); err != nil {
		errors = append(errors, fmt.Sprintf("Ansible temp cleanup: %v", err))
	}

	// Clean up any existing bloom container processes
	if err := cleanupBloomContainers(); err != nil {
		errors = append(errors, fmt.Sprintf("Container cleanup: %v", err))
	}

	// Clean up SSH control sockets
	if err := cleanupSSHControlSockets(); err != nil {
		errors = append(errors, fmt.Sprintf("SSH control socket cleanup: %v", err))
	}

	if len(errors) > 0 {
		fmt.Printf("⚠️  Cleanup warnings: %s\n", strings.Join(errors, "; "))
		// Continue execution - these are warnings, not fatal errors
	} else {
		fmt.Println("✅ Pre-execution cleanup completed successfully")
	}

	return nil
}

// cleanupSSHProcesses kills any hanging SSH processes targeting localhost
func cleanupSSHProcesses() error {
	// Find SSH processes connecting to localhost/127.0.0.1
	cmd := exec.Command("pgrep", "-f", "ssh.*127\\.0\\.0\\.1\\|ssh.*localhost")
	output, err := cmd.Output()
	if err != nil {
		// No processes found is not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("failed to check for SSH processes: %w", err)
	}

	pids := strings.Fields(strings.TrimSpace(string(output)))
	if len(pids) == 0 {
		return nil
	}

	fmt.Printf("🔍 Found %d SSH process(es) targeting localhost, cleaning up...\n", len(pids))

	for _, pid := range pids {
		// First try SIGTERM
		if err := exec.Command("kill", "-TERM", pid).Run(); err == nil {
			fmt.Printf("   Sent SIGTERM to SSH process %s\n", pid)
			// Give process time to terminate gracefully
			time.Sleep(100 * time.Millisecond)
		}

		// Check if still running, if so use SIGKILL
		if err := exec.Command("kill", "-0", pid).Run(); err == nil {
			if err := exec.Command("kill", "-KILL", pid).Run(); err == nil {
				fmt.Printf("   Force killed SSH process %s\n", pid)
			}
		}
	}

	return nil
}

// cleanupAnsibleTempFiles removes Ansible temporary files and locks
func cleanupAnsibleTempFiles() error {
	// Common locations for Ansible temporary files
	tempDirs := []string{
		"/tmp",
		"/var/tmp",
		filepath.Join(os.Getenv("HOME"), ".ansible"),
	}

	// Patterns for Ansible temporary files
	patterns := []string{
		"ansible-*",
		"ansible_*",
		".ansible_async_*",
		"*ansible*tmp*",
	}

	cleanedCount := 0

	for _, tempDir := range tempDirs {
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			continue
		}

		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(tempDir, pattern))
			if err != nil {
				continue
			}

			for _, match := range matches {
				// Check if this looks like an Ansible temp file and is safe to remove
				if isSafeToRemove(match) {
					if err := os.RemoveAll(match); err == nil {
						cleanedCount++
					}
				}
			}
		}
	}

	if cleanedCount > 0 {
		fmt.Printf("   Cleaned up %d Ansible temporary files\n", cleanedCount)
	}

	return nil
}

// cleanupBloomContainers cleans up any existing bloom container processes
func cleanupBloomContainers() error {
	// Look for bloom container processes
	cmd := exec.Command("pgrep", "-f", "__child__.*bloom")
	output, err := cmd.Output()
	if err != nil {
		// No processes found is not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("failed to check for bloom container processes: %w", err)
	}

	pids := strings.Fields(strings.TrimSpace(string(output)))
	if len(pids) == 0 {
		return nil
	}

	fmt.Printf("🐳 Found %d bloom container process(es), cleaning up...\n", len(pids))

	for _, pid := range pids {
		// Send SIGTERM first
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err == nil {
			fmt.Printf("   Terminated bloom container process %s\n", pid)
			time.Sleep(200 * time.Millisecond)
		}

		// Force kill if still running
		if err := exec.Command("kill", "-0", pid).Run(); err == nil {
			if err := exec.Command("kill", "-KILL", pid).Run(); err == nil {
				fmt.Printf("   Force killed bloom container process %s\n", pid)
			}
		}
	}

	return nil
}

// cleanupSSHControlSockets removes SSH control sockets that might be causing issues
func cleanupSSHControlSockets() error {
	// Common locations for SSH control sockets
	socketDirs := []string{
		"/tmp",
		filepath.Join(os.Getenv("HOME"), ".ssh"),
	}

	cleanedCount := 0

	for _, dir := range socketDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}

		// Look for SSH control socket patterns
		matches, err := filepath.Glob(filepath.Join(dir, "*ssh*127.0.0.1*"))
		if err != nil {
			continue
		}

		for _, match := range matches {
			// Check if it's a socket
			if info, err := os.Stat(match); err == nil && info.Mode()&os.ModeSocket != 0 {
				if err := os.Remove(match); err == nil {
					cleanedCount++
				}
			}
		}

		// Also look for generic control socket patterns
		controlMatches, err := filepath.Glob(filepath.Join(dir, "master-*"))
		if err == nil {
			for _, match := range controlMatches {
				if info, err := os.Stat(match); err == nil && info.Mode()&os.ModeSocket != 0 {
					if err := os.Remove(match); err == nil {
						cleanedCount++
					}
				}
			}
		}
	}

	if cleanedCount > 0 {
		fmt.Printf("   Cleaned up %d SSH control sockets\n", cleanedCount)
	}

	return nil
}

// isSafeToRemove checks if a file is safe to remove during cleanup
func isSafeToRemove(path string) bool {
	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Don't remove directories older than 1 hour (might be in use)
	if info.IsDir() && time.Since(info.ModTime()) < time.Hour {
		return false
	}

	// Don't remove files larger than 100MB (probably not temp files)
	if info.Size() > 100*1024*1024 {
		return false
	}

	// Check if the path contains suspicious patterns that indicate it might not be temp
	basename := filepath.Base(path)

	// Only remove if it clearly looks like an Ansible temp file
	ansibleTempPattern := regexp.MustCompile(`^(ansible[-_]|\.ansible_async_)`)
	if !ansibleTempPattern.MatchString(basename) {
		return false
	}

	return true
}

// ContainerPreExecutionCleanup performs cleanup inside the container before running Ansible
// This is where the actual Ansible SSH processes live, so this cleanup is more effective
func ContainerPreExecutionCleanup() error {
	fmt.Println("🧹 Performing container pre-execution cleanup...")

	var errors []string

	// Clean up any existing SSH processes in the container
	if err := cleanupContainerSSHProcesses(); err != nil {
		errors = append(errors, fmt.Sprintf("Container SSH cleanup: %v", err))
	}

	// Clean up Ansible lock files and temp files in container
	if err := cleanupContainerAnsibleFiles(); err != nil {
		errors = append(errors, fmt.Sprintf("Container Ansible cleanup: %v", err))
	}

	// Kill any existing ansible-playbook processes
	if err := cleanupAnsibleProcesses(); err != nil {
		errors = append(errors, fmt.Sprintf("Ansible process cleanup: %v", err))
	}

	if len(errors) > 0 {
		fmt.Printf("⚠️  Container cleanup warnings: %s\n", strings.Join(errors, "; "))
	} else {
		fmt.Println("✅ Container pre-execution cleanup completed successfully")
	}

	return nil
}

// cleanupContainerSSHProcesses kills SSH processes inside the container
func cleanupContainerSSHProcesses() error {
	// In container, look for SSH processes
	cmd := exec.Command("pkill", "-f", "ssh.*127\\.0\\.0\\.1")
	if err := cmd.Run(); err != nil {
		// pkill returns 1 if no processes found, which is fine
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("failed to kill SSH processes: %w", err)
	}
	fmt.Println("   Killed existing SSH processes in container")
	return nil
}

// cleanupContainerAnsibleFiles removes Ansible temp files inside container
func cleanupContainerAnsibleFiles() error {
	// Container-specific temp locations
	tempDirs := []string{
		"/tmp",
		"/var/tmp",
		"/root/.ansible",
	}

	patterns := []string{
		"ansible-*",
		"ansible_*",
		".ansible_async_*",
	}

	cleanedCount := 0

	for _, tempDir := range tempDirs {
		if _, err := os.Stat(tempDir); os.IsNotExist(err) {
			continue
		}

		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(tempDir, pattern))
			if err != nil {
				continue
			}

			for _, match := range matches {
				if err := os.RemoveAll(match); err == nil {
					cleanedCount++
				}
			}
		}
	}

	if cleanedCount > 0 {
		fmt.Printf("   Cleaned up %d Ansible temp files in container\n", cleanedCount)
	}

	return nil
}

// cleanupAnsibleProcesses kills any existing ansible-playbook processes
func cleanupAnsibleProcesses() error {
	cmd := exec.Command("pkill", "-f", "ansible-playbook")
	if err := cmd.Run(); err != nil {
		// pkill returns 1 if no processes found, which is fine
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("failed to kill ansible-playbook processes: %w", err)
	}
	fmt.Println("   Killed existing ansible-playbook processes")
	return nil
}

// setupSignalHandling sets up signal handlers for graceful cleanup
func setupSignalHandling() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		fmt.Println("\n🛑 Received interrupt signal, cleaning up...")
		cleanupContainerSSHProcesses()
		cleanupAnsibleProcesses()
		os.Exit(130) // Exit with 128 + SIGINT
	}()
}

// CleanupOnExit performs cleanup when the program exits (for use with defer)
func CleanupOnExit() {
	fmt.Println("🧹 Performing exit cleanup...")

	// Quick cleanup of SSH processes we might have spawned
	cleanupSSHProcesses()

	// Clean up any control sockets we created
	cleanupSSHControlSockets()

	// Clean up ephemeral SSH keys
	cleanupEphemeralSSH()
}

// cleanupEphemeralSSH attempts to cleanup any remaining ephemeral SSH keys
// This is a safety net in case normal cleanup failed
func cleanupEphemeralSSH() {
	// Try to detect work directory from current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return // Can't determine work directory, skip cleanup
	}

	// Try common username detection patterns
	usernames := []string{
		os.Getenv("SUDO_USER"),
		os.Getenv("USER"),
		"ubuntu", // Default fallback
	}

	for _, username := range usernames {
		if username == "" {
			continue
		}

		// Attempt cleanup with this username
		sshManager := ssh.NewEphemeralSSHManager(cwd, username)
		if err := sshManager.Cleanup(); err == nil {
			fmt.Printf("   ✓ Cleaned up ephemeral SSH keys for user %s\n", username)
			return // Successfully cleaned up
		}
	}
}

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
func CleanupBloomDisks() error {
	fmt.Println("💽 Cleaning bloom-managed disks...")

	// First unmount prior Longhorn disks (equivalent to UnmountPriorLonghornDisks)
	if err := unmountPriorLonghornDisks(); err != nil {
		fmt.Printf("   Warning: Failed to unmount prior Longhorn disks: %v\n", err)
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

	fmt.Println("   Disk cleanup completed")
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
