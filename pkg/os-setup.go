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
	"strconv"
	"strings"
)

var SupportedUbuntuVersions = []string{"20.04", "22.04", "24.04"}

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

func OpenPorts() bool {
	ports := []string{
		"22;tcp", "80;tcp", "443;tcp", "2376;tcp", "2379;tcp", "2380;tcp", "6443;tcp",
		"8472;udp", "9099;tcp", "9345;tcp", "10250;tcp", "10254;tcp", "30000:32767;tcp", "30000:32767;udp",
	}

	for _, entry := range ports {
		parts := strings.Split(entry, ";")
		port, protocol := parts[0], parts[1]
		cmd := exec.Command("sudo", "iptables", "-A", "INPUT", "-p", protocol, "-m", "state", "--state", "NEW", "-m", protocol, "--dport", port, "-j", "ACCEPT")
		if err := cmd.Run(); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to open port %s/%s: %v", port, protocol, err))
			return true
		}
		LogMessage(Debug, fmt.Sprintf("Opened port %s/%s", port, protocol))
	}
	if err := exec.Command("sudo", "iptables-save").Run(); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to save iptables rules: %v", err))
		return true
	}

	LogMessage(Debug, "All iptables rules have been added and saved.")
	return false
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
