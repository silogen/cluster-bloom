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
	// wrapper function to setup logrotate for iscsi logs

	configParams := &ConfigParams{
		SourceFilePath:  "logrotate/iscsi-aggressive",
		DestinationPath: "/etc/logrotate.d/",
		Permissions:     0644,
	}

	if err := createConfig(configParams); err != nil {
		return fmt.Errorf("failed to create logrotate config: %v", err)
	}

	logPaths := []string{
		"/var/log/iscsi/iscsi.log",
		"/var/log/iscsi/iscsi_trc.log",
	}

	configFile := filepath.Join(configParams.DestinationPath, "iscsi-aggressive")
	if err := commentOutLogrotateBlocks(configFile, logPaths); err != nil {
		return fmt.Errorf("failed to comment out logrotate blocks: %v", err)
	}

	if err := applyConfigs(); err != nil {
		return fmt.Errorf("failed to apply logrotate configs: %v", err)
	}

	return nil
}

func checkLogrotateInstalled() bool {
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
	// Check if the config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("config file %s does not exist", configFile)
	}

	// Build the sed pattern from the log paths
	pattern := buildSedPattern(logPaths)

	// Construct the sed command
	// -i.bak creates a backup with .bak extension
	sedCmd := fmt.Sprintf("/%s/,/^}/ s/^/# /", pattern)

	cmd := exec.Command("sed", "-i.bak", sedCmd, configFile)

	// Capture output for error reporting
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sed command failed: %v\nOutput: %s", err, output)
	}

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
		log.Infof(fmt.Sprintf("Error executing logrotate: %v", err))
	} else {
		log.Infof(fmt.Sprintf("  ✓ Successfully ran logrotate"))
	}

	validateLogrotate := exec.Command("bash", "/opt/validate_logrotate.sh")
	output, err := validateLogrotate.CombinedOutput()
	if err != nil {
		log.Infof(fmt.Sprintf("Error running logrotate validation script: %v", err))
	} else {
		log.Infof(fmt.Sprintf("==== start logrotate script output ===="))
		log.Infof(fmt.Sprintf("%s, string(output)"))
		log.Infof("  ✓ Successfully validated logrotate setup")
		if len(output) > 0 {
			log.Infof(fmt.Sprintf("Script output: %s", string(output)))
		}
	}
}

// buildSedPattern creates a sed-compatible regex pattern from log paths
func buildSedPattern(logPaths []string) string {
	var escapedPaths []string

	for _, path := range logPaths {
		// Escape special regex characters in the path
		escaped := escapeForSed(path)
		escapedPaths = append(escapedPaths, escaped)
	}

	// Join with \| for sed's OR operator
	return strings.Join(escapedPaths, `\|`)
}

// escapeForSed escapes special characters for use in sed regex
func escapeForSed(s string) string {
	// Escape common special characters in file paths
	replacer := strings.NewReplacer(
		`/`, `\/`, // Forward slashes
		`.`, `\.`, // Dots
		`*`, `\*`, // Asterisks
		`[`, `\[`, // Square brackets
		`]`, `\]`,
		`^`, `\^`, // Caret
		`$`, `\$`, // Dollar
	)
	return replacer.Replace(s)
}
