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
	port            int
	playbookOverride string
	dryRun          bool
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
		Use:   "ansible <config-file|playbook.yml>",
		Short: "Deploy cluster using Ansible",
		Long: `Deploy a Kubernetes cluster using Ansible playbooks.

Accepts either a bloom.yaml config file or a direct playbook path.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
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

	// Add flags
	rootCmd.PersistentFlags().IntVarP(&port, "port", "p", 62078, "Port for web UI (fails if in use)")
	ansibleCmd.Flags().StringVar(&playbookOverride, "playbook", "", "Override playbook to run (e.g., print-config.yml)")
	ansibleCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Run in check mode without making changes")

	// Add subcommands
	rootCmd.AddCommand(webuiCmd)
	rootCmd.AddCommand(ansibleCmd)
	rootCmd.AddCommand(versionCmd)

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

func runAnsible(configOrPlaybook string) {
	// Determine if this is a config file or direct playbook
	var cfg config.Config
	var playbookName string

	if configOrPlaybook == "hello.yml" || configOrPlaybook == "cluster-bloom.yaml" {
		// Direct playbook execution without config
		playbookName = configOrPlaybook
		cfg = make(config.Config)
	} else {
		// Load and validate config file
		var err error
		cfg, err = config.LoadConfig(configOrPlaybook)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Validate config
		errors := config.Validate(cfg)
		if len(errors) > 0 {
			fmt.Fprintln(os.Stderr, "Configuration validation errors:")
			for _, err := range errors {
				fmt.Fprintf(os.Stderr, "  - %s\n", err)
			}
			os.Exit(1)
		}

		// Use playbook override if specified, otherwise use cluster-bloom.yaml
		if playbookOverride != "" {
			playbookName = playbookOverride
		} else {
			playbookName = "cluster-bloom.yaml"
		}
	}

	// Run the playbook
	exitCode, err := runtime.RunPlaybook(cfg, playbookName, dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}
