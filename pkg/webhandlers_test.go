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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestConfigAPIHandler(t *testing.T) {
	// Clean up before test
	os.Remove("bloom.yaml")
	defer os.Remove("bloom.yaml")

	// Create handler service in config mode
	handler := NewWebHandlerServiceConfig()

	// Prepare test configuration
	config := map[string]interface{}{
		"DOMAIN":        "test.local",
		"FIRST_NODE":    true,
		"GPU_NODE":      false,
		"CLUSTER_DISKS": "/dev/sdb",
		"CERT_OPTION":   "generate",
		"CONTROL_PLANE": true,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Create HTTP request
	req := httptest.NewRequest("POST", "/api/config", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Call the handler
	handler.ConfigAPIHandler(w, req)

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Check success field
	success, ok := result["success"].(bool)
	if !ok {
		t.Error("Response missing 'success' field")
	}
	if !success {
		t.Errorf("Config submission failed: %v", result["error"])
	}

	// Verify bloom.yaml was created
	if _, err := os.Stat("bloom.yaml"); os.IsNotExist(err) {
		t.Error("bloom.yaml was not created")
	}

	// Verify bloom.yaml content
	content, err := os.ReadFile("bloom.yaml")
	if err != nil {
		t.Fatalf("Failed to read bloom.yaml: %v", err)
	}

	// Check that some expected values are in the YAML
	contentStr := string(content)
	expectedValues := []string{
		"DOMAIN: test.local",
		"FIRST_NODE: true",
		"GPU_NODE: false",
		"CLUSTER_DISKS: /dev/sdb",
		"CERT_OPTION: generate",
	}

	for _, expected := range expectedValues {
		if !bytes.Contains(content, []byte(expected)) {
			t.Errorf("bloom.yaml does not contain expected value: %s\nActual content:\n%s", expected, contentStr)
		}
	}

	t.Logf("✅ Test passed! bloom.yaml created with correct content")
}

func TestConfigAPIHandlerInvalidJSON(t *testing.T) {
	handler := NewWebHandlerServiceConfig()

	// Create request with invalid JSON
	req := httptest.NewRequest("POST", "/api/config", bytes.NewBufferString("invalid json {"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ConfigAPIHandler(w, req)

	// Parse response
	var result map[string]interface{}
	json.NewDecoder(w.Result().Body).Decode(&result)

	// Should return error
	if result["success"] == true {
		t.Error("Expected failure for invalid JSON, got success")
	}

	if result["error"] == nil {
		t.Error("Expected error message in response")
	}

	t.Logf("✅ Invalid JSON test passed! Error: %v", result["error"])
}

func TestConfigAPIHandlerWrongMethod(t *testing.T) {
	handler := NewWebHandlerServiceConfig()

	// Try GET instead of POST
	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()

	handler.ConfigAPIHandler(w, req)

	// Should return 405 Method Not Allowed
	if w.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Result().StatusCode)
	}

	t.Logf("✅ Wrong method test passed! Got expected 405 status")
}
