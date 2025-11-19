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
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/silogen/cluster-bloom/pkg"
	"github.com/silogen/cluster-bloom/pkg/mockablecmd"
	logrus "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// TestConfig represents a test case configuration
type TestConfig struct {
	Domain                 string `yaml:"DOMAIN"`
	ClusterDisks           string `yaml:"CLUSTER_DISKS"`
	CertOption             string `yaml:"CERT_OPTION"`
	FirstNode              bool   `yaml:"FIRST_NODE"`
	GPUNode                bool   `yaml:"GPU_NODE"`
	ClusterPremountedDisks string `yaml:"CLUSTER_PREMOUNTED_DISKS,omitempty"`
	ExpectedError          string `yaml:"expected_error,omitempty"`
	ExpectedClusterDisks   string `yaml:"expected_cluster_disks,omitempty"`
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
	if testCase.ExpectedClusterDisks != "" {
		t.Logf("Expected CLUSTER_DISKS: %s", testCase.ExpectedClusterDisks)
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

	// Load config from file
	handlerService.LoadConfigFromFile(configPath, false)
	handlerService.AddRootDeviceToConfig() // This triggers auto-detection

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
	if testCase.ExpectedClusterDisks != "" {
		if actualClusterDisks != testCase.ExpectedClusterDisks {
			t.Errorf("❌ Auto-detected CLUSTER_DISKS mismatch")
			t.Errorf("   Expected: %s", testCase.ExpectedClusterDisks)
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

	// Wait for server to start
	time.Sleep(500 * time.Millisecond)

	// Create browser context
	ctx, cancel := chromedp.NewRemoteAllocator(context.Background(), "ws://127.0.0.1:9222")
	defer cancel()

	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Build dynamic form filling actions based on test case data
	actions := chromedp.Tasks{
		chromedp.Navigate(url),
		chromedp.WaitVisible(`#config-form`, chromedp.ByID),
		chromedp.Sleep(500 * time.Millisecond),
	}

	// Fill in fields from test case
	if testCase.Domain != "" {
		actions = append(actions, chromedp.SetValue(`#DOMAIN`, testCase.Domain, chromedp.ByID))
	}

	// For auto-detect tests, verify pre-filled value instead of setting it
	var actualFormClusterDisks string
	if testCase.ExpectedClusterDisks != "" {
		// This is an auto-detect test - read the pre-filled value
		actions = append(actions, chromedp.Value(`#CLUSTER_DISKS`, &actualFormClusterDisks, chromedp.ByID))
	} else if testCase.ClusterDisks != "" {
		// Normal test - set the value
		actions = append(actions, chromedp.SetValue(`#CLUSTER_DISKS`, testCase.ClusterDisks, chromedp.ByID))
	}

	if testCase.CertOption != "" {
		actions = append(actions, chromedp.SetValue(`#CERT_OPTION`, testCase.CertOption, chromedp.ByID))
	}
	if testCase.ClusterPremountedDisks != "" {
		actions = append(actions, chromedp.SetValue(`#CLUSTER_PREMOUNTED_DISKS`, testCase.ClusterPremountedDisks, chromedp.ByID))
	}

	// Boolean fields (checkboxes)
	actions = append(actions, chromedp.Evaluate(fmt.Sprintf(`document.getElementById('FIRST_NODE').checked = %v`, testCase.FirstNode), nil))
	actions = append(actions, chromedp.Evaluate(fmt.Sprintf(`document.getElementById('GPU_NODE').checked = %v`, testCase.GPUNode), nil))

	// Add save button click
	actions = append(actions,
		chromedp.Click(`button.btn-secondary:nth-of-type(2)`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)

	// Run browser automation
	err = chromedp.Run(ctx, actions)
	if err != nil {
		t.Fatalf("❌ Browser automation failed: %v", err)
	}

	// Verify the pre-filled value appears in browser form
	if testCase.ExpectedClusterDisks != "" {
		if actualFormClusterDisks != testCase.ExpectedClusterDisks {
			t.Errorf("❌ Browser form field mismatch")
			t.Errorf("   Expected: %s", testCase.ExpectedClusterDisks)
			t.Errorf("   Actual:   %s", actualFormClusterDisks)
		} else {
			t.Logf("✅ Browser form field correctly shows: %s", actualFormClusterDisks)
		}
	}
}
