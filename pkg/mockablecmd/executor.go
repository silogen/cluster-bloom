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
	"sync"

	"github.com/spf13/viper"
)

// MockResponse defines a mock command response
type MockResponse struct {
	Output string
	Error  string
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

					mocks[mockID] = response
				}
			}
		}
	})
}

// Run executes a command and returns output and error
// mockID is a string identifier for mocking purposes (e.g., "PrepareLonghornDisksStep.ListMounts")
func Run(mockID string, name string, args ...string) ([]byte, error) {
	// Check if mock exists for this ID
	if mock, exists := mocks[mockID]; exists {
		if mock.Error != "" {
			return []byte(mock.Output), fmt.Errorf("%s", mock.Error)
		}
		return []byte(mock.Output), nil
	}

	// No mock found, execute the actual command
	cmd := exec.Command(name, args...)
	return cmd.Output()
}
