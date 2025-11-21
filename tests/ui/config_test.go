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
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/silogen/cluster-bloom/pkg"
	"github.com/silogen/cluster-bloom/pkg/mockablecmd"
	logrus "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// TestConfig represents a test case configuration (new format)
type TestConfig struct {
	Input  map[string]interface{} `yaml:"input"`
	Mocks  map[string]interface{} `yaml:"mocks,omitempty"`
	Output map[string]interface{} `yaml:"output,omitempty"`
}

// Helper methods to extract values from the new format
func (tc *TestConfig) GetInputString(key string) string {
	if v, ok := tc.Input[key].(string); ok {
		return v
	}
	return ""
}

func (tc *TestConfig) GetInputBool(key string) bool {
	if v, ok := tc.Input[key].(bool); ok {
		return v
	}
	return false
}

func (tc *TestConfig) GetOutputString(key string) string {
	if tc.Output == nil {
		return ""
	}
	if v, ok := tc.Output[key].(string); ok {
		return v
	}
	return ""
}

func (tc *TestConfig) GetExpectedError(field string) string {
	if tc.Output == nil {
		return ""
	}
	if errorMap, ok := tc.Output["error"].(map[interface{}]interface{}); ok {
		if errMsg, ok := errorMap[field].(string); ok {
			return errMsg
		}
	}
	return ""
}

func (tc *TestConfig) HasErrors() bool {
	if tc.Output == nil {
		return false
	}
	_, hasError := tc.Output["error"]
	return hasError
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

// TestConfigBasedTests tests all configuration scenarios
// Each test case runs in its own server instance with mocks loaded
func TestConfigBasedTests(t *testing.T) {
	if os.Getenv("SKIP_BROWSER_TESTS") != "" {
		t.Skip("Skipping browser tests (SKIP_BROWSER_TESTS is set)")
	}

	// Silence logs during tests unless LOG_LEVEL is set
	if os.Getenv("LOG_LEVEL") == "" {
		logrus.SetOutput(io.Discard)
		log.SetOutput(io.Discard)
	}

	// Find all test cases from all directories
	testDir := "testdata/*"
	testFiles, err := filepath.Glob(filepath.Join(testDir, "bloom_*.yaml"))
	if err != nil {
		t.Fatalf("Failed to find test files in %s: %v", testDir, err)
	}

	if len(testFiles) == 0 {
		t.Skip("No test cases found")
	}

	for _, testFile := range testFiles {
		testName := filepath.Base(testFile)
		t.Run(testName, func(t *testing.T) {
			runConfigTest(t, testFile)
		})
	}
}

func runConfigTest(t *testing.T, testCaseFile string) {
	// Load test case
	testCase, err := loadTestCase(testCaseFile)
	if err != nil {
		t.Fatalf("Failed to load test case: %v", err)
	}

	t.Logf("Running test: %s", testCaseFile)
	expectedClusterDisks := testCase.GetOutputString("CLUSTER_DISKS")
	if expectedClusterDisks != "" {
		t.Logf("Expected CLUSTER_DISKS: %s", expectedClusterDisks)
	}

	// Create temporary directory for this test
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "bloom.yaml")

	// Write the test config (with mocks) to bloom.yaml
	configData, err := os.ReadFile(testCaseFile)
	if err != nil {
		t.Fatalf("Failed to read test config: %v", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Reset mocks before loading new ones
	mockablecmd.ResetMocks()

	// Load config into viper first so mocks are available
	var rawConfig map[string]interface{}
	if err := yaml.Unmarshal(configData, &rawConfig); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Set viper values from config (this makes mocks available to mockablecmd)
	for k, v := range rawConfig {
		viper.Set(k, v)
	}

	// Load mocks from viper configuration
	mockablecmd.LoadMocks()

	// Start web server with this config
	port := ":62079" // Use different port to avoid conflicts with main test server
	url := fmt.Sprintf("http://127.0.0.1%s", port)

	handlerService := pkg.NewWebHandlerServiceConfig()

	// Load ONLY mocks from config file, not the config values themselves
	// This prevents pre-filling form fields from test data
	mockablecmd.ResetMocks()
	viper.Reset()
	viper.SetConfigFile(configPath)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}
	mockablecmd.LoadMocks()

	// Don't call LoadConfigFromFile - we want form to start empty
	handlerService.AddRootDeviceToConfig() // This triggers auto-detection with mocks

	// Get prefilled config to verify auto-detection happened
	prefilledConfig := handlerService.GetPrefilledConfig()
	if prefilledConfig == nil {
		t.Fatal("No prefilled config returned")
	}

	actualClusterDisks, ok := prefilledConfig["cluster_disks"]
	if !ok {
		actualClusterDisks = ""
	}

	// Verify auto-detected value (only for autodetect tests)
	if expectedClusterDisks != "" {
		if actualClusterDisks != expectedClusterDisks {
			t.Errorf("❌ Auto-detected CLUSTER_DISKS mismatch")
			t.Errorf("   Expected: %s", expectedClusterDisks)
			t.Errorf("   Actual:   %v", actualClusterDisks)
		} else {
			t.Logf("✅ Auto-detected CLUSTER_DISKS correctly: %v", actualClusterDisks)
		}
	}

	// Start web server for browser testing
	mux := http.NewServeMux()
	mux.HandleFunc("/", handlerService.ConfigWizardHandler)
	mux.HandleFunc("/api/prefilled-config", handlerService.PrefilledConfigAPIHandler)
	mux.HandleFunc("/api/config", handlerService.ConfigAPIHandler)
	mux.HandleFunc("/api/config-only", handlerService.ConfigOnlyAPIHandler)

	server := &http.Server{
		Addr:    "127.0.0.1" + port,
		Handler: pkg.LocalhostOnly(mux),
	}

	go server.ListenAndServe()
	defer server.Close()

	// Create browser context
	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), "http://127.0.0.1:9222")
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Build dynamic form filling actions based on test case data
	actions := chromedp.Tasks{
		chromedp.Navigate(url),
		chromedp.WaitVisible(`#config-form`, chromedp.ByID),
	}

	// Fill in fields from test case input
	if domain := testCase.GetInputString("DOMAIN"); domain != "" {
		actions = append(actions, chromedp.SetValue(`#DOMAIN`, domain, chromedp.ByID))
	}

	// For auto-detect tests, verify pre-filled value instead of setting it
	var actualFormClusterDisks string
	if expectedClusterDisks != "" {
		// This is an auto-detect test - read the pre-filled value
		actions = append(actions, chromedp.Value(`#CLUSTER_DISKS`, &actualFormClusterDisks, chromedp.ByID))
	} else if clusterDisks := testCase.GetInputString("CLUSTER_DISKS"); clusterDisks != "" {
		// Normal test - set the value
		actions = append(actions, chromedp.SetValue(`#CLUSTER_DISKS`, clusterDisks, chromedp.ByID))
	}

	if certOption := testCase.GetInputString("CERT_OPTION"); certOption != "" {
		actions = append(actions, chromedp.SetValue(`#CERT_OPTION`, certOption, chromedp.ByID))
	}
	if clusterPremountedDisks := testCase.GetInputString("CLUSTER_PREMOUNTED_DISKS"); clusterPremountedDisks != "" {
		actions = append(actions, chromedp.SetValue(`#CLUSTER_PREMOUNTED_DISKS`, clusterPremountedDisks, chromedp.ByID))
	}
	if serverIP := testCase.GetInputString("SERVER_IP"); serverIP != "" {
		actions = append(actions, chromedp.SetValue(`#SERVER_IP`, serverIP, chromedp.ByID))
	}
	if joinToken := testCase.GetInputString("JOIN_TOKEN"); joinToken != "" {
		actions = append(actions, chromedp.SetValue(`#JOIN_TOKEN`, joinToken, chromedp.ByID))
	}

	// Boolean fields (checkboxes) - set value and trigger updateConditionals
	firstNode := testCase.GetInputBool("FIRST_NODE")
	gpuNode := testCase.GetInputBool("GPU_NODE")

	var updateResult string
	actions = append(actions, chromedp.Evaluate(fmt.Sprintf(`
		(function() {
			document.getElementById('FIRST_NODE').checked = %v;
			if (typeof updateConditionals === 'function') {
				updateConditionals();
				const serverIP = document.getElementById('SERVER_IP');
				const joinToken = document.getElementById('JOIN_TOKEN');
				return 'updateConditionals called - SERVER_IP required: ' + (serverIP ? serverIP.hasAttribute('required') : 'null') +
					', JOIN_TOKEN required: ' + (joinToken ? joinToken.hasAttribute('required') : 'null');
			} else {
				return 'updateConditionals not found';
			}
		})()
	`, firstNode), &updateResult))
	if testCase.HasErrors() {
		t.Logf("After updateConditionals: %s", updateResult)
	}
	actions = append(actions, chromedp.Evaluate(fmt.Sprintf(`document.getElementById('GPU_NODE').checked = %v`, gpuNode), nil))

	// If this is an expected error test, click submit and check for validation errors
	if testCase.HasErrors() {
		// Force update by unchecking then rechecking FIRST_NODE to trigger updateConditionals
		actions = append(actions,
			chromedp.Evaluate(fmt.Sprintf(`
				document.getElementById('FIRST_NODE').checked = true;
				if (typeof updateConditionals === 'function') updateConditionals();
				document.getElementById('FIRST_NODE').checked = %v;
				if (typeof updateConditionals === 'function') updateConditionals();
			`, firstNode), nil),
			chromedp.Sleep(200*time.Millisecond), // Wait for DOM update
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond), // Wait for validation
		)

		// Collect validation messages for all fields that might have errors
		var pageHTML string
		var formSubmitted bool
		validationVars := make(map[string]*string)
		fields := []string{"DOMAIN", "SERVER_IP", "JOIN_TOKEN", "TLS_CERT", "TLS_KEY"}

		for _, field := range fields {
			varPtr := new(string)
			validationVars[field] = varPtr
			actions = append(actions, chromedp.Evaluate(fmt.Sprintf(
				`document.getElementById('%s') ? document.getElementById('%s').validationMessage : ''`,
				field, field), varPtr))
		}

		actions = append(actions,
			chromedp.InnerHTML(`body`, &pageHTML, chromedp.ByQuery),
			// Check if form was submitted (result div would be visible)
			chromedp.Evaluate(`
				const resultDiv = document.getElementById('result');
				resultDiv ? (resultDiv.style.display !== 'none') : false;
			`, &formSubmitted),
		)

		// Run browser automation
		err = chromedp.Run(ctx, actions)
		if err != nil {
			t.Fatalf("❌ Browser automation failed: %v", err)
		}

		// Check each expected field error
		allErrorsFound := true
		for _, field := range fields {
			expectedError := testCase.GetExpectedError(field)
			if expectedError != "" {
				actualValidation := *validationVars[field]
				// Check both validationMessage and page HTML (for modal errors)
				if strings.Contains(actualValidation, expectedError) || strings.Contains(pageHTML, expectedError) {
					t.Logf("✅ %s validation error found: contains '%s'", field, expectedError)
				} else {
					// Error message must match - no leniency
					t.Errorf("❌ Expected %s error not found", field)
					t.Errorf("   Expected error containing: %s", expectedError)
					t.Errorf("   Actual validationMessage: %s", actualValidation)
					t.Errorf("   Form submitted: %v", formSubmitted)
					allErrorsFound = false
				}
			}
		}

		if allErrorsFound && !formSubmitted {
			t.Logf("✅ All expected validation errors found, form not submitted")
		}
	} else {
		// For non-error tests, check that form is valid before submitting
		var domainValidation string
		var serverIPValidation string
		var joinTokenValidation string
		var tlsCertValidation string
		var tlsKeyValidation string

		// Add save button click for non-error tests
		actions = append(actions,
			chromedp.Click(`button.btn-secondary:nth-of-type(2)`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond), // Wait for any validation/submission
			// Check validation messages on key fields
			chromedp.Evaluate(`document.getElementById('DOMAIN') ? document.getElementById('DOMAIN').validationMessage : ''`, &domainValidation),
			chromedp.Evaluate(`document.getElementById('SERVER_IP') ? document.getElementById('SERVER_IP').validationMessage : ''`, &serverIPValidation),
			chromedp.Evaluate(`document.getElementById('JOIN_TOKEN') ? document.getElementById('JOIN_TOKEN').validationMessage : ''`, &joinTokenValidation),
			chromedp.Evaluate(`document.getElementById('TLS_CERT') ? document.getElementById('TLS_CERT').validationMessage : ''`, &tlsCertValidation),
			chromedp.Evaluate(`document.getElementById('TLS_KEY') ? document.getElementById('TLS_KEY').validationMessage : ''`, &tlsKeyValidation),
		)

		// Run browser automation
		err = chromedp.Run(ctx, actions)
		if err != nil {
			t.Fatalf("❌ Browser automation failed: %v", err)
		}

		// Check for unexpected validation errors
		if domainValidation != "" {
			t.Errorf("❌ Unexpected validation error on DOMAIN: %s", domainValidation)
		}
		if serverIPValidation != "" {
			t.Errorf("❌ Unexpected validation error on SERVER_IP: %s", serverIPValidation)
		}
		if joinTokenValidation != "" {
			t.Errorf("❌ Unexpected validation error on JOIN_TOKEN: %s", joinTokenValidation)
		}
		if tlsCertValidation != "" {
			t.Errorf("❌ Unexpected validation error on TLS_CERT: %s", tlsCertValidation)
		}
		if tlsKeyValidation != "" {
			t.Errorf("❌ Unexpected validation error on TLS_KEY: %s", tlsKeyValidation)
		}
	}

	// Verify the pre-filled value appears in browser form
	if expectedClusterDisks != "" {
		if actualFormClusterDisks != expectedClusterDisks {
			t.Errorf("❌ Browser form field mismatch")
			t.Errorf("   Expected: %s", expectedClusterDisks)
			t.Errorf("   Actual:   %s", actualFormClusterDisks)
		} else {
			t.Logf("✅ Browser form field correctly shows: %s", actualFormClusterDisks)
		}
	}
}
