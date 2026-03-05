package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/silogen/cluster-bloom/pkg/ansible/runtime"
	"github.com/silogen/cluster-bloom/pkg/config"
	"github.com/silogen/cluster-bloom/pkg/webui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	Version      string // Set via ldflags during build
	port         int
	playbookName string
	dryRun       bool
	tags         string
	destroyData  bool
	forceCleanup bool
	extraVars  []string
	verbose    bool
	configFile string
	export     bool
)

func init() {
	// Set the embedded filesystem for webui package
	webui.StaticFS = WebFS
}

func Execute() {
	// Handle __child__ for namespace re-execution
	if len(os.Args) > 1 && os.Args[1] == "__child__" {
		runtime.RunChild()
		return
	}

	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func buildConfigFieldsHelp() string {
	var sb strings.Builder

	sb.WriteString("CONFIGURATION FIELDS\n\n")

	schema := config.Schema()
	currentSection := ""

	for _, field := range schema {
		// Print section header if changed
		if field.Section != currentSection {
			currentSection = field.Section
			sb.WriteString(fmt.Sprintf("%s\n", currentSection))
		}

		// Build field info on one line: NAME (type) - Description [Default: value] [Requires: deps]
		line := fmt.Sprintf("  %-30s %-10s", field.Key, "("+field.Type+")")

		// Add description
		if field.Description != "" {
			line += fmt.Sprintf(" %s", field.Description)
		}

		// Add default if not empty/nil
		if field.Default != nil {
			defaultStr := fmt.Sprintf("%v", field.Default)
			// Skip if empty string, false, or empty array
			if defaultStr != "" && defaultStr != "false" && defaultStr != "[]" {
				if len(defaultStr) > 60 {
					defaultStr = defaultStr[:57] + "..."
				}
				line += fmt.Sprintf(" [Default: %s]", defaultStr)
			}
		}

		// Add options for enum
		if len(field.Options) > 0 {
			line += fmt.Sprintf(" [Options: %s]", strings.Join(field.Options, ", "))
		}

		// Add dependencies
		if field.Dependencies != "" {
			line += fmt.Sprintf(" [Requires: %s]", field.Dependencies)
		}

		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n")
	return sb.String()
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "bloom",
		Short: "Kubernetes Cluster Deployment Tool",
		Long:  `Bloom - A tool for generating bloom.yaml configurations and deploying Kubernetes clusters.`,
		Run: func(cmd *cobra.Command, args []string) {
			// Default action: start webui
			runWebUI(cmd)
		},
	}

	// Set custom help template to show config fields at bottom
	rootCmd.SetHelpTemplate(rootCmd.HelpTemplate() + "\n" + buildConfigFieldsHelp())

	webuiCmd := &cobra.Command{
		Use:   "webui",
		Short: "Start the web UI configuration generator",
		Long:  `Launch a web-based interface for generating bloom.yaml configuration files.`,
		Run: func(cmd *cobra.Command, args []string) {
			runWebUI(cmd)
		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			if Version != "" {
				fmt.Printf("%s\n", Version)
			} else {
				fmt.Println("dev")
			}
		},
	}

	cleanupCmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up existing Bloom cluster installation",
		Long: `Removes RKE2 services, Longhorn mounts, and managed disks from previous Bloom installations.

This command performs the equivalent of Bloom v1 cleanup operations:
- Stops Longhorn services and unmounts all Longhorn-related storage
- Executes RKE2 uninstall script to remove RKE2 components  
- Cleans up bloom-managed disks and removes temp drives

By default, this command requires confirmation before proceeding. Use --force to skip confirmation.`,
		Run: func(cmd *cobra.Command, args []string) {
			checkRootPrivileges("cleanup")
			// Check if force flag is used to bypass confirmation
			if !forceCleanup {
				if !confirmCleanupOperation() {
					fmt.Println("❌ Cleanup aborted by user.")
					os.Exit(0)
				}
			} else {
				fmt.Println("🚀 Force cleanup requested - bypassing confirmation")
			}
			// For standalone cleanup command, we don't have a config, so pass nil
			runClusterCleanup(nil)
		},
	}

	cliCmd := &cobra.Command{
		Use:   "cli <config-file>",
		Short: "Deploy cluster using configuration file",
		Long: `Deploy a Kubernetes cluster using the specified configuration file.

Requires a configuration file (typically bloom.yaml).

Use --export flag to output the generated playbook to stdout instead of executing it.
Example: sudo ./bloom cli bloom.yaml --export > myPlaybook.yaml`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if !export {
				checkRootPrivileges("cli")
			}
			runAnsible(args[0])
		},
	}

	runCmd := &cobra.Command{
		Use:   "run <playbook>",
		Short: "Run an Ansible playbook using Bloom's containerized runtime",
		Long: `Execute an external Ansible playbook on localhost using Bloom's containerized
Ansible runtime. No Ansible or Python installation required on the host.

The playbook's parent directory is mounted into the container, so relative
imports (roles, tasks, vars) within that directory tree work as expected.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			checkRootPrivileges("run")
			runPlaybookDirect(args[0])
		},
	}

	// Add flags
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 62078, "Port for web UI (fails if in use)")

	// Add CLI command flags
	cliCmd.Flags().StringVar(&playbookName, "playbook", "cluster-bloom.yaml", "Playbook to run (default: cluster-bloom.yaml)")
	cliCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Run in check mode without making changes")
	cliCmd.Flags().StringVar(&tags, "tags", "", "Run only tasks with specific tags (e.g., cleanup, validate, storage)")
	cliCmd.Flags().BoolVar(&destroyData, "destroy-data", false, "⚠️  DANGER: Permanently destroys ALL cluster data, storage, and disks. Requires interactive confirmation.")
	cliCmd.Flags().BoolVar(&export, "export", false, "Export generated playbook to stdout instead of executing it")

	// Add run command flags
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Run in check mode without making changes")
	runCmd.Flags().StringVar(&tags, "tags", "", "Run only tasks with specific tags")
	runCmd.Flags().StringArrayVarP(&extraVars, "extra-vars", "e", nil, "Extra variables passed to ansible-playbook (repeatable)")
	runCmd.Flags().StringVarP(&configFile, "config", "c", "", "YAML config file whose keys become ansible extra vars")
	runCmd.Flags().BoolVar(&verbose, "verbose", false, "Show full Ansible output instead of clean summary")

	// Add cleanup-specific flags
	cleanupCmd.Flags().BoolVarP(&forceCleanup, "force", "f", false, "Skip confirmation prompt and force immediate cleanup (USE WITH CAUTION)")

	// Add subcommands
	rootCmd.AddCommand(webuiCmd)
	rootCmd.AddCommand(cliCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(cleanupCmd)

	return rootCmd
}

func runWebUI(cmd *cobra.Command) {
	portSpecified := cmd.Flags().Changed("port")

	server := &webui.Server{Port: port, PortSpecified: portSpecified}
	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start web UI: %v\n", err)
		os.Exit(1)
	}
}

func runAnsible(configFile string) {
	// Load and validate config file
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Validate config (before injecting CLI flags)
	errors := config.Validate(cfg)
	if len(errors) > 0 {
		fmt.Fprintln(os.Stderr, "Configuration validation errors:")
		for _, err := range errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", err)
		}
		os.Exit(1)
	}

	// Handle export mode
	if export {
		if err := exportPlaybook(cfg, playbookName); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting playbook: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle destructive data cleanup if requested
	if destroyData {
		if !confirmDestructiveOperation(cfg) {
			fmt.Println("\n❌ Operation aborted by user. No data was harmed.")
			os.Exit(0)
		}
		runClusterCleanup(cfg)
	}

	// Use clean (terse/emoji) output mode by default
	mode := runtime.OutputClean

	// Run the playbook
	exitCode, err := runtime.RunPlaybook(cfg, playbookName, dryRun, tags, mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

func runPlaybookDirect(playbookPath string) {
	mode := runtime.OutputClean
	if verbose {
		mode = runtime.OutputVerbose
	}

	var allVars []string

	if configFile != "" {
		data, err := os.ReadFile(configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
			os.Exit(1)
		}
		var cfg map[string]any
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing config: %v\n", err)
			os.Exit(1)
		}
		allVars = append(allVars, runtime.ConfigToAnsibleVars(cfg)...)
	}

	allVars = append(allVars, extraVars...)

	exitCode, err := runtime.RunPlaybookDirect(playbookPath, dryRun, tags, allVars, mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

// exportPlaybook extracts and outputs the generated playbook to stdout
func exportPlaybook(cfg config.Config, playbookName string) error {
	// Create temporary directory for playbook extraction
	tempDir, err := os.MkdirTemp("", "bloom-export-*")
	if err != nil {
		return fmt.Errorf("create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	playbookDir := tempDir
	
	// Extract embedded playbooks to temp directory
	if err := runtime.ExtractEmbeddedPlaybooksToDir(playbookDir); err != nil {
		return fmt.Errorf("extract playbooks: %w", err)
	}

	// Extract manifests
	if err := runtime.ExtractManifests(playbookDir); err != nil {
		return fmt.Errorf("extract manifests: %w", err)
	}

	// Get the target playbook path
	playbookPath := filepath.Join(playbookDir, playbookName)
	if _, err := os.Stat(playbookPath); os.IsNotExist(err) {
		return fmt.Errorf("playbook not found: %s", playbookName)
	}

	// Read the playbook content
	playbookContent, err := os.ReadFile(playbookPath)
	if err != nil {
		return fmt.Errorf("read playbook: %w", err)
	}

	// Parse the playbook to inject configuration variables
	var playbook any
	if err := yaml.Unmarshal(playbookContent, &playbook); err != nil {
		return fmt.Errorf("parse playbook: %w", err)
	}

	// Apply configuration to playbook variables
	if err := injectConfigIntoPlaybook(playbook, cfg); err != nil {
		return fmt.Errorf("inject config: %w", err)
	}

	// Output the modified playbook to stdout
	output, err := yaml.Marshal(playbook)
	if err != nil {
		return fmt.Errorf("marshal playbook: %w", err)
	}

	fmt.Print(string(output))
	return nil
}

// injectConfigIntoPlaybook applies configuration values to playbook variables
func injectConfigIntoPlaybook(playbook any, cfg config.Config) error {
	// Handle case where playbook is a list of plays (most common format)
	if playsList, ok := playbook.([]any); ok && len(playsList) > 0 {
		if firstPlay, ok := playsList[0].(map[string]any); ok {
			if vars, ok := firstPlay["vars"].(map[string]any); ok {
				// Apply configuration values to override default vars
				for key, value := range cfg {
					vars[key] = value
				}
			}
		}
	} else if playbookMap, ok := playbook.(map[string]any); ok {
		// Handle case where playbook is a single play object
		if plays, ok := playbookMap["plays"].([]any); ok && len(plays) > 0 {
			if firstPlay, ok := plays[0].(map[string]any); ok {
				if vars, ok := firstPlay["vars"].(map[string]any); ok {
					// Apply configuration values to override default vars
					for key, value := range cfg {
						vars[key] = value
					}
				}
			}
		}
	}
	return nil
}

// confirmDestructiveOperation prompts the user to confirm the dangerous --destroy-data operation
func confirmDestructiveOperation(cfg config.Config) bool {
	fmt.Println("\n⚠️  DANGER: DESTRUCTIVE OPERATION REQUESTED ⚠️")
	fmt.Println()
	fmt.Println("You are about to PERMANENTLY DESTROY:")
	fmt.Println("• Entire Kubernetes cluster (RKE2 uninstall)")
	// Show specific devices that will be wiped if CLUSTER_DISKS is configured
	if clusterDisks, exists := cfg["CLUSTER_DISKS"]; exists && clusterDisks != nil {
		if disksStr, ok := clusterDisks.(string); ok && disksStr != "" {
			fmt.Printf("• All data on these storage devices: %s\n", disksStr)
		}
	}
	fmt.Println()

	// Read user input
	fmt.Print("Type \"yes\" to confirm destruction of all data: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("\n❌ Error reading input: %v\n", err)
		return false
	}

	// Trim whitespace and check for exact match
	input = strings.TrimSpace(input)
	if input != "yes" {
		fmt.Printf("\n❌ Operation aborted. Received: \"%s\", expected: \"yes\"\n", input)
		return false
	}

	fmt.Println("\n✅ Destructive operation confirmed. Proceeding with data destruction...")
	return true
}

// confirmCleanupOperation prompts the user to confirm the cleanup command
func confirmCleanupOperation() bool {
	fmt.Println("\n⚠️  CLUSTER CLEANUP REQUESTED ⚠️")
	fmt.Println()
	fmt.Println("This will PERMANENTLY DESTROY:")
	fmt.Println("• Entire Kubernetes cluster (RKE2 uninstall)")
	fmt.Println("• ALL Longhorn storage volumes and data")
	fmt.Println("• ALL managed disk devices (wipefs + deletion)")
	fmt.Println("• All cluster configuration and state")
	fmt.Println()
	fmt.Println("This action cannot be undone.")
	fmt.Println()

	// Read user input
	fmt.Print("Type \"yes\" to proceed with cleanup: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("\n❌ Error reading input: %v\n", err)
		return false
	}

	// Trim whitespace and check for exact match
	input = strings.TrimSpace(input)
	if input != "yes" {
		fmt.Printf("\n❌ Cleanup aborted. Received: \"%s\", expected: \"yes\"\n", input)
		return false
	}

	fmt.Println("\n✅ Cleanup confirmed. Proceeding...")
	return true
}

// checkRootPrivileges verifies that the current process is running with root privileges
func checkRootPrivileges(commandName string) {
	if os.Getuid() != 0 {
		fmt.Fprintf(os.Stderr, "❌ Error: %s requires root privileges\n\n", commandName)
		fmt.Fprintf(os.Stderr, "Please run this command with root privileges:\n")
		fmt.Fprintf(os.Stderr, "  sudo bloom %s", commandName)

		// Add the original arguments
		if len(os.Args) > 2 {
			for _, arg := range os.Args[2:] {
				fmt.Fprintf(os.Stderr, " %s", arg)
			}
		}
		fmt.Fprintf(os.Stderr, "\n\n")

		os.Exit(1)
	}
}

func runClusterCleanup(cfg config.Config) {
	fmt.Println("🧹 Starting Bloom cluster cleanup...")

	var errors []error

	// Extract CLUSTER_DISKS from config
	clusterDisks := ""
	if disks, exists := cfg["CLUSTER_DISKS"]; exists && disks != nil {
		if disksStr, ok := disks.(string); ok {
			clusterDisks = disksStr
		}
	}

	// Step 1: Clean Longhorn Mounts (equivalent to CleanLonghornMountsStep)
	if err := runtime.CleanupLonghornMounts(); err != nil {
		errors = append(errors, fmt.Errorf("Longhorn cleanup: %w", err))
	}

	// Step 2: Uninstall RKE2 (equivalent to UninstallRKE2Step)
	if err := runtime.UninstallRKE2(); err != nil {
		errors = append(errors, fmt.Errorf("RKE2 uninstall: %w", err))
	}

	// Step 3: Clean Disks (equivalent to CleanDisksStep)
	if err := runtime.CleanupBloomDisks(clusterDisks); err != nil {
		errors = append(errors, fmt.Errorf("Disk cleanup: %w", err))
	}

	// Report results
	if len(errors) > 0 {
		fmt.Printf("⚠️  Cleanup completed with warnings:\n")
		for _, err := range errors {
			fmt.Printf("  - %v\n", err)
		}
		os.Exit(1)
	} else {
		fmt.Println("✅ Bloom cluster cleanup completed successfully")
	}
}
