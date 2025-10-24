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
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/silogen/cluster-bloom/pkg/command"
	"github.com/silogen/cluster-bloom/pkg/fsops"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

//go:embed templates/*
var templateFS embed.FS

type ErrorType int

const (
	ErrorTypeConfig ErrorType = iota
	ErrorTypeOS
	ErrorTypeSystem
	ErrorTypeGeneral
)

type WebHandlerService struct {
	monitor           *WebMonitor
	configMode        bool
	config            map[string]interface{}
	lastError         string
	errorType         ErrorType
	configVersion     int
	prefilledConfig   map[string]interface{}
	oneShot           bool
	validationFailed  bool
	validationErrors  []string
	steps             []Step
	startInstallation func() error
}

func NewWebHandlerService(monitor *WebMonitor) *WebHandlerService {
	return &WebHandlerService{
		monitor:           monitor,
		configMode:        false,
		config:            make(map[string]interface{}),
		errorType:         ErrorTypeGeneral,
		configVersion:     0,
		prefilledConfig:   make(map[string]interface{}),
		oneShot:           false,
		steps:             nil,
		startInstallation: nil,
	}
}

func (h *WebHandlerService) SetInstallationHandler(steps []Step, startCallback func() error) {
	h.steps = steps
	h.startInstallation = startCallback
}

func NewWebHandlerServiceConfig() *WebHandlerService {
	return &WebHandlerService{
		monitor:         nil,
		configMode:      true,
		config:          make(map[string]interface{}),
		errorType:       ErrorTypeGeneral,
		configVersion:   0,
		prefilledConfig: make(map[string]interface{}),
		oneShot:         false,
	}
}

// SetPrefilledConfig sets the prefilled configuration from parsed log data
func (h *WebHandlerService) SetPrefilledConfig(configValues map[string]string) {
	h.prefilledConfig = make(map[string]interface{})
	for key, value := range configValues {
		lowerKey := strings.ToLower(strings.ReplaceAll(key, " ", "_"))
		// Handle boolean values
		if value == "true" || value == "false" {
			h.prefilledConfig[lowerKey] = value == "true"
		} else {
			h.prefilledConfig[lowerKey] = value
		}
	}
	log.Infof("Prefilled config set with %d values from parsed log", len(h.prefilledConfig))
}

func categorizeError(errorMsg string) ErrorType {
	errorMsg = strings.ToLower(errorMsg)

	// OS compatibility errors
	if strings.Contains(errorMsg, "ubuntu") && (strings.Contains(errorMsg, "version") || strings.Contains(errorMsg, "requires")) {
		return ErrorTypeOS
	}
	if strings.Contains(errorMsg, "os-release") || strings.Contains(errorMsg, "operating system") {
		return ErrorTypeOS
	}

	// System resource errors
	if strings.Contains(errorMsg, "memory") || strings.Contains(errorMsg, "cpu") || strings.Contains(errorMsg, "disk space") {
		return ErrorTypeSystem
	}

	// Configuration errors (could be fixed by reconfiguration)
	if strings.Contains(errorMsg, "config") || strings.Contains(errorMsg, "invalid") || strings.Contains(errorMsg, "required") {
		return ErrorTypeConfig
	}

	return ErrorTypeGeneral
}

func (h *WebHandlerService) AddRootDeviceToConfig() {
	rootDisk, err := getRootDiskCmd()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("error trying to get disk where root partition is: %v", err))
	} else {
		h.prefilledConfig["root_device"] = rootDisk
	}
}

func getRootDiskCmd() (string, error) {
	// Get the source device for root mount
	output, err := command.Output("GetRootDiskCmd.Findmnt", true, "findmnt", "-no", "SOURCE", "/")
	if err != nil {
		return "", err
	}

	device := strings.TrimSpace(string(output))

	// Get the parent disk using lsblk
	output, err = command.Output("GetRootDiskCmd.Lsblk", true, "lsblk", "-no", "PKNAME", device)
	if err != nil {
		// If no parent, the device itself is the disk
		return device, nil
	}

	parentDisk := strings.TrimSpace(string(output))
	if parentDisk == "" {
		return device, nil
	}

	return "/dev/" + parentDisk, nil
}

func (h *WebHandlerService) LoadConfigFromFile(configFile string, oneShot bool) {
	h.oneShot = oneShot

	// Read all viper settings and copy them to prefilledConfig
	for _, key := range viper.AllKeys() {
		value := viper.Get(key)
		h.prefilledConfig[key] = value
	}

	// In one-shot mode, also populate the config directly to bypass web UI
	if oneShot {
		h.config = make(map[string]interface{})

		// Convert viper keys to uppercase format expected by the rest of the system
		keyMapping := map[string]string{
			"domain":                   "DOMAIN",
			"server_ip":                "SERVER_IP",
			"join_token":               "JOIN_TOKEN",
			"first_node":               "FIRST_NODE",
			"gpu_node":                 "GPU_NODE",
			"control_plane":            "CONTROL_PLANE",
			"no_disks_for_cluster":     "NO_DISKS_FOR_CLUSTER",
			"cluster_disks":            "CLUSTER_DISKS",
			"cluster_premounted_disks": "CLUSTER_PREMOUNTED_DISKS",
			"use_cert_manager":         "USE_CERT_MANAGER",
			"cert_option":              "CERT_OPTION",
			"tls_cert":                 "TLS_CERT",
			"tls_key":                  "TLS_KEY",
			"oidc_url":                 "OIDC_URL",
			"clusterforge_release":     "CLUSTERFORGE_RELEASE",
			"disabled_steps":           "DISABLED_STEPS",
			"enabled_steps":            "ENABLED_STEPS",
		}

		// Copy configuration with proper key mapping
		for viperKey, value := range h.prefilledConfig {
			if upperKey, exists := keyMapping[viperKey]; exists {
				h.config[upperKey] = value
			} else {
				// Fallback: convert to uppercase
				h.config[strings.ToUpper(viperKey)] = value
			}
		}
	}

}

func (h *WebHandlerService) PrefilledConfigAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Debug logging
	log.Debugf("PrefilledConfigAPIHandler called - config has %d entries", len(h.prefilledConfig))
	if len(h.prefilledConfig) > 0 {
		for key, value := range h.prefilledConfig {
			log.Debugf("  %s: %v", key, value)
		}
	}

	response := map[string]interface{}{
		"config":       h.prefilledConfig,
		"oneShot":      h.oneShot,
		"hasPrefilled": len(h.prefilledConfig) > 0,
	}

	json.NewEncoder(w).Encode(response)
}

func (h *WebHandlerService) DashboardHandler(w http.ResponseWriter, r *http.Request) {
	if h.configMode {
		log.Debug("DashboardHandler: In config mode, redirecting to ConfigWizardHandler")
		h.ConfigWizardHandler(w, r)
		return
	}

	//tmpl, err := template.ParseFiles("templates/dashboard.html")
	tmpl, err := template.ParseFS(templateFS, "templates/dashboard.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// type pageData struct {
	// 	longhornPreviousDisks string
	// }
	// data := pageData{
	// 	longhornPreviousDisks: "/dev/sdd,/dev/sde",
	// }

	w.Header().Set("Content-Type", "text/html")

	// Execute template with data
	err = tmpl.Execute(w, "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//fmt.Fprint(w, tmpl)
}

func (h *WebHandlerService) LogsAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(h.monitor.GetLogs())
}

func (h *WebHandlerService) VariablesAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(h.monitor.GetVariables())
}

func (h *WebHandlerService) StepsAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(h.monitor.GetSteps())
}

func (h *WebHandlerService) ConfigWizardHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(templateFS, "templates/config-wizard.html")

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, longhornPreviousDisks, err := GetDisksFromSelectedConfig(h.prefilledConfig["cluster_disks"].(string))
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Error getting prior Longhorn previous format targets: %v", err))
	}

	_, longhornPreviousMountpoints, err := GetDisksFromLonghornConfig(h.prefilledConfig["cluster_premounted_disks"].(string))
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Error getting prior Longhorn mount points: %v", err))
	}

	log.Infof("ConfigWizardHandler: Previous Longhorn mountpoints: %v", longhornPreviousMountpoints)

	longhornPreviousDisksString := generateDisplayString(longhornPreviousDisks)
	longhornPreviousMountpointsString := generateDisplayString(longhornPreviousMountpoints)

	if strings.TrimSpace(longhornPreviousMountpointsString) == "" {
		longhornPreviousMountpointsString = "unused"
	}

	type pageData struct {
		LonghornPreviousDisks       string
		LonghornPreviousMountpoints string
	}

	data := pageData{
		LonghornPreviousDisks:       longhornPreviousDisksString,
		LonghornPreviousMountpoints: longhornPreviousMountpointsString,
	}

	w.Header().Set("Content-Type", "text/html")

	// Execute template with data
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func generateDisplayString(mountPoints map[string]string) string {
	displayString := ""
	for key, value := range mountPoints {
		if strings.TrimSpace(value) != "" {
			displayString += key + " => " + value + ", "
		}
	}
	// remove trailing comma
	displayString = strings.TrimSuffix(displayString, ", ")

	return displayString
}

func (h *WebHandlerService) ConfigAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var config map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid JSON: " + err.Error(),
		})
		return
	}

	yamlData, err := yaml.Marshal(config)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to generate YAML: " + err.Error(),
		})
		return
	}

	filename := "bloom.yaml"
	if err := fsops.WriteFile(filename, yamlData, 0644); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Failed to save configuration: " + err.Error(),
		})
		return
	}

	h.config = config
	h.configVersion++
	h.lastError = "" // Clear any previous errors

	// Start installation if callback is set (monitoring mode)
	if h.startInstallation != nil {
		go func() {
			log.Info("Starting installation process after configuration save...")
			if err := h.startInstallation(); err != nil {
				log.Errorf("Failed to start installation: %v", err)
			}
		}()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Configuration saved successfully",
		"file":    filename,
	})
}

func (h *WebHandlerService) MonitorHandler(w http.ResponseWriter, r *http.Request) {
	oldConfigMode := h.configMode
	h.configMode = false
	h.DashboardHandler(w, r)
	h.configMode = oldConfigMode
}

func (h *WebHandlerService) GetConfig() map[string]interface{} {
	if len(h.config) == 0 {
		return nil
	}
	return h.config
}

func (h *WebHandlerService) SetError(errorMsg string) {
	h.lastError = errorMsg
	h.errorType = categorizeError(errorMsg)

	// Only switch to config mode for configuration errors
	if h.errorType == ErrorTypeConfig {
		h.configMode = true
	}
	// For OS/System errors, stay in monitoring mode but show error
}

func (h *WebHandlerService) ConfigChanged() bool {
	return h.configVersion > 1 // First config is version 1, changes are version 2+
}

func (h *WebHandlerService) GetLastError() string {
	return h.lastError
}

// ReconfigureHandler archives the existing bloom.log and switches to config mode
func (h *WebHandlerService) ReconfigureHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Info("ReconfigureHandler: Starting reconfigure process")

	// Archive existing bloom.log
	currentDir, _ := os.Getwd()
	logPath := filepath.Join(currentDir, "bloom.log")

	// Only load configuration if we don't already have it
	// (it might have been loaded at startup in monitoring mode)
	if len(h.prefilledConfig) == 0 {
		// Try to load configuration from bloom.yaml first (if it exists)
		yamlPath := filepath.Join(currentDir, "bloom.yaml")
		if _, err := os.Stat(yamlPath); err == nil {
			// Read bloom.yaml
			yamlData, err := os.ReadFile(yamlPath)
			if err == nil {
				var yamlConfig map[string]interface{}
				if err := yaml.Unmarshal(yamlData, &yamlConfig); err == nil {
					h.prefilledConfig = yamlConfig
					log.Infof("ReconfigureHandler: Loaded %d config values from bloom.yaml", len(h.prefilledConfig))
				}
			}
		}
	} else {
		log.Infof("ReconfigureHandler: Using existing prefilled config with %d values", len(h.prefilledConfig))
	}

	// If we still don't have config, try parsing the log
	if len(h.prefilledConfig) == 0 {
		if _, err := os.Stat(logPath); err == nil {
			// Parse the log to get previous configuration
			if status, err := ParseBloomLog(logPath); err == nil {
				log.Infof("ReconfigureHandler: Parsed %d config values from bloom.log", len(status.ConfigValues))
				// Convert config values to prefilled config
				h.prefilledConfig = make(map[string]interface{})

				// Map the parsed values to the config keys used by the web interface
				// The JavaScript expects lowercase keys matching viper format
				for key, value := range status.ConfigValues {
					// Keep key as lowercase to match JavaScript expectations
					lowerKey := strings.ToLower(strings.ReplaceAll(key, " ", "_"))

					// Handle boolean values
					if value == "true" || value == "false" {
						h.prefilledConfig[lowerKey] = value == "true"
					} else {
						h.prefilledConfig[lowerKey] = value
					}
				}

				// Make sure we have the essential values (use lowercase keys)
				if status.Domain != "" {
					h.prefilledConfig["domain"] = status.Domain
				}
				h.prefilledConfig["first_node"] = status.FirstNode
				h.prefilledConfig["control_plane"] = status.ControlPlane
				h.prefilledConfig["gpu_node"] = status.GPUNode
				if status.ServerIP != "" {
					h.prefilledConfig["server_ip"] = status.ServerIP
				}

				log.Infof("Loaded previous configuration with %d values", len(h.prefilledConfig))
				// Log details for debugging
				for key, value := range h.prefilledConfig {
					log.Debugf("  prefilled[%s] = %v", key, value)
				}
			} else {
				log.Warnf("ReconfigureHandler: Failed to parse bloom.log: %v", err)
			}
		}
	}

	// Now archive the file if it exists
	if _, err := os.Stat(logPath); err == nil {
		timestamp := time.Now().Format("20060102-150405")
		archivedPath := filepath.Join(currentDir, fmt.Sprintf("bloom-%s.log", timestamp))

		if err := fsops.Rename(logPath, archivedPath); err != nil {
			log.Errorf("Failed to archive bloom.log: %v", err)
			http.Error(w, fmt.Sprintf("Failed to archive log: %v", err), http.StatusInternalServerError)
			return
		}

		log.Infof("Archived bloom.log to %s", filepath.Base(archivedPath))
	}

	// Switch to config mode
	h.configMode = true
	log.Info("ReconfigureHandler: Switched to config mode")

	// Send success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": "Log archived, ready to reconfigure",
	})
}

func (h *WebHandlerService) ErrorAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	errorTypeStr := "general"
	switch h.errorType {
	case ErrorTypeOS:
		errorTypeStr = "os"
	case ErrorTypeSystem:
		errorTypeStr = "system"
	case ErrorTypeConfig:
		errorTypeStr = "config"
	}

	json.NewEncoder(w).Encode(map[string]string{
		"error":     h.lastError,
		"errorType": errorTypeStr,
	})
}

func (h *WebHandlerService) ValidationErrorAPIHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	var requestData struct {
		Errors []string `json:"errors"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Log validation errors
	LogMessage(Error, "Validation failed in one-shot mode:")
	for _, err := range requestData.Errors {
		LogMessage(Error, fmt.Sprintf("  - %s", err))
	}

	// Signal for server shutdown
	h.validationFailed = true
	h.validationErrors = requestData.Errors

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "error_logged",
	})
}

func LocalhostOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if strings.Contains(host, ":") {
			host = strings.Split(host, ":")[0]
		}

		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			next.ServeHTTP(w, r)
		} else {
			http.Error(w, "Access denied - localhost only", http.StatusForbidden)
		}
	})
}
