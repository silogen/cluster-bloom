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
			keys = append(keys, arg.Key)
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

func formatYAMLLine(key string, value any) string {
	switch v := value.(type) {
	case bool:
		return fmt.Sprintf("%s: %t", key, v)
	case string:
		// Quote strings if they contain special characters or are empty
		if needsQuotes(v) {
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

// GetFieldOrder returns the schema order for deterministic YAML generation
func GetFieldOrder() []string {
	schema := Schema()
	keys := make([]string, len(schema))
	for i, arg := range schema {
		keys[i] = arg.Key
	}
	return keys
}
