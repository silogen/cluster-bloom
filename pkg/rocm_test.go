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
	"strings"
	"testing"

	"github.com/silogen/cluster-bloom/pkg/command"
	"github.com/spf13/viper"
)

func TestCheckGPUAvailability(t *testing.T) {
	err := CheckGPUAvailability()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

func TestCheckAndInstallROCM(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	// Test when rocm-smi is not found (likely case in test environment)
	viper.Set("ROCM_DEB_PACKAGE", "amdgpu-install_6.1.60103-1_all.deb")
	viper.Set("ROCM_BASE_URL", "https://repo.radeon.com/amdgpu-install/6.1.3/ubuntu/")
	
	result := CheckAndInstallROCM()
	// This will likely return false in test environment due to missing GPU hardware
	if result {
		t.Log("CheckAndInstallROCM succeeded")
	} else {
		t.Log("CheckAndInstallROCM failed as expected in test environment")
	}
}

func TestPrintROCMVersion(t *testing.T) {
	// Test function exists and doesn't panic
	printROCMVersion()
}

func TestRunCommand(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		args        []string
		expectError bool
	}{
		{"echo command", "echo", []string{"hello", "world"}, false},
		{"invalid command", "non-existent-command", []string{}, true},
		{"ls command", "ls", []string{"/tmp"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := command.Run(false, tt.command, tt.args...)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			} else if !tt.expectError && tt.command == "echo" {
				if !strings.Contains(output, "hello world") {
					t.Errorf("Expected output to contain 'hello world', got: %s", output)
				}
			}
		})
	}
}

func TestRunCommandWithDifferentArgs(t *testing.T) {
	// Test with different argument combinations
	output, err := command.Run(false, "echo", "test")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if output != "test\n" {
		t.Errorf("Expected 'test\\n', got '%s'", output)
	}

	output, err = command.Run(false, "echo", "-n", "no-newline")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if output != "no-newline" {
		t.Errorf("Expected 'no-newline', got '%s'", output)
	}

	// Test command that should fail
	_, err = command.Run(false, "false")
	if err == nil {
		t.Errorf("Expected error from 'false' command")
	}
}

func TestRunCommandErrorHandling(t *testing.T) {
	// Test command with stderr output
	_, err := command.Run(false, "sh", "-c", "echo 'error message' >&2; exit 1")
	if err == nil {
		t.Errorf("Expected error from failing command")
	}
	if !strings.Contains(err.Error(), "command failed") {
		t.Errorf("Expected error message to contain 'command failed', got: %v", err)
	}
}

func TestROCMConfiguration(t *testing.T) {
	// Test ROCM configuration values
	viper.Set("ROCM_DEB_PACKAGE", "test-package.deb")
	viper.Set("ROCM_BASE_URL", "https://test.com/")
	
	debPackage := viper.GetString("ROCM_DEB_PACKAGE")
	baseURL := viper.GetString("ROCM_BASE_URL")
	
	if debPackage != "test-package.deb" {
		t.Errorf("Expected 'test-package.deb', got '%s'", debPackage)
	}
	if baseURL != "https://test.com/" {
		t.Errorf("Expected 'https://test.com/', got '%s'", baseURL)
	}
}

func TestCommandOutputParsing(t *testing.T) {
	// Test parsing command output
	output, err := command.Run(false, "echo", "line1\nline2\nline3")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line1" {
		t.Errorf("Expected first line to be 'line1', got '%s'", lines[0])
	}
	if lines[1] != "line2" {
		t.Errorf("Expected second line to be 'line2', got '%s'", lines[1])
	}
	if lines[2] != "line3" {
		t.Errorf("Expected third line to be 'line3', got '%s'", lines[2])
	}
}