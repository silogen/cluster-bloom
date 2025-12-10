package config

// Argument represents a configuration field in bloom.yaml
type Argument struct {
	Key          string   `json:"key"`
	Type         string   `json:"type"`
	Default      any      `json:"default"`
	Description  string   `json:"description"`
	Options      []string `json:"options,omitempty"`
	Dependencies string   `json:"dependencies,omitempty"`
	Required     bool     `json:"required"`
	Section      string   `json:"section,omitempty"`
	Pattern      string   `json:"pattern,omitempty"`      // HTML5 validation pattern
	PatternTitle string   `json:"patternTitle,omitempty"` // Custom validation error message
}

// Schema returns all bloom.yaml argument definitions
func Schema() []Argument {
	return []Argument{
		// ========================================
		// 📋 Basic Configuration
		// ========================================
		{
			Key:         "FIRST_NODE",
			Type:        "bool",
			Default:     true,
			Description: "Set to true if this is the first node in the cluster.",
			Required:    false,
			Section:     "📋 Basic Configuration",
		},
		{
			Key:         "GPU_NODE",
			Type:        "bool",
			Default:     true,
			Description: "Set to true if this node has GPUs.",
			Required:    false,
			Section:     "📋 Basic Configuration",
		},
		{
			Key:          "DOMAIN",
			Type:         "string",
			Default:      "",
			Description:  "The domain name for the cluster (e.g., \"cluster.example.com\"). Required for first node.",
			Dependencies: "FIRST_NODE=true",
			Required:     true,
			Section:      "📋 Basic Configuration",
			Pattern:      `^([a-z0-9]([a-z0-9\-]*[a-z0-9])?\.)*[a-z0-9]([a-z0-9\-]*[a-z0-9])?$`,
			PatternTitle: "Domain must be lowercase alphanumeric with dots/hyphens (e.g., example.com or sub.example.com). Cannot start/end with hyphen or dot, no special characters.",
		},

		// ========================================
		// 🔗 Additional Node Configuration
		// ========================================
		{
			Key:          "SERVER_IP",
			Type:         "string",
			Default:      "",
			Description:  "IP address of the RKE2 server. Required for non-first nodes.",
			Dependencies: "FIRST_NODE=false",
			Required:     true,
			Section:      "🔗 Additional Node Configuration",
			Pattern:      `^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$|^$`,
			PatternTitle: "Valid IPv4 address format: xxx.xxx.xxx.xxx (0-255 for each octet)",
		},
		{
			Key:          "JOIN_TOKEN",
			Type:         "string",
			Default:      "",
			Description:  "Token for joining additional nodes to the cluster. Required for non-first nodes.",
			Dependencies: "FIRST_NODE=false",
			Required:     true,
			Section:      "🔗 Additional Node Configuration",
		},
		{
			Key:          "CONTROL_PLANE",
			Type:         "bool",
			Default:      false,
			Description:  "Set to true if this node should be a control plane node (only applies when FIRST_NODE is false).",
			Dependencies: "FIRST_NODE=false",
			Required:     false,
			Section:      "🔗 Additional Node Configuration",
		},

		// ========================================
		// 💾 Storage Configuration
		// ========================================
		{
			Key:         "NO_DISKS_FOR_CLUSTER",
			Type:        "bool",
			Default:     false,
			Description: "Set to true to skip all disk-related operations.",
			Required:    false,
			Section:     "💾 Storage Configuration",
		},
		{
			Key:          "CLUSTER_DISKS",
			Type:         "string",
			Default:      "",
			Description:  "Comma-separated list of disk device paths (e.g., \"/dev/nvme0n1,/dev/nvme1n1\").",
			Required:     false,
			Section:      "💾 Storage Configuration",
			Pattern:      `^(/dev/[a-zA-Z0-9]+)(,/dev/[a-zA-Z0-9]+)*$|^$`,
			PatternTitle: "Enter comma-separated device paths like /dev/nvme0n1,/dev/nvme1n1",
		},
		{
			Key:          "CLUSTER_PREMOUNTED_DISKS",
			Type:         "string",
			Default:      "",
			Description:  "Comma-separated list of premounted disk paths for Longhorn.",
			Required:     false,
			Section:      "💾 Storage Configuration",
			Pattern:      `^(disk[0-9]+)(,disk[0-9]+)*$|^$`,
			PatternTitle: "Enter comma-separated disk names like disk1,disk2,disk3",
		},
		{
			Key:         "SKIP_RANCHER_PARTITION_CHECK",
			Type:        "bool",
			Default:     false,
			Description: "Set to true to skip /var/lib/rancher partition size check.",
			Required:    false,
			Section:     "💾 Storage Configuration",
		},

		// ========================================
		// 🔒 SSL/TLS Configuration
		// ========================================
		{
			Key:          "ADDITIONAL_TLS_SAN_URLS",
			Type:         "string",
			Default:      "",
			Description:  "Additional TLS Subject Alternative Name URLs for Kubernetes API server certificate. Comma-separated (e.g., \"api.example.com, kubernetes.example.com\").",
			Dependencies: "FIRST_NODE=true",
			Required:     false,
			Section:      "🔒 SSL/TLS Configuration",
			Pattern:      `^([a-z0-9]([a-z0-9\-]*[a-z0-9])?(\\.[a-z0-9]([a-z0-9\-]*[a-z0-9])?)*)(\\s*,\\s*[a-z0-9]([a-z0-9\-]*[a-z0-9])?(\\.[a-z0-9]([a-z0-9\-]*[a-z0-9])?)*)*$|^$`,
			PatternTitle: "Enter comma-separated domain names (lowercase alphanumeric with dots/hyphens)",
		},
		{
			Key:          "USE_CERT_MANAGER",
			Type:         "bool",
			Default:      false,
			Description:  "Use cert-manager with Let's Encrypt for automatic TLS certificates.",
			Dependencies: "FIRST_NODE=true",
			Required:     false,
			Section:      "🔒 SSL/TLS Configuration",
		},
		{
			Key:          "CERT_OPTION",
			Type:         "enum",
			Default:      "",
			Description:  "Certificate option when USE_CERT_MANAGER is false. Choose 'existing' or 'generate'.",
			Options:      []string{"existing", "generate"},
			Dependencies: "USE_CERT_MANAGER=false,FIRST_NODE=true",
			Required:     true,
			Section:      "🔒 SSL/TLS Configuration",
		},
		{
			Key:          "TLS_CERT",
			Type:         "string",
			Default:      "",
			Description:  "Path to TLS certificate file for ingress. Required if CERT_OPTION is 'existing'.",
			Dependencies: "CERT_OPTION=existing,FIRST_NODE=true",
			Required:     true,
			Section:      "🔒 SSL/TLS Configuration",
			Pattern:      `^(/[a-zA-Z0-9._\\-]+)+\\.(pem|crt|cert)$|^$`,
			PatternTitle: "Certificate file must be an absolute path ending with .pem, .crt, or .cert",
		},
		{
			Key:          "TLS_KEY",
			Type:         "string",
			Default:      "",
			Description:  "Path to TLS private key file for ingress. Required if CERT_OPTION is 'existing'.",
			Dependencies: "CERT_OPTION=existing,FIRST_NODE=true",
			Required:     true,
			Section:      "🔒 SSL/TLS Configuration",
			Pattern:      `^(/[a-zA-Z0-9._\\-]+)+\\.(pem|key)$|^$`,
			PatternTitle: "Key file must be an absolute path ending with .pem or .key",
		},

		// ========================================
		// ⚙️ Advanced Configuration
		// ========================================
		{
			Key:          "ROCM_BASE_URL",
			Type:         "string",
			Default:      "https://repo.radeon.com/amdgpu-install/7.0.2/ubuntu/",
			Description:  "ROCm base repository URL.",
			Dependencies: "GPU_NODE=true",
			Required:     false,
			Pattern:      `https?://.+`,
			PatternTitle: "Enter a valid URL starting with http:// or https://",
			Section:      "⚙️ Advanced Configuration",
		},
		{
			Key:          "ROCM_DEB_PACKAGE",
			Type:         "string",
			Default:      "amdgpu-install_7.0.2.70002-1_all.deb",
			Description:  "ROCm DEB package name.",
			Dependencies: "GPU_NODE=true",
			Required:     false,
			Section:      "⚙️ Advanced Configuration",
		},
		{
			Key:         "CLUSTERFORGE_RELEASE",
			Type:        "string",
			Default:     "https://github.com/silogen/cluster-forge/releases/download/v1.5.2/release-enterprise-ai-v1.5.2.tar.gz",
			Description: "The version of Cluster-Forge to install. Pass the URL for a specific release, or 'none' to not install ClusterForge.",
			Required:    false,
			Section:     "⚙️ Advanced Configuration",
		},
		{
			Key:         "CF_VALUES",
			Type:        "string",
			Default:     "",
			Description: "Path to ClusterForge values file (e.g., \"values_cf.yaml\"). Optional.",
			Required:    false,
			Section:     "⚙️ Advanced Configuration",
		},
		{
			Key:         "ADDITIONAL_OIDC_PROVIDERS",
			Type:        "array",
			Default:     []any{},
			Description: "Additional OIDC providers for authentication. Each provider needs a URL and audiences.",
			Required:    false,
			Section:     "⚙️ Advanced Configuration",
		},
		{
			Key:         "PRELOAD_IMAGES",
			Type:        "string",
			Default:     "docker.io/rocm/pytorch:rocm6.4_ubuntu24.04_py3.12_pytorch_release_2.6.0,docker.io/rocm/vllm:rocm6.4.1_vllm_0.9.0.1_20250605",
			Description: "Comma-separated list of the container images to preload.",
			Required:    false,
			Section:     "⚙️ Advanced Configuration",
		},
		{
			Key:          "RKE2_INSTALLATION_URL",
			Type:         "string",
			Default:      "https://get.rke2.io",
			Description:  "RKE2 installation script URL.",
			Required:     false,
			Section:      "⚙️ Advanced Configuration",
			Pattern:      `https?://.+`,
			PatternTitle: "Enter a valid URL starting with http:// or https://",
		},
		{
			Key:          "RKE2_VERSION",
			Type:         "string",
			Default:      "v1.34.1+rke2r1",
			Description:  "Specific RKE2 version to install (e.g., \"v1.34.1+rke2r1\").",
			Required:     false,
			Section:      "⚙️ Advanced Configuration",
			Pattern:      `^v[0-9]+\.[0-9]+\.[0-9]+(\+rke2r[0-9]+)?$|^$`,
			PatternTitle: "Version must be in format v1.2.3 or v1.2.3+rke2r1",
		},
		{
			Key:         "RKE2_EXTRA_CONFIG",
			Type:        "string",
			Default:     "",
			Description: "Additional RKE2 configuration in YAML format to append to /etc/rancher/rke2/config.yaml. Example: \"node-name: my-node\\ntls-san:\\n  - example.com\".",
			Required:    false,
			Section:     "⚙️ Advanced Configuration",
		},

		// ========================================
		// 💻 Command Line Options
		// ========================================
		{
			Key:         "DISABLED_STEPS",
			Type:        "string",
			Default:     "",
			Description: "Comma-separated list of step names to skip (e.g., \"SetupLonghornStep,SetupMetallbStep\").",
			Required:    false,
			Section:     "💻 Command Line Options",
		},
		{
			Key:         "ENABLED_STEPS",
			Type:        "string",
			Default:     "",
			Description: "Comma-separated list of steps to run. If empty, run all steps.",
			Required:    false,
			Section:     "💻 Command Line Options",
		},
	}
}
