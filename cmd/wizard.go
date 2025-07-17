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
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// Validation functions
func validateBool(input string) error {
	lower := strings.ToLower(strings.TrimSpace(input))
	validValues := []string{"true", "false", "t", "f", "yes", "no", "y", "n", "1", "0"}
	for _, v := range validValues {
		if lower == v {
			return nil
		}
	}
	return fmt.Errorf("invalid boolean value. Please enter: true/false, yes/no, y/n, or 1/0")
}

func validateIP(input string) error {
	if input == "" {
		return fmt.Errorf("IP address is required")
	}
	ip := net.ParseIP(input)
	if ip == nil {
		return fmt.Errorf("invalid IP address format. Please enter a valid IPv4 or IPv6 address")
	}
	return nil
}

func validateURL(input string) error {
	if input == "" {
		return nil // URLs are optional
	}
	u, err := url.Parse(input)
	if err != nil {
		return fmt.Errorf("invalid URL format: %v", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("URL must include scheme (http/https) and host")
	}
	return nil
}

func validateToken(input string) error {
	if input == "" {
		return fmt.Errorf("token is required")
	}
	// Basic validation - no spaces, reasonable length
	if strings.Contains(input, " ") {
		return fmt.Errorf("token cannot contain spaces")
	}
	if len(input) < 6 {
		return fmt.Errorf("token seems too short (minimum 6 characters)")
	}
	return nil
}

func validateDiskList(input string) error {
	if input == "" {
		return nil // Disk lists are optional
	}
	disks := strings.Split(input, ",")
	for _, disk := range disks {
		disk = strings.TrimSpace(disk)
		if !strings.HasPrefix(disk, "/dev/") {
			return fmt.Errorf("disk path '%s' must start with /dev/", disk)
		}
		// Basic pattern check for disk names
		// Matches: /dev/sda, /dev/sdb1, /dev/nvme0n1, /dev/nvme0n1p1, etc.
		matched, _ := regexp.MatchString(`^/dev/(sd[a-z]+[0-9]*|nvme[0-9]+n[0-9]+(p[0-9]+)?|hd[a-z]+[0-9]*|vd[a-z]+[0-9]*)$`, disk)
		if !matched {
			return fmt.Errorf("invalid disk path format: '%s'. Expected format: /dev/sdX, /dev/nvmeXnY, /dev/hdX, or /dev/vdX", disk)
		}
	}
	return nil
}

func validateStepsList(input string) error {
	if input == "" {
		return nil // Step lists are optional
	}
	
	// Valid step IDs from the codebase
	validSteps := []string{
		"CheckUbuntuStep", "InstallDependentPackagesStep", "OpenPortsStep",
		"CheckPortsBeforeOpeningStep", "InstallK8SToolsStep", "InotifyInstancesStep",
		"SetupAndCheckRocmStep", "SetupRKE2Step", "CleanDisksStep",
		"SetupMultipathStep", "UpdateModprobeStep", "SelectDrivesStep",
		"MountSelectedDrivesStep", "GenerateLonghornDiskStringStep",
		"SetupMetallbStep", "SetupLonghornStep", "CreateMetalLBConfigStep",
		"PrepareRKE2Step", "HasSufficientRancherPartitionStep",
		"NVMEDrivesAvailableStep", "SetupOnePasswordSecretStep",
		"SetupClusterForgeStep", "UpdateUdevRulesStep", "CleanLonghornMountsStep",
		"UninstallRKE2Step", "SetupKubeConfig", "FinalOutput",
	}
	
	steps := strings.Split(input, ",")
	for _, step := range steps {
		step = strings.TrimSpace(step)
		found := false
		for _, validStep := range validSteps {
			if step == validStep {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid step name: '%s'. Run 'bloom help' to see valid step names", step)
		}
	}
	return nil
}

func validateClusterForgeRelease(input string) error {
	if input == "" {
		return nil
	}
	if input == "none" {
		return nil // Special value to skip installation
	}
	// Must be a valid URL
	return validateURL(input)
}

type ConfigOption struct {
	Key         string
	Description string
	Default     interface{}
	Required    bool
	Conditional string // Condition when this option is relevant
	Validator   func(string) error // Validation function
}

var configOptions = []ConfigOption{
	{
		Key:         "FIRST_NODE",
		Description: "Is this the first node in the cluster? Set to false for additional nodes joining an existing cluster.",
		Default:     true,
		Required:    true,
		Validator:   validateBool,
	},
	{
		Key:         "GPU_NODE",
		Description: "Does this node have GPUs? Set to false for CPU-only nodes. When true, ROCm will be installed and configured.",
		Default:     true,
		Required:    true,
		Validator:   validateBool,
	},
	{
		Key:         "SERVER_IP",
		Description: "IP address of the RKE2 server (first node). Required when FIRST_NODE is false.",
		Default:     "",
		Required:    false,
		Conditional: "FIRST_NODE=false",
		Validator:   validateIP,
	},
	{
		Key:         "JOIN_TOKEN",
		Description: "Token used to join additional nodes to the cluster. Required when FIRST_NODE is false.",
		Default:     "",
		Required:    false,
		Conditional: "FIRST_NODE=false",
		Validator:   validateToken,
	},
	{
		Key:         "OIDC_URL",
		Description: "URL of the OIDC provider for authentication. Leave empty to skip OIDC configuration.",
		Default:     "",
		Required:    false,
		Validator:   validateURL,
	},
	{
		Key:         "SKIP_DISK_CHECK",
		Description: "Skip disk-related operations. Set to true if you don't want automatic disk setup.",
		Default:     false,
		Required:    false,
		Validator:   validateBool,
	},
	{
		Key:         "SELECTED_DISKS",
		Description: "Comma-separated list of specific disk devices to use. Example: '/dev/sdb,/dev/sdc'. Leave empty for automatic selection.",
		Default:     "",
		Required:    false,
		Validator:   validateDiskList,
	},
	{
		Key:         "LONGHORN_DISKS",
		Description: "Comma-separated list of disk paths for Longhorn storage. Leave empty for automatic configuration.",
		Default:     "",
		Required:    false,
		Validator:   validateDiskList,
	},
	{
		Key:         "ONEPASS_CONNECT_TOKEN",
		Description: "1Password Connect integration token. Leave empty to skip 1Password setup.",
		Default:     "",
		Required:    false,
		Validator:   nil, // No specific validation for tokens
	},
	{
		Key:         "CLUSTERFORGE_RELEASE",
		Description: "ClusterForge release URL or 'none' to skip installation.",
		Default:     "https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz",
		Required:    false,
		Validator:   validateClusterForgeRelease,
	},
	{
		Key:         "DISABLED_STEPS",
		Description: "Comma-separated list of steps to skip. Example: 'SetupLonghornStep,SetupMetallbStep'. Leave empty to run all steps.",
		Default:     "",
		Required:    false,
		Validator:   validateStepsList,
	},
	{
		Key:         "ENABLED_STEPS",
		Description: "Comma-separated list of steps to run. If specified, only these steps will run. Leave empty to run all steps.",
		Default:     "",
		Required:    false,
		Validator:   validateStepsList,
	},
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
- Optional integrations (1Password, ClusterForge)
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

	config := make(map[string]interface{})
	reader := bufio.NewReader(os.Stdin)

	for _, option := range configOptions {
		// Skip conditional options if condition not met
		if option.Conditional != "" && !shouldAskConditional(config, option.Conditional) {
			continue
		}

		value := askForOption(reader, option)
		if value != nil {
			config[option.Key] = value
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
	fmt.Println("To use this configuration, run:")
	fmt.Printf("  sudo ./bloom --config %s\n\n", filename)
	
	// Show generated config
	fmt.Println("Generated configuration:")
	fmt.Println("─────────────────────────")
	fmt.Println(string(yamlData))
}

func shouldAskConditional(config map[string]interface{}, condition string) bool {
	parts := strings.Split(condition, "=")
	if len(parts) != 2 {
		return true
	}

	key := parts[0]
	expectedValue := parts[1]
	
	if actualValue, exists := config[key]; exists {
		return fmt.Sprintf("%v", actualValue) == expectedValue
	}
	
	return false
}

func askForOption(reader *bufio.Reader, option ConfigOption) interface{} {
	fmt.Printf("\n%s:\n", strings.ToUpper(option.Key))
	fmt.Printf("  %s\n", option.Description)
	
	if option.Conditional != "" {
		fmt.Printf("  (Relevant when: %s)\n", option.Conditional)
	}
	
	for {
		defaultStr := formatDefault(option.Default)
		if option.Default == "" {
			fmt.Printf("  Default: %s [press Enter to skip]\n", defaultStr)
		} else {
			fmt.Printf("  Default: %s [press Enter to use default]\n", defaultStr)
		}
		fmt.Print("  Enter value: ")

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			return option.Default
		}

		input = strings.TrimSpace(input)
		
		// Use default if empty
		if input == "" {
			if option.Default == "" {
				return nil // Don't include empty strings in config
			}
			return option.Default
		}

		// Validate input if validator exists
		if option.Validator != nil {
			if err := option.Validator(input); err != nil {
				fmt.Printf("  ❌ Error: %v\n", err)
				fmt.Println("  Please try again.")
				continue
			}
		}

		// Convert to appropriate type
		return convertValue(input, option.Default)
	}
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