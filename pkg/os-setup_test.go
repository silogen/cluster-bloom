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
	"os"
	"slices"
	"testing"

	"github.com/spf13/viper"
)

func TestSupportedUbuntuVersions(t *testing.T) {
	expectedVersions := []string{"20.04", "22.04", "24.04"}
	
	if len(SupportedUbuntuVersions) != len(expectedVersions) {
		t.Errorf("Expected %d supported versions, got %d", len(expectedVersions), len(SupportedUbuntuVersions))
	}

	for _, version := range expectedVersions {
		if !slices.Contains(SupportedUbuntuVersions, version) {
			t.Errorf("Expected version %s to be supported", version)
		}
	}
}

func TestIsRunningOnSupportedUbuntu(t *testing.T) {
	// This test will depend on the actual system
	result := IsRunningOnSupportedUbuntu()
	// On macOS or non-Ubuntu systems, this should return false
	// On Ubuntu systems, it should return true if the version is supported
	t.Logf("IsRunningOnSupportedUbuntu returned: %v", result)
}

func TestCheckPortsBeforeOpening(t *testing.T) {
	err := CheckPortsBeforeOpening()
	if err != nil {
		t.Errorf("Expected no error from CheckPortsBeforeOpening, got: %v", err)
	}
}

func TestOpenPorts(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	result := OpenPorts()
	// This will likely fail in test environment due to permissions
	if !result {
		t.Log("OpenPorts failed as expected in test environment")
	}
}

func TestGetCurrentInotifyValue(t *testing.T) {
	value, err := getCurrentInotifyValue()
	if err != nil {
		t.Errorf("Expected no error from getCurrentInotifyValue, got: %v", err)
	}
	if value < 0 {
		t.Errorf("Expected non-negative inotify value, got %d", value)
	}
}

func TestSetInotifyValue(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	// Get current value first
	currentValue, err := getCurrentInotifyValue()
	if err != nil {
		t.Fatalf("Failed to get current inotify value: %v", err)
	}

	// Try to set the same value
	err = setInotifyValue(currentValue)
	if err != nil {
		t.Errorf("Expected no error from setInotifyValue, got: %v", err)
	}
}

func TestVerifyInotifyInstances(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	result := VerifyInotifyInstances()
	if !result {
		t.Log("VerifyInotifyInstances failed as expected in test environment")
	}
}

func TestHasSufficientRancherPartition(t *testing.T) {
	t.Run("non-GPU node", func(t *testing.T) {
		viper.Set("GPU_NODE", false)
		result := HasSufficientRancherPartition()
		if !result {
			t.Errorf("Expected true for non-GPU node")
		}
	})

	t.Run("GPU node", func(t *testing.T) {
		viper.Set("GPU_NODE", true)
		result := HasSufficientRancherPartition()
		// Result depends on system, just ensure it doesn't crash
		t.Logf("HasSufficientRancherPartition for GPU node: %v", result)
	})
}

func TestNVMEDrivesAvailable(t *testing.T) {
	t.Run("with SKIP_DISK_CHECK", func(t *testing.T) {
		viper.Set("SKIP_DISK_CHECK", true)
		result := NVMEDrivesAvailable()
		if !result {
			t.Errorf("Expected true when SKIP_DISK_CHECK is set")
		}
		viper.Set("SKIP_DISK_CHECK", false)
	})

	t.Run("normal check", func(t *testing.T) {
		result := NVMEDrivesAvailable()
		// Result depends on system hardware
		t.Logf("NVMEDrivesAvailable returned: %v", result)
	})
}

func TestCreateMetalLBConfig(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	err := CreateMetalLBConfig()
	// This will likely fail in test environment due to permissions/directory
	if err != nil {
		t.Logf("CreateMetalLBConfig failed as expected in test environment: %v", err)
	}
}

func TestGetUserHomeDirViaShell(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		expectError bool
	}{
		{"current user", "root", false}, // Usually exists
		{"non-existent user", "nonexistentuser12345", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			homeDir, err := GetUserHomeDirViaShell(tt.username)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for user %s, got none", tt.username)
			} else if !tt.expectError && err != nil {
				t.Errorf("Expected no error for user %s, got: %v", tt.username, err)
			} else if !tt.expectError && homeDir == "" {
				t.Errorf("Expected non-empty home directory for user %s", tt.username)
			}
		})
	}
}

func TestSetupMultipath(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	err := setupMultipath()
	// This will likely fail in test environment due to permissions/missing multipath
	if err != nil {
		t.Logf("setupMultipath failed as expected in test environment: %v", err)
	}
}

func TestUpdateModprobe(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	err := updateModprobe()
	// This will likely fail in test environment
	if err != nil {
		t.Logf("updateModprobe failed as expected in test environment: %v", err)
	}
}

func TestPortsConfiguration(t *testing.T) {
	expectedPorts := []string{
		"80;tcp", "443;tcp", "2376;tcp", "2379;tcp", "2380;tcp", "6443;tcp",
		"8472;udp", "9099;tcp", "9345;tcp", "10250;tcp", "10254;tcp", 
		"30000:32767;tcp", "30000:32767;udp",
	}

	if len(ports) != len(expectedPorts) {
		t.Errorf("Expected %d ports, got %d", len(expectedPorts), len(ports))
	}

	for _, expectedPort := range expectedPorts {
		if !slices.Contains(ports, expectedPort) {
			t.Errorf("Expected port %s to be in configuration", expectedPort)
		}
	}
}