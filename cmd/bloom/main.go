package main

import (
	"fmt"
	"os"

	"github.com/silogen/cluster-bloom/internal/config"
	"github.com/silogen/cluster-bloom/pkg/ansible/runtime"
	"github.com/silogen/cluster-bloom/pkg/webui"
)

func init() {
	// Set the embedded filesystem for webui package
	webui.StaticFS = WebFS
}

func main() {
	// Handle __child__ for namespace re-execution
	if len(os.Args) > 1 && os.Args[1] == "__child__" {
		runtime.RunChild()
		return
	}

	// Default to webui if no command provided
	if len(os.Args) < 2 {
		runWebUI()
		return
	}

	command := os.Args[1]

	switch command {
	case "ansible":
		runAnsible()
	case "webui":
		runWebUI()
	case "version":
		fmt.Println("Bloom V2.0.0-alpha")
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runAnsible() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: ansible command requires a config file or playbook")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  sudo bloom ansible <config-file>     # Deploy with bloom.yaml config")
		fmt.Fprintln(os.Stderr, "  sudo bloom ansible <playbook.yml>    # Run specific playbook")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Examples:")
		fmt.Fprintln(os.Stderr, "  sudo bloom ansible bloom.yaml        # Full cluster deployment")
		fmt.Fprintln(os.Stderr, "  sudo bloom ansible hello.yml         # Test playbook")
		os.Exit(1)
	}

	configOrPlaybook := os.Args[2]

	// Check if running as root (required for Linux namespaces)
	if os.Getuid() != 0 {
		fmt.Fprintln(os.Stderr, "Error: ansible command must be run as root")
		fmt.Fprintln(os.Stderr, "Please run with sudo:")
		fmt.Fprintf(os.Stderr, "  sudo bloom ansible %s\n", configOrPlaybook)
		os.Exit(1)
	}

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

		// Use cluster-bloom.yaml playbook for config-based deployment
		playbookName = "cluster-bloom.yaml"
	}

	// Run the playbook
	exitCode, err := runtime.RunPlaybook(cfg, playbookName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

func runWebUI() {
	port := 62078
	portSpecified := false

	// Parse port flag if provided
	for i, arg := range os.Args {
		if arg == "--port" || arg == "-p" {
			if i+1 < len(os.Args) {
				fmt.Sscanf(os.Args[i+1], "%d", &port)
				portSpecified = true
			}
		}
	}

	server := &webui.Server{Port: port, PortSpecified: portSpecified}
	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start web UI: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Bloom - Kubernetes Cluster Deployment Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  bloom [command] [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  (none)      Start web UI (default)")
	fmt.Println("  ansible     Deploy cluster using Ansible (requires root)")
	fmt.Println("  webui       Start the web UI configuration generator")
	fmt.Println("  version     Show version information")
	fmt.Println("  help        Show this help message")
	fmt.Println()
	fmt.Println("Ansible Usage:")
	fmt.Println("  sudo bloom ansible <config-file>     # Deploy with bloom.yaml config")
	fmt.Println("  sudo bloom ansible <playbook.yml>    # Run specific playbook")
	fmt.Println()
	fmt.Println("Web UI Options:")
	fmt.Println("  --port, -p  Specify port (fails if in use). Default: auto-find from 62078")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  bloom                                # Start web UI (default)")
	fmt.Println("  bloom --port 8080                    # Start web UI on specific port")
	fmt.Println("  sudo bloom ansible bloom.yaml        # Deploy cluster")
	fmt.Println("  sudo bloom ansible hello.yml         # Test Ansible runtime")
}
