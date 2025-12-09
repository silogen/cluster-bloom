package config

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
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
			strVal, _ := value.(string)

			switch arg.Type {
			case "enum":
				if strVal != "" {
					if !contains(arg.Options, strVal) {
						errors = append(errors, fmt.Sprintf("%s must be one of: %s", arg.Key, strings.Join(arg.Options, ", ")))
					}
				}
			case "bool":
				// Bool conversion is handled by YAML parser
			case "string":
				// Additional validation based on field name/pattern
				if err := validateStringField(arg.Key, strVal); err != nil {
					errors = append(errors, err.Error())
				}
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

// validateStringField performs field-specific validation based on field name
func validateStringField(key, value string) error {
	if value == "" {
		return nil
	}

	// Domain validation
	if key == "DOMAIN" {
		return validateDomain(value)
	}

	// IP address validation
	if key == "SERVER_IP" || strings.HasSuffix(key, "_IP") {
		return validateIPAddress(value)
	}

	// URL validation
	if strings.Contains(key, "URL") || strings.Contains(key, "ISSUER") {
		return validateURL(value)
	}

	// Email validation
	if strings.Contains(key, "EMAIL") {
		return validateEmail(value)
	}

	// File path validation (for cert/key files)
	if strings.Contains(key, "CERT") || strings.Contains(key, "KEY") || strings.Contains(key, "_FILE") {
		return validateFilePath(value)
	}

	return nil
}

// validateDomain validates domain name format
func validateDomain(domain string) error {
	if domain == "" {
		return nil
	}

	// Domain regex: alphanumeric with hyphens, dots separate labels
	domainRegex := regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`)

	if !domainRegex.MatchString(domain) {
		return fmt.Errorf("invalid domain format: %s (must contain only letters, numbers, hyphens, and dots; cannot start/end with hyphen or dot)", domain)
	}

	// Additional checks
	if strings.Contains(domain, "..") {
		return fmt.Errorf("invalid domain format: %s (cannot contain consecutive dots)", domain)
	}

	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return fmt.Errorf("invalid domain format: %s (cannot start or end with a dot)", domain)
	}

	if strings.Contains(domain, "_") {
		return fmt.Errorf("invalid domain format: %s (underscores not allowed in domain names)", domain)
	}

	return nil
}

// validateIPAddress validates IP address format
func validateIPAddress(ipStr string) error {
	if ipStr == "" {
		return nil
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return fmt.Errorf("invalid IP address format: %s", ipStr)
	}

	if ip.IsLoopback() {
		return fmt.Errorf("loopback IP address not allowed: %s", ipStr)
	}

	if ip.IsUnspecified() {
		return fmt.Errorf("unspecified IP address (0.0.0.0 or ::) not allowed: %s", ipStr)
	}

	return nil
}

// validateURL validates URL format
func validateURL(urlStr string) error {
	if urlStr == "" {
		return nil
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %v", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: must be http or https, got '%s'", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("invalid URL: missing host")
	}

	return nil
}

// validateEmail validates email address format
func validateEmail(email string) error {
	if email == "" {
		return nil
	}

	// Basic email regex
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format: %s", email)
	}

	return nil
}

// validateFilePath validates file path format
func validateFilePath(path string) error {
	if path == "" {
		return nil
	}

	// Check for empty/whitespace only
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("file path cannot be empty or whitespace only")
	}

	// Additional path validation could be added here
	// (e.g., check for valid characters, absolute path, etc.)

	return nil
}
