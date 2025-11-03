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
	_ "embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

//go:embed scripts/longhorn_preflight_check.sh
var longhornPreflightScript []byte

//go:embed scripts/longhorn_validate_pvc_creation.sh
var longhornPVCValidationScript []byte

func CheckPackageInstallConnections() error {
	cmd := exec.Command("apt-get", "update")
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to verify apt-get connection: %w\nOutput: %s", err, output)
	}

	cmd = exec.Command("snap", "info", "core")
	cmd.Env = os.Environ()
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to verify snap connection: %w\nOutput: %s", err, output)
	}

	osRelease, err := exec.Command("sh", "-c", "grep VERSION_CODENAME /etc/os-release | cut -d= -f2").Output()
	if err != nil {
		return fmt.Errorf("Error getting Ubuntu codename: %w", err)
	}
	ubuntuCodename := strings.TrimSpace(string(osRelease))

	clusterForgeUrl := viper.GetString("CLUSTERFORGE_RELEASE")
	rocmUrl := viper.GetString("ROCM_BASE_URL") + ubuntuCodename + "/" + viper.GetString("ROCM_DEB_PACKAGE")
	rke2Url := viper.GetString("RKE2_INSTALLATION_URL")

	var otherRepositories = []string{clusterForgeUrl, rocmUrl, rke2Url}
	for _, url := range otherRepositories {
		cmd = exec.Command("wget", "--spider", url)
		cmd.Env = os.Environ()
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Failed to verify download from repository: %w\nOutput: %s", err, output)
		}
	}

	LogMessage(Debug, "Package installation connections are available")
	return nil
}

func InstallDependentPackages() error {
	packagesToInstall := []string{
		"open-iscsi",
		"jq",
		"nfs-common",
		"chrony",
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
		{"snap", "install", "helm", "--classic"},
		{"snap", "install", "yq"},
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

func setupManifests(folder string) error {
	targetDir := rke2ManifestDirectory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
	}
	err := fs.WalkDir(manifestFiles, filepath.Join("manifests", folder), func(path string, d fs.DirEntry, err error) error {
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

func SetupClusterForge() error {
	url := viper.GetString("CLUSTERFORGE_RELEASE")
	if url == "none" {
		LogMessage(Info, "Not installing ClusterForge as CLUSTERFORGE_RELEASE is set to none")
		return nil
	}
	cmd := exec.Command("wget", url, "-O", "clusterforge.tar.gz")
	output, err := cmd.Output()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to download ClusterForge: %v, output: %v", err, output))
		return err
	} else {
		LogMessage(Info, "Successfully downloaded ClusterForge")
	}

	cmd = exec.Command("tar", "-xzvf", "clusterforge.tar.gz")
	output, err = cmd.Output()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to unzip clusterforge.tar.gz: %v, output %v", err, output))
		return err
	} else {
		LogMessage(Info, "Successfully unzipped clusterforge.tar.gz")
	}

	domain := viper.GetString("DOMAIN")
	valuesFile := viper.GetString("CF_VALUES")

	// Get the original user when running with sudo
	originalUser := os.Getenv("SUDO_USER")

	cmd = exec.Command("sudo", "chown", "-R", fmt.Sprintf("%s:%s", originalUser, originalUser), "cluster-forge")
	output, err = cmd.Output()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to change ownership of Clusterforge folder: %v, output %v", err, output))
		return err
	} else {
		LogMessage(Info, fmt.Sprintf("Successfully updated ownership of Clusterforge folder to %s", originalUser))
	}

	scriptsDir := "cluster-forge/scripts"

	if originalUser != "" {
		// Run as the original user to avoid sudo issues with bootstrap script
		if valuesFile != "" {
			cmd = exec.Command("sudo", "-u", originalUser, "bash", "./bootstrap.sh", domain, valuesFile)
		} else {
			cmd = exec.Command("sudo", "-u", originalUser, "bash", "./bootstrap.sh", domain)
		}
	} else {
		// Fallback if not running with sudo
		if valuesFile != "" {
			cmd = exec.Command("bash", "./bootstrap.sh", domain, valuesFile)
		} else {
			cmd = exec.Command("bash", "./bootstrap.sh", domain)
		}
	}
	cmd.Dir = scriptsDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to install ClusterForge: %v", err))
		LogMessage(Error, fmt.Sprintf("ClusterForge bootstrap script output: %s", string(output)))
		return err
	} else {
		LogMessage(Info, fmt.Sprintf("ClusterForge deployment output: %s", output))
	}
	return nil
}

func LonghornPreflightCheck() error {
	// runs a system-level check to ensure Longhorn can be installed successfully
	cmd := exec.Command("bash", "-s")
	cmd.Stdin = strings.NewReader(string(longhornPreflightScript))
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		LogMessage(Error, fmt.Sprintf("Longhorn preflight check failed: %v", err))
		return err
	}

	LogMessage(Info, "Longhorn preflight check completed successfully")
	return nil
}

func LonghornValidatePVCCreation() error {
	// runs a cluster-level check to ensure Longhorn can create PVCs successfully
	cmd := exec.Command("bash", "-s")
	cmd.Stdin = strings.NewReader(string(longhornPVCValidationScript))
	cmd.Env = os.Environ()

	if err := cmd.Run(); err != nil {
		LogMessage(Error, fmt.Sprintf("Longhorn PVC creation check failed: %v", err))
		return err
	}

	LogMessage(Info, "Longhorn PVC creation check completed successfully")
	return nil
}
