package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func setupViperForTest() {
	viper.Reset()
	// Set some reasonable defaults for testing
	viper.Set("FIRST_NODE", true)
	viper.Set("GPU_NODE", true)
	viper.Set("SKIP_DISK_CHECK", false)
	viper.Set("USE_CERT_MANAGER", false)
	viper.Set("CLUSTERFORGE_RELEASE", "https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz")
	viper.Set("ROCM_BASE_URL", "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/")
	viper.Set("ROCM_DEB_PACKAGE", "amdgpu-install_6.3.60302-1_all.deb")
	viper.Set("RKE2_INSTALLATION_URL", "https://get.rke2.io")
}

func TestArgs_EvaluateDependency(t *testing.T) {
	setupViperForTest()

	tests := []struct {
		name       string
		dependency UsedWhen
		viperSetup func()
		expected   bool
	}{
		{
			name:       "equals_true with true value",
			dependency: UsedWhen{Arg: "FIRST_NODE", Type: "equals_true"},
			viperSetup: func() { viper.Set("FIRST_NODE", true) },
			expected:   true,
		},
		{
			name:       "equals_true with false value",
			dependency: UsedWhen{Arg: "FIRST_NODE", Type: "equals_true"},
			viperSetup: func() { viper.Set("FIRST_NODE", false) },
			expected:   false,
		},
		{
			name:       "equals_false with false value",
			dependency: UsedWhen{Arg: "FIRST_NODE", Type: "equals_false"},
			viperSetup: func() { viper.Set("FIRST_NODE", false) },
			expected:   true,
		},
		{
			name:       "equals_false with true value",
			dependency: UsedWhen{Arg: "FIRST_NODE", Type: "equals_false"},
			viperSetup: func() { viper.Set("FIRST_NODE", true) },
			expected:   false,
		},
		{
			name:       "equals_existing with existing value",
			dependency: UsedWhen{Arg: "CERT_OPTION", Type: "equals_existing"},
			viperSetup: func() { viper.Set("CERT_OPTION", "existing") },
			expected:   true,
		},
		{
			name:       "equals_existing with generate value",
			dependency: UsedWhen{Arg: "CERT_OPTION", Type: "equals_existing"},
			viperSetup: func() { viper.Set("CERT_OPTION", "generate") },
			expected:   false,
		},
		{
			name:       "equals_generate with generate value",
			dependency: UsedWhen{Arg: "CERT_OPTION", Type: "equals_generate"},
			viperSetup: func() { viper.Set("CERT_OPTION", "generate") },
			expected:   true,
		},
		{
			name:       "unknown dependency type",
			dependency: UsedWhen{Arg: "FIRST_NODE", Type: "unknown_type"},
			viperSetup: func() { viper.Set("FIRST_NODE", true) },
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupViperForTest()
			tt.viperSetup()
			result := evaluateDependency(tt.dependency)
			if result != tt.expected {
				t.Errorf("evaluateDependency() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestArgs_IsArgRequired(t *testing.T) {
	setupViperForTest()

	tests := []struct {
		name       string
		arg        Arg
		viperSetup func()
		expected   bool
	}{
		{
			name: "no dependencies",
			arg: Arg{
				Key:          "GPU_NODE",
				Dependencies: nil,
			},
			viperSetup: func() {},
			expected:   true,
		},
		{
			name: "single dependency satisfied",
			arg: Arg{
				Key:          "DOMAIN",
				Dependencies: []UsedWhen{{"FIRST_NODE", "equals_true"}},
			},
			viperSetup: func() { viper.Set("FIRST_NODE", true) },
			expected:   true,
		},
		{
			name: "single dependency not satisfied",
			arg: Arg{
				Key:          "DOMAIN",
				Dependencies: []UsedWhen{{"FIRST_NODE", "equals_true"}},
			},
			viperSetup: func() { viper.Set("FIRST_NODE", false) },
			expected:   false,
		},
		{
			name: "multiple dependencies all satisfied",
			arg: Arg{
				Key: "TLS_CERT",
				Dependencies: []UsedWhen{
					{"CERT_OPTION", "equals_existing"},
					UsedWhen{"USE_CERT_MANAGER", "equals_false"},
				},
			},
			viperSetup: func() {
				viper.Set("CERT_OPTION", "existing")
				viper.Set("USE_CERT_MANAGER", false)
			},
			expected: true,
		},
		{
			name: "multiple dependencies partially satisfied",
			arg: Arg{
				Key: "TLS_CERT",
				Dependencies: []UsedWhen{
					{"CERT_OPTION", "equals_existing"},
					UsedWhen{"USE_CERT_MANAGER", "equals_false"},
				},
			},
			viperSetup: func() {
				viper.Set("CERT_OPTION", "existing")
				viper.Set("USE_CERT_MANAGER", true)
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupViperForTest()
			tt.viperSetup()
			result := IsArgUsed(tt.arg)
			if result != tt.expected {
				t.Errorf("isArgRequired() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestArgs_ValidateArgs_TypeValidation(t *testing.T) {
	setupViperForTest()

	// Create a temporary file for file validation tests
	tmpFile, err := os.CreateTemp("", "test_cert_*.pem")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tests := []struct {
		name        string
		viperSetup  func()
		expectError bool
		errorPart   string
	}{
		{
			name: "valid URL",
			viperSetup: func() {
				viper.Set("OIDC_URL", "https://auth.example.com")
				// Set up minimal valid configuration to avoid other validation errors
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", true)
			},
			expectError: false,
		},
		{
			name: "invalid URL",
			viperSetup: func() {
				viper.Set("OIDC_URL", "not-a-url")
				// Set up minimal valid configuration to avoid other validation errors
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", true)
			},
			expectError: true,
			errorPart:   "invalid URL format: missing scheme or host",
		},
		{
			name: "valid file path",
			viperSetup: func() {
				viper.Set("TLS_CERT", tmpFile.Name())
				viper.Set("TLS_KEY", tmpFile.Name())
				// Set up required configuration for TLS_CERT to be validated
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", false)
				viper.Set("CERT_OPTION", "existing")
			},
			expectError: false,
		},
		{
			name: "non-absolute file path",
			viperSetup: func() {
				viper.Set("TLS_CERT", "relative/path.pem")
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", false)
				viper.Set("CERT_OPTION", "existing")
			},
			expectError: true,
			errorPart:   "must be an absolute file path",
		},
		{
			name: "non-existent file",
			viperSetup: func() {
				viper.Set("TLS_CERT", "/nonexistent/file.pem")
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", false)
				viper.Set("CERT_OPTION", "existing")
			},
			expectError: true,
			errorPart:   "file does not exist",
		},
		{
			name: "valid enum value",
			viperSetup: func() {
				viper.Set("FIRST_NODE", true)
				viper.Set("USE_CERT_MANAGER", false)
				viper.Set("CERT_OPTION", "existing")
				viper.Set("DOMAIN", "example.com")
				// Since CERT_OPTION is 'existing', we need TLS_CERT and TLS_KEY
				viper.Set("TLS_CERT", tmpFile.Name())
				viper.Set("TLS_KEY", tmpFile.Name())
			},
			expectError: false,
		},
		{
			name: "invalid enum value",
			viperSetup: func() {
				viper.Set("FIRST_NODE", true)
				viper.Set("USE_CERT_MANAGER", false)
				viper.Set("CERT_OPTION", "invalid_option")
				viper.Set("DOMAIN", "example.com")
			},
			expectError: true,
			errorPart:   "must be one of",
		},
		{
			name: "non-empty URL validation",
			viperSetup: func() {
				// Test that non-empty prefix works with URL type
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", true)
				viper.Set("OIDC_URL", "not-a-url")
			},
			expectError: true,
			errorPart:   "invalid URL format: missing scheme or host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupViperForTest()
			tt.viperSetup()
			err := ValidateArgs()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorPart) {
					t.Errorf("Expected error containing '%s' but got: %v", tt.errorPart, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestArgs_ValidateArgs_RequiredFields(t *testing.T) {
	setupViperForTest()

	tests := []struct {
		name        string
		viperSetup  func()
		expectError bool
		errorPart   string
	}{
		{
			name: "first node with domain - valid",
			viperSetup: func() {
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", true)
			},
			expectError: false,
		},
		{
			name: "first node without domain - invalid",
			viperSetup: func() {
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "")
				viper.Set("USE_CERT_MANAGER", true)
			},
			expectError: true,
			errorPart:   "DOMAIN is required",
		},
		{
			name: "additional node with tokens - valid",
			viperSetup: func() {
				viper.Set("FIRST_NODE", false)
				viper.Set("SERVER_IP", "192.168.1.100")
				viper.Set("JOIN_TOKEN", "K10abcdef1234567890abcdef1234567890abcdef123456789")
			},
			expectError: false,
		},
		{
			name: "additional node without server IP - invalid",
			viperSetup: func() {
				viper.Set("FIRST_NODE", false)
				viper.Set("SERVER_IP", "")
				viper.Set("JOIN_TOKEN", "K10abcdef1234567890abcdef1234567890abcdef123456789")
			},
			expectError: true,
			errorPart:   "SERVER_IP is required",
		},
		{
			name: "additional node without join token - invalid",
			viperSetup: func() {
				viper.Set("FIRST_NODE", false)
				viper.Set("SERVER_IP", "192.168.1.100")
				viper.Set("JOIN_TOKEN", "")
			},
			expectError: true,
			errorPart:   "JOIN_TOKEN is required",
		},
		{
			name: "cert manager disabled with cert option - valid",
			viperSetup: func() {
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", false)
				viper.Set("CERT_OPTION", "generate")
			},
			expectError: false,
		},
		{
			name: "cert manager disabled without cert option - invalid",
			viperSetup: func() {
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", false)
				viper.Set("CERT_OPTION", "")
			},
			expectError: true,
			errorPart:   "must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupViperForTest()
			tt.viperSetup()
			err := ValidateArgs()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorPart) {
					t.Errorf("Expected error containing '%s' but got: %v", tt.errorPart, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestArgs_ValidateArgs_ValidCombinations(t *testing.T) {
	setupViperForTest()

	// Create a temporary file for file validation tests
	tmpFile, err := os.CreateTemp("", "test_cert_*.pem")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tests := []struct {
		name       string
		viperSetup func()
	}{
		{
			name: "first node with cert manager",
			viperSetup: func() {
				viper.Set("FIRST_NODE", true)
				viper.Set("GPU_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", true)
			},
		},
		{
			name: "first node with generated certs",
			viperSetup: func() {
				viper.Set("FIRST_NODE", true)
				viper.Set("GPU_NODE", false)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", false)
				viper.Set("CERT_OPTION", "generate")
			},
		},
		{
			name: "first node with existing certs",
			viperSetup: func() {
				viper.Set("FIRST_NODE", true)
				viper.Set("GPU_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", false)
				viper.Set("CERT_OPTION", "existing")
				viper.Set("TLS_CERT", tmpFile.Name())
				viper.Set("TLS_KEY", tmpFile.Name())
			},
		},
		{
			name: "additional node",
			viperSetup: func() {
				viper.Set("FIRST_NODE", false)
				viper.Set("GPU_NODE", false)
				viper.Set("SERVER_IP", "192.168.1.100")
				viper.Set("JOIN_TOKEN", "K10abcdef1234567890abcdef1234567890abcdef123456789")
			},
		},
		{
			name: "additional control plane node",
			viperSetup: func() {
				viper.Set("FIRST_NODE", false)
				viper.Set("CONTROL_PLANE", true)
				viper.Set("GPU_NODE", true)
				viper.Set("SERVER_IP", "192.168.1.100")
				viper.Set("JOIN_TOKEN", "K10abcdef1234567890abcdef1234567890abcdef123456789")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupViperForTest()
			tt.viperSetup()
			err := ValidateArgs()
			if err != nil {
				t.Errorf("Expected valid configuration but got error: %v", err)
			}
		})
	}
}

func TestArgs_ValidateArgs_CustomValidator(t *testing.T) {
	setupViperForTest()

	tests := []struct {
		name        string
		viperSetup  func()
		expectError bool
		errorPart   string
	}{
		{
			name: "custom validator success - valid JOIN_TOKEN",
			viperSetup: func() {
				viper.Set("FIRST_NODE", false)
				viper.Set("SERVER_IP", "192.168.1.100")
				viper.Set("JOIN_TOKEN", "K10abcdef1234567890abcdef1234567890abcdef123456789")
			},
			expectError: false,
		},
		{
			name: "custom validator failure - JOIN_TOKEN too short",
			viperSetup: func() {
				viper.Set("FIRST_NODE", false)
				viper.Set("SERVER_IP", "192.168.1.100")
				viper.Set("JOIN_TOKEN", "short")
			},
			expectError: true,
			errorPart:   "JOIN_TOKEN is too short (minimum 32 characters), got 5 characters",
		},
		{
			name: "custom validator failure - JOIN_TOKEN too long",
			viperSetup: func() {
				viper.Set("FIRST_NODE", false)
				viper.Set("SERVER_IP", "192.168.1.100")
				// Create a 520 character token (over the 512 limit)
				longToken := ""
				for i := 0; i < 52; i++ {
					longToken += "1234567890"
				}
				viper.Set("JOIN_TOKEN", longToken)
			},
			expectError: true,
			errorPart:   "JOIN_TOKEN is too long (maximum 512 characters), got 520 characters",
		},
		{
			name: "custom validator failure - JOIN_TOKEN invalid characters",
			viperSetup: func() {
				viper.Set("FIRST_NODE", false)
				viper.Set("SERVER_IP", "192.168.1.100")
				viper.Set("JOIN_TOKEN", "K10abcdef1234567890abcdef1234567890abcdef123456789@#$")
			},
			expectError: true,
			errorPart:   "JOIN_TOKEN contains invalid characters (only alphanumeric, +, /, =, _, ., :, - allowed)",
		},
		{
			name: "custom validator not called when dependency not satisfied",
			viperSetup: func() {
				// FIRST_NODE=true means JOIN_TOKEN dependency not satisfied, so validator shouldn't be called
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", true)
				viper.Set("JOIN_TOKEN", "invalid@#$")
			},
			expectError: false,
		},
		{
			name: "custom validator not called when value is empty and not required",
			viperSetup: func() {
				viper.Set("FIRST_NODE", false)
				viper.Set("SERVER_IP", "192.168.1.100")
				viper.Set("JOIN_TOKEN", "")
			},
			expectError: true,
			errorPart:   "JOIN_TOKEN is required", // Should fail on required validation, not custom validator
		},
		{
			name: "step names validator success - valid DISABLED_STEPS",
			viperSetup: func() {
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", true)
				viper.Set("DISABLED_STEPS", "SetupLonghornStep,SetupMetallbStep")
			},
			expectError: false,
		},
		{
			name: "step names validator success - valid ENABLED_STEPS",
			viperSetup: func() {
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", true)
				viper.Set("ENABLED_STEPS", "CheckUbuntuStep,SetupRKE2Step")
			},
			expectError: false,
		},
		{
			name: "step names validator failure - invalid DISABLED_STEPS",
			viperSetup: func() {
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", true)
				viper.Set("DISABLED_STEPS", "InvalidStepName,SetupLonghornStep")
			},
			expectError: true,
			errorPart:   "invalid step name 'InvalidStepName'",
		},
		{
			name: "step names validator failure - invalid ENABLED_STEPS",
			viperSetup: func() {
				viper.Set("FIRST_NODE", true)
				viper.Set("DOMAIN", "cluster.example.com")
				viper.Set("USE_CERT_MANAGER", true)
				viper.Set("ENABLED_STEPS", "CheckUbuntuStep,NonExistentStep")
			},
			expectError: true,
			errorPart:   "invalid step name 'NonExistentStep'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupViperForTest()
			tt.viperSetup()
			err := ValidateArgs()

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorPart) {
					t.Errorf("Expected error containing '%s' but got: %v", tt.errorPart, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}
