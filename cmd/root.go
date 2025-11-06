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
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/silogen/cluster-bloom/pkg"
	"github.com/silogen/cluster-bloom/pkg/args"
	"github.com/silogen/cluster-bloom/pkg/mockablecmd"
)

var rootCmd = &cobra.Command{
	Use:   "bloom",
	Short: "Cluster-Bloom creates a cluster",
	Long:  displayHelp(),
	Run: func(cmd *cobra.Command, args []string) {
		// In one-shot mode, config file must exist
		if oneShot {
			if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
				fmt.Printf("❌ Error: --one-shot requires a valid config file\n")
				fmt.Printf("❌ Config file '%s' does not exist\n", cfgFile)
				fmt.Println("💡 Use --config flag to specify a valid configuration file")
				os.Exit(1)
			}
		}

		// Handle reconfigure flag
		if reconfigure {
			fmt.Println("🚀 Starting fresh configuration...")
			fmt.Println()
			// Continue to configuration interface
			runWebInterfaceWithConfig()
			return
		}

		// Check if bloom.log exists with meaningful content when no config provided
		if _, err := os.Stat(cfgFile); err != nil {
			currentDir, _ := os.Getwd()
			logPath := filepath.Join(currentDir, "bloom.log")
			if hasMeaningfulLogContent(logPath) {
				// bloom.log exists - start webui for monitoring
				fmt.Println("🔍 Found existing bloom.log - starting monitoring interface...")
				fmt.Println()
				startWebUIMonitoring()
				return
			}
		}

		// No existing log or config provided - start web interface for configuration
		runWebInterfaceWithConfig()
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

var cfgFile string
var oneShot bool
var reconfigure bool

func init() {
	// Ensure arguments are initialized before help is displayed
	SetArguments()

	// Update help text after arguments are initialized
	rootCmd.Long = displayHelp()

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "bloom.yaml", "config file (default is ./bloom.yaml)")
	rootCmd.PersistentFlags().BoolVar(&oneShot, "one-shot", false, "skip confirmation when using --config (useful for automation)")
	rootCmd.PersistentFlags().BoolVar(&reconfigure, "reconfigure", false, "archive existing bloom.log and start fresh configuration")
	rootCmd.AddCommand(helpCmd)
	rootCmd.AddCommand(cliCmd)
}

func initConfig() {
	viper.SetConfigFile(cfgFile)
	// Note: WatchConfig() is removed to prevent concurrent map write issues
	// when multiple web handlers access viper simultaneously
	SetArguments()
	// Set defaults from args package
	for _, arg := range args.Arguments {
		viper.SetDefault(arg.Key, arg.Default)
	}
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err == nil {
		// Only log if we're not in the default UI mode
		// (setupLogging and logConfigValues will be called later when needed)
	}

	// Load mocks from config if present
	mockablecmd.LoadMocks()
}

func setupLogging() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	currentDir, err := os.Getwd()
	if err != nil {
		// Still log to stderr if we can't get current dir
		fmt.Fprintf(os.Stderr, "Could not determine current directory: %v\n", err)
		return
	}

	logPath := filepath.Join(currentDir, "bloom.log")

	// Archive existing bloom.log if it exists
	if _, err := os.Stat(logPath); err == nil {
		timestamp := time.Now().Format("20060102-150405")
		archivedPath := filepath.Join(currentDir, fmt.Sprintf("bloom-%s.log", timestamp))
		if err := os.Rename(logPath, archivedPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to archive bloom.log: %v\n", err)
		}
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Still log to stderr if we can't open the file
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

// hasMeaningfulLogContent checks if bloom.log exists and contains meaningful installation data
// Returns true only if the log file exists and has actual step execution logs
func hasMeaningfulLogContent(logPath string) bool {
	// Check if file exists and is not empty
	if stat, err := os.Stat(logPath); err != nil || stat.Size() == 0 {
		return false
	}

	// Try to parse the log - if parsing succeeds and contains steps or meaningful data, it's valid
	if status, err := pkg.ParseBloomLog(logPath); err == nil {
		// Consider the log meaningful if it has steps or configuration values or errors
		return len(status.Steps) > 0 || len(status.ConfigValues) > 0 || len(status.Errors) > 0
	}

	return false
}

func rootSteps() []pkg.Step {
	preK8Ssteps := []pkg.Step{
		pkg.ValidateArgsStep,
		pkg.ValidateSystemRequirementsStep,
		pkg.CheckUbuntuStep,
		pkg.HasSufficientRancherPartitionStep,
		pkg.InstallDependentPackagesStep,
		pkg.CleanLonghornMountsStep,
		pkg.UninstallRKE2Step,
		pkg.CleanDisksStep,
		pkg.SetupMultipathStep,
		pkg.UpdateModprobeStep,
		pkg.PrepareLonghornDisksStep,
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
		pkg.SetupKubeConfig,
	}
	postK8Ssteps := []pkg.Step{
		pkg.CreateChronyConfigStep,
		pkg.PreloadImagesStep,
		pkg.LonghornPreflightCheckStep,
		pkg.SetupLonghornStep,
		pkg.SetupMetallbStep,
		pkg.CreateMetalLBConfigStep,
		pkg.CreateDomainConfigStep,
		pkg.WaitForClusterReady,
		pkg.CreateBloomConfigMapStepFunc(Version),
		pkg.SetupClusterForgeStep,
	}

	postK8Ssteps = append(postK8Ssteps, pkg.FinalOutput)
	combinedSteps := append(append(preK8Ssteps, k8Ssteps...), postK8Ssteps...)

	// Set step IDs in args package for validation
	stepIDs := make([]string, len(combinedSteps))
	for i, step := range combinedSteps {
		stepIDs[i] = step.Id
	}
	args.SetAllSteps(stepIDs)

	return combinedSteps
}

func displayHelp() string {
	helpContent := `
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
` + args.GenerateArgsHelp() + `

Usage:
  Use the --config flag to specify a configuration file that will pre-fill the web interface, or set the above variables in the environment or a Viper-compatible config file.
  Use --one-shot with --config to auto-proceed after loading configuration for automated deployments.
`
	return helpContent
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
	// Setup logging early
	setupLogging()

	// Display initial status
	pkg.CheckAndDisplayExistingStatus()

	currentDir, _ := os.Getwd()
	logPath := filepath.Join(currentDir, "bloom.log")
	yamlPath := filepath.Join(currentDir, "bloom.yaml")

	// Find an available port starting from 62078
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
	fmt.Println("🔗 For remote access, create an SSH tunnel:")
	fmt.Printf("   ssh -L %d:127.0.0.1:%d user@remote-server\n", portNum, portNum)
	fmt.Printf("   Then access: http://127.0.0.1:%d\n\n", portNum)
	fmt.Println()

	// Start web interface in monitoring mode
	monitor := pkg.NewWebMonitor()
	pkg.SetGlobalWebMonitor(monitor)

	// Parse existing log to populate initial status
	var status *pkg.BloomStatus
	if parsedStatus, err := pkg.ParseBloomLog(logPath); err == nil {
		status = parsedStatus
		// First, initialize ALL expected steps based on configuration
		allSteps := rootSteps()
		enabledSteps := pkg.CalculateEnabledSteps(allSteps)
		for i, step := range enabledSteps {
			monitor.InitializeStep(step, i+1)
		}

		// Create a map of step names to IDs for matching
		stepNameToID := make(map[string]string)
		for _, step := range enabledSteps {
			stepNameToID[step.Name] = step.Id
		}

		// Then update the ones that were actually executed according to the log
		for _, step := range status.Steps {
			// Find the corresponding step ID
			stepID := stepNameToID[step.Name]
			if stepID == "" {
				// Fallback: use the name as-is if we can't find a match
				stepID = step.Name
			}

			// Add log entry for step start
			monitor.AddLog("INFO", fmt.Sprintf("Starting step: %s", step.Name), stepID)

			// Set step status and add relevant logs
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
				monitor.CompleteStep(stepID, fmt.Errorf("%s", step.Error))
			case "skipped":
				monitor.SkipStep(stepID)
				monitor.AddLog("INFO", fmt.Sprintf("Step %s is skipped", step.Name), stepID)
			case "running":
				monitor.StartStep(stepID)
			}
		}

		// Add error logs to monitor
		for _, errMsg := range status.Errors {
			monitor.AddLog("ERROR", errMsg, "system")
		}

		// Add OS error if present
		if status.OSError != "" {
			monitor.AddLog("ERROR", status.OSError, "system")
		}

		// Set variables from parsed status
		monitor.SetVariable("domain", status.Domain)
		monitor.SetVariable("first_node", fmt.Sprintf("%v", status.FirstNode))
		monitor.SetVariable("gpu_node", fmt.Sprintf("%v", status.GPUNode))
		// Use the actual enabled steps count for total
		monitor.SetVariable("total_steps", len(enabledSteps))

		// Add all configuration values to monitor (skip empty values)
		for key, value := range status.ConfigValues {
			if value != "" {
				monitor.SetVariable(key, value)
			}
		}

		// Set overall installation status
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

	// Set up installation capability for monitoring mode reconfigure
	handlerService.SetInstallationHandler(rootSteps(), func() error {
		log.Info("Restarting bloom with new configuration...")

		// Reinitialize logging to archive the current log
		setupLogging()

		// Start installation with new configuration
		return pkg.RunStepsWithCLI(rootSteps())
	})

	// Load configuration from bloom.yaml if it exists (prioritize this over log)
	if _, err := os.Stat(yamlPath); err == nil {
		yamlData, err := os.ReadFile(yamlPath)
		if err == nil {
			var yamlConfig map[string]interface{}
			if err := yaml.Unmarshal(yamlData, &yamlConfig); err == nil {
				// Convert to map[string]string
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
		// Fall back to config values from log if no yaml file
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
	mux.HandleFunc("/api/config-only", handlerService.ConfigOnlyAPIHandler)

	handler := pkg.LocalhostOnly(mux)
	server := &http.Server{
		Addr:    "127.0.0.1" + port,
		Handler: handler,
	}

	// Start watching the log file
	go pkg.WatchLogFile(monitor)

	fmt.Println("📊 Web interface is running. Press Ctrl+C to stop...")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Printf("Web server error: %v\n", err)
	}
}

func runWebInterfaceWithConfig() {
	fmt.Println("🚀 Starting Cluster-Bloom Web Interface...")
	fmt.Println()

	// Find an available port starting from 62078
	portNum := findAvailablePort(62078)
	port := fmt.Sprintf(":%d", portNum)
	url := fmt.Sprintf("http://127.0.0.1%s", port)

	fmt.Printf("📄 Configuration file: %s\n", cfgFile)
	if oneShot {
		fmt.Println("⚡ One-shot mode: will auto-proceed after loading configuration")
	} else {
		fmt.Println("🔄 Pre-filled configuration ready for review and confirmation")
	}
	fmt.Println()

	fmt.Printf("🌐 Web interface starting on %s\n", url)
	fmt.Println("📊 Configuration interface accessible only from localhost")
	fmt.Printf("🔧 Configure your cluster at %s\n", url)
	fmt.Println()
	fmt.Println("🔗 For remote access, create an SSH tunnel:")
	fmt.Printf("   ssh -L %d:127.0.0.1:%d user@remote-server\n", portNum, portNum)
	fmt.Printf("   Then access: http://127.0.0.1:%d\n\n", portNum)

	// Pass config file information to the web interface
	// Also pass setupLogging and logConfigValues functions to be called when installation starts
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

		if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
			fmt.Printf("❌ Configuration file %s does not exist. Use --config flag to specify a config file.\n", cfgFile)
			fmt.Println("💡 Run 'bloom' without arguments to use the web interface for configuration.")
			os.Exit(1)
		}

		// Setup logging and log config values for CLI mode
		setupLogging()
		logConfigValues()

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
		rootCmd.Help()
	},
}
