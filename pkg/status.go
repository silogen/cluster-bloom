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
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type StepStatus struct {
	Name      string
	Status    string
	Timestamp time.Time
	Error     string
}

type BloomStatus struct {
	LogFile         string
	LastModified    time.Time
	Steps           []StepStatus
	Kubeconfig      string
	ClusterDisks            []string
	ClusterPremountedDisks  []string
	Domain          string
	FirstNode       bool
	ControlPlane    bool
	GPUNode         bool
	ServerIP        string
	ClusterForge    string
	Errors          []string
	OSError         string
	TotalSteps      int
	// Additional config values
	ConfigValues    map[string]string
}

func ParseBloomLog(logPath string) (*BloomStatus, error) {
	file, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	status := &BloomStatus{
		LogFile:      logPath,
		LastModified: stat.ModTime(),
		Steps:        []StepStatus{},
		Errors:       []string{},
		ConfigValues: make(map[string]string),
	}

	scanner := bufio.NewScanner(file)

	stepRegex := regexp.MustCompile(`msg="Starting step:\s+([^"]*)"`)
	completedRegex := regexp.MustCompile(`msg="Completed in\s+[^"]*"`)
	errorRegex := regexp.MustCompile(`level=error.*msg="([^"]*)"`)
	failedRegex := regexp.MustCompile(`msg="Execution failed:\s+([^"]*)"`)
	osErrorRegex := regexp.MustCompile(`(OS compatibility error|Ubuntu version not supported|This server is not supported)`)
	kubeconfigRegex := regexp.MustCompile(`(KUBECONFIG=.*|export KUBECONFIG=.*)`)
	clusterDisksRegex := regexp.MustCompile(`cluster_disks:\s*\[(.*)\]|CLUSTER_DISKS:\s*(.*)`)
	clusterPremountedDisksRegex := regexp.MustCompile(`cluster_premounted_disks:\s*(.*)|CLUSTER_PREMOUNTED_DISKS:\s*(.*)`)
	totalStepsRegex := regexp.MustCompile(`msg="Total steps to execute:\s*(\d+)"`)

	var currentStep *StepStatus
	inConfigSection := false

	for scanner.Scan() {
		line := scanner.Text()

		// Check if we're entering configuration values section
		if strings.Contains(line, "Configuration values:") {
			inConfigSection = true
			continue
		}

		// Check if we're starting a step (which ends config section)
		if strings.Contains(line, "Starting step:") {
			inConfigSection = false
		}

		// Parse configuration values when in config section
		if inConfigSection && strings.Contains(line, "level=info") {
			// Extract the message content
			msgRegex := regexp.MustCompile(`msg="([^"]*)"`)
			if matches := msgRegex.FindStringSubmatch(line); len(matches) > 1 {
				msg := matches[1]
				// Parse key:value pairs
				if colonIdx := strings.Index(msg, ":"); colonIdx > 0 {
					key := strings.TrimSpace(msg[:colonIdx])
					value := strings.TrimSpace(msg[colonIdx+1:])

					// Don't store messages that are clearly not config
					if !strings.Contains(key, "Starting") && !strings.Contains(key, "Completed") &&
					   !strings.Contains(key, "Execution") && !strings.Contains(key, "Error") {
						// Store in ConfigValues map
						status.ConfigValues[key] = value

						// Also store specific important values
						switch strings.ToLower(key) {
						case "domain":
							status.Domain = value
						case "first_node":
							status.FirstNode = value == "true"
						case "control_plane":
							status.ControlPlane = value == "true"
						case "gpu_node":
							status.GPUNode = value == "true"
						case "server_ip":
							status.ServerIP = value
						case "clusterforge_release":
							status.ClusterForge = value
						}
					}
				}
			}
		} else if inConfigSection && !strings.Contains(line, "level=info") {
			// Also end config section if we encounter non-info level logs
			inConfigSection = false
		}

		if matches := osErrorRegex.FindStringSubmatch(line); len(matches) > 0 {
			if status.OSError == "" {
				status.OSError = line
			}
		}

		if matches := stepRegex.FindStringSubmatch(line); len(matches) > 1 {
			if currentStep != nil {
				status.Steps = append(status.Steps, *currentStep)
			}
			currentStep = &StepStatus{
				Name:      matches[1],
				Status:    "running",
				Timestamp: parseTimestamp(line),
			}
		}

		if completedRegex.MatchString(line) && currentStep != nil {
			currentStep.Status = "completed"
		}

		if matches := errorRegex.FindStringSubmatch(line); len(matches) > 1 {
			errorMsg := matches[1]
			status.Errors = append(status.Errors, errorMsg)
			if currentStep != nil {
				currentStep.Status = "failed"
				currentStep.Error = errorMsg
			}
		}

		// Also check for "Execution failed" messages
		if matches := failedRegex.FindStringSubmatch(line); len(matches) > 1 {
			errorMsg := matches[1]
			status.Errors = append(status.Errors, errorMsg)
			if currentStep != nil && currentStep.Status != "failed" {
				currentStep.Status = "failed"
				if currentStep.Error == "" {
					currentStep.Error = errorMsg
				}
			}
		}

		if strings.Contains(line, "Step") && strings.Contains(line, "is skipped") {
			if currentStep != nil {
				currentStep.Status = "skipped"
			}
		}

		if matches := kubeconfigRegex.FindStringSubmatch(line); len(matches) > 0 {
			status.Kubeconfig = matches[0]
		}

		if matches := clusterDisksRegex.FindStringSubmatch(line); len(matches) > 0 {
			disksStr := matches[1]
			if disksStr == "" && len(matches) > 2 {
				disksStr = matches[2]
			}
			if disksStr != "" {
				status.ClusterDisks = parseDisks(disksStr)
			}
		}

		if matches := clusterPremountedDisksRegex.FindStringSubmatch(line); len(matches) > 1 {
			disksStr := matches[1]
			if disksStr == "" && len(matches) > 2 {
				disksStr = matches[2]
			}
			if disksStr != "" {
				status.ClusterPremountedDisks = parseDisks(disksStr)
			}
		}

		// Parse total steps
		if matches := totalStepsRegex.FindStringSubmatch(line); len(matches) > 1 {
			if totalSteps, err := strconv.Atoi(matches[1]); err == nil {
				status.TotalSteps = totalSteps
			}
		}

	}

	if currentStep != nil {
		status.Steps = append(status.Steps, *currentStep)
	}

	return status, nil
}

func parseTimestamp(line string) time.Time {
	timestampRegex := regexp.MustCompile(`time="([^"]*)"`)
	if matches := timestampRegex.FindStringSubmatch(line); len(matches) > 1 {
		if t, err := time.Parse(time.RFC3339, matches[1]); err == nil {
			return t
		}
	}
	return time.Now()
}

func parseDisks(disksStr string) []string {
	disksStr = strings.TrimSpace(disksStr)
	disksStr = strings.Trim(disksStr, "[]")

	if disksStr == "" {
		return []string{}
	}

	disks := strings.Split(disksStr, ",")
	for i := range disks {
		disks[i] = strings.TrimSpace(disks[i])
		disks[i] = strings.Trim(disks[i], "'\"")
	}
	return disks
}

func DisplayBloomStatus(status *BloomStatus) {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║               🌸 Cluster-Bloom Status Report 🌸              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Check if there are any failures
	hasFailures := false
	for _, step := range status.Steps {
		if step.Status == "failed" {
			hasFailures = true
			break
		}
	}

	// Show prominent failure warning if there are failures
	if hasFailures || len(status.Errors) > 0 {
		fmt.Println("❌ INSTALLATION FAILED - ERRORS DETECTED")
		fmt.Println("════════════════════════════════════════")
		fmt.Println()
	}

	if status.OSError != "" {
		fmt.Println("⚠️  OS COMPATIBILITY ERROR:")
		fmt.Printf("   %s\n", status.OSError)
		fmt.Println("   This server does not meet OS requirements.")
		fmt.Println("   Please use Ubuntu 20.04, 22.04, or 24.04.")
		fmt.Println()
	}

	fmt.Printf("📄 Log file: %s\n", status.LogFile)
	fmt.Printf("🕐 Last updated: %s\n", status.LastModified.Format(time.RFC3339))
	fmt.Println()

	// If there are errors, show full configuration that was used
	if hasFailures || len(status.Errors) > 0 {
		fmt.Println("⚙️  Configuration Used (at time of error):")
		fmt.Println("════════════════════════════════════════")

		// Show important configs first
		importantKeys := []string{"domain", "first_node", "control_plane", "gpu_node", "server_ip",
			"no_disks_for_cluster", "cluster_disks", "cluster_premounted_disks", "rocm_base_url", "disabled_steps", "enabled_steps"}

		for _, key := range importantKeys {
			if val, exists := status.ConfigValues[key]; exists && val != "" {
				// Skip redacted values unless they're important for debugging
				if key != "join_token" || val != "---redacted---" {
					fmt.Printf("   %s: %s\n", key, val)
				}
			}
		}

		// Show any other non-empty configs
		for key, val := range status.ConfigValues {
			isImportant := false
			for _, iKey := range importantKeys {
				if key == iKey {
					isImportant = true
					break
				}
			}
			if !isImportant && val != "" && val != "---redacted---" {
				fmt.Printf("   %s: %s\n", key, val)
			}
		}
		fmt.Println()
	} else {
		// Normal summary for successful runs
		fmt.Println("📋 Configuration Summary:")
		fmt.Println("════════════════════════════")
		if status.Domain != "" {
			fmt.Printf("🌐 Domain: %s\n", status.Domain)
		}
		fmt.Printf("🎯 First Node: %v\n", status.FirstNode)
		fmt.Printf("🎮 Control Plane: %v\n", status.ControlPlane)
		fmt.Printf("🖥️  GPU Node: %v\n", status.GPUNode)
		if status.ServerIP != "" {
			fmt.Printf("🖧  Server IP: %s\n", status.ServerIP)
		}
		if status.ClusterForge != "" && status.ClusterForge != "none" {
			fmt.Printf("🔧 ClusterForge: %s\n", truncateString(status.ClusterForge, 50))
		}
		fmt.Println()
	}

	if len(status.ClusterDisks) > 0 || len(status.ClusterPremountedDisks) > 0 {
		fmt.Println("💾 Disk Configuration:")
		fmt.Println("════════════════════════════")
		if len(status.ClusterDisks) > 0 {
			fmt.Printf("📀 Cluster Disks: %s\n", strings.Join(status.ClusterDisks, ", "))
		}
		if len(status.ClusterPremountedDisks) > 0 {
			fmt.Printf("🗄️  Cluster Premounted Disks: %s\n", strings.Join(status.ClusterPremountedDisks, ", "))
		}
		fmt.Println()
	}

	fmt.Println("📊 Step Execution Status:")
	fmt.Println("════════════════════════════")

	totalSteps := len(status.Steps)
	completedSteps := 0
	failedSteps := 0
	skippedSteps := 0
	runningSteps := 0

	for _, step := range status.Steps {
		var icon string
		switch step.Status {
		case "completed":
			icon = "✅"
			completedSteps++
		case "failed":
			icon = "❌"
			failedSteps++
		case "skipped":
			icon = "⏭️"
			skippedSteps++
		case "running":
			icon = "🔄"
			runningSteps++
		default:
			icon = "⏸️"
		}

		fmt.Printf("%s %s\n", icon, step.Name)
		if step.Error != "" {
			fmt.Printf("   └─ Error: %s\n", step.Error)
		}
	}

	fmt.Println()
	fmt.Println("📈 Summary:")
	// Show expected total if available, otherwise show what we found
	if status.TotalSteps > 0 {
		fmt.Printf("   Progress: %d of %d steps\n", len(status.Steps), status.TotalSteps)
		if len(status.Steps) < status.TotalSteps {
			fmt.Printf("   ⏸️  Stopped after %d steps\n", len(status.Steps))
		}
	} else {
		fmt.Printf("   Total Steps Found: %d\n", totalSteps)
	}
	fmt.Printf("   ✅ Completed: %d\n", completedSteps)
	fmt.Printf("   ❌ Failed: %d\n", failedSteps)
	fmt.Printf("   ⏭️  Skipped: %d\n", skippedSteps)
	fmt.Printf("   🔄 Running: %d\n", runningSteps)
	fmt.Println()

	if status.Kubeconfig != "" {
		fmt.Println("🎯 Kubeconfig:")
		fmt.Println("════════════════════════════")
		fmt.Printf("   %s\n", status.Kubeconfig)
		fmt.Println()
	}

	if len(status.Errors) > 0 {
		fmt.Println("⚠️  Errors Encountered:")
		fmt.Println("════════════════════════════")
		for i, err := range status.Errors {
			if i < 5 {
				fmt.Printf("   • %s\n", err)
			}
		}
		if len(status.Errors) > 5 {
			fmt.Printf("   ... and %d more errors\n", len(status.Errors)-5)
		}
		fmt.Println()
	}

	if failedSteps > 0 || runningSteps > 0 {
		fmt.Println("💡 Next Steps:")
		if failedSteps > 0 {
			fmt.Println("   • Review the errors above and fix any issues")
			fmt.Println("   • Re-run bloom with appropriate configuration")
		} else if runningSteps > 0 {
			fmt.Println("   • Installation is still in progress")
			fmt.Println("   • Check back later or monitor the log file")
		}
	} else if completedSteps == totalSteps && totalSteps > 0 {
		fmt.Println("🎉 Installation completed successfully!")
		fmt.Println("   • Restart your session to enable k9s")
		if status.FirstNode {
			fmt.Println("   • Run the command in additional_node_command.txt to add nodes")
		} else {
			fmt.Println("   • Run the content of longhorn_drive_setup.txt to mount drives")
		}
	}

	fmt.Println()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func CheckAndDisplayExistingStatus() bool {
	currentDir, err := os.Getwd()
	if err != nil {
		return false
	}

	logPath := filepath.Join(currentDir, "bloom.log")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return false
	}

	status, err := ParseBloomLog(logPath)
	if err != nil {
		fmt.Printf("Error parsing bloom.log: %v\n", err)
		return false
	}

	DisplayBloomStatus(status)
	return true
}