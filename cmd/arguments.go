package cmd

import (
	"github.com/silogen/cluster-bloom/pkg/args"
)

func SetArguments() {
	args.SetArguments([]args.Arg{
		// Core cluster configuration
		{
			Key:         "FIRST_NODE",
			Default:     true,
			Description: "Set to true if this is the first node in the cluster.",
			Type:        "bool",
		},
		{
			Key:         "GPU_NODE",
			Default:     true,
			Description: "Set to true if this node has GPUs.",
			Type:        "bool",
		},
		{
			Key:          "CONTROL_PLANE",
			Default:      false,
			Description:  "Set to true if this node should be a control plane node (only applies when FIRST_NODE is false).",
			Type:         "bool",
			Dependencies: "FIRST_NODE=false",
		},
		{
			Key:          "SERVER_IP",
			Default:      "",
			Description:  "IP address of the RKE2 server. Required for non-first nodes.",
			Type:         "non-empty-ip-address",
			Dependencies: "FIRST_NODE=false",
		},
		{
			Key:          "JOIN_TOKEN",
			Default:      "",
			Description:  "Token for joining additional nodes to the cluster. Required for non-first nodes.",
			Type:         "non-empty-string",
			Dependencies: "FIRST_NODE=false",
			Validators:   []func(value string) error{args.ValidateJoinTokenArg},
		},

		// Network and domain configuration
		{
			Key:          "DOMAIN",
			Default:      "",
			Description:  "The domain name for the cluster (e.g., \"cluster.example.com\"). Required.",
			Type:         "non-empty-string",
			Dependencies: "FIRST_NODE=true",
		},
		{
			Key:          "NODE_IP",
			Default:      "",
			Description:  "The IP address to advertise for this node. Optional.",
			Type:         "ip-address",
			Dependencies: "FIRST_NODE=false",
		},
		{
			Key:          "NODE_EXTERNAL_IP",
			Default:      "",
			Description:  "The external IP address to advertise for this node. Optional.",
			Type:         "ip-address",
			Dependencies: "FIRST_NODE=false",
		},
		{
			Key:          "ADVERTISE_ADDRESS",
			Default:      "",
			Description:  "The IP address the  apiserver uses to advertise to members of the cluster. Optional.",
			Type:         "ip-address",
			Dependencies: "FIRST_NODE=false",
		},
		{
			Key:         "CF_VALUES",
			Default:     "",
			Description: "Path to ClusterForge values file (e.g., \"values_cf.yaml\"). Optional.",
			Type:        "string",
		},
		// TLS/Certificate configuration
		{
			Key:          "USE_CERT_MANAGER",
			Default:      false,
			Description:  "Use cert-manager with Let's Encrypt for automatic TLS certificates.",
			Type:         "bool",
			Dependencies: "FIRST_NODE=true",
		},
		{
			Key:          "CERT_OPTION",
			Default:      "",
			Description:  "Certificate option when USE_CERT_MANAGER is false. Choose 'existing' or 'generate'.",
			Type:         "enum",
			Options:      []string{"existing", "generate"},
			Dependencies: "USE_CERT_MANAGER=false,FIRST_NODE=true",
		},
		{
			Key:          "TLS_CERT",
			Default:      "",
			Description:  "Path to TLS certificate file for ingress. Required if CERT_OPTION is 'existing'.",
			Type:         "file",
			Dependencies: "CERT_OPTION=existing",
		},
		{
			Key:          "TLS_KEY",
			Default:      "",
			Description:  "Path to TLS private key file for ingress. Required if CERT_OPTION is 'existing'.",
			Type:         "file",
			Dependencies: "CERT_OPTION=existing",
		},

		// Authentication
		{
			Key:         "ADDITIONAL_OIDC_PROVIDERS",
			Default:     []interface{}{},
			Description: "Additional OIDC providers for authentication. Each provider needs a URL and audiences. Example: [{\"url\": \"https://provider.com/realms/k8s\", \"audiences\": [\"k8s\"]}]",
			Type:        "array",
		},
		{
			Key:         "ADDITIONAL_TLS_SAN_URLS",
			Default:     []string{},
			Description: "Additional TLS Subject Alternative Name URLs for Kubernetes API server certificate. Example: [\"api.example.com\", \"kubernetes.example.com\"]",
			Type:        "string-array",
		},

		// ROCm configuration (depends on GPU_NODE)
		{
			Key:          "ROCM_BASE_URL",
			Default:      "https://repo.radeon.com/amdgpu-install/7.0.2/ubuntu/",
			Description:  "ROCm base repository URL.",
			Type:         "non-empty-url",
			Dependencies: "GPU_NODE=true",
		},
		{
			Key:          "ROCM_DEB_PACKAGE",
			Default:      "amdgpu-install_7.0.2.70002-1_all.deb",
			Description:  "ROCm DEB package name.",
			Type:         "non-empty-string",
			Dependencies: "GPU_NODE=true",
		},

		// Disk and storage configuration
		{
			Key:         "NO_DISKS_FOR_CLUSTER",
			Default:     false,
			Description: "Set to true to skip disk-related operations.",
			Type:        "bool",
			Validators:  []func(value string) error{args.ValidateSkipDiskCheckConsistency},
		},
		{
			Key:         "SKIP_RANCHER_PARTITION_CHECK",
			Default:     false,
			Description: "Set to true to skip /var/lib/rancher partition size check.",
			Type:        "bool",
		},
		{
			Key:          "CLUSTER_PREMOUNTED_DISKS",
			Default:      "",
			Description:  "Comma-separated list of disk paths to use for Longhorn.",
			Type:         "string",
			Validators:   []func(value string) error{args.ValidateLonghornDisksArg},
			Dependencies: "NO_DISKS_FOR_CLUSTER=false",
		},
		{
			Key:          "CLUSTER_DISKS",
			Default:      "",
			Description:  "Comma-separated list of disk devices. Example: \"/dev/sdb,/dev/sdc\".",
			Type:         "string",
			Dependencies: "NO_DISKS_FOR_CLUSTER=false",
		},

		// External component URLs
		{
			Key:         "RKE2_INSTALLATION_URL",
			Default:     "https://get.rke2.io",
			Description: "RKE2 installation script URL.",
			Type:        "non-empty-url",
		},
		{
			Key:         "RKE2_VERSION",
			Default:     "v1.34.1+rke2r1",
			Description: "Specific RKE2 version to install (e.g., \"v1.34.1+rke2r1\").",
			Type:        "string",
		},
		{
			Key:         "RKE2_EXTRA_CONFIG",
			Default:     "",
			Description: "Additional RKE2 configuration in YAML format to append to /etc/rancher/rke2/config.yaml. Example: \"node-name: my-node\\ntls-san:\\n  - example.com\".",
			Type:        "string",
			Validators:  []func(value string) error{args.ValidateYAMLFormat},
		},
		{
			Key:         "CLUSTERFORGE_RELEASE",
			Default:     "https://github.com/silogen/cluster-forge/releases/download/v1.7.0/release-enterprise-ai-v1.7.0.tar.gz",
			Description: "The version of Cluster-Forge to install. Pass the URL for a specific release, or 'none' to not install ClusterForge.",
			Type:        "url",
		},
		{
			Key:         "PRELOAD_IMAGES",
			Default:     "",
			Description: "Comma-separated list of the container images to preload.",
			Type:        "string",
		},

		// Step control
		{
			Key:         "DISABLED_STEPS",
			Default:     "",
			Description: "Comma-separated list of steps to skip. Example: \"SetupLonghornStep,SetupMetallbStep\".",
			Type:        "string",
			Validators:  []func(value string) error{args.ValidateStepNamesArg, args.ValidateDisabledStepsWarnings, args.ValidateDisabledStepsConflict},
		},
		{
			Key:         "ENABLED_STEPS",
			Default:     "",
			Description: "Comma-separated list of steps to perform. If empty, perform all. Example: \"SetupLonghornStep,SetupMetallbStep\".",
			Type:        "string",
			Validators:  []func(value string) error{args.ValidateStepNamesArg},
		},
	})
}
