package config

import (
	"testing"
)

func TestValidate_ValidConfigs(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "valid first node minimal",
			config: Config{
				"FIRST_NODE":            true,
				"GPU_NODE":              false,
				"DOMAIN":                "cluster.example.com",
				"NO_DISKS_FOR_CLUSTER": true,
				"CERT_OPTION":           "generate",
			},
		},
		{
			name: "valid first node with cert-manager",
			config: Config{
				"FIRST_NODE":       true,
				"GPU_NODE":         true,
				"DOMAIN":           "ai.cluster.com",
				"USE_CERT_MANAGER": true,
				"CLUSTER_DISKS":    "/dev/nvme0n1,/dev/nvme1n1",
			},
		},
		{
			name: "valid additional node",
			config: Config{
				"FIRST_NODE":            false,
				"GPU_NODE":              false,
				"SERVER_IP":             "192.168.1.10",
				"JOIN_TOKEN":            "K10token::server:abc123",
				"NO_DISKS_FOR_CLUSTER": true,
			},
		},
		{
			name: "valid with premounted disks",
			config: Config{
				"FIRST_NODE":              true,
				"DOMAIN":                  "test.local",
				"CLUSTER_PREMOUNTED_DISKS": "/mnt/disk1,/mnt/disk2",
				"CERT_OPTION":             "generate",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := Validate(tt.config)
			if len(errors) > 0 {
				t.Errorf("Expected valid config to pass, got errors: %v", errors)
			}
		})
	}
}
