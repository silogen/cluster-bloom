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

	"github.com/silogen/cluster-bloom/pkg/fsops"
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
	tmpFile, err := fsops.CreateTemp("", "bloom-configmap-*.yaml")
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to create temporary file: %v", err))
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer fsops.Remove(tmpFile.Name())

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
