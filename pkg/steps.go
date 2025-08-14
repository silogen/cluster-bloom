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
	"embed"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

var rke2ManifestDirectory = "/var/lib/rancher/rke2/server/manifests"

//go:embed manifests/*/*.yaml
var manifestFiles embed.FS

//go:embed templates/*.yaml
var templateFiles embed.FS

var CheckUbuntuStep = Step{
	Id:          "CheckUbuntuStep",
	Name:        "Check Ubuntu Version",
	Description: "Verify running on supported Ubuntu version",
	Action: func() StepResult {
		if !IsRunningOnSupportedUbuntu() {
			return StepResult{
				Error: fmt.Errorf("this tool requires Ubuntu with one of these versions: %s",
					strings.Join(SupportedUbuntuVersions, ", ")),
			}
		}
		return StepResult{Error: nil}
	},
}

var InstallDependentPackagesStep = Step{
	Id:          "InstallDependentPackagesStep",
	Name:        "Install Dependent Packages",
	Description: "Ensure jq, nfs-common, and open-iscsi are installed",
	Action: func() StepResult {
		err := InstallDependentPackages()
		if err != nil {
			return StepResult{
				Error: fmt.Errorf("setup of packages failed: %s", err.Error()),
			}
		}
		return StepResult{Error: nil}
	},
}

var OpenPortsStep = Step{
	Id:          "OpenPortsStep",
	Name:        "Open Ports",
	Description: "Ensure needed ports are open in iptables",
	Action: func() StepResult {
		if !OpenPorts() {
			return StepResult{
				Error: fmt.Errorf("opening ports failed"),
			}
		}
		return StepResult{Error: nil}
	},
}

var CheckPortsBeforeOpeningStep = Step{
	Id:          "CheckPortsBeforeOpeningStep",
	Name:        "Checking Ports",
	Description: "Ensure needed ports are not in use",
	Action: func() StepResult {
		err := CheckPortsBeforeOpening()
		if err != nil {
			return StepResult{
				Error: fmt.Errorf("Checking ports failed: %s", err.Error()),
			}
		}
		return StepResult{Error: nil}
	},
}

var InstallK8SToolsStep = Step{
	Id:          "InstallK8SToolsStep",
	Name:        "Install Kubernetes tools",
	Description: "Install kubectl and k9s",
	Action: func() StepResult {
		err := installK8sTools()
		if err != nil {
			return StepResult{
				Error: fmt.Errorf("setup of tools failed: %s", err.Error()),
			}
		}
		return StepResult{Error: nil}
	},
}

var InotifyInstancesStep = Step{
	Id:          "InotifyInstancesStep",
	Name:        "Verify inotify instances",
	Description: "Verify, or update, inotify instances",
	Action: func() StepResult {
		if !VerifyInotifyInstances() {
			return StepResult{
				Error: fmt.Errorf("setup of inotify failed"),
			}
		}
		return StepResult{Error: nil}
	},
}

var SetupAndCheckRocmStep = Step{
	Id:          "SetupAndCheckRocmStep",
	Name:        "Setup and Check ROCm",
	Description: "Verify, setup, and check ROCm devices",
	Skip: func() bool {
		if !viper.GetBool("GPU_NODE") {
			LogMessage(Info, "Skipping ROCm setup for non-GPU node")
			return true
		}
		return false
	},
	Action: func() StepResult {
		if !CheckAndInstallROCM() {
			return StepResult{
				Error: fmt.Errorf("setup of ROCm failed"),
			}
		}
		cmd := exec.Command("sh", "-c", `rocm-smi -i --json | jq -r '.[] | .["Device Name"]' | sort | uniq -c`)
		output, err := cmd.CombinedOutput()
		if err != nil {
			LogMessage(Error, "Failed to execute rocm-smi: "+err.Error())
			return StepResult{
				Error: fmt.Errorf("failed to execute rocm-smi: %w", err),
			}
		}
		// Check if the first characters are an integer
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if len(line) > 0 {
				parts := strings.Fields(line)
				if len(parts) > 0 {
					if _, err := strconv.Atoi(parts[0]); err != nil {
						LogMessage(Error, "rocm-smi did not return any GPUs: "+string(output))
						return StepResult{
							Error: fmt.Errorf("rocm-smi did not return any GPUs: %s", string(output)),
						}
					}
				}
			}
		}
		// Log the output of rocm-smi
		LogMessage(Info, "ROCm Devices:\n"+string(output))
		return StepResult{Error: nil}
	},
}

var SetupRKE2Step = Step{
	Id:          "SetupRKE2Step",
	Name:        "Setup RKE2",
	Description: "Setup RKE2 server and configure necessary modules",
	Action: func() StepResult {
		var err error
		if viper.GetBool("FIRST_NODE") {
			err = SetupFirstRKE2()
		} else if viper.GetBool("CONTROL_PLANE") {
			err = SetupRKE2ControlPlane()
		} else {
			err = SetupRKE2Additional()
		}
		if err != nil {
			return StepResult{Error: err}
		}
		return StepResult{Error: nil}
	},
}

var CleanDisksStep = Step{
	Id:          "CleanDisksStep",
	Name:        "Clean disks",
	Description: "remove any previous longhorn temp drives",
	Action: func() StepResult {
		err := CleanDisks()
		if err != nil {
			return StepResult{Error: err}
		}
		return StepResult{Error: nil}
	},
}

var SetupMultipathStep = Step{
	Id:          "SetupMultipathStep",
	Name:        "Setup Multipath",
	Description: "Configure multipath to blacklist standard devices",
	Action: func() StepResult {
		err := setupMultipath()
		if err != nil {
			return StepResult{
				Error: fmt.Errorf("multipath setup failed: %w", err),
			}
		}
		return StepResult{Error: nil}
	},
}

var UpdateModprobeStep = Step{
	Id:          "UpdateModprobeStep",
	Name:        "Update Modprobe",
	Description: "Update Modprobe to unblacklist amdgpu",
	Action: func() StepResult {
		err := updateModprobe()
		if err != nil {
			return StepResult{
				Error: fmt.Errorf("update Modprobe failed: %w", err),
			}
		}
		return StepResult{Error: nil}
	},
}

var SelectDrivesStep = Step{
	Id:          "SelectDrivesStep",
	Name:        "Select Unmounted Disks",
	Description: "Identify and select unmounted physical disks",
	Action: func() StepResult {
		if viper.IsSet("SELECTED_DISKS") && viper.GetString("SELECTED_DISKS") != "" {
			disks := strings.Split(viper.GetString("SELECTED_DISKS"), ",")
			LogMessage(Info, fmt.Sprintf("Selected disks: %v", disks))
			for _, disk := range disks {
				cmd := exec.Command("umount", "-Av", disk)
				output, _ := cmd.CombinedOutput()
				LogMessage(Info, fmt.Sprintf("unmounted disk %s: %s", disk, string(output)))
			}
			viper.Set("selected_disks", disks)
			return StepResult{Error: nil}
		}
		disks, err := GetUnmountedPhysicalDisks()
		if err != nil {
			return StepResult{
				Error: fmt.Errorf("failed to get unmounted disks: %v", err),
			}
		}
		if len(disks) == 0 {
			LogMessage(Info, "No unmounted physical disks found")
			return StepResult{Error: nil}
		}
		cmd := exec.Command("sh", "-c", "lsblk |awk '/nvme/ {print $0}'")
		output, err := cmd.Output()
		if err != nil {
			return StepResult{
				Error: fmt.Errorf("failed to get disk info: %v", err),
			}
		}
		diskinfo := string(output)
		options := make([]string, len(disks))
		for i, disk := range disks {
			options[i] = disk
		}

		result, err := ShowOptionsScreen(
			"Unmounted Disks",
			"Select disks to format and mount\n\n"+diskinfo+"\n\nThe suggested drives are pre-selected, arrow keys to navigate, spacebar to select, enter to confirm\n\nd when done, q to quit",
			options,
			options,
		)
		if err != nil {
			return StepResult{
				Error: fmt.Errorf("error selecting disks: %v", err),
			}
		}

		if result.Canceled {
			return StepResult{
				Error: fmt.Errorf("disk selection canceled"),
			}
		}
		LogMessage(Info, fmt.Sprintf("Selected disks: %v", result.Selected))

		// Store the selected disks for the next step
		viper.Set("selected_disks", result.Selected)

		return StepResult{Message: fmt.Sprintf("Selected disks: %v", result.Selected)}
	},
}

var MountSelectedDrivesStep = Step{
	Id:          "MountSelectedDrivesStep",
	Name:        "Mount Selected Disks",
	Description: "Mount the selected physical disks",
	Skip: func() bool {
		if viper.IsSet("LONGHORN_DISKS") && viper.GetString("LONGHORN_DISKS") != "" {
			LogMessage(Info, "Skipping drive mounting as LONGHORN_DISKS is set.")
			return true
		}
		if viper.GetBool("SKIP_DISK_CHECK") {
			LogMessage(Info, "Skipping drive mounting as SKIP_DISK_CHECK is set.")
			return true
		}
		return false
	},
	Action: func() StepResult {
		selectedDisks := viper.GetStringSlice("selected_disks")
		if len(selectedDisks) == 0 {
			LogMessage(Info, "No disks selected for mounting")
			return StepResult{Error: nil}
		}

		mountError := MountDrives(selectedDisks)
		if mountError != nil {
			return StepResult{
				Error: fmt.Errorf("error mounting disks: %v", mountError),
			}
		}
		persistError := PersistMountedDisks()
		if persistError != nil {
			return StepResult{
				Error: fmt.Errorf("error persisting mounted disks: %v", persistError),
			}
		}
		LogMessage(Info, fmt.Sprintf("Mounted and persisted disks: %v", selectedDisks))
		return StepResult{Error: nil}
	},
}

var GenerateNodeLabelsStep = Step{
	Id:          "GenerateNodeLabelsStep",
	Name:        "Generate node Labels",
	Description: "Generate labels for the node based on its configuration",
	Action: func() StepResult {
		err := GenerateNodeLabels()
		if err != nil {
			return StepResult{Error: err}
		}
		return StepResult{Error: nil}
	},
}

var SetupMetallbStep = Step{
	Id:          "SetupMetallbStep",
	Name:        "Setup MetalLB manifests",
	Description: "Copy MetalLB YAML files to the RKE2 manifests directory",
	Skip: func() bool {
		if viper.GetBool("FIRST_NODE") == false {
			LogMessage(Info, "Skipping GenerateLonghornDiskString as SKIP_DISK_CHECK is set.")
			return true
		}
		return false
	},
	Action: func() StepResult {
		if viper.GetBool("FIRST_NODE") {
			err := setupManifests("metallb")
			if err != nil {
				return StepResult{Error: err}
			}
		} else {
			return StepResult{Error: nil}
		}
		return StepResult{Error: nil}
	},
}

var SetupLonghornStep = Step{
	Id:          "SetupLonghornStep",
	Name:        "Setup Longhorn manifests",
	Description: "Copy Longhorn YAML files to the RKE2 manifests directory",
	Skip: func() bool {
		if viper.GetBool("SKIP_DISK_CHECK") {
			LogMessage(Info, "Skipping GenerateLonghornDiskString as SKIP_DISK_CHECK is set.")
			return true
		}
		return false
	},
	Action: func() StepResult {
		if viper.GetBool("FIRST_NODE") {
			err := setupManifests("longhorn")
			if err != nil {
				return StepResult{Error: err}
			}
		} else {
			return StepResult{Error: nil}
		}
		return StepResult{Error: nil}
	},
}

var CreateMetalLBConfigStep = Step{
	Id:          "CreateMetalLBConfigStep",
	Name:        "Setup AddressPool for MetalLB",
	Description: "Create IPAddressPool and L2Advertisement resources for MetalLB",
	Skip: func() bool {
		if !viper.GetBool("FIRST_NODE") {
			LogMessage(Info, "Skipping for additional nodes.")
			return true
		}
		return false
	},
	Action: func() StepResult {
		err := CreateMetalLBConfig()
		if err != nil {
			return StepResult{Error: err}
		}
		return StepResult{Error: nil}
	},
}
var PrepareRKE2Step = Step{
	Id:          "PrepareRKE2Step",
	Name:        "Prepare for RKE2",
	Description: "RKE2 preparations",
	Action: func() StepResult {

		err := PrepareRKE2()
		if err != nil {
			return StepResult{Error: err}
		}
		return StepResult{Error: nil}
	},
}

var HasSufficientRancherPartitionStep = Step{
	Id:          "HasSufficientRancherPartitionStep",
	Name:        "Check /var/lib/rancher Partition Size",
	Description: "Check if the /var/lib/rancher partition size is sufficient",
	Skip: func() bool {
		if !viper.GetBool("GPU_NODE") {
			LogMessage(Info, "Skipping /var/lib/rancher partition check for CPU node.")
			return true
		}
		return false
	},
	Action: func() StepResult {

		if HasSufficientRancherPartition() {
			return StepResult{Error: nil}
		}
		return StepResult{Error: fmt.Errorf("/var/lib/rancher partition size is less than the recommended 500GB")}
	},
}

var NVMEDrivesAvailableStep = Step{
	Id:          "NVMEDrivesAvailableStep",
	Name:        "Check NVMe Drives",
	Description: "Check if NVMe drives are available",
	Skip: func() bool {
		if !viper.GetBool("GPU_NODE") {
			LogMessage(Info, "Skipped for non-GPU node")
			return true
		}
		if viper.GetBool("SKIP_DISK_CHECK") {
			LogMessage(Info, "Skipping NVME drive check as SKIP_DISK_CHECK is set.")
			return true
		}
		if viper.GetString("SELECTED_DISKS") != "" {
			LogMessage(Info, "Skipping NVME drive check as SELECTED_DISKS is set.")
			return true
		}

		return false
	},
	Action: func() StepResult {
		if NVMEDrivesAvailable() {
			return StepResult{Error: nil}
		}
		return StepResult{Error: fmt.Errorf("no NVMe drives available (either unmounted or mounted at /mnt/disk*)")}
	},
}

var SetupKubeConfig = Step{
	Id:          "SetupKubeConfig",
	Name:        "Setup KubeConfig",
	Description: "Setup and configure KubeConfig, and additional cluster setup command",
	Skip: func() bool {
		if !viper.GetBool("FIRST_NODE") {
			LogMessage(Info, "Skipping SetupKubeConfig for additional nodes.")
			return true
		}
		return false
	},
	Action: func() StepResult {
		cmd := exec.Command("sh", "-c", "ip route get 1.1.1.1 | awk '{print $7; exit}'")
		output, err := cmd.Output()
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to get main IP: %v", err))
			return StepResult{Error: fmt.Errorf("failed to get main IP: %w", err)}
		}
		mainIP := strings.TrimSpace(string(output))

		sedCmd := fmt.Sprintf("sudo sed -i 's/127\\.0\\.0\\.1/%s/g' /etc/rancher/rke2/rke2.yaml", mainIP)
		if err := exec.Command("sh", "-c", sedCmd).Run(); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to update RKE2 config file: %v", err))
			return StepResult{Error: fmt.Errorf("failed to update RKE2 config file: %w", err)}
		}

		// Get the actual user's home directory, not root's
		userHome := os.Getenv("HOME")
		if userHome == "" {
			if user := os.Getenv("SUDO_USER"); user != "" {
				userHome = "/home/" + user
			} else {
				userHome = os.ExpandEnv("$HOME")
			}
		}

		kubeDir := filepath.Join(userHome, ".kube")
		if err := os.MkdirAll(kubeDir, 0755); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to create .kube directory: %v", err))
			return StepResult{Error: fmt.Errorf("failed to create .kube directory: %w", err)}
		}

		// Change ownership of .kube directory to the actual user if running with sudo
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			chownCmd := fmt.Sprintf("sudo chown %s:%s %s", sudoUser, sudoUser, kubeDir)
			if err := exec.Command("sh", "-c", chownCmd).Run(); err != nil {
				LogMessage(Error, fmt.Sprintf("Failed to change ownership of .kube directory: %v", err))
				return StepResult{Error: fmt.Errorf("failed to change ownership of .kube directory: %w", err)}
			}
		}

		kubeconfigPath := filepath.Join(userHome, ".kube", "config")
		sedCmd = fmt.Sprintf("sudo sed 's/127\\.0\\.0\\.1/%s/g' /etc/rancher/rke2/rke2.yaml > %s", mainIP, kubeconfigPath)
		if err := exec.Command("sh", "-c", sedCmd).Run(); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to update KUBECONFIG file: %v", err))
			return StepResult{Error: fmt.Errorf("failed to update KUBECONFIG file: %w", err)}
		}

		// Change ownership to the actual user if running with sudo
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
			chownCmd := fmt.Sprintf("sudo chown %s:%s %s", sudoUser, sudoUser, kubeconfigPath)
			if err := exec.Command("sh", "-c", chownCmd).Run(); err != nil {
				LogMessage(Error, fmt.Sprintf("Failed to change ownership of KUBECONFIG file: %v", err))
				return StepResult{Error: fmt.Errorf("failed to change ownership of KUBECONFIG file: %w", err)}
			}
		}

		if err := exec.Command("chmod", "600", kubeconfigPath).Run(); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to set permissions on KUBECONFIG file: %v", err))
			return StepResult{Error: fmt.Errorf("failed to set permissions on KUBECONFIG file: %w", err)}
		}
		currentUser, err := user.Current()
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to get current user: %v", err))
			return StepResult{Error: fmt.Errorf("failed to get current user: %w", err)}
		}
		userHomeDir := currentUser.HomeDir
		if os.Getenv("SUDO_USER") != "" {
			sudoUserName := os.Getenv("SUDO_USER")
			LogMessage(Debug, fmt.Sprintf("Attempting to get sudo user home directory: %s", sudoUserName))
			homedir, err := GetUserHomeDirViaShell(sudoUserName)
			if err != nil {
				LogMessage(Error, fmt.Sprintf("Failed to get home directory for sudo user '%s': %v. Using current user's home directory instead.", sudoUserName, err))
				// Continue with currentUser.HomeDir as fallback
			} else {
				userHomeDir = homedir
			}
		}
		if err := os.MkdirAll(fmt.Sprintf("%s/.kube", userHomeDir), 0755); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to create .kube directory for non-sudo user: %v", err))
		}
		cpCmd := fmt.Sprintf("sudo cp $HOME/.kube/config %s/.kube/config", userHomeDir)
		if err := exec.Command("sh", "-c", cpCmd).Run(); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to copy KUBECONFIG file to non-sudo user's home directory: %v", err))
		}

		if err := exec.Command("sudo", "chown", fmt.Sprintf("%s:%s", os.Getenv("SUDO_USER"), os.Getenv("SUDO_USER")), fmt.Sprintf("%s/.kube/config", userHomeDir)).Run(); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to change ownership of KUBECONFIG file: %v", err))
		}

		// Store the path to k9s in a variable
		k9sPath := "/snap/k9s/current/bin"

		// Check if k9s path is already in bashrc
		bashrcPath := fmt.Sprintf("%s/.bashrc", userHomeDir)
		bashrcContent, err := os.ReadFile(bashrcPath)
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to read .bashrc: %v", err))
		} else {
			// Check if k9s path already exists in .bashrc
			if !strings.Contains(string(bashrcContent), k9sPath) {
				// Add k9s path to .bashrc if not found
				pathCmd := fmt.Sprintf("echo 'export PATH=$PATH:%s' >> %s", k9sPath, bashrcPath)
				if err = exec.Command("sh", "-c", pathCmd).Run(); err != nil {
					LogMessage(Error, fmt.Sprintf("Failed to update PATH: %v", err))
				} else {
					LogMessage(Info, fmt.Sprintf("Path in .bashrc updated to contain %s", string(k9sPath)))
				}
			}
		}

		return StepResult{Error: nil}
	},
}

var CreateBloomConfigMapStep = Step{
	Id:          "CreateBloomConfigMapStep",
	Name:        "Create Bloom ConfigMap",
	Description: "Create a ConfigMap with bloom configuration in the default namespace",
	Skip: func() bool {
		if !viper.GetBool("FIRST_NODE") {
			LogMessage(Info, "Skipped for additional node")
			return true
		}
		return false
	},
	Action: func() StepResult {
		// Wait for the cluster to be ready
		LogMessage(Info, "Waiting for cluster to be ready...")
		time.Sleep(10 * time.Second)

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
			}
		} else {
			LogMessage(Info, "No bloom.yaml config file found, skipping ConfigMap creation")
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
			return StepResult{Error: fmt.Errorf("failed to create temporary file: %w", err)}
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(configMapYAML); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to write ConfigMap YAML: %v", err))
			return StepResult{Error: fmt.Errorf("failed to write ConfigMap YAML: %w", err)}
		}
		tmpFile.Close()

		// Apply the ConfigMap using kubectl
		cmd := exec.Command("/var/lib/rancher/rke2/bin/kubectl", "--kubeconfig", "/etc/rancher/rke2/rke2.yaml", "apply", "-f", tmpFile.Name())
		output, err := cmd.CombinedOutput()
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to create ConfigMap: %v, output: %s", err, string(output)))
			return StepResult{Error: fmt.Errorf("failed to create ConfigMap: %w", err)}
		}

		LogMessage(Info, "Successfully created bloom ConfigMap in default namespace")
		return StepResult{Message: "Bloom ConfigMap created successfully"}
	},
}

var CreateDomainConfigStep = Step{
	Id:          "CreateDomainConfigStep",
	Name:        "Create Domain Configuration",
	Description: "Create domain ConfigMap and TLS secret for ingress configuration",
	Skip: func() bool {
		if !viper.GetBool("FIRST_NODE") {
			LogMessage(Info, "Skipped for additional node")
			return true
		}
		return false
	},
	Action: func() StepResult {
		domain := viper.GetString("DOMAIN")

		// Wait for the cluster to be ready
		LogMessage(Info, "Waiting for cluster to be ready...")
		time.Sleep(5 * time.Second)

		// Create domain ConfigMap
		useCertManager := viper.GetBool("USE_CERT_MANAGER")

		configMapYAML := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-domain
  namespace: default
data:
  domain: "%s"
  use-cert-manager: "%t"
`, domain, useCertManager)

		// Write ConfigMap to temporary file and apply
		tmpFile, err := os.CreateTemp("", "domain-configmap-*.yaml")
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to create temporary file: %v", err))
			return StepResult{Error: fmt.Errorf("failed to create temporary file: %w", err)}
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(configMapYAML); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to write domain ConfigMap: %v", err))
			return StepResult{Error: fmt.Errorf("failed to write domain ConfigMap: %w", err)}
		}
		tmpFile.Close()

		// Apply the ConfigMap
		cmd := exec.Command("/var/lib/rancher/rke2/bin/kubectl", "--kubeconfig", "/etc/rancher/rke2/rke2.yaml", "apply", "-f", tmpFile.Name())
		output, err := cmd.CombinedOutput()
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to create domain ConfigMap: %v, output: %s", err, string(output)))
			return StepResult{Error: fmt.Errorf("failed to create domain ConfigMap: %w", err)}
		}

		LogMessage(Info, "Successfully created domain ConfigMap")

		// Handle TLS certificates
		if !useCertManager {
			certOption := viper.GetString("CERT_OPTION")
			tlsCertPath := viper.GetString("TLS_CERT")
			tlsKeyPath := viper.GetString("TLS_KEY")

			// Handle certificate generation or use existing
			if certOption == "generate" {
				LogMessage(Info, "Generating self-signed certificate for domain: "+domain)

				// Create temporary directory for certificate files
				tempDir, err := os.MkdirTemp("", "bloom-tls-*")
				if err != nil {
					LogMessage(Error, fmt.Sprintf("Failed to create temp directory: %v", err))
					return StepResult{Error: fmt.Errorf("failed to create temp directory: %w", err)}
				}
				defer os.RemoveAll(tempDir)

				tlsCertPath = filepath.Join(tempDir, "tls.crt")
				tlsKeyPath = filepath.Join(tempDir, "tls.key")

				// Generate self-signed certificate using openssl
				cmd := exec.Command("openssl", "req", "-x509", "-nodes", "-days", "365", "-newkey", "rsa:2048",
					"-keyout", tlsKeyPath,
					"-out", tlsCertPath,
					"-subj", fmt.Sprintf("/CN=%s", domain),
					"-addext", fmt.Sprintf("subjectAltName=DNS:%s,DNS:*.%s", domain, domain))

				output, err := cmd.CombinedOutput()
				if err != nil {
					LogMessage(Error, fmt.Sprintf("Failed to generate self-signed certificate: %v, output: %s", err, string(output)))
					return StepResult{Error: fmt.Errorf("failed to generate self-signed certificate: %w", err)}
				}
				LogMessage(Info, "Successfully generated self-signed certificate")
			} else if certOption == "existing" {
				// Verify certificate and key files exist
				if tlsCertPath == "" || tlsKeyPath == "" {
					LogMessage(Error, "CERT_OPTION is 'existing' but TLS_CERT or TLS_KEY not provided")
					return StepResult{Error: fmt.Errorf("TLS certificate files not provided")}
				}
				if _, err := os.Stat(tlsCertPath); os.IsNotExist(err) {
					LogMessage(Error, fmt.Sprintf("TLS certificate file not found: %s", tlsCertPath))
					return StepResult{Error: fmt.Errorf("TLS certificate file not found: %s", tlsCertPath)}
				}
				if _, err := os.Stat(tlsKeyPath); os.IsNotExist(err) {
					LogMessage(Error, fmt.Sprintf("TLS key file not found: %s", tlsKeyPath))
					return StepResult{Error: fmt.Errorf("TLS key file not found: %s", tlsKeyPath)}
				}
			} else {
				LogMessage(Info, "Domain configured but no certificate option specified")
				return StepResult{Message: "Domain ConfigMap created but no TLS configuration applied"}
			}

			// Create kgateway-system namespace
			namespaceYAML := `apiVersion: v1
kind: Namespace
metadata:
  name: kgateway-system
`
			tmpNsFile, err := os.CreateTemp("", "kgateway-namespace-*.yaml")
			if err != nil {
				LogMessage(Error, fmt.Sprintf("Failed to create temporary namespace file: %v", err))
				return StepResult{Error: fmt.Errorf("failed to create temporary namespace file: %w", err)}
			}
			defer os.Remove(tmpNsFile.Name())

			if _, err := tmpNsFile.WriteString(namespaceYAML); err != nil {
				LogMessage(Error, fmt.Sprintf("Failed to write namespace YAML: %v", err))
				return StepResult{Error: fmt.Errorf("failed to write namespace YAML: %w", err)}
			}
			tmpNsFile.Close()

			// Apply the namespace
			cmd := exec.Command("/var/lib/rancher/rke2/bin/kubectl", "--kubeconfig", "/etc/rancher/rke2/rke2.yaml", "apply", "-f", tmpNsFile.Name())
			output, err := cmd.CombinedOutput()
			if err != nil {
				LogMessage(Error, fmt.Sprintf("Failed to create kgateway-system namespace: %v, output: %s", err, string(output)))
				return StepResult{Error: fmt.Errorf("failed to create kgateway-system namespace: %w", err)}
			}
			LogMessage(Info, "Successfully created kgateway-system namespace")

			// Create TLS secret using kubectl
			cmd = exec.Command("/var/lib/rancher/rke2/bin/kubectl",
				"--kubeconfig", "/etc/rancher/rke2/rke2.yaml",
				"create", "secret", "tls", "cluster-tls",
				"--cert", tlsCertPath,
				"--key", tlsKeyPath,
				"-n", "kgateway-system",
				"--dry-run=client", "-o", "yaml")

			secretYAML, err := cmd.Output()
			if err != nil {
				LogMessage(Error, fmt.Sprintf("Failed to generate TLS secret: %v", err))
				return StepResult{Error: fmt.Errorf("failed to generate TLS secret: %w", err)}
			}

			// Apply the secret
			tmpSecretFile, err := os.CreateTemp("", "tls-secret-*.yaml")
			if err != nil {
				LogMessage(Error, fmt.Sprintf("Failed to create temporary secret file: %v", err))
				return StepResult{Error: fmt.Errorf("failed to create temporary secret file: %w", err)}
			}
			defer os.Remove(tmpSecretFile.Name())

			if _, err := tmpSecretFile.Write(secretYAML); err != nil {
				LogMessage(Error, fmt.Sprintf("Failed to write TLS secret: %v", err))
				return StepResult{Error: fmt.Errorf("failed to write TLS secret: %w", err)}
			}
			tmpSecretFile.Close()

			cmd = exec.Command("/var/lib/rancher/rke2/bin/kubectl", "--kubeconfig", "/etc/rancher/rke2/rke2.yaml", "apply", "-f", tmpSecretFile.Name())
			output, err = cmd.CombinedOutput()
			if err != nil {
				LogMessage(Error, fmt.Sprintf("Failed to create TLS secret: %v, output: %s", err, string(output)))
				return StepResult{Error: fmt.Errorf("failed to create TLS secret: %w", err)}
			}

			LogMessage(Info, "Successfully created TLS secret")
			return StepResult{Message: "Domain ConfigMap and TLS secret created successfully"}
		} else {
			LogMessage(Info, "Cert-manager will be used for TLS certificates")
			return StepResult{Message: "Domain ConfigMap created, cert-manager will handle TLS"}
		}
	},
}

var SetupClusterForgeStep = Step{
	Id:          "SetupClusterForgeStep",
	Name:        "Setup Cluster Forge",
	Description: "Setup and configure Cluster Forge",
	Skip: func() bool {
		if !viper.GetBool("FIRST_NODE") {
			LogMessage(Info, "Skipping for additional nodes.")
			return true
		}
		return false
	},
	Action: func() StepResult {
		err := SetupClusterForge()
		if err != nil {
			return StepResult{Error: fmt.Errorf("failed to setup Cluster Forge: %v", err)}
		}
		return StepResult{Error: nil}
	},
}
var FinalOutput = Step{
	Id:          "FinalOutput",
	Name:        "Output",
	Description: "Generate output after installation",
	Action: func() StepResult {
		if viper.GetBool("FIRST_NODE") {
			tokenCmd := exec.Command("sh", "-c", "cat /var/lib/rancher/rke2/server/node-token")
			tokenOutput, err := tokenCmd.Output()
			if err != nil {
				LogMessage(Error, fmt.Sprintf("Failed to get join token: %v", err))
				return StepResult{Error: fmt.Errorf("failed to get join token: %w", err)}
			}
			joinToken := strings.TrimSpace(string(tokenOutput))

			mainIPCmd := exec.Command("sh", "-c", "ip route get 1.1.1.1 | awk '{print $7; exit}'")
			mainIPOutput, err := mainIPCmd.Output()
			if err != nil {
				LogMessage(Error, fmt.Sprintf("Failed to get main IP: %v", err))
				return StepResult{Error: fmt.Errorf("failed to get main IP: %w", err)}
			}
			mainIP := strings.TrimSpace(string(mainIPOutput))
			oneLineScript := fmt.Sprintf("echo -e 'FIRST_NODE: false\\nJOIN_TOKEN: %s\\nSERVER_IP: %s' > bloom.yaml && sudo ./bloom --config bloom.yaml", joinToken, mainIP)
			file, err := os.Create("additional_node_command.txt")
			if err != nil {
				LogMessage(Error, fmt.Sprintf("Failed to create additional_node_command.txt: %v", err))
				return StepResult{Error: fmt.Errorf("failed to create additional_node_command.txt: %w", err)}
			}
			defer file.Close()

			_, err = file.WriteString(oneLineScript)
			if err != nil {
				LogMessage(Error, fmt.Sprintf("Failed to write to additional_node_command.txt: %v", err))
				return StepResult{Error: fmt.Errorf("failed to write to additional_node_command.txt: %w", err)}
			}

			LogMessage(Info, "To setup additional nodes to join the cluster, copy and run the command from additional_node_command.txt")
			return StepResult{Message: "To setup additional nodes to join the cluster, copy and run the command from additional_node_command.txt"}
		} else {
			message := "The content of longhorn_drive_setup.txt must be run in order to mount drives properly. " +
				"This can be done in the control node, which was installed first, or with a valid kubeconfig for the cluster."
			LogMessage(Info, message)
			return StepResult{Message: message}
		}
	},
}

var UpdateUdevRulesStep = Step{
	Id:          "UpdateUdevRulesStep",
	Name:        "Update Udev Rules",
	Description: "Update AMD device-specific udev rules",
	Skip: func() bool {
		if !viper.GetBool("GPU_NODE") {
			LogMessage(Info, "Skipped for non-GPU node")
			return true
		}
		return false
	},
	Action: func() StepResult {
		var fileName = "/etc/udev/rules.d/70-amdgpu.rules"

		var fileContent = strings.Join([]string{
			"KERNEL==\"kfd\", MODE=\"0666\"",
			"SUBSYSTEM==\"drm\", KERNEL==\"renderD*\", MODE=\"0666\"",
		}, "\n")

		cmd := exec.Command("sudo", "tee", fileName)
		cmd.Stdin = strings.NewReader(fileContent)
		err := cmd.Run()
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to write to file: %v", err))
			return StepResult{Error: fmt.Errorf("Failed to write to file: %v", err)}
		}

		err = exec.Command("sudo", "udevadm", "control", "--reload-rules").Run()
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to reload udev rules: %v", err))
			return StepResult{Error: fmt.Errorf("Failed to reload udev rules: %v", err)}
		}

		err = exec.Command("sudo", "udevadm", "trigger").Run()
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to trigger udev: %v", err))
			return StepResult{Error: fmt.Errorf("Failed to trigger udev: %v", err)}
		}

		return StepResult{Error: nil}
	},
}

var CleanLonghornMountsStep = Step{
	Id:          "CleanLonghornMountsStep",
	Name:        "Clean Longhorn Mounts",
	Description: "Clean up Longhorn PVCs and mounts before RKE2 uninstall",
	Action: func() StepResult {
		LogMessage(Info, "Cleaning Longhorn mounts and PVCs")

		// Stop Longhorn services first if they exist
		cmd := exec.Command("sh", "-c", "sudo systemctl stop longhorn-* 2>/dev/null || true")
		cmd.CombinedOutput()

		// Find and unmount all Longhorn-related mounts
		for i := 0; i < 3; i++ {
			// Unmount Longhorn device files
			cmd = exec.Command("sh", "-c", "sudo umount -lf /dev/longhorn/pvc* 2>/dev/null || true")
			cmd.CombinedOutput()

			// Find /mnt/disk* mount points that contain longhorn-disk.cfg and unmount them
			cmd = exec.Command("sh", "-c", `
				for mount_point in /mnt/disk*; do
					if [ -d "$mount_point" ] && find "$mount_point" -name "longhorn-disk.cfg" 2>/dev/null | grep -q .; then
						echo "Found longhorn-disk.cfg in $mount_point, unmounting..."
						sudo umount -lf "$mount_point" 2>/dev/null || true
					fi
				done
			`)
			cmd.CombinedOutput()

			// Find and unmount CSI volume mounts
			cmd = exec.Command("sh", "-c", "sudo umount -Af /var/lib/kubelet/pods/*/volumes/kubernetes.io~csi/pvc-* 2>/dev/null || true")
			cmd.CombinedOutput()
			cmd = exec.Command("sh", "-c", "sudo umount -Af /var/lib/kubelet/pods/*/volumes/kubernetes.io~csi/*/mount 2>/dev/null || true")
			cmd.CombinedOutput()

			// Find and unmount CSI plugin mounts
			cmd = exec.Command("sh", "-c", "mount | grep 'driver.longhorn.io' | awk '{print $3}' | xargs -r sudo umount -lf 2>/dev/null || true")
			cmd.CombinedOutput()

			// Find and unmount any remaining kubelet plugin mounts
			cmd = exec.Command("sh", "-c", "sudo umount -Af /var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/* 2>/dev/null || true")
			cmd.CombinedOutput()
		}

		// Force kill any processes using Longhorn mounts
		cmd = exec.Command("sh", "-c", "sudo fuser -km /dev/longhorn/ 2>/dev/null || true")
		cmd.CombinedOutput()

		// Clean up device files
		cmd = exec.Command("sh", "-c", "sudo rm -rf /dev/longhorn/pvc-* 2>/dev/null || true")
		cmd.CombinedOutput()

		// Clean up kubelet CSI mounts
		cmd = exec.Command("sh", "-c", "sudo rm -rf /var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/* 2>/dev/null || true")
		cmd.CombinedOutput()

		LogMessage(Info, "Longhorn cleanup completed")
		return StepResult{Error: nil}
	},
}

var UninstallRKE2Step = Step{
	Id:          "UninstallRKE2Step",
	Name:        "Uninstall RKE2",
	Description: "Execute the RKE2 uninstall script if it exists",
	Action: func() StepResult {
		LogMessage(Info, "Uninstalling RKE2, which takes a couple minutes.")
		cmd := exec.Command("/usr/local/bin/rke2-uninstall.sh")
		output, err := cmd.CombinedOutput()
		if err != nil {
			LogMessage(Info, fmt.Sprintf("RKE2 uninstall script output: %s", string(output)))
			LogMessage(Info, fmt.Sprintf("RKE2 uninstall script encountered and ignored an error: %v", err))
			return StepResult{Error: nil}
		}
		LogMessage(Info, fmt.Sprintf("RKE2 uninstall script output: %s", string(output)))
		LogMessage(Info, "RKE2 uninstall script executed successfully.")
		return StepResult{Error: nil}
	},
}
