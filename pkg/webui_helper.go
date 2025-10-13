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
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

func GetPriorLonghornDisks(longhornFromConfig map[string]interface{}) (disks []string, mountPoints map[string]string, err error) {
	mountPoints = make(map[string]string)

	// Step 1: Try LONGHORN_DISKS first
	disks, mountPoints, err = GetDisksFromLonghornConfig(longhornFromConfig["longhorn_disks"].(string))
	if err != nil {
		LogMessage(Warn, fmt.Sprintf("GetDisksFromLonghornConfig failed: %v", err))
	} else if len(disks) > 0 {
		LogMessage(Info, "Successfully found disks from LONGHORN_DISKS configuration")
		return disks, mountPoints, nil
	}

	// Step 2: Try SELECTED_DISKS if Step 1 failed or returned no disks
	disks, mountPoints, err = GetDisksFromSelectedConfig(longhornFromConfig["selected_disks"].(string))
	if err != nil {
		LogMessage(Warn, fmt.Sprintf("GetDisksFromSelectedConfig failed: %v", err))
	} else if len(disks) > 0 {
		LogMessage(Info, "Successfully found disks from SELECTED_DISKS configuration")
		return disks, mountPoints, nil
	}

	// Step 3: Try bloom.log if Step 1 and 2 failed or returned no disks
	disks, mountPoints, err = GetDisksFromBloomLog()
	if err != nil {
		LogMessage(Warn, fmt.Sprintf("GetDisksFromBloomLog failed: %v", err))
	} else if len(disks) > 0 {
		LogMessage(Info, "Successfully found disks from bloom.log")
		return disks, mountPoints, nil
	}

	// All functions failed or returned no disks
	LogMessage(Error, "No longhorn disks found from any source (LONGHORN_DISKS, SELECTED_DISKS, bloom.log)")
	return nil, nil, fmt.Errorf("no longhorn disks found from any configuration source")
}

func GetDisksFromLonghornConfig(longhornFromConfig string) (disks []string, mountPoints map[string]string, e error) {
	// Accept LONGHORN_DISKS in any case (e.g., longhorn_disks, Longhorn_Disks)
	var val string
	mountPoints = make(map[string]string)

	if longhornFromConfig == "" {
		for _, k := range viper.AllKeys() {
			if strings.ToLower(k) == "longhorn_disks" {
				val = viper.GetString(k)
				// ensure canonical key exists for the rest of the function
				viper.Set("LONGHORN_DISKS", val)
				break
			}
		}
		// fallback: try direct lookup (covers env-style keys)
		if val == "" {
			val = viper.GetString("LONGHORN_DISKS")
		}
		if val == "" {
			return nil, nil, nil
		}
	} else {
		val = longhornFromConfig
	}

	LogMessage(Info, "Found LONGHORN_DISKS configuration")
	longhornDiskPaths := strings.Split(val, ",")
	var targetDisks []string

	for _, diskPath := range longhornDiskPaths {
		diskPath = strings.TrimSpace(diskPath)
		if diskPath == "" {
			continue
		}

		device, mountPoint := getDeviceFromMountpoint(diskPath)
		targetDisks = append(targetDisks, device)
		mountPoints[device] = mountPoint
	}

	if len(targetDisks) > 0 {
		LogMessage(Info, fmt.Sprintf("Found %d longhorn disks from LONGHORN_DISKS", len(targetDisks)))
		return targetDisks, mountPoints, nil
	}

	LogMessage(Info, "No valid devices found from LONGHORN_DISKS")
	return nil, nil, nil
}

func getDeviceFromMountpoint(diskPath string) (device string, mountPoint string) {
	mountPoint = "/mnt/" + diskPath
	cmd := exec.Command("lsblk", "-no", "NAME,MOUNTPOINT")
	output, err := cmd.Output()
	if err != nil {
		LogMessage(Warn, fmt.Sprintf("Failed to run lsblk: %v", err))
		return "", ""
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	devicePath := ""
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[1] == mountPoint {
			devicePath = "/dev/" + fields[0]
			//targetDisks = append(targetDisks, devicePath)
			LogMessage(Debug, fmt.Sprintf("Found device %s for mount path %s", devicePath, mountPoint))
			break
		}
	}
	return devicePath, mountPoint
}

func getMountpointsFromDevice(device string) string {
	var mountPoints []string
	
	// Check for regular mount points from /proc/mounts
	shellCmd := fmt.Sprintf("awk '$1 == \"%s\" {print $2}' /proc/mounts", device)
	cmd := exec.Command("sh", "-c", shellCmd)
	output, err := cmd.Output()
	if err == nil {
		mounts := strings.TrimSpace(string(output))
		if mounts != "" {
			mountPoints = append(mountPoints, strings.Split(mounts, "\n")...)
		}
	}
	
	// Check if device is used as swap
	swapCmd := fmt.Sprintf("awk '$1 == \"%s\" {print \"[swap]\"}' /proc/swaps", device)
	cmd = exec.Command("sh", "-c", swapCmd)
	swapOutput, err := cmd.Output()
	if err == nil {
		swap := strings.TrimSpace(string(swapOutput))
		if swap != "" {
			mountPoints = append(mountPoints, swap)
		}
	}
	
	// Check partitions of the device for swap
	lsblkCmd := fmt.Sprintf("lsblk -no NAME,FSTYPE %s 2>/dev/null | awk '$2 == \"swap\" {print \"/dev/\" $1 \" [swap]\"}'", device)
	cmd = exec.Command("sh", "-c", lsblkCmd)
	lsblkOutput, err := cmd.Output()
	if err == nil {
		partitionSwaps := strings.TrimSpace(string(lsblkOutput))
		if partitionSwaps != "" {
			for _, line := range strings.Split(partitionSwaps, "\n") {
				if line != "" {
					mountPoints = append(mountPoints, line)
				}
			}
		}
	}
	
	return strings.Join(mountPoints, ",")
}

func GetDisksFromSelectedConfig(longhornFromConfig string) (disks []string, mountPoints map[string]string, e error) {
	var targetDisks []string
	mountPoints = make(map[string]string)

	if longhornFromConfig == "" {
		if !viper.IsSet("SELECTED_DISKS") || viper.GetString("SELECTED_DISKS") == "" {
			return nil, nil, nil
		}

		LogMessage(Info, "Found SELECTED_DISKS configuration")
		disks := strings.Split(viper.GetString("SELECTED_DISKS"), ",")

		for _, disk := range disks {
			disk = strings.TrimSpace(disk)
			if disk != "" {
				targetDisks = append(targetDisks, disk)
			}
		}

		if len(targetDisks) == 0 {
			LogMessage(Info, "No valid disks found in SELECTED_DISKS")
			return nil, nil, nil
		}
	} else {
		targetDisks = strings.Split(longhornFromConfig, ",")
	}

	diskInfo := ""
	for _, disk := range targetDisks {
		cmd := exec.Command("lsblk", "-no", "SIZE,MODEL,MOUNTPOINT", disk)
		output, err := cmd.Output()
		if err != nil {
			diskInfo += fmt.Sprintf("%s: (Unable to get info)\n", disk)
		} else {
			info := strings.TrimSpace(string(output))
			diskInfo += fmt.Sprintf("%s: %s\n", disk, info)
		}
		mountPoints[disk] = getMountpointsFromDevice(disk)
	}

	LogMessage(Info, fmt.Sprintf("Found %d disks from SELECTED_DISKS:\n%s", len(targetDisks), diskInfo))
	return targetDisks, mountPoints, nil
}

func GetDisksFromBloomLog() (disks []string, mountPoints map[string]string, e error) {
	LogMessage(Info, "Searching bloom.log for selected disks")
	logFile := "bloom.log"
	content, err := os.ReadFile(logFile)
	if err != nil {
		LogMessage(Warn, fmt.Sprintf("Failed to read bloom.log: %v", err))
		return nil, nil, nil
	}

	lines := strings.Split(string(content), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if strings.Contains(line, "[blue]Message: Selected disks:") {
			re := regexp.MustCompile(`\[blue\]Message: Selected disks: \[(.*?)\]`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				diskListStr := matches[1]
				targetDisks := strings.Fields(diskListStr)

				if len(targetDisks) == 0 {
					LogMessage(Info, "Found bloom.log entry but no disks listed")
					return nil, nil, nil
				}

				LogMessage(Info, fmt.Sprintf("Found %d disks from bloom.log", len(targetDisks)))
				return targetDisks, mountPoints, nil
			}
			break
		}
	}

	LogMessage(Info, "No '[blue]Message: Selected disks:' found in bloom.log")
	return nil, nil, nil
}
