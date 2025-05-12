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
	"io"
	"os/exec"
	"strings"
	"github.com/spf13/viper"
	log "github.com/sirupsen/logrus"
)

func CheckGPUAvailability() error {
	LogMessage(Info, "Running lsmod to check for amdgpu module")

	output, err := exec.Command("sh", "-c", "lsmod").CombinedOutput()

	if err != nil {
		return fmt.Errorf("Failed to run lsmod: "+err.Error())
	} 

	// grep will give an error if the module is not found, but we want to check the output
	output, err = exec.Command("sh", "-c", "lsmod | grep '^amdgpu'").CombinedOutput()	
	if len(output) == 0 {
		LogMessage(Warn, "WARNING: The amdgpu module is not loaded")
	} else {
		LogMessage(Info, "Result of lsmod: "+string(output))
	}
	return nil
}

func CheckAndInstallROCM() bool {
	_, err := exec.LookPath("rocm-smi")
	if err == nil {
		printROCMVersion()
		return true
	}
	LogMessage(Warn, "rocm-smi not found")
	output, err := exec.Command("sh", "-c", "grep VERSION_CODENAME /etc/os-release | cut -d= -f2").Output()
	if err != nil {
		LogMessage(Error, "Error getting Ubuntu codename: "+err.Error())
		return false
	}
	ubuntuCodename := strings.TrimSpace(string(output))
	_, err = runCommand("sudo", "apt", "update")
	if err != nil {
		LogMessage(Error, "Failed to update packages: "+err.Error())
		return false
	}

	unameR, err := exec.Command("uname", "-r").Output()
	if err != nil {
		LogMessage(Error, "Error getting kernel version: "+err.Error())
		return false
	}
	kernelVersion := strings.TrimSpace(string(unameR))
	_, err = runCommand("sudo", "apt", "install", "linux-headers-"+kernelVersion, "linux-modules-extra-"+kernelVersion)
	if err != nil {
		LogMessage(Error, "Failed to install Linux headers: "+err.Error())
		return false
	}
	_, err = runCommand("sudo", "apt", "install", "python3-setuptools", "python3-wheel")
	if err != nil {
		LogMessage(Error, "Failed to install Python dependencies: "+err.Error())
		return false
	}

	debFile := viper.GetString("ROCM_DEB_PACKAGE")
	url := viper.GetString("ROCM_BASE_URL") + ubuntuCodename + "/" + debFile
	_, err = runCommand("wget", url)
	if err != nil {
		LogMessage(Error, "Failed to download amdgpu-install: "+err.Error())
		return false
	} else {
		LogMessage(Info, "Successfully downloaded amdgpu-install")
	}
	_, err = runCommand("sudo", "apt", "install", "-y", "./"+debFile)
	if err != nil {
		LogMessage(Error, "Failed to install amdgpu-install package: "+err.Error())
		return false
	} else {
		LogMessage(Info, "Successfully installed amdgpu-install package")
	}
	_, err = runCommand("sudo", "amdgpu-install", "--usecase=rocm,dkms", "--yes")
	if err != nil {
		LogMessage(Error, "Failed to install ROCm: "+err.Error())
		return false
	} else {
		LogMessage(Info, "Successfully installed ROCm")
	}
	_, err = exec.Command("modprobe", "amdgpu").Output()
	if err != nil {
		LogMessage(Error, "Error loading modprobe amdgpu: "+err.Error())
		return false
	}

	printROCMVersion()
	return true
}

func printROCMVersion() {
	output, err := exec.Command("cat", "/opt/rocm/.info/version").Output()
	if err != nil {
		LogMessage(Error, "Error reading ROCm version: "+err.Error())
		return
	}
	LogMessage(Info, "ROCm Version: "+strings.TrimSpace(string(output)))
}

func runCommand(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			LogMessage(Debug, fmt.Sprintf("[%s %s] stderr: %s", command, strings.Join(args, " "), line))
			log.Debug(fmt.Sprintf("[%s %s] stderr: %s", command, strings.Join(args, " "), line))
		}
	}()
	stdoutBytes, err := io.ReadAll(stdout)
	if err != nil {
		return "", fmt.Errorf("failed to read stdout: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return string(stdoutBytes), fmt.Errorf("command failed: %w", err)
	}

	return string(stdoutBytes), nil
}
