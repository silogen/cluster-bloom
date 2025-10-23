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

package sysvalidation

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/silogen/cluster-bloom/pkg/command"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Disk space requirements (in GB)
const (
	MinRootPartitionSizeGB      = 20
	MinRootAvailableSpaceGB     = 10
	MinVarAvailableSpaceGB      = 5
	RecommendedRootPartitionGB  = 20
)

// Memory requirements (in GB)
const (
	MinMemoryGB         = 4
	RecommendedMemoryGB = 8
)

// CPU requirements
const (
	MinCPUCores         = 2
	RecommendedCPUCores = 4
)

// Supported Ubuntu versions
var SupportedUbuntuVersions = []string{"20.04", "22.04", "24.04"}

// Required kernel modules
var (
	RequiredKernelModules = []string{
		"overlay",      // Required for container runtimes
		"br_netfilter", // Required for Kubernetes networking
	}
	RequiredGPUModules = []string{
		"amdgpu", // AMD GPU driver
	}
)

// ValidateResourceRequirements validates system resource requirements and compatibility
func ValidateResourceRequirements() error {
	// Validate partition sizes and disk space
	if err := validateDiskSpace(); err != nil {
		return err
	}

	// Validate memory and CPU requirements
	if err := validateSystemResources(); err != nil {
		return err
	}

	// Validate Ubuntu version compatibility
	if err := validateUbuntuVersion(); err != nil {
		return err
	}

	// Validate required kernel modules and drivers (non-fatal warnings)
	validateKernelModules()

	return nil
}

// validateDiskSpace checks partition sizes and available disk space
func validateDiskSpace() error {
	// Check root partition size (minimum 20GB recommended for Kubernetes)
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		log.Warnf("Could not check root partition size: %v", err)
		return nil // Non-fatal
	}

	// Calculate available space in GB
	availableGB := float64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
	totalGB := float64(stat.Blocks*uint64(stat.Bsize)) / (1024 * 1024 * 1024)

	if totalGB < MinRootPartitionSizeGB {
		log.Warnf("Root partition size is %.1fGB, recommended minimum is %dGB for Kubernetes", totalGB, RecommendedRootPartitionGB)
	}

	if availableGB < MinRootAvailableSpaceGB {
		return fmt.Errorf("insufficient disk space: %.1fGB available, minimum %dGB required", availableGB, MinRootAvailableSpaceGB)
	}

	// Check /var partition if it exists separately
	if err := syscall.Statfs("/var", &stat); err == nil {
		varAvailableGB := float64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
		if varAvailableGB < MinVarAvailableSpaceGB {
			log.Warnf("/var partition has only %.1fGB available, recommend at least %dGB for container images", varAvailableGB, MinVarAvailableSpaceGB)
		}
	}

	return nil
}

// validateSystemResources checks memory and CPU requirements
func validateSystemResources() error {
	// Check memory requirements (minimum 4GB for Kubernetes)
	memInfo, err := os.Open("/proc/meminfo")
	if err != nil {
		log.Warnf("Could not read memory information: %v", err)
		return nil // Non-fatal
	}
	defer memInfo.Close()

	scanner := bufio.NewScanner(memInfo)
	var totalMemKB int64
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if mem, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
					totalMemKB = mem
					break
				}
			}
		}
	}

	if totalMemKB > 0 {
		totalMemGB := float64(totalMemKB) / (1024 * 1024)
		if totalMemGB < MinMemoryGB {
			return fmt.Errorf("insufficient memory: %.1fGB available, minimum %dGB required for Kubernetes", totalMemGB, MinMemoryGB)
		}
		if totalMemGB < RecommendedMemoryGB {
			log.Warnf("Memory is %.1fGB, recommend at least %dGB for optimal performance", totalMemGB, RecommendedMemoryGB)
		}
	}

	// Check CPU count (minimum 2 cores for Kubernetes)
	cpuInfo, err := os.Open("/proc/cpuinfo")
	if err != nil {
		log.Warnf("Could not read CPU information: %v", err)
		return nil // Non-fatal
	}
	defer cpuInfo.Close()

	cpuCount := 0
	scanner = bufio.NewScanner(cpuInfo)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "processor") {
			cpuCount++
		}
	}

	if cpuCount < MinCPUCores {
		return fmt.Errorf("insufficient CPU cores: %d available, minimum %d cores required for Kubernetes", cpuCount, MinCPUCores)
	}
	if cpuCount < RecommendedCPUCores {
		log.Warnf("CPU has %d cores, recommend at least %d cores for optimal performance", cpuCount, RecommendedCPUCores)
	}

	return nil
}

// validateUbuntuVersion checks Ubuntu version compatibility
func validateUbuntuVersion() error {
	// Read /etc/os-release for Ubuntu version information
	osRelease, err := os.Open("/etc/os-release")
	if err != nil {
		log.Warnf("Could not read OS release information: %v", err)
		return nil // Non-fatal, might not be Ubuntu
	}
	defer osRelease.Close()

	scanner := bufio.NewScanner(osRelease)
	var distroID, versionID string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			distroID = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		}
		if strings.HasPrefix(line, "VERSION_ID=") {
			versionID = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}

	// Check if it's Ubuntu
	if distroID != "ubuntu" {
		log.Warnf("Not running on Ubuntu (detected: %s) - some features may not work as expected", distroID)
		return nil
	}

	// Validate Ubuntu version
	supported := false
	for _, version := range SupportedUbuntuVersions {
		if versionID == version {
			supported = true
			break
		}
	}

	if !supported {
		log.Warnf("Ubuntu version %s may not be fully supported. Supported versions: %s",
			versionID, strings.Join(SupportedUbuntuVersions, ", "))
	}

	return nil
}

// validateKernelModules checks for required kernel modules and drivers
func validateKernelModules() {
	// Check for required kernel modules (non-fatal, just warnings)
	for _, module := range RequiredKernelModules {
		if !isModuleLoaded(module) && !isModuleAvailable(module) {
			log.Warnf("Kernel module '%s' is not loaded and may not be available - this could cause issues", module)
		}
	}

	// Check for GPU-related modules if GPU_NODE is true
	if viper.GetBool("GPU_NODE") {
		for _, module := range RequiredGPUModules {
			if !isModuleLoaded(module) && !isModuleAvailable(module) {
				log.Warnf("GPU module '%s' is not loaded - GPU functionality may not work", module)
			}
		}
	}

	// Check if Docker/containerd can use overlay2 storage driver
	if !isModuleLoaded("overlay") {
		log.Warnf("Overlay filesystem module not loaded - container runtime may fall back to less efficient storage driver")
	}
}

// isModuleLoaded checks if a kernel module is currently loaded
func isModuleLoaded(moduleName string) bool {
	output, err := command.Output(true, "lsmod") // Read-only: run in dry-run
	if err != nil {
		return false
	}
	return strings.Contains(string(output), moduleName)
}

// isModuleAvailable checks if a kernel module is available to load
func isModuleAvailable(moduleName string) bool {
	err := command.SimpleRun(true, "modinfo", moduleName) // Read-only: run in dry-run
	return err == nil
}
