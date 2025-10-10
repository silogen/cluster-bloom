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

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/silogen/cluster-bloom/pkg"
)

var mockCmd = &cobra.Command{
	Use:   "mock",
	Short: "Run mock installation with simulated steps",
	Long: `Run a mock installation that simulates all steps without actually executing them.
This is useful for testing the UI and final output without making system changes.

All steps will return success and generate mock variables that would normally be
created during a real installation (like join tokens, IPs, disk selections, etc.)

Example:
  bloom mock --config mock-config.yaml
  bloom mock  # Interactive mode

Example mock-config.yaml:
  FIRST_NODE: true
  GPU_NODE: true
  DOMAIN: cluster.example.com
  USE_CERT_MANAGER: false
  CERT_OPTION: generate
  METALLB_IP_RANGE: 192.168.1.240-192.168.1.250
`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🧪 Running MOCK installation - no actual system changes will be made")
		fmt.Println()
		runMockSteps()
	},
}

func init() {
	rootCmd.AddCommand(mockCmd)
}

func runMockSteps() {
	// Get version for ConfigMap step
	version := "2.0.0-mock"

	// Build steps array based on FIRST_NODE flag
	var steps []pkg.Step

	if viper.GetBool("FIRST_NODE") {
		// First node installation steps
		steps = []pkg.Step{
			pkg.MockValidateArgsStep,
			pkg.MockValidateSystemRequirementsStep,
			pkg.MockCheckUbuntuStep,
			pkg.MockInstallDependentPackagesStep,
			pkg.MockCreateChronyConfigStep,
			pkg.MockCheckPortsBeforeOpeningStep,
			pkg.MockOpenPortsStep,
			pkg.MockInstallK8SToolsStep,
			pkg.MockInotifyInstancesStep,
			pkg.MockUpdateUdevRulesStep,
			pkg.MockSetupAndCheckRocmStep,
			pkg.MockPrepareRKE2Step,
			pkg.MockHasSufficientRancherPartitionStep,
			pkg.MockNVMEDrivesAvailableStep,
			pkg.MockCleanDisksStep,
			pkg.MockSetupMultipathStep,
			pkg.MockUpdateModprobeStep,
			pkg.MockSelectDrivesStep,
			pkg.MockMountSelectedDrivesStep,
			pkg.MockGenerateNodeLabelsStep,
			pkg.MockSetupRKE2Step,
			pkg.MockSetupKubeConfig,
			pkg.MockSetupMetallbStep,
			pkg.MockCreateMetalLBConfigStep,
			pkg.MockSetupLonghornStep,
			pkg.MockCreateBloomConfigMapStepFunc(version),
			pkg.MockCreateDomainConfigStep,
			pkg.MockSetupClusterForgeStep,
			pkg.MockFinalOutput,
		}
	} else {
		// Additional node installation steps
		steps = []pkg.Step{
			pkg.MockValidateArgsStep,
			pkg.MockValidateSystemRequirementsStep,
			pkg.MockCheckUbuntuStep,
			pkg.MockInstallDependentPackagesStep,
			pkg.MockCreateChronyConfigStep,
			pkg.MockCheckPortsBeforeOpeningStep,
			pkg.MockOpenPortsStep,
			pkg.MockInstallK8SToolsStep,
			pkg.MockInotifyInstancesStep,
			pkg.MockUpdateUdevRulesStep,
			pkg.MockSetupAndCheckRocmStep,
			pkg.MockPrepareRKE2Step,
			pkg.MockHasSufficientRancherPartitionStep,
			pkg.MockNVMEDrivesAvailableStep,
			pkg.MockCleanDisksStep,
			pkg.MockSetupMultipathStep,
			pkg.MockUpdateModprobeStep,
			pkg.MockSelectDrivesStep,
			pkg.MockMountSelectedDrivesStep,
			pkg.MockGenerateNodeLabelsStep,
			pkg.MockSetupRKE2Step,
			pkg.MockCreateBloomConfigMapStepFunc(version),
			pkg.MockFinalOutput,
		}
	}

	// Run with UI
	err := pkg.RunStepsWithUI(steps)
	if err != nil {
		pkg.LogMessage(pkg.Error, fmt.Sprintf("Mock execution encountered error: %v", err))
	}

	fmt.Println()
	fmt.Println("🧪 MOCK Mode Complete!")
	fmt.Println("💡 No actual system changes were made during this mock run")
	fmt.Println()
}
