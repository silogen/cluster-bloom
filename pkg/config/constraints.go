package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ValidateConstraints validates configuration against schema constraints
func ValidateConstraints(cfg Config) []string {
	var errors []string

	// Load constraints from schema
	constraints, err := loadSchemaConstraints()
	if err != nil {
		return []string{fmt.Sprintf("Failed to load constraints: %v", err)}
	}

	// Check each constraint
	for _, constraint := range constraints {
		// Mutually exclusive fields - only check if at least one field is present
		if len(constraint.MutuallyExclusive) >= 2 {
			if anyFieldPresent(cfg, constraint.MutuallyExclusive) {
				if err := checkMutuallyExclusive(cfg, constraint.MutuallyExclusive); err != nil {
					errors = append(errors, err.Error())
				}
			}
		}

		// One-of constraints - always check (must have exactly one)
		if len(constraint.OneOf) > 0 {
			if err := checkOneOfFields(cfg, constraint.OneOf, constraint.Error); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}

	return errors
}

// anyFieldPresent checks if any of the fields exist in config
func anyFieldPresent(cfg Config, fields []string) bool {
	for _, field := range fields {
		val, exists := cfg[field]
		if exists && val != nil && val != "" {
			return true
		}
	}
	return false
}

// checkMutuallyExclusive verifies only one of the fields is set
func checkMutuallyExclusive(cfg Config, fields []string) error {
	setFields := []string{}

	for _, field := range fields {
		val, exists := cfg[field]
		if exists && val != nil && val != "" {
			setFields = append(setFields, field)
		}
	}

	if len(setFields) > 1 {
		return fmt.Errorf("fields %v are mutually exclusive, but %v are set", fields, setFields)
	}

	return nil
}

// checkOneOfFields verifies exactly one of the fields is set
func checkOneOfFields(cfg Config, fields []string, errorMsg string) error {
	setCount := 0
	var setFields []string

	for _, field := range fields {
		if isFieldSet(cfg, field) {
			setCount++
			setFields = append(setFields, field)
		}
	}

	if setCount != 1 {
		if errorMsg != "" {
			return fmt.Errorf("%s", errorMsg)
		}
		if setCount == 0 {
			return fmt.Errorf("exactly one of %v must be set, but none are set", fields)
		}
		return fmt.Errorf("exactly one of %v must be set, but %v are set", fields, setFields)
	}

	return nil
}

// ConstraintDef represents a constraint from the schema
type ConstraintDef struct {
	MutuallyExclusive []string `yaml:"mutually_exclusive" json:"mutually_exclusive,omitempty"`
	OneOf             []string `yaml:"one_of" json:"one_of,omitempty"`
	Error             string   `yaml:"error" json:"error,omitempty"`
}

// Alias for backward compatibility
type constraintDef = ConstraintDef

// isFieldSet checks if a field has a truthy value
func isFieldSet(cfg Config, field string) bool {
	val, exists := cfg[field]
	if !exists || val == nil {
		return false
	}

	// Check boolean fields
	if boolVal, ok := val.(bool); ok {
		return boolVal
	}

	// Check string fields
	if strVal, ok := val.(string); ok {
		return strVal != ""
	}

	return true
}

// schemaConstraints represents the constraints section
type schemaConstraints struct {
	Constraints []constraintDef `yaml:"constraints"`
}

var cachedConstraints []constraintDef

// LoadConstraints loads constraints from the schema file (exported for API)
func LoadConstraints() ([]ConstraintDef, error) {
	return loadSchemaConstraints()
}

// loadSchemaConstraints loads constraints from the schema file
func loadSchemaConstraints() ([]constraintDef, error) {
	if cachedConstraints != nil {
		return cachedConstraints, nil
	}

	// Use the embedded schema data from schema_loader.go
	var schema schemaConstraints
	if err := yaml.Unmarshal(schemaData, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	cachedConstraints = schema.Constraints
	return cachedConstraints, nil
}
