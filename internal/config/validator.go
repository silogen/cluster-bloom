package config

import (
	"fmt"
	"strings"
)

// Validate validates a configuration against the schema
func Validate(cfg Config) []string {
	var errors []string
	schema := Schema()

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
			switch arg.Type {
			case "enum":
				if strVal, ok := value.(string); ok {
					if !contains(arg.Options, strVal) {
						errors = append(errors, fmt.Sprintf("%s must be one of: %s", arg.Key, strings.Join(arg.Options, ", ")))
					}
				}
			case "bool":
				// Bool conversion is handled by YAML parser
			}
		}
	}

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
