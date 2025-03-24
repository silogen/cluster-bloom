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
	"time"

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
				pkg.LogMessage(pkg.Debug, "Config value for 'demo': "+viper.GetString("demo"))
				pkg.LogMessage(pkg.Info, "simulating work")
				time.Sleep(2 * time.Second)
				return pkg.StepResult{Error: nil}
			},
		},
		{
			Name:        "Install Packages",
			Description: "Install required packages",
			Action: func() pkg.StepResult {
				pkg.LogMessage(pkg.Info, "simulating work")
				time.Sleep(2 * time.Second)
				return pkg.StepResult{Error: nil}
			},
		},
		{
			Name:        "Configure Firewall",
			Description: "Open required ports",
			Action: func() pkg.StepResult {
				pkg.LogMessage(pkg.Info, "simulating work")
				time.Sleep(2 * time.Second)
				return pkg.StepResult{Error: nil}
			},
		},
		{
			Name:        "Verify Configuration",
			Description: "Verify inotify instances",
			Action: func() pkg.StepResult {
				pkg.LogMessage(pkg.Info, "simulating work")
				time.Sleep(2 * time.Second)
				return pkg.StepResult{Error: nil}
			},
		},
	}

	pkg.RunStepsWithUI(steps)
}
