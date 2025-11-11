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
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

type Logger interface {
	LogMessage(level int, message string)
}

type ConfigParams struct {
	SourceFilePath  string
	DestinationPath string
	Permissions     os.FileMode
	Logger          Logger
}

func Configure() error {
	// configure logrotate with agressive setup to deal with iscsi log spam

	// preflight
	if !isLogrotateInstalled() {
		err := fmt.Errorf("logrotate not installed, aborting logrotate setup")
		log.Error(err)
		return err
	}

	configParams := &ConfigParams{
		SourceFilePath:  "logrotate/iscsi-aggressive",
		DestinationPath: "/etc/logrotate.d/",
		Permissions:     0644,
	}

	if err := createConfig(configParams); err != nil {
		err := fmt.Errorf("failed to create logrotate config: %v", err)
		log.Error(err)
		return err
	}

	logPaths := []string{
		"/var/log/iscsi/iscsi.log",
		"/var/log/iscsi/iscsi_trc.log",
	}

	configFile := filepath.Join(configParams.DestinationPath, "iscsi-aggressive")

	// comment out existing logrotate blocks for iscsi logs
	if err := commentOutLogrotateBlocks(configFile, logPaths); err != nil {
		err := fmt.Errorf("failed to comment out logrotate blocks: %v", err)
		log.Error(err)
		return err
	}

	if err := enableHourlyRotation(); err != nil {
		err := fmt.Errorf("failed to enable hourly logrotate config: %v", err)
		log.Error(err)
		return err
	}

	if err := applyConfigs(); err != nil {
		err := fmt.Errorf("failed to apply logrotate configs: %v", err)
		log.Error(err)
		return err
	}

	return nil
}

func isLogrotateInstalled() bool {
	_, err := exec.LookPath("logrotate")
	return err == nil
}

func createConfig(options *ConfigParams) error {
	var configFiles embed.FS

	sourceFilePath := options.SourceFilePath
	destinationPath := options.DestinationPath

	// default permissions to 0644 if not provided
	permissions := options.Permissions
	if permissions == 0 {
		permissions = os.FileMode(0644)
	}

	// strip out leading folders from the sourceFile if present
	sourceFile := filepath.Base(sourceFilePath)
	destinationFile := filepath.Join(destinationPath, sourceFile)
	// Ensure the destination directory exists
	if err := os.MkdirAll(destinationPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", destinationPath, err)
	}

	// Check if the file already exists
	if _, err := os.Stat(destinationFile); err == nil {
		fmt.Printf("  ✓ %s already exists, skipping creation.\n", destinationFile)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check if file %s exists: %v", destinationFile, err)
	}

	fmt.Printf("Installing logrotate config: %s -> %s\n", sourceFile, destinationFile)

	// Read the embedded file
	content, err := configFiles.ReadFile(sourceFilePath)
	if err != nil {
		return fmt.Errorf("failed to read embedded file %s: %v", sourceFilePath, err)
	}

	// Write the file with proper permissions
	if err := os.WriteFile(destinationFile, content, permissions); err != nil {
		return fmt.Errorf("failed to write file %s: %v", destinationFile, err)
	}

	fmt.Printf("  ✓ Successfully created %s\n", destinationFile)
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

func enableHourlyRotation() error {
	logrotateCmd := "/usr/sbin/logrotate -f /etc/logrotate.d/iscsi-aggressive"

	// Check if the hourly cron file exists
	hourlyCronFile := "/etc/cron.hourly/logrotate-hourly"
	if _, err := os.Stat(hourlyCronFile); os.IsNotExist(err) {
		// add cron.hourly folder if it doesn't exist
		if _, err := os.Stat("/etc/cron.hourly"); os.IsNotExist(err) {
			if err := os.MkdirAll("/etc/cron.hourly", 0755); err != nil {
				return fmt.Errorf("failed to create /etc/cron.hourly directory: %v", err)
			}
		}

		// create an empty file
		emptyFile, err := os.Create(hourlyCronFile)
		if err != nil {
			return fmt.Errorf("failed to create hourly cron file %s: %v", hourlyCronFile, err)
		}

		// write the logrotate command to the file
		_, err = emptyFile.WriteString(fmt.Sprintf("#!/bin/sh\n%s\n", logrotateCmd))
		if err != nil {
			return fmt.Errorf("failed to write to hourly cron file %s: %v", hourlyCronFile, err)
		}
		emptyFile.Close()
	} else {
		// file exists, ensure the logrotate command is present
		content, err := os.ReadFile(hourlyCronFile)
		if err != nil {
			return fmt.Errorf("failed to read hourly cron file %s: %v", hourlyCronFile, err)
		}

		if strings.Contains(string(content), logrotateCmd) {
			// command already present, nothing to do
			return nil
		} else {
			// append the command to the file
			f, err := os.OpenFile(hourlyCronFile, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("failed to open hourly cron file %s for appending: %v", hourlyCronFile, err)
			}
			defer f.Close()

			_, err = f.WriteString("\n" + logrotateCmd + "\n")
			if err != nil {
				return fmt.Errorf("failed to append to hourly cron file %s: %v", hourlyCronFile, err)
			}
		}
	}

	return nil
}

func applyConfigs() error {
	// Run logrotate in debug mode to verify config
	debugLogrotate := exec.Command("sudo", "logrotate", "-d", "/etc/logrotate.d/iscsi-aggressive")
	if err := debugLogrotate.Run(); err != nil {
		log.Infof("Error executing logrotate: %v", err)
	} else {
		log.Infof("  ✓ Successfully ran logrotate")
	}

	validateLogrotate := exec.Command("bash", "/opt/validate_logrotate.sh")
	output, err := validateLogrotate.CombinedOutput()
	if err != nil {
		log.Infof("Error running logrotate validation script: %v", err)
	} else {
		log.Infof("==== start logrotate script output ====")
		log.Infof("  ✓ Successfully validated logrotate setup")
		if len(output) > 0 {
			log.Infof("Script output: %s", string(output))
		}
	}
	return nil
}
