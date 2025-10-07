package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/silogen/cluster-bloom/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type UsedWhen struct {
	Arg  string
	Type string
}

type Arg struct {
	Key          string
	Default      interface{}
	Description  string
	Type         string
	Options      []string
	Dependencies []UsedWhen
	Validators   []func(value string) error
}

var Arguments = []Arg{
	// Core cluster configuration
	{
		Key:         "FIRST_NODE",
		Default:     true,
		Description: "Set to true if this is the first node in the cluster (default: true).",
		Type:        "bool",
	},
	{
		Key:          "CONTROL_PLANE",
		Default:      false,
		Description:  "Set to true if this node should be a control plane node (default: false, only applies when FIRST_NODE is false).",
		Type:         "bool",
		Dependencies: []UsedWhen{{"FIRST_NODE", "equals_false"}},
	},
	{
		Key:         "GPU_NODE",
		Default:     true,
		Description: "Set to true if this node has GPUs (default: true).",
		Type:        "bool",
	},

	// Network configuration
	{
		Key:          "DOMAIN",
		Default:      "",
		Description:  "The domain name for the cluster (e.g., \"cluster.example.com\") (required).",
		Type:         "non-empty-string",
		Dependencies: []UsedWhen{{"FIRST_NODE", "equals_true"}},
	},
	{
		Key:          "SERVER_IP",
		Default:      "",
		Description:  "IP address of the RKE2 server (required for non-first nodes).",
		Type:         "non-empty-string",
		Dependencies: []UsedWhen{{"FIRST_NODE", "equals_false"}},
	},
	{
		Key:          "JOIN_TOKEN",
		Default:      "",
		Description:  "Token for joining additional nodes to the cluster (required for non-first nodes).",
		Type:         "non-empty-string",
		Dependencies: []UsedWhen{{"FIRST_NODE", "equals_false"}},
		Validators:   []func(value string) error{validateJoinTokenArg},
	},

	// TLS/Certificate configuration
	{
		Key:          "USE_CERT_MANAGER",
		Default:      false,
		Description:  "Use cert-manager with Let's Encrypt for automatic TLS certificates (default: false).",
		Type:         "bool",
		Dependencies: []UsedWhen{{"FIRST_NODE", "equals_true"}},
	},
	{
		Key:          "CERT_OPTION",
		Default:      "",
		Description:  "Certificate option when USE_CERT_MANAGER is false. Choose 'existing' or 'generate' (default: \"\").",
		Type:         "enum",
		Options:      []string{"existing", "generate"},
		Dependencies: []UsedWhen{{"USE_CERT_MANAGER", "equals_false"}, UsedWhen{"FIRST_NODE", "equals_true"}},
	},
	{
		Key:          "TLS_CERT",
		Default:      "",
		Description:  "Path to TLS certificate file for ingress (required if CERT_OPTION is 'existing').",
		Type:         "file",
		Dependencies: []UsedWhen{{"CERT_OPTION", "equals_existing"}},
	},
	{
		Key:          "TLS_KEY",
		Default:      "",
		Description:  "Path to TLS private key file for ingress (required if CERT_OPTION is 'existing').",
		Type:         "file",
		Dependencies: []UsedWhen{{"CERT_OPTION", "equals_existing"}},
	},

	// Authentication
	{
		Key:         "OIDC_URL",
		Default:     "",
		Description: "The URL of the OIDC provider (default: \"\").",
		Type:        "url",
	},

	// Disk and storage configuration
	{
		Key:         "SKIP_DISK_CHECK",
		Default:     false,
		Description: "Set to true to skip disk-related operations (default: false).",
		Type:        "bool",
	},
	{
		Key:         "LONGHORN_DISKS",
		Default:     "",
		Description: "Comma-separated list of disk paths to use for Longhorn (default: \"\").",
		Type:        "string",
		Validators:  []func(value string) error{validateLonghornDisksArg},
	},
	{
		Key:         "SELECTED_DISKS",
		Default:     "",
		Description: "Comma-separated list of disk devices. Example \"/dev/sdb,/dev/sdc\" (default: \"\").",
		Type:        "string",
	},

	// GPU/ROCm configuration
	{
		Key:          "ROCM_BASE_URL",
		Default:      "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/",
		Description:  "ROCm base repository URL (default: this URL).",
		Type:         "non-empty-url",
		Dependencies: []UsedWhen{{"GPU_NODE", "equals_true"}},
	},
	{
		Key:          "ROCM_DEB_PACKAGE",
		Default:      "amdgpu-install_6.3.60302-1_all.deb",
		Description:  "ROCm DEB package name (default: this package).",
		Type:         "non-empty-string",
		Dependencies: []UsedWhen{{"GPU_NODE", "equals_true"}},
	},

	// External URLs
	{
		Key:         "CLUSTERFORGE_RELEASE",
		Default:     "https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz",
		Description: "The version of Cluster-Forge to install (default: this URL). Pass the URL for a specific release, or 'none' to not install ClusterForge.",
		Type:        "url",
	},
	{
		Key:         "RKE2_INSTALLATION_URL",
		Default:     "https://get.rke2.io",
		Description: "RKE2 installation script URL (default: this URL).",
		Type:        "non-empty-url",
	},

	// Step control
	{
		Key:         "DISABLED_STEPS",
		Default:     "",
		Description: "Comma-separated list of steps to skip. Example \"SetupLonghornStep,SetupMetallbStep\" (default: \"\").",
		Type:        "string",
		Validators:  []func(value string) error{validateStepNamesArg, validateDisabledStepsWarnings, validateDisabledStepsConflict},
	},
	{
		Key:         "ENABLED_STEPS",
		Default:     "",
		Description: "Comma-separated list of steps to perform. If empty, perform all. Example \"SetupLonghornStep,SetupMetallbStep\" (default: \"\").",
		Type:        "string",
		Validators:  []func(value string) error{validateStepNamesArg},
	},
}

func evaluateDependency(dep UsedWhen) bool {
	switch {
	case dep.Type == "equals_true":
		return viper.GetBool(dep.Arg)
	case dep.Type == "equals_false":
		return !viper.GetBool(dep.Arg)
	case strings.HasPrefix(dep.Type, "equals_"):
		expectedValue := strings.TrimPrefix(dep.Type, "equals_")
		return viper.GetString(dep.Arg) == expectedValue
	default:
		return false
	}
}

func IsArgUsed(arg Arg) bool {
	if len(arg.Dependencies) == 0 {
		return true
	}

	// All dependencies must be satisfied for the arg to be used
	for _, dep := range arg.Dependencies {
		if !evaluateDependency(dep) {
			return false
		}
	}
	return true
}

// validateJoinTokenArg validates RKE2/K3s join token format
func validateJoinTokenArg(token string) error {
	// RKE2/K3s tokens are typically:
	// - Base64-encoded or hex strings
	// - Usually 64+ characters long
	// - Contain alphanumeric characters, +, /, =

	// Empty tokens are handled by validateToken function
	if token == "" {
		return nil
	}

	if len(token) < 32 {
		return fmt.Errorf("JOIN_TOKEN is too short (minimum 32 characters), got %d characters", len(token))
	}

	if len(token) > 512 {
		return fmt.Errorf("JOIN_TOKEN is too long (maximum 512 characters), got %d characters", len(token))
	}

	// Allow base64 characters, hex characters, and common separators including colons
	validTokenPattern := regexp.MustCompile(`^[a-zA-Z0-9+/=_.:-]+$`)
	if !validTokenPattern.MatchString(token) {
		return fmt.Errorf("JOIN_TOKEN contains invalid characters (only alphanumeric, +, /, =, _, ., :, - allowed)")
	}

	return nil
}

// validateStepNamesArg validates that step names are valid against the steps from rootSteps
func validateStepNamesArg(stepNames string) error {
	if stepNames == "" {
		return nil // Empty step lists are allowed
	}

	// Get valid step IDs from rootSteps by extracting their Id field
	validStepIDs := make([]string, len(rootSteps))
	for i, step := range rootSteps {
		validStepIDs[i] = step.Id
	}

	// Split comma-separated list and validate each step name
	inputSteps := strings.Split(stepNames, ",")
	for _, inputStep := range inputSteps {
		inputStep = strings.TrimSpace(inputStep)
		if inputStep == "" {
			continue // Skip empty entries
		}

		// Check if step name is valid
		valid := false
		for _, validStep := range validStepIDs {
			if inputStep == validStep {
				valid = true
				break
			}
		}

		if !valid {
			return fmt.Errorf("invalid step name '%s'. Valid step names are: %s",
				inputStep, strings.Join(validStepIDs, ", "))
		}
	}

	return nil
}

// validateDisabledStepsWarnings warns about disabling essential steps
func validateDisabledStepsWarnings(stepNames string) error {
	if stepNames == "" {
		return nil
	}

	// Check for essential steps being disabled
	if strings.Contains(stepNames, "CheckUbuntuStep") {
		log.Warnf("CheckUbuntuStep is disabled - system compatibility may not be verified")
	}

	if strings.Contains(stepNames, "SetupRKE2Step") {
		log.Warnf("SetupRKE2Step is disabled - Kubernetes cluster will not be set up")
	}

	// Check if SetupAndCheckRocmStep is disabled when GPU_NODE=true
	if strings.Contains(stepNames, "SetupAndCheckRocmStep") && viper.GetBool("GPU_NODE") {
		log.Warnf("GPU_NODE=true but SetupAndCheckRocmStep is disabled - GPU functionality may not work")
	}

	return nil
}

// validateDisabledStepsConflict ensures DISABLED_STEPS and ENABLED_STEPS are not both set
func validateDisabledStepsConflict(stepNames string) error {
	if stepNames == "" {
		return nil
	}

	enabledSteps := viper.GetString("ENABLED_STEPS")
	if enabledSteps != "" {
		return fmt.Errorf("DISABLED_STEPS and ENABLED_STEPS cannot both be set - use one or the other")
	}

	return nil
}

// validateLonghornDisksArg validates LONGHORN_DISKS configuration
func validateLonghornDisksArg(disks string) error {
	// Use the same logic as the existing validation in root.go
	longhornDiskString := pkg.ParseLonghornDiskConfig()
	if len(longhornDiskString) > 63 {
		return fmt.Errorf("LONGHORN_DISKS configuration too long (%d characters), maximum 63 characters allowed. Parsed string: %s",
			len(longhornDiskString), longhornDiskString)
	}
	if strings.Contains(longhornDiskString, "/") {
		return fmt.Errorf("LONGHORN_DISKS must not contain slashes. Parsed string: %s", longhornDiskString)
	}

	return nil
}

func ValidateArgs() error {
	var errors []string

	for _, arg := range Arguments {
		value := viper.GetString(arg.Key)

		// Check if this argument is needed based on its dependencies

		if !IsArgUsed(arg) {
			continue
		}

		// Check for non-empty prefix
		required := strings.HasPrefix(arg.Type, "non-empty-")
		baseType := arg.Type
		if required {
			baseType = strings.TrimPrefix(arg.Type, "non-empty-")
		}

		// Type-specific validation
		switch baseType {
		case "bool":
			// viper.GetBool handles string-to-bool conversion, so we're good
			continue
		case "url":
			if value != "" && value != "none" {
				if u, err := url.Parse(value); err != nil {
					errors = append(errors, fmt.Sprintf("%s: invalid URL format: %v", arg.Key, err))
				} else if u.Scheme == "" || u.Host == "" {
					errors = append(errors, fmt.Sprintf("%s: invalid URL format: missing scheme or host", arg.Key))
				}
			}
		case "file":
			if value != "" {
				if !filepath.IsAbs(value) {
					errors = append(errors, fmt.Sprintf("%s: must be an absolute file path", arg.Key))
				}
				if _, err := os.Stat(value); os.IsNotExist(err) {
					errors = append(errors, fmt.Sprintf("%s: file does not exist: %s", arg.Key, value))
				}
			}
		case "enum":
			if len(arg.Options) > 0 {
				validOption := false
				for _, option := range arg.Options {
					if value == option {
						validOption = true
						break
					}
				}
				if !validOption {
					errors = append(errors, fmt.Sprintf("%s: must be one of %v, got: %s", arg.Key, arg.Options, value))
				}
			}
		case "string":
			// Basic string validation can be added here if needed
		}

		// Run custom validators if provided
		for _, validator := range arg.Validators {
			if err := validator(value); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", arg.Key, err))
			}
		}

		// Check if field is required and empty
		if required && value == "" {
			errors = append(errors, fmt.Sprintf("%s is required", arg.Key))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n- %s", strings.Join(errors, "\n- "))
	}

	return nil
}
