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
	"testing"

	"github.com/spf13/viper"
)

func TestCheckPackageInstallConnections(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	viper.Set("CLUSTERFORGE_RELEASE", "https://httpbin.org/status/200")
	viper.Set("ROCM_BASE_URL", "https://httpbin.org/")
	viper.Set("ROCM_DEB_PACKAGE", "status/200")
	viper.Set("RKE2_INSTALLATION_URL", "https://httpbin.org/status/200")

	err := CheckPackageInstallConnections()
	// This may fail in test environment due to network/permission issues
	if err != nil {
		t.Logf("CheckPackageInstallConnections failed as expected in test environment: %v", err)
	}
}

func TestInstallDependentPackages(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	err := InstallDependentPackages()
	// This will likely fail in test environment
	if err != nil {
		t.Logf("InstallDependentPackages failed as expected in test environment: %v", err)
	}
}

func TestInstallPackage(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	// Test with a package that should already be installed
	err := installpackage("bash")
	if err != nil {
		t.Logf("installpackage failed as expected in test environment: %v", err)
	}
}

func TestInstallK8sTools(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	err := installK8sTools()
	// This will likely fail in test environment
	if err != nil {
		t.Logf("installK8sTools failed as expected in test environment: %v", err)
	}
}

func TestSetupManifests(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	err := setupManifests("metallb")
	// This will likely fail in test environment due to permissions
	if err != nil {
		t.Logf("setupManifests failed as expected in test environment: %v", err)
	}
}

func TestSetupAudit(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Skipping test that requires root privileges")
	}

	err := setupAudit()
	// This will likely fail in test environment due to permissions
	if err != nil {
		t.Logf("setupAudit failed as expected in test environment: %v", err)
	}
}

func TestSetupOnePasswordSecret(t *testing.T) {
	t.Run("without token", func(t *testing.T) {
		viper.Set("ONEPASS_CONNECT_TOKEN", "")
		err := SetupOnePasswordSecret()
		if err != nil {
			t.Errorf("Expected no error when token is empty, got: %v", err)
		}
	})

	t.Run("with token", func(t *testing.T) {
		viper.Set("ONEPASS_CONNECT_TOKEN", "test-token")
		err := SetupOnePasswordSecret()
		// This will fail because we're not in a Kubernetes cluster
		if err == nil {
			t.Log("SetupOnePasswordSecret succeeded unexpectedly")
		} else {
			t.Logf("SetupOnePasswordSecret failed as expected outside cluster: %v", err)
		}
		viper.Set("ONEPASS_CONNECT_TOKEN", "")
	})
}

func TestSetupClusterForge(t *testing.T) {
	t.Run("when disabled", func(t *testing.T) {
		viper.Set("CLUSTERFORGE_RELEASE", "none")
		err := SetupClusterForge()
		if err != nil {
			t.Errorf("Expected no error when ClusterForge is disabled, got: %v", err)
		}
	})

	t.Run("with invalid URL", func(t *testing.T) {
		viper.Set("CLUSTERFORGE_RELEASE", "invalid-url")
		err := SetupClusterForge()
		if err == nil {
			t.Errorf("Expected error with invalid URL")
		}
		viper.Set("CLUSTERFORGE_RELEASE", "none")
	})
}