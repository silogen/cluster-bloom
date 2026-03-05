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
		if err := exportPlaybook(cfg, playbookName, destroyData); err != nil {
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
func exportPlaybook(cfg config.Config, playbookName string, includeDestroyData bool) error {
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

	// Parse the playbook to inject configuration variables and inline tasks
	var playbook any
	if err := yaml.Unmarshal(playbookContent, &playbook); err != nil {
		return fmt.Errorf("parse playbook: %w", err)
	}

	// Apply configuration to playbook variables
	if err := injectConfigIntoPlaybook(playbook, cfg); err != nil {
		return fmt.Errorf("inject config: %w", err)
	}

	// Inline include_tasks references
	if err := inlineIncludedTasks(playbook, playbookDir); err != nil {
		return fmt.Errorf("inline tasks: %w", err)
	}

	// Inline manifest file content
	if err := inlineManifestFiles(playbook, playbookDir); err != nil {
		return fmt.Errorf("inline manifest files: %w", err)
	}

	// Prepend cleanup tasks if --destroy-data is requested
	if includeDestroyData {
		if err := prependCleanupTasks(playbook, cfg); err != nil {
			return fmt.Errorf("prepend cleanup tasks: %w", err)
		}
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

// inlineIncludedTasks finds and inlines include_tasks directives with actual task content
func inlineIncludedTasks(playbook any, playbookDir string) error {
	// Handle case where playbook is a list of plays
	if playsList, ok := playbook.([]any); ok && len(playsList) > 0 {
		for _, play := range playsList {
			if playMap, ok := play.(map[string]any); ok {
				if err := inlineTasksInPlay(playMap, playbookDir); err != nil {
					return err
				}
			}
		}
	} else if playbookMap, ok := playbook.(map[string]any); ok {
		// Handle case where playbook is a single play object
		if plays, ok := playbookMap["plays"].([]any); ok {
			for _, play := range plays {
				if playMap, ok := play.(map[string]any); ok {
					if err := inlineTasksInPlay(playMap, playbookDir); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// inlineTasksInPlay processes a single play and inlines include_tasks directives
func inlineTasksInPlay(play map[string]any, playbookDir string) error {
	tasks, ok := play["tasks"]
	if !ok {
		return nil
	}

	tasksList, ok := tasks.([]any)
	if !ok {
		return nil
	}

	var newTasks []any
	
	for _, task := range tasksList {
		taskMap, ok := task.(map[string]any)
		if !ok {
			newTasks = append(newTasks, task)
			continue
		}

		// Check if this is an include_tasks directive
		if includeTasksPath, exists := taskMap["include_tasks"]; exists {
			if pathStr, ok := includeTasksPath.(string); ok {
				// Read the included task file
				includedTasks, err := readIncludedTaskFile(filepath.Join(playbookDir, pathStr))
				if err != nil {
					return fmt.Errorf("read included task file %s: %w", pathStr, err)
				}

				// Copy metadata from the include_tasks directive to each included task
				for _, includedTask := range includedTasks {
					if includedTaskMap, ok := includedTask.(map[string]any); ok {
						// Copy tags if they exist on the include_tasks directive
						if tags, exists := taskMap["tags"]; exists {
							if existingTags, hasExisting := includedTaskMap["tags"]; hasExisting {
								// Merge tags if the included task already has tags
								if existingTagsList, ok := existingTags.([]any); ok {
									if includeTagsList, ok := tags.([]any); ok {
										mergedTags := append(existingTagsList, includeTagsList...)
										includedTaskMap["tags"] = mergedTags
									}
								}
							} else {
								// Add tags if included task doesn't have any
								includedTaskMap["tags"] = tags
							}
						}

						// Copy when condition if it exists on the include_tasks directive
						if when, exists := taskMap["when"]; exists {
							if existingWhen, hasExisting := includedTaskMap["when"]; hasExisting {
								// Combine when conditions with 'and'
								includedTaskMap["when"] = []any{existingWhen, when}
							} else {
								// Add when condition if included task doesn't have one
								includedTaskMap["when"] = when
							}
						}
					}
				}

				// Add all included tasks instead of the include_tasks directive
				newTasks = append(newTasks, includedTasks...)
			} else {
				newTasks = append(newTasks, task)
			}
		} else {
			// Check for block structure and inline tasks within blocks
			if block, exists := taskMap["block"]; exists {
				if blockTasks, ok := block.([]any); ok {
					var newBlockTasks []any
					for _, blockTask := range blockTasks {
						if blockTaskMap, ok := blockTask.(map[string]any); ok {
							if includeTasksPath, exists := blockTaskMap["include_tasks"]; exists {
								if pathStr, ok := includeTasksPath.(string); ok {
									includedTasks, err := readIncludedTaskFile(filepath.Join(playbookDir, pathStr))
									if err != nil {
										return fmt.Errorf("read included task file %s: %w", pathStr, err)
									}
									newBlockTasks = append(newBlockTasks, includedTasks...)
								} else {
									newBlockTasks = append(newBlockTasks, blockTask)
								}
							} else {
								newBlockTasks = append(newBlockTasks, blockTask)
							}
						} else {
							newBlockTasks = append(newBlockTasks, blockTask)
						}
					}
					taskMap["block"] = newBlockTasks
				}
			}
			newTasks = append(newTasks, task)
		}
	}

	play["tasks"] = newTasks
	return nil
}

// inlineManifestFiles finds copy tasks that reference manifest files and inlines their content
func inlineManifestFiles(playbook any, playbookDir string) error {
	// Handle case where playbook is a list of plays
	if playsList, ok := playbook.([]any); ok && len(playsList) > 0 {
		for _, play := range playsList {
			if playMap, ok := play.(map[string]any); ok {
				if err := inlineManifestsInPlay(playMap, playbookDir); err != nil {
					return err
				}
			}
		}
	} else if playbookMap, ok := playbook.(map[string]any); ok {
		// Handle case where playbook is a single play object
		if plays, ok := playbookMap["plays"].([]any); ok {
			for _, play := range plays {
				if playMap, ok := play.(map[string]any); ok {
					if err := inlineManifestsInPlay(playMap, playbookDir); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// inlineManifestsInPlay processes tasks in a play to inline manifest file content
func inlineManifestsInPlay(play map[string]any, playbookDir string) error {
	tasks, ok := play["tasks"]
	if !ok {
		return nil
	}

	if err := processTasksForManifests(tasks, playbookDir); err != nil {
		return err
	}
	return nil
}

// processTasksForManifests recursively processes tasks to inline manifest files
func processTasksForManifests(tasks any, playbookDir string) error {
	tasksList, ok := tasks.([]any)
	if !ok {
		return nil
	}

	for _, task := range tasksList {
		taskMap, ok := task.(map[string]any)
		if !ok {
			continue
		}

		// Process copy tasks
		if copyTask, exists := taskMap["copy"]; exists {
			if copyMap, ok := copyTask.(map[string]any); ok {
				if srcPath, hasSrc := copyMap["src"].(string); hasSrc {
					if isManifestPath(srcPath) {
						// Read the manifest file and inline its content
						manifestPath := filepath.Join(playbookDir, srcPath)
						content, err := os.ReadFile(manifestPath)
						if err != nil {
							return fmt.Errorf("read manifest file %s: %w", manifestPath, err)
						}

						// Replace src with content
						delete(copyMap, "src")
						copyMap["content"] = string(content)
					}
				}
			}
		}

		// Process block tasks recursively
		if block, exists := taskMap["block"]; exists {
			if err := processTasksForManifests(block, playbookDir); err != nil {
				return err
			}
		}

		// Process rescue tasks recursively
		if rescue, exists := taskMap["rescue"]; exists {
			if err := processTasksForManifests(rescue, playbookDir); err != nil {
				return err
			}
		}

		// Process always tasks recursively
		if always, exists := taskMap["always"]; exists {
			if err := processTasksForManifests(always, playbookDir); err != nil {
				return err
			}
		}
	}
	return nil
}

// isManifestPath checks if a path references a manifest file
func isManifestPath(path string) bool {
	return strings.HasPrefix(path, "manifests/") && strings.HasSuffix(path, ".yaml")
}

// readIncludedTaskFile reads and parses an included task file
func readIncludedTaskFile(taskFilePath string) ([]any, error) {
	if _, err := os.Stat(taskFilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("task file not found: %s", taskFilePath)
	}

	content, err := os.ReadFile(taskFilePath)
	if err != nil {
		return nil, fmt.Errorf("read task file: %w", err)
	}

	var tasks []any
	if err := yaml.Unmarshal(content, &tasks); err != nil {
		return nil, fmt.Errorf("parse task file: %w", err)
	}

	return tasks, nil
}

// prependCleanupTasks adds cluster cleanup tasks to the beginning of the playbook when --destroy-data is used
func prependCleanupTasks(playbook any, cfg config.Config) error {
	// Extract CLUSTER_DISKS from config
	clusterDisks := ""
	if disks, exists := cfg["CLUSTER_DISKS"]; exists && disks != nil {
		if disksStr, ok := disks.(string); ok {
			clusterDisks = disksStr
		}
	}

	// Create cleanup tasks
	cleanupTasks := []map[string]any{
		{
			"name": "⚠️ DESTRUCTIVE CLEANUP: Remove existing Bloom cluster installation",
			"tags": []string{"cleanup", "destroy-data"},
			"block": []map[string]any{
				{
					"name": "Display destructive operation warning",
					"debug": map[string]any{
						"msg": []string{
							"⚠️  DANGER: DESTRUCTIVE OPERATION IN PROGRESS ⚠️",
							"",
							"This playbook will PERMANENTLY DESTROY:",
							"• Entire Kubernetes cluster (RKE2 uninstall)",
							"• ALL Longhorn storage volumes and data",
							"• ALL managed disk devices (wipefs + deletion)",
							fmt.Sprintf("• All data on storage devices: %s", clusterDisks),
							"",
							"This action cannot be undone.",
						},
					},
				},
				{
					"name": "Stop and disable RKE2 server service",
					"systemd": map[string]any{
						"name":    "rke2-server",
						"state":   "stopped",
						"enabled": false,
					},
					"failed_when": false,
				},
				{
					"name": "Stop and disable RKE2 agent service", 
					"systemd": map[string]any{
						"name":    "rke2-agent",
						"state":   "stopped",
						"enabled": false,
					},
					"failed_when": false,
				},
				{
					"name": "Clean Longhorn mounts and processes",
					"shell": "pkill -f longhorn || true; for mount in $(mount | grep longhorn | awk '{print $3}'); do umount \"$mount\" 2>/dev/null || true; done; rm -rf /var/lib/longhorn/* 2>/dev/null || true; echo 'Longhorn cleanup completed'",
					"register": "longhorn_cleanup",
					"failed_when": false,
				},
				{
					"name": "Run RKE2 uninstall script",
					"shell": "/usr/local/bin/rke2-uninstall.sh",
					"register": "rke2_uninstall",
					"failed_when": false,
				},
				{
					"name": "Clean RKE2 directories and files",
					"shell": "rm -rf /var/lib/rancher/rke2 /etc/rancher/rke2 /var/lib/kubelet /var/log/pods /var/log/containers; rm -f /usr/local/bin/rke2* /usr/local/bin/kubectl /usr/local/bin/crictl /usr/local/bin/ctr; echo 'RKE2 cleanup completed'",
					"register": "rke2_cleanup",
					"failed_when": false,
				},
			},
		},
	}

	// Add disk cleanup if CLUSTER_DISKS is specified
	if clusterDisks != "" {
		diskCleanupTask := map[string]any{
			"name": "Clean and wipe cluster disks",
			"tags": []string{"cleanup", "destroy-data", "storage"},
			"block": []map[string]any{
				{
					"name": "Convert CLUSTER_DISKS to list for cleanup",
					"set_fact": map[string]any{
						"cluster_disks_cleanup_list": fmt.Sprintf("{{ '%s'.split(',') }}", clusterDisks),
					},
				},
				{
					"name": "Unmount cluster disks",
					"shell": "umount {{ item }} 2>/dev/null || true",
					"loop": "{{ cluster_disks_cleanup_list }}",
					"failed_when": false,
				},
				{
					"name": "Remove fstab entries for cluster disks",
					"lineinfile": map[string]any{
						"path":   "/etc/fstab",
						"regexp": "{{ item | regex_escape }}",
						"state":  "absent",
					},
					"loop": "{{ cluster_disks_cleanup_list }}",
					"failed_when": false,
				},
				{
					"name": "Wipe filesystem signatures from cluster disks",
					"shell": "wipefs -a {{ item }} 2>/dev/null || true",
					"loop": "{{ cluster_disks_cleanup_list }}",
					"failed_when": false,
				},
				{
					"name": "Remove mount point directories",
					"file": map[string]any{
						"path":  "/mnt/disk{{ ansible_loop.index0 }}",
						"state": "absent",
					},
					"loop": "{{ cluster_disks_cleanup_list }}",
					"loop_control": map[string]any{
						"extended": true,
					},
					"failed_when": false,
				},
			},
		}
		cleanupTasks = append(cleanupTasks, diskCleanupTask)
	}

	// Add final cleanup completion task
	finalTask := map[string]any{
		"name": "Cleanup completion summary",
		"debug": map[string]any{
			"msg": []string{
				"✅ Destructive cleanup completed",
				"• RKE2 services stopped and uninstalled",
				"• Longhorn storage cleaned",
				"• Disk devices wiped and unmounted",
				"• System ready for fresh installation",
				"",
				"Proceeding with normal cluster deployment...",
			},
		},
		"tags": []string{"cleanup", "destroy-data"},
	}
	cleanupTasks = append(cleanupTasks, finalTask)

	// Prepend cleanup tasks to the existing playbook tasks
	if playsList, ok := playbook.([]any); ok && len(playsList) > 0 {
		if firstPlay, ok := playsList[0].(map[string]any); ok {
			if tasks, ok := firstPlay["tasks"].([]any); ok {
				// Convert cleanup tasks to []any
				var cleanupTasksAny []any
				for _, task := range cleanupTasks {
					cleanupTasksAny = append(cleanupTasksAny, task)
				}
				
				// Prepend cleanup tasks to existing tasks
				newTasks := append(cleanupTasksAny, tasks...)
				firstPlay["tasks"] = newTasks
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
