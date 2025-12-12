package config

import (
	"fmt"
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// ConstraintDefinition represents a constraint from the schema
type ConstraintDefinition struct {
	MutuallyExclusive []string       `yaml:"mutually_exclusive"`
	OneOf             []string       `yaml:"one_of"`
	Error             string         `yaml:"error"`
	Raw               map[string]any `yaml:",inline"`
}

// SchemaWithConstraints represents the schema file structure
type SchemaWithConstraints struct {
	Constraints []ConstraintDefinition `yaml:"constraints"`
}

var constraintsCache *SchemaWithConstraints

// loadConstraints loads constraints from schema file
func loadConstraints(t *testing.T) *SchemaWithConstraints {
	if constraintsCache != nil {
		return constraintsCache
	}

	data, err := os.ReadFile("../../schema/bloom.yaml.schema.yaml")
	if err != nil {
		t.Fatalf("Failed to read schema file: %v", err)
	}

	var schema SchemaWithConstraints
	if err := yaml.Unmarshal(data, &schema); err != nil {
		t.Fatalf("Failed to parse schema file: %v", err)
	}

	constraintsCache = &schema
	return constraintsCache
}

// TestMutuallyExclusiveConstraints tests all mutually_exclusive constraints from schema
func TestMutuallyExclusiveConstraints(t *testing.T) {
	schema := loadConstraints(t)

	for _, constraint := range schema.Constraints {
		if len(constraint.MutuallyExclusive) > 0 {
			fields := constraint.MutuallyExclusive

			t.Run("both_"+fields[0]+"_and_"+fields[1]+"_set", func(t *testing.T) {
				cfg := Config{
					fields[0]: "value1",
					fields[1]: "value2",
				}

				errors := ValidateConstraints(cfg)
				if len(errors) == 0 {
					t.Errorf("Expected constraint violation when both %s and %s are set", fields[0], fields[1])
				}
			})

			t.Run("only_"+fields[0]+"_set", func(t *testing.T) {
				cfg := Config{
					fields[0]: "value1",
				}

				errors := ValidateConstraints(cfg)
				if len(errors) > 0 {
					t.Errorf("Expected no constraint violation when only %s is set: %v", fields[0], errors)
				}
			})

			t.Run("only_"+fields[1]+"_set", func(t *testing.T) {
				cfg := Config{
					fields[1]: "value1",
				}

				errors := ValidateConstraints(cfg)
				if len(errors) > 0 {
					t.Errorf("Expected no constraint violation when only %s is set: %v", fields[1], errors)
				}
			})

			t.Run("neither_set", func(t *testing.T) {
				cfg := Config{}

				errors := ValidateConstraints(cfg)
				if len(errors) > 0 {
					t.Errorf("Expected no constraint violation when neither field is set: %v", errors)
				}
			})
		}
	}
}

// TestOneOfConstraints tests all one_of constraints from schema dynamically
func TestOneOfConstraints(t *testing.T) {
	schema := loadConstraints(t)

	for i, constraint := range schema.Constraints {
		if len(constraint.OneOf) > 0 {
			fields := constraint.OneOf
			t.Run("one_of_constraint_"+fmt.Sprint(i), func(t *testing.T) {
				// Test: no fields set (should fail)
				t.Run("no_fields_set", func(t *testing.T) {
					cfg := Config{}
					for _, field := range fields {
						if field == "NO_DISKS_FOR_CLUSTER" {
							cfg[field] = false
						} else {
							cfg[field] = ""
						}
					}
					errors := ValidateConstraints(cfg)
					if len(errors) == 0 {
						t.Errorf("Expected error when no fields are set")
					}
				})

				// Test: exactly one field set (should pass)
				for _, field := range fields {
					t.Run("only_"+field+"_set", func(t *testing.T) {
						cfg := Config{}
						for _, f := range fields {
							if f == field {
								// Set this field with appropriate value
								if field == "NO_DISKS_FOR_CLUSTER" {
									cfg[field] = true
								} else {
									cfg[field] = "test-value"
								}
							} else {
								cfg[f] = ""
							}
						}
						errors := ValidateConstraints(cfg)
						if len(errors) > 0 {
							t.Errorf("Expected no error when only %s is set: %v", field, errors)
						}
					})
				}

				// Test: multiple fields set (should fail)
				if len(fields) >= 2 {
					t.Run("multiple_fields_set", func(t *testing.T) {
						cfg := Config{}
						// Set first two fields
						for i, field := range fields {
							if i < 2 {
								if field == "NO_DISKS_FOR_CLUSTER" {
									cfg[field] = true
								} else {
									cfg[field] = "test-value"
								}
							} else {
								cfg[field] = ""
							}
						}
						errors := ValidateConstraints(cfg)
						if len(errors) == 0 {
							t.Errorf("Expected error when multiple fields are set")
						}
					})
				}
			})
		}
	}
}


// TestAllConstraintsAreParsed verifies all constraints in schema are readable
func TestAllConstraintsAreParsed(t *testing.T) {
	schema := loadConstraints(t)

	if len(schema.Constraints) == 0 {
		t.Error("No constraints found in schema")
	}

	t.Logf("Found %d constraints in schema", len(schema.Constraints))

	for i, constraint := range schema.Constraints {
		hasConstraint := false

		if len(constraint.MutuallyExclusive) > 0 {
			hasConstraint = true
			t.Logf("Constraint %d: mutually_exclusive %v", i, constraint.MutuallyExclusive)
		}
		if len(constraint.OneOf) > 0 {
			hasConstraint = true
			t.Logf("Constraint %d: one_of (storage) with %d conditions", i, len(constraint.OneOf))
		}

		if !hasConstraint {
			t.Errorf("Constraint %d has no recognized constraint type: %+v", i, constraint)
		}
	}
}
