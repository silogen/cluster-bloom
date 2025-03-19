package pkg

import (
	"os"
	"os/exec"
	"strings"
)

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
	if err := runCommand("sudo", "apt", "update"); err != nil {
		LogMessage(Error, "Failed to update packages: "+err.Error())
		return false
	}
	unameR, err := exec.Command("uname", "-r").Output()
	if err != nil {
		LogMessage(Error, "Error getting kernel version: "+err.Error())
		return false
	}
	kernelVersion := strings.TrimSpace(string(unameR))
	if err := runCommand("sudo", "apt", "install", "linux-headers-"+kernelVersion, "linux-modules-extra-"+kernelVersion); err != nil {
		LogMessage(Error, "Failed to install Linux headers: "+err.Error())
		return false
	}
	if err := runCommand("sudo", "apt", "install", "python3-setuptools", "python3-wheel"); err != nil {
		LogMessage(Error, "Failed to install Python dependencies: "+err.Error())
		return false
	}
	debFile := "amdgpu-install_6.3.60302-1_all.deb"
	url := "https://repo.radeon.com/amdgpu-install/6.3.2/" + ubuntuCodename + "/jammy/" + debFile
	if err := runCommand("wget", url); err != nil {
		LogMessage(Error, "Failed to download amdgpu-install: "+err.Error())
		return false
	}
	if err := runCommand("sudo", "apt", "install", "./"+debFile); err != nil {
		LogMessage(Error, "Failed to install amdgpu-install package: "+err.Error())
		return false
	}
	if err := runCommand("sudo", "amdgpu-install", "--usecase=rocm,dkms"); err != nil {
		LogMessage(Error, "Failed to install ROCm: "+err.Error())
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

func runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
