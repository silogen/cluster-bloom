package pkg

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

func SetupFirstRKE2() error {
	commands := []struct {
		command string
		args    []string
	}{
		{"modprobe", []string{"iscsi_tcp"}},
		{"modprobe", []string{"dm_mod"}},
		{"sh", []string{"-c", "/usr/local/bin/rke2-uninstall.sh || true"}},
		{"sh", []string{"-c", "clean_disks"}},
		{"mkdir", []string{"-p", "/etc/rancher/rke2"}},
		{"chmod", []string{"0755", "/etc/rancher/rke2"}},
		{"sh", []string{"-c", "curl -sfL https://get.rke2.io | sh -"}},
		{"systemctl", []string{"enable", "rke2-server.service"}},
		{"systemctl", []string{"start", "rke2-server.service"}},
	}

	for _, cmd := range commands {
		if err := runCommand(cmd.command, cmd.args...); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to execute command '%s %v': %v", cmd.command, cmd.args, err))
			return fmt.Errorf("failed to execute command '%s %v': %w", cmd.command, cmd.args, err)
		}
		LogMessage(Info, fmt.Sprintf("Successfully executed command: %s %v", cmd.command, cmd.args))
	}

	return nil
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
	if err := os.MkdirAll("/etc/rancher/rke2", 0755); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to create directory /etc/rancher/rke2: %v", err))
		return err
	}

	configContent := fmt.Sprintf("server: https://%s:9345\ntoken: %s\n", serverIP, joinToken)
	if err := os.WriteFile(rke2ConfigPath, []byte(configContent), 0644); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to write to %s: %v", rke2ConfigPath, err))
		return err
	}
	commands := []struct {
		command string
		args    []string
	}{
		{"modprobe", []string{"iscsi_tcp"}},
		{"modprobe", []string{"dm_mod"}},
		{"sh", []string{"-c", "/usr/local/bin/rke2-uninstall.sh || true"}},
		{"sh", []string{"-c", "curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE=agent sh -"}},
		{"systemctl", []string{"enable", "rke2-agent.service"}},
		{"systemctl", []string{"start", "rke2-agent.service"}},
	}
	for _, cmd := range commands {
		if err := runCommand(cmd.command, cmd.args...); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to execute command '%s %v': %v", cmd.command, cmd.args, err))
			return fmt.Errorf("failed to execute command '%s %v': %w", cmd.command, cmd.args, err)
		}
		LogMessage(Info, fmt.Sprintf("Successfully executed command: %s %v", cmd.command, cmd.args))
	}

	return nil
}
