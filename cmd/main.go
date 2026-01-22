package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/silogen/cluster-bloom/pkg/ansible/runtime"
	"github.com/silogen/cluster-bloom/pkg/config"
	"github.com/silogen/cluster-bloom/pkg/webui"
	"github.com/spf13/cobra"
)

var (
	Version      string // Set via ldflags during build
	port         int
	playbookName string
	dryRun       bool
	tags         string
	destroyData  bool
	forceCleanup bool
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
			runClusterCleanup()
		},
	}

	cliCmd := &cobra.Command{
		Use:   "cli <config-file>",
		Short: "Deploy cluster using configuration file",
		Long: `Deploy a Kubernetes cluster using the specified configuration file.

Requires a configuration file (typically bloom.yaml).`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			checkRootPrivileges("cli")
			runAnsible(args[0])
		},
	}

	// Add flags
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 62078, "Port for web UI (fails if in use)")

	// Add CLI command flags
	cliCmd.Flags().StringVar(&playbookName, "playbook", "cluster-bloom.yaml", "Playbook to run (default: cluster-bloom.yaml)")
	cliCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Run in check mode without making changes")
	cliCmd.Flags().StringVar(&tags, "tags", "", "Run only tasks with specific tags (e.g., cleanup, validate, storage)")
	cliCmd.Flags().BoolVar(&destroyData, "destroy-data", false, "⚠️  DANGER: Permanently destroys ALL cluster data, storage, and disks. Requires interactive confirmation.")

	// Add cleanup-specific flags
	cleanupCmd.Flags().BoolVarP(&forceCleanup, "force", "f", false, "Skip confirmation prompt and force immediate cleanup (USE WITH CAUTION)")

	// Add subcommands
	rootCmd.AddCommand(webuiCmd)
	rootCmd.AddCommand(cliCmd)
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

	// Handle destructive data cleanup if requested
	if destroyData {
		if !confirmDestructiveOperation(cfg) {
			fmt.Println("\n❌ Operation aborted by user. No data was harmed.")
			os.Exit(0)
		}
		runClusterCleanup()
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

func runClusterCleanup() {
	fmt.Println("🧹 Starting Bloom cluster cleanup...")

	var errors []string

	// Step 1: Clean Longhorn Mounts (equivalent to CleanLonghornMountsStep)
	if err := runtime.CleanupLonghornMounts(); err != nil {
		errors = append(errors, fmt.Sprintf("Longhorn cleanup: %v", err))
	}

	// Step 2: Uninstall RKE2 (equivalent to UninstallRKE2Step)
	if err := runtime.UninstallRKE2(); err != nil {
		errors = append(errors, fmt.Sprintf("RKE2 uninstall: %v", err))
	}

	// Step 3: Clean Disks (equivalent to CleanDisksStep)
	if err := runtime.CleanupBloomDisks(); err != nil {
		errors = append(errors, fmt.Sprintf("Disk cleanup: %v", err))
	}

	// Report results
	if len(errors) > 0 {
		fmt.Printf("⚠️  Cleanup completed with warnings: %s\n", strings.Join(errors, "; "))
		os.Exit(1)
	} else {
		fmt.Println("✅ Bloom cluster cleanup completed successfully")
	}
}
