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
			Dependencies: []args.UsedWhen{{Arg: "FIRST_NODE", Type: "equals_false"}},
		},
		{
			Key:          "SERVER_IP",
			Default:      "",
			Description:  "IP address of the RKE2 server. Required for non-first nodes.",
			Type:         "non-empty-ip-address",
			Dependencies: []args.UsedWhen{{Arg: "FIRST_NODE", Type: "equals_false"}},
		},
		{
			Key:          "JOIN_TOKEN",
			Default:      "",
			Description:  "Token for joining additional nodes to the cluster. Required for non-first nodes.",
			Type:         "non-empty-string",
			Dependencies: []args.UsedWhen{{Arg: "FIRST_NODE", Type: "equals_false"}},
			Validators:   []func(value string) error{args.ValidateJoinTokenArg},
		},

		// Network and domain configuration
		{
			Key:          "DOMAIN",
			Default:      "",
			Description:  "The domain name for the cluster (e.g., \"cluster.example.com\"). Required.",
			Type:         "non-empty-string",
			Dependencies: []args.UsedWhen{{Arg: "FIRST_NODE", Type: "equals_true"}},
		},

		// TLS/Certificate configuration
		{
			Key:          "USE_CERT_MANAGER",
			Default:      false,
			Description:  "Use cert-manager with Let's Encrypt for automatic TLS certificates.",
			Type:         "bool",
			Dependencies: []args.UsedWhen{{Arg: "FIRST_NODE", Type: "equals_true"}},
		},
		{
			Key:          "CERT_OPTION",
			Default:      "",
			Description:  "Certificate option when USE_CERT_MANAGER is false. Choose 'existing' or 'generate'.",
			Type:         "enum",
			Options:      []string{"existing", "generate"},
			Dependencies: []args.UsedWhen{{Arg: "USE_CERT_MANAGER", Type: "equals_false"}, {Arg: "FIRST_NODE", Type: "equals_true"}},
		},
		{
			Key:          "TLS_CERT",
			Default:      "",
			Description:  "Path to TLS certificate file for ingress. Required if CERT_OPTION is 'existing'.",
			Type:         "file",
			Dependencies: []args.UsedWhen{{Arg: "CERT_OPTION", Type: "equals_existing"}},
		},
		{
			Key:          "TLS_KEY",
			Default:      "",
			Description:  "Path to TLS private key file for ingress. Required if CERT_OPTION is 'existing'.",
			Type:         "file",
			Dependencies: []args.UsedWhen{{Arg: "CERT_OPTION", Type: "equals_existing"}},
		},

		// Authentication
		{
			Key:         "OIDC_URL",
			Default:     "",
			Description: "The URL of the OIDC provider.",
			Type:        "url",
		},

		// ROCm configuration (depends on GPU_NODE)
		{
			Key:          "ROCM_BASE_URL",
			Default:      "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/",
			Description:  "ROCm base repository URL.",
			Type:         "non-empty-url",
			Dependencies: []args.UsedWhen{{Arg: "GPU_NODE", Type: "equals_true"}},
		},
		{
			Key:          "ROCM_DEB_PACKAGE",
			Default:      "amdgpu-install_6.3.60302-1_all.deb",
			Description:  "ROCm DEB package name.",
			Type:         "non-empty-string",
			Dependencies: []args.UsedWhen{{Arg: "GPU_NODE", Type: "equals_true"}},
		},

		// Disk and storage configuration
		{
			Key:         "SKIP_DISK_CHECK",
			Default:     false,
			Description: "Set to true to skip disk-related operations.",
			Type:        "bool",
			Validators:  []func(value string) error{args.ValidateSkipDiskCheckConsistency},
		},
		{
			Key:         "LONGHORN_DISKS",
			Default:     "",
			Description: "Comma-separated list of disk paths to use for Longhorn.",
			Type:        "string",
			Validators:  []func(value string) error{args.ValidateLonghornDisksArg},
		},
		{
			Key:         "SELECTED_DISKS",
			Default:     "",
			Description: "Comma-separated list of disk devices. Example: \"/dev/sdb,/dev/sdc\".",
			Type:        "string",
		},

		// External component URLs
		{
			Key:         "RKE2_INSTALLATION_URL",
			Default:     "https://get.rke2.io",
			Description: "RKE2 installation script URL.",
			Type:        "non-empty-url",
		},
		{
			Key:         "CLUSTERFORGE_RELEASE",
			Default:     "https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz",
			Description: "The version of Cluster-Forge to install. Pass the URL for a specific release, or 'none' to not install ClusterForge.",
			Type:        "url",
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
