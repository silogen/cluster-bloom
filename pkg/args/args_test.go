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

package args

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestSetAndGetAllSteps(t *testing.T) {
	testSteps := []string{"Step1", "Step2", "Step3"}

	SetAllSteps(testSteps)

	result := GetAllStepIDs()
	if len(result) != len(testSteps) {
		t.Errorf("Expected %d steps, got %d", len(testSteps), len(result))
	}

	for i, step := range testSteps {
		if result[i] != step {
			t.Errorf("Expected step %d to be %s, got %s", i, step, result[i])
		}
	}
}

func TestIsArgUsed(t *testing.T) {
	// Save original config
	originalViper := viper.AllSettings()
	defer func() {
		viper.Reset()
		for k, v := range originalViper {
			viper.Set(k, v)
		}
	}()

	tests := []struct {
		name       string
		arg        Arg
		viperSetup map[string]interface{}
		wantUsed   bool
	}{
		{
			name: "No dependencies - always used",
			arg: Arg{
				Key:          "FIRST_NODE",
				Dependencies: []UsedWhen{},
			},
			viperSetup: map[string]interface{}{},
			wantUsed:   true,
		},
		{
			name: "Dependency satisfied - equals_true",
			arg: Arg{
				Key:          "CONTROL_PLANE",
				Dependencies: []UsedWhen{{"FIRST_NODE", "equals_false"}},
			},
			viperSetup: map[string]interface{}{
				"FIRST_NODE": false,
			},
			wantUsed: true,
		},
		{
			name: "Dependency not satisfied - equals_true",
			arg: Arg{
				Key:          "CONTROL_PLANE",
				Dependencies: []UsedWhen{{"FIRST_NODE", "equals_false"}},
			},
			viperSetup: map[string]interface{}{
				"FIRST_NODE": true,
			},
			wantUsed: false,
		},
		{
			name: "Dependency satisfied - equals_specific_value",
			arg: Arg{
				Key:          "TLS_CERT",
				Dependencies: []UsedWhen{{"CERT_OPTION", "equals_existing"}},
			},
			viperSetup: map[string]interface{}{
				"CERT_OPTION": "existing",
			},
			wantUsed: true,
		},
		{
			name: "Multiple dependencies - all satisfied",
			arg: Arg{
				Key: "TLS_CERT",
				Dependencies: []UsedWhen{
					{"USE_CERT_MANAGER", "equals_false"},
					{"CERT_OPTION", "equals_existing"},
				},
			},
			viperSetup: map[string]interface{}{
				"USE_CERT_MANAGER": false,
				"CERT_OPTION":      "existing",
			},
			wantUsed: true,
		},
		{
			name: "Multiple dependencies - one not satisfied",
			arg: Arg{
				Key: "TLS_CERT",
				Dependencies: []UsedWhen{
					{"USE_CERT_MANAGER", "equals_false"},
					{"CERT_OPTION", "equals_existing"},
				},
			},
			viperSetup: map[string]interface{}{
				"USE_CERT_MANAGER": true,
				"CERT_OPTION":      "existing",
			},
			wantUsed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			for k, v := range tt.viperSetup {
				viper.Set(k, v)
			}

			result := IsArgUsed(tt.arg)
			if result != tt.wantUsed {
				t.Errorf("IsArgUsed() = %v, want %v", result, tt.wantUsed)
			}
		})
	}
}

func TestValidateJoinTokenArg(t *testing.T) {
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
			err := validateJoinTokenArg(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateJoinTokenArg() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateStepNamesArg(t *testing.T) {
	// Setup step IDs for testing
	SetAllSteps([]string{"SetupLonghornStep", "SetupMetallbStep", "CheckUbuntuStep", "SetupRKE2Step"})

	tests := []struct {
		name      string
		stepNames string
		wantErr   bool
	}{
		{"Valid single step", "SetupLonghornStep", false},
		{"Valid multiple steps", "SetupLonghornStep,SetupMetallbStep", false},
		{"Valid steps with spaces", "SetupLonghornStep, SetupMetallbStep", false},
		{"Empty string (allowed)", "", false},
		{"Invalid step name", "InvalidStep", true},
		{"Valid and invalid mixed", "SetupLonghornStep,InvalidStep", true},
		{"Invalid step with typo", "SetupLonghornStepTypo", true},
		{"Empty entries", "SetupLonghornStep,,SetupMetallbStep", false},
		{"Trailing comma", "SetupLonghornStep,", false},
		{"Leading comma", ",SetupLonghornStep", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStepNamesArg(tt.stepNames)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateStepNamesArg() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDisabledStepsWarnings(t *testing.T) {
	// Save original config
	originalViper := viper.AllSettings()
	defer func() {
		viper.Reset()
		for k, v := range originalViper {
			viper.Set(k, v)
		}
	}()

	tests := []struct {
		name      string
		stepNames string
		gpuNode   bool
		wantErr   bool
	}{
		{"Empty steps", "", false, false},
		{"Essential step CheckUbuntuStep", "CheckUbuntuStep", false, false},
		{"Essential step SetupRKE2Step", "SetupRKE2Step", false, false},
		{"ROCm step with GPU_NODE=true", "SetupAndCheckRocmStep", true, false},
		{"ROCm step with GPU_NODE=false", "SetupAndCheckRocmStep", false, false},
		{"Non-essential step", "SetupLonghornStep", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("GPU_NODE", tt.gpuNode)

			err := validateDisabledStepsWarnings(tt.stepNames)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDisabledStepsWarnings() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDisabledStepsConflict(t *testing.T) {
	// Save original config
	originalViper := viper.AllSettings()
	defer func() {
		viper.Reset()
		for k, v := range originalViper {
			viper.Set(k, v)
		}
	}()

	tests := []struct {
		name          string
		disabledSteps string
		enabledSteps  string
		wantErr       bool
	}{
		{"No enabled steps", "SetupLonghornStep", "", false},
		{"No disabled steps", "", "SetupLonghornStep", false},
		{"Both empty", "", "", false},
		{"Both set - conflict", "SetupLonghornStep", "CheckUbuntuStep", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("ENABLED_STEPS", tt.enabledSteps)

			err := validateDisabledStepsConflict(tt.disabledSteps)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDisabledStepsConflict() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSkipDiskCheckConsistency(t *testing.T) {
	// Save original config
	originalViper := viper.AllSettings()
	defer func() {
		viper.Reset()
		for k, v := range originalViper {
			viper.Set(k, v)
		}
	}()

	tests := []struct {
		name          string
		skipDiskCheck bool
		longhornDisks string
		selectedDisks string
		wantErr       bool
	}{
		{"SKIP=true, no disks", true, "", "", false},
		{"SKIP=false, disks set", false, "/dev/sdb", "", false},
		{"SKIP=true, disks set (warning)", true, "/dev/sdb", "", false},
		{"SKIP=false, no disks (warning)", false, "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("SKIP_DISK_CHECK", tt.skipDiskCheck)
			viper.Set("LONGHORN_DISKS", tt.longhornDisks)
			viper.Set("SELECTED_DISKS", tt.selectedDisks)

			err := validateSkipDiskCheckConsistency("")
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSkipDiskCheckConsistency() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateArgs(t *testing.T) {
	// Setup step IDs for testing
	SetAllSteps([]string{"SetupLonghornStep", "SetupMetallbStep", "CheckUbuntuStep"})

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
			name: "Valid first node config",
			config: map[string]interface{}{
				"FIRST_NODE":            true,
				"GPU_NODE":              true,
				"DOMAIN":                "cluster.example.com",
				"OIDC_URL":              "https://auth.example.com",
				"ROCM_BASE_URL":         "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/",
				"RKE2_INSTALLATION_URL": "https://get.rke2.io",
				"CLUSTERFORGE_RELEASE":  "https://github.com/example/repo/releases/v1.0/release.tar.gz",
			},
			wantErr: false,
		},
		{
			name: "Valid additional node config",
			config: map[string]interface{}{
				"FIRST_NODE": false,
				"GPU_NODE":   false,
				"SERVER_IP":  "192.168.1.100",
				"JOIN_TOKEN": "K10831EXAMPLE::server:aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789",
			},
			wantErr: false,
		},
		{
			name: "Missing required DOMAIN for first node",
			config: map[string]interface{}{
				"FIRST_NODE": true,
				"GPU_NODE":   true,
			},
			wantErr: true,
		},
		{
			name: "Missing required SERVER_IP for additional node",
			config: map[string]interface{}{
				"FIRST_NODE": false,
				"JOIN_TOKEN": "K10831EXAMPLE::server:aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789",
			},
			wantErr: true,
		},
		{
			name: "Invalid URL",
			config: map[string]interface{}{
				"FIRST_NODE": true,
				"DOMAIN":     "cluster.example.com",
				"OIDC_URL":   "ftp://invalid.com",
			},
			wantErr: true,
		},
		{
			name: "Invalid IP address",
			config: map[string]interface{}{
				"FIRST_NODE": false,
				"SERVER_IP":  "invalid-ip",
				"JOIN_TOKEN": "K10831EXAMPLE::server:aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789",
			},
			wantErr: true,
		},
		{
			name: "Invalid step name",
			config: map[string]interface{}{
				"FIRST_NODE":     true,
				"DOMAIN":         "cluster.example.com",
				"DISABLED_STEPS": "InvalidStepName",
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

			err := ValidateArgs()
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateArgsHelp(t *testing.T) {
	help := GenerateArgsHelp()

	if help == "" {
		t.Error("GenerateArgsHelp() returned empty string")
	}

	// Check that it contains some expected argument names
	expectedArgs := []string{"FIRST_NODE", "GPU_NODE", "DOMAIN", "SERVER_IP"}
	for _, arg := range expectedArgs {
		if !strings.Contains(help, arg) {
			t.Errorf("GenerateArgsHelp() does not contain %s", arg)
		}
	}
}
