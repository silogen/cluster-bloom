package runtime

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed playbooks
var embeddedPlaybooks embed.FS

const (
	WorkDir = "/var/lib/bloom"
	LogDir  = "/var/log/bloom"
)

func RunPlaybook(config map[string]any, playbookName string) (int, error) {
	rootfs := filepath.Join(WorkDir, "rootfs")
	playbookDir := filepath.Join(WorkDir, "playbooks")

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

	extraArgs := configToAnsibleVars(config)

	exitCode := RunContainer(rootfs, playbookDir, playbookName, extraArgs)
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
		args = append(args, "-e", fmt.Sprintf("%s=%v", key, value))
	}
	return args
}
