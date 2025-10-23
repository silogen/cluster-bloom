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

package dryrun

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v2"
)

var (
	enabled    bool
	mu         sync.RWMutex
	mockValues map[string]MockResult
)

// MockResult represents the return value for a mocked command
type MockResult struct {
	Output string      `yaml:"output"`
	Error  interface{} `yaml:"error"` // Can be string or null
}

// MockConfig represents the structure of the mock values YAML file
type MockConfig struct {
	Mocks map[string]MockResult `yaml:"mocks"`
}

func init() {
	mockValues = make(map[string]MockResult)
}

// SetDryRun sets the dry-run mode
func SetDryRun(dryRun bool) {
	mu.Lock()
	defer mu.Unlock()
	enabled = dryRun
}

// IsDryRun returns true if dry-run mode is enabled
func IsDryRun() bool {
	mu.RLock()
	defer mu.RUnlock()
	return enabled
}

// LoadMockValues loads mock return values from a YAML file
func LoadMockValues(filepath string) error {
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read mock values file: %w", err)
	}

	var config MockConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse mock values YAML: %w", err)
	}

	mockValues = config.Mocks
	return nil
}

// GetMockValue retrieves a mock value for a given command name
// Returns (output, error, found)
func GetMockValue(name string) (string, error, bool) {
	mu.RLock()
	defer mu.RUnlock()

	result, found := mockValues[name]
	if !found {
		return "", nil, false
	}

	// Handle error field which can be string or null
	var err error
	if result.Error != nil {
		if errStr, ok := result.Error.(string); ok && errStr != "" {
			err = fmt.Errorf("%s", errStr)
		}
	}

	return result.Output, err, true
}

// ClearMockValues clears all loaded mock values
func ClearMockValues() {
	mu.Lock()
	defer mu.Unlock()
	mockValues = make(map[string]MockResult)
}
