package main

import (
	"fmt"
	"os"

	"github.com/silogen/cluster-bloom/pkg/webui"
)

func init() {
	// Set the embedded filesystem for webui package
	webui.StaticFS = WebFS
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
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
	fmt.Println("  bloom <command> [options]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  webui       Start the web UI configuration generator")
	fmt.Println("  version     Show version information")
	fmt.Println("  help        Show this help message")
	fmt.Println()
	fmt.Println("Web UI Options:")
	fmt.Println("  --port, -p  Specify port (fails if in use). Default: auto-find from 62078")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  bloom webui              # Auto-find available port from 62078")
	fmt.Println("  bloom webui --port 9090  # Use port 9090 (fails if in use)")
}
