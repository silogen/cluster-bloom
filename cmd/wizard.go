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
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/silogen/cluster-bloom/pkg/args"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// Helper function to get first validator from an arg
func getValidator(arg args.Arg) func(string) error {
	if len(arg.Validators) > 0 {
		return arg.Validators[0]
	}
	return nil
}

// Helper function to check if arg is required based on type
func isArgRequired(arg args.Arg) bool {
	return strings.HasPrefix(arg.Type, "non-empty-")
}

// Helper to convert args.UsedWhen dependencies to conditional string for display
func getDependencyString(arg args.Arg) string {
	if len(arg.Dependencies) == 0 {
		return ""
	}
	// For wizard display, just show the first dependency
	dep := arg.Dependencies[0]
	return fmt.Sprintf("%s=%s", dep.Arg, strings.TrimPrefix(dep.Type, "equals_"))
}

// Check if arg dependencies are satisfied based on config map
func checkDependencies(arg args.Arg, config map[string]interface{}) bool {
	if len(arg.Dependencies) == 0 {
		return true
	}

	for _, dep := range arg.Dependencies {
		value, exists := config[dep.Arg]
		if !exists {
			return false
		}

		switch {
		case dep.Type == "equals_true":
			if boolVal, ok := value.(bool); !ok || !boolVal {
				return false
			}
		case dep.Type == "equals_false":
			if boolVal, ok := value.(bool); !ok || boolVal {
				return false
			}
		case strings.HasPrefix(dep.Type, "equals_"):
			expectedValue := strings.TrimPrefix(dep.Type, "equals_")
			if fmt.Sprintf("%v", value) != expectedValue {
				return false
			}
		}
	}
	return true
}

var wizardCmd = &cobra.Command{
	Use:   "wizard",
	Short: "Interactive wizard to generate bloom.yaml configuration",
	Long: `
The wizard command guides you through configuring Cluster-Bloom by asking questions
about each configuration option. It generates a bloom.yaml file that can be used
with the --config flag to install a server.

The wizard will ask about:
- Node type (first node vs additional node)
- GPU availability and ROCm setup
- Storage configuration
- Optional integrations (ClusterForge)
- Advanced options (OIDC, custom steps)
`,
	Run: func(cmd *cobra.Command, args []string) {
		runWizard()
	},
}

func init() {
	rootCmd.AddCommand(wizardCmd)
}

func runWizard() {
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    Cluster-Bloom Configuration Wizard         ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("This wizard will guide you through configuring Cluster-Bloom.")
	fmt.Println("Press Enter to use default values shown in brackets.")
	fmt.Println()

	// Initialize arguments
	SetArguments()

	config := make(map[string]interface{})
	reader := bufio.NewReader(os.Stdin)

	for _, arg := range args.Arguments {
		// Skip if dependencies not met
		if !checkDependencies(arg, config) {
			continue
		}

		value := askForArg(reader, arg, config)
		if value != nil {
			config[arg.Key] = value
		}
	}

	// Validate TLS configuration
	if useCertManager, exists := config["USE_CERT_MANAGER"]; exists && !useCertManager.(bool) {
		certOption, hasCertOption := config["CERT_OPTION"]
		if !hasCertOption || certOption == nil || certOption == "" {
			fmt.Println("\n❌ Error: When USE_CERT_MANAGER is false, CERT_OPTION must be specified")
			fmt.Println("Please run the wizard again and choose 'existing' or 'generate' for certificate option.")
			os.Exit(1)
		}

		if certOption == "existing" {
			tlsCert, hasCert := config["TLS_CERT"]
			tlsKey, hasKey := config["TLS_KEY"]

			if !hasCert || tlsCert == nil || tlsCert == "" || !hasKey || tlsKey == nil || tlsKey == "" {
				fmt.Println("\n❌ Error: When CERT_OPTION is 'existing', both TLS_CERT and TLS_KEY must be provided")
				fmt.Println("Please run the wizard again and provide TLS certificate files.")
				os.Exit(1)
			}
		}
	}

	// Generate YAML
	yamlData, err := yaml.Marshal(config)
	if err != nil {
		fmt.Printf("Error generating YAML: %v\n", err)
		os.Exit(1)
	}

	// Write to file
	filename := "bloom.yaml"
	err = os.WriteFile(filename, yamlData, 0644)
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    Configuration Complete!                    ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Printf("\nConfiguration saved to: %s\n\n", filename)

	// Show generated config
	fmt.Println("Generated configuration:")
	fmt.Println("─────────────────────────")
	fmt.Println(string(yamlData))
	fmt.Println()

	// Ask if user wants to run bloom now
	fmt.Print("Would you like to run bloom with this configuration now? (y/n): ")
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		return
	}

	input = strings.ToLower(strings.TrimSpace(input))
	if input == "y" || input == "yes" {
		fmt.Println("\nRunning bloom with the generated configuration...")
		fmt.Printf("Command: sudo ./bloom --config %s\n\n", filename)

		// Execute bloom with the generated config
		err := runBloomWithConfig(filename)
		if err != nil {
			fmt.Printf("Error running bloom: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Println("\nTo use this configuration later, run:")
		fmt.Printf("  sudo ./bloom --config %s\n\n", filename)
	}
}

func runBloomWithConfig(configFile string) error {
	// Check if the bloom executable exists
	bloomPath := "./bloom"
	if _, err := os.Stat(bloomPath); os.IsNotExist(err) {
		return fmt.Errorf("bloom executable not found at %s", bloomPath)
	}

	// Execute bloom with the config file
	cmd := exec.Command("sudo", bloomPath, "--config", configFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

func askForArg(reader *bufio.Reader, arg args.Arg, config map[string]interface{}) interface{} {
	fmt.Printf("\n%s:\n", strings.ToUpper(arg.Key))
	fmt.Printf("  %s\n", arg.Description)

	depString := getDependencyString(arg)
	if depString != "" {
		fmt.Printf("  (Relevant when: %s)\n", depString)
	}

	required := isArgRequired(arg)
	validator := getValidator(arg)

	// Get type-specific validator based on arg.Type
	if validator == nil {
		switch arg.Type {
		case "bool":
			validator = args.ValidateBool
		case "url", "non-empty-url":
			validator = args.ValidateURL
		case "ip-address", "non-empty-ip-address":
			validator = args.ValidateIPAddress
		case "file":
			validator = validateFilePath
		case "enum":
			validator = func(input string) error {
				if len(arg.Options) > 0 {
					for _, opt := range arg.Options {
						if input == opt {
							return nil
						}
					}
					return fmt.Errorf("must be one of %v", arg.Options)
				}
				return nil
			}
		}
	}

	for {
		defaultStr := formatDefault(arg.Default)
		if arg.Default == "" {
			fmt.Printf("  Default: %s [press Enter to skip]\n", defaultStr)
		} else {
			fmt.Printf("  Default: %s [press Enter to use default]\n", defaultStr)
		}
		fmt.Print("  Enter value: ")

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			return arg.Default
		}

		input = strings.TrimSpace(input)

		// Use default if empty
		if input == "" {
			if required && (arg.Default == nil || arg.Default == "" || arg.Default == false) {
				fmt.Printf("  ❌ Error: This field is required\n")
				fmt.Println("  Please try again.")
				continue
			}
			if arg.Default == "" {
				return nil // Don't include empty strings in config
			}
			return arg.Default
		}

		// Validate input if validator exists
		if validator != nil {
			if err := validator(input); err != nil {
				fmt.Printf("  ❌ Error: %v\n", err)
				fmt.Println("  Please try again.")
				continue
			}
		}

		// Convert to appropriate type
		return convertValue(input, arg.Default)
	}
}

func validateFilePath(input string) error {
	if input == "" {
		return nil // File path is optional
	}
	// Check if file exists
	if _, err := os.Stat(input); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", input)
	}
	return nil
}

func formatDefault(value interface{}) string {
	switch v := value.(type) {
	case string:
		if v == "" {
			return "(empty)"
		}
		return fmt.Sprintf("'%s'", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func convertValue(input string, defaultValue interface{}) interface{} {
	switch defaultValue.(type) {
	case bool:
		// Accept various boolean representations
		lower := strings.ToLower(input)
		switch lower {
		case "true", "t", "yes", "y", "1":
			return true
		case "false", "f", "no", "n", "0":
			return false
		default:
			// Try to parse as boolean
			if b, err := strconv.ParseBool(input); err == nil {
				return b
			}
			return input
		}
	case int:
		if i, err := strconv.Atoi(input); err == nil {
			return i
		}
		return input
	default:
		// Return as string
		return input
	}
}
