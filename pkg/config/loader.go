package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfig reads and parses a bloom.yaml configuration file
func LoadConfig(filepath string) (Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

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

	// Apply defaults for any missing keys
	for _, arg := range args {
		if arg.Default == nil {
			continue
		}

		// Check if the key exists in config, if not set the default
		if _, exists := (*config)[arg.Key]; !exists {
			(*config)[arg.Key] = arg.Default
		}
	}

	return nil
}
