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
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

func setupManifests() error {
	targetDir := rke2ManifestDirectory
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

func SetupOnePasswordSecret() error {
	token := viper.GetString("ONEPASS_CONNECT_TOKEN")
	if token == "" {
		LogMessage(Info, "ONEPASS_CONNECT_TOKEN is not set. Skipping secret creation.")
		return nil
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}
	ctx := context.Background()

	namespace := "external-secrets"
	_, err = clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = clientset.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace,
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create namespace %s: %w", namespace, err)
			}
			LogMessage(Info, fmt.Sprintf("Namespace %s created successfully", namespace))
		} else {
			return fmt.Errorf("failed to check if namespace %s exists: %w", namespace, err)
		}
	} else {
		LogMessage(Debug, fmt.Sprintf("Namespace %s already exists, skipping creation", namespace))
	}

	secretName := "onepassword-connect-token"
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"token": []byte(token),
		},
	}

	_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create secret %s in namespace %s: %w", secretName, namespace, err)
	}

	LogMessage(Info, fmt.Sprintf("Secret %s created successfully in namespace %s", secretName, namespace))
	return nil
}

func SetupClusterForge() error {
	cmd := exec.Command("wget", "https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz")
	output, err := cmd.Output()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to download ClusterForge: %v", err))
		return err
	} else {
		LogMessage(Info, fmt.Sprintf("Successfully downloaded ClusterForge"))
	}

	cmd = exec.Command("tar", "-xzvf", "deploy-release.tar.gz")
	output, err = cmd.Output()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to unzip deploy-release.tar.gz: %v", err))
		return err
	} else {
		LogMessage(Info, fmt.Sprintf("Successfully unzipped deploy-release.tar.gz"))
	}

	cmd = exec.Command("sudo", "bash", "core/deploy.sh")
	output, err = cmd.Output()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to install ClusterForge: %v", err))
		return err
	} else {
		LogMessage(Info, fmt.Sprintf("ClusterForge deployment output: %s", output))
	}
	return nil
}
