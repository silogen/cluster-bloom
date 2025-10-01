package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Dependency struct {
	Arg  string
	Type string
}

type ArgDefault struct {
	Key          string
	Default      interface{}
	Description  string
	Type         string
	Options      []string
	Dependencies []Dependency
}

var AllArgDefaults = []ArgDefault{
	{"FIRST_NODE", true, "Set to true if this is the first node in the cluster (default: true).", "bool", nil, nil},
	{"CONTROL_PLANE", false, "Set to true if this node should be a control plane node (default: false, only applies when FIRST_NODE is false).", "bool", nil, []Dependency{{"FIRST_NODE", "equals_false"}}},
	{"GPU_NODE", true, "Set to true if this node has GPUs (default: true).", "bool", nil, nil},
	{"OIDC_URL", "", "The URL of the OIDC provider (default: \"\").", "url", nil, nil},
	{"SKIP_DISK_CHECK", false, "Set to true to skip disk-related operations (default: false).", "bool", nil, nil},
	{"LONGHORN_DISKS", "", "Comma-separated list of disk paths to use for Longhorn (default: \"\").", "string", nil, []Dependency{{"SKIP_DISK_CHECK", "equals_false"}}},
	{"CLUSTERFORGE_RELEASE", "https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz", "The version of Cluster-Forge to install (default: this URL). Pass the URL for a specific release, or 'none' to not install ClusterForge.", "url", nil, nil},
	{"ROCM_BASE_URL", "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/", "ROCm base repository URL (default: this URL).", "url", nil, []Dependency{{"GPU_NODE", "equals_true"}}},
	{"ROCM_DEB_PACKAGE", "amdgpu-install_6.3.60302-1_all.deb", "ROCm DEB package name (default: this package).", "string", nil, []Dependency{{"GPU_NODE", "equals_true"}}},
	{"RKE2_INSTALLATION_URL", "https://get.rke2.io", "RKE2 installation script URL (default: this URL).", "url", nil, nil},
	{"DISABLED_STEPS", "", "Comma-separated list of steps to skip. Example \"SetupLonghornStep,SetupMetallbStep\" (default: \"\").", "string", nil, nil},
	{"ENABLED_STEPS", "", "Comma-separated list of steps to perform. If empty, perform all. Example \"SetupLonghornStep,SetupMetallbStep\" (default: \"\").", "string", nil, nil},
	{"SELECTED_DISKS", "", "Comma-separated list of disk devices. Example \"/dev/sdb,/dev/sdc\" (default: \"\").", "string", nil, []Dependency{{"SKIP_DISK_CHECK", "equals_false"}}},
	{"DOMAIN", "", "The domain name for the cluster (e.g., \"cluster.example.com\") (required).", "string", nil, []Dependency{{"FIRST_NODE", "equals_true"}}},
	{"TLS_CERT", "", "Path to TLS certificate file for ingress (required if CERT_OPTION is 'existing').", "file", nil, []Dependency{{"CERT_OPTION", "equals_existing"}, {"USE_CERT_MANAGER", "equals_false"}}},
	{"TLS_KEY", "", "Path to TLS private key file for ingress (required if CERT_OPTION is 'existing').", "file", nil, []Dependency{{"CERT_OPTION", "equals_existing"}, {"USE_CERT_MANAGER", "equals_false"}}},
	{"USE_CERT_MANAGER", false, "Use cert-manager with Let's Encrypt for automatic TLS certificates (default: false).", "bool", nil, []Dependency{{"FIRST_NODE", "equals_true"}}},
	{"CERT_OPTION", "", "Certificate option when USE_CERT_MANAGER is false. Choose 'existing' or 'generate' (default: \"\").", "enum", []string{"existing", "generate"}, []Dependency{{"USE_CERT_MANAGER", "equals_false"}, {"FIRST_NODE", "equals_true"}}},
	{"JOIN_TOKEN", "", "Token for joining additional nodes to the cluster (required for non-first nodes).", "string", nil, []Dependency{{"FIRST_NODE", "equals_false"}}},
	{"SERVER_IP", "", "IP address of the RKE2 server (required for non-first nodes).", "string", nil, []Dependency{{"FIRST_NODE", "equals_false"}}},
}

func ValidateArgs() error {
	var errors []string

	for _, arg := range AllArgDefaults {
		value := viper.GetString(arg.Key)

		switch arg.Type {
		case "bool":
			// viper.GetBool handles string-to-bool conversion, so we're good
			continue
		case "url":
			if value != "" && value != "none" {
				if _, err := url.Parse(value); err != nil {
					errors = append(errors, fmt.Sprintf("%s: invalid URL format: %v", arg.Key, err))
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
			if arg.Key == "CERT_OPTION" && value != "" {
				if value != "existing" && value != "generate" {
					errors = append(errors, fmt.Sprintf("%s: must be 'existing' or 'generate', got: %s", arg.Key, value))
				}
			}
		case "string":
			// Basic string validation can be added here if needed
			continue
		}
	}

	// Cross-field validation
	if !viper.GetBool("FIRST_NODE") {
		if viper.GetString("SERVER_IP") == "" {
			errors = append(errors, "SERVER_IP is required when FIRST_NODE is false")
		}
		if viper.GetString("JOIN_TOKEN") == "" {
			errors = append(errors, "JOIN_TOKEN is required when FIRST_NODE is false")
		}
	}

	if viper.GetBool("FIRST_NODE") && viper.GetString("DOMAIN") == "" {
		errors = append(errors, "DOMAIN is required when FIRST_NODE is true")
	}

	if !viper.GetBool("USE_CERT_MANAGER") && viper.GetBool("FIRST_NODE") {
		certOption := viper.GetString("CERT_OPTION")
		if certOption == "" {
			errors = append(errors, "CERT_OPTION is required when USE_CERT_MANAGER is false")
		} else if certOption == "existing" {
			if viper.GetString("TLS_CERT") == "" {
				errors = append(errors, "TLS_CERT is required when CERT_OPTION is 'existing'")
			}
			if viper.GetString("TLS_KEY") == "" {
				errors = append(errors, "TLS_KEY is required when CERT_OPTION is 'existing'")
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n- %s", strings.Join(errors, "\n- "))
	}

	return nil
}
