package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/silogen/cluster-bloom/pkg/ansible/runtime"
	"github.com/silogen/cluster-bloom/pkg/config"
	"github.com/silogen/cluster-bloom/pkg/webui"
	"github.com/spf13/cobra"
)

var (
	port         int
	playbookName string
	dryRun       bool
	tags         string
	destroyData  bool
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
		Long:  `Bloom - A tool for generating bloom.yaml configurations and deploying Kubernetes clusters using Ansible.`,
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

	ansibleCmd := &cobra.Command{
		Use:   "ansible <config-file>",
		Short: "Deploy cluster using Ansible",
		Long: `Deploy a Kubernetes cluster using Ansible playbooks.

Requires a configuration file (typically bloom.yaml). Use --playbook to specify which playbook to run.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if destroyData {
				runClusterCleanup()
			}
			runAnsible(args[0])
		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Bloom V2.0.0-alpha")
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

The cleanup runs immediately without confirmation prompts.`,
		Run: func(cmd *cobra.Command, args []string) {
			runClusterCleanup()
		},
	}

	// Add flags
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 62078, "Port for web UI (fails if in use)")
	ansibleCmd.Flags().StringVar(&playbookName, "playbook", "cluster-bloom.yaml", "Playbook to run (default: cluster-bloom.yaml)")
	ansibleCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Run in check mode without making changes")
	ansibleCmd.Flags().StringVar(&tags, "tags", "", "Run only tasks with specific tags (e.g., cleanup, validate, storage)")
	ansibleCmd.Flags().BoolVar(&destroyData, "destroy-data", false, "WARNING: Destroys ALL existing data (cluster + disks) for fresh deployment")

	// Add subcommands
	rootCmd.AddCommand(webuiCmd)
	rootCmd.AddCommand(ansibleCmd)
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

	// Run the playbook
	exitCode, err := runtime.RunPlaybook(cfg, playbookName, dryRun, tags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(exitCode)
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
