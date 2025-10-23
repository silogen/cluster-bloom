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
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var rke2ConfigContent = `
cni: cilium
cluster-cidr: 10.242.0.0/16
service-cidr: 10.243.0.0/16

disable: rke2-ingress-nginx
audit-log-path: "/var/lib/rancher/rke2/server/logs/kube-apiserver-audit.log"
audit-log-maxage: 30
audit-log-maxbackup: 10
audit-log-maxsize: 100
audit-policy-file: "/etc/rancher/rke2/audit-policy.yaml"
`

var oidcConfigTemplate = `
kube-apiserver-arg:
  - "--oidc-issuer-url=%s"
  - "--oidc-client-id=k8s"
  - "--oidc-username-claim=preferred_username"
  - "--oidc-groups-claim=groups"
  - "--oidc-ca-file=/etc/rancher/rke2/oidc-ca.crt"
  - "--oidc-username-prefix=oidc"
  - "--oidc-groups-prefix=oidc"
`

func FetchAndSaveOIDCCertificate(url string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("openssl s_client -showcerts -connect %s:443 </dev/null | sed -n '/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/p'", url))
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to fetch certificate from %s: %v", url, err)
	}
	if err := os.WriteFile("/etc/rancher/rke2/oidc-ca.crt", output, 0644); err != nil {
		return fmt.Errorf("failed to write certificate: %v", err)
	}
	return nil
}

func PrepareRKE2() error {
	commands := []struct {
		command string
		args    []string
	}{
		{"modprobe", []string{"iscsi_tcp"}},
		{"modprobe", []string{"dm_mod"}},
		{"mkdir", []string{"-p", "/etc/rancher/rke2"}},
		{"chmod", []string{"0755", "/etc/rancher/rke2"}},
	}

	for _, cmd := range commands {
		_, err := runCommand(cmd.command, cmd.args...)
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to execute command '%s %v': %v", cmd.command, cmd.args, err))
			return fmt.Errorf("failed to execute command '%s %v': %w", cmd.command, cmd.args, err)
		}
		LogMessage(Info, fmt.Sprintf("Successfully executed command: %s %v", cmd.command, cmd.args))
	}
	err := setupAudit()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to setup audit: %v", err))
		return fmt.Errorf("failed to setup audit policy: %w", err)
	}
	rke2ConfigPath := "/etc/rancher/rke2/config.yaml"
	if err := os.WriteFile(rke2ConfigPath, []byte(rke2ConfigContent), 0644); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to write to %s: %v", rke2ConfigPath, err))
		return err
	}

	certPath := "/etc/rancher/rke2/oidc-ca.crt"
	if _, err := os.Stat(certPath); err == nil {
		if err := os.Remove(certPath); err != nil {
			return fmt.Errorf("failed to remove existing certificate at %s: %v", certPath, err)
		}
		LogMessage(Info, fmt.Sprintf("Removed existing certificate at %s", certPath))
	}
	oidcURL := viper.GetString("OIDC_URL")
	if oidcURL != "" {
		if err := FetchAndSaveOIDCCertificate(oidcURL); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to fetch and save OIDC certificate: %v", err))
		}
		LogMessage(Info, fmt.Sprintf("Fetched and saved OIDC certificate from %s", oidcURL))
		configContent := fmt.Sprintf(oidcConfigTemplate, oidcURL)

		file, err := os.OpenFile(rke2ConfigPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open %s for appending: %v", rke2ConfigPath, err)
		}
		defer file.Close()

		if _, err := file.WriteString(configContent); err != nil {
			return fmt.Errorf("failed to append to %s: %v", rke2ConfigPath, err)
		}
	}

	return nil
}

func SetupFirstRKE2() error {
	commands := []struct {
		command string
		args    []string
	}{
		{"sh", []string{"-c", "curl -sfL " + viper.GetString("RKE2_INSTALLATION_URL") + " | sh -"}},
		{"systemctl", []string{"enable", "rke2-server.service"}},
	}

	for _, cmd := range commands {
		_, err := runCommand(cmd.command, cmd.args...)
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to execute command '%s %v': %v", cmd.command, cmd.args, err))
			return fmt.Errorf("failed to execute command '%s %v': %w", cmd.command, cmd.args, err)
		}
		LogMessage(Info, fmt.Sprintf("Successfully executed command: %s %v", cmd.command, cmd.args))
	}

	if err := startServiceWithTimeout("rke2-server", 2*time.Minute); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to start rke2-server service: %v", err))
		return err
	}

	return nil
}

func startServiceWithTimeout(serviceName string, timeout time.Duration) error {
	_, err := runCommand("systemctl", "start", serviceName+".service")
	LogMessage(Info, fmt.Sprintf("Starting service %s", serviceName))
	if err != nil {
		return fmt.Errorf("failed to start service %s: %w", serviceName, err)
	}

	LogMessage(Info, fmt.Sprintf("Waiting for service %s to become active (timeout: %v)", serviceName, timeout))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// The exec.Command is fine here as it uses CombinedOutput
		isActiveCmd := exec.Command("systemctl", "is-active", serviceName+".service")
		output, err := isActiveCmd.CombinedOutput()
		status := string(output)
		if err == nil && status == "active\n" {
			LogMessage(Info, fmt.Sprintf("Service %s is now active", serviceName))
			return nil
		}
		LogMessage(Info, fmt.Sprintf("Service %s status: %s", serviceName, status))
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for service %s to become active", serviceName)
}

func SetupRKE2Additional() error {
	serverIP := viper.GetString("SERVER_IP")
	if serverIP == "" {
		return fmt.Errorf("SERVER_IP configuration item is not set")
	}
	joinToken := viper.GetString("JOIN_TOKEN")
	if joinToken == "" {
		return fmt.Errorf("JOIN_TOKEN configuration item is not set")
	}
	rke2ConfigPath := "/etc/rancher/rke2/config.yaml"

	configContent := fmt.Sprintf("\nserver: https://%s:9345\ntoken: %s\n", serverIP, joinToken)
	file, err := os.OpenFile(rke2ConfigPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to open %s for appending: %v", rke2ConfigPath, err))
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(configContent); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to append to %s: %v", rke2ConfigPath, err))
		return err
	}

	LogMessage(Info, fmt.Sprintf("Appended configuration to %s", rke2ConfigPath))
	commands := []struct {
		command string
		args    []string
	}{
		{"sh", []string{"-c", "curl -sfL " + viper.GetString("RKE2_INSTALLATION_URL") + " | INSTALL_RKE2_TYPE=agent sh -"}},
		{"systemctl", []string{"enable", "rke2-agent.service"}},
	}
	for _, cmd := range commands {
		_, err := runCommand(cmd.command, cmd.args...)
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to execute command '%s %v': %v", cmd.command, cmd.args, err))
			return fmt.Errorf("failed to execute command '%s %v': %w", cmd.command, cmd.args, err)
		}
		LogMessage(Info, fmt.Sprintf("Successfully executed command: %s %v", cmd.command, cmd.args))
	}

	if err := startServiceWithTimeout("rke2-agent", 2*time.Minute); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to start rke2-agent service: %v", err))
		return err
	}

	return nil
}

func SetupRKE2ControlPlane() error {
	serverIP := viper.GetString("SERVER_IP")
	if serverIP == "" {
		return fmt.Errorf("SERVER_IP configuration item is not set")
	}
	joinToken := viper.GetString("JOIN_TOKEN")
	if joinToken == "" {
		return fmt.Errorf("JOIN_TOKEN configuration item is not set")
	}
	rke2ConfigPath := "/etc/rancher/rke2/config.yaml"

	configContent := fmt.Sprintf("\nserver: https://%s:9345\ntoken: %s\n", serverIP, joinToken)
	file, err := os.OpenFile(rke2ConfigPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to open %s for appending: %v", rke2ConfigPath, err))
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(configContent); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to append to %s: %v", rke2ConfigPath, err))
		return err
	}

	LogMessage(Info, fmt.Sprintf("Appended configuration to %s", rke2ConfigPath))
	commands := []struct {
		command string
		args    []string
	}{
		{"sh", []string{"-c", "curl -sfL " + viper.GetString("RKE2_INSTALLATION_URL") + " | INSTALL_RKE2_TYPE=server sh -"}},
		{"systemctl", []string{"enable", "rke2-server.service"}},
	}
	for _, cmd := range commands {
		_, err := runCommand(cmd.command, cmd.args...)
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to execute command '%s %v': %v", cmd.command, cmd.args, err))
			return fmt.Errorf("failed to execute command '%s %v': %w", cmd.command, cmd.args, err)
		}
		LogMessage(Info, fmt.Sprintf("Successfully executed command: %s %v", cmd.command, cmd.args))
	}

	if err := startServiceWithTimeout("rke2-server", 2*time.Minute); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to start rke2-server service: %v", err))
		return err
	}

	return nil
}

func PreloadImages() error {

	imagesDir := "/var/lib/rancher/rke2/agent/images"

	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return fmt.Errorf("failed to create images directory %s: %v", imagesDir, err)
	}
	imageList := viper.GetStringSlice("IMAGE_LIST")

	preloadImagesFile := "/var/lib/rancher/rke2/agent/images/airm_images.txt"
	LogMessage(Info, fmt.Sprintf("Caching images to %s", preloadImagesFile))
	if err := os.WriteFile(preloadImagesFile, []byte(strings.Join(imageList, "\n")), 0644); err != nil {
		return fmt.Errorf("failed to write airgapped images file %s: %v", preloadImagesFile, err)
	}
	return nil
}
