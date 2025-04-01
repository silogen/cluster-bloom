/**
 * Copyright 2025 Advanced Micro Devices, Inc.  All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
**/

package pkg

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

func InstallDependentPackages() error {
	packagesToInstall := []string{
		"open-iscsi",
		"jq",
		"nfs-common",
	}

	for _, pkg := range packagesToInstall {
		LogMessage(Debug, fmt.Sprintf("Installing package: %s", pkg))
		err := installpackage(pkg)
		if err != nil {
			return fmt.Errorf("failed to install %s: %w", pkg, err)
		}
	}
	err := installK8sTools()
	if err != nil {
		return fmt.Errorf("failed to install k8s tools: %w", err)
	}

	LogMessage(Info, "All packages installed successfully")
	return nil
}

func installpackage(pkgName string) error {
	cmd := exec.Command("apt-get", "install", "-y", pkgName)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install package: %w\nOutput: %s", err, output)
	}

	LogMessage(Debug, fmt.Sprintf("Successfully installed %s", pkgName))
	return nil
}

func installK8sTools() error {
	cmds := [][]string{
		{"snap", "install", "kubectl", "--classic"},
		{"snap", "install", "k9s"},
	}

	for _, cmd := range cmds {
		err := exec.Command("sudo", cmd...).Run()
		if err != nil {
			return fmt.Errorf("failed to execute command %v: %w", cmd, err)
		}
	}

	LogMessage(Info, "Kubernetes tools installed successfully.")
	return nil
}

func setupLonghorn() error {
	targetDir := "/var/lib/rancher/rke2/server/manifests"
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
	}
	err := fs.WalkDir(manifestFiles, "manifests", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".yaml" {
			content, err := manifestFiles.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", path, err)
			}
			targetPath := filepath.Join(targetDir, filepath.Base(path))
			if err := os.WriteFile(targetPath, content, 0644); err != nil {
				return fmt.Errorf("failed to write file %s: %w", targetPath, err)
			}
			LogMessage(Info, fmt.Sprintf("Copied %s to %s", path, targetPath))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to copy template files: %w", err)
	}

	LogMessage(Info, "Longhorn setup completed successfully")
	return nil
}

func setupAudit() error {
	sourceFile := "templates/audit-policy.yaml"
	targetDir := "/etc/rancher/rke2"
	targetPath := filepath.Join(targetDir, filepath.Base(sourceFile))

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
	}

	content, err := templateFiles.ReadFile(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", sourceFile, err)
	}

	if err := os.WriteFile(targetPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", targetPath, err)
	}

	LogMessage(Info, fmt.Sprintf("Copied %s to %s", sourceFile, targetPath))
	LogMessage(Info, "Audit policy setup completed successfully")
	return nil
}
