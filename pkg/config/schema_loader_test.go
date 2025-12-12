package config

import (
	"testing"
)

func TestLoadSchema(t *testing.T) {
	args, err := LoadSchema()
	if err != nil {
		t.Fatalf("LoadSchema() failed: %v", err)
	}

	if len(args) == 0 {
		t.Fatal("LoadSchema() returned no arguments")
	}

	// Check that we have expected number of fields (26 fields in schema)
	if len(args) != 26 {
		t.Errorf("Expected 26 arguments, got %d", len(args))
	}

	// Verify critical fields are present
	foundDomain := false
	foundServerIP := false
	for _, arg := range args {
		if arg.Key == "DOMAIN" {
			foundDomain = true
			if arg.Type != "domain" {
				t.Errorf("DOMAIN type should be 'domain', got '%s'", arg.Type)
			}
			if arg.Section != "📋 Basic Configuration" {
				t.Errorf("DOMAIN section should be '📋 Basic Configuration', got '%s'", arg.Section)
			}
			if arg.Pattern == "" {
				t.Error("DOMAIN should have a pattern")
			}
			if arg.PatternTitle == "" {
				t.Error("DOMAIN should have a patternTitle")
			}
			if arg.Dependencies != "FIRST_NODE=true" {
				t.Errorf("DOMAIN dependencies should be 'FIRST_NODE=true', got '%s'", arg.Dependencies)
			}
		}
		if arg.Key == "SERVER_IP" {
			foundServerIP = true
			if arg.Dependencies != "FIRST_NODE=false" {
				t.Errorf("SERVER_IP dependencies should be 'FIRST_NODE=false', got '%s'", arg.Dependencies)
			}
		}
	}

	if !foundDomain {
		t.Error("DOMAIN field not found in loaded schema")
	}
	if !foundServerIP {
		t.Error("SERVER_IP field not found in loaded schema")
	}
}

func TestLoadSchemaTypes(t *testing.T) {
	args, err := LoadSchema()
	if err != nil {
		t.Fatalf("LoadSchema() failed: %v", err)
	}

	// Check specific type mappings
	typeTests := []struct {
		key          string
		expectedType string
	}{
		{"FIRST_NODE", "bool"},
		{"DOMAIN", "domain"},
		{"SERVER_IP", "ipv4"},
		{"CERT_OPTION", "enum"},
		{"ADDITIONAL_OIDC_PROVIDERS", "array"},
	}

	argMap := make(map[string]Argument)
	for _, arg := range args {
		argMap[arg.Key] = arg
	}

	for _, tt := range typeTests {
		arg, ok := argMap[tt.key]
		if !ok {
			t.Errorf("Field %s not found", tt.key)
			continue
		}
		if arg.Type != tt.expectedType {
			t.Errorf("Field %s: expected type '%s', got '%s'", tt.key, tt.expectedType, arg.Type)
		}
	}
}

func TestLoadSchemaEnumOptions(t *testing.T) {
	args, err := LoadSchema()
	if err != nil {
		t.Fatalf("LoadSchema() failed: %v", err)
	}

	// Find CERT_OPTION and verify it has options
	for _, arg := range args {
		if arg.Key == "CERT_OPTION" {
			if len(arg.Options) != 2 {
				t.Errorf("CERT_OPTION should have 2 options, got %d", len(arg.Options))
			}
			expectedOptions := map[string]bool{"existing": true, "generate": true}
			for _, opt := range arg.Options {
				if !expectedOptions[opt] {
					t.Errorf("Unexpected option for CERT_OPTION: %s", opt)
				}
			}
			return
		}
	}
	t.Error("CERT_OPTION not found in schema")
}

func TestLoadSchemaComplexDependencies(t *testing.T) {
	args, err := LoadSchema()
	if err != nil {
		t.Fatalf("LoadSchema() failed: %v", err)
	}

	// Test complex dependency parsing
	for _, arg := range args {
		if arg.Key == "CERT_OPTION" {
			expected := "USE_CERT_MANAGER=false,FIRST_NODE=true"
			if arg.Dependencies != expected {
				t.Errorf("CERT_OPTION dependencies should be '%s', got '%s'", expected, arg.Dependencies)
			}
			return
		}
	}
	t.Error("CERT_OPTION not found in schema")
}

func TestSchemaSorting(t *testing.T) {
	args, err := LoadSchema()
	if err != nil {
		t.Fatalf("LoadSchema() failed: %v", err)
	}

	// Verify args are sorted by section
	sections := []string{
		"📋 Basic Configuration",
		"🔗 Additional Node Configuration",
		"💾 Storage Configuration",
		"🔒 SSL/TLS Configuration",
		"⚙️ Advanced Configuration",
		"💻 Command Line Options",
	}

	currentSectionIdx := -1
	for _, arg := range args {
		sectionIdx := -1
		for i, section := range sections {
			if arg.Section == section {
				sectionIdx = i
				break
			}
		}
		if sectionIdx < currentSectionIdx {
			t.Errorf("Arguments not properly sorted by section. Found %s after section index %d", arg.Section, currentSectionIdx)
		}
		if sectionIdx > currentSectionIdx {
			currentSectionIdx = sectionIdx
		}
	}
}
