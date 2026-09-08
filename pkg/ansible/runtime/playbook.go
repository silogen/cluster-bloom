package runtime

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed playbooks
var embeddedPlaybooks embed.FS

func getWorkDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get current directory: %w", err)
	}
	return filepath.Join(cwd, ".bloom"), nil
}

func backupLogFile(workDir string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	logPath := filepath.Join(cwd, "bloom.log")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return nil
	}

	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(cwd, fmt.Sprintf("bloom-%s.log", timestamp))

	if err := os.Rename(logPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup bloom.log: %w", err)
	}

	fmt.Printf("Backed up bloom.log to %s\n", filepath.Base(backupPath))
	return nil
}

func RunPlaybook(config map[string]any, playbookName string, dryRun bool, tags string, outputMode OutputMode, version string) (int, error) {
	workDir, err := getWorkDir()
	if err != nil {
		return 1, err
	}

	playbookDir := filepath.Join(workDir, "playbooks")

	os.RemoveAll(playbookDir)
	if err := extractEmbeddedPlaybooks(playbookDir); err != nil {
		return 1, fmt.Errorf("extract playbooks: %w", err)
	}

	if err := ExtractManifests(playbookDir); err != nil {
		return 1, fmt.Errorf("extract manifests: %w", err)
	}

	extraVars, err := ConfigToAnsibleVars(config)
	if err != nil {
		return 1, err
	}
	playbookPath := filepath.Join(playbookDir, playbookName)

	return RunPlaybookDirect(playbookPath, dryRun, tags, extraVars, outputMode, version)
}

func extractEmbeddedPlaybooks(destDir string) error {
	return fs.WalkDir(embeddedPlaybooks, "playbooks", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel("playbooks", path)
		if relPath == "." {
			return os.MkdirAll(destDir, 0755)
		}

		destPath := filepath.Join(destDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		content, err := embeddedPlaybooks.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, content, 0644)
	})
}

func RunPlaybookDirect(playbookPath string, dryRun bool, tags string, extraVars []string, outputMode OutputMode, version string) (int, error) {
	absPath, err := filepath.Abs(playbookPath)
	if err != nil {
		return 1, fmt.Errorf("resolve playbook path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return 1, fmt.Errorf("playbook not found: %s", absPath)
	}

	playbookDir := filepath.Dir(absPath)
	playbookName := filepath.Base(absPath)

	workDir, err := getWorkDir()
	if err != nil {
		return 1, err
	}

	if err := backupLogFile(workDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	rootfs := filepath.Join(workDir, "rootfs")

	if !ImageCached(rootfs) {
		fmt.Println("Downloading Ansible runtime image (this may take a few minutes)...")
		if err := os.MkdirAll(rootfs, 0755); err != nil {
			return 1, fmt.Errorf("create rootfs dir: %w", err)
		}
		if err := PullAndExtractImage(ImageRef, rootfs, true); err != nil {
			return 1, fmt.Errorf("pull image: %w", err)
		}
		fmt.Println("Image ready.")
	} else {
		fmt.Println("Using cached Ansible runtime image.")
	}

	var extraArgs []string
	for _, v := range extraVars {
		extraArgs = append(extraArgs, "-e", v)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return 1, fmt.Errorf("get current directory: %w", err)
	}
	extraArgs = append(extraArgs, "-e", fmt.Sprintf(`{"BLOOM_DIR": "%s"}`, cwd))
	extraArgs = append(extraArgs, "-e", fmt.Sprintf(`{"BLOOM_VERSION": "%s"}`, version))

	exitCode := RunContainer(rootfs, playbookDir, playbookName, extraArgs, dryRun, tags, outputMode)
	return exitCode, nil
}

// ExtractEmbeddedPlaybooksToDir extracts embedded playbooks to the specified directory
func ExtractEmbeddedPlaybooksToDir(destDir string) error {
	return extractEmbeddedPlaybooks(destDir)
}

// ConfigToAnsibleVars renders each config key as its own `-e '{"KEY": value}'`
// argument.
//
// Everything goes through the JSON encoder, strings included. Formatting a
// string with fmt produced a broken argument for any value containing a quote,
// a backslash or a newline - RKE2_EXTRA_CONFIG and CILIUM_HELM_VALUES are
// multi-line by design, and ansible-playbook parses a -e value that starts with
// `{` using a YAML loader, which either folds the newlines into spaces or
// rejects the run outright. These are argv entries, not shell words, so JSON
// escaping is the only escaping needed.
func ConfigToAnsibleVars(config map[string]any) ([]string, error) {
	vars := make([]string, 0, len(config))
	for key, value := range config {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		// Keep URLs and shell-ish values readable in logs and in bloom.log;
		// & is valid JSON but makes every diff harder to read.
		enc.SetEscapeHTML(false)
		if err := enc.Encode(map[string]any{key: value}); err != nil {
			return nil, fmt.Errorf("encode ansible var %s: %w", key, err)
		}
		vars = append(vars, strings.TrimRight(buf.String(), "\n"))
	}
	return vars, nil
}
