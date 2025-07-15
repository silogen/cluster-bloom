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
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/silogen/cluster-bloom/pkg"
)

var demoCmd = &cobra.Command{
	Use:   "demo-ui",
	Short: "Silly demo to show UI",
	Long:  "Silly demo to show UI",
	Run: func(cmd *cobra.Command, args []string) {
		pkg.LogMessage(pkg.Info, "Config value for 'demo': "+viper.GetString("demo"))
		pkg.LogMessage(pkg.Debug, "Starting package installation")
		demoSteps()
	},
}

func init() {
	rootCmd.AddCommand(demoCmd)
}

func demoSteps() {
	steps := []pkg.Step{
		pkg.DemoCheckUbuntuStep,
		pkg.DemoPackagesStep,
		pkg.DemoFirewallStep,
		pkg.DemoMinioStep,
		pkg.DemoDashboardStep,
	}

	pkg.RunStepsWithUI(steps)
}
