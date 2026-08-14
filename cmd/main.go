package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/silogen/cluster-bloom/pkg/ansible/runtime"
	"github.com/silogen/cluster-bloom/pkg/config"
	"github.com/silogen/cluster-bloom/pkg/webui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	Version              string // Set via ldflags during build
	port                 int
	playbookName         string
	dryRun               bool
	tags                 string
	destroyData          bool
	pauseK3s             bool
	preserveRKE2         bool
	autoConfirm          bool // --yes/-y, --auto-confirm-prompts, cleanup's --force/-f all bind here
	extraVars            []string
	verbose              bool
	configFile           string
	export               bool
	showVersion          bool
	clusterListenIP      string
	cleanupPreflightOnly bool
)

// rebootMarkerPath is where reboot_required_check.yaml records a pending
// reboot (e.g. after the amdgpu driver install). Lives under /var/lib, not
// BLOOM_DIR, so it persists regardless of which directory the user happens to
// invoke bloom from next.
const rebootMarkerPath = "/var/lib/bloom/reboot-required.json"

// rebootRequiredMarker mirrors the JSON written by reboot_required_check.yaml.
// Attempted acts as a loop-guard: once bloom has rebooted for a given
// unresolved condition, it will not offer to reboot again for the same
// marker — ansible's own fail message takes over with manual-intervention
// instructions instead of bloom silently rebooting forever.
type rebootRequiredMarker struct {
	Reason     string   `json:"reason"`
	Packages   []string `json:"packages"`
	Attempted  bool     `json:"attempted"`
	DetectedAt string   `json:"detected_at"`
	RunID      string   `json:"run_id"`
}

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

ClusterForge Bootstrap (deferred install only):
  Only needed if the initial bloom cli used CLUSTERFORGE_RELEASE: none (or "").
  After all nodes have joined, deploy ClusterForge from the first control plane node:
    sudo bloom cli bloom.yaml --tags deploy_clusterforge
  Before running, set CLUSTERFORGE_RELEASE to a release tag in bloom.yaml (not "none").
  If CLUSTERFORGE_RELEASE was already set during the initial bloom cli, ClusterForge
  deploys automatically and this step is not required.

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
  1. Fail-closed storage preflight against config, fstab, live mounts, and protected devices
  2. Best-effort node drain (if cluster is reachable) with ~30s timeout
     - Uses --force and --disable-eviction to bypass stuck pods
     - Skips volume detach wait if no Longhorn volumes detected
  3. Logs out Longhorn-only iSCSI sessions and stops Longhorn processes
  4. Force-unmounts all Longhorn/CSI/kubelet volumes
  5. Uninstalls RKE2 and removes all RKE2 directories
  6. Pre-cleans bloom artifacts (pvc-*, replicas, longhorn-disk.cfg) from the future
     mount range — preserving user files in those directories
  7. Cleans premounted disk contents (CLUSTER_PREMOUNTED_DISKS) while keeping the
     filesystem and fstab entry intact
  8. Removes strictly tagged Bloom fstab entries and wipes CLUSTER_DISKS devices

When a config file is provided, CLUSTER_DISKS, CLUSTER_PREMOUNTED_DISKS, and
RANCHER_DISK are read from it and must agree with live storage state. Before
confirmation, a disk wipe preview is shown:
  - Bloom-managed mounts to be wiped (with user file warnings)
  - The future mount range that will be pre-cleaned
  - User files listed (up to 5), or count shown if more than 5
  - lost+found folders excluded (ext4 system folder, not user data)

Mount index allocation is fstab- and config-aware: the lowest contiguous range starting
from index 0 that does not conflict with premounted disk indexes is chosen, ensuring
CLUSTER_DISKS and CLUSTER_PREMOUNTED_DISKS can coexist without collision.

By default, this command requires confirmation before proceeding. Use --force (or --yes/-y, --auto-confirm-prompts) to skip confirmation.`,
		Run: func(cmd *cobra.Command, args []string) {
			checkRootPrivileges("cleanup")
			// Load config early so the preview can use it before confirmation
			cfg := config.Config{}
			if len(args) > 0 {
				var err error
				cfg, err = config.LoadConfig(args[0])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: could not load config %s: %v\n", args[0], err)
					os.Exit(1)
				}
				fmt.Printf("Using config: %s\n", args[0])
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
			configWasProvided := len(args) > 0
			rancherExplicit := configWasProvided && strings.TrimSpace(rancherDisk) != ""
			storage, err := runtime.ResolveCleanupStorage(
				clusterDisks, premountedDisks, rancherDisk, configWasProvided, rancherExplicit)
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ Cleanup preflight failed: %v\n", err)
				os.Exit(1)
			}
			if err := runtime.RunCleanupPreflight(storage); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Cleanup preflight failed: %v\n", err)
				os.Exit(1)
			}
			clusterDisks = storage.ClusterDisks
			premountedDisks = storage.PremountedDisks
			rancherDisk = storage.RancherDisk
			cfg["CLUSTER_DISKS"] = clusterDisks
			cfg["CLUSTER_PREMOUNTED_DISKS"] = premountedDisks
			cfg["RANCHER_DISK"] = rancherDisk

			// Show disk wipe preview before asking for confirmation
			runtime.PrintDiskWipePreview(clusterDisks, premountedDisks, rancherDisk)
			if cleanupPreflightOnly {
				fmt.Println("✅ Preflight-only cleanup validation completed; no changes were made")
				return
			}
			// Check if force/--yes flag is used to bypass confirmation
			if !autoConfirm {
				if !confirmCleanupOperation() {
					fmt.Println("❌ Cleanup aborted by user.")
					os.Exit(0)
				}
			} else {
				fmt.Println("🚀 Force cleanup requested - bypassing confirmation")
			}
			options := cleanupRunOptions{
				configWasProvided: true,
				rancherExplicit:   storage.RancherExplicit,
			}
			if err := runClusterCleanup(cfg, options); err != nil {
				fmt.Fprintf(os.Stderr, "❌ Cleanup stopped: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cliCmd := &cobra.Command{
		Use:   "cli <config-file>",
		Short: "Deploy cluster using configuration file",
		Long: `Deploy a Kubernetes cluster using the specified configuration file.

Requires a configuration file (typically bloom.yaml).

Common workflows:
  Deploy a cluster:
    sudo bloom cli bloom.yaml

  Check node readiness without deploying:
    sudo bloom cli bloom.yaml --tags validate_node

  Deploy deferred ClusterForge from the first control plane after all nodes join
  (set CLUSTERFORGE_RELEASE first):
    sudo bloom cli bloom.yaml --tags deploy_clusterforge

  Update TLS certificates using a separate config:
    sudo bloom cli cert-update.yaml --tags update_cert

  Install or reconcile the AMD DKMS driver only (no cluster deploy, no host
  ROCm). Useful after a manual ROCm uninstall when GPU_NODE is true and the
  node has no unsupported amdgpu-dkms/DKMS registrations left:
    sudo bloom cli bloom.yaml --tags gpu
  Bloom may end the play and offer to reboot; rerun the same command after reboot.

  Export a self-contained playbook without executing it:
    ./bloom cli bloom.yaml --export`,
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
	rootCmd.PersistentFlags().BoolVarP(&autoConfirm, "yes", "y", false, "Automatically confirm all interactive prompts (--destroy-data, cleanup, reboot-required). Same as --auto-confirm-prompts. USE WITH CAUTION")
	rootCmd.PersistentFlags().BoolVar(&autoConfirm, "auto-confirm-prompts", false, "Alias for --yes/-y")

	// Add CLI command flags
	cliCmd.Flags().StringVar(&playbookName, "playbook", "cluster-bloom.yaml", "Playbook to run (default: cluster-bloom.yaml)")
	cliCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Run in check mode without making changes")
	cliCmd.Flags().StringVar(&tags, "tags", "", "Run only Ansible tasks matching tags (e.g. gpu, validate_node, deploy_clusterforge, update_cert)")
	cliCmd.Flags().BoolVar(&destroyData, "destroy-data", false, "⚠️  DANGER: Wipes cluster (RKE2 uninstall, Longhorn cleanup, disk wipe). Shows disk preview before confirmation. Equivalent to running bloom cleanup then redeploying.")
	cliCmd.Flags().BoolVar(&pauseK3s, "pause-k3s", false, "Legacy alias: k3s conflicts are paused automatically; this flag still forces the pause step")
	cliCmd.Flags().BoolVar(&preserveRKE2, "preserve-existing-rke2", false, "Resume/reconcile an existing RKE2 installation without treating its service and state directories as data-safety conflicts")
	cliCmd.Flags().StringVar(&clusterListenIP, "cluster-listen-ip", "", "IP address or CIDR for cluster binding (e.g., 192.168.1.100 or 192.168.1.0/24)")
	cliCmd.Flags().BoolVar(&export, "export", false, "Export the playbook to ./bloom-playbook/ (overwrites if exists) instead of executing it")

	// Add run command flags
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Run in check mode without making changes")
	runCmd.Flags().StringVar(&tags, "tags", "", "Run only tasks with specific tags")
	runCmd.Flags().StringArrayVarP(&extraVars, "extra-vars", "e", nil, "Extra variables passed to ansible-playbook (repeatable)")
	runCmd.Flags().StringVarP(&configFile, "config", "c", "", "YAML config file whose keys become ansible extra vars")
	runCmd.Flags().BoolVar(&verbose, "verbose", false, "Show full Ansible output instead of clean summary")

	// Add cleanup-specific flags
	// --force/-f is a historical alias for --yes/-y, bound to the same
	// variable so either name bypasses cleanup's confirmation prompt.
	cleanupCmd.Flags().BoolVarP(&autoConfirm, "force", "f", false, "Skip confirmation prompt and force immediate cleanup. Alias for --yes/-y (USE WITH CAUTION)")
	cleanupCmd.Flags().BoolVar(&cleanupPreflightOnly, "preflight-only", false, "Validate bloom.yaml, fstab, live mounts, and protected devices without making changes")

	// Add subcommands
	rootCmd.AddCommand(webuiCmd)
	rootCmd.AddCommand(cliCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(updateCmd())

	// Keep the complete configuration reference discoverable from root help
	// without obscuring the focused help for individual subcommands.
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		appendix := "For the configuration field reference, see './bloom --help'.\n"
		if cmd == rootCmd {
			appendix = buildConfigFieldsHelp()
		}

		defaultTemplate := cmd.HelpTemplate()
		cmd.SetHelpTemplate(defaultTemplate + "\n" + appendix)
		defaultHelp(cmd, args)
		cmd.SetHelpTemplate(defaultTemplate)
	})

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

	// Inject CLI flag values into config (CLI flags override file values)
	if clusterListenIP != "" {
		cfg["CLUSTER_LISTEN_IP"] = clusterListenIP
	}

	// Validate config (after injecting CLI flags)
	// Skip validation for cert update tags to allow separate cert-update-config.yaml
	if tags == "" || (!strings.Contains(tags, "update_cert") && !strings.Contains(tags, "deploy_clusterforge")) {
		errors := config.Validate(cfg)
		if len(errors) > 0 {
			fmt.Fprintln(os.Stderr, "Configuration validation errors:")
			for _, err := range errors {
				fmt.Fprintf(os.Stderr, "  - %s\n", err)
			}
			os.Exit(1)
		}
	}

	// Internal Ansible variables: inject only after schema validation so they are
	// not rejected as unknown user-facing bloom.yaml keys.
	cfg["bloom_config_file"] = configFile
	if preserveRKE2 {
		cfg["RKE2_PRESERVE_EXISTING"] = true
	}
	if pauseK3s {
		cfg["PAUSE_K3S"] = true
	}

	// Resolve host-driver policy plus GPU Operator/DeviceConfig defaults and
	// inject them as ansible vars before export/run.
	if err := config.ApplyGPUStackVars(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving GPU stack defaults: %v\n", err)
		os.Exit(1)
	}
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

	// Tie any reboot-required marker written by ansible to this invocation.
	// This prevents a stale marker from an earlier run from triggering a reboot
	// when the current playbook fails before reaching GPU preparation.
	runID := newBloomRunID()
	cfg["bloom_run_id"] = runID

	// Handle destructive data cleanup if requested
	if destroyData {
		rancherDisk, _ := cfg["RANCHER_DISK"].(string)
		options := cleanupRunOptions{
			configWasProvided: true,
			rancherExplicit:   strings.TrimSpace(rancherDisk) != "",
		}
		storage, err := validateCleanupStorage(cfg, options)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Cleanup preflight failed: %v\n", err)
			os.Exit(1)
		}
		// Preflight validates the canonical kernel names, but the playbook keeps
		// the operator's original spelling: a /dev/disk/by-id path stays valid if
		// the kernel renumbers devices between preflight and the wipe.
		cfg["CLUSTER_DISKS"] = storage.DeployClusterDisks()
		cfg["CLUSTER_PREMOUNTED_DISKS"] = storage.PremountedDisks
		cfg["RANCHER_DISK"] = storage.DeployRancherDisk()
		options.rancherExplicit = storage.RancherExplicit
		if !confirmDestructiveOperation(cfg) {
			fmt.Println("\n❌ Operation aborted by user. No data was harmed.")
			os.Exit(0)
		}
		if err := runClusterCleanup(cfg, options); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Cleanup stopped: %v\n", err)
			os.Exit(1)
		}
	}

	// Use clean (terse/emoji) output mode by default
	mode := runtime.OutputClean

	// Run the playbook
	exitCode, err := runtime.RunPlaybook(cfg, playbookName, dryRun, tags, mode, Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Print the post-run ClusterForge summary from the host, where kubectl can
	// check for real deployment evidence. A failed playbook or a successful
	// early exit for the mapped-driver reboot did not deploy ClusterForge.
	if exitCode == 0 && !rebootRequiredForRun(runID) {
		printClusterForgeSummary(cfg, configFile, tags)
	}

	os.Exit(maybeHandleRebootRequired(exitCode, runID))
}

func runPlaybookDirect(playbookPath string) {
	mode := runtime.OutputClean
	if verbose {
		mode = runtime.OutputVerbose
	}

	var allVars []string
	runID := newBloomRunID()

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
	allVars = append(allVars, fmt.Sprintf(`{"bloom_run_id": %q}`, runID))

	exitCode, err := runtime.RunPlaybookDirect(playbookPath, dryRun, tags, allVars, mode, Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(maybeHandleRebootRequired(exitCode, runID))
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
	tweaked, err := tweakRootPlaybookForExportContent(playbookContent)
	if err != nil {
		return fmt.Errorf("tweak playbook: %w", err)
	}
	if err := os.WriteFile(playbookPath, tweaked, 0644); err != nil {
		return fmt.Errorf("write playbook: %w", err)
	}

	varsBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal vars: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "bloom-vars.yaml"), varsBytes, 0644); err != nil {
		return fmt.Errorf("write vars: %w", err)
	}

	const inventoryINI = "localhost ansible_connection=local\n"
	if err := os.WriteFile(filepath.Join(outDir, "inventory.ini"), []byte(inventoryINI), 0644); err != nil {
		return fmt.Errorf("write inventory: %w", err)
	}
	const ansibleCfg = "[defaults]\ninventory = inventory.ini\n"
	if err := os.WriteFile(filepath.Join(outDir, "ansible.cfg"), []byte(ansibleCfg), 0644); err != nil {
		return fmt.Errorf("write ansible.cfg: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Exported playbook to ./%s/\n", outDir)
	fmt.Fprintf(os.Stderr, "  Run with: cd %s && ansible-playbook %s\n", outDir, playbookName)
	return nil
}

// tweakRootPlaybookForExportContent adjusts the exported root playbook in place
// without YAML round-tripping. Go's yaml.Marshal alphabetizes map keys, which
// breaks Ansible task parsing (e.g. "become" before "command" is treated as a module).
func tweakRootPlaybookForExportContent(content []byte) ([]byte, error) {
	s := string(content)

	const hostsAll = "  hosts: all\n"
	const hostsLocalhost = "  hosts: localhost\n"
	if !strings.Contains(s, hostsAll) {
		return nil, fmt.Errorf("expected %q in playbook", strings.TrimSpace(hostsAll))
	}
	s = strings.Replace(s, hostsAll, hostsLocalhost, 1)

	const varsFilesBlock = "  vars_files:\n    - bloom-vars.yaml\n"
	if !strings.Contains(s, "vars_files:") {
		s = strings.Replace(s, hostsLocalhost, hostsLocalhost+varsFilesBlock, 1)
	}

	const bloomDirDefault = `    BLOOM_DIR: "/tmp/bloom"`
	const bloomDirExport = `    BLOOM_DIR: "{{ ansible_env.PWD | default(playbook_dir) }}"`
	switch {
	case strings.Contains(s, bloomDirDefault):
		s = strings.Replace(s, bloomDirDefault, bloomDirExport, 1)
	case strings.Contains(s, bloomDirExport):
		// already export-ready
	default:
		return nil, fmt.Errorf("expected BLOOM_DIR default %q in playbook vars", bloomDirDefault)
	}

	return []byte(s), nil
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

	if autoConfirm {
		fmt.Println("🚀 --yes/--auto-confirm-prompts set - bypassing confirmation")
		return true
	}

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
	fmt.Println("• ALL managed disk devices (wipefs + reformat)")
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

// confirmYesNo prompts a lightweight [y/N] confirmation (default No), the
// convention used for disruptive-but-recoverable actions like a reboot as
// opposed to the stricter typed-"yes" prompts used for irreversible data
// destruction. Auto-confirms without prompting when autoConfirm is set.
func confirmYesNo(prompt string) bool {
	if autoConfirm {
		fmt.Printf("%s [y/N]: y (auto-confirmed via --yes/--auto-confirm-prompts)\n", prompt)
		return true
	}

	fmt.Printf("%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}

// maybeHandleRebootRequired checks for the marker that reboot_required_check.yaml
// writes when a kernel/amdgpu driver package update needs a reboot before the
// GPU is usable. If found and not yet acted on, it offers to reboot the node
// right away; the ansible task's own loop-guard (the marker's "attempted"
// flag) prevents bloom from ever offering to reboot a second time for the
// same unresolved condition, so no special handling is needed here for that
// case beyond leaving the original exit code and letting ansible's
// manual-intervention failure message speak for itself.
//
// Deliberately does not run inside the namespaced/pivot-rooted ansible child
// process: this runs in the original top-level bloom process, which executes
// directly on the host, so `systemctl reboot` here reboots the real machine.
func newBloomRunID() string {
	return fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
}

// rebootRequiredForRun reports whether this invocation reached the driver
// reboot handoff. It also prevents a full invocation that ended early at that
// handoff from being mistaken for a completed ClusterForge deployment.
func rebootRequiredForRun(runID string) bool {
	data, err := os.ReadFile(rebootMarkerPath)
	if err != nil {
		return false
	}
	var marker rebootRequiredMarker
	return json.Unmarshal(data, &marker) == nil &&
		!marker.Attempted &&
		marker.RunID != "" &&
		marker.RunID == runID
}

func maybeHandleRebootRequired(exitCode int, runID string) int {
	// Never offer an automatic reboot after a failed playbook. In particular,
	// an early data-safety failure means no GPU task ran in this invocation.
	if exitCode != 0 {
		return exitCode
	}

	data, err := os.ReadFile(rebootMarkerPath)
	if err != nil {
		return exitCode
	}

	var marker rebootRequiredMarker
	if err := json.Unmarshal(data, &marker); err != nil ||
		marker.Attempted ||
		marker.RunID == "" ||
		marker.RunID != runID {
		return exitCode
	}

	fmt.Println("\n⏳ REBOOT REQUIRED:")
	fmt.Println(marker.Reason)
	if len(marker.Packages) > 0 {
		fmt.Println("Packages that triggered this:")
		for _, p := range marker.Packages {
			fmt.Printf("  - %s\n", p)
		}
	}
	fmt.Println("The GPU will not work correctly until this node is rebooted.")

	if !confirmYesNo("Reboot now?") {
		fmt.Println("\n⏭️(skipped) Reboot declined.")
		fmt.Println("Reboot manually when ready (`sudo reboot`) and re-run this cluster-bloom binary")
		return exitCode
	}

	marker.Attempted = true
	if updated, err := json.MarshalIndent(marker, "", "  "); err == nil {
		if err := os.WriteFile(rebootMarkerPath, updated, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update reboot marker (risk of a reboot loop if the next run hits the same issue): %v\n", err)
		}
	}

	fmt.Println("\n🔄 Rebooting now. Re-run this exact bloom command once the node is back up.")
	_ = exec.Command("sync").Run()
	if err := exec.Command("systemctl", "reboot").Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to trigger reboot via systemctl: %v\n", err)
		fmt.Fprintln(os.Stderr, "Please reboot manually: sudo reboot")
	}
	return exitCode
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

type cleanupRunOptions struct {
	configWasProvided bool
	rancherExplicit   bool
}

func validateCleanupStorage(cfg config.Config, options cleanupRunOptions) (runtime.CleanupStorage, error) {
	clusterDisks := ""
	if disks, exists := cfg["CLUSTER_DISKS"]; exists && disks != nil {
		if disksStr, ok := disks.(string); ok {
			clusterDisks = disksStr
		}
	}

	premountedDisks := ""
	if pm, exists := cfg["CLUSTER_PREMOUNTED_DISKS"]; exists && pm != nil {
		if pmStr, ok := pm.(string); ok {
			premountedDisks = pmStr
		}
	}

	rancherDisk := ""
	if rd, exists := cfg["RANCHER_DISK"]; exists && rd != nil {
		if rdStr, ok := rd.(string); ok {
			rancherDisk = rdStr
		}
	}

	storage, err := runtime.ResolveCleanupStorage(
		clusterDisks, premountedDisks, rancherDisk,
		options.configWasProvided, options.rancherExplicit)
	if err != nil {
		return runtime.CleanupStorage{}, err
	}
	if err := runtime.RunCleanupPreflight(storage); err != nil {
		return runtime.CleanupStorage{}, err
	}
	return storage, nil
}

func runClusterCleanup(cfg config.Config, options cleanupRunOptions) error {
	fmt.Println("🧹 Starting Bloom cluster cleanup...")

	// Initialize signal handling for graceful shutdown
	runtime.InitSignalHandling()

	storage, err := validateCleanupStorage(cfg, options)
	if err != nil {
		return fmt.Errorf("cleanup preflight failed: %w", err)
	}
	clusterDisks := storage.ClusterDisks
	premountedDisks := storage.PremountedDisks
	rancherDisk := storage.RancherDisk

	fmt.Printf("   ⚙️  Config: CLUSTER_DISKS=%q, CLUSTER_PREMOUNTED_DISKS=%q, RANCHER_DISK=%q\n", clusterDisks, premountedDisks, rancherDisk)
	// Step 1: Clean Longhorn Mounts (equivalent to CleanLonghornMountsStep)
	if err := runtime.CleanupLonghornMounts(); err != nil {
		return fmt.Errorf("Longhorn cleanup failed: %w", err)
	}

	// Step 2: Uninstall RKE2 (equivalent to UninstallRKE2Step)
	if err := runtime.UninstallRKE2(); err != nil {
		return fmt.Errorf("RKE2 uninstall failed: %w", err)
	}

	// Step 2.5: Process validation removed - config-independent cleanup proven sufficient

	// Step 3: Pre-clean bloom artifacts from directories in the future mount range,
	// leaving user files intact. Done before fstab is rewritten so mounts are still valid.
	if err := runtime.PrecleanFutureMountPoints(clusterDisks, premountedDisks); err != nil {
		return fmt.Errorf("future mount pre-clean failed: %w", err)
	}

	// Step 4: Clean premounted disk contents BEFORE CleanupBloomDisks strips fstab.
	// unmountPriorLonghornDisks (called inside CleanupBloomDisks) removes bloom fstab
	// entries and unmounts the disks; if we run after that, mount falls back to device
	// scan which may fail. Running here while fstab is intact guarantees the mount works.
	if err := runtime.CleanupPremountedDisks(premountedDisks); err != nil {
		return fmt.Errorf("premounted disk cleanup failed: %w", err)
	}

	// Step 4.5: Clean RANCHER_DISK configuration — unmount bind mount and clean data
	// Always call - let function decide based on actual mount status
	if err := runtime.CleanupRancherDisk(rancherDisk, storage.RancherExplicit); err != nil {
		return fmt.Errorf("RANCHER_DISK cleanup failed: %w", err)
	}

	// Step 5: Clean Disks — strips fstab entries and wipes CLUSTER_DISKS
	if err := runtime.CleanupBloomDisks(clusterDisks); err != nil {
		return fmt.Errorf("disk cleanup failed: %w", err)
	}

	fmt.Println("✅ Bloom cluster cleanup completed successfully")
	return nil
}
