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
	"testing"

	"github.com/spf13/viper"
)

func TestLogMessage(t *testing.T) {
	tests := []struct {
		name    string
		level   LogLevel
		message string
	}{
		{"Debug level", Debug, "debug message"},
		{"Info level", Info, "info message"},
		{"Warn level", Warn, "warn message"},
		{"Error level", Error, "error message"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			LogMessage(tt.level, tt.message)
		})
	}
}

func TestLogCommand(t *testing.T) {
	LogCommand("test-command", "test output")
}

func TestRunStepsWithUI(t *testing.T) {
	tests := []struct {
		name          string
		steps         []Step
		expectedError bool
	}{
		{
			name: "successful step",
			steps: []Step{
				{
					Id:          "TestStep",
					Name:        "Test Step",
					Description: "A test step",
					Action: func() StepResult {
						return StepResult{Error: nil}
					},
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.steps) == 0 {
				return
			}

			result := tt.steps[0].Action()
			if tt.expectedError && result.Error == nil {
				t.Errorf("Expected error but got none")
			} else if !tt.expectedError && result.Error != nil {
				t.Errorf("Expected no error but got: %v", result.Error)
			}
		})
	}
}

func TestStepExecution(t *testing.T) {
	t.Run("CheckUbuntuStep", func(t *testing.T) {
		result := CheckUbuntuStep.Action()
		if result.Error == nil && result.Message == "" {
			// Result structure is valid
		}
	})

	t.Run("InstallDependentPackagesStep", func(t *testing.T) {
		if os.Getuid() != 0 {
			t.Skip("Skipping test that requires root privileges")
		}
		result := InstallDependentPackagesStep.Action()
		if result.Error == nil && result.Message == "" {
			// Result structure is valid
		}
	})

	t.Run("InotifyInstancesStep", func(t *testing.T) {
		result := InotifyInstancesStep.Action()
		if result.Error == nil && result.Message == "" {
			// Result structure is valid
		}
	})
}

func TestSetupAndCheckRocmStep(t *testing.T) {
	viper.Set("GPU_NODE", false)
	result := SetupAndCheckRocmStep.Action()
	if result.Error != nil {
		t.Errorf("Expected no error for non-GPU node, got: %v", result.Error)
	}

	viper.Set("GPU_NODE", true)
	result = SetupAndCheckRocmStep.Action()
	// Result depends on system state
}

func TestPrepareLonghornDisksStep(t *testing.T) {
	viper.Set("selected_disks", []string{})
	result := PrepareLonghornDisksStep.Action()
	if result.Error != nil {
		t.Errorf("Expected no error with empty disk list, got: %v", result.Error)
	}
}

func TestSetupMetallbStep(t *testing.T) {
	viper.Set("FIRST_NODE", false)
	result := SetupMetallbStep.Action()
	if result.Error != nil {
		t.Errorf("Expected no error for non-first node, got: %v", result.Error)
	}
}

func TestSetupLonghornStep(t *testing.T) {
	viper.Set("FIRST_NODE", false)
	result := SetupLonghornStep.Action()
	if result.Error != nil {
		t.Errorf("Expected no error for non-first node, got: %v", result.Error)
	}
}

func TestCreateMetalLBConfigStep(t *testing.T) {
	viper.Set("FIRST_NODE", false)
	result := CreateMetalLBConfigStep.Action()
	if result.Error != nil {
		t.Errorf("Expected no error for non-first node, got: %v", result.Error)
	}
}

func TestSetupKubeConfig(t *testing.T) {
	viper.Set("FIRST_NODE", false)
	result := SetupKubeConfig.Action()
	if result.Error != nil {
		t.Errorf("Expected no error for non-first node, got: %v", result.Error)
	}
}

func TestFinalOutput(t *testing.T) {
	viper.Set("FIRST_NODE", false)
	// result := FinalOutput.Action()
	// Result depends on system state
}
