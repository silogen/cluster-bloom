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

package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		paramName string
		wantErr   bool
	}{
		{"Valid HTTPS URL", "https://example.com", "OIDC_URL", false},
		{"Valid HTTP URL", "http://example.com", "ROCM_BASE_URL", false},
		{"Valid URL with path", "https://github.com/user/repo/releases/download/v1.0/file.tar.gz", "CLUSTERFORGE_RELEASE", false},
		{"Empty URL (allowed)", "", "OIDC_URL", false},
		{"CLUSTERFORGE_RELEASE none value", "none", "CLUSTERFORGE_RELEASE", false},
		{"CLUSTERFORGE_RELEASE NONE value", "NONE", "CLUSTERFORGE_RELEASE", false},
		{"Invalid scheme FTP", "ftp://example.com", "OIDC_URL", true},
		{"Invalid scheme file", "file:///path/to/file", "ROCM_BASE_URL", true},
		{"Missing scheme", "example.com", "OIDC_URL", true},
		{"Invalid URL format", "ht!tp://example.com", "RKE2_INSTALLATION_URL", true},
		{"Missing host", "https://", "OIDC_URL", true},
		{"Just protocol", "https", "OIDC_URL", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.url, tt.paramName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAllURLs(t *testing.T) {
	// Save original config
	originalViper := viper.AllSettings()
	defer func() {
		viper.Reset()
		for k, v := range originalViper {
			viper.Set(k, v)
		}
	}()

	tests := []struct {
		name    string
		config  map[string]string
		wantErr bool
	}{
		{
			name: "All valid URLs",
			config: map[string]string{
				"OIDC_URL":              "https://auth.example.com",
				"CLUSTERFORGE_RELEASE":  "https://github.com/example/repo/releases/download/v1.0/release.tar.gz",
				"ROCM_BASE_URL":        "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/",
				"RKE2_INSTALLATION_URL": "https://get.rke2.io",
			},
			wantErr: false,
		},
		{
			name: "Valid with empty optional URLs",
			config: map[string]string{
				"OIDC_URL":              "",
				"CLUSTERFORGE_RELEASE":  "none",
				"ROCM_BASE_URL":        "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/",
				"RKE2_INSTALLATION_URL": "https://get.rke2.io",
			},
			wantErr: false,
		},
		{
			name: "Invalid OIDC_URL",
			config: map[string]string{
				"OIDC_URL":              "ftp://invalid.com",
				"CLUSTERFORGE_RELEASE":  "https://github.com/example/repo/releases/download/v1.0/release.tar.gz",
				"ROCM_BASE_URL":        "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/",
				"RKE2_INSTALLATION_URL": "https://get.rke2.io",
			},
			wantErr: true,
		},
		{
			name: "Invalid ROCM_BASE_URL",
			config: map[string]string{
				"OIDC_URL":              "https://auth.example.com",
				"CLUSTERFORGE_RELEASE":  "https://github.com/example/repo/releases/download/v1.0/release.tar.gz",
				"ROCM_BASE_URL":        "not-a-url",
				"RKE2_INSTALLATION_URL": "https://get.rke2.io",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			for k, v := range tt.config {
				viper.Set(k, v)
			}

			err := validateAllURLs()
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAllURLs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateIPAddress(t *testing.T) {
	tests := []struct {
		name      string
		ip        string
		paramName string
		wantErr   bool
	}{
		{"Valid IPv4 address", "192.168.1.100", "SERVER_IP", false},
		{"Valid IPv4 private address", "10.0.0.1", "SERVER_IP", false},
		{"Valid IPv6 address", "2001:db8::1", "SERVER_IP", false},
		{"Empty IP (allowed)", "", "SERVER_IP", false},
		{"Invalid IP format", "192.168.1", "SERVER_IP", true},
		{"Invalid IP with letters", "192.168.1.abc", "SERVER_IP", true},
		{"Loopback IPv4", "127.0.0.1", "SERVER_IP", true},
		{"Loopback IPv6", "::1", "SERVER_IP", true},
		{"Unspecified IPv4", "0.0.0.0", "SERVER_IP", true},
		{"Unspecified IPv6", "::", "SERVER_IP", true},
		{"Out of range IPv4", "256.256.256.256", "SERVER_IP", true},
		{"Invalid characters", "192.168.1.1.1", "SERVER_IP", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIPAddress(tt.ip, tt.paramName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIPAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAllIPs(t *testing.T) {
	// Save original config
	originalViper := viper.AllSettings()
	defer func() {
		viper.Reset()
		for k, v := range originalViper {
			viper.Set(k, v)
		}
	}()

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
	}{
		{
			name: "FIRST_NODE true - no SERVER_IP validation",
			config: map[string]interface{}{
				"FIRST_NODE": true,
			},
			wantErr: false,
		},
		{
			name: "FIRST_NODE false with valid SERVER_IP",
			config: map[string]interface{}{
				"FIRST_NODE": false,
				"SERVER_IP":  "192.168.1.100",
			},
			wantErr: false,
		},
		{
			name: "FIRST_NODE false with invalid SERVER_IP",
			config: map[string]interface{}{
				"FIRST_NODE": false,
				"SERVER_IP":  "invalid-ip",
			},
			wantErr: true,
		},
		{
			name: "FIRST_NODE false with loopback SERVER_IP",
			config: map[string]interface{}{
				"FIRST_NODE": false,
				"SERVER_IP":  "127.0.0.1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			for k, v := range tt.config {
				viper.Set(k, v)
			}

			err := validateAllIPs()
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAllIPs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateJoinToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"Valid long token", "K10831EXAMPLE::server:aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789", false},
		{"Valid base64-like token", "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXowMTIzNDU2Nzg5", false},
		{"Valid hex token", "a1b2c3d4e5f6789012345678901234567890abcdef1234567890", false},
		{"Valid token with separators", "token_part1.token_part2-token_part3", false},
		{"Empty token (allowed)", "", false},
		{"Too short token", "short", true},
		{"Too long token", strings.Repeat("a", 513), true},
		{"Invalid characters", "token with spaces", true},
		{"Invalid characters special", "token@#$%^&*()", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJoinToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateJoinToken() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateOnePasswordToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"Valid JWT token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", false},
		{"Valid long base64 token", strings.Repeat("YWJjZGVmZ2hpamtsbW5vcA", 3), false},
		{"Valid token with separators", strings.Repeat("token_part1.token_part2-token_part3.", 2), false},
		{"Empty token (allowed)", "", false},
		{"Too short token", "short", true},
		{"Too long token", strings.Repeat("a", 2049), true},
		{"Invalid JWT - 2 parts", "header.payload", true},
		{"Invalid JWT - 4 parts", "header.payload.signature.extra", true},
		{"Invalid JWT - empty part", "header..signature", true},
		{"Invalid characters in JWT", "header@.payload#.signature$", true},
		{"Invalid characters in regular token", "token with spaces", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOnePasswordToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOnePasswordToken() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		paramName string
		wantErr   bool
	}{
		{"Valid JOIN_TOKEN", "K10831EXAMPLE::server:aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789", "JOIN_TOKEN", false},
		{"Valid ONEPASS_CONNECT_TOKEN", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", "ONEPASS_CONNECT_TOKEN", false},
		{"Empty token", "", "JOIN_TOKEN", false},
		{"Unknown token type", "some-token", "UNKNOWN_TOKEN", true},
		{"Invalid JOIN_TOKEN", "short", "JOIN_TOKEN", true},
		{"Invalid ONEPASS_CONNECT_TOKEN", "short", "ONEPASS_CONNECT_TOKEN", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateToken(tt.token, tt.paramName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateToken() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAllTokens(t *testing.T) {
	// Save original config
	originalViper := viper.AllSettings()
	defer func() {
		viper.Reset()
		for k, v := range originalViper {
			viper.Set(k, v)
		}
	}()

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
	}{
		{
			name: "FIRST_NODE true - no JOIN_TOKEN validation",
			config: map[string]interface{}{
				"FIRST_NODE": true,
			},
			wantErr: false,
		},
		{
			name: "FIRST_NODE false with valid JOIN_TOKEN",
			config: map[string]interface{}{
				"FIRST_NODE": false,
				"JOIN_TOKEN": "K10831EXAMPLE::server:aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789",
			},
			wantErr: false,
		},
		{
			name: "FIRST_NODE false with invalid JOIN_TOKEN",
			config: map[string]interface{}{
				"FIRST_NODE": false,
				"JOIN_TOKEN": "short",
			},
			wantErr: true,
		},
		{
			name: "Valid ONEPASS_CONNECT_TOKEN",
			config: map[string]interface{}{
				"FIRST_NODE":               true,
				"ONEPASS_CONNECT_TOKEN":    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			},
			wantErr: false,
		},
		{
			name: "Invalid ONEPASS_CONNECT_TOKEN",
			config: map[string]interface{}{
				"FIRST_NODE":               true,
				"ONEPASS_CONNECT_TOKEN":    "short",
			},
			wantErr: true,
		},
		{
			name: "Both tokens valid",
			config: map[string]interface{}{
				"FIRST_NODE":               false,
				"JOIN_TOKEN":               "K10831EXAMPLE::server:aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789",
				"ONEPASS_CONNECT_TOKEN":    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			for k, v := range tt.config {
				viper.Set(k, v)
			}

			err := validateAllTokens()
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAllTokens() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateStepNames(t *testing.T) {
	tests := []struct {
		name      string
		stepNames string
		paramName string
		wantErr   bool
	}{
		{"Valid single step", "SetupLonghornStep", "DISABLED_STEPS", false},
		{"Valid multiple steps", "SetupLonghornStep,SetupMetallbStep", "DISABLED_STEPS", false},
		{"Valid steps with spaces", "SetupLonghornStep, SetupMetallbStep", "ENABLED_STEPS", false},
		{"Empty string (allowed)", "", "DISABLED_STEPS", false},
		{"Invalid step name", "InvalidStep", "DISABLED_STEPS", true},
		{"Valid and invalid mixed", "SetupLonghornStep,InvalidStep", "ENABLED_STEPS", true},
		{"Invalid step with typo", "SetupLonghornStepTypo", "DISABLED_STEPS", true},
		{"Empty entries", "SetupLonghornStep,,SetupMetallbStep", "DISABLED_STEPS", false},
		{"Trailing comma", "SetupLonghornStep,", "ENABLED_STEPS", false},
		{"Leading comma", ",SetupLonghornStep", "DISABLED_STEPS", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStepNames(tt.stepNames, tt.paramName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateStepNames() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAllStepNames(t *testing.T) {
	// Save original config
	originalViper := viper.AllSettings()
	defer func() {
		viper.Reset()
		for k, v := range originalViper {
			viper.Set(k, v)
		}
	}()

	tests := []struct {
		name    string
		config  map[string]string
		wantErr bool
	}{
		{
			name: "Valid DISABLED_STEPS",
			config: map[string]string{
				"DISABLED_STEPS": "SetupLonghornStep,SetupMetallbStep",
			},
			wantErr: false,
		},
		{
			name: "Valid ENABLED_STEPS",
			config: map[string]string{
				"ENABLED_STEPS": "CheckUbuntuStep,InstallDependentPackagesStep",
			},
			wantErr: false,
		},
		{
			name: "Both valid step parameters",
			config: map[string]string{
				"DISABLED_STEPS": "SetupClusterForgeStep",
				"ENABLED_STEPS":  "CheckUbuntuStep,SetupRKE2Step,FinalOutput",
			},
			wantErr: false,
		},
		{
			name: "Empty step parameters (allowed)",
			config: map[string]string{
				"DISABLED_STEPS": "",
				"ENABLED_STEPS":  "",
			},
			wantErr: false,
		},
		{
			name: "Invalid DISABLED_STEPS",
			config: map[string]string{
				"DISABLED_STEPS": "InvalidStep,SetupLonghornStep",
			},
			wantErr: true,
		},
		{
			name: "Invalid ENABLED_STEPS",
			config: map[string]string{
				"ENABLED_STEPS": "CheckUbuntuStep,NonExistentStep",
			},
			wantErr: true,
		},
		{
			name: "Valid DISABLED_STEPS, invalid ENABLED_STEPS",
			config: map[string]string{
				"DISABLED_STEPS": "SetupLonghornStep",
				"ENABLED_STEPS":  "BadStepName",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			for k, v := range tt.config {
				viper.Set(k, v)
			}

			err := validateAllStepNames()
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAllStepNames() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateConfigurationConflicts(t *testing.T) {
	// Save original config
	originalViper := viper.AllSettings()
	defer func() {
		viper.Reset()
		for k, v := range originalViper {
			viper.Set(k, v)
		}
	}()

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
		wantLog bool // Whether we expect warning logs
	}{
		{
			name: "Valid first node configuration",
			config: map[string]interface{}{
				"FIRST_NODE": true,
				"GPU_NODE":   true,
			},
			wantErr: false,
			wantLog: false,
		},
		{
			name: "Valid additional node configuration",
			config: map[string]interface{}{
				"FIRST_NODE": false,
				"SERVER_IP":  "192.168.1.100",
				"JOIN_TOKEN": "K10831EXAMPLE::server:aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789",
			},
			wantErr: false,
			wantLog: false,
		},
		{
			name: "FIRST_NODE=false missing SERVER_IP",
			config: map[string]interface{}{
				"FIRST_NODE": false,
				"JOIN_TOKEN": "K10831EXAMPLE::server:aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789",
			},
			wantErr: true,
			wantLog: false,
		},
		{
			name: "FIRST_NODE=false missing JOIN_TOKEN",
			config: map[string]interface{}{
				"FIRST_NODE": false,
				"SERVER_IP":  "192.168.1.100",
			},
			wantErr: true,
			wantLog: false,
		},
		{
			name: "GPU_NODE=true with empty ROCM_BASE_URL (warning)",
			config: map[string]interface{}{
				"FIRST_NODE":     true,
				"GPU_NODE":       true,
				"ROCM_BASE_URL":  "",
			},
			wantErr: false,
			wantLog: true,
		},
		{
			name: "GPU_NODE=true with disabled ROCm step (warning)",
			config: map[string]interface{}{
				"FIRST_NODE":     true,
				"GPU_NODE":       true,
				"DISABLED_STEPS": "SetupAndCheckRocmStep",
			},
			wantErr: false,
			wantLog: true,
		},
		{
			name: "SKIP_DISK_CHECK=true with disk parameters (warning)",
			config: map[string]interface{}{
				"FIRST_NODE":      true,
				"SKIP_DISK_CHECK": true,
				"LONGHORN_DISKS":  "/dev/sdb,/dev/sdc",
			},
			wantErr: false,
			wantLog: true,
		},
		{
			name: "SKIP_DISK_CHECK=false with no disk parameters (warning)",
			config: map[string]interface{}{
				"FIRST_NODE":      true,
				"SKIP_DISK_CHECK": false,
				"LONGHORN_DISKS":  "",
				"SELECTED_DISKS":  "",
			},
			wantErr: false,
			wantLog: true,
		},
		{
			name: "Conflicting step configuration - same step enabled and disabled",
			config: map[string]interface{}{
				"FIRST_NODE":     true,
				"DISABLED_STEPS": "SetupLonghornStep,SetupMetallbStep",
				"ENABLED_STEPS":  "SetupLonghornStep,CheckUbuntuStep",
			},
			wantErr: true,
			wantLog: false,
		},
		{
			name: "Non-conflicting step configuration",
			config: map[string]interface{}{
				"FIRST_NODE":     true,
				"DISABLED_STEPS": "SetupLonghornStep",
				"ENABLED_STEPS":  "CheckUbuntuStep,SetupRKE2Step",
			},
			wantErr: false,
			wantLog: false,
		},
		{
			name: "Essential step disabled (warning)",
			config: map[string]interface{}{
				"FIRST_NODE":     true,
				"DISABLED_STEPS": "CheckUbuntuStep,SetupRKE2Step",
			},
			wantErr: false,
			wantLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			for k, v := range tt.config {
				viper.Set(k, v)
			}

			err := validateConfigurationConflicts()
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfigurationConflicts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateResourceRequirements(t *testing.T) {
	// Note: This test is primarily for structure validation
	// Actual resource checks are system-dependent and may not be testable in all environments
	err := validateResourceRequirements()
	// We expect this to either pass or fail gracefully with specific error messages
	if err != nil {
		t.Logf("validateResourceRequirements() returned error (may be expected on limited systems): %v", err)
	}
}

func TestValidateDiskSpace(t *testing.T) {
	// Test that the function runs without crashing
	err := validateDiskSpace()
	if err != nil {
		t.Logf("validateDiskSpace() returned error (may be expected on limited systems): %v", err)
	}
}

func TestValidateSystemResources(t *testing.T) {
	// Test that the function runs without crashing
	err := validateSystemResources()
	if err != nil {
		t.Logf("validateSystemResources() returned error (may be expected on limited systems): %v", err)
	}
}

func TestValidateUbuntuVersion(t *testing.T) {
	// Test that the function runs without crashing
	err := validateUbuntuVersion()
	if err != nil {
		t.Logf("validateUbuntuVersion() returned error (may be expected on non-Ubuntu systems): %v", err)
	}
}

func TestValidateKernelModules(t *testing.T) {
	// Test that the function runs without crashing (it only logs warnings)
	validateKernelModules()
	// No assertions needed as this function only logs warnings
}

func TestIsModuleLoaded(t *testing.T) {
	// Test with a module that should always exist on Linux systems
	result := isModuleLoaded("kernel") // This might not exist, but function should not crash
	t.Logf("isModuleLoaded('kernel') returned: %v", result)
}

func TestIsModuleAvailable(t *testing.T) {
	// Test with a common module
	result := isModuleAvailable("overlay")
	t.Logf("isModuleAvailable('overlay') returned: %v", result)
}

// Integration tests for complete configuration scenarios
func TestValidationIntegration(t *testing.T) {
	// Save original config
	originalViper := viper.AllSettings()
	defer func() {
		viper.Reset()
		for k, v := range originalViper {
			viper.Set(k, v)
		}
	}()

	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
		desc    string
	}{
		{
			name: "Valid first node configuration",
			config: map[string]interface{}{
				"FIRST_NODE":             true,
				"GPU_NODE":               true,
				"OIDC_URL":              "https://auth.example.com",
				"CLUSTERFORGE_RELEASE":  "https://github.com/example/repo/releases/download/v1.0/release.tar.gz",
				"ROCM_BASE_URL":        "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/",
				"RKE2_INSTALLATION_URL": "https://get.rke2.io",
				"DISABLED_STEPS":        "",
				"ENABLED_STEPS":         "",
			},
			wantErr: false,
			desc:    "Complete valid configuration for first node with GPU",
		},
		{
			name: "Valid additional node configuration",
			config: map[string]interface{}{
				"FIRST_NODE":             false,
				"GPU_NODE":               false,
				"SERVER_IP":              "192.168.1.100",
				"JOIN_TOKEN":             "K10831EXAMPLE::server:aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789",
				"OIDC_URL":              "",
				"CLUSTERFORGE_RELEASE":  "none",
				"ROCM_BASE_URL":        "",
				"RKE2_INSTALLATION_URL": "https://get.rke2.io",
				"DISABLED_STEPS":        "SetupAndCheckRocmStep,SetupClusterForgeStep",
				"ENABLED_STEPS":         "",
			},
			wantErr: false,
			desc:    "Complete valid configuration for additional node without GPU",
		},
		{
			name: "Invalid URL configuration",
			config: map[string]interface{}{
				"FIRST_NODE":             true,
				"GPU_NODE":               true,
				"OIDC_URL":              "ftp://invalid.com",
				"CLUSTERFORGE_RELEASE":  "https://github.com/example/repo/releases/download/v1.0/release.tar.gz",
				"ROCM_BASE_URL":        "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/",
				"RKE2_INSTALLATION_URL": "https://get.rke2.io",
			},
			wantErr: true,
			desc:    "Configuration with invalid URL scheme should fail URL validation",
		},
		{
			name: "Missing required parameters for additional node",
			config: map[string]interface{}{
				"FIRST_NODE": false,
				"GPU_NODE":   false,
			},
			wantErr: true,
			desc:    "Additional node missing SERVER_IP and JOIN_TOKEN should fail conflict validation",
		},
		{
			name: "Invalid step names",
			config: map[string]interface{}{
				"FIRST_NODE":     true,
				"GPU_NODE":       true,
				"DISABLED_STEPS": "InvalidStepName,SetupLonghornStep",
				"ENABLED_STEPS":  "",
			},
			wantErr: true,
			desc:    "Configuration with invalid step names should fail step validation",
		},
		{
			name: "Conflicting step configuration",
			config: map[string]interface{}{
				"FIRST_NODE":     true,
				"GPU_NODE":       true,
				"DISABLED_STEPS": "SetupLonghornStep",
				"ENABLED_STEPS":  "SetupLonghornStep,CheckUbuntuStep",
			},
			wantErr: true,
			desc:    "Configuration with conflicting step settings should fail conflict validation",
		},
		{
			name: "Invalid token format",
			config: map[string]interface{}{
				"FIRST_NODE": false,
				"GPU_NODE":   false,
				"SERVER_IP":  "192.168.1.100",
				"JOIN_TOKEN": "short",
			},
			wantErr: true,
			desc:    "Configuration with invalid token format should fail token validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			for k, v := range tt.config {
				viper.Set(k, v)
			}

			// Test URL validation
			urlErr := validateAllURLs()
			if urlErr != nil && !tt.wantErr {
				t.Errorf("validateAllURLs() failed unexpectedly: %v", urlErr)
				return
			}
			if urlErr != nil && tt.wantErr {
				t.Logf("validateAllURLs() failed as expected: %v", urlErr)
				return // Expected failure
			}

			// Test IP validation
			ipErr := validateAllIPs()
			if ipErr != nil && !tt.wantErr {
				t.Errorf("validateAllIPs() failed unexpectedly: %v", ipErr)
				return
			}
			if ipErr != nil && tt.wantErr {
				t.Logf("validateAllIPs() failed as expected: %v", ipErr)
				return // Expected failure
			}

			// Test token validation
			tokenErr := validateAllTokens()
			if tokenErr != nil && !tt.wantErr {
				t.Errorf("validateAllTokens() failed unexpectedly: %v", tokenErr)
				return
			}
			if tokenErr != nil && tt.wantErr {
				t.Logf("validateAllTokens() failed as expected: %v", tokenErr)
				return // Expected failure
			}

			// Test step name validation
			stepErr := validateAllStepNames()
			if stepErr != nil && !tt.wantErr {
				t.Errorf("validateAllStepNames() failed unexpectedly: %v", stepErr)
				return
			}
			if stepErr != nil && tt.wantErr {
				t.Logf("validateAllStepNames() failed as expected: %v", stepErr)
				return // Expected failure
			}

			// Test configuration conflicts
			conflictErr := validateConfigurationConflicts()
			if conflictErr != nil && !tt.wantErr {
				t.Errorf("validateConfigurationConflicts() failed unexpectedly: %v", conflictErr)
				return
			}
			if conflictErr != nil && tt.wantErr {
				t.Logf("validateConfigurationConflicts() failed as expected: %v", conflictErr)
				return // Expected failure
			}

			// Test resource requirements (may fail on limited systems, so we handle gracefully)
			resourceErr := validateResourceRequirements()
			if resourceErr != nil {
				t.Logf("validateResourceRequirements() returned error (may be system-dependent): %v", resourceErr)
			}

			// If we reach here and expected an error, the test should fail
			if tt.wantErr {
				t.Errorf("Expected validation to fail but all validations passed for: %s", tt.desc)
			}
		})
	}
}