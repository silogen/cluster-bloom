package rsyslog

import (
	"embed"
	"fmt"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

func Configure() error {
	// configure rsyslog rate limiting to reduce iSCSI log spam
	err := setupRsyslogRateLimiting()
	if err != nil {
		return fmt.Errorf("failed to setup rsyslog rate limiting: %v", err)
	}

	// apply the new configuration by restarting rsyslog
	err = applyConfig()
	if err != nil {
		return fmt.Errorf("failed to apply rsyslog configuration: %v", err)
	}

	return nil
}

func setupRsyslogRateLimiting() error {
	// function to copy 01-iscsi-filter.conf to /etc/rsyslog.d/01-iscsi-filter.conf
	var configFiles embed.FS

	sourceFilePath := "logrotate/01-iscsi-filter.conf"
	destinationPath := "/etc/rsyslog.d/01-iscsi-filter.conf"

	// Read the embedded file
	content, err := configFiles.ReadFile(sourceFilePath)
	if err != nil {
		return fmt.Errorf("failed to read embedded file %s: %v", sourceFilePath, err)
	}

	// Write the file with proper permissions
	if err := os.WriteFile(destinationPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %v", destinationPath, err)
	}

	log.Infof("  ✓ Successfully created rsyslog rate limiting config at %s", destinationPath)
	return nil
}

func applyConfig() error {
	// Restart rsyslog to apply changes
	restartSyslog := exec.Command("sudo", "systemctl", "restart", "rsyslog")
	if err := restartSyslog.Run(); err != nil {
		log.Errorf("Error restarting rsyslog via systemctl: %v", err)
		return err
	} else {
		log.Infof("  ✓ Successfully restarted rsyslog via systemctl")
	}
	return nil
}
