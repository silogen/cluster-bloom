package config

import (
	"fmt"
	"strings"
)

// GenerateYAML generates a bloom.yaml file from the configuration
func GenerateYAML(cfg Config) string {
	var lines []string

	// Get schema to maintain order and get defaults
	schema := Schema()

	// Create a sorted list of keys for consistent output
	var keys []string
	for _, arg := range schema {
		if value, exists := cfg[arg.Key]; exists && value != nil {
			// Always include FIRST_NODE and GPU_NODE
			if arg.Key == "FIRST_NODE" || arg.Key == "GPU_NODE" {
				keys = append(keys, arg.Key)
				continue
			}
			// Only include other fields if value differs from default
			if !isDefaultValue(arg, value) {
				keys = append(keys, arg.Key)
			}
		}
	}

	// Generate YAML lines
	for _, key := range keys {
		value := cfg[key]
		line := formatYAMLLine(key, value)
		if line != "" {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n") + "\n"
}

func isDefaultValue(arg Argument, value any) bool {
	// Special case: If value is explicitly empty string and default is non-empty,
	// this is NOT a default value (user intentionally cleared it)
	if strVal, ok := value.(string); ok && strVal == "" {
		if defaultStr, ok := arg.Default.(string); ok && defaultStr != "" {
			return false // Empty string overriding non-empty default
		}
	}

	// Compare with default value
	switch defaultVal := arg.Default.(type) {
	case bool:
		if boolVal, ok := value.(bool); ok {
			return boolVal == defaultVal
		}
		if strVal, ok := value.(string); ok {
			return (strVal == "true" && defaultVal) || (strVal == "false" && !defaultVal) || strVal == ""
		}
	case string:
		if strVal, ok := value.(string); ok {
			return strVal == defaultVal
		}
	case []any:
		if arrVal, ok := value.([]any); ok {
			return len(arrVal) == 0 && len(defaultVal) == 0
		}
		if strVal, ok := value.(string); ok {
			return strVal == ""
		}
	}
	return false
}

func formatYAMLLine(key string, value any) string {
	switch v := value.(type) {
	case bool:
		return fmt.Sprintf("%s: %t", key, v)
	case string:
		// Quote strings if they contain special characters OR are empty
		if needsQuotes(v) || v == "" {
			return fmt.Sprintf("%s: \"%s\"", key, escapeString(v))
		}
		return fmt.Sprintf("%s: %s", key, v)
	case []any:
		// Handle arrays (for ADDITIONAL_OIDC_PROVIDERS)
		if len(v) == 0 {
			return fmt.Sprintf("%s: []", key)
		}
		// For now, output as empty array or JSON-like format
		return fmt.Sprintf("%s: []", key)
	default:
		return fmt.Sprintf("%s: %v", key, v)
	}
}

func needsQuotes(s string) bool {
	if s == "" {
		return false
	}
	// Check for special YAML characters
	special := []string{":", "#", "[", "]", "{", "}", ",", "&", "*", "!", "|", ">", "'", "\"", "%", "@", "`"}
	for _, char := range special {
		if strings.Contains(s, char) {
			return true
		}
	}
	return false
}

func escapeString(s string) string {
	// Escape quotes in strings
	return strings.ReplaceAll(s, "\"", "\\\"")
}
