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
package logrotate

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

//go:embed conf/iscsi-aggressive.conf
var iscsiAggressive []byte

//go:embed conf/rke2.conf
var rke2Components []byte

const (
	cronFilePath          = "/etc/cron.d/logrotate"
	logrotateConfigISCSI  = "/etc/logrotate.d/iscsi-aggressive"
	logrotateConfigRKE2   = "/etc/logrotate.d/rke2"
	logrotateCommandISCSI = "/usr/sbin/logrotate -f " + logrotateConfigISCSI
	logrotateCommandRKE2  = "/usr/sbin/logrotate -f " + logrotateConfigRKE2
	logFilePath           = "/var/log/logrotate-bloom.log"
	cronContent           = `# Managed by AMD cluster-bloom utility - do not edit manually
SHELL=/bin/sh
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin

# iSCSI logrotate - runs every 10 minutes
*/10 * * * * root ` + logrotateCommandISCSI + ` >> ` + logFilePath + ` 2>&1

# logroate for RKE2 logs - runs hourly
0 * * * * root /usr/sbin/logrotate -f ` + logrotateConfigRKE2 + ` >> ` + logFilePath + ` 2>&1
`
)

// logrotateConfig represents a logrotate configuration to be deployed
type logrotateConfig struct {
	destPath      string
	content       []byte
	logPathsToFix []string // log paths in existing configs to comment out
}

type Logger interface {
	LogMessage(level int, message string)
}

func Configure() error {
	// configure logrotate with aggressive setup to deal with iscsi log spam

	// preflight
	if !isLogrotateInstalled() {
		err := fmt.Errorf("logrotate not installed, aborting logrotate setup")
		log.Error(err)
		return err
	}

	// Define configurations to deploy
	configs := []logrotateConfig{
		{
			destPath: logrotateConfigISCSI,
			content:  iscsiAggressive,
			logPathsToFix: []string{
				"/var/log/iscsi/iscsi.log",
				"/var/log/iscsi/iscsi_trc.log",
			},
		},
		{
			destPath:      logrotateConfigRKE2,
			content:       rke2Components,
			logPathsToFix: []string{}, // RKE2 config doesn't need conflict resolution
		},
	}

	// Deploy each configuration
	for _, cfg := range configs {
		if err := deployConfig(cfg); err != nil {
			return fmt.Errorf("failed to deploy config %s: %v", cfg.destPath, err)
		}
	}

	// enable cronjob for logrotate execution (ISCSI specific)
	if err := setupCronJob(); err != nil {
		return fmt.Errorf("failed to setup cronjob: %v", err)
	}

	// apply the new logrotate configs
	if err := applyConfigs(configs); err != nil {
		return fmt.Errorf("failed to apply logrotate configs: %v", err)
	}

	log.Info("Logrotate configuration completed successfully.")
	return nil
}

func isLogrotateInstalled() bool {
	_, err := exec.LookPath("logrotate")
	return err == nil
}

// deployConfig handles creation and conflict resolution for a single logrotate config
func deployConfig(cfg logrotateConfig) error {
	// Create the config file
	if err := createConfig(cfg.destPath, cfg.content); err != nil {
		return fmt.Errorf("failed to create config: %v", err)
	}

	// Comment out existing logrotate blocks for conflict prevention / deduplication
	if len(cfg.logPathsToFix) > 0 {
		if err := commentOutLogrotateBlocks(cfg.destPath, cfg.logPathsToFix); err != nil {
			return fmt.Errorf("failed to comment out logrotate blocks: %v", err)
		}
	}

	return nil
}

func createConfig(destPath string, content []byte) error {
	filePermissions := os.FileMode(0644)

	// Ensure the destination directory exists
	logrotateDir := path.Dir(destPath)
	if err := os.MkdirAll(logrotateDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", logrotateDir, err)
	}

	// Check if the file already exists
	if _, err := os.Stat(destPath); err == nil {
		log.Infof("%s already exists, skipping creation.", destPath)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check if file %s exists: %v", destPath, err)
	}

	// Write the file from embedded content
	err := os.WriteFile(destPath, content, filePermissions)
	if err != nil {
		return fmt.Errorf("error writing logrotate config file: %v", err)
	}

	log.Infof("Created logrotate config: %s", destPath)
	return nil
}

func commentOutLogrotateBlocks(configFile string, logPaths []string) error {
	// conflict prevention / deduplication: comment out existing logrotate blocks for the specified log paths

	// Check if the config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("config file %s does not exist", configFile)
	}

	// Read the entire file
	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %v", configFile, err)
	}

	lines := strings.Split(string(content), "\n")
	var result []string
	inTargetBlock := false
	braceDepth := 0

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check if this line contains any of our target log paths
		for _, logPath := range logPaths {
			if strings.Contains(trimmedLine, logPath) {
				inTargetBlock = true
				braceDepth = 0
				break
			}
		}

		if inTargetBlock {
			// Count braces to track block boundaries
			braceDepth += strings.Count(trimmedLine, "{")
			braceDepth -= strings.Count(trimmedLine, "}")

			// Comment out the line
			result = append(result, "# "+line)

			// Exit block when we've closed all braces
			if braceDepth == 0 && strings.Contains(trimmedLine, "}") {
				inTargetBlock = false
			}
		} else {
			result = append(result, line)
		}
	}

	// Create backup
	backupFile := configFile + ".bak"
	if err := os.WriteFile(backupFile, content, 0644); err != nil {
		return fmt.Errorf("failed to create backup file %s: %v", backupFile, err)
	}

	// Write the modified content
	if err := os.WriteFile(configFile, []byte(strings.Join(result, "\n")), 0644); err != nil {
		return fmt.Errorf("failed to write modified config file %s: %v", configFile, err)
	}

	log.Infof("Successfully commented out logrotate blocks for: %v", logPaths)
	return nil
}

func setupCronJob() error {
	// Check if cron file already exists and contains our logrotate commands
	if existingContent, err := os.ReadFile(cronFilePath); err == nil {
		if strings.Contains(string(existingContent), logrotateCommandISCSI) &&
			strings.Contains(string(existingContent), logrotateCommandRKE2) {
			log.Info("Cron job already exists with logrotate commands, skipping")
			return ensureCronLogFile()
		}
		log.Info("Cron job exists but doesn't contain expected logrotate commands, updating...")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing cron file: %w", err)
	}

	// Ensure /etc/cron.d directory exists
	cronDir := filepath.Dir(cronFilePath)
	if err := os.MkdirAll(cronDir, 0755); err != nil {
		return fmt.Errorf("failed to create cron.d directory: %w", err)
	}

	// Write the cron file
	if err := os.WriteFile(cronFilePath, []byte(cronContent), 0644); err != nil {
		return fmt.Errorf("failed to write cron file: %w", err)
	}

	log.Info("Cron job created/updated at ", cronFilePath)

	return ensureCronLogFile()
}

func ensureCronLogFile() error {
	// Create log file if it doesn't exist
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		f, err := os.Create(logFilePath)
		if err != nil {
			return fmt.Errorf("failed to create log file: %w", err)
		}
		f.Close()

		// Set appropriate permissions
		if err := os.Chmod(logFilePath, 0644); err != nil {
			log.Warning("Failed to set log file permissions: ", err)
		}

		log.Info("Log file created at ", logFilePath)
	} else if err != nil {
		return fmt.Errorf("failed to check log file: %w", err)
	} else {
		log.Debug("Log file already exists at ", logFilePath)
	}

	return nil
}

func applyConfigs(configs []logrotateConfig) error {
	// Run logrotate in debug mode to verify each config
	for _, cfg := range configs {
		debugLogrotate := exec.Command("sudo", "logrotate", "-d", cfg.destPath)
		if err := debugLogrotate.Run(); err != nil {
			log.Warnf("Error executing logrotate for %s: %v", cfg.destPath, err)
		} else {
			log.Infof("✓ Successfully validated logrotate config: %s", cfg.destPath)
		}
	}

	return nil
}
