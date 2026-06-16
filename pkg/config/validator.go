package config

import (
	_ "embed"
	"fmt"
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

// loadTypePatterns loads regex patterns from embedded schema
func loadTypePatterns() (map[string]*regexp.Regexp, error) {
	if cachedPatterns != nil {
		return cachedPatterns, nil
	}

	// Use the embedded schema data from schema_loader.go
	var schema SchemaTypes
	if err := yaml.Unmarshal(schemaData, &schema); err != nil {
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

	// Build set of valid keys from schema
	validKeys := make(map[string]bool)
	for _, arg := range schema {
		validKeys[arg.Key] = true
	}

	// Check for unknown keys in config
	for key := range cfg {
		if !validKeys[key] {
			errors = append(errors, fmt.Sprintf("Unknown configuration key: %s", key))
		}
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
			case "seq":
				// Validate sequence/array fields
				if sequence, ok := value.([]interface{}); ok {
					// Check if sequence has validation rules
					if len(arg.Sequence) > 0 {
						seqDef := arg.Sequence[0]
						if seqDef.Pattern != "" {
							pattern := regexp.MustCompile(seqDef.Pattern)
							for i, item := range sequence {
								if itemStr, ok := item.(string); ok {
									if itemStr != "" && !pattern.MatchString(itemStr) {
										patternTitle := seqDef.PatternTitle
										if patternTitle == "" {
											patternTitle = fmt.Sprintf("invalid format: %s", itemStr)
										}
										errors = append(errors, fmt.Sprintf("%s[%d]: %s", arg.Key, i, patternTitle))
									}
								}
							}
						}
					}
				} else {
					// Handle legacy comma-separated string format for ADDITIONAL_TLS_SAN_URLS
					if strVal, isString := value.(string); isString && arg.Key == "ADDITIONAL_TLS_SAN_URLS" && strVal != "" {
						items := strings.Split(strVal, ",")
						if len(arg.Sequence) > 0 {
							seqDef := arg.Sequence[0]
							if seqDef.Pattern != "" {
								pattern := regexp.MustCompile(seqDef.Pattern)
								for i, item := range items {
									itemStr := strings.TrimSpace(item)
									if itemStr != "" && !pattern.MatchString(itemStr) {
										patternTitle := seqDef.PatternTitle
										if patternTitle == "" {
											patternTitle = fmt.Sprintf("invalid format: %s", itemStr)
										}
										errors = append(errors, fmt.Sprintf("%s[%d]: %s", arg.Key, i, patternTitle))
									}
								}
							}
						}
					}
				}
			case "clusterListenIp":
				// Special validation for CLUSTER_LISTEN_IP - supports string only (IP or CIDR)
				if isString {
					if strVal != "" {
						// Validate single IP or CIDR
						if pattern, ok := patterns[arg.Type]; ok {
							if !pattern.MatchString(strVal) {
								errors = append(errors, fmt.Sprintf("invalid CLUSTER_LISTEN_IP format: '%s'\n"+
									"  Expected formats:\n"+
									"    - Exact IP: \"192.168.1.100\"\n"+
									"    - CIDR subnet: \"192.168.1.0/24\"\n"+
									"  Note: Interface existence will be validated during deployment.", strVal))
							}
						}
					}
					// Empty string is valid (means auto-detection)
				} else if value != nil {
					errors = append(errors, fmt.Sprintf("CLUSTER_LISTEN_IP must be a string (IP address or CIDR), got %T", value))
				}
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

	// Companion validation: DOCKERHUB_USER and DOCKERHUB_TOKEN must be set together
	dockerUser, userExists := cfg["DOCKERHUB_USER"]
	dockerToken, tokenExists := cfg["DOCKERHUB_TOKEN"]
	userSet := userExists && dockerUser != nil && dockerUser != ""
	tokenSet := tokenExists && dockerToken != nil && dockerToken != ""
	if userSet != tokenSet {
		if userSet {
			errors = append(errors, "DOCKERHUB_TOKEN is required when DOCKERHUB_USER is set")
		} else {
			errors = append(errors, "DOCKERHUB_USER is required when DOCKERHUB_TOKEN is set")
		}
	}

	// Special validation for ADDITIONAL_TLS_SAN_URLS (critical security check)
	if tlsSans, exists := cfg["ADDITIONAL_TLS_SAN_URLS"]; exists && tlsSans != nil {
		var domains []string
		
		// Handle array format
		if sequence, ok := tlsSans.([]interface{}); ok {
			for _, item := range sequence {
				if itemStr, ok := item.(string); ok {
					domains = append(domains, itemStr)
				}
			}
		}
		// Handle legacy string format
		if strVal, ok := tlsSans.(string); ok && strVal != "" {
			for _, item := range strings.Split(strVal, ",") {
				domains = append(domains, strings.TrimSpace(item))
			}
		}
		
		// Check for wildcards in any domain
		for i, domain := range domains {
			if strings.Contains(domain, "*") {
				errors = append(errors, fmt.Sprintf("ADDITIONAL_TLS_SAN_URLS[%d]: Wildcard domains (*.domain.com) are not supported by RKE2. Found: %s", i, domain))
			}
		}
	}

	// Validate constraints (mutually exclusive, one-of, etc.)
	constraintErrors := ValidateConstraints(cfg)
	errors = append(errors, constraintErrors...)

	// Fail-fast on unsupported GPU stack (family x ROCm x GPU Operator) combos.
	if gpuStackErr := validateGPUStack(cfg); gpuStackErr != "" {
		errors = append(errors, gpuStackErr)
	}

	return errors
}

// validateGPUStack resolves GPU_STACK_FAMILY against the compatibility matrix
// and returns a non-empty error string for unsupported combinations. The
// pattern type already rejects malformed values; this catches family/ROCm/
// operator combinations marked unsupported in the matrix.
func validateGPUStack(cfg Config) string {
	family := ""
	if v, ok := cfg["GPU_STACK_FAMILY"]; ok && v != nil {
		if s, isStr := v.(string); isStr {
			family = s
		}
	}
	if _, err := ResolveStackProfile(family); err != nil {
		return err.Error()
	}
	return ""
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
