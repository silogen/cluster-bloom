package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// TypePatternDef represents a type pattern from schema
type TypePatternDef struct {
	Type         string `yaml:"type"`
	Pattern      string `yaml:"pattern"`
	ErrorMessage string `yaml:"errorMessage"`
}

// SchemaTypes represents the types section from schema
type SchemaTypes struct {
	Types map[string]TypePatternDef `yaml:"types"`
}

var cachedPatterns map[string]*regexp.Regexp

// loadTypePatterns loads regex patterns from schema
func loadTypePatterns() (map[string]*regexp.Regexp, error) {
	if cachedPatterns != nil {
		return cachedPatterns, nil
	}

	paths := []string{
		"schema/bloom.yaml.schema.yaml",
		"../../schema/bloom.yaml.schema.yaml",
		"/workspace/cluster-bloom/schema/bloom.yaml.schema.yaml",
	}

	var data []byte
	var err error
	for _, path := range paths {
		data, err = os.ReadFile(path)
		if err == nil {
			break
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read schema file: %w", err)
	}

	var schema SchemaTypes
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	patterns := make(map[string]*regexp.Regexp)
	for typeName, typeDef := range schema.Types {
		if typeDef.Pattern != "" {
			patterns[typeName] = regexp.MustCompile(typeDef.Pattern)
		}
	}

	cachedPatterns = patterns
	return patterns, nil
}

// Validate validates a configuration against the schema
func Validate(cfg Config) []string {
	var errors []string
	schema := Schema()

	// Load type patterns from schema
	patterns, err := loadTypePatterns()
	if err != nil {
		errors = append(errors, fmt.Sprintf("Failed to load validation patterns: %v", err))
		patterns = make(map[string]*regexp.Regexp) // Continue with empty patterns
	}

	for _, arg := range schema {
		// Check if field is visible based on dependencies
		if !isArgVisible(arg, cfg) {
			continue
		}

		value, exists := cfg[arg.Key]

		// Check required fields
		if arg.Required {
			if !exists || value == nil || value == "" {
				errors = append(errors, fmt.Sprintf("%s is required", arg.Key))
				continue
			}
		}

		// Type-specific validation
		if exists && value != nil {
			strVal, isString := value.(string)

			switch arg.Type {
			case "enum":
				if isString && strVal != "" {
					if !contains(arg.Options, strVal) {
						errors = append(errors, fmt.Sprintf("%s must be one of: %s", arg.Key, strings.Join(arg.Options, ", ")))
					}
				}
			case "bool":
				// Bool conversion is handled by YAML parser
			case "str":
				// Plain string, no pattern validation
			default:
				// Check if this type has a pattern
				if isString && strVal != "" {
					if pattern, ok := patterns[arg.Type]; ok {
						if !pattern.MatchString(strVal) {
							errors = append(errors, fmt.Sprintf("invalid %s format: %s", arg.Type, strVal))
						}
					}
				}
			}
		}
	}

	// Validate constraints (mutually exclusive, one-of, etc.)
	constraintErrors := ValidateConstraints(cfg)
	errors = append(errors, constraintErrors...)

	return errors
}

func isArgVisible(arg Argument, cfg Config) bool {
	if arg.Dependencies == "" {
		return true
	}

	// Split by comma for AND logic
	deps := strings.Split(arg.Dependencies, ",")
	for _, dep := range deps {
		dep = strings.TrimSpace(dep)
		if !evaluateDependency(dep, cfg) {
			return false
		}
	}
	return true
}

func evaluateDependency(depStr string, cfg Config) bool {
	parts := strings.SplitN(depStr, "=", 2)
	if len(parts) != 2 {
		return false
	}

	key := strings.TrimSpace(parts[0])
	expectedValue := strings.TrimSpace(parts[1])

	actualValue, exists := cfg[key]
	if !exists {
		return false
	}

	// Handle boolean comparisons
	if expectedValue == "true" {
		if boolVal, ok := actualValue.(bool); ok {
			return boolVal
		}
		if strVal, ok := actualValue.(string); ok {
			return strVal == "true"
		}
		return false
	}

	if expectedValue == "false" {
		if boolVal, ok := actualValue.(bool); ok {
			return !boolVal
		}
		if strVal, ok := actualValue.(string); ok {
			return strVal == "false" || strVal == ""
		}
		return true
	}

	// Handle string comparisons
	if strVal, ok := actualValue.(string); ok {
		return strVal == expectedValue
	}

	return false
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
