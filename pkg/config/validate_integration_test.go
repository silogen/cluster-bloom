package config

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// SchemaDefinition represents the complete schema structure
type SchemaDefinition struct {
	Schema struct {
		Type    string                    `yaml:"type"`
		Mapping map[string]SchemaField    `yaml:"mapping"`
	} `yaml:"schema"`
	Types       map[string]TypeDefinition `yaml:"types"`
	Constraints []ConstraintDef           `yaml:"constraints"`
}

// SchemaField represents a field in the schema mapping
type SchemaField struct {
	Type       string      `yaml:"type"`
	Default    interface{} `yaml:"default"`
	Desc       string      `yaml:"desc"`
	Section    string      `yaml:"section"`
	Required   string      `yaml:"required"`
	Applicable string      `yaml:"applicable"`
	Values     []string    `yaml:"values"`
	Examples   []string    `yaml:"examples"`
}

var schemaDefinition *SchemaDefinition

// loadSchemaDefinition loads the complete schema file
func loadSchemaDefinition(t *testing.T) *SchemaDefinition {
	if schemaDefinition != nil {
		return schemaDefinition
	}

	data, err := os.ReadFile("../../schema/bloom.yaml.schema.yaml")
	if err != nil {
		t.Fatalf("Failed to read schema file: %v", err)
	}

	var schema SchemaDefinition
	if err := yaml.Unmarshal(data, &schema); err != nil {
		t.Fatalf("Failed to parse schema file: %v", err)
	}

	schemaDefinition = &schema
	return schemaDefinition
}

// getBaseValidConfig returns a minimal valid config for first node
func getBaseValidConfig() Config {
	return Config{
		"FIRST_NODE":            true,
		"GPU_NODE":              false,
		"DOMAIN":                "test.example.com",
		"NO_DISKS_FOR_CLUSTER": true,
		"CERT_OPTION":           "generate",
	}
}

// getAdditionalNodeConfig returns a valid config for additional node
func getAdditionalNodeConfig() Config {
	return Config{
		"FIRST_NODE":            false,
		"GPU_NODE":              false,
		"SERVER_IP":             "192.168.1.10",
		"JOIN_TOKEN":            "K10token::server:abc123",
		"NO_DISKS_FOR_CLUSTER": true,
	}
}

// TestValidate_AllTypesWithSchemaExamples tests each type's validation using schema examples
func TestValidate_AllTypesWithSchemaExamples(t *testing.T) {
	schema := loadSchemaDefinition(t)

	// Map field names to their types from schema
	fieldTypes := make(map[string]string)
	for fieldName, field := range schema.Schema.Mapping {
		fieldTypes[fieldName] = field.Type
	}

	// Test each type that has examples
	for typeName, typeDef := range schema.Types {
		if len(typeDef.Examples.Valid) == 0 && len(typeDef.Examples.Invalid) == 0 {
			continue
		}

		// Find a field that uses this type
		var testFieldName string
		for fieldName, fieldType := range fieldTypes {
			if fieldType == typeName {
				testFieldName = fieldName
				break
			}
		}

		if testFieldName == "" {
			continue // No field uses this type
		}

		t.Run(fmt.Sprintf("type_%s_field_%s", typeName, testFieldName), func(t *testing.T) {
			// Test valid examples
			for _, validValue := range typeDef.Examples.Valid {
				if validValue == "" && typeName != "str" {
					continue // Skip empty strings for non-string types
				}

				t.Run(fmt.Sprintf("valid_%s", sanitizeName(validValue)), func(t *testing.T) {
					config := buildConfigWithField(testFieldName, validValue)
					errors := Validate(config)

					// Filter out errors not related to our test field
					relevantErrors := filterErrorsForField(errors, testFieldName, typeName)

					if len(relevantErrors) > 0 {
						t.Errorf("Expected valid value %q for %s (type %s) to pass, got errors: %v",
							validValue, testFieldName, typeName, relevantErrors)
					}
				})
			}

			// Test invalid examples
			for _, invalidValue := range typeDef.Examples.Invalid {
				if invalidValue == "" {
					continue // Skip empty strings
				}

				t.Run(fmt.Sprintf("invalid_%s", sanitizeName(invalidValue)), func(t *testing.T) {
					config := buildConfigWithField(testFieldName, invalidValue)
					errors := Validate(config)

					// Should have at least one error
					if len(errors) == 0 {
						t.Errorf("Expected invalid value %q for %s (type %s) to fail validation, but it passed",
							invalidValue, testFieldName, typeName)
					}
				})
			}
		})
	}
}

// buildConfigWithField creates a valid config with a specific field set
func buildConfigWithField(fieldName string, value string) Config {
	// Start with base valid config
	var config Config

	// Determine which base config to use based on field
	if fieldName == "SERVER_IP" || fieldName == "JOIN_TOKEN" || fieldName == "CONTROL_PLANE" {
		config = getAdditionalNodeConfig()
	} else {
		config = getBaseValidConfig()
	}

	// Set the test field
	config[fieldName] = value

	// Handle special cases for dependent fields
	switch fieldName {
	case "TLS_CERT":
		config["CERT_OPTION"] = "existing"
		// TLS_CERT already set to test value above
		config["TLS_KEY"] = "/etc/ssl/private/key.pem" // Valid companion
	case "TLS_KEY":
		config["CERT_OPTION"] = "existing"
		config["TLS_CERT"] = "/etc/ssl/certs/cert.pem" // Valid companion
		// TLS_KEY already set to test value above
	case "CERT_OPTION":
		if value == "existing" {
			config["TLS_CERT"] = "/etc/ssl/certs/cert.pem"
			config["TLS_KEY"] = "/etc/ssl/private/key.pem"
		}
	case "USE_CERT_MANAGER":
		if value == "true" || value == "True" {
			delete(config, "CERT_OPTION")
		}
	case "ROCM_BASE_URL", "ROCM_DEB_PACKAGE":
		config["GPU_NODE"] = true
	case "CLUSTER_DISKS":
		delete(config, "NO_DISKS_FOR_CLUSTER")
	case "CLUSTER_PREMOUNTED_DISKS":
		delete(config, "NO_DISKS_FOR_CLUSTER")
	}

	return config
}

// filterErrorsForField filters errors to only those relevant to the test field
func filterErrorsForField(errors []string, fieldName string, typeName string) []string {
	var relevant []string

	for _, err := range errors {
		errLower := strings.ToLower(err)
		fieldLower := strings.ToLower(fieldName)

		// Include errors that mention the field name
		if strings.Contains(errLower, fieldLower) {
			relevant = append(relevant, err)
			continue
		}

		// Include errors about the type validation
		if strings.Contains(errLower, "domain") && typeName == "domain" {
			relevant = append(relevant, err)
			continue
		}
		if strings.Contains(errLower, "ip") && typeName == "ipv4" {
			relevant = append(relevant, err)
			continue
		}
		if strings.Contains(errLower, "url") && typeName == "url" {
			relevant = append(relevant, err)
			continue
		}
	}

	return relevant
}

// sanitizeName creates a valid test name from a value
func sanitizeName(value string) string {
	if value == "" {
		return "empty"
	}

	// Replace problematic characters
	name := strings.ReplaceAll(value, "/", "_slash_")
	name = strings.ReplaceAll(name, ":", "_colon_")
	name = strings.ReplaceAll(name, ".", "_dot_")
	name = strings.ReplaceAll(name, " ", "_space_")
	name = strings.ReplaceAll(name, ",", "_comma_")
	name = strings.ReplaceAll(name, "-", "_dash_")
	name = strings.ReplaceAll(name, "@", "_at_")
	name = strings.ReplaceAll(name, "//", "_")

	// Truncate if too long
	if len(name) > 100 {
		name = name[:100]
	}

	return name
}

// TestValidate_RequiredFieldsFromSchema tests required fields based on schema dependencies
func TestValidate_RequiredFieldsFromSchema(t *testing.T) {
	schema := loadSchemaDefinition(t)

	tests := []struct {
		name       string
		config     Config
		wantError  string
	}{
		{
			name: "DOMAIN required when FIRST_NODE=true",
			config: Config{
				"FIRST_NODE":            true,
				"NO_DISKS_FOR_CLUSTER": true,
			},
			wantError: "DOMAIN",
		},
		{
			name: "SERVER_IP required when FIRST_NODE=false",
			config: Config{
				"FIRST_NODE":            false,
				"JOIN_TOKEN":            "K10token::server:abc",
				"NO_DISKS_FOR_CLUSTER": true,
			},
			wantError: "SERVER_IP",
		},
		{
			name: "JOIN_TOKEN required when FIRST_NODE=false",
			config: Config{
				"FIRST_NODE":            false,
				"SERVER_IP":             "192.168.1.10",
				"NO_DISKS_FOR_CLUSTER": true,
			},
			wantError: "JOIN_TOKEN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := Validate(tt.config)

			found := false
			for _, err := range errors {
				if strings.Contains(err, tt.wantError) {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected error containing %q, got: %v", tt.wantError, errors)
			}
		})
	}

	_ = schema // Use schema to avoid unused warning
}

// TestValidate_ConstraintsFromSchema tests constraints defined in schema
func TestValidate_ConstraintsFromSchema(t *testing.T) {
	schema := loadSchemaDefinition(t)

	if len(schema.Constraints) == 0 {
		t.Skip("No constraints defined in schema")
	}

	for i, constraint := range schema.Constraints {
		t.Run(fmt.Sprintf("constraint_%d", i), func(t *testing.T) {
			// Test mutually exclusive constraints
			if len(constraint.MutuallyExclusive) >= 2 {
				t.Run("mutually_exclusive", func(t *testing.T) {
					config := getBaseValidConfig()

					// Set all mutually exclusive fields
					for _, field := range constraint.MutuallyExclusive {
						config[field] = "test-value"
					}

					errors := Validate(config)
					if len(errors) == 0 {
						t.Errorf("Expected error when all mutually exclusive fields %v are set",
							constraint.MutuallyExclusive)
					}
				})
			}

			// Test one-of constraints
			if len(constraint.OneOf) > 0 {
				t.Run("one_of_none_set", func(t *testing.T) {
					config := getBaseValidConfig()

					// Remove all one-of fields
					for _, field := range constraint.OneOf {
						delete(config, field)
					}

					errors := Validate(config)
					if len(errors) == 0 {
						t.Errorf("Expected error when none of one-of fields %v are set",
							constraint.OneOf)
					}
				})

				t.Run("one_of_multiple_set", func(t *testing.T) {
					config := getBaseValidConfig()

					// Set multiple one-of fields
					if len(constraint.OneOf) >= 2 {
						config[constraint.OneOf[0]] = true
						config[constraint.OneOf[1]] = "/dev/sda"

						errors := Validate(config)
						if len(errors) == 0 {
							t.Errorf("Expected error when multiple one-of fields are set: %v",
								constraint.OneOf)
						}
					}
				})
			}
		})
	}
}

func TestValidate_UnknownKeyRejected(t *testing.T) {
	config := Config{
		"FIRST_NODE":           true,
		"DOMAIN":               "test.example.com",
		"NO_DISKS_FOR_CLUSTER": true,
		"INVALID_KEY":          "some-value",
	}

	errors := Validate(config)

	if len(errors) == 0 {
		t.Fatal("Expected validation error for unknown key INVALID_KEY, but got none")
	}

	foundError := false
	for _, err := range errors {
		if strings.Contains(err, "Unknown configuration key") && strings.Contains(err, "INVALID_KEY") {
			foundError = true
			break
		}
	}

	if !foundError {
		t.Errorf("Expected 'Unknown configuration key: INVALID_KEY' error, but got: %v", errors)
	}
}
