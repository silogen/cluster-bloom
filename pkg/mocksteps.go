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
	"time"

	"github.com/spf13/viper"
)

// Mock versions of all steps that return success immediately with simulated variables

var MockValidateArgsStep = Step{
	Id:          "ValidateArgsStep",
	Name:        "Validate Configuration",
	Description: "Validate all configuration arguments (MOCK)",
	Action: func() StepResult {
		time.Sleep(200 * time.Millisecond)
		LogMessage(Info, "Mock: Configuration validated successfully")
		return StepResult{Error: nil}
	},
}

var MockValidateSystemRequirementsStep = Step{
	Id:          "ValidateSystemRequirementsStep",
	Name:        "Validate System Requirements",
	Description: "Validate system resources (MOCK)",
	Action: func() StepResult {
		time.Sleep(300 * time.Millisecond)
		LogMessage(Info, "Mock: System requirements validated (32GB RAM, 8 CPUs, 500GB disk)")
		return StepResult{Error: nil}
	},
}

var MockCheckUbuntuStep = Step{
	Id:          "CheckUbuntuStep",
	Name:        "Check Ubuntu Version",
	Description: "Verify running on supported Ubuntu version (MOCK)",
	Action: func() StepResult {
		time.Sleep(100 * time.Millisecond)
		LogMessage(Info, "Mock: Ubuntu 22.04 LTS detected")
		return StepResult{Error: nil}
	},
}

var MockInstallDependentPackagesStep = Step{
	Id:          "InstallDependentPackagesStep",
	Name:        "Install Dependent Packages",
	Description: "Ensure jq, nfs-common, open-iscsi, and chrony are installed (MOCK)",
	Action: func() StepResult {
		time.Sleep(500 * time.Millisecond)
		LogMessage(Info, "Mock: Installed packages: jq, nfs-common, open-iscsi, chrony")
		return StepResult{Error: nil}
	},
}

var MockCreateChronyConfigStep = Step{
	Id:          "CreateChronyConfigStep",
	Name:        "Create Chrony Config",
	Description: "Create chrony config for time synchronization (MOCK)",
	Action: func() StepResult {
		time.Sleep(200 * time.Millisecond)
		if viper.GetBool("FIRST_NODE") {
			LogMessage(Info, "Mock: Created chrony config for first node (server mode)")
		} else {
			LogMessage(Info, "Mock: Created chrony config for additional node (client mode)")
		}
		return StepResult{Error: nil}
	},
}

var MockOpenPortsStep = Step{
	Id:          "OpenPortsStep",
	Name:        "Open Ports",
	Description: "Ensure needed ports are open in iptables (MOCK)",
	Action: func() StepResult {
		time.Sleep(300 * time.Millisecond)
		LogMessage(Info, "Mock: Opened ports: 6443, 9345, 10250, 2379-2380, 30000-32767")
		return StepResult{Error: nil}
	},
}

var MockCheckPortsBeforeOpeningStep = Step{
	Id:          "CheckPortsBeforeOpeningStep",
	Name:        "Checking Ports",
	Description: "Ensure needed ports are not in use (MOCK)",
	Action: func() StepResult {
		time.Sleep(250 * time.Millisecond)
		LogMessage(Info, "Mock: All required ports are available")
		return StepResult{Error: nil}
	},
}

var MockInstallK8SToolsStep = Step{
	Id:          "InstallK8SToolsStep",
	Name:        "Install Kubernetes tools",
	Description: "Install kubectl and k9s (MOCK)",
	Action: func() StepResult {
		time.Sleep(400 * time.Millisecond)
		LogMessage(Info, "Mock: Installed kubectl v1.28.3 and k9s v0.27.4")
		return StepResult{Error: nil}
	},
}

var MockInotifyInstancesStep = Step{
	Id:          "InotifyInstancesStep",
	Name:        "Verify inotify instances",
	Description: "Verify, or update, inotify instances (MOCK)",
	Action: func() StepResult {
		time.Sleep(150 * time.Millisecond)
		LogMessage(Info, "Mock: Updated fs.inotify.max_user_instances to 8192")
		return StepResult{Error: nil}
	},
}

var MockSetupAndCheckRocmStep = Step{
	Id:          "SetupAndCheckRocmStep",
	Name:        "Setup and Check ROCm",
	Description: "Verify, setup, and check ROCm devices (MOCK)",
	Skip: func() bool {
		if !viper.GetBool("GPU_NODE") {
			LogMessage(Info, "Skipping ROCm setup for non-GPU node")
			return true
		}
		return false
	},
	Action: func() StepResult {
		time.Sleep(800 * time.Millisecond)
		LogMessage(Info, "Mock: ROCm 6.0.2 installed successfully")
		LogMessage(Info, "Mock: Detected GPUs:\n      8   AMD Instinct MI250X")
		viper.Set("gpu_count", 8)
		viper.Set("gpu_model", "AMD Instinct MI250X")
		return StepResult{Error: nil}
	},
}

var MockSetupRKE2Step = Step{
	Id:          "SetupRKE2Step",
	Name:        "Setup RKE2",
	Description: "Setup RKE2 server and configure necessary modules (MOCK)",
	Action: func() StepResult {
		time.Sleep(1200 * time.Millisecond)
		if viper.GetBool("FIRST_NODE") {
			LogMessage(Info, "Mock: RKE2 v1.28.3+rke2r1 server setup complete")
			LogMessage(Info, "Mock: Generated cluster token: K10abc123xyz...")
			viper.Set("join_token", "K10abc123xyz456def789ghi012jkl345::server:mock-token-data")
		} else if viper.GetBool("CONTROL_PLANE") {
			LogMessage(Info, "Mock: RKE2 control plane joined to cluster")
		} else {
			LogMessage(Info, "Mock: RKE2 worker node joined to cluster")
		}
		return StepResult{Error: nil}
	},
}

var MockCleanDisksStep = Step{
	Id:          "CleanDisksStep",
	Name:        "Clean disks",
	Description: "Remove any previous longhorn temp drives (MOCK)",
	Action: func() StepResult {
		time.Sleep(300 * time.Millisecond)
		LogMessage(Info, "Mock: Cleaned previous Longhorn disk configurations")
		return StepResult{Error: nil}
	},
}

var MockSetupMultipathStep = Step{
	Id:          "SetupMultipathStep",
	Name:        "Setup Multipath",
	Description: "Configure multipath to blacklist standard devices (MOCK)",
	Action: func() StepResult {
		time.Sleep(250 * time.Millisecond)
		LogMessage(Info, "Mock: Configured multipath blacklist for /dev/sda, /dev/sdb")
		return StepResult{Error: nil}
	},
}

var MockUpdateModprobeStep = Step{
	Id:          "UpdateModprobeStep",
	Name:        "Update Modprobe",
	Description: "Update Modprobe to unblacklist amdgpu (MOCK)",
	Action: func() StepResult {
		time.Sleep(200 * time.Millisecond)
		LogMessage(Info, "Mock: Updated modprobe.d to enable amdgpu module")
		return StepResult{Error: nil}
	},
}

var MockSelectDrivesStep = Step{
	Id:          "SelectDrivesStep",
	Name:        "Select Unmounted Disks",
	Description: "Identify and select unmounted physical disks (MOCK)",
	Action: func() StepResult {
		time.Sleep(400 * time.Millisecond)

		// If SELECTED_DISKS is already set, use it
		if viper.IsSet("SELECTED_DISKS") && viper.GetString("SELECTED_DISKS") != "" {
			LogMessage(Info, "Mock: Using pre-configured disk selection")
			viper.Set("selected_disks", []string{"/dev/nvme0n1", "/dev/nvme1n1"})
			return StepResult{Error: nil}
		}

		// Simulate disk selection
		mockDisks := []string{"/dev/nvme0n1", "/dev/nvme1n1", "/dev/nvme2n1", "/dev/nvme3n1"}
		LogMessage(Info, "Mock: Found unmounted disks: /dev/nvme0n1, /dev/nvme1n1, /dev/nvme2n1, /dev/nvme3n1")
		LogMessage(Info, "Mock: Auto-selected disks for testing: /dev/nvme0n1, /dev/nvme1n1")

		viper.Set("selected_disks", mockDisks[:2])
		return StepResult{Message: "Selected disks: /dev/nvme0n1, /dev/nvme1n1"}
	},
}

var MockMountSelectedDrivesStep = Step{
	Id:          "MountSelectedDrivesStep",
	Name:        "Mount Selected Disks",
	Description: "Mount the selected physical disks (MOCK)",
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
		time.Sleep(600 * time.Millisecond)
		LogMessage(Info, "Mock: Formatted and mounted /dev/nvme0n1 to /mnt/disk1")
		LogMessage(Info, "Mock: Formatted and mounted /dev/nvme1n1 to /mnt/disk2")
		LogMessage(Info, "Mock: Updated /etc/fstab for persistent mounts")
		viper.Set("mounted_disks", []string{"/mnt/disk1", "/mnt/disk2"})
		return StepResult{Error: nil}
	},
}

var MockGenerateNodeLabelsStep = Step{
	Id:          "GenerateNodeLabelsStep",
	Name:        "Generate node Labels",
	Description: "Generate labels for the node based on its configuration (MOCK)",
	Action: func() StepResult {
		time.Sleep(250 * time.Millisecond)
		if viper.GetBool("GPU_NODE") {
			LogMessage(Info, "Mock: Generated node labels: gpu=true, gpu-model=mi250x")
		} else {
			LogMessage(Info, "Mock: Generated node labels: gpu=false")
		}
		return StepResult{Error: nil}
	},
}

var MockSetupMetallbStep = Step{
	Id:          "SetupMetallbStep",
	Name:        "Setup MetalLB manifests",
	Description: "Copy MetalLB YAML files to the RKE2 manifests directory (MOCK)",
	Skip: func() bool {
		if viper.GetBool("FIRST_NODE") == false {
			LogMessage(Info, "Skipping MetalLB setup for additional nodes.")
			return true
		}
		return false
	},
	Action: func() StepResult {
		time.Sleep(400 * time.Millisecond)
		LogMessage(Info, "Mock: Deployed MetalLB v0.13.12 to cluster")
		return StepResult{Error: nil}
	},
}

var MockSetupLonghornStep = Step{
	Id:          "SetupLonghornStep",
	Name:        "Setup Longhorn manifests",
	Description: "Copy Longhorn YAML files to the RKE2 manifests directory (MOCK)",
	Skip: func() bool {
		if viper.GetBool("SKIP_DISK_CHECK") {
			LogMessage(Info, "Skipping Longhorn setup as SKIP_DISK_CHECK is set.")
			return true
		}
		return false
	},
	Action: func() StepResult {
		time.Sleep(800 * time.Millisecond)
		LogMessage(Info, "Mock: Deployed Longhorn v1.5.3 to cluster")
		LogMessage(Info, "Mock: Configured storage with 3.8TB usable capacity")
		LogMessage(Info, "Mock: Node annotation job completed successfully")
		return StepResult{Error: nil}
	},
}

var MockCreateMetalLBConfigStep = Step{
	Id:          "CreateMetalLBConfigStep",
	Name:        "Setup AddressPool for MetalLB",
	Description: "Create IPAddressPool and L2Advertisement resources for MetalLB (MOCK)",
	Skip: func() bool {
		if !viper.GetBool("FIRST_NODE") {
			LogMessage(Info, "Skipping for additional nodes.")
			return true
		}
		return false
	},
	Action: func() StepResult {
		time.Sleep(350 * time.Millisecond)
		ipRange := viper.GetString("METALLB_IP_RANGE")
		if ipRange == "" {
			ipRange = "192.168.1.240-192.168.1.250"
		}
		LogMessage(Info, "Mock: Created MetalLB IPAddressPool with range: "+ipRange)
		LogMessage(Info, "Mock: Created L2Advertisement for local network")
		return StepResult{Error: nil}
	},
}

var MockPrepareRKE2Step = Step{
	Id:          "PrepareRKE2Step",
	Name:        "Prepare for RKE2",
	Description: "RKE2 preparations (MOCK)",
	Action: func() StepResult {
		time.Sleep(300 * time.Millisecond)
		LogMessage(Info, "Mock: Created RKE2 configuration directories")
		LogMessage(Info, "Mock: Downloaded RKE2 installation script")
		return StepResult{Error: nil}
	},
}

var MockHasSufficientRancherPartitionStep = Step{
	Id:          "HasSufficientRancherPartitionStep",
	Name:        "Check /var/lib/rancher Partition Size",
	Description: "Check if the /var/lib/rancher partition size is sufficient (MOCK)",
	Skip: func() bool {
		if !viper.GetBool("GPU_NODE") {
			LogMessage(Info, "Skipping /var/lib/rancher partition check for CPU node.")
			return true
		}
		return false
	},
	Action: func() StepResult {
		time.Sleep(150 * time.Millisecond)
		LogMessage(Info, "Mock: /var/lib/rancher partition has 850GB available (exceeds 500GB minimum)")
		return StepResult{Error: nil}
	},
}

var MockNVMEDrivesAvailableStep = Step{
	Id:          "NVMEDrivesAvailableStep",
	Name:        "Check NVMe Drives",
	Description: "Check if NVMe drives are available (MOCK)",
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
		time.Sleep(200 * time.Millisecond)
		LogMessage(Info, "Mock: Found 4 NVMe drives available")
		return StepResult{Error: nil}
	},
}

var MockSetupKubeConfig = Step{
	Id:          "SetupKubeConfig",
	Name:        "Setup KubeConfig",
	Description: "Setup and configure KubeConfig (MOCK)",
	Skip: func() bool {
		if !viper.GetBool("FIRST_NODE") {
			LogMessage(Info, "Skipping SetupKubeConfig for additional nodes.")
			return true
		}
		return false
	},
	Action: func() StepResult {
		time.Sleep(450 * time.Millisecond)
		mockIP := "10.0.100.50"
		LogMessage(Info, "Mock: Configured kubeconfig with server IP: "+mockIP)
		LogMessage(Info, "Mock: Created ~/.kube/config for current user")
		LogMessage(Info, "Mock: Updated PATH to include k9s")
		viper.Set("server_ip", mockIP)
		return StepResult{Error: nil}
	},
}

func MockCreateBloomConfigMapStepFunc(version string) Step {
	return Step{
		Id:          "CreateBloomConfigMapStep",
		Name:        "Create Bloom ConfigMap",
		Description: "Create a ConfigMap with bloom configuration (MOCK)",
		Action: func() StepResult {
			time.Sleep(350 * time.Millisecond)
			if viper.GetBool("FIRST_NODE") {
				LogMessage(Info, "Mock: Created bloom ConfigMap in default namespace")
				LogMessage(Info, "Mock: ConfigMap version: "+version)
			} else {
				LogMessage(Info, "Mock: Created bloom ConfigMap Pod for node configuration")
			}
			return StepResult{Error: nil}
		},
	}
}

var MockCreateDomainConfigStep = Step{
	Id:          "CreateDomainConfigStep",
	Name:        "Create Domain Configuration",
	Description: "Create domain ConfigMap and TLS secret (MOCK)",
	Skip: func() bool {
		if !viper.GetBool("FIRST_NODE") {
			LogMessage(Info, "Skipped for additional node")
			return true
		}
		return false
	},
	Action: func() StepResult {
		time.Sleep(500 * time.Millisecond)
		domain := viper.GetString("DOMAIN")
		if domain == "" {
			domain = "cluster.example.com"
		}

		useCertManager := viper.GetBool("USE_CERT_MANAGER")

		LogMessage(Info, "Mock: Created domain ConfigMap with domain: "+domain)

		if !useCertManager {
			certOption := viper.GetString("CERT_OPTION")
			if certOption == "generate" {
				LogMessage(Info, "Mock: Generated self-signed TLS certificate for "+domain)
			} else if certOption == "existing" {
				LogMessage(Info, "Mock: Using existing TLS certificate")
			}
			LogMessage(Info, "Mock: Created TLS secret in kgateway-system namespace")
		} else {
			LogMessage(Info, "Mock: Cert-manager will handle TLS certificates")
		}

		return StepResult{Message: "Domain configuration complete"}
	},
}

var MockSetupClusterForgeStep = Step{
	Id:          "SetupClusterForgeStep",
	Name:        "Setup Cluster Forge",
	Description: "Setup and configure Cluster Forge (MOCK)",
	Skip: func() bool {
		if !viper.GetBool("FIRST_NODE") {
			LogMessage(Info, "Skipping for additional nodes.")
			return true
		}
		return false
	},
	Action: func() StepResult {
		time.Sleep(700 * time.Millisecond)
		LogMessage(Info, "Mock: Deployed Cluster Forge v1.0.5")
		LogMessage(Info, "Mock: Cluster Forge UI accessible at https://forge.cluster.example.com")
		return StepResult{Error: nil}
	},
}

var MockFinalOutput = Step{
	Id:          "FinalOutput",
	Name:        "Output",
	Description: "Generate output after installation (MOCK)",
	Action: func() StepResult {
		time.Sleep(300 * time.Millisecond)

		if viper.GetBool("FIRST_NODE") {
			// Mock join token
			mockToken := "K10abc123xyz456def789ghi012jkl345::server:mock-token-data-very-long-string"
			mockIP := viper.GetString("server_ip")
			if mockIP == "" {
				mockIP = "10.0.100.50"
			}

			LogMessage(Info, "Mock: Generated join token for additional nodes")
			LogMessage(Info, "Mock: Server IP: "+mockIP)
			LogMessage(Info, "Mock: Additional nodes should use JOIN_TOKEN and SERVER_IP from additional_node_command.txt")

			// Set variables for the final output display
			viper.Set("join_token", mockToken)
			viper.Set("server_ip", mockIP)

			return StepResult{Message: "Mock: To setup additional nodes, use the command in additional_node_command.txt"}
		} else {
			message := "Mock: Longhorn drive setup instructions available in longhorn_drive_setup.txt"
			LogMessage(Info, message)
			return StepResult{Message: message}
		}
	},
}

var MockUpdateUdevRulesStep = Step{
	Id:          "UpdateUdevRulesStep",
	Name:        "Update Udev Rules",
	Description: "Update AMD device-specific udev rules (MOCK)",
	Skip: func() bool {
		if !viper.GetBool("GPU_NODE") {
			LogMessage(Info, "Skipped for non-GPU node")
			return true
		}
		return false
	},
	Action: func() StepResult {
		time.Sleep(200 * time.Millisecond)
		LogMessage(Info, "Mock: Created /etc/udev/rules.d/70-amdgpu.rules")
		LogMessage(Info, "Mock: Reloaded udev rules and triggered device updates")
		return StepResult{Error: nil}
	},
}

var MockCleanLonghornMountsStep = Step{
	Id:          "CleanLonghornMountsStep",
	Name:        "Clean Longhorn Mounts",
	Description: "Clean up Longhorn PVCs and mounts (MOCK)",
	Action: func() StepResult {
		time.Sleep(400 * time.Millisecond)
		LogMessage(Info, "Mock: Cleaned up Longhorn mounts from /dev/longhorn/*")
		LogMessage(Info, "Mock: Cleaned up CSI plugin mounts")
		return StepResult{Error: nil}
	},
}

var MockUninstallRKE2Step = Step{
	Id:          "UninstallRKE2Step",
	Name:        "Uninstall RKE2",
	Description: "Execute the RKE2 uninstall script (MOCK)",
	Action: func() StepResult {
		time.Sleep(600 * time.Millisecond)
		LogMessage(Info, "Mock: Stopped RKE2 services")
		LogMessage(Info, "Mock: Removed RKE2 installation and data directories")
		LogMessage(Info, "Mock: Uninstall completed successfully")
		return StepResult{Error: nil}
	},
}
