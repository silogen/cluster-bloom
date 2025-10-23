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
	"strings"

	"github.com/silogen/cluster-bloom/pkg/command"
	"github.com/spf13/viper"
)

func CheckGPUAvailability() error {
	LogMessage(Info, "Running lsmod to check for amdgpu module")

	output, err := command.CombinedOutput(true, "sh", "-c", "lsmod")

	if err != nil {
		return fmt.Errorf("failed to run lsmod: %w", err)
	}

	// grep will give an error if the module is not found, but we want to check the output
	output, err = command.CombinedOutput(true, "sh", "-c", "lsmod | grep '^amdgpu'")
	if len(output) == 0 {
		LogMessage(Warn, "WARNING: The amdgpu module is not loaded")
	} else {
		LogMessage(Info, "Result of lsmod: "+string(output))
	}
	return nil
}

func CheckAndInstallROCM() bool {
	_, err := command.LookPath("rocm-smi")
	if err == nil {
		printROCMVersion()
		return true
	}
	LogMessage(Warn, "rocm-smi not found")
	output, err := command.Output(true, "sh", "-c", "grep VERSION_CODENAME /etc/os-release | cut -d= -f2")
	if err != nil {
		LogMessage(Error, "Error getting Ubuntu codename: "+err.Error())
		return false
	}
	ubuntuCodename := strings.TrimSpace(string(output))
	_, err = command.Run(false, "sudo", "apt", "update")
	if err != nil {
		LogMessage(Error, "Failed to update packages: "+err.Error())
		return false
	}

	unameR, err := command.Output(true, "uname", "-r")
	if err != nil {
		LogMessage(Error, "Error getting kernel version: "+err.Error())
		return false
	}
	kernelVersion := strings.TrimSpace(string(unameR))
	_, err = command.Run(false, "sudo", "apt", "install", "linux-headers-"+kernelVersion, "linux-modules-extra-"+kernelVersion)
	if err != nil {
		LogMessage(Error, "Failed to install Linux headers: "+err.Error())
		return false
	}
	_, err = command.Run(false, "sudo", "apt", "install", "python3-setuptools", "python3-wheel")
	if err != nil {
		LogMessage(Error, "Failed to install Python dependencies: "+err.Error())
		return false
	}

	debFile := viper.GetString("ROCM_DEB_PACKAGE")
	url := viper.GetString("ROCM_BASE_URL") + ubuntuCodename + "/" + debFile
	_, err = command.Run(false, "wget", url)
	if err != nil {
		LogMessage(Error, "Failed to download amdgpu-install: "+err.Error())
		return false
	} else {
		LogMessage(Info, "Successfully downloaded amdgpu-install")
	}
	_, err = command.Run(false, "sudo", "apt", "install", "-y", "./"+debFile)
	if err != nil {
		LogMessage(Error, "Failed to install amdgpu-install package: "+err.Error())
		return false
	} else {
		LogMessage(Info, "Successfully installed amdgpu-install package")
	}
	_, err = command.Run(false, "sudo", "amdgpu-install", "--usecase=rocm,dkms", "--yes")
	if err != nil {
		LogMessage(Error, "Failed to install ROCm: "+err.Error())
		return false
	} else {
		LogMessage(Info, "Successfully installed ROCm")
	}
	_, err = command.Output(true, "modprobe", "amdgpu")
	if err != nil {
		LogMessage(Error, "Error loading modprobe amdgpu: "+err.Error())
		return false
	}

	printROCMVersion()
	return true
}

func printROCMVersion() {
	output, err := command.Output(true, "cat", "/opt/rocm/.info/version")
	if err != nil {
		LogMessage(Error, "Error reading ROCm version: "+err.Error())
		return
	}
	LogMessage(Info, "ROCm Version: "+strings.TrimSpace(string(output)))
}
