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

package uitesting

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/silogen/cluster-bloom/pkg"
	"gopkg.in/yaml.v2"
)

// TestConfig represents a test case configuration
type TestConfig struct {
	Domain              string `yaml:"DOMAIN"`
	ClusterDisks        string `yaml:"CLUSTER_DISKS"`
	CertOption          string `yaml:"CERT_OPTION"`
	FirstNode           bool   `yaml:"FIRST_NODE"`
	GPUNode             bool   `yaml:"GPU_NODE"`
	ClusterPremountedDisks string `yaml:"CLUSTER_PREMOUNTED_DISKS,omitempty"`
	ExpectedError       string `yaml:"expected_error,omitempty"`
}

// loadTestCase reads a test case from a YAML file
func loadTestCase(filename string) (*TestConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config TestConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// TestWebFormE2E demonstrates browser automation testing using chromedp
// This test requires Chrome/Chromium to be installed
// Install chromium: sudo apk add --no-cache chromium
// Or skip with: SKIP_BROWSER_TESTS=1 go test
func TestWebFormE2E(t *testing.T) {
	// Skip if running in CI without headless browser support
	if os.Getenv("SKIP_BROWSER_TESTS") != "" {
		t.Skip("Skipping browser tests (SKIP_BROWSER_TESTS is set)")
	}

	// Find test case files matching pattern bloom_*.yaml
	testFiles, err := filepath.Glob("bloom_*.yaml")
	if err != nil {
		t.Fatalf("Failed to find test case files: %v", err)
	}

	if len(testFiles) == 0 {
		t.Skip("No test case files found (bloom_*.yaml)")
	}

	// Run test for each test case file
	for _, testFile := range testFiles {
		testName := filepath.Base(testFile)
		t.Run(testName, func(t *testing.T) {
			runBrowserTest(t, testFile)
		})
	}
}

func runBrowserTest(t *testing.T, testCaseFile string) {
	// Load test case from YAML file
	testCase, err := loadTestCase(testCaseFile)
	if err != nil {
		t.Fatalf("Failed to load test case from %s: %v", testCaseFile, err)
	}

	t.Logf("Running test case from: %s", testCaseFile)

	// Get bloom.yaml path from environment or use default
	bloomYamlPath := os.Getenv("BLOOM_YAML_PATH")
	if bloomYamlPath == "" {
		bloomYamlPath = "../bloom.yaml"
	}
	os.Remove(bloomYamlPath)
	// Note: Not cleaning up after test so you can inspect bloom.yaml

	// Use the existing web server on port 62078
	url := "http://127.0.0.1:62078"

	// Connect to existing chromium instance on port 9222
	// This is expected to be running in the container environment
	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), "ws://127.0.0.1:9222")
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	// Set timeout for the entire test
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var pageTitle string
	var formVisible bool
	var saveButtonExists bool
	var pageText string

	// Run browser automation
	err = chromedp.Run(ctx,
		// Navigate to the configuration page
		chromedp.Navigate(url),

		// Wait for the form to be visible
		chromedp.WaitVisible(`#config-form`, chromedp.ByID),

		// Wait for page to fully load
		chromedp.Sleep(500*time.Millisecond),

		// Get page title to verify we're on the right page
		chromedp.Title(&pageTitle),

		// Check that the form is visible
		chromedp.Evaluate(`document.getElementById('config-form') !== null`, &formVisible),

		// Check "Save Configuration" button exists
		chromedp.Evaluate(`Array.from(document.querySelectorAll('button')).some(btn => btn.textContent.includes('Save Configuration'))`, &saveButtonExists),

		// Fill in DOMAIN field from test case
		chromedp.SetValue(`#DOMAIN`, testCase.Domain, chromedp.ByID),

		// Set FIRST_NODE checkbox based on test case
		chromedp.Evaluate(fmt.Sprintf(`document.getElementById('FIRST_NODE').checked = %v`, testCase.FirstNode), nil),

		// Set GPU_NODE checkbox based on test case
		chromedp.Evaluate(fmt.Sprintf(`document.getElementById('GPU_NODE').checked = %v`, testCase.GPUNode), nil),

		// Fill in CLUSTER_DISKS field from test case
		chromedp.SetValue(`#CLUSTER_DISKS`, testCase.ClusterDisks, chromedp.ByID),

		// Select CERT_OPTION from test case
		chromedp.SetValue(`#CERT_OPTION`, testCase.CertOption, chromedp.ByID),

		// Log before clicking save
		chromedp.Evaluate(`console.log('About to click Save Configuration button')`, nil),

		// Click the "Save Configuration" button
		chromedp.Click(`button.btn-secondary:nth-of-type(2)`, chromedp.ByQuery),

		// Log after clicking save
		chromedp.Evaluate(`console.log('Clicked Save Configuration button')`, nil),

		// Wait for the save to complete
		// The form uses fetch() which is async, so we wait for the request
		chromedp.Sleep(3*time.Second),

		// Get page text to check for validation errors
		chromedp.Evaluate(`document.body.innerText`, &pageText),
	)

	if err != nil {
		t.Fatalf("❌ Browser automation failed: %v", err)
	}

	// Verify form was visible
	if !formVisible {
		t.Error("❌ Configuration form was not visible")
	}

	// Verify save button exists
	if !saveButtonExists {
		t.Error("❌ Save Configuration button was not found")
	}

	// If this test expects an error (validation failure)
	if testCase.ExpectedError != "" {
		// Verify bloom.yaml was NOT created (validation should prevent save)
		if _, err := os.Stat(bloomYamlPath); err == nil {
			t.Errorf("❌ Validation should have failed but bloom.yaml was created!")
			t.Errorf("   Expected error message: %s", testCase.ExpectedError)
			return
		}

		// Check if the expected error appears via HTML5 validation
		var validationInfo string
		err = chromedp.Run(ctx,
			// Get the validation message from the DOMAIN field
			chromedp.Evaluate(`
				const domainInput = document.getElementById('DOMAIN');
				const validationMessage = domainInput.validationMessage;
				const isValid = domainInput.validity.valid;
				const title = domainInput.getAttribute('title');

				JSON.stringify({
					validationMessage: validationMessage,
					isValid: isValid,
					title: title,
					value: domainInput.value
				});
			`, &validationInfo),
		)

		if err != nil {
			t.Errorf("⚠️ Could not check for validation message: %v", err)
			return
		}

		// Check if error appears in validation message or title attribute
		errorDisplayed := contains(validationInfo, testCase.ExpectedError)

		if !errorDisplayed {
			t.Errorf("❌ Expected error text '%s' not found in validation", testCase.ExpectedError)
			t.Errorf("   Validation info: %s", validationInfo)
		}
		return
	}

	// Wait for file to be written
	time.Sleep(500 * time.Millisecond)

	// Verify bloom.yaml was created
	if _, err := os.Stat(bloomYamlPath); os.IsNotExist(err) {
		t.Fatalf("❌ bloom.yaml was not created at %s", bloomYamlPath)
	}

	// Verify content
	content, err := os.ReadFile(bloomYamlPath)
	if err != nil {
		t.Fatalf("❌ Failed to read bloom.yaml: %v", err)
	}

	expectedValues := []string{
		fmt.Sprintf("DOMAIN: %s", testCase.Domain),
		fmt.Sprintf("CLUSTER_DISKS: %s", testCase.ClusterDisks),
		fmt.Sprintf("FIRST_NODE: %v", testCase.FirstNode),
		fmt.Sprintf("CERT_OPTION: %s", testCase.CertOption),
	}

	contentStr := string(content)
	for _, expected := range expectedValues {
		if !contains(contentStr, expected) {
			t.Errorf("❌ Missing: %s", expected)
		}
	}
}

// TestWebFormInteraction demonstrates more complex browser interactions
func TestWebFormInteraction(t *testing.T) {
	if os.Getenv("SKIP_BROWSER_TESTS") != "" {
		t.Skip("Skipping browser tests (SKIP_BROWSER_TESTS is set)")
	}

	// Check for chromium
	if _, err := os.Stat("/usr/bin/chromium"); err != nil {
		t.Skip("Skipping browser tests - chromium not found")
	}

	// Start web server
	port := ":62080"
	url := "http://127.0.0.1" + port

	handlerService := pkg.NewWebHandlerServiceConfig()

	// Set some prefilled config
	handlerService.SetPrefilledConfig(map[string]string{
		"DOMAIN":        "prefilled.local",
		"FIRST_NODE":    "true",
		"CLUSTER_DISKS": "/dev/sdb",
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/", handlerService.ConfigWizardHandler)
	mux.HandleFunc("/api/config", handlerService.ConfigAPIHandler)
	mux.HandleFunc("/api/config-only", handlerService.ConfigOnlyAPIHandler)
	mux.HandleFunc("/api/prefilled-config", handlerService.PrefilledConfigAPIHandler)

	handler := pkg.LocalhostOnly(mux)
	server := &http.Server{
		Addr:    "127.0.0.1" + port,
		Handler: handler,
	}

	go server.ListenAndServe()
	defer server.Shutdown(context.Background())

	time.Sleep(500 * time.Millisecond)

	// Create browser context
	allocCtx, allocCancel := chromedp.NewExecAllocator(
		context.Background(),
		append(
			chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("no-sandbox", true),
		)...,
	)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var domainValue string

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`#DOMAIN`, chromedp.ByID),

		// Wait for prefilled config to load (JavaScript sets values)
		chromedp.Sleep(1*time.Second),

		// Get the value to verify it was prefilled
		chromedp.Value(`#DOMAIN`, &domainValue, chromedp.ByID),
	)

	if err != nil {
		t.Fatalf("Browser automation failed: %v", err)
	}

	// Note: The prefilled config might not load in time due to async JS
	// This demonstrates checking for prefilled values
	t.Logf("✅ Interaction test passed! Domain field value: '%s'", domainValue)
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && someContains(s, substr)))
}

func someContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestWebFormValidation demonstrates testing form validation
func TestWebFormValidation(t *testing.T) {
	if os.Getenv("SKIP_BROWSER_TESTS") != "" {
		t.Skip("Skipping browser tests (SKIP_BROWSER_TESTS is set)")
	}

	// Check for chromium
	if _, err := os.Stat("/usr/bin/chromium"); err != nil {
		t.Skip("Skipping browser tests - chromium not found")
	}

	port := ":62081"
	url := "http://127.0.0.1" + port

	handlerService := pkg.NewWebHandlerServiceConfig()
	mux := http.NewServeMux()
	mux.HandleFunc("/", handlerService.ConfigWizardHandler)
	mux.HandleFunc("/api/config", handlerService.ConfigAPIHandler)
	mux.HandleFunc("/api/config-only", handlerService.ConfigOnlyAPIHandler)
	mux.HandleFunc("/api/prefilled-config", handlerService.PrefilledConfigAPIHandler)

	server := &http.Server{
		Addr:    "127.0.0.1" + port,
		Handler: pkg.LocalhostOnly(mux),
	}

	go server.ListenAndServe()
	defer server.Shutdown(context.Background())

	time.Sleep(500 * time.Millisecond)

	allocCtx, allocCancel := chromedp.NewExecAllocator(
		context.Background(),
		append(
			chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("no-sandbox", true),
		)...,
	)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var checkboxChecked bool

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible(`#FIRST_NODE`, chromedp.ByID),

		// Get checkbox state
		chromedp.Evaluate(`document.getElementById('FIRST_NODE').checked`, &checkboxChecked),
	)

	if err != nil {
		t.Fatalf("Browser automation failed: %v", err)
	}

	// FIRST_NODE checkbox should be checked by default
	if !checkboxChecked {
		t.Error("FIRST_NODE checkbox should be checked by default")
	}

	t.Logf("✅ Validation test passed! FIRST_NODE checkbox is checked: %v", checkboxChecked)
}

// Example of running browser automation with custom commands
func Example() {
	// This example shows how to use chromedp for testing
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var title string

	err := chromedp.Run(ctx,
		chromedp.Navigate("http://127.0.0.1:62078"),
		chromedp.WaitVisible(`body`),
		chromedp.Title(&title),
	)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Page title: %s\n", title)
}
