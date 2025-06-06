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
	"github.com/silogen/cluster-bloom/pkg"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test a node to check readiness for cluster use",
	Long:  `Runs through steps to validate all settings and prerequisite SW are setup correctly`,
	Run: func(cmd *cobra.Command, args []string) {
		pkg.LogMessage(pkg.Info, "Config value for 'test': "+viper.GetString("test"))
		pkg.LogMessage(pkg.Debug, "Starting node testing")
		testSteps()
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func testSteps() {
	steps := []pkg.Step{
		pkg.DemoCheckUbuntuStep,
		pkg.SelectDrivesStep,
		pkg.MountSelectedDrivesStep,
		pkg.DemoDashboardStep,
	}

	pkg.RunStepsWithUI(steps)
}
