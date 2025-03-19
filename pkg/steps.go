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
	"strings"

	"github.com/rivo/tview"
	"github.com/spf13/viper"
)

//go:embed templates/*.yaml
var templateFiles embed.FS

var CheckUbuntuStep = Step{
	Name:        "Check Ubuntu Version",
	Description: "Verify running on supported Ubuntu version",
	Action: func() error {
		if !IsRunningOnSupportedUbuntu() {
			return fmt.Errorf("this tool requires Ubuntu with one of these versions: %s",
				strings.Join(SupportedUbuntuVersions, ", "))
		}
		return nil
	},
}
var InstallDependentPackagesStep = Step{
	Name:        "Install Dependent Packages",
	Description: "Ensure jq, nfs-common, and open-iscsi are installed",
	Action: func() error {
		err := InstallDependentPackages()
		if err != nil {
			return fmt.Errorf("setup of packages failed : %s",
				err.Error())
		}
		return nil
	},
}
var OpenPortsStep = Step{
	Name:        "Open Ports",
	Description: "Ensure needed ports are open in iptables",
	Action: func() error {
		if !OpenPorts() {
			return fmt.Errorf("opening ports failed")
		}
		return nil
	},
}
var InstallK8SToolsStep = Step{
	Name:        "Install Kubernetes tools",
	Description: "Install kubectl and k9s",
	Action: func() error {
		err := installK8sTools()
		if err != nil {
			return fmt.Errorf("setup of tools failed : %s",
				err.Error())
		}
		return nil
	},
}
var InotifyInstancesStep = Step{
	Name:        "Verify inotify instances",
	Description: "Verify, or update, inotify instances",
	Action: func() error {
		if !VerifyInotifyInstances() {
			return fmt.Errorf("setup of inotify failed")
		}
		return nil
	},
}
var SetupAndCheckRocmStep = Step{
	Name:        "Setup and Check ROCm",
	Description: "Verify, setup, and check ROCm devices",
	Action: func() error {
		if viper.GetBool("GPU_NODE") {
			if !CheckAndInstallROCM() {
				return fmt.Errorf("setup of ROCm failed")
			}
			cmd := exec.Command("sh", "-c", `rocm-smi -i --json | jq -r '.[] | .["Device Name"]' | sort | uniq -c`)
			output, err := cmd.CombinedOutput()
			if err != nil {
				LogMessage(Error, "Failed to execute rocm-smi: "+err.Error())
				return fmt.Errorf("failed to execute rocm-smi: %w", err)
			}
			LogMessage(Info, "ROCm Devices:\n"+string(output))

			return nil
		} else {
			return nil
		}
	},
}
var SetupRKE2Step = Step{
	Name:        "Setup RKE2",
	Description: "Setup RKE2 server and configure necessary modules",
	Action: func() error {
		if viper.GetBool("FIRST_NODE") {
			return SetupFirstRKE2()
		} else {
			return SetupRKE2Additional()
		}
	},
}
var CleanDisksStep = Step{
	Name:        "Clean disks",
	Description: "remove any previous longhorn temp drives",
	Action: func() error {
		return CleanDisks()
	},
}
var MountDrivesStep = Step{
	Name:        "Mount drives",
	Description: "mount any empty nvme drives",
	Action: func() error {
		return MountAndPersistNVMeDrives()
	},
}
var GenerateLonghornDiskStringStep = Step{
	Name:        "Generate Longhorn Disk String",
	Description: "Generate Longhorn disk configuration string for NVMe drives",
	Action: func() error {
		jsonString, err := GenerateLonghornDiskString()
		if err != nil {
			return err
		}
		if jsonString != "" {
			LogMessage(Info, fmt.Sprintf("Longhorn disk configuration string: %s", jsonString))
		} else {
			LogMessage(Info, "No Longhorn disk configuration string generated.")
		}
		return nil
		// TODO apply the disk string!
	},
}

var SetupLonghornStep = Step{
	Name:        "Setup Longhorn",
	Description: "Copy Longhorn YAML files to the RKE2 manifests directory",
	Action: func() error {
		return setupLonghorn()
	},
}

var SetupKubeConfig = Step{
	Name:        "Setup KubeConfit",
	Description: "Setup and configure KubeConfig, and additional cluster setup command",
	Action: func() error {
		cmd := exec.Command("sh", "-c", "ip route get 1.1.1.1 | awk '{print $7; exit}'")
		output, err := cmd.Output()
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to get main IP: %v", err))
			return fmt.Errorf("failed to get main IP: %w", err)
		}
		mainIP := strings.TrimSpace(string(output))

		sedCmd := fmt.Sprintf("sudo sed -i 's/127\\.0\\.0\\.1/%s/g' /etc/rancher/rke2/rke2.yaml", mainIP)
		if err := exec.Command("sh", "-c", sedCmd).Run(); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to update RKE2 config file: %v", err))
			return fmt.Errorf("failed to update RKE2 config file: %w", err)
		}

		if err := os.MkdirAll(os.ExpandEnv("$HOME/.kube"), 0755); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to create .kube directory: %v", err))
			return fmt.Errorf("failed to create .kube directory: %w", err)
		}

		sedCmd = fmt.Sprintf("sudo sed 's/127\\.0\\.0\\.1/%s/g' /etc/rancher/rke2/rke2.yaml | tee $HOME/.kube/config", mainIP)
		if err := exec.Command("sh", "-c", sedCmd).Run(); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to update KUBECONFIG file: %v", err))
			return fmt.Errorf("failed to update KUBECONFIG file: %w", err)
		}

		if err := exec.Command("chmod", "600", os.ExpandEnv("$HOME/.kube/config")).Run(); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to set permissions on KUBECONFIG file: %v", err))
			return fmt.Errorf("failed to set permissions on KUBECONFIG file: %w", err)
		}
		tokenCmd := exec.Command("sh", "-c", "cat /var/lib/rancher/rke2/server/node-token")
		tokenOutput, err := tokenCmd.Output()
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to get join token: %v", err))
			return fmt.Errorf("failed to get join token: %w", err)
		}
		joinToken := strings.TrimSpace(string(tokenOutput))
		// TODO this is totally wrong but placeholder for now
		joinCommand := fmt.Sprintf("export JOIN_TOKEN=%s; export SERVER_IP=%s; curl https://silogen.github.io/cluster-forge/deploy.sh | sudo bash", joinToken, mainIP)
		LogMessage(Info, "To setup additional nodes to join the cluster, run the following:")
		LogMessage(Info, joinCommand)

		app := tview.NewApplication()
		modal := tview.NewModal().
			SetText(fmt.Sprintf("To setup additional nodes to join the cluster, run the following:\n\n%s", joinCommand)).
			AddButtons([]string{"OK"})

		if err := app.SetRoot(modal, true).Run(); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to display modal dialog: %v", err))
			return fmt.Errorf("failed to display modal dialog: %w", err)
		}

		return nil
	},
}
