package config

import (
	"strings"
	"testing"
)

func TestValidate_RequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantErrors  []string
		description string
	}{
		{
			name: "DOMAIN required when FIRST_NODE=true",
			config: Config{
				"FIRST_NODE":            true,
				"GPU_NODE":              false,
				"NO_DISKS_FOR_CLUSTER": true,
			},
			wantErrors:  []string{"DOMAIN is required"},
			description: "DOMAIN is required for first node",
		},
		{
			name: "SERVER_IP required when FIRST_NODE=false",
			config: Config{
				"FIRST_NODE":            false,
				"GPU_NODE":              false,
				"NO_DISKS_FOR_CLUSTER": true,
			},
			wantErrors:  []string{"SERVER_IP is required"},
			description: "SERVER_IP is required for additional nodes",
		},
		{
			name: "JOIN_TOKEN required when FIRST_NODE=false",
			config: Config{
				"FIRST_NODE":            false,
				"GPU_NODE":              false,
				"SERVER_IP":             "10.0.0.1",
				"NO_DISKS_FOR_CLUSTER": true,
			},
			wantErrors:  []string{"JOIN_TOKEN is required"},
			description: "JOIN_TOKEN is required for additional nodes",
		},
		{
			name: "CERT_OPTION required when USE_CERT_MANAGER=false and FIRST_NODE=true",
			config: Config{
				"FIRST_NODE":            true,
				"DOMAIN":                "test.example.com",
				"USE_CERT_MANAGER":      false,
				"NO_DISKS_FOR_CLUSTER": true,
			},
			wantErrors:  []string{"CERT_OPTION is required"},
			description: "CERT_OPTION required when not using cert-manager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := Validate(tt.config)

			if len(errors) == 0 && len(tt.wantErrors) > 0 {
				t.Errorf("Expected errors %v, got none", tt.wantErrors)
				return
			}

			for _, wantErr := range tt.wantErrors {
				found := false
				for _, gotErr := range errors {
					if strings.Contains(gotErr, wantErr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing %q, got errors: %v", wantErr, errors)
				}
			}
		})
	}
}

func TestValidate_DomainPattern(t *testing.T) {
	tests := []struct {
		name       string
		domain     string
		shouldFail bool
	}{
		// Valid domains
		{"valid lowercase domain", "example.com", false},
		{"valid subdomain", "cluster.example.com", false},
		{"valid multi-level", "k8s.internal.company.com", false},
		{"valid with hyphens", "ai-cluster.local", false},
		{"valid single char", "a", false},

		// Invalid domains - should fail validation
		{"uppercase domain", "EXAMPLE.COM", true},
		{"uppercase in subdomain", "cluster.EXAMPLE.com", true},
		{"mixed case", "Example.Com", true},
		{"starts with hyphen", "-example.com", true},
		{"ends with hyphen", "example-.com", true},
		{"double dot", "example..com", true},
		{"starts with dot", ".example.com", true},
		{"ends with dot", "example.com.", true},
		{"contains underscore", "example_test.com", true},
		{"contains space", "example com", true},
		{"contains path", "example.com/path", true},
		{"url not domain", "https://example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				"FIRST_NODE":            true,
				"DOMAIN":                tt.domain,
				"NO_DISKS_FOR_CLUSTER": true,
				"CERT_OPTION":           "generate",
			}

			errors := Validate(config)

			if tt.shouldFail {
				if len(errors) == 0 {
					t.Errorf("Expected validation to fail for domain %q, but it passed", tt.domain)
				}
				// Check that error is about domain format
				foundDomainError := false
				for _, err := range errors {
					if strings.Contains(err, "domain") || strings.Contains(err, "DOMAIN") {
						foundDomainError = true
						break
					}
				}
				if !foundDomainError {
					t.Errorf("Expected domain validation error for %q, got: %v", tt.domain, errors)
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected validation to pass for domain %q, got errors: %v", tt.domain, errors)
				}
			}
		})
	}
}

func TestValidate_IPAddress(t *testing.T) {
	tests := []struct {
		name       string
		ip         string
		shouldFail bool
	}{
		// Valid IPs
		{"valid private IP", "10.100.100.11", false},
		{"valid private 192", "192.168.1.100", false},
		{"valid private 172", "172.16.0.1", false},

		// Invalid IPs - should fail
		{"out of range first octet", "256.0.0.1", true},
		{"out of range second octet", "1.256.0.1", true},
		{"out of range third octet", "1.2.256.1", true},
		{"out of range fourth octet", "1.2.3.256", true},
		{"incomplete IP", "192.168.1", true},
		{"too many octets", "1.2.3.4.5", true},
		{"non-numeric", "a.b.c.d", true},
		{"with CIDR", "192.168.1.1/24", true},
		{"domain not IP", "example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				"FIRST_NODE":            false,
				"SERVER_IP":             tt.ip,
				"JOIN_TOKEN":            "K10abc::server:token",
				"NO_DISKS_FOR_CLUSTER": true,
			}

			errors := Validate(config)

			if tt.shouldFail {
				if len(errors) == 0 {
					t.Errorf("Expected validation to fail for IP %q, but it passed", tt.ip)
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected validation to pass for IP %q, got errors: %v", tt.ip, errors)
				}
			}
		})
	}
}

func TestValidate_Constraints(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		shouldFail  bool
		errorContains string
	}{
		{
			name: "valid - NO_DISKS_FOR_CLUSTER only",
			config: Config{
				"FIRST_NODE":            true,
				"DOMAIN":                "test.example.com",
				"NO_DISKS_FOR_CLUSTER": true,
				"CERT_OPTION":           "generate",
			},
			shouldFail: false,
		},
		{
			name: "valid - CLUSTER_DISKS only",
			config: Config{
				"FIRST_NODE":   true,
				"DOMAIN":       "test.example.com",
				"CLUSTER_DISKS": "/dev/nvme0n1",
				"CERT_OPTION":  "generate",
			},
			shouldFail: false,
		},
		{
			name: "invalid - no storage option",
			config: Config{
				"FIRST_NODE":  true,
				"DOMAIN":      "test.example.com",
				"CERT_OPTION": "generate",
			},
			shouldFail: true,
			errorContains: "storage",
		},
		{
			name: "invalid - multiple storage options",
			config: Config{
				"FIRST_NODE":            true,
				"DOMAIN":                "test.example.com",
				"NO_DISKS_FOR_CLUSTER": true,
				"CLUSTER_DISKS":         "/dev/nvme0n1",
				"CERT_OPTION":           "generate",
			},
			shouldFail: true,
		},
		{
			name: "invalid - mutually exclusive DISABLED_STEPS and ENABLED_STEPS",
			config: Config{
				"FIRST_NODE":            true,
				"DOMAIN":                "test.example.com",
				"NO_DISKS_FOR_CLUSTER": true,
				"CERT_OPTION":           "generate",
				"DISABLED_STEPS":        "step1",
				"ENABLED_STEPS":         "step2",
			},
			shouldFail: true,
			errorContains: "mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := Validate(tt.config)

			if tt.shouldFail {
				if len(errors) == 0 {
					t.Errorf("Expected validation to fail, but it passed")
				}
				if tt.errorContains != "" {
					found := false
					for _, err := range errors {
						if strings.Contains(strings.ToLower(err), strings.ToLower(tt.errorContains)) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error containing %q, got: %v", tt.errorContains, errors)
					}
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected validation to pass, got errors: %v", errors)
				}
			}
		})
	}
}

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

func TestValidate_EnumValidation(t *testing.T) {
	tests := []struct {
		name       string
		certOption string
		shouldFail bool
	}{
		{"valid existing", "existing", false},
		{"valid generate", "generate", false},
		{"invalid option", "invalid", true},
		{"invalid uppercase", "EXISTING", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				"FIRST_NODE":            true,
				"DOMAIN":                "test.example.com",
				"USE_CERT_MANAGER":      false,
				"CERT_OPTION":           tt.certOption,
				"NO_DISKS_FOR_CLUSTER": true,
			}

			// Add required cert files when CERT_OPTION=existing
			if tt.certOption == "existing" {
				config["TLS_CERT"] = "/etc/ssl/certs/cert.pem"
				config["TLS_KEY"] = "/etc/ssl/private/key.pem"
			}

			errors := Validate(config)

			if tt.shouldFail {
				if len(errors) == 0 {
					t.Errorf("Expected validation to fail for CERT_OPTION=%q, but it passed", tt.certOption)
				}
			} else {
				if len(errors) > 0 {
					t.Errorf("Expected validation to pass for CERT_OPTION=%q, got errors: %v", tt.certOption, errors)
				}
			}
		})
	}
}
