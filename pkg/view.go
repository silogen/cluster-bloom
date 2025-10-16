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
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Step struct {
	Id          string
	Name        string
	Description string
	Action      func() StepResult
	Skip        func() bool
}

var (
	globalWebMonitor  *WebMonitor
	ContinueOnFailure bool = false
)

func SetGlobalWebMonitor(monitor *WebMonitor) {
	globalWebMonitor = monitor
}

func LogToUI(message string) {
	if globalWebMonitor != nil {
		globalWebMonitor.AddLog("INFO", message, "system")
	}
}

func RunStepsWithUI(steps []Step) error {
	var nonDisabledSteps []Step
	var enabledSteps []Step
	if viper.IsSet("DISABLED_STEPS") && viper.GetString("DISABLED_STEPS") != "" {
		disabledStepNames := strings.Split(viper.GetString("DISABLED_STEPS"), ",")
		for _, step := range steps {
			if !slices.Contains(disabledStepNames, step.Id) {
				nonDisabledSteps = append(nonDisabledSteps, step)
			}
		}
	} else {
		nonDisabledSteps = steps
	}
	if viper.IsSet("ENABLED_STEPS") && viper.GetString("ENABLED_STEPS") != "" {
		enabledStepNames := strings.Split(viper.GetString("ENABLED_STEPS"), ",")
		for _, step := range nonDisabledSteps {
			if slices.Contains(enabledStepNames, step.Id) {
				enabledSteps = append(enabledSteps, step)
			}
		}
	} else {
		enabledSteps = nonDisabledSteps
	}

	monitor := NewWebMonitor()
	globalWebMonitor = monitor

	for i, step := range enabledSteps {
		monitor.InitializeStep(step, i+1)
	}

	monitor.SetVariable("app_version", "2.0.0")
	monitor.SetVariable("startup_time", time.Now().Format(time.RFC3339))
	monitor.SetVariable("total_steps", len(enabledSteps))
	monitor.AddLog("INFO", "Cluster-Bloom installation started", "system")

	handlerService := NewWebHandlerService(monitor)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handlerService.DashboardHandler)
	mux.HandleFunc("/api/logs", handlerService.LogsAPIHandler)
	mux.HandleFunc("/api/variables", handlerService.VariablesAPIHandler)
	mux.HandleFunc("/api/steps", handlerService.StepsAPIHandler)
	mux.HandleFunc("/api/error", handlerService.ErrorAPIHandler)
	mux.HandleFunc("/api/config", handlerService.ConfigAPIHandler)
	mux.HandleFunc("/api/completion-info", handlerService.CompletionInfoAPIHandler)
	mux.HandleFunc("/configure", handlerService.ConfigWizardHandler)

	handler := LocalhostOnly(mux)
	port := ":62078"
	url := fmt.Sprintf("http://127.0.0.1%s", port)

	server := &http.Server{
		Addr:    "127.0.0.1" + port,
		Handler: handler,
	}

	serverErr := make(chan error, 1)
	go func() {
		fmt.Printf("🚀 Starting Cluster-Bloom web interface on %s\n", url)
		fmt.Println("📊 Dashboard accessible only from localhost")
		fmt.Printf("🌐 Monitor progress at %s\n", url)
		fmt.Println()
		fmt.Println("🔗 For remote access, create an SSH tunnel:")
		fmt.Printf("   ssh -L %s:127.0.0.1%s user@remote-server\n", port[1:], port)
		fmt.Printf("   Then access: http://127.0.0.1%s\n\n", port)

		serverErr <- server.ListenAndServe()
	}()

	done := make(chan bool)
	var finalErr error

	go watchLogFile(monitor)

	go func() {
		defer func() { done <- true }()

		for _, step := range enabledSteps {
			monitor.StartStep(step.Id)
			monitor.AddLog("INFO", fmt.Sprintf("Starting step: %s", step.Name), step.Id)

			startTime := time.Now()

			result := StepResult{Error: nil, Message: ""}
			if step.Skip != nil && step.Skip() {
				monitor.AddLog("INFO", fmt.Sprintf("Step %s is skipped", step.Name), step.Id)
				monitor.SkipStep(step.Id)
			} else {
				result = step.Action()
			}

			duration := time.Since(startTime)

			if result.Error != nil {
				finalErr = result.Error
				monitor.AddLog("ERROR", fmt.Sprintf("Error: %v", result.Error), step.Id)
				monitor.CompleteStep(step.Id, result.Error)
				break
			} else {
				if result.Message != "" {
					monitor.AddLog("INFO", fmt.Sprintf("Message: %s", result.Message), step.Id)
				}
				monitor.AddLog("INFO", fmt.Sprintf("Completed in %v", duration.Round(time.Millisecond)), step.Id)
				monitor.CompleteStep(step.Id, nil)
			}

			time.Sleep(500 * time.Millisecond)
		}

		if finalErr != nil {
			monitor.AddLog("ERROR", fmt.Sprintf("Execution failed: %v", finalErr), "system")
		} else {
			monitor.AddLog("INFO", "All steps completed successfully!", "system")
		}

		if finalErr == nil {
			fmt.Printf("\n🎉 Installation completed!\n")
		} else {
			fmt.Printf("\n❌ Installation failed!\n")
		}
		fmt.Printf("📊 View detailed results at %s\n", url)
		fmt.Println("\nPress Ctrl+C to stop the web server and exit...")
	}()

	select {
	case <-done:
	case err := <-serverErr:
		if err != http.ErrServerClosed {
			return fmt.Errorf("web server error: %v", err)
		}
	}

	fmt.Println("\n=== Cluster Bloom Execution Summary ===")
	fmt.Println()

	for _, step := range enabledSteps {
		stepStatus := monitor.GetSteps()[step.Id]
		status := "[ ]"
		if stepStatus != nil {
			switch stepStatus.Status {
			case "completed":
				status = "[✓]"
			case "failed":
				status = "[✗]"
			case "skipped":
				status = "[~]"
			case "running":
				status = "[→]"
			}
		}
		fmt.Printf("%s %s\n", status, step.Name)
	}

	fmt.Println()
	if viper.GetBool("FIRST_NODE") {
		fmt.Println("To setup additional nodes to join the cluster, run the command in additional_node_command.txt")
	} else {
		fmt.Println("The content of longhorn_drive_setup.txt must be run in order to mount drives properly. " +
			"This can be done in the control node, which was installed first, or with a valid kubeconfig for the cluster.")
	}
	fmt.Println()
	if finalErr != nil {
		fmt.Printf("Execution failed: %v\n", finalErr)
	} else {
		fmt.Println("Execution completed. Restart your session to enable k9s.")
		fmt.Println("All steps completed successfully!")
	}

	fmt.Printf("\n📊 Web interface still available at %s\n", url)
	fmt.Println("Press Ctrl+C to stop the web server and exit...")

	select {
	case err := <-serverErr:
		if err != http.ErrServerClosed {
			return fmt.Errorf("web server error: %v", err)
		}
	}

	if finalErr != nil {
		return finalErr
	}
	return nil
}

func RunWebInterfaceWithConfig(port string, steps []Step, configFile string, oneShot bool, setupLogging func(), logConfig func()) error {
	handlerService := NewWebHandlerServiceConfig()

	// If config file provided, pre-fill the configuration
	if configFile != "" {
		handlerService.LoadConfigFromFile(configFile, oneShot)
	}
	handlerService.AddRootDeviceToConfig()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handlerService.DashboardHandler)
	mux.HandleFunc("/api/config", handlerService.ConfigAPIHandler)
	mux.HandleFunc("/api/error", handlerService.ErrorAPIHandler)
	mux.HandleFunc("/api/validation-error", handlerService.ValidationErrorAPIHandler)
	mux.HandleFunc("/monitor", handlerService.MonitorHandler)
	mux.HandleFunc("/api/prefilled-config", handlerService.PrefilledConfigAPIHandler)

	handler := LocalhostOnly(mux)
	server := &http.Server{
		Addr:    "127.0.0.1" + port,
		Handler: handler,
	}

	configReceived := make(chan bool)
	serverErr := make(chan error, 1)

	go func() {
		serverErr <- server.ListenAndServe()
	}()

	go func() {
		for {
			time.Sleep(1 * time.Second)
			if handlerService.GetConfig() != nil {
				configReceived <- true
				break
			}
		}
	}()

	// Check for validation failures in one-shot mode
	validationFailed := make(chan bool)
	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			if handlerService.validationFailed {
				validationFailed <- true
				break
			}
		}
	}()

	for {
		select {
		case <-configReceived:
			fmt.Println("✅ Configuration received from web interface")
			fmt.Println("🔄 Starting installation...")
			fmt.Println()

			// Setup logging now that we're about to start installation
			if setupLogging != nil {
				setupLogging()
			}

			// Log the configuration values
			if logConfig != nil {
				logConfig()
			}

			// Switch to monitoring mode but keep same server
			handlerService.configMode = false
			monitor := NewWebMonitor()
			handlerService.monitor = monitor
			globalWebMonitor = monitor

			// Add monitoring endpoints to the same server
			mux.HandleFunc("/api/logs", handlerService.LogsAPIHandler)
			mux.HandleFunc("/api/variables", handlerService.VariablesAPIHandler)
			mux.HandleFunc("/api/steps", handlerService.StepsAPIHandler)
			mux.HandleFunc("/api/completion-info", handlerService.CompletionInfoAPIHandler)

			// Run installation
			installErr := runStepsInBackground(steps, monitor)

			if installErr != nil {
				errorType := categorizeError(installErr.Error())

				// In one-shot mode, exit immediately on installation failure
				if oneShot {
					switch errorType {
					case ErrorTypeOS:
						fmt.Printf("❌ Installation failed: %v\n", installErr)
						fmt.Println("⚠️  This server is not supported due to OS compatibility issues")
						fmt.Println("📋 Please use a supported Ubuntu version (20.04, 22.04, or 24.04)")
					case ErrorTypeSystem:
						fmt.Printf("❌ Installation failed: %v\n", installErr)
						fmt.Println("⚠️  This server does not meet minimum system requirements")
						fmt.Println("📋 Please upgrade hardware or use a different server")
					default:
						fmt.Printf("❌ Installation failed: %v\n", installErr)
					}

					server.Close()
					return fmt.Errorf("installation failed: %v", installErr)
				}

				// Interactive mode: show error and wait for reconfiguration
				switch errorType {
				case ErrorTypeOS:
					fmt.Printf("❌ Installation failed: %v\n", installErr)
					fmt.Println("⚠️  This server is not supported due to OS compatibility issues")
					fmt.Println("📋 Please use a supported Ubuntu version (20.04, 22.04, or 24.04)")
					fmt.Println("🌐 Configuration interface available at /configure (reconfiguration cannot fix OS issues)")
				case ErrorTypeSystem:
					fmt.Printf("❌ Installation failed: %v\n", installErr)
					fmt.Println("⚠️  This server does not meet minimum system requirements")
					fmt.Println("📋 Please upgrade hardware or use a different server")
					fmt.Println("🌐 Configuration interface available at /configure")
				default:
					fmt.Printf("❌ Installation failed: %v\n", installErr)
					fmt.Println("🔄 Web interface available for reconfiguration at /configure")
				}
				handlerService.SetError(installErr.Error())

				// Wait for new configuration
				go func() {
					for {
						time.Sleep(1 * time.Second)
						if handlerService.ConfigChanged() {
							configReceived <- true
							break
						}
					}
				}()
			} else {
				fmt.Println("✅ Installation completed successfully!")

				// In one-shot mode, exit after successful installation
				if oneShot {
					server.Close()
					return nil
				}

				fmt.Println("📊 Web interface will remain available for monitoring")
				// Keep server running for monitoring
			}

		case <-validationFailed:
			// Validation failed in one-shot mode - shut down server and exit
			fmt.Printf("\n❌ Validation failed in one-shot mode:\n")
			for _, err := range handlerService.validationErrors {
				fmt.Printf("   - %s\n", err)
			}
			fmt.Printf("\n💡 Please fix the configuration errors and try again.\n")

			server.Close()
			return fmt.Errorf("configuration validation failed")

		case err := <-serverErr:
			if err != http.ErrServerClosed {
				return fmt.Errorf("web server error: %v", err)
			}
			return nil
		}
	}
}

func runStepsInBackground(steps []Step, monitor *WebMonitor) error {
	enabledSteps := CalculateEnabledSteps(steps)

	for i, step := range enabledSteps {
		monitor.InitializeStep(step, i+1)
	}

	monitor.SetVariable("total_steps", len(enabledSteps))
	// Log the total number of steps for parsing later
	LogMessage(Info, fmt.Sprintf("Total steps to execute: %d", len(enabledSteps)))
	monitor.AddLog("INFO", "Installation started", "system")

	go watchLogFile(monitor)

	var finalErr error

	for _, step := range enabledSteps {
		monitor.StartStep(step.Id)
		monitor.AddLog("INFO", fmt.Sprintf("Starting step: %s", step.Name), step.Id)
		// Also log to file
		LogMessage(Info, fmt.Sprintf("Starting step: %s", step.Name))

		startTime := time.Now()

		result := StepResult{Error: nil, Message: ""}
		if step.Skip != nil && step.Skip() {
			monitor.AddLog("INFO", fmt.Sprintf("Step %s is skipped", step.Name), step.Id)
			monitor.SkipStep(step.Id)
			// Also log to file
			LogMessage(Info, fmt.Sprintf("Step %s is skipped", step.Name))
		} else {
			result = step.Action()
		}

		duration := time.Since(startTime)

		if result.Error != nil {
			finalErr = result.Error
			monitor.AddLog("ERROR", fmt.Sprintf("Error: %v", result.Error), step.Id)
			monitor.CompleteStep(step.Id, result.Error)
			// Also log to file
			LogMessage(Error, fmt.Sprintf("Execution failed: %v", result.Error))
			break
		} else {
			if result.Message != "" {
				monitor.AddLog("INFO", fmt.Sprintf("Message: %s", result.Message), step.Id)
				// Also log to file
				LogMessage(Info, result.Message)
			}
			monitor.AddLog("INFO", fmt.Sprintf("Completed in %v", duration.Round(time.Millisecond)), step.Id)
			monitor.CompleteStep(step.Id, nil)
			// Also log to file
			LogMessage(Info, fmt.Sprintf("Completed in %v", duration.Round(time.Millisecond)))
		}

		time.Sleep(500 * time.Millisecond)
	}

	if finalErr != nil {
		errorType := categorizeError(finalErr.Error())
		switch errorType {
		case ErrorTypeOS:
			monitor.AddLog("ERROR", fmt.Sprintf("OS compatibility error: %v", finalErr), "system")
			monitor.AddLog("ERROR", "This server is not supported - OS version incompatible", "system")
		case ErrorTypeSystem:
			monitor.AddLog("ERROR", fmt.Sprintf("System requirements error: %v", finalErr), "system")
			monitor.AddLog("ERROR", "Server does not meet minimum system requirements", "system")
		default:
			monitor.AddLog("ERROR", fmt.Sprintf("Installation failed: %v", finalErr), "system")
		}
	} else {
		monitor.AddLog("INFO", "All steps completed successfully!", "system")
	}

	return finalErr
}

// CalculateEnabledSteps filters the provided steps based on DISABLED_STEPS and ENABLED_STEPS configuration
func CalculateEnabledSteps(steps []Step) []Step {
	var nonDisabledSteps []Step
	var enabledSteps []Step
	if viper.IsSet("DISABLED_STEPS") && viper.GetString("DISABLED_STEPS") != "" {
		disabledStepNames := strings.Split(viper.GetString("DISABLED_STEPS"), ",")
		for _, step := range steps {
			if !slices.Contains(disabledStepNames, step.Id) {
				nonDisabledSteps = append(nonDisabledSteps, step)
			}
		}
	} else {
		nonDisabledSteps = steps
	}
	if viper.IsSet("ENABLED_STEPS") && viper.GetString("ENABLED_STEPS") != "" {
		enabledStepNames := strings.Split(viper.GetString("ENABLED_STEPS"), ",")
		for _, step := range nonDisabledSteps {
			if slices.Contains(enabledStepNames, step.Id) {
				enabledSteps = append(enabledSteps, step)
			}
		}
	} else {
		enabledSteps = nonDisabledSteps
	}
	return enabledSteps
}

func RunStepsWithCLI(steps []Step) error {
	enabledSteps := CalculateEnabledSteps(steps)

	fmt.Printf("🚀 Starting installation with %d steps\n", len(enabledSteps))
	fmt.Println("════════════════════════════════════════")
	fmt.Println()

	// Log the total number of steps for parsing later
	LogMessage(Info, fmt.Sprintf("Total steps to execute: %d", len(enabledSteps)))

	var finalErr error

	for i, step := range enabledSteps {
		fmt.Printf("[%d/%d] %s\n", i+1, len(enabledSteps), step.Name)
		fmt.Printf("      %s\n", step.Description)
		// Log to file
		LogMessage(Info, fmt.Sprintf("Starting step: %s", step.Name))

		startTime := time.Now()

		result := StepResult{Error: nil, Message: ""}
		if step.Skip != nil && step.Skip() {
			fmt.Printf("      ⏭️  SKIPPED\n")
			// Log to file
			LogMessage(Info, fmt.Sprintf("Step %s is skipped", step.Name))
		} else {
			result = step.Action()
		}

		duration := time.Since(startTime)

		if result.Error != nil {
			finalErr = result.Error
			fmt.Printf("      ❌ FAILED: %v\n", result.Error)
			// Log to file
			LogMessage(Error, fmt.Sprintf("Execution failed: %v", result.Error))
			break
		} else {
			if result.Message != "" {
				fmt.Printf("      💬 %s\n", result.Message)
				// Log to file
				LogMessage(Info, result.Message)
			}
			fmt.Printf("      ✅ COMPLETED in %v\n", duration.Round(time.Millisecond))
			// Log to file
			LogMessage(Info, fmt.Sprintf("Completed in %v", duration.Round(time.Millisecond)))
		}
		fmt.Println()

		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("════════════════════════════════════════")
	fmt.Println("🏁 Installation Summary")
	fmt.Println("════════════════════════════════════════")

	for i, step := range enabledSteps {
		status := "[ ]"
		if i < len(enabledSteps) {
			if finalErr != nil && enabledSteps[i].Id == step.Id {
				status = "[❌]"
			} else if finalErr == nil || i < len(enabledSteps)-1 {
				status = "[✅]"
			}
		}
		fmt.Printf("%s %s\n", status, step.Name)
	}

	fmt.Println()
	if viper.GetBool("FIRST_NODE") {
		fmt.Println("📝 To setup additional nodes to join the cluster, run the command in additional_node_command.txt")
	} else {
		fmt.Println("📝 The content of longhorn_drive_setup.txt must be run in order to mount drives properly.")
		fmt.Println("   This can be done in the control node, which was installed first, or with a valid kubeconfig for the cluster.")
	}
	fmt.Println()
	if finalErr != nil {
		fmt.Printf("❌ Execution failed: %v\n", finalErr)
		return finalErr
	} else {
		fmt.Println("✅ Execution completed. Restart your session to enable k9s.")
		fmt.Println("🎉 All steps completed successfully!")
	}

	return nil
}

func WatchLogFile(monitor *WebMonitor) {
	watchLogFile(monitor)
}

func watchLogFile(monitor *WebMonitor) {
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Could not determine current directory: %v\n", err)
		return
	}
	logPath := filepath.Join(currentDir, "bloom.log")
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			time.Sleep(500 * time.Millisecond)
		} else {
			break
		}
	}
	file, err := os.Open(logPath)
	if err != nil {
		fmt.Printf("Could not open log file for reading: %v\n", err)
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		fmt.Printf("Could not stat log file: %v\n", err)
		return
	}
	currentSize := stat.Size()
	file.Seek(currentSize, 0)
	scanner := bufio.NewScanner(file)
	for {
		stat, err := file.Stat()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		if offset, err := file.Seek(0, 1); err == nil && stat.Size() < offset {
			file.Seek(0, 0)
			scanner = bufio.NewScanner(file)
		}
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "level=") {
				parts := strings.Split(line, " ")
				level := "INFO"
				for _, part := range parts {
					if strings.HasPrefix(part, "level=") {
						level = strings.ToUpper(strings.TrimPrefix(part, "level="))
						break
					}
				}
				monitor.AddLog(level, line, "file-watcher")
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func LogMessage(level LogLevel, message string) {
	var levelStr string
	switch level {
	case Debug:
		log.Debug(message)
		levelStr = "DEBUG"
	case Info:
		log.Info(message)
		levelStr = "INFO"
	case Warn:
		log.Warn(message)
		levelStr = "WARN"
	case Error:
		log.Error(message)
		levelStr = "ERROR"
	default:
		log.Info(message)
		levelStr = "INFO"
	}
	if globalWebMonitor != nil {
		globalWebMonitor.AddLog(levelStr, message, "system")
	}
}

func LogCommand(commandName string, output string) {
	header := "Command output from " + commandName + ":"
	log.Info(header)
	log.Info(output)
	if globalWebMonitor != nil {
		globalWebMonitor.AddLog("INFO", header, "command")
		globalWebMonitor.AddLog("INFO", output, "command")
	}
}

func SetVariable(name string, value interface{}) {
	viper.Set(name, value)
	if globalWebMonitor != nil {
		globalWebMonitor.SetVariable(name, value)
	}
}

type LogLevel int

const (
	Debug LogLevel = iota
	Info
	Warn
	Error
)

type StepResult struct {
	Error   error
	Message string
}
type OptionResult struct {
	Selected []string
	Indexes  []int
	Canceled bool
}

func ShowOptionsScreen(title string, message string, options []string, preSelected []string) (OptionResult, error) {
	if globalWebMonitor == nil {
		return OptionResult{Canceled: true}, errors.New("web monitor not initialized")
	}

	globalWebMonitor.AddLog("INFO", fmt.Sprintf("User selection required: %s", title), "user-input")
	globalWebMonitor.AddLog("INFO", fmt.Sprintf("Options: %s", strings.Join(options, ", ")), "user-input")

	fmt.Printf("\n=== %s ===\n", title)
	if message != "" {
		fmt.Printf("%s\n\n", message)
	}

	fmt.Println("Available options:")
	for i, option := range options {
		isPreSelected := false
		for _, preSelect := range preSelected {
			if option == preSelect {
				isPreSelected = true
				break
			}
		}
		marker := " "
		if isPreSelected {
			marker = "✓"
		}
		fmt.Printf("  %d) [%s] %s\n", i+1, marker, option)
	}

	fmt.Printf("\nEnter your selections (comma-separated numbers, e.g., 1,3,5) or press Enter for pre-selected: ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return OptionResult{Canceled: true}, errors.New("user canceled the selection")
	}

	input := strings.TrimSpace(scanner.Text())

	var selectedItems []string
	var selectedIndexes []int

	if input == "" {
		selectedItems = preSelected
		for i, option := range options {
			for _, preSelect := range preSelected {
				if option == preSelect {
					selectedIndexes = append(selectedIndexes, i)
					break
				}
			}
		}
	} else {
		parts := strings.Split(input, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if index := parseSelection(part); index >= 0 && index < len(options) {
				selectedItems = append(selectedItems, options[index])
				selectedIndexes = append(selectedIndexes, index)
			}
		}
	}

	globalWebMonitor.AddLog("INFO", fmt.Sprintf("User selected: %s", strings.Join(selectedItems, ", ")), "user-input")

	return OptionResult{
		Selected: selectedItems,
		Indexes:  selectedIndexes,
		Canceled: false,
	}, nil
}

func parseSelection(s string) int {
	if i, err := fmt.Sscanf(s, "%d", new(int)); err == nil && i == 1 {
		var num int
		fmt.Sscanf(s, "%d", &num)
		return num - 1
	}
	return -1
}
