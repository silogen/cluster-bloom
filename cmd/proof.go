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
	"github.com/silogen/cluster-bloom/pkg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var proofCmd = &cobra.Command{
	Use:   "proof",
	Short: "Test a node to check readiness for cluster use",
	Long:  `Runs through steps to validate all settings and prerequisite SW are setup correctly`,
	Run: func(cmd *cobra.Command, args []string) {
		pkg.LogMessage(pkg.Info, "Config value for 'proof': "+viper.GetString("proof"))
		pkg.LogMessage(pkg.Debug, "Starting node proofing")
		proofSteps()
	},
}

func init() {
	rootCmd.AddCommand(proofCmd)
}

func proofSteps() {
	steps := []pkg.Step{
		{
			Name:        "Check Ubuntu Version",
			Description: "Verify running on supported Ubuntu version",
			Action: func() pkg.StepResult {
				pkg.LogMessage(pkg.Info, "Checking supported Ubuntu version")
				if !pkg.IsRunningOnSupportedUbuntu() {
					return pkg.StepResult{
						Error: fmt.Errorf("Checking supported Ubuntu version failed"),
					}
				}
				return pkg.StepResult{Error: nil}
			},
		},
		{
			Name:        "Verify Packages installation connections",
			Description: "Verify Packages installation connections are available",
			Action: func() pkg.StepResult {
				pkg.LogMessage(pkg.Info, "Checking connectivity for package installations")
				err := pkg.CheckPackageInstallConnections()
				if err != nil {
					return pkg.StepResult {
						Error: fmt.Errorf("Checking package installation connections failed: %s", err.Error()),
					}
				}
				return pkg.StepResult{Error: nil}
			},
		},
		{
			Name:        "Check the availability of GPU (if on GPU node)",
			Description: "Check the availability of GPU (if on GPU node)",
			Action: func() pkg.StepResult {
				if !viper.GetBool("GPU_NODE") {
					pkg.LogMessage(pkg.Info, "Not checking the availability of GPU (not a GPU node)")
					return pkg.StepResult{Error: nil}
				}
				pkg.LogMessage(pkg.Info, "Check the availability of GPU on the node")

				err := pkg.CheckGPUAvailability()
				if err != nil {
					return pkg.StepResult{
						Error: fmt.Errorf("Checking GPU availability Failed: %s", err.Error()),
					}
				}
				return pkg.StepResult{Error: nil}
			},
		},
		{
			Name:        "Configure Firewall",
			Description: "Open required ports",
			Action: func() pkg.StepResult {
				pkg.LogMessage(pkg.Info, "Proofing firewall and ports")
				err := pkg.CheckPortsBeforeOpening()
				if err != nil {
					return pkg.StepResult{
						Error: fmt.Errorf("Checking Ports Before Opening Failed: %s", err.Error()),
					}
				}
				return pkg.StepResult{Error: nil}
			},
		},
		{
			Name:        "Verify Inotify Configuration",
			Description: "Verify max Inotify instances are set correctly",
			Action: func() pkg.StepResult {
				pkg.LogMessage(pkg.Info, "Proofing inotify configuration")
				err := pkg.CheckInotifyConfig()
				if err != nil {
					return pkg.StepResult{
						Error: fmt.Errorf("Checking Inotify configuration failed: %s", err.Error()),
					}
				}
				return pkg.StepResult{Error: nil}
			},
		},
	}

	pkg.RunStepsWithUI(steps)
}
