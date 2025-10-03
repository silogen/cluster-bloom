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
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/silogen/cluster-bloom/pkg"
)

var rootCmd = &cobra.Command{
	Use:   "bloom",
	Short: "Cluster-Bloom creates a cluster",
	Long: `
Cluster-Bloom installs and configures a Kubernetes cluster.
It installs ROCm and other needed settings to prepare a (primarily AMD GPU) node to be part of a Kubernetes cluster,
and ready to be deployed with Cluster-Forge.

By default, running without arguments will:
- Start the web-based configuration interface if no bloom.log exists
- Display status and start monitoring interface if bloom.log exists

Use --config to specify a configuration file that will pre-fill the web interface.
Use --one-shot with --config to auto-proceed after loading configuration (useful for automation).
Use --reconfigure to archive existing bloom.log and start fresh configuration.
Use 'bloom cli --config <file>' for terminal-only mode.

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
  Use the --config flag to specify a configuration file that will pre-fill the web interface, or set the above variables in the environment or a Viper-compatible config file.
  Use --one-shot with --config to auto-proceed after loading configuration for automated deployments.
`,
	Run: func(cmd *cobra.Command, args []string) {
		if reconfigure {
			currentDir, _ := os.Getwd()
			logPath := filepath.Join(currentDir, "bloom.log")

			if _, err := os.Stat(logPath); err == nil {
				timestamp := time.Now().Format("20060102-150405")
				archivedPath := filepath.Join(currentDir, fmt.Sprintf("bloom-%s.log", timestamp))

				if err := os.Rename(logPath, archivedPath); err != nil {
					fmt.Printf("❌ Failed to archive bloom.log: %v\n", err)
					os.Exit(1)
				}

				fmt.Printf("✅ Archived bloom.log to %s\n", filepath.Base(archivedPath))
				fmt.Println("🚀 Starting fresh configuration...")
				fmt.Println()
			}
			runWebInterfaceWithConfig()
			return
		}

		if cfgFile == "" {
			currentDir, _ := os.Getwd()
			logPath := filepath.Join(currentDir, "bloom.log")
			if _, err := os.Stat(logPath); err == nil {
				fmt.Println("🔍 Found existing bloom.log - starting monitoring interface...")
				fmt.Println()
				startWebUIMonitoring()
				return
			}
		}

		runWebInterfaceWithConfig()
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

var cfgFile string
var oneShot bool
var reconfigure bool

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

func validateAllTokens() error {
	if !viper.GetBool("FIRST_NODE") {
		joinToken := viper.GetString("JOIN_TOKEN")
		if err := validateToken(joinToken, "JOIN_TOKEN"); err != nil {
			return err
		}
	}

	return nil
}

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

func validateStepNames(stepNames, paramName string) error {
	if stepNames == "" {
		return nil
	}

	steps := strings.Split(stepNames, ",")
	for _, step := range steps {
		step = strings.TrimSpace(step)
		if step == "" {
			continue
		}

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

func validateAllStepNames() error {
	if viper.IsSet("DISABLED_STEPS") {
		disabledSteps := viper.GetString("DISABLED_STEPS")
		if err := validateStepNames(disabledSteps, "DISABLED_STEPS"); err != nil {
			return err
		}
	}

	if viper.IsSet("ENABLED_STEPS") {
		enabledSteps := viper.GetString("ENABLED_STEPS")
		if err := validateStepNames(enabledSteps, "ENABLED_STEPS"); err != nil {
			return err
		}
	}

	return nil
}

func validateConfigurationConflicts() error {
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

	if viper.GetBool("GPU_NODE") {
		rocmBaseURL := viper.GetString("ROCM_BASE_URL")
		if rocmBaseURL == "" {
			log.Warnf("GPU_NODE=true but ROCM_BASE_URL is empty - ROCm installation may fail")
		}

		disabledSteps := viper.GetString("DISABLED_STEPS")
		if strings.Contains(disabledSteps, "SetupAndCheckRocmStep") {
			log.Warnf("GPU_NODE=true but SetupAndCheckRocmStep is disabled - GPU functionality may not work")
		}
	}

	skipDiskCheck := viper.GetBool("SKIP_DISK_CHECK")
	longhornDisks := viper.GetString("LONGHORN_DISKS")
	selectedDisks := viper.GetString("SELECTED_DISKS")

	if skipDiskCheck && (longhornDisks != "" || selectedDisks != "") {
		log.Warnf("SKIP_DISK_CHECK=true but disk parameters are set (LONGHORN_DISKS or SELECTED_DISKS) - disk operations will be skipped")
	}

	if !skipDiskCheck && longhornDisks == "" && selectedDisks == "" {
		log.Warnf("SKIP_DISK_CHECK=false but no disk parameters specified - automatic disk detection will be used")
	}

	disabledSteps := viper.GetString("DISABLED_STEPS")
	enabledSteps := viper.GetString("ENABLED_STEPS")

	if disabledSteps != "" && enabledSteps != "" {
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

	if strings.Contains(disabledSteps, "CheckUbuntuStep") {
		log.Warnf("CheckUbuntuStep is disabled - system compatibility may not be verified")
	}

	if strings.Contains(disabledSteps, "SetupRKE2Step") {
		log.Warnf("SetupRKE2Step is disabled - Kubernetes cluster will not be set up")
	}

	return nil
}

func validateResourceRequirements() error {
	if err := validateDiskSpace(); err != nil {
		return err
	}

	if err := validateSystemResources(); err != nil {
		return err
	}

	if err := validateUbuntuVersion(); err != nil {
		return err
	}

	validateKernelModules()

	return nil
}

func validateDiskSpace() error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		log.Warnf("Could not check root partition size: %v", err)
		return nil
	}

	availableGB := float64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
	totalGB := float64(stat.Blocks*uint64(stat.Bsize)) / (1024 * 1024 * 1024)

	if totalGB < 20 {
		log.Warnf("Root partition size is %.1fGB, recommended minimum is 20GB for Kubernetes", totalGB)
	}

	if availableGB < 10 {
		return fmt.Errorf("insufficient disk space: %.1fGB available, minimum 10GB required", availableGB)
	}

	if err := syscall.Statfs("/var", &stat); err == nil {
		varAvailableGB := float64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
		if varAvailableGB < 5 {
			log.Warnf("/var partition has only %.1fGB available, recommend at least 5GB for container images", varAvailableGB)
		}
	}

	return nil
}

func validateSystemResources() error {
	memInfo, err := os.Open("/proc/meminfo")
	if err != nil {
		log.Warnf("Could not read memory information: %v", err)
		return nil
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

	cpuInfo, err := os.Open("/proc/cpuinfo")
	if err != nil {
		log.Warnf("Could not read CPU information: %v", err)
		return nil
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

func validateUbuntuVersion() error {
	osRelease, err := os.Open("/etc/os-release")
	if err != nil {
		log.Warnf("Could not read OS release information: %v", err)
		return nil
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

	if distroID != "ubuntu" {
		log.Warnf("Not running on Ubuntu (detected: %s) - some features may not work as expected", distroID)
		return nil
	}

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

func validateKernelModules() {
	requiredModules := []string{
		"overlay",      // Required for container runtimes
		"br_netfilter", // Required for Kubernetes networking
	}

	for _, module := range requiredModules {
		if !isModuleLoaded(module) && !isModuleAvailable(module) {
			log.Warnf("Kernel module '%s' is not loaded and may not be available - this could cause issues", module)
		}
	}

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

	if !isModuleLoaded("overlay") {
		log.Warnf("Overlay filesystem module not loaded - container runtime may fall back to less efficient storage driver")
	}
}

func isModuleLoaded(moduleName string) bool {
	cmd := exec.Command("lsmod")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), moduleName)
}

func isModuleAvailable(moduleName string) bool {
	cmd := exec.Command("modinfo", moduleName)
	err := cmd.Run()
	return err == nil
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&oneShot, "one-shot", false, "skip confirmation when using --config (useful for automation)")
	rootCmd.PersistentFlags().BoolVar(&reconfigure, "reconfigure", false, "archive existing bloom.log and start fresh configuration")
	rootCmd.AddCommand(helpCmd)
	rootCmd.AddCommand(cliCmd)
}

func initConfig() {
	if cfgFile == "" {
		return
	}

	setupLogging()

	if cfgFile != "" {
		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			log.Fatalf("Config file does not exist: %s", cfgFile)
		}
		viper.SetConfigFile(cfgFile)
	} else {
		if _, err := os.Stat("bloom.yaml"); err == nil {
			viper.SetConfigFile("bloom.yaml")
			log.Info("Using config file: bloom.yaml")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				log.Fatalf("Could not determine home directory: %v", err)
			}
			viper.AddConfigPath(home)
			viper.SetConfigType("yaml")
			viper.SetConfigName(".bloom")
		}
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

	logConfigValues()

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

	if viper.GetBool("FIRST_NODE") {
		if !viper.GetBool("USE_CERT_MANAGER") {
			certOption := viper.GetString("CERT_OPTION")

			if certOption == "existing" {
				tlsCert := viper.GetString("TLS_CERT")
				tlsKey := viper.GetString("TLS_KEY")

				if tlsCert == "" || tlsKey == "" {
					log.Fatalf("When CERT_OPTION is 'existing', both TLS_CERT and TLS_KEY must be provided")
				}

				if _, err := os.Stat(tlsCert); os.IsNotExist(err) {
					log.Fatalf("TLS_CERT file does not exist: %s", tlsCert)
				}
				if _, err := os.Stat(tlsKey); os.IsNotExist(err) {
					log.Fatalf("TLS_KEY file does not exist: %s", tlsKey)
				}
			} else if certOption == "generate" {
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

	if err := validateAllURLs(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	if !viper.GetBool("FIRST_NODE") {
		serverIP := viper.GetString("SERVER_IP")
		if err := validateIPAddress(serverIP, "SERVER_IP"); err != nil {
			log.Fatalf("Configuration validation failed: %v", err)
		}
	}

	if err := validateAllTokens(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	if err := validateAllStepNames(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	if err := validateConfigurationConflicts(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	if err := validateResourceRequirements(); err != nil {
		log.Fatalf("System requirements validation failed: %v", err)
	}
}

func setupLogging() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not determine current directory: %v\n", err)
		return
	}

	logPath := filepath.Join(currentDir, "bloom.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open log file: %v\n", err)
		return
	}
	log.SetOutput(logFile)
}

func logConfigValues() {
	log.Info("Configuration values:")
	allKeys := viper.AllKeys()
	if len(allKeys) == 0 {
		log.Warn("No configuration values found in viper")
	} else {
		for _, key := range allKeys {
			value := viper.Get(key)
			if key == "join_token" {
				value = "---redacted---"
			}
			log.Infof("%s: %v", key, value)
		}
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
		pkg.CreateBloomConfigMapStepFunc(Version),
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
  Use the --config flag to specify a configuration file that will pre-fill the web interface, or set the above variables in the environment or a Viper-compatible config file.
  Use --one-shot with --config to auto-proceed after loading configuration for automated deployments.
`
	fmt.Println(helpContent)
}

func findAvailablePort(startPort int) int {
	for port := startPort; port < startPort+100; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			ln.Close()
			return port
		}
	}
	return startPort // fallback to original port if nothing available
}


func startWebUIMonitoring() {
	setupLogging()

	pkg.CheckAndDisplayExistingStatus()

	currentDir, _ := os.Getwd()
	logPath := filepath.Join(currentDir, "bloom.log")
	yamlPath := filepath.Join(currentDir, "bloom.yaml")

	portNum := findAvailablePort(62078)
	port := fmt.Sprintf(":%d", portNum)
	url := fmt.Sprintf("http://127.0.0.1%s", port)

	fmt.Println()
	fmt.Printf("🌐 Starting web monitoring interface on %s\n", url)
	fmt.Println("📊 Monitoring existing bloom.log file")
	fmt.Printf("🔧 View detailed status at %s\n", url)
	fmt.Println()
	fmt.Println("💡 To run a new installation instead, use:")
	fmt.Println("   bloom --config <config-file>")
	fmt.Println()

	monitor := pkg.NewWebMonitor()
	pkg.SetGlobalWebMonitor(monitor)

	var status *pkg.BloomStatus
	if parsedStatus, err := pkg.ParseBloomLog(logPath); err == nil {
		status = parsedStatus
		allSteps := rootSteps()
		enabledSteps := pkg.CalculateEnabledSteps(allSteps)
		for i, step := range enabledSteps {
			monitor.InitializeStep(step, i+1)
		}

		stepNameToID := make(map[string]string)
		for _, step := range enabledSteps {
			stepNameToID[step.Name] = step.Id
		}

		for _, step := range status.Steps {
			stepID := stepNameToID[step.Name]
			if stepID == "" {
				stepID = step.Name
			}

			monitor.AddLog("INFO", fmt.Sprintf("Starting step: %s", step.Name), stepID)

			switch step.Status {
			case "completed":
				monitor.StartStep(stepID)
				monitor.CompleteStep(stepID, nil)
				monitor.AddLog("INFO", fmt.Sprintf("Step %s completed", step.Name), stepID)
			case "failed":
				monitor.StartStep(stepID)
				if step.Error != "" {
					monitor.AddLog("ERROR", step.Error, stepID)
				}
				monitor.CompleteStep(stepID, fmt.Errorf(step.Error))
			case "skipped":
				monitor.SkipStep(stepID)
				monitor.AddLog("INFO", fmt.Sprintf("Step %s is skipped", step.Name), stepID)
			case "running":
				monitor.StartStep(stepID)
			}
		}

		for _, errMsg := range status.Errors {
			monitor.AddLog("ERROR", errMsg, "system")
		}

		if status.OSError != "" {
			monitor.AddLog("ERROR", status.OSError, "system")
		}

		monitor.SetVariable("domain", status.Domain)
		monitor.SetVariable("first_node", fmt.Sprintf("%v", status.FirstNode))
		monitor.SetVariable("gpu_node", fmt.Sprintf("%v", status.GPUNode))
		monitor.SetVariable("total_steps", len(enabledSteps))

		for key, value := range status.ConfigValues {
			if value != "" {
				monitor.SetVariable(key, value)
			}
		}

		hasErrors := len(status.Errors) > 0
		for _, step := range status.Steps {
			if step.Status == "failed" {
				hasErrors = true
				break
			}
		}
		if hasErrors {
			monitor.SetVariable("installation_status", "failed")
		} else if len(status.Steps) > 0 {
			allCompleted := true
			for _, step := range status.Steps {
				if step.Status != "completed" && step.Status != "skipped" {
					allCompleted = false
					break
				}
			}
			if allCompleted {
				monitor.SetVariable("installation_status", "completed")
			} else {
				monitor.SetVariable("installation_status", "in_progress")
			}
		}
	}

	handlerService := pkg.NewWebHandlerService(monitor)

	handlerService.SetInstallationHandler(rootSteps(), func() error {
		log.Info("Restarting bloom with new configuration...")

		if _, err := os.Stat("bloom.log"); err == nil {
			timestamp := time.Now().Format("20060102-150405")
			archivedPath := fmt.Sprintf("bloom-%s.log", timestamp)
			if err := os.Rename("bloom.log", archivedPath); err != nil {
				log.Errorf("Failed to archive bloom.log: %v", err)
			} else {
				log.Infof("Archived bloom.log to %s", archivedPath)
			}
		}

		return pkg.RunStepsWithCLI(rootSteps())
	})

	if _, err := os.Stat(yamlPath); err == nil {
		yamlData, err := os.ReadFile(yamlPath)
		if err == nil {
			var yamlConfig map[string]interface{}
			if err := yaml.Unmarshal(yamlData, &yamlConfig); err == nil {
				configValues := make(map[string]string)
				for k, v := range yamlConfig {
					if v != nil {
						configValues[k] = fmt.Sprintf("%v", v)
					}
				}
				handlerService.SetPrefilledConfig(configValues)
				log.Infof("Loaded %d config values from bloom.yaml for monitoring", len(configValues))
			}
		}
	} else if status != nil && len(status.ConfigValues) > 0 {
		handlerService.SetPrefilledConfig(status.ConfigValues)
		log.Infof("Using %d config values from bloom.log", len(status.ConfigValues))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", handlerService.DashboardHandler)
	mux.HandleFunc("/monitor", handlerService.MonitorHandler)
	mux.HandleFunc("/api/logs", handlerService.LogsAPIHandler)
	mux.HandleFunc("/api/variables", handlerService.VariablesAPIHandler)
	mux.HandleFunc("/api/steps", handlerService.StepsAPIHandler)
	mux.HandleFunc("/api/error", handlerService.ErrorAPIHandler)
	mux.HandleFunc("/api/reconfigure", handlerService.ReconfigureHandler)
	mux.HandleFunc("/api/prefilled-config", handlerService.PrefilledConfigAPIHandler)
	mux.HandleFunc("/api/config", handlerService.ConfigAPIHandler)

	handler := pkg.LocalhostOnly(mux)
	server := &http.Server{
		Addr:    "127.0.0.1" + port,
		Handler: handler,
	}

	go pkg.WatchLogFile(monitor)

	fmt.Println("📊 Web interface is running. Press Ctrl+C to stop...")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Printf("Web server error: %v\n", err)
	}
}

func runWebInterfaceWithConfig() {
	fmt.Println("🚀 Starting Cluster-Bloom Web Interface...")
	fmt.Println()

	portNum := findAvailablePort(62078)
	port := fmt.Sprintf(":%d", portNum)
	url := fmt.Sprintf("http://127.0.0.1%s", port)

	if cfgFile != "" {
		fmt.Printf("📄 Configuration file: %s\n", cfgFile)
		if oneShot {
			fmt.Println("⚡ One-shot mode: will auto-proceed after loading configuration")
		} else {
			fmt.Println("🔄 Pre-filled configuration ready for review and confirmation")
		}
		fmt.Println()
	}

	fmt.Printf("🌐 Web interface starting on %s\n", url)
	fmt.Println("📊 Configuration interface accessible only from localhost")
	fmt.Printf("🔧 Configure your cluster at %s\n", url)
	fmt.Println()
	fmt.Println("🔗 For remote access, create an SSH tunnel:")
	fmt.Printf("   ssh -L %d:127.0.0.1:%d user@remote-server\n", portNum, portNum)
	fmt.Printf("   Then access: http://127.0.0.1:%d\n\n", portNum)

	err := pkg.RunWebInterfaceWithConfig(port, rootSteps(), cfgFile, oneShot, setupLogging, logConfigValues)
	if err != nil {
		log.Fatal(err)
	}
}


var cliCmd = &cobra.Command{
	Use:   "cli",
	Short: "Run with CLI-only interface (logs only)",
	Long: `
Run Cluster-Bloom with command-line interface only. This mode shows logs in the terminal
and requires a configuration file to be provided via --config flag.

This mode is useful for:
- Automated deployments
- Headless environments
- CI/CD pipelines
- Users who prefer terminal-only interfaces
`,
	Run: func(cmd *cobra.Command, args []string) {
		if cfgFile == "" {
			fmt.Println("❌ CLI mode requires a configuration file. Use --config flag to specify one.")
			fmt.Println("💡 Run 'bloom' without arguments to use the web interface for configuration.")
			os.Exit(1)
		}


		fmt.Println("🚀 Starting Cluster-Bloom in CLI mode...")
		fmt.Printf("📄 Using configuration: %s\n", cfgFile)
		fmt.Println("📋 Logs will be displayed in terminal")
		fmt.Println()

		log.Debug("Starting package installation in CLI mode")
		pkg.RunStepsWithCLI(rootSteps())
	},
}

var helpCmd = &cobra.Command{
	Use:   "help",
	Short: "Display help information",
	Run: func(cmd *cobra.Command, args []string) {
		displayHelp()
	},
}

