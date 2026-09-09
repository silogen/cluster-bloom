package config

import (
	"strings"
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
				"FIRST_NODE":           true,
				"GPU_NODE":             false,
				"DOMAIN":               "cluster.example.com",
				"CLUSTER_SIZE":         "small",
				"NO_DISKS_FOR_CLUSTER": true,
				"CERT_OPTION":          "generate",
			},
		},
		{
			name: "valid first node with cert-manager",
			config: Config{
				"FIRST_NODE":       true,
				"GPU_NODE":         true,
				"DOMAIN":           "ai.cluster.com",
				"CLUSTER_SIZE":     "medium",
				"USE_CERT_MANAGER": true,
				"CLUSTER_DISKS":    "/dev/nvme0n1,/dev/nvme1n1",
			},
		},
		{
			name: "valid additional node",
			config: Config{
				"FIRST_NODE":           false,
				"GPU_NODE":             false,
				"SERVER_IP":            "192.168.1.10",
				"JOIN_TOKEN":           "K10token::server:abc123",
				"NO_DISKS_FOR_CLUSTER": true,
			},
		},
		{
			name: "valid with premounted disks",
			config: Config{
				"FIRST_NODE":               true,
				"DOMAIN":                   "test.local",
				"CLUSTER_SIZE":             "large",
				"CLUSTER_PREMOUNTED_DISKS": "/mnt/disk1,/mnt/disk2",
				"CERT_OPTION":              "generate",
			},
		},
		{
			name: "valid large cluster with regular disks",
			config: Config{
				"FIRST_NODE":    true,
				"DOMAIN":        "large.cluster.local",
				"CLUSTER_SIZE":  "large",
				"CLUSTER_DISKS": "/dev/nvme0n1,/dev/nvme1n1",
				"CERT_OPTION":   "generate",
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

func TestValidateRejectsStringForBooleanField(t *testing.T) {
	errors := Validate(Config{
		"FIRST_NODE":           true,
		"GPU_NODE":             "false",
		"DOMAIN":               "cluster.example.com",
		"CLUSTER_SIZE":         "small",
		"NO_DISKS_FOR_CLUSTER": true,
		"CERT_OPTION":          "generate",
	})
	if len(errors) == 0 {
		t.Fatal("expected quoted boolean value to be rejected")
	}
}

// CILIUM_HELM_VALUES is `type: map`, which had no case in the validator's type
// switch until this key existed. Without one it fell through to the regex
// branch, found no regex named "map", and got zero validation - a scalar or a
// list would have sailed through to Ansible and blown up mid-deploy.
func TestValidateCiliumHelmValues(t *testing.T) {
	base := func(value any) Config {
		cfg := Config{
			"FIRST_NODE":           true,
			"GPU_NODE":             false,
			"DOMAIN":               "cluster.example.com",
			"CLUSTER_SIZE":         "small",
			"NO_DISKS_FOR_CLUSTER": true,
			"CERT_OPTION":          "generate",
		}
		if value != nil {
			cfg["CILIUM_HELM_VALUES"] = value
		}
		return cfg
	}

	valid := map[string]any{
		"absent":       nil,
		"empty map":    map[string]any{},
		"empty string": "",
		"nested map": map[string]any{
			"hubble": map[string]any{"enabled": true, "relay": map[string]any{"enabled": true}},
		},
		// What the web UI textarea and environment overrides send: neither can
		// express a nested map natively.
		"yaml string": "hubble:\n  enabled: true\n  relay:\n    enabled: true\n",
	}
	for name, value := range valid {
		t.Run("valid/"+name, func(t *testing.T) {
			if errors := Validate(base(value)); len(errors) > 0 {
				t.Errorf("expected %#v to be accepted, got errors: %v", value, errors)
			}
		})
	}

	invalid := map[string]any{
		"scalar":                  3,
		"bare string":             "hubble",
		"list":                    []any{"hubble"},
		"yaml string of a list":   "- hubble\n- operator\n",
		"yaml string of a scalar": "3",
		"malformed yaml":          "hubble:\n  enabled: true\n   bad-indent: 1\n",
	}
	for name, value := range invalid {
		t.Run("invalid/"+name, func(t *testing.T) {
			errors := Validate(base(value))
			if len(errors) == 0 {
				t.Fatalf("expected %#v to be rejected", value)
			}
			// A message that does not name the key leaves an operator staring
			// at a 40-key bloom.yaml with no idea which line is wrong.
			for _, e := range errors {
				if strings.Contains(e, "CILIUM_HELM_VALUES") {
					return
				}
			}
			t.Errorf("no error names CILIUM_HELM_VALUES: %v", errors)
		})
	}
}

func TestValidateRejectsStringForSequenceField(t *testing.T) {
	errors := Validate(Config{
		"FIRST_NODE":           true,
		"GPU_NODE":             false,
		"DOMAIN":               "cluster.example.com",
		"CLUSTER_SIZE":         "small",
		"NO_DISKS_FOR_CLUSTER": true,
		"CERT_OPTION":          "generate",
		"DNS_SERVERS":          "8.8.8.8,1.1.1.1",
	})
	if len(errors) == 0 {
		t.Fatal("expected string sequence value to be rejected")
	}
}
