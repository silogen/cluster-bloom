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
	extraVars    []string
	verbose      bool
	configFile   string
	export       bool
	showVersion  bool
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
		Long: `Bloom - tool for deploying and managing Kubernetes clusters.

Commands:
  webui     Launch web UI configuration wizard (default action)
  cli       Deploy cluster from a bloom.yaml config file
  cleanup   Remove an existing cluster installation
  run       Execute an exported Ansible playbook
  version   Show version information

Common workflows:

  # Deploy a cluster
  sudo ./bloom cli bloom.yaml

  # Tear down a cluster (keeps disk data)
  sudo ./bloom cleanup bloom.yaml

  # Tear down a cluster and wipe all disk data  ⚠️ IRREVERSIBLE
  sudo ./bloom cleanup bloom.yaml --destroy-data

  # Fresh redeploy (teardown + redeploy in one step)  ⚠️ IRREVERSIBLE
  sudo ./bloom cli bloom.yaml --destroy-data

  # Export playbook for inspection
  ./bloom cli bloom.yaml --export > deployment.yaml`,
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

This command performs the equivalent of Bloom v1 cleanup operations:
- Stops Longhorn services and unmounts all Longhorn-related storage
- Executes RKE2 uninstall script to remove RKE2 components
- Cleans up bloom-managed disks and removes temp drives

Optionally provide a config file so CLUSTER_DISKS and CLUSTER_PREMOUNTED_DISKS are known.
Use --destroy-data to also wipe all disk data (requires config file for full disk coverage).
By default, this command requires confirmation before proceeding. Use --force to skip confirmation.`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			checkRootPrivileges("cleanup")

			// Load config if a config file was provided
			var cfg config.Config
			if len(args) == 1 {
				var err error
				cfg, err = config.LoadConfig(args[0])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
					os.Exit(1)
				}
			}

			if !forceCleanup {
				if destroyData {
					if !confirmDestructiveOperation(cfg) {
						fmt.Println("\n❌ Operation aborted by user. No data was harmed.")
						os.Exit(0)
					}
				} else {
					if !confirmCleanupOperation() {
						fmt.Println("❌ Cleanup aborted by user.")
						os.Exit(0)
					}
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
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version information")

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
	cleanupCmd.Flags().BoolVar(&destroyData, "destroy-data", false, "⚠️  DANGER: Permanently destroys ALL cluster data, storage, and disks. Requires interactive confirmation.")

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

	// Fix hosts value for export (should be localhost instead of all)
	if err := fixHostsValueForExport(playbook); err != nil {
		return fmt.Errorf("fix hosts value: %w", err)
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

				// Add BLOOM_DIR variable to vars section for export
				// This variable is normally injected at runtime as the current working directory
				// For exported playbooks, use ansible's built-in playbook_dir variable or current working directory
				if _, exists := vars["BLOOM_DIR"]; !exists {
					vars["BLOOM_DIR"] = "{{ ansible_env.PWD | default(playbook_dir) }}"
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

					// Add BLOOM_DIR variable to vars section for export
					// This variable is normally injected at runtime as the current working directory
					// For exported playbooks, use ansible's built-in playbook_dir variable or current working directory
					if _, exists := vars["BLOOM_DIR"]; !exists {
						vars["BLOOM_DIR"] = "{{ ansible_env.PWD | default(playbook_dir) }}"
					}
				}
			}
		}
	}
	return nil
}

// fixHostsValueForExport changes hosts from 'all' to 'localhost' for exported playbooks
func fixHostsValueForExport(playbook any) error {
	// Handle case where playbook is a list of plays (most common format)
	if playsList, ok := playbook.([]any); ok && len(playsList) > 0 {
		if firstPlay, ok := playsList[0].(map[string]any); ok {
			firstPlay["hosts"] = "localhost"
		}
	} else if playbookMap, ok := playbook.(map[string]any); ok {
		// Handle case where playbook is a single play object
		if plays, ok := playbookMap["plays"].([]any); ok && len(plays) > 0 {
			if firstPlay, ok := plays[0].(map[string]any); ok {
				firstPlay["hosts"] = "localhost"
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

	tasksList, ok := tasks.([]any)
	if !ok {
		return nil
	}

	newTasks, err := processTasksForManifestInlining(tasksList, playbookDir)
	if err != nil {
		return err
	}

	play["tasks"] = newTasks
	return nil
}

// processTasksForManifestInlining recursively processes tasks to inline manifest files and expand loops
func processTasksForManifestInlining(tasksList []any, playbookDir string) ([]any, error) {
	var newTasksList []any

	for _, task := range tasksList {
		taskMap, ok := task.(map[string]any)
		if !ok {
			newTasksList = append(newTasksList, task)
			continue
		}

		// Process copy tasks with loops (e.g., manifests/local-path/{{ item }})
		if copyTask, exists := taskMap["copy"]; exists {
			if copyMap, ok := copyTask.(map[string]any); ok {
				if srcPath, hasSrc := copyMap["src"].(string); hasSrc {
					if _, hasLoop := taskMap["loop"]; hasLoop && isManifestPathWithVariable(srcPath) {
						// Expand loop into individual copy tasks
						expandedTasks, err := expandLoopCopyTask(taskMap, copyMap, playbookDir)
						if err != nil {
							return nil, fmt.Errorf("expand loop copy task: %w", err)
						}
						newTasksList = append(newTasksList, expandedTasks...)
						continue
					} else if isManifestPath(srcPath) {
						// Handle single file copy task
						if err := inlineManifestContent(copyMap, srcPath, playbookDir); err != nil {
							return nil, err
						}
					}
				}
			}
		}

		// Process template tasks (e.g., local-path-config.yaml)
		if templateTask, exists := taskMap["template"]; exists {
			if templateMap, ok := templateTask.(map[string]any); ok {
				if srcPath, hasSrc := templateMap["src"].(string); hasSrc {
					if isManifestPath(srcPath) {
						// Convert template to copy with inlined content
						if err := convertTemplateTaskToCopy(taskMap, templateMap, srcPath, playbookDir); err != nil {
							return nil, fmt.Errorf("convert template task: %w", err)
						}
					}
				}
			}
		}

		// Process block tasks recursively
		if block, exists := taskMap["block"]; exists {
			if blockList, ok := block.([]any); ok {
				newBlock, err := processTasksForManifestInlining(blockList, playbookDir)
				if err != nil {
					return nil, err
				}
				taskMap["block"] = newBlock
			}
		}

		// Process rescue tasks recursively
		if rescue, exists := taskMap["rescue"]; exists {
			if rescueList, ok := rescue.([]any); ok {
				newRescue, err := processTasksForManifestInlining(rescueList, playbookDir)
				if err != nil {
					return nil, err
				}
				taskMap["rescue"] = newRescue
			}
		}

		// Process always tasks recursively
		if always, exists := taskMap["always"]; exists {
			if alwaysList, ok := always.([]any); ok {
				newAlways, err := processTasksForManifestInlining(alwaysList, playbookDir)
				if err != nil {
					return nil, err
				}
				taskMap["always"] = newAlways
			}
		}

		newTasksList = append(newTasksList, task)
	}

	return newTasksList, nil
}

// isManifestPath checks if a path references a manifest file
func isManifestPath(path string) bool {
	return strings.HasPrefix(path, "manifests/") && (strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".sh"))
}

// isManifestPathWithVariable checks if a path references a manifest with Ansible variables
func isManifestPathWithVariable(path string) bool {
	return strings.HasPrefix(path, "manifests/") && strings.Contains(path, "{{")
}

// inlineManifestContent reads manifest content and inlines it into a copy task
func inlineManifestContent(copyMap map[string]any, srcPath, playbookDir string) error {
	manifestPath := filepath.Join(playbookDir, srcPath)
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest file %s: %w", manifestPath, err)
	}

	// Replace src with content
	delete(copyMap, "src")
	copyMap["content"] = string(content)
	return nil
}

// expandLoopCopyTask expands a copy task with loop into individual copy tasks
func expandLoopCopyTask(taskMap map[string]any, copyMap map[string]any, playbookDir string) ([]any, error) {
	srcPath := copyMap["src"].(string)
	destPath, hasDest := copyMap["dest"].(string)
	if !hasDest {
		return nil, fmt.Errorf("copy task missing dest")
	}

	loop, hasLoop := taskMap["loop"]
	if !hasLoop {
		return nil, fmt.Errorf("expected loop in task")
	}

	loopItems, ok := loop.([]any)
	if !ok {
		return nil, fmt.Errorf("loop must be a list")
	}

	var expandedTasks []any

	// Extract task metadata (name, tags, etc.)
	taskName := "Copy manifest"
	if name, hasName := taskMap["name"].(string); hasName {
		taskName = name
	}

	for _, item := range loopItems {
		itemStr, ok := item.(string)
		if !ok {
			continue
		}

		// Replace {{ item }} with actual item
		actualSrcPath := strings.ReplaceAll(srcPath, "{{ item }}", itemStr)
		actualDestPath := strings.ReplaceAll(destPath, "{{ item }}", itemStr)

		// Read manifest content
		manifestPath := filepath.Join(playbookDir, actualSrcPath)
		content, err := os.ReadFile(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("read manifest file %s: %w", manifestPath, err)
		}

		// Create individual copy task
		newTask := map[string]any{
			"name": fmt.Sprintf("%s (%s)", taskName, itemStr),
			"copy": map[string]any{
				"content": string(content),
				"dest":    actualDestPath,
				"mode":    copyMap["mode"], // Preserve mode if present
			},
		}

		// Copy other task properties (tags, when, etc.)
		for key, value := range taskMap {
			if key != "copy" && key != "loop" && key != "name" {
				newTask[key] = value
			}
		}

		expandedTasks = append(expandedTasks, newTask)
	}

	return expandedTasks, nil
}

// convertTemplateTaskToCopy converts a template task to a copy task with inlined content
func convertTemplateTaskToCopy(taskMap map[string]any, templateMap map[string]any, srcPath, playbookDir string) error {
	// For now, we'll treat template files as regular files and inline their content
	// This is a simplification - full template processing would require variable substitution
	manifestPath := filepath.Join(playbookDir, srcPath)
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read template file %s: %w", manifestPath, err)
	}

	// Convert template to copy
	delete(taskMap, "template")
	taskMap["copy"] = map[string]any{
		"content": string(content),
		"dest":    templateMap["dest"],
		"mode":    templateMap["mode"],
	}

	// Add a warning comment that this was converted from a template
	if name, hasName := taskMap["name"].(string); hasName {
		taskMap["name"] = fmt.Sprintf("%s (converted from template)", name)
	}

	return nil
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

	// Extract CLUSTER_PREMOUNTED_DISKS from config
	premountedDisks := ""
	if pm, exists := cfg["CLUSTER_PREMOUNTED_DISKS"]; exists && pm != nil {
		if pmStr, ok := pm.(string); ok {
			premountedDisks = pmStr
		}
	}

	// Use the DRY cleanup task generator from runtime package
	cleanupTasks := runtime.GenerateCleanupTasks(clusterDisks, premountedDisks)

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
	fmt.Println("• All Longhorn storage volumes and data")
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

	// Extract premounted disks config once for use in steps below
	premountedDisks := ""
	if pm, exists := cfg["CLUSTER_PREMOUNTED_DISKS"]; exists && pm != nil {
		if pmStr, ok := pm.(string); ok {
			premountedDisks = pmStr
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

	// Step 3: Clean premounted disk contents BEFORE CleanupBloomDisks strips fstab.
	// unmountPriorLonghornDisks (called inside CleanupBloomDisks) removes bloom fstab
	// entries and unmounts the disks; if we run after that, mount falls back to device
	// scan which may fail. Running here while fstab is intact guarantees the mount works.
	if err := runtime.CleanupPremountedDisks(premountedDisks); err != nil {
		errors = append(errors, fmt.Errorf("Premounted disk cleanup: %w", err))
	}

	// Step 4: Clean Disks — strips fstab entries and wipes CLUSTER_DISKS.
	// Pass premountedDisks so those mount points' fstab entries are preserved
	// and the disks stay mounted for the subsequent bloom deployment.
	if err := runtime.CleanupBloomDisks(clusterDisks, premountedDisks); err != nil {
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
