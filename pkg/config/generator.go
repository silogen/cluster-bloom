package config

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
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
	case map[string]any:
		// An untouched `{}` (or a web UI textarea left blank) is the default
		// and must not be written to the generated file.
		if mapVal, ok := value.(map[string]any); ok {
			return len(mapVal) == 0 && len(defaultVal) == 0
		}
		if strVal, ok := value.(string); ok {
			return strings.TrimSpace(strVal) == ""
		}
	}
	return false
}

func formatYAMLLine(key string, value any) string {
	switch v := value.(type) {
	case bool:
		return fmt.Sprintf("%s: %t", key, v)
	case map[string]any:
		// Nested maps go to the YAML encoder, not to fmt: the default branch
		// below renders a Go map as `map[operator:map[replicas:1]]`, which is
		// not YAML, and any hand-rolled replacement has to re-solve quoting and
		// key ordering. yaml.v3 sorts map keys, so regenerating an unchanged
		// config is a byte-for-byte no-op.
		if len(v) == 0 {
			return fmt.Sprintf("%s: {}", key)
		}
		return marshalYAMLField(key, v)
	case string:
		// A multi-line value cannot be a double-quoted scalar containing
		// literal newlines. escapeString only escapes `"`, so RKE2_EXTRA_CONFIG
		// round-tripped through the web UI came back corrupt. Let the encoder
		// pick a block scalar.
		if strings.Contains(v, "\n") {
			return marshalYAMLField(key, v)
		}
		// Always quote CLUSTER_LISTEN_IP for consistency
		if key == "CLUSTER_LISTEN_IP" {
			return fmt.Sprintf("%s: \"%s\"", key, escapeString(v))
		}
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
		// Generate proper YAML array format
		return formatYAMLArray(key, v)
	default:
		return fmt.Sprintf("%s: %v", key, v)
	}
}

func formatYAMLArray(key string, arr []any) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("%s:", key))
	for _, item := range arr {
		if itemMap, ok := item.(map[string]any); ok {
			// Handle complex array elements like OIDC providers
			lines = append(lines, formatMapAsYAMLItem(itemMap))
		} else {
			// Handle simple array elements
			if needsQuotes(fmt.Sprintf("%v", item)) {
				lines = append(lines, fmt.Sprintf("  - \"%v\"", item))
			} else {
				lines = append(lines, fmt.Sprintf("  - %v", item))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func formatMapAsYAMLItem(itemMap map[string]any) string {
	var lines []string
	first := true
	for key, value := range itemMap {
		switch v := value.(type) {
		case string:
			if first {
				if needsQuotes(v) || v == "" {
					lines = append(lines, fmt.Sprintf("  - %s: \"%s\"", key, escapeString(v)))
				} else {
					lines = append(lines, fmt.Sprintf("  - %s: %s", key, v))
				}
				first = false
			} else {
				if needsQuotes(v) || v == "" {
					lines = append(lines, fmt.Sprintf("    %s: \"%s\"", key, escapeString(v)))
				} else {
					lines = append(lines, fmt.Sprintf("    %s: %s", key, v))
				}
			}
		case []any:
			// Handle nested arrays (like audiences)
			if first {
				lines = append(lines, fmt.Sprintf("  - %s:", key))
				first = false
			} else {
				lines = append(lines, fmt.Sprintf("    %s:", key))
			}
			for _, arrItem := range v {
				if needsQuotes(fmt.Sprintf("%v", arrItem)) {
					lines = append(lines, fmt.Sprintf("      - \"%v\"", arrItem))
				} else {
					lines = append(lines, fmt.Sprintf("      - %v", arrItem))
				}
			}
		default:
			if first {
				lines = append(lines, fmt.Sprintf("  - %s: %v", key, value))
				first = false
			} else {
				lines = append(lines, fmt.Sprintf("    %s: %v", key, value))
			}
		}
	}
	return strings.Join(lines, "\n")
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

// marshalYAMLField renders one top-level `key: value` field with the YAML
// encoder, at the 2-space indent the rest of the generated file uses (yaml.v3
// defaults to 4). Key order is deterministic: yaml.v3 sorts map keys.
func marshalYAMLField(key string, value any) string {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(map[string]any{key: value}); err != nil {
		return ""
	}
	if err := enc.Close(); err != nil {
		return ""
	}
	return strings.TrimRight(buf.String(), "\n")
}
