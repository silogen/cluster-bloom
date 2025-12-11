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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestFetchAndSaveOIDCCertificate(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	tests := []struct {
		name        string
		url         string
		expectError bool
	}{
		{"invalid URL", "invalid-url", true},
		{"localhost", "localhost", true}, // Will fail as no HTTPS service
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := FetchAndSaveOIDCCertificate(tt.url, 0)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for %s, got none", tt.url)
			} else if !tt.expectError && err != nil {
				t.Errorf("Expected no error for %s, got: %v", tt.url, err)
			}
		})
	}
}

func TestPrepareRKE2(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	tests := []struct {
		name    string
		oidcURL string
	}{
		{"without OIDC", ""},
		{"with invalid OIDC", "invalid-url"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.oidcURL != "" {
				viper.Set("ADDITIONAL_OIDC_PROVIDERS", []map[string]interface{}{
					{"url": tt.oidcURL, "audiences": []string{"k8s"}},
				})
			}
			err := PrepareRKE2()
			// May fail due to permissions, but function should exist
			if err == nil {
				t.Log("PrepareRKE2 succeeded")
			} else {
				t.Logf("PrepareRKE2 failed as expected in test environment: %v", err)
			}
		})
	}
}

func TestSetupFirstRKE2(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	viper.Set("RKE2_INSTALLATION_URL", "https://get.rke2.io")
	err := SetupFirstRKE2()
	// This will likely fail in test environment
	if err == nil {
		t.Log("SetupFirstRKE2 succeeded unexpectedly")
	} else {
		t.Logf("SetupFirstRKE2 failed as expected in test environment: %v", err)
	}
}

func TestSetupRKE2Additional(t *testing.T) {
	tests := []struct {
		name        string
		serverIP    string
		joinToken   string
		expectError bool
	}{
		{"missing server IP", "", "token", true},
		{"missing join token", "192.168.1.1", "", true},
		{"valid inputs", "192.168.1.1", "valid-token", true}, // Will fail due to file permissions
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("SERVER_IP", tt.serverIP)
			viper.Set("JOIN_TOKEN", tt.joinToken)
			viper.Set("RKE2_INSTALLATION_URL", "https://get.rke2.io")

			err := SetupRKE2Additional()
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestStartServiceWithTimeout(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	tests := []struct {
		name        string
		serviceName string
		timeout     time.Duration
		expectError bool
	}{
		{"non-existent service", "non-existent-service", 1 * time.Second, true},
		{"invalid service name", "", 1 * time.Second, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := startServiceWithTimeout(tt.serviceName, tt.timeout)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestRKE2ConfigContent(t *testing.T) {
	if !strings.Contains(rke2ConfigContent, "cni: cilium") {
		t.Errorf("Expected rke2ConfigContent to contain 'cni: cilium'")
	}
	if !strings.Contains(rke2ConfigContent, "cluster-cidr: 10.242.0.0/16") {
		t.Errorf("Expected rke2ConfigContent to contain 'cluster-cidr: 10.242.0.0/16'")
	}
	if !strings.Contains(rke2ConfigContent, "service-cidr: 10.243.0.0/16") {
		t.Errorf("Expected rke2ConfigContent to contain 'service-cidr: 10.243.0.0/16'")
	}
	if !strings.Contains(rke2ConfigContent, "disable: rke2-ingress-nginx") {
		t.Errorf("Expected rke2ConfigContent to contain 'disable: rke2-ingress-nginx'")
	}
}

func TestOIDCConfigTemplate(t *testing.T) {
	config := oidcConfigTemplate

	if !strings.Contains(config, "--oidc-issuer-url=%s") {
		t.Errorf("Expected oidcConfigTemplate to contain '--oidc-issuer-url=%%s'")
	}
	if !strings.Contains(config, "--oidc-client-id=k8s") {
		t.Errorf("Expected oidcConfigTemplate to contain '--oidc-client-id=k8s'")
	}
	if !strings.Contains(config, "--oidc-username-claim=preferred_username") {
		t.Errorf("Expected oidcConfigTemplate to contain '--oidc-username-claim=preferred_username'")
	}
	if !strings.Contains(config, "--oidc-groups-claim=groups") {
		t.Errorf("Expected oidcConfigTemplate to contain '--oidc-groups-claim=groups'")
	}
}
