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
}

// Schema returns all bloom.yaml argument definitions
func Schema() []Argument {
	return []Argument{
		// Node Configuration
		{
			Key:         "FIRST_NODE",
			Type:        "bool",
			Default:     true,
			Description: "Set to true if this is the first node in the cluster.",
			Required:    false,
		},
		{
			Key:         "GPU_NODE",
			Type:        "bool",
			Default:     true,
			Description: "Set to true if this node has GPUs.",
			Required:    false,
		},
		{
			Key:          "CONTROL_PLANE",
			Type:         "bool",
			Default:      false,
			Description:  "Set to true if this node should be a control plane node (only applies when FIRST_NODE is false).",
			Dependencies: "FIRST_NODE=false",
			Required:     false,
		},

		// Cluster Join (for additional nodes)
		{
			Key:          "SERVER_IP",
			Type:         "string",
			Default:      "",
			Description:  "IP address of the RKE2 server. Required for non-first nodes.",
			Dependencies: "FIRST_NODE=false",
			Required:     true,
		},
		{
			Key:          "JOIN_TOKEN",
			Type:         "string",
			Default:      "",
			Description:  "Token for joining additional nodes to the cluster. Required for non-first nodes.",
			Dependencies: "FIRST_NODE=false",
			Required:     true,
		},

		// Domain & Networking
		{
			Key:          "DOMAIN",
			Type:         "string",
			Default:      "",
			Description:  "The domain name for the cluster (e.g., \"cluster.example.com\"). Required for first node.",
			Dependencies: "FIRST_NODE=true",
			Required:     true,
		},

		// Certificates
		{
			Key:          "USE_CERT_MANAGER",
			Type:         "bool",
			Default:      false,
			Description:  "Use cert-manager with Let's Encrypt for automatic TLS certificates.",
			Dependencies: "FIRST_NODE=true",
			Required:     false,
		},
		{
			Key:          "CERT_OPTION",
			Type:         "enum",
			Default:      "",
			Description:  "Certificate option when USE_CERT_MANAGER is false. Choose 'existing' or 'generate'.",
			Options:      []string{"existing", "generate"},
			Dependencies: "USE_CERT_MANAGER=false,FIRST_NODE=true",
			Required:     true,
		},
		{
			Key:          "TLS_CERT",
			Type:         "string",
			Default:      "",
			Description:  "Path to TLS certificate file for ingress. Required if CERT_OPTION is 'existing'.",
			Dependencies: "CERT_OPTION=existing,FIRST_NODE=true",
			Required:     true,
		},
		{
			Key:          "TLS_KEY",
			Type:         "string",
			Default:      "",
			Description:  "Path to TLS private key file for ingress. Required if CERT_OPTION is 'existing'.",
			Dependencies: "CERT_OPTION=existing,FIRST_NODE=true",
			Required:     true,
		},

		// GPU/ROCm
		{
			Key:          "ROCM_BASE_URL",
			Type:         "string",
			Default:      "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/",
			Description:  "ROCm base repository URL.",
			Dependencies: "GPU_NODE=true",
			Required:     false,
		},

		// Storage
		{
			Key:         "CLUSTER_DISKS",
			Type:        "string",
			Default:     "",
			Description: "Comma-separated list of disk device paths (e.g., \"/dev/nvme0n1,/dev/nvme1n1\").",
			Required:    false,
		},
		{
			Key:         "CLUSTER_PREMOUNTED_DISKS",
			Type:        "string",
			Default:     "",
			Description: "Comma-separated list of premounted disk paths for Longhorn.",
			Required:    false,
		},
		{
			Key:         "NO_DISKS_FOR_CLUSTER",
			Type:        "bool",
			Default:     false,
			Description: "Set to true to skip all disk-related operations.",
			Required:    false,
		},
		{
			Key:         "SKIP_RANCHER_PARTITION_CHECK",
			Type:        "bool",
			Default:     false,
			Description: "Set to true to skip /var/lib/rancher partition size check.",
			Required:    false,
		},

		// ClusterForge
		{
			Key:         "CLUSTERFORGE_RELEASE",
			Type:        "string",
			Default:     "https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz",
			Description: "The version of Cluster-Forge to install. Pass the URL for a specific release, or 'none' to not install ClusterForge.",
			Required:    false,
		},
		{
			Key:         "CF_VALUES",
			Type:        "string",
			Default:     "",
			Description: "Path to ClusterForge values file (e.g., \"values_cf.yaml\"). Optional.",
			Required:    false,
		},

		// Step Control
		{
			Key:         "DISABLED_STEPS",
			Type:        "string",
			Default:     "",
			Description: "Comma-separated list of step names to skip (e.g., \"SetupLonghornStep,SetupMetallbStep\").",
			Required:    false,
		},
		{
			Key:         "ENABLED_STEPS",
			Type:        "string",
			Default:     "",
			Description: "Comma-separated list of steps to run. If empty, run all steps.",
			Required:    false,
		},

		// OIDC Authentication
		{
			Key:          "OIDC_ISSUER_URL",
			Type:         "string",
			Default:      "",
			Description:  "OIDC issuer URL for authentication (e.g., \"https://accounts.google.com\").",
			Dependencies: "FIRST_NODE=true",
			Required:     false,
		},
		{
			Key:          "OIDC_ADMIN_EMAIL",
			Type:         "string",
			Default:      "",
			Description:  "Email address of the admin user for OIDC authentication.",
			Dependencies: "FIRST_NODE=true",
			Required:     false,
		},
		{
			Key:         "ADDITIONAL_OIDC_PROVIDERS",
			Type:        "array",
			Default:     []any{},
			Description: "Additional OIDC providers for authentication. Each provider needs a URL and audiences.",
			Required:    false,
		},

		// Misc
		{
			Key:         "PRELOAD_IMAGES",
			Type:        "string",
			Default:     "",
			Description: "Container images to preload.",
			Required:    false,
		},
	}
}
