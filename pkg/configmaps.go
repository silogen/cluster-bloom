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
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/viper"
)

func CreateConfigMap(version string) error {
	bloomConfig := make(map[string]string)

	configFile := viper.ConfigFileUsed()
	if configFile != "" {
		content, err := os.ReadFile(configFile)
		if err == nil {

			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])

					viperValue := viper.GetString(key)
					if viperValue != "" {
						bloomConfig[key] = viperValue
					}
				}
			}
		} else {
			LogMessage(Info, fmt.Sprintf("Could not read config file %s: %v", configFile, err))
			return fmt.Errorf("Could not read config file %s: %v", configFile, err)

		}
	} else {
		LogMessage(Info, "No bloom.yaml config file found, skipping ConfigMap creation")
		return fmt.Errorf("No bloom.yaml config file found, skipping ConfigMap creation")
	}

	if viper.GetString("TLS_CERT") != "" && viper.GetString("TLS_KEY") != "" {
		bloomConfig["tls_secret_name"] = "cluster-tls"
		bloomConfig["tls_secret_namespace"] = "kgateway-system"
	}

	configMapYAML := `apiVersion: v1
kind: ConfigMap
metadata:
  name: bloom
  namespace: default
data:
  BLOOM_VERSION: "` + version + `"
`
	// Add each configuration item
	for key, value := range bloomConfig {
		// Escape any special characters in the value
		escapedValue := strings.ReplaceAll(value, "\n", "\\n")
		escapedValue = strings.ReplaceAll(escapedValue, "\"", "\\\"")
		configMapYAML += fmt.Sprintf("  %s: \"%s\"\n", key, escapedValue)
	}
	// Write to temporary file
	tmpFile, err := os.CreateTemp("", "bloom-configmap-*.yaml")
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to create temporary file: %v", err))
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configMapYAML); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to write ConfigMap YAML: %v", err))
		return fmt.Errorf("failed to write ConfigMap YAML: %w", err)
	}
	tmpFile.Close()

	// Apply the ConfigMap using kubectl
	cmd := exec.Command("/var/lib/rancher/rke2/bin/kubectl", "--kubeconfig", "/etc/rancher/rke2/rke2.yaml", "apply", "-f", tmpFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to create ConfigMap: %v, output: %s", err, string(output)))
		return fmt.Errorf("failed to create ConfigMap: %w", err)
	}
	return nil
}

var podTemplate = `apiVersion: v1
kind: Pod
metadata:
  name: bloom-yaml
  namespace: default
  annotations:
    bloom.yaml: |
%s
spec:
  restartPolicy: OnFailure
  containers:
  - name: bloom-yaml-create
    image: alpine:latest
    resources:
      requests:
        memory: "16Mi"
        cpu: "10m"
      limits:
        memory: "64Mi"
    command:
      - /bin/sh
      - -c
      - |
        echo "Bloom YAML for node in annotation"
`

func CreateConfigMapPod() error {
	file, err := os.Open(viper.ConfigFileUsed())
	if err != nil {
		return fmt.Errorf("Error opening file: %v", err)
	}
	defer file.Close()
	bloomContent := ""

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "JOIN_TOKEN") {
			bloomContent += "        " + line + "\n"
		}
	}
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	podYaml := fmt.Sprintf(podTemplate, bloomContent)
	if err := os.WriteFile("/var/lib/rancher/rke2/agent/pod-manifests/bloom-yaml-creator.yaml", []byte(podYaml), 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", "/tmp/test", err)
	}
	return nil

}
