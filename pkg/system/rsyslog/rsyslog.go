package rsyslog

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path"

	log "github.com/sirupsen/logrus"
)

//go:embed 01-iscsi-filter.conf
var iscsiFilterConfig []byte

func Configure() error {
	// configure rsyslog rate limiting to reduce iSCSI log spam

	rsyslogConfigFile := "/etc/rsyslog.d/01-iscsi-filter.conf"

	// Check if config already exists - if so, skip to preserve modifications
	if _, err := os.Stat(rsyslogConfigFile); err == nil {
		log.Infof("Rsyslog config already exists at %s, skipping overwrite", rsyslogConfigFile)
		return nil
	}

	// Ensure the destination directory exists
	rsyslogDir := path.Dir(rsyslogConfigFile)
	if err := os.MkdirAll(rsyslogDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", rsyslogDir, err)
	}

	// Write the embedded config file
	if err := os.WriteFile(rsyslogConfigFile, iscsiFilterConfig, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %v", rsyslogConfigFile, err)
	}

	// apply the new configuration by restarting rsyslog
	if err := applyConfig(); err != nil {
		return fmt.Errorf("failed to apply rsyslog configuration: %v", err)
	}

	return nil
}

func applyConfig() error {
	// Restart rsyslog to apply changes
	restartSyslog := exec.Command("sudo", "systemctl", "restart", "rsyslog")
	if err := restartSyslog.Run(); err != nil {
		return fmt.Errorf("Error restarting rsyslog via systemctl: %v", err)
	}

	return nil
}
