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
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// Wrapper functions to match ConfigOption validator signature
func validateURLWrapper(input string) error {
	return validateURL(input, "URL")
}

func validateTokenWrapper(input string) error {
	return validateToken(input, "TOKEN")
}

func validateIPWrapper(input string) error {
	return validateIPAddress(input, "IP_ADDRESS")
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
		"MountSelectedDrivesStep", "GenerateNodeLabelsStep",
		"SetupMetallbStep", "SetupLonghornStep", "CreateMetalLBConfigStep",
		"PrepareRKE2Step", "HasSufficientRancherPartitionStep",
		"NVMEDrivesAvailableStep",
		"SetupClusterForgeStep", "UpdateUdevRulesStep", "CleanLonghornMountsStep",
		"UninstallRKE2Step", "SetupKubeConfig", "CreateBloomConfigMapStep",
		"CreateDomainConfigStep", "FinalOutput",
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
	return validateURL(input, "CLUSTERFORGE_RELEASE")
}

func validateDomain(input string) error {
	if input == "" {
		return fmt.Errorf("domain is required")
	}
	// Basic domain validation - must contain at least one dot and valid characters
	domainRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	if !domainRegex.MatchString(input) {
		return fmt.Errorf("invalid domain format: '%s'. Expected format: example.com or subdomain.example.com", input)
	}
	return nil
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

func validateCertOption(input string) error {
	if input == "" {
		return fmt.Errorf("certificate option is required when USE_CERT_MANAGER is false")
	}
	lower := strings.ToLower(strings.TrimSpace(input))
	if lower != "existing" && lower != "generate" {
		return fmt.Errorf("invalid option: must be 'existing' or 'generate'")
	}
	return nil
}

type ConfigOption struct {
	Key         string
	Description string
	Default     interface{}
	Required    bool
	Conditional string             // Condition when this option is relevant
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
		Key:         "CONTROL_PLANE",
		Description: "Should this node be a control plane node? Set to true for control plane nodes when FIRST_NODE is false.",
		Default:     false,
		Required:    false,
		Conditional: "FIRST_NODE=false",
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
		Validator:   validateIPWrapper,
	},
	{
		Key:         "JOIN_TOKEN",
		Description: "Token used to join additional nodes to the cluster. Required when FIRST_NODE is false.",
		Default:     "",
		Required:    false,
		Conditional: "FIRST_NODE=false",
		Validator:   validateTokenWrapper,
	},
	{
		Key:         "OIDC_URL",
		Description: "URL of the OIDC provider for authentication. To use the bundled cluster-internal Keycloak, use `kc.<your_domain>`. Leave empty to skip OIDC configuration.",
		Default:     "",
		Required:    false,
		Validator:   validateURLWrapper,
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
	{
		Key:         "DOMAIN",
		Description: "Domain name for the cluster (e.g., cluster.example.com). Used for ingress configuration.",
		Default:     "",
		Required:    true,
		Validator:   validateDomain,
	},
	{
		Key:         "USE_CERT_MANAGER",
		Description: "Use cert-manager with Let's Encrypt for automatic TLS certificates. Set to false to provide your own certificates.",
		Default:     false,
		Required:    false,
		Validator:   validateBool,
	},
	{
		Key:         "CERT_OPTION",
		Description: "Certificate option when USE_CERT_MANAGER is false. Choose 'existing' to use existing certificate files, or 'generate' to create a self-signed certificate.",
		Default:     "",
		Required:    false,
		Conditional: "USE_CERT_MANAGER=false",
		Validator:   validateCertOption,
	},
	{
		Key:         "TLS_CERT",
		Description: "Path to TLS certificate file for ingress (PEM format). Required if CERT_OPTION is 'existing'.",
		Default:     "",
		Required:    false,
		Conditional: "CERT_OPTION=existing",
		Validator:   validateFilePath,
	},
	{
		Key:         "TLS_KEY",
		Description: "Path to TLS private key file for ingress (PEM format). Required if CERT_OPTION is 'existing'.",
		Default:     "",
		Required:    false,
		Conditional: "CERT_OPTION=existing",
		Validator:   validateFilePath,
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
			if option.Required && (option.Default == nil || option.Default == "" || option.Default == false) {
				fmt.Printf("  ❌ Error: This field is required\n")
				fmt.Println("  Please try again.")
				continue
			}
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
