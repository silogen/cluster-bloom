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
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

var SupportedUbuntuVersions = []string{"20.04", "22.04", "24.04"}
var ports = []string{
	//"22;tcp", Ignoring ssh because it will always be required anyhow.
	"80;tcp", "443;tcp", "2376;tcp", "2379;tcp", "2380;tcp", "6443;tcp",
	"8472;udp", "9099;tcp", "9345;tcp", "10250;tcp", "10254;tcp", "30000:32767;tcp", "30000:32767;udp",
}

func IsRunningOnSupportedUbuntu() bool {
	osReleaseContent, err := os.ReadFile("/etc/os-release")
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to read /etc/os-release: %v", err))
		return false
	}
	osReleaseStr := string(osReleaseContent)
	if !strings.Contains(osReleaseStr, "ID=ubuntu") {
		LogMessage(Error, "This system is not running Ubuntu")
		return false
	}
	var version string
	for _, line := range strings.Split(osReleaseStr, "\n") {
		if strings.HasPrefix(line, "VERSION_ID=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
			break
		}
	}

	if version == "" {
		LogMessage(Error, "Could not detect Ubuntu version")
		return false
	}

	for _, supportedVersion := range SupportedUbuntuVersions {
		if version == supportedVersion {
			LogMessage(Info, fmt.Sprintf("Running on supported Ubuntu version %s", version))
			return true
		}
	}

	LogMessage(Error, fmt.Sprintf("Ubuntu version %s is not supported", version))
	return false
}

func CheckPortsBeforeOpening() error {
	// In dry-run mode, just output port status with lsof
	LogMessage(Info, "Proof mode: checking port status with lsof...")

	var portsInUse bool = false
	var stepErr error

	// Check each port specifically
	for _, entry := range ports {
		parts := strings.Split(entry, ";")
		port, protocol := parts[0], parts[1]

		// Handle port ranges poorly
		if strings.Contains(port, ":") {
			// For ranges, we can't easily check with lsof, so just log the info
			LogMessage(Info, fmt.Sprintf("Would configure port range %s (%s) in actual run", port, protocol))
			continue
		}

		// For specific ports, check with lsof
		portCmd := exec.Command("lsof", "-i", fmt.Sprintf("%s:%s", strings.ToUpper(protocol), port))
		var portOutput bytes.Buffer
		portCmd.Stdout = &portOutput
		portCmd.Stderr = &portOutput

		if err := portCmd.Run(); err != nil {
			// lsof returns non-zero if no processes are using the port
			LogMessage(Info, fmt.Sprintf("Port %s (%s) is not currently in use", port, protocol))
		} else {
			portsInUse = true
			if portOutput.String() != "" {
				LogMessage(Info, fmt.Sprintf("Port %s (%s) status:\n%s", port, protocol, portOutput.String()))
			}
		}
	}

	if portsInUse {
		LogMessage(Warn, "WARNING: Some Ports are in use, see bloom.log")
	}

	LogMessage(Info, "Proof completed - no changes made to iptables")
	return stepErr
}

func OpenPorts() bool {

	for _, entry := range ports {
		parts := strings.Split(entry, ";")
		port, protocol := parts[0], parts[1]

		// Check if rule exists first to avoid duplicates
		checkCmd := exec.Command("sudo", "iptables", "-C", "INPUT", "-p", protocol, "-m", "state", "--state", "NEW", "-m", protocol, "--dport", port, "-j", "ACCEPT")
		if checkCmd.Run() == nil {
			// Rule already exists
			LogMessage(Info, fmt.Sprintf("Rule for %s/%s already exists", port, protocol))
			continue
		}

		// Add the rule
		cmd := exec.Command("sudo", "iptables", "-A", "INPUT", "-p", protocol, "-m", "state", "--state", "NEW", "-m", protocol, "--dport", port, "-j", "ACCEPT")
		if err := cmd.Run(); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to open port %s/%s: %v", port, protocol, err))
			return false
		}
		LogMessage(Info, fmt.Sprintf("Opened port %s/%s", port, protocol))
	}
	if err := exec.Command("sudo", "iptables-save").Run(); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to save iptables rules: %v", err))
		return false
	}

	LogMessage(Debug, "All iptables rules have been added and saved.")
	return true
}

const (
	targetValue = 512
	sysctlFile  = "/etc/sysctl.conf"
	sysctlParam = "fs.inotify.max_user_instances"
)

func getCurrentInotifyValue() (int, error) {
	cmd := exec.Command("sysctl", "-n", sysctlParam)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return 0, err
	}
	val, err := strconv.Atoi(strings.TrimSpace(out.String()))
	if err != nil {
		return 0, err
	}
	return val, nil
}

func setInotifyValue(value int) error {
	cmd := exec.Command("sudo", "sysctl", "-w", fmt.Sprintf("%s=%d", sysctlParam, value))
	return cmd.Run()
}

func CheckInotifyConfig() error {
	currentValue, err := getCurrentInotifyValue()
	if err != nil {
		return fmt.Errorf("failed to get current inotify instances: %w", err)
	}

	if currentValue <= targetValue {
		LogMessage(Warn, fmt.Sprintf("WARNING: Current inotify instances (%d) is less than or equal to requirement (%d)", currentValue, targetValue))
	} else {
		LogMessage(Info, fmt.Sprintf("Current inotify instances (%d) is greater than or equal to requirement (%d)", currentValue, targetValue))
	}

	return nil
}

func updateSysctlConf(value int) error {
	data, err := os.ReadFile(sysctlFile)
	if err != nil {
		if os.IsNotExist(err) {
			LogMessage(Warn, fmt.Sprintf("%s not found, creating a new file", sysctlFile))
			data = []byte{}
		} else {
			return err
		}
	}

	lines := strings.Split(string(data), "\n")
	found := false

	for i, line := range lines {
		if strings.HasPrefix(line, sysctlParam+"=") {
			lines[i] = fmt.Sprintf("%s=%d", sysctlParam, value)
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, fmt.Sprintf("%s=%d", sysctlParam, value))
	}

	return os.WriteFile(sysctlFile, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func VerifyInotifyInstances() bool {
	currentValue, err := getCurrentInotifyValue()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to get current inotify instances: %v", err))
		return false
	}

	if currentValue < targetValue {
		LogMessage(Info, fmt.Sprintf("Increasing %s from %d to %d", sysctlParam, currentValue, targetValue))

		if err := setInotifyValue(targetValue); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to update %s via sysctl: %v", sysctlParam, err))
			return false
		}

		if err := updateSysctlConf(targetValue); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to update %s in %s: %v", sysctlParam, sysctlFile, err))
			return false
		}

		LogMessage(Info, "Successfully updated inotify max user instances")
		return true
	} else {
		LogMessage(Info, "No update required, current value is sufficient")
		return true
	}
}

func HasSufficientRancherPartition() bool {
	if !viper.GetBool("GPU_NODE") {
		LogMessage(Info, "Skipping /var/lib/rancher partition check for CPU node.")
		return true
	}
	cmd := exec.Command("mkdir", "-p", "/var/lib/rancher")
	output, err := cmd.Output()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to create /var/lib/rancher: %v", err))
		return false
	}
	cmd = exec.Command("df", "-BG", "/var/lib/rancher")
	output, err = cmd.Output()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to get /var/lib/rancher partition size: %v", err))
		return false
	}
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		LogMessage(Error, "Unexpected df command output format")
		return false
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		LogMessage(Error, "Unexpected df command output fields")
		return false
	}
	sizeStr := strings.TrimSuffix(fields[1], "G")
	size, err := strconv.ParseFloat(sizeStr, 64)
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to parse /var/lib/rancher partition size: %v", err))
		return false
	}
	if size >= 500 {
		LogMessage(Info, fmt.Sprintf("/var/lib/rancher partition size (%.1fGB) is sufficient", size))
		return true
	}
	LogMessage(Warn, fmt.Sprintf("/var/lib/rancher partition size (%.1fGB) is less than the recommended 500GB", size))
	return false
}

func CreateMetalLBConfig() error {
	cmd := exec.Command("sh", "-c", "ip route get 1 | awk '{print $7; exit}'")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to determine default IP: %v", err)
	}
	defaultIP := strings.TrimSpace(string(output))
	if defaultIP == "" {
		return fmt.Errorf("default IP address could not be determined")
	}

	metallbConfig := fmt.Sprintf(`apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: cluster-bloom-ip-pool
  namespace: metallb-system
spec:
  addresses:
  - %s/32
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: cluster-bloom-l2-advertisement
  namespace: metallb-system
`, defaultIP)
	manifestPath := path.Join(rke2ManifestDirectory, "metallb-address.yaml")
	if err := os.WriteFile(manifestPath, []byte(metallbConfig), 0644); err != nil {
		return fmt.Errorf("failed to write MetalLB configuration to %s: %v", manifestPath, err)
	}

	LogMessage(Info, fmt.Sprintf("MetalLB configuration written to %s with IP %s", manifestPath, defaultIP))
	return nil
}

// GetUserHomeDirViaShell gets a user's home directory using shell tilde expansion
func GetUserHomeDirViaShell(username string) (string, error) {
	// Use shell's tilde expansion to get the home directory
	cmd := exec.Command("sh", "-c", fmt.Sprintf("eval echo ~%s", username))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory for user %s: %w", username, err)
	}

	homeDir := strings.TrimSpace(string(output))
	// Check if the expansion was successful (if it wasn't, the shell would just echo back ~username)
	if homeDir == fmt.Sprintf("~%s", username) {
		return "", fmt.Errorf("user %s not found or home directory not available", username)
	}

	return homeDir, nil
}

func setupMultipath() error {
	configFile := "/etc/multipath.conf"
	blacklistEntry := `devnode "^sd[a-z0-9]+"`
	configContent := "blacklist {\n    devnode \"^sd[a-z0-9]+\"\n}\n"
	// Check if the configuration file exists
	_, err := os.Stat(configFile)
	if os.IsNotExist(err) {
		LogMessage(Info, "Creating default multipath configuration file...")

		err = os.WriteFile(configFile, []byte(configContent), 0644)
		if err != nil {
			return fmt.Errorf("failed to create multipath.conf: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check if multipath.conf exists: %w", err)
	} else {
		// File exists, check if the blacklist entry is already there
		configData, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to read multipath.conf: %w", err)
		}
		newConfigData := ""
		if !strings.Contains(string(configData), blacklistEntry) {
			LogMessage(Info, "Adding blacklist entry to multipath.conf...")
			// Replace this with more robust regex if the file structure varies significantly
			if strings.Contains(string(configData), "blacklist {") {
				newConfigData = strings.Replace(
					string(configData),
					"blacklist {",
					"blacklist {\n    "+blacklistEntry,
					1)
			} else {
				// if no blacklistEntry still, add it
				newConfigData = string(configData) + configContent
			}

			err = os.WriteFile(configFile, []byte(newConfigData), 0644)
			if err != nil {
				return fmt.Errorf("failed to update multipath.conf: %w", err)
			}

			// Restart multipath service
			LogMessage(Info, "Restarting multipathd.service...")
			_, err = runCommand("systemctl", "restart", "multipathd.service")
			if err != nil {
				return fmt.Errorf("failed to restart multipathd service: %w", err)
			}

			// Verify configuration
			LogMessage(Info, "Verifying multipath configuration...")
			output, err := runCommand("multipath", "-t")
			if err != nil {
				LogMessage(Warn, fmt.Sprintf("Multipath verification returned: %s", output))
				return fmt.Errorf("multipath configuration verification failed: %w", err)
			}
		} else {
			LogMessage(Info, "Blacklist entry already present in multipath.conf")
		}
	}

	return nil
}

// checkAmdgpuBlacklist checks if amdgpu is blacklisted in kernel cmdline or modprobe.d
func checkAmdgpuBlacklist() (bool, error) {
	// Check kernel command line
	cmdlineCmd := exec.Command("sh", "-c", "cat /proc/cmdline")
	cmdlineOutput, err := cmdlineCmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to read /proc/cmdline: %w", err)
	}

	hasKernelBlacklist := strings.Contains(string(cmdlineOutput), "modprobe.blacklist=amdgpu")

	// Check modprobe.d
	modprobeCmd := exec.Command("sh", "-c", "grep -r '^[[:space:]]*blacklist[[:space:]]*amdgpu' /etc/modprobe.d/ 2>/dev/null || true")
	modprobeOutput, _ := modprobeCmd.CombinedOutput()
	hasModprobeBlacklist := len(modprobeOutput) > 0

	if hasKernelBlacklist {
		LogMessage(Warn, "amdgpu is blacklisted in ACTIVE kernel command line (reboot required to fix)")
	}
	if hasModprobeBlacklist {
		LogMessage(Warn, fmt.Sprintf("amdgpu is blacklisted in modprobe.d:\n%s", string(modprobeOutput)))
	}

	return hasKernelBlacklist || hasModprobeBlacklist, nil
}

// removeAmdgpuBlacklist removes amdgpu blacklist from modprobe.d and GRUB configuration
// Returns true if reboot is required, and any error encountered
func removeAmdgpuBlacklist() (bool, error) {
	LogMessage(Info, "Starting amdgpu blacklist removal process...")
	needsReboot := false

	// Step 1: Check current kernel command line for blacklist
	checkCmdlineCmd := exec.Command("sh", "-c", "cat /proc/cmdline | grep -o 'modprobe.blacklist=amdgpu' || true")
	cmdlineOutput, _ := checkCmdlineCmd.CombinedOutput()
	if len(cmdlineOutput) > 0 {
		LogMessage(Warn, "Found 'modprobe.blacklist=amdgpu' in ACTIVE kernel command line")
		needsReboot = true
	}

	// Step 2: Remove amdgpu blacklist from modprobe.d
	LogMessage(Info, "Removing amdgpu blacklist from /etc/modprobe.d/...")

	sedCmd := exec.Command("sh", "-c", "sed -i '/^[[:space:]]*blacklist[[:space:]]*amdgpu/s/^/# /' /etc/modprobe.d/*.conf 2>/dev/null || true")
	if output, err := sedCmd.CombinedOutput(); err != nil {
		LogMessage(Warn, fmt.Sprintf("sed command output: %s", string(output)))
	}

	// Step 3: Update GRUB configuration
	LogMessage(Info, "Updating GRUB configuration to remove amdgpu blacklist...")

	// Backup original GRUB config
	backupCmd := exec.Command("cp", "/etc/default/grub", "/etc/default/grub.backup")
	if err := backupCmd.Run(); err != nil {
		LogMessage(Warn, fmt.Sprintf("Failed to backup GRUB config: %v", err))
	} else {
		LogMessage(Info, "Backed up /etc/default/grub to /etc/default/grub.backup")
	}

	// Check if GRUB config has the blacklist
	checkGrubCmd := exec.Command("sh", "-c", "grep 'modprobe.blacklist=amdgpu' /etc/default/grub || true")
	checkGrubOutput, _ := checkGrubCmd.CombinedOutput()
	if len(checkGrubOutput) > 0 {
		needsReboot = true
		LogMessage(Info, "Found amdgpu blacklist in GRUB config, removing...")
	}

	// Remove modprobe.blacklist=amdgpu from GRUB
	grubSedCmd := exec.Command("sh", "-c", `sed -i 's/modprobe\.blacklist=amdgpu[[:space:]]*//g' /etc/default/grub`)
	if output, err := grubSedCmd.CombinedOutput(); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to update GRUB config: %s", string(output)))
		return needsReboot, fmt.Errorf("failed to update GRUB configuration: %w", err)
	}
	LogMessage(Info, "Successfully updated /etc/default/grub")

	// Step 4: Verify GRUB changes
	verifyGrubCmd := exec.Command("sh", "-c", "grep -E 'GRUB_CMDLINE_LINUX' /etc/default/grub")
	if verifyOutput, err := verifyGrubCmd.CombinedOutput(); err == nil {
		LogMessage(Info, fmt.Sprintf("Updated GRUB config:\n%s", string(verifyOutput)))
	}

	// Step 5: Update GRUB
	LogMessage(Info, "Running update-grub...")
	updateGrubCmd := exec.Command("update-grub")
	if output, err := updateGrubCmd.CombinedOutput(); err != nil {
		LogMessage(Error, fmt.Sprintf("update-grub failed: %s", string(output)))
		return needsReboot, fmt.Errorf("failed to run update-grub: %w", err)
	}
	LogMessage(Info, "Successfully ran update-grub")

	// Step 6: Verify no active blacklist in config files
	verifyCmd := exec.Command("sh", "-c", "grep -r '^[[:space:]]*blacklist[[:space:]]*amdgpu' /etc/modprobe.d/ 2>/dev/null || true")
	verifyOutput, _ := verifyCmd.CombinedOutput()
	if len(verifyOutput) > 0 {
		LogMessage(Warn, fmt.Sprintf("WARNING: Still found uncommented blacklist entries:\n%s", string(verifyOutput)))
		return needsReboot, fmt.Errorf("blacklist entries still present after cleanup")
	} else {
		LogMessage(Info, "Verified: No active amdgpu blacklist entries in /etc/modprobe.d/")
	}

	LogMessage(Info, "Successfully removed amdgpu blacklist from modprobe.d and GRUB")

	if needsReboot {
		LogMessage(Warn, "════════════════════════════════════════════════════════")
		LogMessage(Warn, "  REBOOT REQUIRED FOR CHANGES TO TAKE EFFECT!")
		LogMessage(Warn, "  The kernel was booted with amdgpu blacklisted.")
		LogMessage(Warn, "  Run: sudo reboot")
		LogMessage(Warn, "════════════════════════════════════════════════════════")
	}

	return needsReboot, nil
}

// verifyAmdgpuDriverBinding verifies that the amdgpu module is loaded and GPUs are bound
func verifyAmdgpuDriverBinding() error {
	LogMessage(Info, "Verifying amdgpu driver binding...")

	// Check if amdgpu module is loaded
	lsmodCmd := exec.Command("sh", "-c", "lsmod | grep '^amdgpu' || true")
	lsmodOutput, _ := lsmodCmd.CombinedOutput()
	if len(lsmodOutput) == 0 {
		LogMessage(Error, "amdgpu module is NOT loaded")
		return fmt.Errorf("amdgpu module not loaded")
	}
	LogMessage(Info, "amdgpu module is loaded")

	// Check if GPUs are bound to amdgpu driver
	lspciCmd := exec.Command("sh", "-c", "lspci -nnk -d 1002:75a3 | grep -A 3 'Processing accelerators'")
	lspciOutput, _ := lspciCmd.CombinedOutput()

	if !strings.Contains(string(lspciOutput), "Kernel driver in use: amdgpu") {
		LogMessage(Error, "MI355X GPUs are NOT bound to amdgpu driver")
		LogMessage(Info, fmt.Sprintf("lspci output:\n%s", string(lspciOutput)))
		return fmt.Errorf("GPUs not bound to amdgpu driver")
	}
	LogMessage(Info, "MI355X GPUs are bound to amdgpu driver")

	// Check for render nodes
	renderNodesCmd := exec.Command("sh", "-c", "ls -la /dev/dri/renderD* 2>/dev/null || true")
	renderNodesOutput, _ := renderNodesCmd.CombinedOutput()
	if len(renderNodesOutput) == 0 {
		LogMessage(Warn, "No render nodes found in /dev/dri/")
		return fmt.Errorf("no render nodes found")
	}
	LogMessage(Info, fmt.Sprintf("Render nodes found:\n%s", string(renderNodesOutput)))

	LogMessage(Info, "Successfully verified amdgpu driver binding")
	return nil
}
