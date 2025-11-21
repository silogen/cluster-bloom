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

package integration

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/silogen/cluster-bloom/pkg"
	"github.com/silogen/cluster-bloom/pkg/mockablecmd"
	logrus "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// TestStepIntegration tests step execution with various configurations
func TestStepIntegration(t *testing.T) {
	// Silence logs during tests unless LOG_LEVEL is set
	if os.Getenv("LOG_LEVEL") == "" {
		logrus.SetOutput(io.Discard)
		log.SetOutput(io.Discard)
	}

	// Find all test cases in step subdirectories
	testFiles, err := filepath.Glob("step/**/*/bloom.yaml")
	if err != nil {
		t.Fatalf("Failed to find test files: %v", err)
	}

	if len(testFiles) == 0 {
		t.Skip("No integration test cases found")
	}

	for _, testFile := range testFiles {
		testName := filepath.Dir(testFile)
		t.Run(testName, func(t *testing.T) {
			runStepTest(t, testFile)
		})
	}
}

func runStepTest(t *testing.T, testCaseFile string) {
	t.Logf("Running integration test: %s", testCaseFile)

	// Reset state
	mockablecmd.ResetMocks()
	viper.Reset()

	// Read test config
	configData, err := os.ReadFile(testCaseFile)
	if err != nil {
		t.Fatalf("Failed to read test config: %v", err)
	}

	// Parse config
	var rawConfig map[string]interface{}
	if err := yaml.Unmarshal(configData, &rawConfig); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Load into viper
	for k, v := range rawConfig {
		viper.Set(k, v)
	}

	// Load mocks
	mockablecmd.LoadMocks()

	// Get enabled steps
	enabledSteps := viper.GetString("ENABLED_STEPS")
	if enabledSteps == "" {
		t.Fatal("No ENABLED_STEPS specified in test config")
	}

	t.Logf("Testing step: %s", enabledSteps)

	// Execute the step based on name
	var stepErr error
	switch enabledSteps {
	case "PrepareLonghornDisksStep":
		stepErr = runPrepareLonghornDisksStep(t)
	default:
		t.Fatalf("Unknown step: %s", enabledSteps)
	}

	// Check for expected error
	expectedError, hasExpectedError := rawConfig["expected_error"]
	if hasExpectedError {
		expectedErrStr := expectedError.(string)
		if stepErr == nil {
			t.Errorf("Expected error containing '%s' but step succeeded", expectedErrStr)
		} else if !strings.Contains(stepErr.Error(), expectedErrStr) {
			t.Errorf("Expected error containing '%s' but got '%s'", expectedErrStr, stepErr.Error())
		} else {
			t.Logf("✅ Got expected error: %s", stepErr.Error())
		}
	} else {
		if stepErr != nil {
			t.Errorf("Step failed: %v", stepErr)
		} else {
			t.Logf("✅ Step completed successfully")
		}
	}
}

func runPrepareLonghornDisksStep(t *testing.T) error {
	// Get cluster disks
	disksStr := viper.GetString("CLUSTER_DISKS")
	if disksStr == "" {
		t.Log("No CLUSTER_DISKS specified")
		return nil
	}

	// Split and trim disks
	diskParts := strings.Split(disksStr, ",")
	var disks []string
	for _, d := range diskParts {
		d = strings.TrimSpace(d)
		if d != "" {
			disks = append(disks, d)
		}
	}
	t.Logf("Processing %d disk(s): %v", len(disks), disks)

	// Mount drives
	mountedMap, err := pkg.MountDrives(disks)
	if err != nil {
		return err
	}

	t.Logf("Mounted %d disk(s)", len(mountedMap))

	// Persist mounts
	if err := pkg.PersistMountedDisks(mountedMap); err != nil {
		return err
	}

	// Skip GenerateNodeLabels as it requires actual RKE2 config file
	// In integration tests, we're only testing the disk mounting logic
	t.Logf("✅ Disks mounted and persisted successfully")

	return nil
}
