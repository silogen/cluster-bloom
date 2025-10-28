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

package mockablecmd

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// MockResponse defines a mock command response
type MockResponse struct {
	Output string
	Error  string
	Args   []interface{} // Expected arguments for validation
}

var (
	mocks     map[string]MockResponse
	mocksOnce sync.Once
)

// LoadMocks loads mock configurations from viper
func LoadMocks() {
	mocksOnce.Do(func() {
		mocks = make(map[string]MockResponse)

		if !viper.IsSet("mocks") {
			return
		}

		mocksData := viper.Get("mocks")
		if mocksMap, ok := mocksData.(map[string]interface{}); ok {
			for mockID, mockData := range mocksMap {
				if mockMap, ok := mockData.(map[string]interface{}); ok {
					response := MockResponse{}

					if o, ok := mockMap["output"]; ok {
						if s, ok := o.(string); ok {
							response.Output = s
						}
					}

					if e, ok := mockMap["error"]; ok {
						if s, ok := e.(string); ok {
							response.Error = s
						}
					}

					if a, ok := mockMap["args"]; ok {
						if argsList, ok := a.([]interface{}); ok {
							response.Args = argsList
						}
					}

					mocks[mockID] = response
				}
			}
		}
	})
}

// Run executes a command and returns output and error
// mockID is a string identifier for mocking purposes (e.g., "PrepareLonghornDisksStep.ListMounts")
func Run(mockID string, name string, args ...string) ([]byte, error) {
	// Log all command executions
	cmdString := name
	if len(args) > 0 {
		cmdString = fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	}
	log.Infof("mockablecmd.Run: mockID=%q, cmd=%q", mockID, cmdString)

	// Normalize mockID to lowercase for case-insensitive lookup (viper lowercases keys)
	mockIDLower := strings.ToLower(mockID)

	// Check if mock exists for this ID
	if mock, exists := mocks[mockIDLower]; exists {
		log.Infof("mockablecmd.Run: using mock for %q (normalized to %q)", mockID, mockIDLower)

		// Validate args if specified in mock
		if len(mock.Args) > 0 {
			if err := validateArgs(mock.Args, name, args); err != nil {
				log.Errorf("mockablecmd.Run: mock arg validation failed for %s: %v", mockID, err)
				return nil, fmt.Errorf("mock arg validation failed for %s: %w", mockID, err)
			}
		}

		if mock.Error != "" {
			log.Infof("mockablecmd.Run: returning mocked error for %q: %s", mockID, mock.Error)
			return []byte(mock.Output), fmt.Errorf("%s", mock.Error)
		}
		log.Infof("mockablecmd.Run: returning mocked output for %q", mockID)
		return []byte(mock.Output), nil
	}

	// No mock found, execute the actual command
	log.Infof("mockablecmd.Run: no mock found for %q, executing real command: %s", mockID, cmdString)
	cmd := exec.Command(name, args...)
	return cmd.Output()
}

// validateArgs validates that the actual command and args match the expected ones
func validateArgs(expectedArgs []interface{}, actualName string, actualArgs []string) error {
	// First element should be the command name
	if len(expectedArgs) == 0 {
		return nil
	}

	expectedName, ok := expectedArgs[0].(string)
	if !ok {
		return fmt.Errorf("expected command name as string, got %T", expectedArgs[0])
	}

	if expectedName != actualName {
		return fmt.Errorf("expected command %q, got %q", expectedName, actualName)
	}

	// Validate remaining args
	expectedArgsList := expectedArgs[1:]
	if len(expectedArgsList) != len(actualArgs) {
		return fmt.Errorf("expected %d args, got %d", len(expectedArgsList), len(actualArgs))
	}

	for i, expected := range expectedArgsList {
		expectedStr, ok := expected.(string)
		if !ok {
			return fmt.Errorf("expected arg[%d] as string, got %T", i, expected)
		}
		if expectedStr != actualArgs[i] {
			return fmt.Errorf("arg[%d]: expected %q, got %q", i, expectedStr, actualArgs[i])
		}
	}

	return nil
}
