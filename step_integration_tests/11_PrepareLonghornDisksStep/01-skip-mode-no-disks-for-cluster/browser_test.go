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

package test01

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"gopkg.in/yaml.v3"
)

// TestSkipModeViaBrowser tests PrepareLonghornDisksStep skip mode by submitting
// configuration through the web browser interface.
// This test assumes cluster-bloom is already running on port 62078 (default).
// Run cluster-bloom first: cd /workspace && ./cluster-bloom
// This test requires remote Chrome on port 9222.
func TestSkipModeViaBrowser(t *testing.T) {
	// Check if remote Chrome is available on port 9222
	_, err := http.Get("http://localhost:9222/json/version")
	if err != nil {
		t.Skip("Skipping test - remote Chrome not available on port 9222. Start it with: docker run -d --name chromium --network container:<name> -e PORT=9222 browserless/chrome:latest")
	}

	// Try to find cluster-bloom on one of its default ports
	var url string
	var resp *http.Response

	for _, port := range []int{62078, 62079, 62080} {
		testURL := fmt.Sprintf("http://127.0.0.1:%d", port)
		resp, err = http.Get(testURL)
		if err == nil {
			resp.Body.Close()
			url = testURL
			break
		}
	}

	if url == "" {
		t.Skip("Skipping test - cluster-bloom is not running on ports 62078-62080. Start it with: ./cluster-bloom")
	}

	t.Logf("Found cluster-bloom running on %s", url)

	currentDir, _ := os.Getwd()

	// Load test case configuration from bloom_testcase.yaml
	testCaseFile := filepath.Join(currentDir, "bloom_testcase.yaml")
	testCaseData, err := os.ReadFile(testCaseFile)
	if err != nil {
		t.Fatalf("Failed to read bloom_testcase.yaml: %v", err)
	}

	var testConfig map[string]interface{}
	if err := yaml.Unmarshal(testCaseData, &testConfig); err != nil {
		t.Fatalf("Failed to parse bloom_testcase.yaml: %v", err)
	}

	t.Logf("Loaded test configuration from bloom_testcase.yaml: %+v", testConfig)

	// Connect to remote Chrome on port 9222
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), "ws://localhost:9222")
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var currentURL string
	var successMessageVisible bool

	// Build chromedp actions dynamically from test config
	var actions []chromedp.Action

	// Navigate to configuration page
	actions = append(actions,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`#config-form`, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),
	)

	// Fill form fields from test config
	for key, value := range testConfig {
		// Skip non-form fields like "mocks"
		if key == "mocks" {
			continue
		}

		fieldID := key

		switch v := value.(type) {
		case bool:
			// For boolean fields, use JavaScript to set checked property
			checkJS := fmt.Sprintf(`document.getElementById('%s').checked = %t`, fieldID, v)
			actions = append(actions,
				chromedp.Evaluate(checkJS, nil),
			)
		case string:
			// For string values, set the input value
			actions = append(actions,
				chromedp.SetValue("#"+fieldID, v, chromedp.ByID),
			)
		}
	}

	// Add submit and verification actions
	actions = append(actions,
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Evaluate(`document.getElementById('result') && document.getElementById('result').innerText.includes('Configuration Saved Successfully')`, &successMessageVisible),
		chromedp.Sleep(3500*time.Millisecond),
		chromedp.Location(&currentURL),
	)

	// Run browser automation
	browserErr := chromedp.Run(ctx, actions...)

	if browserErr != nil {
		t.Fatalf("Browser automation failed: %v", browserErr)
	}

	// Verify success message was shown
	if !successMessageVisible {
		t.Error("Success message was not displayed after form submission")
	} else {
		t.Logf("✅ Success message displayed: 'Configuration Saved Successfully'")
	}

	// Verify redirect to monitoring page
	if currentURL != url+"/monitor" {
		t.Errorf("Expected redirect to %s/monitor but got: %s", url, currentURL)
	} else {
		t.Logf("✅ Successfully redirected to monitoring page: %s", currentURL)
	}

	// Verify bloom.yaml was created (wait a bit for file write)
	time.Sleep(1 * time.Second)
	yamlPath := filepath.Join(currentDir, "bloom.yaml")
	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		t.Fatalf("bloom.yaml was not created by browser submission at %s", yamlPath)
	}

	// Verify configuration contains expected values from test case
	content, err := os.ReadFile("bloom.yaml")
	if err != nil {
		t.Fatalf("Failed to read bloom.yaml: %v", err)
	}

	contentStr := string(content)

	// Check each value from bloom_testcase.yaml
	for key, value := range testConfig {
		// Skip non-form fields like "mocks"
		if key == "mocks" {
			continue
		}

		var expectedStr string
		switch v := value.(type) {
		case bool:
			expectedStr = fmt.Sprintf("%s: %t", key, v)
		case string:
			expectedStr = fmt.Sprintf("%s: %s", key, v)
		default:
			expectedStr = fmt.Sprintf("%s: %v", key, v)
		}

		if !contains(contentStr, expectedStr) {
			t.Errorf("bloom.yaml missing expected value: %s\nActual content:\n%s", expectedStr, contentStr)
		}
	}

	t.Logf("✅ Configuration submitted successfully via browser!")
	t.Logf("Generated configuration:\n%s", contentStr)

	// Note: Since cluster-bloom is running externally, we can't directly verify
	// installation execution from this test. The external cluster-bloom process will:
	// 1. Receive the configuration via the form submission
	// 2. Start the installation automatically
	// 3. Create bloom.log with installation progress
	// 4. Execute the PrepareLonghornDisksStep which will skip due to NO_DISKS_FOR_CLUSTER=true
	//
	// To verify installation completion, check the external cluster-bloom output
	// or inspect bloom.log after the test completes.

	t.Logf("Complete browser interaction verified:")
	t.Logf("  1. Form filled via browser ✓")
	t.Logf("  2. bloom.yaml created ✓")
	t.Logf("  3. Success message displayed ✓")
	t.Logf("  4. Redirected to /monitor ✓")
	t.Logf("")
	t.Logf("💡 Check the cluster-bloom console output to verify installation execution")
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
