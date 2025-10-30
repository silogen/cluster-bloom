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
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestRun(t *testing.T) {
	t.Run("successful command", func(t *testing.T) {
		output, err := Run("test.echo", "echo", "hello")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if !strings.Contains(string(output), "hello") {
			t.Errorf("Expected output to contain 'hello', got: %s", string(output))
		}
	})

	t.Run("command with error", func(t *testing.T) {
		_, err := Run("test.ls", "ls", "/nonexistent/path/that/does/not/exist")
		if err == nil {
			t.Error("Expected error for nonexistent path, got nil")
		}
	})

	t.Run("command with multiple args", func(t *testing.T) {
		output, err := Run("test.echo.multiple", "echo", "hello", "world")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		result := strings.TrimSpace(string(output))
		if result != "hello world" {
			t.Errorf("Expected 'hello world', got: %s", result)
		}
	})
}

func TestRunWithMocks(t *testing.T) {
	// Setup mocks in viper
	viper.Set("mocks", map[string]interface{}{
		"test.mock.success": map[string]interface{}{
			"output": "mocked output",
		},
		"test.mock.error": map[string]interface{}{
			"output": "error output",
			"error":  "mocked error",
		},
	})

	// Reset the sync.Once to allow reloading
	mocks = make(map[string]MockResponse)
	LoadMocks()

	t.Run("mocked command with success", func(t *testing.T) {
		output, err := Run("test.mock.success", "echo", "should not run")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if string(output) != "mocked output" {
			t.Errorf("Expected 'mocked output', got: %s", string(output))
		}
	})

	t.Run("mocked command with error", func(t *testing.T) {
		output, err := Run("test.mock.error", "echo", "should not run")
		if err == nil {
			t.Error("Expected error, got nil")
		}
		if err.Error() != "mocked error" {
			t.Errorf("Expected error 'mocked error', got: %v", err)
		}
		if string(output) != "error output" {
			t.Errorf("Expected 'error output', got: %s", string(output))
		}
	})

	t.Run("unmocked command runs normally", func(t *testing.T) {
		output, err := Run("test.unmocked", "echo", "real command")
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if !strings.Contains(string(output), "real command") {
			t.Errorf("Expected output to contain 'real command', got: %s", string(output))
		}
	})

	// Cleanup
	viper.Set("mocks", nil)
	mocks = make(map[string]MockResponse)
}
