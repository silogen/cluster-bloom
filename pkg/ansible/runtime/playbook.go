package runtime

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

func RunPlaybook(config map[string]any, playbookName string, dryRun bool, tags string) (int, error) {
	workDir, err := getWorkDir()
	if err != nil {
		return 1, err
	}

	if err := backupLogFile(workDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	rootfs := filepath.Join(workDir, "rootfs")
	playbookDir := filepath.Join(workDir, "playbooks")

	if !ImageCached(rootfs) {
		fmt.Println("Downloading Ansible image (this may take a few minutes)...")
		if err := os.MkdirAll(rootfs, 0755); err != nil {
			return 1, fmt.Errorf("create rootfs dir: %w", err)
		}
		if err := PullAndExtractImage(ImageRef, rootfs, true); err != nil {
			return 1, fmt.Errorf("pull image: %w", err)
		}
		fmt.Println("Image ready.")
	} else {
		fmt.Println("Using cached Ansible image.")
	}

	os.RemoveAll(playbookDir)
	if err := extractEmbeddedPlaybooks(playbookDir); err != nil {
		return 1, fmt.Errorf("extract playbooks: %w", err)
	}

	if err := ExtractManifests(playbookDir); err != nil {
		return 1, fmt.Errorf("extract manifests: %w", err)
	}

	extraArgs := configToAnsibleVars(config)

	// Add BLOOM_DIR to Ansible variables (current working directory, not .bloom subdir)
	cwd, err := os.Getwd()
	if err != nil {
		return 1, fmt.Errorf("get current directory: %w", err)
	}
	extraArgs = append(extraArgs, "-e", fmt.Sprintf(`{"BLOOM_DIR": "%s"}`, cwd))

	exitCode := RunContainer(rootfs, playbookDir, playbookName, extraArgs, dryRun, tags)
	return exitCode, nil
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

func configToAnsibleVars(config map[string]any) []string {
	var args []string
	for key, value := range config {
		// Use @file syntax to pass values as JSON to preserve types
		// This ensures booleans stay as booleans, not strings
		var valueStr string
		switch v := value.(type) {
		case bool:
			// Pass as JSON boolean
			if v {
				valueStr = "true"
			} else {
				valueStr = "false"
			}
			// Use JSON format to preserve boolean type
			args = append(args, "-e", fmt.Sprintf(`{"`+key+`": `+valueStr+`}`))
		case string:
			// Quote strings in JSON
			valueStr = fmt.Sprintf(`"%s"`, v)
			args = append(args, "-e", fmt.Sprintf(`{"`+key+`": `+valueStr+`}`))
		default:
			// Numbers and other types
			valueStr = fmt.Sprintf("%v", v)
			args = append(args, "-e", fmt.Sprintf(`{"`+key+`": `+valueStr+`}`))
		}
	}
	return args
}
