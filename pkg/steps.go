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
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

var rke2ManifestDirectory = "/var/lib/rancher/rke2/server/manifests"

//go:embed manifests/*/*.yaml
var manifestFiles embed.FS

//go:embed templates/*.yaml
var templateFiles embed.FS

var CheckUbuntuStep = Step{
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
	Name:        "Setup and Check ROCm",
	Description: "Verify, setup, and check ROCm devices",
	Action: func() StepResult {
		if viper.GetBool("GPU_NODE") {
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
		}
		return StepResult{Error: nil}
	},
}

var SetupRKE2Step = Step{
	Name:        "Setup RKE2",
	Description: "Setup RKE2 server and configure necessary modules",
	Action: func() StepResult {
		var err error
		if viper.GetBool("FIRST_NODE") {
			err = SetupFirstRKE2()
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
	Name:        "Select Unmounted Disks",
	Description: "Identify and select unmounted physical disks",
	Action: func() StepResult {
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
		cmd := exec.Command("sh", "-c", "lsblk |grep nvme")
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
	Name:        "Mount Selected Disks",
	Description: "Mount the selected physical disks",
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

var UninstallRKE2Step = Step{
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
var GenerateLonghornDiskStringStep = Step{
	Name:        "Generate Longhorn Disk String",
	Description: "Generate Longhorn disk configuration string for NVMe drives",
	Action: func() StepResult {
		err := GenerateLonghornDiskString()
		if err != nil {
			return StepResult{Error: err}
		}
		return StepResult{Error: nil}
	},
}

var SetupManifestsStep = Step{
	Name:        "Setup Longhorn And MetalLB manifests",
	Description: "Copy Longhorn and MetalLB YAML files to the RKE2 manifests directory",
	Action: func() StepResult {
		if viper.GetBool("FIRST_NODE") {
			err := setupManifests()
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
	Name:        "Setup AddressPool for MetalLB",
	Description: "Create IPAddressPool and L2Advertisement resources for MetalLB",
	Action: func() StepResult {
		if viper.GetBool("FIRST_NODE") {
			err := CreateMetalLBConfig()
			if err != nil {
				return StepResult{Error: err}
			}
		} else {
			return StepResult{Error: nil}
		}
		return StepResult{Error: nil}
	},
}
var PrepareRKE2Step = Step{
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

var HasSufficientRootPartitionStep = Step{
	Name:        "Check Root Partition Size",
	Description: "Check if the root partition size is sufficient",
	Action: func() StepResult {

		if HasSufficientRootPartition() {
			return StepResult{Error: nil}
		}
		return StepResult{Error: fmt.Errorf("root partition size is less than the recommended 500GB")}
	},
}

var NVMEDrivesAvailableStep = Step{
	Name:        "Check NVMe Drives",
	Description: "Check if NVMe drives are available",
	Action: func() StepResult {

		if NVMEDrivesAvailable() {
			return StepResult{Error: nil}
		}
		return StepResult{Error: fmt.Errorf("no NVMe drives available (either unmounted or mounted at /mnt/disk*)")}
	},
}

var SetupKubeConfig = Step{
	Name:        "Setup KubeConfig",
	Description: "Setup and configure KubeConfig, and additional cluster setup command",
	Action: func() StepResult {
		if !viper.GetBool("FIRST_NODE") {
			return StepResult{Error: nil}
		}
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

		if err := os.MkdirAll(os.ExpandEnv("$HOME/.kube"), 0755); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to create .kube directory: %v", err))
			return StepResult{Error: fmt.Errorf("failed to create .kube directory: %w", err)}
		}

		sedCmd = fmt.Sprintf("sudo sed 's/127\\.0\\.0\\.1/%s/g' /etc/rancher/rke2/rke2.yaml | tee $HOME/.kube/config", mainIP)
		if err := exec.Command("sh", "-c", sedCmd).Run(); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to update KUBECONFIG file: %v", err))
			return StepResult{Error: fmt.Errorf("failed to update KUBECONFIG file: %w", err)}
		}

		if err := exec.Command("chmod", "600", os.ExpandEnv("$HOME/.kube/config")).Run(); err != nil {
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

var SetupOnePasswordSecretStep = Step{
	Name:        "Setup 1Password Secret",
	Description: "Setup and configure connect server secrets for 1Password",
	Action: func() StepResult {
		err := SetupOnePasswordSecret()
		if err != nil {
			return StepResult{Error: fmt.Errorf("failed to setup 1Password secret: %v", err)}
		}
		return StepResult{Error: nil}
	},
}

var SetupClusterForgeStep = Step{
	Name:        "Setup Cluster Forge",
	Description: "Setup and configure Cluster Forge",
	Action: func() StepResult {
		err := SetupClusterForge()
		if err != nil {
			return StepResult{Error: fmt.Errorf("failed to setup Cluster Forge: %v", err)}
		}
		return StepResult{Error: nil}
	},
}
var FinalOutput = Step{
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

var SetRenderGroupStep = Step{
	Name:        "Set Render Group",
	Description: "Make video the group of /dev/dri/renderD*",
	Action: func() StepResult {
		if !viper.GetBool("GPU_NODE") {
			return StepResult{Message: "Skip Set Render Group for non-GPU node"}
		}
		return StepResult{Error: exec.Command("/bin/sh", "-c", "sudo chgrp video /dev/dri/renderD*").Run()}
	},
}
