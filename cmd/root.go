/**
 * Copyright 2025 Advanced Micro Devices, Inc.  All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
**/

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/silogen/cluster-bloom/pkg"
)

var rootCmd = &cobra.Command{
	Use:   "bloom",
	Short: "Cluster-Bloom creates a cluster",
	Long: `
Cluster-Bloom installs and configures a Kubernetes cluster.
It installs ROCm and other needed settings to prepare a (primarily AMD GPU) node to be part of a Kubernetes cluster,
and ready to be deployed with Cluster-Forge.

By default, running without arguments will start the interactive configuration wizard.
Use --config to specify a configuration file and skip the wizard.

Available Configuration Variables:
  - FIRST_NODE: Set to true if this is the first node in the cluster (default: true).
  - CONTROL_PLANE: Set to true if this node should be a control plane node (default: false, only applies when FIRST_NODE is false).
  - GPU_NODE: Set to true if this node has GPUs (default: true).
  - OIDC_URL: The URL of the OIDC provider (default: "").
  - SERVER_IP: The IP address of the RKE2 server (required for additional nodes).
  - JOIN_TOKEN: The token used to join additional nodes to the cluster (required for additional nodes).
  - SKIP_DISK_CHECK: Set to true to skip disk-related operations (default: false).
  - LONGHORN_DISKS: Comma-separated list of disk paths to use for Longhorn (default: "").
  - CLUSTERFORGE_RELEASE: The version of Cluster-Forge to install (default: "https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz"). Pass the URL for a specific release, or 'none' to not install ClusterForge.
  - DISABLED_STEPS: Comma-separated list of steps to skip. Example "SetupLonghornStep,SetupMetallbStep" (default: "").
  - ENABLED_STEPS: Comma-separated list of steps to perform. If empty, perform all. Example "SetupLonghornStep,SetupMetallbStep" (default: "").
  - SELECTED_DISKS: Comma-separated list of disk devices. Example "/dev/sdb,/dev/sdc" (default: "").
  - DOMAIN: The domain name for the cluster (e.g., "cluster.example.com") (required).
  - USE_CERT_MANAGER: Use cert-manager with Let's Encrypt for automatic TLS certificates (default: false).
  - CERT_OPTION: Certificate option when USE_CERT_MANAGER is false. Choose 'existing' or 'generate' (default: "").
  - TLS_CERT: Path to TLS certificate file for ingress (required if CERT_OPTION is 'existing').
  - TLS_KEY: Path to TLS private key file for ingress (required if CERT_OPTION is 'existing').

Usage:
  Use the --config flag to specify a configuration file, or set the above variables in the environment or a Viper-compatible config file.
`,
	Run: func(cmd *cobra.Command, args []string) {
		// Run wizard by default if no config file is specified
		if cfgFile == "" {
			runWizard()
			return
		}

		log.Debug("Starting package installation")
		pkg.RunStepsWithUI(rootSteps())
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

var cfgFile string

// validateAllURLs validates all URL-type configuration parameters
func validateAllURLs() error {
	urlParams := map[string]string{
		"OIDC_URL":              viper.GetString("OIDC_URL"),
		"CLUSTERFORGE_RELEASE":  viper.GetString("CLUSTERFORGE_RELEASE"),
		"ROCM_BASE_URL":         viper.GetString("ROCM_BASE_URL"),
		"RKE2_INSTALLATION_URL": viper.GetString("RKE2_INSTALLATION_URL"),
	}

	for paramName, urlValue := range urlParams {
		if err := validateURL(urlValue, paramName); err != nil {
			return err
		}
	}

	return nil
}

// validateAllTokens validates all token configuration parameters
func validateAllTokens() error {
	// Validate JOIN_TOKEN if it's required (when FIRST_NODE is false)
	if !viper.GetBool("FIRST_NODE") {
		joinToken := viper.GetString("JOIN_TOKEN")
		if err := validateToken(joinToken, "JOIN_TOKEN"); err != nil {
			return err
		}
	}

	return nil
}

// validStepIDs contains all valid step identifiers
var validStepIDs = []string{
	"CheckUbuntuStep",
	"InstallDependentPackagesStep",
	"OpenPortsStep",
	"CheckPortsBeforeOpeningStep",
	"InstallK8SToolsStep",
	"InotifyInstancesStep",
	"SetupAndCheckRocmStep",
	"SetupRKE2Step",
	"CleanDisksStep",
	"SetupMultipathStep",
	"UpdateModprobeStep",
	"SelectDrivesStep",
	"MountSelectedDrivesStep",
	"GenerateNodeLabelsStep",
	"SetupMetallbStep",
	"SetupLonghornStep",
	"CreateMetalLBConfigStep",
	"PrepareRKE2Step",
	"HasSufficientRancherPartitionStep",
	"NVMEDrivesAvailableStep",
	"SetupKubeConfig",
	"CreateBloomConfigMapStep",
	"CreateDomainConfigStep",
	"SetupClusterForgeStep",
	"FinalOutput",
	"UpdateUdevRulesStep",
	"CleanLonghornMountsStep",
    "CreateChronyConfigStep",
	"UninstallRKE2Step",
}

// validateStepNames validates that step names in DISABLED_STEPS and ENABLED_STEPS are valid
func validateStepNames(stepNames, paramName string) error {
	if stepNames == "" {
		return nil // Empty step lists are allowed
	}

	// Split comma-separated list and validate each step name
	steps := strings.Split(stepNames, ",")
	for _, step := range steps {
		step = strings.TrimSpace(step)
		if step == "" {
			continue // Skip empty entries
		}

		// Check if step name is valid
		valid := false
		for _, validStep := range validStepIDs {
			if step == validStep {
				valid = true
				break
			}
		}

		if !valid {
			return fmt.Errorf("invalid step name '%s' in %s. Valid step names are: %s",
				step, paramName, strings.Join(validStepIDs, ", "))
		}
	}

	return nil
}

// validateAllStepNames validates all step name configuration parameters
func validateAllStepNames() error {
	// Validate DISABLED_STEPS
	if viper.IsSet("DISABLED_STEPS") {
		disabledSteps := viper.GetString("DISABLED_STEPS")
		if err := validateStepNames(disabledSteps, "DISABLED_STEPS"); err != nil {
			return err
		}
	}

	// Validate ENABLED_STEPS
	if viper.IsSet("ENABLED_STEPS") {
		enabledSteps := viper.GetString("ENABLED_STEPS")
		if err := validateStepNames(enabledSteps, "ENABLED_STEPS"); err != nil {
			return err
		}
	}

	return nil
}

// validateConfigurationConflicts detects and warns about conflicting configuration combinations
func validateConfigurationConflicts() error {
	// Check FIRST_NODE=false requires SERVER_IP and JOIN_TOKEN
	if !viper.GetBool("FIRST_NODE") {
		serverIP := viper.GetString("SERVER_IP")
		joinToken := viper.GetString("JOIN_TOKEN")

		if serverIP == "" {
			return fmt.Errorf("when FIRST_NODE=false, SERVER_IP must be provided")
		}
		if joinToken == "" {
			return fmt.Errorf("when FIRST_NODE=false, JOIN_TOKEN must be provided")
		}
	}

	// Check GPU_NODE vs ROCm requirements
	if viper.GetBool("GPU_NODE") {
		// If GPU_NODE is true, we expect ROCm-related configurations to be valid
		rocmBaseURL := viper.GetString("ROCM_BASE_URL")
		if rocmBaseURL == "" {
			log.Warnf("GPU_NODE=true but ROCM_BASE_URL is empty - ROCm installation may fail")
		}

		// Check if SetupAndCheckRocmStep is disabled when GPU_NODE=true
		disabledSteps := viper.GetString("DISABLED_STEPS")
		if strings.Contains(disabledSteps, "SetupAndCheckRocmStep") {
			log.Warnf("GPU_NODE=true but SetupAndCheckRocmStep is disabled - GPU functionality may not work")
		}
	}

	// Check SKIP_DISK_CHECK consistency with disk-related parameters
	skipDiskCheck := viper.GetBool("SKIP_DISK_CHECK")
	longhornDisks := viper.GetString("LONGHORN_DISKS")
	selectedDisks := viper.GetString("SELECTED_DISKS")

	if skipDiskCheck && (longhornDisks != "" || selectedDisks != "") {
		log.Warnf("SKIP_DISK_CHECK=true but disk parameters are set (LONGHORN_DISKS or SELECTED_DISKS) - disk operations will be skipped")
	}

	if !skipDiskCheck && longhornDisks == "" && selectedDisks == "" {
		log.Warnf("SKIP_DISK_CHECK=false but no disk parameters specified - automatic disk detection will be used")
	}

	// Check for conflicting step configurations
	disabledSteps := viper.GetString("DISABLED_STEPS")
	enabledSteps := viper.GetString("ENABLED_STEPS")

	if disabledSteps != "" && enabledSteps != "" {
		// Parse both lists and check for overlaps
		disabled := strings.Split(disabledSteps, ",")
		enabled := strings.Split(enabledSteps, ",")

		for _, disabledStep := range disabled {
			disabledStep = strings.TrimSpace(disabledStep)
			if disabledStep == "" {
				continue
			}

			for _, enabledStep := range enabled {
				enabledStep = strings.TrimSpace(enabledStep)
				if enabledStep == "" {
					continue
				}

				if disabledStep == enabledStep {
					return fmt.Errorf("step '%s' is both enabled and disabled - this is conflicting", disabledStep)
				}
			}
		}
	}

	// Check for essential steps being disabled
	if strings.Contains(disabledSteps, "CheckUbuntuStep") {
		log.Warnf("CheckUbuntuStep is disabled - system compatibility may not be verified")
	}

	if strings.Contains(disabledSteps, "SetupRKE2Step") {
		log.Warnf("SetupRKE2Step is disabled - Kubernetes cluster will not be set up")
	}

	return nil
}

// validateResourceRequirements validates system resource requirements and compatibility
func validateResourceRequirements() error {
	// Validate partition sizes and disk space
	if err := validateDiskSpace(); err != nil {
		return err
	}

	// Validate memory and CPU requirements
	if err := validateSystemResources(); err != nil {
		return err
	}

	// Validate Ubuntu version compatibility
	if err := validateUbuntuVersion(); err != nil {
		return err
	}

	// Validate required kernel modules and drivers (non-fatal warnings)
	validateKernelModules()

	return nil
}

// validateDiskSpace checks partition sizes and available disk space
func validateDiskSpace() error {
	// Check root partition size (minimum 20GB recommended for Kubernetes)
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		log.Warnf("Could not check root partition size: %v", err)
		return nil // Non-fatal
	}

	// Calculate available space in GB
	availableGB := float64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
	totalGB := float64(stat.Blocks*uint64(stat.Bsize)) / (1024 * 1024 * 1024)

	if totalGB < 20 {
		log.Warnf("Root partition size is %.1fGB, recommended minimum is 20GB for Kubernetes", totalGB)
	}

	if availableGB < 10 {
		return fmt.Errorf("insufficient disk space: %.1fGB available, minimum 10GB required", availableGB)
	}

	// Check /var partition if it exists separately
	if err := syscall.Statfs("/var", &stat); err == nil {
		varAvailableGB := float64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
		if varAvailableGB < 5 {
			log.Warnf("/var partition has only %.1fGB available, recommend at least 5GB for container images", varAvailableGB)
		}
	}

	return nil
}

// validateSystemResources checks memory and CPU requirements
func validateSystemResources() error {
	// Check memory requirements (minimum 4GB for Kubernetes)
	memInfo, err := os.Open("/proc/meminfo")
	if err != nil {
		log.Warnf("Could not read memory information: %v", err)
		return nil // Non-fatal
	}
	defer memInfo.Close()

	scanner := bufio.NewScanner(memInfo)
	var totalMemKB int64
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if mem, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
					totalMemKB = mem
					break
				}
			}
		}
	}

	if totalMemKB > 0 {
		totalMemGB := float64(totalMemKB) / (1024 * 1024)
		if totalMemGB < 4 {
			return fmt.Errorf("insufficient memory: %.1fGB available, minimum 4GB required for Kubernetes", totalMemGB)
		}
		if totalMemGB < 8 {
			log.Warnf("Memory is %.1fGB, recommend at least 8GB for optimal performance", totalMemGB)
		}
	}

	// Check CPU count (minimum 2 cores for Kubernetes)
	cpuInfo, err := os.Open("/proc/cpuinfo")
	if err != nil {
		log.Warnf("Could not read CPU information: %v", err)
		return nil // Non-fatal
	}
	defer cpuInfo.Close()

	cpuCount := 0
	scanner = bufio.NewScanner(cpuInfo)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "processor") {
			cpuCount++
		}
	}

	if cpuCount < 2 {
		return fmt.Errorf("insufficient CPU cores: %d available, minimum 2 cores required for Kubernetes", cpuCount)
	}
	if cpuCount < 4 {
		log.Warnf("CPU has %d cores, recommend at least 4 cores for optimal performance", cpuCount)
	}

	return nil
}

// validateUbuntuVersion checks Ubuntu version compatibility
func validateUbuntuVersion() error {
	// Read /etc/os-release for Ubuntu version information
	osRelease, err := os.Open("/etc/os-release")
	if err != nil {
		log.Warnf("Could not read OS release information: %v", err)
		return nil // Non-fatal, might not be Ubuntu
	}
	defer osRelease.Close()

	scanner := bufio.NewScanner(osRelease)
	var distroID, versionID string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			distroID = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		}
		if strings.HasPrefix(line, "VERSION_ID=") {
			versionID = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}

	// Check if it's Ubuntu
	if distroID != "ubuntu" {
		log.Warnf("Not running on Ubuntu (detected: %s) - some features may not work as expected", distroID)
		return nil
	}

	// Validate Ubuntu version (support 20.04, 22.04, 24.04)
	supportedVersions := []string{"20.04", "22.04", "24.04"}
	supported := false
	for _, version := range supportedVersions {
		if versionID == version {
			supported = true
			break
		}
	}

	if !supported {
		log.Warnf("Ubuntu version %s may not be fully supported. Supported versions: %s",
			versionID, strings.Join(supportedVersions, ", "))
	}

	return nil
}

// validateKernelModules checks for required kernel modules and drivers
func validateKernelModules() {
	// Check for required kernel modules (non-fatal, just warnings)
	requiredModules := []string{
		"overlay",      // Required for container runtimes
		"br_netfilter", // Required for Kubernetes networking
	}

	for _, module := range requiredModules {
		if !isModuleLoaded(module) && !isModuleAvailable(module) {
			log.Warnf("Kernel module '%s' is not loaded and may not be available - this could cause issues", module)
		}
	}

	// Check for GPU-related modules if GPU_NODE is true
	if viper.GetBool("GPU_NODE") {
		gpuModules := []string{
			"amdgpu", // AMD GPU driver
		}

		for _, module := range gpuModules {
			if !isModuleLoaded(module) && !isModuleAvailable(module) {
				log.Warnf("GPU module '%s' is not loaded - GPU functionality may not work", module)
			}
		}
	}

	// Check if Docker/containerd can use overlay2 storage driver
	if !isModuleLoaded("overlay") {
		log.Warnf("Overlay filesystem module not loaded - container runtime may fall back to less efficient storage driver")
	}
}

// isModuleLoaded checks if a kernel module is currently loaded
func isModuleLoaded(moduleName string) bool {
	cmd := exec.Command("lsmod")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), moduleName)
}

// isModuleAvailable checks if a kernel module is available to load
func isModuleAvailable(moduleName string) bool {
	cmd := exec.Command("modinfo", moduleName)
	err := cmd.Run()
	return err == nil
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.AddCommand(helpCmd)
}

func initConfig() {
	// Skip validation if running wizard or no config file specified
	if (len(os.Args) > 1 && os.Args[1] == "wizard") || cfgFile == "" {
		return
	}

	if cfgFile != "" {
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			log.Fatalf("Config file does not exist: %s", cfgFile)
		}
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("Could not determine home directory: %v", err)
		}
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".bloom")
	}

	viper.SetDefault("FIRST_NODE", true)
	viper.SetDefault("CONTROL_PLANE", false)
	viper.SetDefault("GPU_NODE", true)
	viper.SetDefault("OIDC_URL", "")
	viper.SetDefault("SKIP_DISK_CHECK", "false")
	viper.SetDefault("LONGHORN_DISKS", "")
	viper.SetDefault("CLUSTERFORGE_RELEASE", "https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz")
	viper.SetDefault("ROCM_BASE_URL", "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/")
	viper.SetDefault("ROCM_DEB_PACKAGE", "amdgpu-install_6.3.60302-1_all.deb")
	viper.SetDefault("RKE2_INSTALLATION_URL", "https://get.rke2.io")
	viper.SetDefault("DISABLED_STEPS", "")
	viper.SetDefault("ENABLED_STEPS", "")
	viper.SetDefault("SELECTED_DISKS", "")
	viper.SetDefault("DOMAIN", "")
	viper.SetDefault("TLS_CERT", "")
	viper.SetDefault("TLS_KEY", "")
	viper.SetDefault("USE_CERT_MANAGER", false)
	viper.SetDefault("CERT_OPTION", "")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err == nil {
		log.Infof("Using config file: %s", viper.ConfigFileUsed())
	}

	if viper.GetBool("FIRST_NODE") { // leaving the loop expecting more default options
		requiredConfigs := []string{"DOMAIN"}
		for _, config := range requiredConfigs {
			if !viper.IsSet(config) || viper.GetString(config) == "" {
				log.Fatalf("Required configuration item '%s' is not set", config)
			}
		}
	}

	if !viper.GetBool("FIRST_NODE") {
		requiredConfigs := []string{"SERVER_IP", "JOIN_TOKEN"}
		for _, config := range requiredConfigs {
			if !viper.IsSet(config) {
				log.Fatalf("Required configuration item '%s' is not set", config)
			}
		}
	}

	// Validate TLS configuration on FIRST_NODE when USE_CERT_MANAGER is false
	if viper.GetBool("FIRST_NODE") {
		if !viper.GetBool("USE_CERT_MANAGER") {
			certOption := viper.GetString("CERT_OPTION")

			// Only validate TLS_CERT and TLS_KEY if using existing certificates
			if certOption == "existing" {
				tlsCert := viper.GetString("TLS_CERT")
				tlsKey := viper.GetString("TLS_KEY")

				if tlsCert == "" || tlsKey == "" {
					log.Fatalf("When CERT_OPTION is 'existing', both TLS_CERT and TLS_KEY must be provided")
				}

				// Verify the files exist
				if _, err := os.Stat(tlsCert); os.IsNotExist(err) {
					log.Fatalf("TLS_CERT file does not exist: %s", tlsCert)
				}
				if _, err := os.Stat(tlsKey); os.IsNotExist(err) {
					log.Fatalf("TLS_KEY file does not exist: %s", tlsKey)
				}
			} else if certOption == "generate" {
				// No validation needed for generate option - certificates will be created automatically
				log.Println("Self-signed certificates will be generated during setup")
			} else if certOption != "" {
				log.Fatalf("Invalid CERT_OPTION value: %s. Must be 'existing' or 'generate'", certOption)
			} else {
				log.Fatalf("When USE_CERT_MANAGER is false, CERT_OPTION must be set to either 'existing' or 'generate'")
			}
		}
	}

	if viper.IsSet("LONGHORN_DISKS") && viper.GetString("LONGHORN_DISKS") != "" {
		longhornDiskString := pkg.ParseLonghornDiskConfig()
		if len(longhornDiskString) > 63 {
			log.Fatalf("Too many disks, %s is longer than 63", pkg.ParseLonghornDiskConfig())
		}
	}

	// Validate URL parameters
	if err := validateAllURLs(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Validate IP address parameters (SERVER_IP if required)
	if !viper.GetBool("FIRST_NODE") {
		serverIP := viper.GetString("SERVER_IP")
		if err := validateIPAddress(serverIP, "SERVER_IP"); err != nil {
			log.Fatalf("Configuration validation failed: %v", err)
		}
	}

	// Validate token parameters
	if err := validateAllTokens(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Validate step name parameters
	if err := validateAllStepNames(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Validate configuration conflicts
	if err := validateConfigurationConflicts(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Validate system resource requirements
	if err := validateResourceRequirements(); err != nil {
		log.Fatalf("System requirements validation failed: %v", err)
	}

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	currentDir, err := os.Getwd()
	if err != nil {
		log.Warnf("Could not determine current directory: %v", err)
		return
	}

	logPath := filepath.Join(currentDir, "bloom.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Warnf("Could not open log file: %v", err)
		return
	}
	log.SetOutput(logFile)
	logConfigValues()
}

func logConfigValues() {
	log.Info("Configuration values:")
	for _, key := range viper.AllKeys() {
		value := viper.Get(key)
		if key == "join_token" {
			value = "---redacted---"
		}
		log.Infof("%s: %v", key, value)
	}
}

func rootSteps() []pkg.Step {
	preK8Ssteps := []pkg.Step{
		pkg.CheckUbuntuStep,
		pkg.HasSufficientRancherPartitionStep,
		pkg.NVMEDrivesAvailableStep,
		pkg.InstallDependentPackagesStep,
		pkg.CleanLonghornMountsStep,
		pkg.UninstallRKE2Step,
		pkg.CleanDisksStep,
		pkg.SetupMultipathStep,
		pkg.UpdateModprobeStep,
		pkg.SelectDrivesStep,
		pkg.MountSelectedDrivesStep,
		pkg.PrepareRKE2Step,
		pkg.GenerateNodeLabelsStep,
		pkg.InstallK8SToolsStep,
		pkg.InotifyInstancesStep,
		pkg.SetupAndCheckRocmStep,
		pkg.OpenPortsStep,
		pkg.UpdateUdevRulesStep,
	}
	k8Ssteps := []pkg.Step{
		pkg.SetupRKE2Step,
	}
	postK8Ssteps := []pkg.Step{
		pkg.CreateChronyConfigStep,
		pkg.SetupLonghornStep,
		pkg.SetupMetallbStep,
		pkg.CreateMetalLBConfigStep,
		pkg.SetupKubeConfig,
		pkg.CreateDomainConfigStep,
		pkg.CreateBloomConfigMapStep,
		pkg.SetupClusterForgeStep,
	}

	postK8Ssteps = append(postK8Ssteps, pkg.FinalOutput)
	combinedSteps := append(append(preK8Ssteps, k8Ssteps...), postK8Ssteps...)
	return combinedSteps
}

func displayHelp() {
	helpContent := `
Cluster-Bloom Help:

Available Configuration Variables:
  - FIRST_NODE: Set to true if this is the first node in the cluster (default: true).
  - CONTROL_PLANE: Set to true if this node should be a control plane node (default: false, only applies when FIRST_NODE is false).
  - GPU_NODE: Set to true if this node has GPUs (default: true).
  - OIDC_URL: The URL of the OIDC provider (default: "").
  - SERVER_IP: The IP address of the RKE2 server (required for additional nodes).
  - JOIN_TOKEN: The token used to join additional nodes to the cluster (required for additional nodes).
  - SKIP_DISK_CHECK: Set to true to skip disk-related operations (default: false).
  - LONGHORN_DISKS: Comma-separated list of disk paths to use for Longhorn (default: "").
  - CLUSTERFORGE_RELEASE: The version of Cluster-Forge to install (default: "https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz"). Pass the URL for a specific release, or 'none' to not install ClusterForge.
  - DISABLED_STEPS: Comma-separated list of steps to skip. Example "SetupLonghornStep,SetupMetallbStep" (default: "").
  - ENABLED_STEPS: Comma-separated list of steps to perform. If empty, perform all. Example "SetupLonghornStep,SetupMetallbStep" (default: "").
  - SELECTED_DISKS: Comma-separated list of disk devices. Example "/dev/sdb,/dev/sdc" (default: "").
  - DOMAIN: The domain name for the cluster (e.g., "cluster.example.com") (required).
  - USE_CERT_MANAGER: Use cert-manager with Let's Encrypt for automatic TLS certificates (default: false).
  - CERT_OPTION: Certificate option when USE_CERT_MANAGER is false. Choose 'existing' or 'generate' (default: "").
  - TLS_CERT: Path to TLS certificate file for ingress (required if CERT_OPTION is 'existing').
  - TLS_KEY: Path to TLS private key file for ingress (required if CERT_OPTION is 'existing').

Usage:
  Use the --config flag to specify a configuration file, or set the above variables in the environment or a Viper-compatible config file.
`
	fmt.Println(helpContent)
}

var helpCmd = &cobra.Command{
	Use:   "help",
	Short: "Display help information",
	Run: func(cmd *cobra.Command, args []string) {
		displayHelp()
	},
}
