package config

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// GenerateYAML generates a bloom.yaml file from the configuration.
//
// It returns an error rather than dropping a field it cannot render. The
// encoder path below is unreachable for values that came from a YAML or JSON
// decode (plain, acyclic data), but silently omitting a key the user set is a
// worse failure than a loud one: they would export a bloom.yaml, read it, and
// find their setting simply absent.
func GenerateYAML(cfg Config) (string, error) {
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
		line, err := formatYAMLLine(key, value)
		if err != nil {
			return "", err
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n") + "\n", nil
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

func formatYAMLLine(key string, value any) (string, error) {
	switch v := value.(type) {
	case bool:
		return fmt.Sprintf("%s: %t", key, v), nil
	case map[string]any:
		// Nested maps go to the YAML encoder, not to fmt: the default branch
		// below renders a Go map as `map[operator:map[replicas:1]]`, which is
		// not YAML, and any hand-rolled replacement has to re-solve quoting and
		// key ordering. yaml.v3 sorts map keys, so regenerating an unchanged
		// config is a byte-for-byte no-op.
		if len(v) == 0 {
			return fmt.Sprintf("%s: {}", key), nil
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
			return fmt.Sprintf("%s: \"%s\"", key, escapeString(v)), nil
		}
		// Quote strings if they contain special characters OR are empty
		if needsQuotes(v) || v == "" {
			return fmt.Sprintf("%s: \"%s\"", key, escapeString(v)), nil
		}
		return fmt.Sprintf("%s: %s", key, v), nil
	case []any:
		// Handle arrays (for ADDITIONAL_OIDC_PROVIDERS)
		if len(v) == 0 {
			return fmt.Sprintf("%s: []", key), nil
		}
		// Generate proper YAML array format
		return formatYAMLArray(key, v), nil
	default:
		return fmt.Sprintf("%s: %v", key, v), nil
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
func marshalYAMLField(key string, value any) (out string, err error) {
	// yaml.v3 splits its failures two ways: it returns an error for a write
	// failure or a TypeError, but *panics* for a type it cannot represent at
	// all (a channel, a func). Config values come from a YAML or JSON decode,
	// so neither is reachable today; funnelling both into one error keeps a
	// dormant branch from later turning into a dropped key or, in the web
	// handler, a connection dropped by net/http's own recover with no response.
	defer func() {
		if r := recover(); r != nil {
			out, err = "", fmt.Errorf("render %s as YAML: %v", key, r)
		}
	}()

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(map[string]any{key: value}); err != nil {
		return "", fmt.Errorf("render %s as YAML: %w", key, err)
	}
	if err := enc.Close(); err != nil {
		return "", fmt.Errorf("render %s as YAML: %w", key, err)
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}
