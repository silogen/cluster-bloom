package runtime

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed manifests/longhorn/*.yaml
var longhornManifests embed.FS

//go:embed manifests/local-path/*.yaml
var localPathManifests embed.FS

//go:embed manifests/scripts/*.sh
var scriptsManifests embed.FS

// ExtractManifests extracts embedded manifests to the specified playbook directory
func ExtractManifests(playbookDir string) error {
	manifestsDir := filepath.Join(playbookDir, "manifests")

	// Extract Longhorn manifests
	if err := extractFS(longhornManifests, "manifests/longhorn", filepath.Join(manifestsDir, "longhorn")); err != nil {
		return fmt.Errorf("extract longhorn manifests: %w", err)
	}

	// Extract local-path manifests
	if err := extractFS(localPathManifests, "manifests/local-path", filepath.Join(manifestsDir, "local-path")); err != nil {
		return fmt.Errorf("extract local-path manifests: %w", err)
	}

	// Extract scripts
	if err := extractFS(scriptsManifests, "manifests/scripts", filepath.Join(manifestsDir, "scripts")); err != nil {
		return fmt.Errorf("extract scripts: %w", err)
	}

	return nil
}

// extractFS extracts files from an embedded filesystem to the destination directory
func extractFS(embedFS embed.FS, srcPath, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create dest dir %s: %w", destDir, err)
	}

	return fs.WalkDir(embedFS, srcPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Read embedded file
		data, err := embedFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded file %s: %w", path, err)
		}

		// Calculate relative path and destination
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return fmt.Errorf("calculate relative path: %w", err)
		}
		destPath := filepath.Join(destDir, relPath)

		// Write to destination
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return fmt.Errorf("write file %s: %w", destPath, err)
		}

		return nil
	})
}
