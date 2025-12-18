package args

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/silogen/cluster-bloom/pkg/mockablecmd"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type Arg struct {
	Key          string
	Default      interface{}
	Description  string
	Type         string
	Options      []string
	Dependencies string // Comma-separated conditions like "GPU_NODE=true,CERT_OPTION=existing"
	Validators   []func(value string) error
}

var allStepIDs []string

func SetAllSteps(stepIDs []string) {
	allStepIDs = stepIDs
}

func GetAllStepIDs() []string {
	return allStepIDs
}

var Arguments []Arg

func SetArguments(args []Arg) {
	Arguments = args
}

// parseDependency parses a single dependency string like "GPU_NODE=true" or "CERT_OPTION=existing"
func parseDependency(depStr string) (argName string, expectedValue string, ok bool) {
	parts := strings.SplitN(depStr, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

// evaluateDependency checks if a dependency condition is met
func evaluateDependency(depStr string) bool {
	argName, expectedValue, ok := parseDependency(depStr)
	if !ok {
		return false
	}

	// Check boolean values
	if expectedValue == "true" {
		return viper.GetBool(argName)
	}
	if expectedValue == "false" {
		return !viper.GetBool(argName)
	}

	// Check string values
	return viper.GetString(argName) == expectedValue
}

func IsArgUsed(arg Arg) bool {
	if arg.Dependencies == "" {
		return true
	}

	// Split by comma and evaluate each dependency
	deps := strings.Split(arg.Dependencies, ",")
	for _, dep := range deps {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		if !evaluateDependency(dep) {
			return false
		}
	}
	return true
}

// ValidateJoinTokenArg validates RKE2/K3s join token format
func ValidateJoinTokenArg(token string) error {
	// RKE2/K3s tokens are typically:
	// - Base64-encoded or hex strings
	// - Usually 64+ characters long
	// - Contain alphanumeric characters, +, /, =

	// Empty tokens are handled by validateToken function
	if token == "" {
		return nil
	}

	if len(token) < 32 {
		return fmt.Errorf("JOIN_TOKEN is too short (minimum 32 characters), got %d characters", len(token))
	}

	if len(token) > 512 {
		return fmt.Errorf("JOIN_TOKEN is too long (maximum 512 characters), got %d characters", len(token))
	}

	// Allow base64 characters, hex characters, and common separators including colons
	validTokenPattern := regexp.MustCompile(`^[a-zA-Z0-9+/=_.:-]+$`)
	if !validTokenPattern.MatchString(token) {
		return fmt.Errorf("JOIN_TOKEN contains invalid characters (only alphanumeric, +, /, =, _, ., :, - allowed)")
	}

	return nil
}

func ValidateListOfHostnames(hostnames string) error {
	if hostnames == "" {
		return nil // Empty input is allowed
	}

	hostNameList := strings.Split(hostnames, ",")
	for _, hostNameStr := range hostNameList {
		hostNameStr = strings.TrimSpace(hostNameStr)
		if hostNameStr == "" {
			continue // Skip empty entries
		}
		NotValidIPErr := ValidateIPAddress(hostNameStr);
		NotValidHostnameErr := ValidateHostname(hostNameStr);

		if NotValidHostnameErr != nil && NotValidIPErr != nil {
			if NotValidHostnameErr != nil {
				return fmt.Errorf("invalid hostname in TLS SAN '%s': %v", hostNameStr, NotValidHostnameErr)
			}
			if NotValidIPErr != nil {
				return fmt.Errorf("invalid IP address in TLS SAN '%s': %v", hostNameStr, NotValidIPErr)
			}
		}
	}
	return nil
}

// ValidateStepNamesArg validates that step names are valid against the steps from rootSteps
func ValidateStepNamesArg(stepNames string) error {
	if stepNames == "" {
		return nil // Empty step lists are allowed
	}

	// Split comma-separated list and validate each step name
	inputSteps := strings.Split(stepNames, ",")
	for _, inputStep := range inputSteps {
		inputStep = strings.TrimSpace(inputStep)
		if inputStep == "" {
			continue // Skip empty entries
		}

		// Check if step name is valid
		valid := false
		for _, validStep := range allStepIDs {
			if inputStep == validStep {
				valid = true
				break
			}
		}

		if !valid {
			return fmt.Errorf("invalid step name '%s'. Valid step names are: %s",
				inputStep, strings.Join(allStepIDs, ", "))
		}
	}

	return nil
}

// ValidateDisabledStepsWarnings warns about disabling essential steps
func ValidateDisabledStepsWarnings(stepNames string) error {
	if stepNames == "" {
		return nil
	}

	// Check for essential steps being disabled
	if strings.Contains(stepNames, "CheckUbuntuStep") {
		log.Warnf("CheckUbuntuStep is disabled - system compatibility may not be verified")
	}

	if strings.Contains(stepNames, "SetupRKE2Step") {
		log.Warnf("SetupRKE2Step is disabled - Kubernetes cluster will not be set up")
	}

	// Check if SetupAndCheckRocmStep is disabled when GPU_NODE=true
	if strings.Contains(stepNames, "SetupAndCheckRocmStep") && viper.GetBool("GPU_NODE") {
		log.Warnf("GPU_NODE=true but SetupAndCheckRocmStep is disabled - GPU functionality may not work")
	}

	return nil
}

// ValidateDisabledStepsConflict ensures DISABLED_STEPS and ENABLED_STEPS are not both set
func ValidateDisabledStepsConflict(stepNames string) error {
	if stepNames == "" {
		return nil
	}

	enabledSteps := viper.GetString("ENABLED_STEPS")
	if enabledSteps != "" {
		return fmt.Errorf("DISABLED_STEPS and ENABLED_STEPS cannot both be set - use one or the other")
	}

	return nil
}

// ValidateSkipDiskCheckConsistency warns about inconsistencies with NO_DISKS_FOR_CLUSTER
func ValidateSkipDiskCheckConsistency(skipDiskCheckStr string) error {
	skipDiskCheck := viper.GetBool("NO_DISKS_FOR_CLUSTER")
	longhornDisks := viper.GetString("CLUSTER_PREMOUNTED_DISKS")
	selectedDisks := viper.GetString("CLUSTER_DISKS")

	if skipDiskCheck && (longhornDisks != "" || selectedDisks != "") {
		log.Warnf("NO_DISKS_FOR_CLUSTER=true but disk parameters are set (CLUSTER_PREMOUNTED_DISKS or CLUSTER_DISKS) - disk operations will be skipped")
	}

	if !skipDiskCheck && longhornDisks == "" && selectedDisks == "" {
		log.Warnf("NO_DISKS_FOR_CLUSTER=false but no disk parameters specified - automatic disk detection will be used")
	}

	return nil
}

// validatePremountedNotBloomManaged checks that CLUSTER_PREMOUNTED_DISKS paths are not bloom-managed in /etc/fstab
func validatePremountedNotBloomManaged(diskList []string) error {
	const bloomFstabTag = "# managed by cluster-bloom"

	fstabContent, err := mockablecmd.ReadFile("ValidateArgs.ReadFstab", "/etc/fstab")
	if err != nil {
		// If we can't read fstab, warn but don't fail validation
		log.Warnf("Could not read /etc/fstab to validate CLUSTER_PREMOUNTED_DISKS: %v", err)
		return nil
	}

	lines := strings.Split(string(fstabContent), "\n")
	for lineNum, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip lines that aren't bloom-managed
		if !strings.HasSuffix(trimmedLine, bloomFstabTag) {
			continue
		}

		// Extract mount point from bloom-managed entry
		fields := strings.Fields(trimmedLine)
		if len(fields) < 2 {
			continue
		}

		mountPoint := fields[1]

		// Check if this mount point is in CLUSTER_PREMOUNTED_DISKS
		for _, disk := range diskList {
			disk = strings.TrimSpace(disk)
			if disk == mountPoint {
				return fmt.Errorf("CLUSTER_PREMOUNTED_DISKS contains %s which is tagged '# managed by cluster-bloom' in /etc/fstab at line %d:\n  %s\nPlease delete the tag from the fstab line first or use a different mount point", disk, lineNum+1, trimmedLine)
			}
		}
	}

	return nil
}

// ValidateLonghornDisksArg validates CLUSTER_PREMOUNTED_DISKS configuration
func ValidateLonghornDisksArg(disks string) error {
	selectedDisks := viper.GetString("CLUSTER_DISKS")

	// validate format of disks (comma-separated list) and each is an absolute path which exists
	clusterPremountedDisks := viper.GetString("CLUSTER_PREMOUNTED_DISKS")
	if clusterPremountedDisks != "" {
		diskList := strings.Split(clusterPremountedDisks, ",")
		for _, disk := range diskList {
			disk = strings.TrimSpace(disk)
			if disk == "" {
				continue
			}
			if !filepath.IsAbs(disk) {
				return fmt.Errorf("CLUSTER_PREMOUNTED_DISKS contains a non-absolute path: %s", disk)
			}
			if _, err := os.Stat(disk); os.IsNotExist(err) {
				return fmt.Errorf("CLUSTER_PREMOUNTED_DISKS contains a path that does not exist: %s", disk)
			}
		}

		// Check that none of the CLUSTER_PREMOUNTED_DISKS are bloom-managed in /etc/fstab
		if err := validatePremountedNotBloomManaged(diskList); err != nil {
			return err
		}
	}

	// Both cannot be set
	if disks != "" && selectedDisks != "" {
		return fmt.Errorf("CLUSTER_PREMOUNTED_DISKS and CLUSTER_DISKS cannot both be set - use one or the other")
	}

	// At least one must be set
	if disks == "" && selectedDisks == "" {
		return fmt.Errorf("either CLUSTER_PREMOUNTED_DISKS or CLUSTER_DISKS must be set")
	}

	return nil
}

// ValidateYAMLFormat validates that the provided string is valid YAML
func ValidateYAMLFormat(yamlStr string) error {
	if yamlStr == "" {
		return nil
	}

	var data interface{}
	if err := yaml.Unmarshal([]byte(yamlStr), &data); err != nil {
		return fmt.Errorf("invalid YAML format: %v", err)
	}

	if data == nil {
		return fmt.Errorf("YAML content cannot be empty or null")
	}

	return nil
}

func GenerateArgsHelp() string {
	var helpLines []string

	for _, arg := range Arguments {
		// Format: - KEY: Description (default: value).
		defaultStr := fmt.Sprintf("%v", arg.Default)
		if arg.Type == "string" || arg.Type == "non-empty-string" {
			defaultStr = fmt.Sprintf("\"%s\"", defaultStr)
		}

		helpLine := fmt.Sprintf("  - %s: %s (default: %s).", arg.Key, arg.Description, defaultStr)
		helpLines = append(helpLines, helpLine)
	}

	return strings.Join(helpLines, "\n")
}

// ValidateIPAddress validates an IP address string
func ValidateIPAddress(ipStr string) error {
	if ipStr == "" {
		return nil // Empty IPs are allowed for optional parameters
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", ipStr)
	}

	if ip.IsLoopback() {
		return fmt.Errorf("loopback IP address not allowed: %s", ipStr)
	}

	if ip.IsUnspecified() {
		return fmt.Errorf("unspecified IP address (0.0.0.0 or ::) not allowed: %s", ipStr)
	}

	return nil
}

// ValidateURL validates a URL string
func ValidateURL(urlStr string) error {
	if urlStr == "" {
		return nil // Empty URLs are allowed for optional parameters
	}

	// Handle special case for CLUSTERFORGE_RELEASE
	if strings.ToLower(urlStr) == "none" {
		return nil
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %v", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: must be http or https, got %s", parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("invalid URL: missing host")
	}

	return nil
}

// ValidateHostname validates a hostname
func ValidateHostname(hostnameStr string) error {
	if hostnameStr == "" {
		return nil // Empty hostnames are allowed for optional parameters
	}

	if strings.ToLower(hostnameStr) == "none" {
		return nil
	}

	_, err := url.Parse(hostnameStr)
	if err != nil {
		return fmt.Errorf("invalid hostname format: %v", err)
	}

	return nil
}
// ValidateToken validates a token string (currently supports JOIN_TOKEN format)
func ValidateToken(token string) error {
	return ValidateJoinTokenArg(token)
}

// ValidateBool validates a boolean input string
func ValidateBool(input string) error {
	lower := strings.ToLower(strings.TrimSpace(input))
	validValues := []string{"true", "false", "t", "f", "yes", "no", "y", "n", "1", "0"}
	for _, v := range validValues {
		if lower == v {
			return nil
		}
	}
	return fmt.Errorf("invalid boolean value. Please enter: true/false, yes/no, y/n, or 1/0")
}

// ValidateDeprecatedArgs checks if any deprecated arguments are being used
func ValidateDeprecatedArgs() error {
	var errors []string

	// Map of old argument names to new argument names
	deprecatedArgs := map[string]string{
		"SKIP_DISK_CHECK": "NO_DISKS_FOR_CLUSTER",
		"LONGHORN_DISKS":  "CLUSTER_PREMOUNTED_DISKS",
		"SELECTED_DISKS":  "CLUSTER_DISKS",
	}

	// Check if any deprecated arguments are set in viper
	for oldArg, newArg := range deprecatedArgs {
		if viper.IsSet(oldArg) {
			errors = append(errors, fmt.Sprintf("argument '%s' has been renamed to '%s'. Please update your configuration to use '%s' instead", oldArg, newArg, newArg))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("deprecated arguments detected:\n- %s", strings.Join(errors, "\n- "))
	}

	return nil
}

func ValidateArgs() error {
	var errors []string

	// Check for deprecated arguments first
	if err := ValidateDeprecatedArgs(); err != nil {
		return err
	}

	for _, arg := range Arguments {
		value := viper.GetString(arg.Key)

		// Check if this argument is needed based on its dependencies

		if !IsArgUsed(arg) {
			continue
		}

		// Check for non-empty prefix
		required := strings.HasPrefix(arg.Type, "non-empty-")
		baseType := arg.Type
		if required {
			baseType = strings.TrimPrefix(arg.Type, "non-empty-")
		}

		// Type-specific validation
		switch baseType {
		case "bool":
			// viper.GetBool handles string-to-bool conversion, so we're good
		case "url":
			if err := ValidateURL(value); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", arg.Key, err))
			}
		case "file":
			if value != "" {
				if !filepath.IsAbs(value) {
					errors = append(errors, fmt.Sprintf("%s: must be an absolute file path", arg.Key))
				}
				if _, err := os.Stat(value); os.IsNotExist(err) {
					errors = append(errors, fmt.Sprintf("%s: file does not exist: %s", arg.Key, value))
				}
			}
		case "enum":
			if len(arg.Options) > 0 {
				validOption := false
				for _, option := range arg.Options {
					if value == option {
						validOption = true
						break
					}
				}
				if !validOption {
					errors = append(errors, fmt.Sprintf("%s: must be one of %v, got: %s", arg.Key, arg.Options, value))
				}
			}
		case "ip-address":
			if err := ValidateIPAddress(value); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", arg.Key, err))
			}
		case "string":
			// Basic string validation can be added here if needed
		}

		// Run custom validators if provided
		for _, validator := range arg.Validators {
			if err := validator(value); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", arg.Key, err))
			}
		}

		// Check if field is required and empty
		if required && value == "" {
			errors = append(errors, fmt.Sprintf("%s is required", arg.Key))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n- %s", strings.Join(errors, "\n- "))
	}

	return nil
}
