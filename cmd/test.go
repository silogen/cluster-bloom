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
	"os"
	"strings"
	"time"

	"github.com/silogen/cluster-bloom/pkg"
	"github.com/silogen/cluster-bloom/pkg/args"
	"github.com/silogen/cluster-bloom/pkg/mockablecmd"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var testCmd = &cobra.Command{
	Use:   "test [config-file...]",
	Short: "Test a node to check readiness for cluster use",
	Long:  `Runs through steps to validate all settings and prerequisite SW are setup correctly. Accepts multiple config files as arguments.`,
	Run: func(cmd *cobra.Command, args []string) {
		testSteps(args)
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func testSteps(configFiles []string) {
	if len(configFiles) == 0 {
		fmt.Println("Error: No config files provided")
		fmt.Println("Usage: cluster-bloom test <config-file> [config-file...]")
		os.Exit(1)
	}

	fmt.Println("---")
	fmt.Printf("total_configs: %d\n", len(configFiles))
	fmt.Println("test_runs:")

	allSuccess := true
	passedCount := 0
	failedCount := 0
	var failedConfigs []string

	for configIdx, configFile := range configFiles {
		success := runTestConfig(configFile, configIdx)
		if success {
			passedCount++
		} else {
			failedCount++
			failedConfigs = append(failedConfigs, configFile)
			allSuccess = false
		}
	}

	// Print overall summary
	fmt.Println("overall_summary:")
	fmt.Printf("  total: %d\n", len(configFiles))
	fmt.Printf("  passed: %d\n", passedCount)
	fmt.Printf("  failed: %d\n", failedCount)
	if len(failedConfigs) > 0 {
		fmt.Println("  failed_configs:")
		for _, config := range failedConfigs {
			fmt.Printf("    - %s\n", config)
		}
	}
	if allSuccess {
		fmt.Println("  success: true")
	} else {
		fmt.Println("  success: false")
		os.Exit(1)
	}
}

func runTestConfig(configFile string, configIdx int) bool {
	fmt.Printf("  - config_file: %s\n", configFile)
	fmt.Printf("    config_number: %d\n", configIdx+1)

	// Reset viper and mocks for this config
	viper.Reset()
	mockablecmd.ResetMocks()

	// Set defaults from args package
	for _, arg := range args.Arguments {
		viper.SetDefault(arg.Key, arg.Default)
	}

	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("    error: \"Failed to read config: %v\"\n", err)
		fmt.Println("    success: false")
		return false
	}

	// Load mocks from the new config
	mockablecmd.LoadMocks()

	// Get the root steps (the actual installation steps)
	steps := rootSteps()
	enabledSteps := pkg.CalculateEnabledSteps(steps)

	// Get expected error from config if present
	expectedError := viper.GetString("expected_error")

	pkg.LogMessage(pkg.Info, fmt.Sprintf("Total steps to execute: %d", len(enabledSteps)))

	fmt.Printf("    total_steps: %d\n", len(enabledSteps))
	fmt.Println("    steps:")

	var finalErr error
	var completedSteps []string
	var failedStep string

	for i, step := range enabledSteps {
		fmt.Printf("      - name: %s\n", step.Name)
		fmt.Printf("        description: %s\n", step.Description)
		fmt.Printf("        number: %d\n", i+1)

		pkg.LogMessage(pkg.Info, fmt.Sprintf("Starting step: %s", step.Name))

		startTime := time.Now()

		result := pkg.StepResult{Error: nil, Message: ""}
		if step.Skip != nil && step.Skip() {
			fmt.Println("        status: skipped")
			pkg.LogMessage(pkg.Info, fmt.Sprintf("Step %s is skipped", step.Name))
		} else {
			result = step.Action()
		}

		duration := time.Since(startTime)

		if result.Error != nil {
			finalErr = result.Error
			failedStep = step.Name
			fmt.Println("        status: failed")
			fmt.Printf("        error: \"%v\"\n", result.Error)
			fmt.Printf("        duration_ms: %d\n", duration.Milliseconds())
			pkg.LogMessage(pkg.Error, fmt.Sprintf("Execution failed: %v", result.Error))
			break
		} else {
			completedSteps = append(completedSteps, step.Name)
			fmt.Println("        status: completed")
			if result.Message != "" {
				fmt.Printf("        message: \"%s\"\n", result.Message)
				pkg.LogMessage(pkg.Info, result.Message)
			}
			fmt.Printf("        duration_ms: %d\n", duration.Milliseconds())
			pkg.LogMessage(pkg.Info, fmt.Sprintf("Completed in %v", duration.Round(time.Millisecond)))
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Print summary for this config
	fmt.Println("    summary:")
	fmt.Printf("      total: %d\n", len(enabledSteps))
	fmt.Printf("      completed: %d\n", len(completedSteps))

	// Determine success based on expected error
	success := false
	if finalErr != nil {
		fmt.Printf("      failed: 1\n")
		fmt.Printf("      failed_step: %s\n", failedStep)
		fmt.Printf("      error: \"%v\"\n", finalErr)

		// Check if this error was expected
		if expectedError != "" && strings.Contains(finalErr.Error(), expectedError) {
			success = true
			fmt.Printf("      expected_error_matched: true\n")
		}

		if success {
			fmt.Println("      success: true")
		} else {
			fmt.Println("      success: false")
		}
	} else {
		fmt.Printf("      failed: 0\n")
		success = true
		fmt.Println("      success: true")
	}

	if len(completedSteps) > 0 {
		fmt.Println("      completed_steps:")
		for _, stepName := range completedSteps {
			fmt.Printf("        - %s\n", stepName)
		}
	}

	return success
}
