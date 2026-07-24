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
	Version         string // Set via ldflags during build
	port            int
	playbookName    string
	dryRun          bool
	tags            string
	destroyData     bool
	forceCleanup    bool
	extraVars       []string
	verbose         bool
	configFile      string
	export          bool
	showVersion     bool
	clusterListenIP string
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
		Long: `Bloom - A tool for generating bloom.yaml configurations and deploying Kubernetes clusters.

Certificate Updates:
  To update TLS certificates in an existing cluster, use a separate config with --tags:
    bloom cli cert-update-config.yaml --tags update_cert
  See 'bloom cli --help' for details.`,
		Run: func(cmd *cobra.Command, args []string) {
			if showVersion {
				if Version != "" {
					fmt.Printf("%s\n", Version)
				} else {
					fmt.Println("dev")
				}
				return
			}
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
		Use:   "cleanup [config-file]",
		Short: "Clean up existing Bloom cluster installation",
		Long: `Removes RKE2 services, Longhorn mounts, and managed disks from previous Bloom installations.

This command performs the full cluster teardown sequence:
  1. Best-effort node drain (if cluster is reachable) with ~30s timeout
     - Uses --force and --disable-eviction to bypass stuck pods
     - Skips volume detach wait if no Longhorn volumes detected
  2. Logs out iSCSI sessions and stops Longhorn processes
  3. Force-unmounts all Longhorn/CSI/kubelet volumes
  4. Uninstalls RKE2 and removes all RKE2 directories
  5. Pre-cleans bloom artifacts (pvc-*, replicas, longhorn-disk.cfg) from the future
     mount range — preserving user files in those directories
  6. Cleans premounted disk contents (CLUSTER_PREMOUNTED_DISKS) while keeping the
     filesystem and fstab entry intact
  7. Removes bloom-managed fstab entries and wipes CLUSTER_DISKS devices

When a config file is provided, CLUSTER_DISKS and CLUSTER_PREMOUNTED_DISKS are read
from it. Before confirmation, a disk wipe preview is shown:
  - Bloom-managed mounts to be wiped (with user file warnings)
  - The future mount range that will be pre-cleaned
  - User files listed (up to 5), or count shown if more than 5
  - lost+found folders excluded (ext4 system folder, not user data)

Mount index allocation is fstab- and config-aware: the lowest contiguous range starting
from index 0 that does not conflict with premounted disk indexes is chosen, ensuring
CLUSTER_DISKS and CLUSTER_PREMOUNTED_DISKS can coexist without collision.

By default, this command requires confirmation before proceeding. Use --force to skip confirmation.`,
		Run: func(cmd *cobra.Command, args []string) {
			checkRootPrivileges("cleanup")
			// Load config early so the preview can use it before confirmation
			var cfg config.Config
			if len(args) > 0 {
				var err error
				cfg, err = config.LoadConfig(args[0])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not load config %s: %v\n", args[0], err)
				} else {
					fmt.Printf("Using config: %s\n", args[0])
				}
			}
			// Extract disk vars for the preview
			clusterDisks := ""
			if d, ok := cfg["CLUSTER_DISKS"].(string); ok {
				clusterDisks = d
			}
			premountedDisks := ""
			if p, ok := cfg["CLUSTER_PREMOUNTED_DISKS"].(string); ok {
				premountedDisks = p
			}
			rancherDisk := ""
			if r, ok := cfg["RANCHER_DISK"].(string); ok {
				rancherDisk = r
			}
			// Show disk wipe preview before asking for confirmation
			runtime.PrintDiskWipePreview(clusterDisks, premountedDisks, rancherDisk)
			// Check if force flag is used to bypass confirmation
			if !forceCleanup {
				if !confirmCleanupOperation() {
					fmt.Println("❌ Cleanup aborted by user.")
					os.Exit(0)
				}
			} else {
				fmt.Println("🚀 Force cleanup requested - bypassing confirmation")
			}
			runClusterCleanup(cfg)
		},
	}

	cliCmd := &cobra.Command{
		Use:   "cli <config-file>",
		Short: "Deploy cluster using configuration file",
		Long: `Deploy a Kubernetes cluster using the specified configuration file.

Requires a configuration file (typically bloom.yaml).

Certificate Updates:
  To update TLS certificates in an existing cluster:
    1. Create cert-update.yaml:
         FIRST_NODE: true
         NEW_TLS_CERT: /home/ubuntu/tls-cert.pem
         NEW_TLS_KEY: /home/ubuntu/tls-key.pem
         RESTART_ENVOY_PODS: true
    2. Run: bloom cli cert-update.yaml --tags update_cert
  This skips schema validation and runs only certificate update tasks.

Export Mode:
  Use --export flag to write a self-contained playbook directory (./bloom-playbook/)
  instead of executing it. The directory contains the root playbook, a bloom-vars.yaml
  file derived from your config, and the tasks/ and manifests/ trees. Run it with:
    ansible-playbook bloom-playbook/cluster-bloom.yaml
  Example: ./bloom cli bloom.yaml --export`,
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
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version information")

	// Add CLI command flags
	cliCmd.Flags().StringVar(&playbookName, "playbook", "cluster-bloom.yaml", "Playbook to run (default: cluster-bloom.yaml)")
	cliCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Run in check mode without making changes")
	cliCmd.Flags().StringVar(&tags, "tags", "", "Run only tasks with specific tags (e.g., cleanup, validate, storage)")
	cliCmd.Flags().BoolVar(&destroyData, "destroy-data", false, "⚠️  DANGER: Wipes cluster (RKE2 uninstall, Longhorn cleanup, disk wipe). Shows disk preview before confirmation. Equivalent to running bloom cleanup then redeploying.")
	cliCmd.Flags().StringVar(&clusterListenIP, "cluster-listen-ip", "", "IP address or CIDR for cluster binding (e.g., 192.168.1.100 or 192.168.1.0/24)")
	cliCmd.Flags().BoolVar(&export, "export", false, "Export the playbook to ./bloom-playbook/ (overwrites if exists) instead of executing it")

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

// partialConfigTags are tag-scoped runs that perform node-local checks/prep and
// don't need a complete cluster config. They may run with a minimal or empty
// bloom.yaml (required-field enforcement is relaxed for them).
var partialConfigTags = map[string]bool{
	"validate_node": true,
}

// envIsTrue reports whether an environment variable is set to a truthy value
// (true/TRUE/1/yes), matching the loose bypass-flag parsing used elsewhere.
func envIsTrue(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "true", "1", "yes":
		return true
	default:
		return false
	}
}

// tagsAllowPartialConfig reports whether every requested tag is a node-local
// tag that can run without a full cluster config. Returns false if tags is
// empty or includes any tag that needs full validation.
func tagsAllowPartialConfig(tags string) bool {
	if tags == "" {
		return false
	}
	for _, t := range strings.Split(tags, ",") {
		if !partialConfigTags[strings.TrimSpace(t)] {
			return false
		}
	}
	return true
}

func runAnsible(configFile string) {
	// Load and validate config file
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Inject CLI flag values into config (CLI flags override file values)
	if clusterListenIP != "" {
		cfg["CLUSTER_LISTEN_IP"] = clusterListenIP
	}

	// Strip keys from older bloom releases (warn-and-continue) so a stale
	// bloom.yaml keeps working with clear migration guidance, instead of
	// hard-failing on an "Unknown configuration key" that can't distinguish a
	// removed key from a typo. Must run before Validate (which flags unknown
	// keys) so the deprecated keys are already gone by then.
	for _, w := range config.ApplyDeprecations(cfg) {
		fmt.Fprintf(os.Stderr, "⚠️  %s\n", w)
	}

	// Validate config (after injecting CLI flags).
	//   - update_cert: skip validation entirely (uses a separate
	//     cert-update-config.yaml with its own keys).
	//   - node-local diagnostic/prep tags (e.g. validate_node): validate in
	//     "optional" mode so they run with a minimal or empty bloom.yaml. We
	//     still flag unknown keys and bad values, but don't require full
	//     cluster config (DOMAIN, CERT_OPTION, ...).
	//   - everything else: full validation.
	var validationErrors []string
	switch {
	case tags != "" && strings.Contains(tags, "update_cert"):
		// skip
	case tags != "" && tagsAllowPartialConfig(tags):
		validationErrors = config.ValidateOptional(cfg)
	default:
		validationErrors = config.Validate(cfg)
	}
	if len(validationErrors) > 0 {
		fmt.Fprintln(os.Stderr, "Configuration validation errors:")
		for _, err := range validationErrors {
			fmt.Fprintf(os.Stderr, "  - %s\n", err)
		}
		os.Exit(1)
	}

	// Resolve GPU-family stack defaults (host ROCm + GPU Operator + DeviceConfig)
	// and inject them as ansible vars before export/run.
	if err := config.ApplyGPUStackVars(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving GPU stack defaults: %v\n", err)
		os.Exit(1)
	}

	// Populate the Ansible OS-check variable from the Go single source of truth
	// (config.SupportedOSes) so the playbook's validate_node Ubuntu check and the
	// Go-side sshd guidance never drift. Injected after validation, so it is not
	// treated as an unknown config key. Overrides the playbook's fallback default.
	cfg["supported_ubuntu_versions"] = config.SupportedUbuntuVersions()

	// Handle export mode
	if export {
		if destroyData {
			fmt.Fprintln(os.Stderr, "Error: --destroy-data is not supported with --export")
			os.Exit(1)
		}
		if err := exportPlaybook(cfg, playbookName); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting playbook: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Host OS support pre-flight: fail early and clearly by name on an
	// unsupported OS (e.g. openSUSE) instead of surfacing Ubuntu-specific sshd
	// guidance or an opaque Ansible failure. Bypass with
	// BLOOM_ALLOW_UNSUPPORTED_OS=true for development/testing on other distros.
	if host, err := config.DetectHostOS(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not determine host OS (%v); skipping OS support check.\n", err)
	} else if !host.IsSupported() {
		if envIsTrue("BLOOM_ALLOW_UNSUPPORTED_OS") {
			fmt.Fprintf(os.Stderr, "⚠️  %s is not a supported OS; proceeding anyway because BLOOM_ALLOW_UNSUPPORTED_OS is set.\n", host.DisplayName())
		} else {
			fmt.Fprintf(os.Stderr, `❌ %s is not a supported operating system.
   cluster-bloom officially supports:
     %s
   Run bloom on a supported OS, or set BLOOM_ALLOW_UNSUPPORTED_OS=true to proceed anyway (unsupported).
`, host.DisplayName(), config.SupportedOSSummary())
			os.Exit(1)
		}
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
	exitCode, err := runtime.RunPlaybook(cfg, playbookName, dryRun, tags, mode, Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Print the post-run summary from the host (not the ansible child), so it
	// can check for real deployment evidence via kubectl rather than trusting
	// config alone. Skipped for export runs (which return earlier). The playbook
	// exit code gates which guidance is shown (deploy banner only on success,
	// remediation hints on failure).
	printClusterForgeSummary(cfg, configFile, tags, exitCode)

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

	exitCode, err := runtime.RunPlaybookDirect(playbookPath, dryRun, tags, allVars, mode, Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(exitCode)
}

// exportPlaybook writes a self-contained playbook directory (./bloom-playbook/)
// containing the root playbook, a vars file derived from cfg, and the task and
// manifest trees.
func exportPlaybook(cfg config.Config, playbookName string) error {
	const outDir = "bloom-playbook"

	if err := os.RemoveAll(outDir); err != nil {
		return fmt.Errorf("remove existing %s: %w", outDir, err)
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", outDir, err)
	}

	if err := runtime.ExtractEmbeddedPlaybooksToDir(outDir); err != nil {
		return fmt.Errorf("extract playbooks: %w", err)
	}
	if err := runtime.ExtractManifests(outDir); err != nil {
		return fmt.Errorf("extract manifests: %w", err)
	}

	playbookPath := filepath.Join(outDir, playbookName)
	if _, err := os.Stat(playbookPath); os.IsNotExist(err) {
		return fmt.Errorf("playbook not found: %s", playbookName)
	}
	playbookContent, err := os.ReadFile(playbookPath)
	if err != nil {
		return fmt.Errorf("read playbook: %w", err)
	}
	var playbook any
	if err := yaml.Unmarshal(playbookContent, &playbook); err != nil {
		return fmt.Errorf("parse playbook: %w", err)
	}
	if err := tweakRootPlaybookForExport(playbook); err != nil {
		return fmt.Errorf("tweak playbook: %w", err)
	}
	out, err := yaml.Marshal(playbook)
	if err != nil {
		return fmt.Errorf("marshal playbook: %w", err)
	}
	if err := os.WriteFile(playbookPath, out, 0644); err != nil {
		return fmt.Errorf("write playbook: %w", err)
	}

	varsBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal vars: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "bloom-vars.yaml"), varsBytes, 0644); err != nil {
		return fmt.Errorf("write vars: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Exported playbook to ./%s/\n", outDir)
	fmt.Fprintf(os.Stderr, "  Run with: ansible-playbook %s/%s\n", outDir, playbookName)
	return nil
}

// tweakRootPlaybookForExport adjusts the first play of the exported playbook so
// it runs standalone: targets localhost, loads bloom-vars.yaml, and exposes
// BLOOM_DIR (normally injected at runtime as the working directory).
func tweakRootPlaybookForExport(playbook any) error {
	plays, ok := playbook.([]any)
	if !ok || len(plays) == 0 {
		return fmt.Errorf("unexpected playbook structure: expected non-empty list of plays")
	}
	first, ok := plays[0].(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected playbook structure: first play is not a map")
	}

	first["hosts"] = "localhost"

	existing, _ := first["vars_files"].([]any)
	first["vars_files"] = append([]any{"bloom-vars.yaml"}, existing...)

	vars, ok := first["vars"].(map[string]any)
	if !ok {
		vars = map[string]any{}
		first["vars"] = vars
	}
	vars["BLOOM_DIR"] = "{{ ansible_env.PWD | default(playbook_dir) }}"
	return nil
}

// confirmDestructiveOperation prompts the user to confirm the dangerous --destroy-data operation
func confirmDestructiveOperation(cfg config.Config) bool {
	fmt.Println("\n⚠️  DANGER: DESTRUCTIVE OPERATION REQUESTED ⚠️")
	fmt.Println()
	fmt.Println("You are about to PERMANENTLY DESTROY:")
	fmt.Println("• Entire Kubernetes cluster (RKE2 uninstall)")
	// Show specific devices that will be wiped if CLUSTER_DISKS is configured
	clusterDisks := ""
	if d, exists := cfg["CLUSTER_DISKS"]; exists && d != nil {
		if disksStr, ok := d.(string); ok && disksStr != "" {
			clusterDisks = disksStr
			fmt.Printf("• All data on these storage devices: %s\n", disksStr)
		}
	}
	premountedDisks := ""
	if p, exists := cfg["CLUSTER_PREMOUNTED_DISKS"]; exists && p != nil {
		if pmStr, ok := p.(string); ok {
			premountedDisks = pmStr
		}
	}
	rancherDisk := ""
	if r, exists := cfg["RANCHER_DISK"]; exists && r != nil {
		if rdStr, ok := r.(string); ok {
			rancherDisk = rdStr
		}
	}
	// Show the same disk wipe preview as the standalone cleanup command
	runtime.PrintDiskWipePreview(clusterDisks, premountedDisks, rancherDisk)
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

	// Initialize signal handling for graceful shutdown
	runtime.InitSignalHandling()

	var errors []error

	// Extract CLUSTER_DISKS from config
	clusterDisks := ""
	if disks, exists := cfg["CLUSTER_DISKS"]; exists && disks != nil {
		if disksStr, ok := disks.(string); ok {
			clusterDisks = disksStr
		}
	}

	// Extract premounted disks config once for use in steps below
	premountedDisks := ""
	if pm, exists := cfg["CLUSTER_PREMOUNTED_DISKS"]; exists && pm != nil {
		if pmStr, ok := pm.(string); ok {
			premountedDisks = pmStr
		}
	}

	// Extract RANCHER_DISK from config
	rancherDisk := ""
	if rd, exists := cfg["RANCHER_DISK"]; exists && rd != nil {
		if rdStr, ok := rd.(string); ok {
			rancherDisk = rdStr
		}
	}

	fmt.Printf("   ⚙️  Config: CLUSTER_DISKS=%q, CLUSTER_PREMOUNTED_DISKS=%q, RANCHER_DISK=%q\n", clusterDisks, premountedDisks, rancherDisk)
	// Step 1: Clean Longhorn Mounts (equivalent to CleanLonghornMountsStep)
	if err := runtime.CleanupLonghornMounts(); err != nil {
		errors = append(errors, fmt.Errorf("Longhorn cleanup: %w", err))
	}

	// Step 2: Uninstall RKE2 (equivalent to UninstallRKE2Step)
	if err := runtime.UninstallRKE2(); err != nil {
		errors = append(errors, fmt.Errorf("RKE2 uninstall: %w", err))
	}

	// Step 2.5: Process validation removed - config-independent cleanup proven sufficient

	// Step 3: Pre-clean bloom artifacts from directories in the future mount range,
	// leaving user files intact. Done before fstab is rewritten so mounts are still valid.
	if err := runtime.PrecleanFutureMountPoints(clusterDisks, premountedDisks); err != nil {
		errors = append(errors, fmt.Errorf("Future mount pre-clean: %w", err))
	}

	// Step 4: Clean premounted disk contents BEFORE CleanupBloomDisks strips fstab.
	// unmountPriorLonghornDisks (called inside CleanupBloomDisks) removes bloom fstab
	// entries and unmounts the disks; if we run after that, mount falls back to device
	// scan which may fail. Running here while fstab is intact guarantees the mount works.
	if err := runtime.CleanupPremountedDisks(premountedDisks); err != nil {
		errors = append(errors, fmt.Errorf("Premounted disk cleanup: %w", err))
	}

	// Step 4.5: Clean RANCHER_DISK configuration — unmount bind mount and clean data
	// Always call - let function decide based on actual mount status
	if err := runtime.CleanupRancherDisk(""); err != nil {
		errors = append(errors, fmt.Errorf("RANCHER_DISK cleanup: %w", err))
	}

	// Step 5: Clean Disks — strips fstab entries and wipes CLUSTER_DISKS
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
