package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadConfig reads and parses a bloom.yaml configuration file
func LoadConfig(filepath string) (Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Decode into a plain map[string]any rather than Config directly: yaml.v3
	// recursively decodes nested mappings using the destination's named map
	// type, so unmarshaling straight into Config would turn nested maps (e.g.
	// CILIUM_HELM_VALUES) into config.Config instead of map[string]any,
	// breaking every map[string]any type assertion downstream.
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	config := Config(raw)

	// Apply defaults from schema
	if err := applyDefaults(&config); err != nil {
		return nil, fmt.Errorf("apply defaults: %w", err)
	}

	return config, nil
}

// applyDefaults applies default values from the schema to the config
func applyDefaults(config *Config) error {
	// Load schema to get default values
	args, err := LoadSchema()
	if err != nil {
		return fmt.Errorf("load schema: %w", err)
	}

	// Apply defaults and environment variables for any missing keys
	for _, arg := range args {
		// Check if the key exists in config
		if _, exists := (*config)[arg.Key]; !exists {
			// Check for environment variable first
			if envVal := os.Getenv(arg.Key); envVal != "" {
				parsed, err := parseEnvironmentValue(arg, envVal)
				if err != nil {
					return fmt.Errorf("%s: %w", arg.Key, err)
				}
				(*config)[arg.Key] = parsed
			} else if arg.Default != nil {
				// Apply default if no environment variable
				(*config)[arg.Key] = arg.Default
			}
		}
	}

	return nil
}

// parseEnvironmentValue preserves schema types for environment overrides.
// Otherwise FIX_DNS=false reaches Ansible as the truthy string "false".
func parseEnvironmentValue(arg Argument, value string) (any, error) {
	switch arg.Type {
	case "bool":
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("must be a boolean, got %q", value)
		}
		return parsed, nil
	case "array":
		var parsed []any
		if err := yaml.Unmarshal([]byte(value), &parsed); err == nil {
			return parsed, nil
		}

		// Accept comma-separated string lists; structured values must use
		// YAML/JSON list syntax.
		for _, item := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				parsed = append(parsed, trimmed)
			}
		}
		return parsed, nil
	case "map":
		// No comma-separated fallback here, unlike "array": a nested mapping has
		// no natural shorthand, so the value has to be YAML (or JSON, which YAML
		// is a superset of). Parsing it here rather than letting the raw string
		// through means a typo is reported against the environment variable
		// instead of surfacing later from the validator.
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return map[string]any{}, nil
		}
		var parsed map[string]any
		if err := yaml.Unmarshal([]byte(trimmed), &parsed); err != nil {
			return nil, fmt.Errorf("must be a YAML mapping: %w", err)
		}
		if parsed == nil {
			return nil, fmt.Errorf("must be a YAML mapping, got %q", value)
		}
		return parsed, nil
	default:
		return value, nil
	}
}
