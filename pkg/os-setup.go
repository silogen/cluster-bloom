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
	"path"
	"strconv"
	"strings"

	"github.com/silogen/cluster-bloom/pkg/command"
	"github.com/silogen/cluster-bloom/pkg/fsops"
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
		portCmd := command.Cmd("lsof", "-i", fmt.Sprintf("%s:%s", strings.ToUpper(protocol), port))
		var portOutput bytes.Buffer
		if portCmd != nil {
			portCmd.Stdout = &portOutput
			portCmd.Stderr = &portOutput
		}

		var err error
		if portCmd != nil {
			err = portCmd.Run()
		}
		if err != nil {
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
		if command.SimpleRun(false, "sudo", "iptables", "-C", "INPUT", "-p", protocol, "-m", "state", "--state", "NEW", "-m", protocol, "--dport", port, "-j", "ACCEPT") == nil {
			// Rule already exists
			LogMessage(Info, fmt.Sprintf("Rule for %s/%s already exists", port, protocol))
			continue
		}

		// Add the rule
		if err := command.SimpleRun(false, "sudo", "iptables", "-A", "INPUT", "-p", protocol, "-m", "state", "--state", "NEW", "-m", protocol, "--dport", port, "-j", "ACCEPT"); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to open port %s/%s: %v", port, protocol, err))
			return false
		}
		LogMessage(Info, fmt.Sprintf("Opened port %s/%s", port, protocol))
	}
	if err := command.SimpleRun(false, "sudo", "iptables-save"); err != nil {
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
	cmd := command.Cmd("sysctl", "-n", sysctlParam)
	if cmd == nil {
		return targetValue, nil // Return target value in dry-run mode
	}
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
	return command.SimpleRun(false, "sudo", "sysctl", "-w", fmt.Sprintf("%s=%d", sysctlParam, value))
}

func CheckInotifyConfig() error {
	currentValue, err := getCurrentInotifyValue()
	if err != nil {
		return fmt.Errorf("Failed to get current inotify instances: " + err.Error())
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

	return fsops.WriteFile(sysctlFile, []byte(strings.Join(lines, "\n")+"\n"), 0644)
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
	output, err := command.Output(false, "mkdir", "-p", "/var/lib/rancher")
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to create /var/lib/rancher: %v", err))
		return false
	}
	output, err = command.Output(true, "df", "-BG", "/var/lib/rancher")
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
	output, err := command.Output(true, "sh", "-c", "ip route get 1 | awk '{print $7; exit}'")
	if err != nil{
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
	if err := fsops.WriteFile(manifestPath, []byte(metallbConfig), 0644); err != nil {
		return fmt.Errorf("failed to write MetalLB configuration to %s: %v", manifestPath, err)
	}

	LogMessage(Info, fmt.Sprintf("MetalLB configuration written to %s with IP %s", manifestPath, defaultIP))
	return nil
}

// GetUserHomeDirViaShell gets a user's home directory using shell tilde expansion
func GetUserHomeDirViaShell(username string) (string, error) {
	// Use shell's tilde expansion to get the home directory
	output, err := command.Output(true, "sh", "-c", fmt.Sprintf("eval echo ~%s", username))
	if err != nil{
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

		err = fsops.WriteFile(configFile, []byte(configContent), 0644)
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

			err = fsops.WriteFile(configFile, []byte(newConfigData), 0644)
			if err != nil {
				return fmt.Errorf("failed to update multipath.conf: %w", err)
			}

			// Restart multipath service
			LogMessage(Info, "Restarting multipathd.service...")
			_, err = command.Run(false, "systemctl", "restart", "multipathd.service")
			if err != nil {
				return fmt.Errorf("failed to restart multipathd service: %w", err)
			}

			// Verify configuration
			LogMessage(Info, "Verifying multipath configuration...")
			output, err := command.Run(true, "multipath", "-t")
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

func updateModprobe() error {
	output, err := command.Output(true, "sh", "-c", "sudo sed -i '/^blacklist amdgpu/s/^/# /' /etc/modprobe.d/*.conf")
	if err != nil{
		LogMessage(Warn, fmt.Sprintf("Modprobe configuration returned: %s", output))
		return fmt.Errorf("failed to configure Modprobe: %w", err)
	} else {
		LogMessage(Info, "")
	}
	output, err = command.Output(true, "modprobe", "amdgpu")
	if err != nil{
		LogMessage(Warn, fmt.Sprintf("Modprobe amdgpu returned: %s", output))
		return fmt.Errorf("failed to modprobe amdgpu: %w", err)
	} else {
		LogMessage(Info, "")
	}
	return nil
}
