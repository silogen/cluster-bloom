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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

type ErrorType int

const (
	ErrorTypeConfig ErrorType = iota
	ErrorTypeOS
	ErrorTypeSystem
	ErrorTypeGeneral
)

type WebHandlerService struct {
	monitor          *WebMonitor
	configMode       bool
	config           map[string]interface{}
	lastError        string
	errorType        ErrorType
	configVersion    int
	prefilledConfig  map[string]interface{}
	oneShot          bool
	validationFailed bool
	validationErrors []string
	steps            []Step
	startInstallation func() error
}

func NewWebHandlerService(monitor *WebMonitor) *WebHandlerService {
	return &WebHandlerService{
		monitor:         monitor,
		configMode:      false,
		config:          make(map[string]interface{}),
		errorType:       ErrorTypeGeneral,
		configVersion:   0,
		prefilledConfig: make(map[string]interface{}),
		oneShot:         false,
		steps:           nil,
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
			"domain":               "DOMAIN",
			"server_ip":            "SERVER_IP",
			"join_token":           "JOIN_TOKEN",
			"first_node":           "FIRST_NODE",
			"gpu_node":             "GPU_NODE",
			"control_plane":        "CONTROL_PLANE",
			"skip_disk_check":      "SKIP_DISK_CHECK",
			"selected_disks":       "SELECTED_DISKS",
			"longhorn_disks":       "LONGHORN_DISKS",
			"use_cert_manager":     "USE_CERT_MANAGER",
			"cert_option":          "CERT_OPTION",
			"tls_cert":             "TLS_CERT",
			"tls_key":              "TLS_KEY",
			"oidc_url":             "OIDC_URL",
			"clusterforge_release": "CLUSTERFORGE_RELEASE",
			"disabled_steps":       "DISABLED_STEPS",
			"enabled_steps":        "ENABLED_STEPS",
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
	log.Infof("PrefilledConfigAPIHandler called - config has %d entries", len(h.prefilledConfig))
	if len(h.prefilledConfig) > 0 {
		for key, value := range h.prefilledConfig {
			log.Debugf("  %s: %v", key, value)
		}
	}

	response := map[string]interface{}{
		"config":   h.prefilledConfig,
		"oneShot":  h.oneShot,
		"hasPrefilled": len(h.prefilledConfig) > 0,
	}

	json.NewEncoder(w).Encode(response)
}

func (h *WebHandlerService) DashboardHandler(w http.ResponseWriter, r *http.Request) {
	if h.configMode {
		log.Info("DashboardHandler: In config mode, redirecting to ConfigWizardHandler")
		h.ConfigWizardHandler(w, r)
		return
	}

	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Cluster-Bloom Installation Monitor</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container { max-width: 1400px; margin: 0 auto; }
        .header {
            background: white;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .tabs {
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 20px;
        }
        .tab-nav {
            display: flex;
            border-bottom: 2px solid #f0f0f0;
            background: #fafafa;
            border-radius: 8px 8px 0 0;
        }
        .tab-button {
            padding: 15px 25px;
            background: none;
            border: none;
            cursor: pointer;
            font-size: 15px;
            color: #666;
            transition: all 0.3s ease;
            position: relative;
            font-weight: 600;
        }
        .tab-button:hover {
            background: rgba(76,175,80,0.05);
        }
        .tab-button.active {
            color: #4CAF50;
        }
        .tab-button.active::after {
            content: '';
            position: absolute;
            bottom: -2px;
            left: 0;
            right: 0;
            height: 3px;
            background: #4CAF50;
        }
        .tab-content {
            padding: 20px;
        }
        .tab-panel {
            display: none;
        }
        .tab-panel.active {
            display: block;
        }
        .section {
            background: white;
            margin-bottom: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .section-header {
            background: #4CAF50;
            color: white;
            padding: 15px 20px;
            border-radius: 8px 8px 0 0;
            font-weight: bold;
        }
        .section-content { padding: 20px; }
        .step-entry {
            padding: 15px;
            border-bottom: 1px solid #eee;
            display: flex;
            align-items: center;
            justify-content: space-between;
        }
        .step-entry:last-child { border-bottom: none; }
        .step-main {
            flex: 1;
        }
        .step-name {
            font-weight: bold;
            margin-bottom: 5px;
        }
        .step-description {
            color: #666;
            font-size: 14px;
        }
        .step-info {
            text-align: right;
            min-width: 200px;
        }
        .log-entry, .variable-entry {
            padding: 10px;
            border-bottom: 1px solid #eee;
            font-family: 'Courier New', monospace;
            font-size: 14px;
        }
        .log-entry:last-child, .variable-entry:last-child {
            border-bottom: none;
        }
        .log-level {
            display: inline-block;
            padding: 2px 8px;
            border-radius: 4px;
            font-size: 12px;
            font-weight: bold;
            margin-right: 10px;
        }
        .log-level.INFO { background: #e3f2fd; color: #1976d2; }
        .log-level.ERROR { background: #ffebee; color: #d32f2f; }
        .log-level.WARN { background: #fff3e0; color: #f57c00; }
        .log-level.DEBUG { background: #f3e5f5; color: #7b1fa2; }
        .timestamp { color: #666; margin-right: 10px; }
        .step-name-log { color: #4CAF50; font-weight: bold; margin-right: 10px; }
        .variable-name { color: #2196F3; font-weight: bold; }
        .variable-type { color: #666; font-style: italic; margin-left: 10px; }
        .status {
            display: inline-block;
            padding: 4px 12px;
            border-radius: 4px;
            font-size: 12px;
            font-weight: bold;
            text-transform: uppercase;
        }
        .status.completed { background: #e8f5e8; color: #2e7d32; }
        .status.failed { background: #ffebee; color: #d32f2f; }
        .status.running { background: #fff3e0; color: #f57c00; }
        .status.pending { background: #f5f5f5; color: #666; }
        .status.skipped { background: #e0e0e0; color: #424242; }
        .progress-bar {
            width: 100%;
            background-color: #e0e0e0;
            border-radius: 4px;
            overflow: hidden;
            margin-top: 10px;
        }
        .progress-fill {
            height: 8px;
            background-color: #4CAF50;
            transition: width 0.3s ease;
        }
        .progress-text {
            text-align: center;
            margin-top: 5px;
            font-size: 14px;
            color: #666;
        }
        .error-msg { color: #d32f2f; margin-top: 5px; font-size: 12px; }
        .duration { color: #666; font-size: 12px; }
        .controls { margin-bottom: 20px; }
        .btn {
            background: #4CAF50;
            color: white;
            border: none;
            padding: 10px 20px;
            border-radius: 4px;
            cursor: pointer;
            margin-right: 10px;
            margin-bottom: 10px;
        }
        .btn:hover { background: #45a049; }
        .btn:disabled { background: #ccc; cursor: not-allowed; }
        .overview {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 20px;
            margin-bottom: 20px;
        }
        .overview-card {
            background: white;
            padding: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            text-align: center;
        }
        .overview-number {
            font-size: 2em;
            font-weight: bold;
            color: #4CAF50;
        }
        .overview-label {
            color: #666;
            margin-top: 5px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🚀 Cluster-Bloom Installation Monitor</h1>
            <p>Real-time monitoring of Kubernetes cluster installation and configuration</p>
            <div class="controls">
                <button class="btn" onclick="refreshData()">Refresh</button>
                <button class="btn btn-secondary" id="reconfigure-btn" onclick="handleReconfigure()">Reconfigure</button>
            </div>
            <div id="os-error-banner" style="display: none; background: #fff3e0; border: 2px solid #f57c00; border-radius: 4px; padding: 15px; margin-top: 15px;">
                <h3 style="color: #f57c00; margin: 0 0 10px 0;">⚠️ Unsupported Operating System</h3>
                <p id="os-error-message" style="margin: 0; color: #666;"></p>
                <p style="margin: 10px 0 0 0; color: #666; font-style: italic;">This server cannot run Cluster-Bloom. Please use a supported Ubuntu version (20.04, 22.04, or 24.04).</p>
            </div>
        </div>

        <div class="overview" id="overview"></div>

        <div class="tabs">
            <div class="tab-nav">
                <button class="tab-button active" onclick="switchTab('steps-tab')">📋 Installation Steps</button>
                <button class="tab-button" onclick="switchTab('variables-tab')">🔧 Variables</button>
                <button class="tab-button" onclick="switchTab('logs-tab')">📊 Recent Logs</button>
            </div>
            <div class="tab-content">
                <div id="steps-tab" class="tab-panel active">
                    <div class="progress-bar">
                        <div class="progress-fill" id="overall-progress"></div>
                    </div>
                    <div class="progress-text" id="progress-text"></div>
                    <div id="steps"></div>
                </div>
                <div id="variables-tab" class="tab-panel">
                    <div id="variables"></div>
                </div>
                <div id="logs-tab" class="tab-panel">
                    <div id="logs"></div>
                </div>
            </div>
        </div>
    </div>

    <script>
        let lastRefresh = 0;

        function switchTab(tabId) {
            // Remove active class from all tabs and panels
            document.querySelectorAll('.tab-button').forEach(btn => {
                btn.classList.remove('active');
            });
            document.querySelectorAll('.tab-panel').forEach(panel => {
                panel.classList.remove('active');
            });

            // Add active class to clicked tab and corresponding panel
            event.target.classList.add('active');
            document.getElementById(tabId).classList.add('active');
        }

        function handleReconfigure() {
            if (!confirm('This will archive the current bloom.log and restart the configuration process. Continue?')) {
                return;
            }

            fetch('/api/reconfigure', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                }
            })
            .then(response => response.json())
            .then(data => {
                if (data.status === 'success') {
                    // Redirect to configuration page
                    window.location.href = '/';
                } else {
                    alert('Failed to reconfigure: ' + (data.message || 'Unknown error'));
                }
            })
            .catch(error => {
                console.error('Reconfigure error:', error);
                alert('Failed to reconfigure: ' + error.message);
            });
        }

        function refreshData() {
            const now = Date.now();
            if (now - lastRefresh < 500) return; // Throttle requests
            lastRefresh = now;

            Promise.all([
                fetch('/api/logs').then(r => r.json()),
                fetch('/api/variables').then(r => r.json()),
                fetch('/api/steps').then(r => r.json())
            ]).then(([logs, variables, steps]) => {
                updateOverview(steps);
                updateSteps(steps);
                updateVariables(variables);
                updateLogs(logs);
            }).catch(err => {
                console.error('Failed to refresh data:', err);
            });
        }

        function updateOverview(steps) {
            const stepsArray = Object.values(steps);
            const totalSteps = stepsArray.length;
            const completedSteps = stepsArray.filter(s => s.status === 'completed').length;
            const failedSteps = stepsArray.filter(s => s.status === 'failed').length;
            const runningSteps = stepsArray.filter(s => s.status === 'running').length;

            const container = document.getElementById('overview');
            container.innerHTML = ` + "`" + `
                <div class="overview-card">
                    <div class="overview-number">${totalSteps}</div>
                    <div class="overview-label">Total Steps</div>
                </div>
                <div class="overview-card">
                    <div class="overview-number" style="color: #4CAF50">${completedSteps}</div>
                    <div class="overview-label">Completed</div>
                </div>
                <div class="overview-card">
                    <div class="overview-number" style="color: #f57c00">${runningSteps}</div>
                    <div class="overview-label">Running</div>
                </div>
                <div class="overview-card">
                    <div class="overview-number" style="color: #d32f2f">${failedSteps}</div>
                    <div class="overview-label">Failed</div>
                </div>
            ` + "`" + `;
        }

        function updateSteps(steps) {
            const stepsArray = Object.values(steps).sort((a, b) => a.progress - b.progress);
            const totalSteps = stepsArray.length;
            const completedSteps = stepsArray.filter(s => s.status === 'completed' || s.status === 'skipped').length;
            const progress = totalSteps > 0 ? (completedSteps / totalSteps) * 100 : 0;

            // Update progress bar
            document.getElementById('overall-progress').style.width = progress + '%';
            document.getElementById('progress-text').textContent = ` + "`" + `${completedSteps} of ${totalSteps} steps completed (${Math.round(progress)}%)` + "`" + `;

            const container = document.getElementById('steps');
            container.innerHTML = stepsArray.map(step => {
                const duration = step.duration || '';
                const errorMsg = step.error ? ` + "`" + `<div class="error-msg">Error: ${step.error}</div>` + "`" + ` : '';
                return ` + "`" + `<div class="step-entry">
                    <div class="step-main">
                        <div class="step-name">${step.name}</div>
                        <div class="step-description">${step.description}</div>
                        ${errorMsg}
                    </div>
                    <div class="step-info">
                        <span class="status ${step.status}">${step.status}</span>
                        ${duration ? ` + "`" + `<div class="duration">${duration}</div>` + "`" + ` : ''}
                    </div>
                </div>` + "`" + `;
            }).join('');
        }

        function updateLogs(logs) {
            const container = document.getElementById('logs');
            container.innerHTML = logs.slice(0, 50).map(log => {
                const timestamp = new Date(log.timestamp).toLocaleTimeString();
                return ` + "`" + `<div class="log-entry">
                    <span class="timestamp">${timestamp}</span>
                    <span class="log-level ${log.level}">${log.level}</span>
                    <span class="step-name-log">[${log.step}]</span>
                    ${log.message}
                </div>` + "`" + `;
            }).join('');
        }

        function updateVariables(variables) {
            const container = document.getElementById('variables');
            container.innerHTML = Object.values(variables).map(variable => {
                return ` + "`" + `<div class="variable-entry">
                    <span class="variable-name">${variable.name}</span>
                    <span class="variable-type">(${variable.type})</span>
                    = ${JSON.stringify(variable.value)}
                </div>` + "`" + `;
            }).join('');
        }

        // Check for OS errors and adjust UI accordingly
        function checkForOSErrors() {
            fetch('/api/error')
                .then(response => response.json())
                .then(data => {
                    if (data.error && data.errorType === 'os') {
                        document.getElementById('os-error-message').textContent = data.error;
                        document.getElementById('os-error-banner').style.display = 'block';

                        // Update reconfigure button behavior for OS errors
                        const reconfigBtn = document.getElementById('reconfigure-btn');
                        reconfigBtn.style.background = '#666';
                        reconfigBtn.textContent = 'View Configuration';
                        reconfigBtn.onclick = function() {
                            alert('Configuration cannot resolve OS compatibility issues. Please use a supported Ubuntu version (20.04, 22.04, or 24.04).');
                            window.location.href = '/';
                        };
                    }
                })
                .catch(err => {
                    console.log('No OS error to display');
                });
        }

        // Auto-refresh every 2 seconds
        setInterval(refreshData, 2000);
        refreshData();
        checkForOSErrors();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
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
	html := `
<!DOCTYPE html>
<html>
<head>
    <title>Cluster-Bloom Configuration Wizard</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container { max-width: 800px; margin: 0 auto; }
        .header {
            background: white;
            padding: 30px;
            border-radius: 8px;
            margin-bottom: 30px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            text-align: center;
        }
        .config-section {
            background: white;
            margin-bottom: 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            overflow: hidden;
        }
        .section-header {
            background: #4CAF50;
            color: white;
            padding: 15px 20px;
            font-weight: bold;
            font-size: 18px;
        }
        .section-content { padding: 25px; }
        .form-group {
            margin-bottom: 25px;
        }
        .form-group label {
            display: block;
            font-weight: bold;
            margin-bottom: 8px;
            color: #333;
        }
        .form-group .description {
            font-size: 14px;
            color: #666;
            margin-bottom: 10px;
            line-height: 1.4;
        }
        .form-group input, .form-group select {
            width: 100%;
            padding: 12px;
            border: 2px solid #ddd;
            border-radius: 4px;
            font-size: 16px;
            transition: border-color 0.3s;
        }
        .form-group input:focus, .form-group select:focus {
            outline: none;
            border-color: #4CAF50;
        }
        .checkbox-group {
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .checkbox-group input[type="checkbox"] {
            width: auto;
            transform: scale(1.2);
        }
        .btn {
            background: #4CAF50;
            color: white;
            border: none;
            padding: 15px 30px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 16px;
            font-weight: bold;
            transition: background-color 0.3s;
        }
        .btn:hover { background: #45a049; }
        .btn:disabled { background: #ccc; cursor: not-allowed; }
        .btn-secondary {
            background: #666;
            margin-right: 10px;
        }
        .btn-secondary:hover { background: #555; }
        .actions {
            text-align: center;
            padding: 30px;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .conditional {
            display: none;
            opacity: 0.6;
        }
        .conditional.show {
            display: block;
            opacity: 1;
        }
        .progress-bar {
            width: 100%;
            background-color: #e0e0e0;
            border-radius: 4px;
            overflow: hidden;
            margin-bottom: 20px;
        }
        .progress-fill {
            height: 8px;
            background-color: #4CAF50;
            transition: width 0.3s ease;
        }
        .error { color: #d32f2f; margin-top: 5px; font-size: 14px; }
        .success { color: #2e7d32; margin-top: 5px; font-size: 14px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🚀 Cluster-Bloom Configuration Wizard</h1>
            <p>Configure your Kubernetes cluster installation step by step</p>
            <div class="progress-bar">
                <div class="progress-fill" id="progress-fill" style="width: 10%"></div>
            </div>
            <div id="error-banner" style="display: none; border-radius: 4px; padding: 15px; margin-top: 15px;">
                <h3 id="error-title" style="margin: 0 0 10px 0;"></h3>
                <p id="error-message" style="margin: 0; color: #666;"></p>
                <p id="error-suggestion" style="margin: 10px 0 0 0; color: #666; font-style: italic;"></p>
            </div>
        </div>

        <form id="config-form" novalidate>
            <!-- Basic Configuration -->
            <div class="config-section">
                <div class="section-header">📋 Basic Configuration</div>
                <div class="section-content">
                    <div class="form-group">
                        <label for="FIRST_NODE">Node Type</label>
                        <div class="description">Is this the first node in the cluster? Select 'No' for additional nodes joining an existing cluster.</div>
                        <div class="checkbox-group">
                            <input type="checkbox" id="FIRST_NODE" name="FIRST_NODE" checked onchange="updateConditionals()">
                            <label for="FIRST_NODE">This is the first node in the cluster</label>
                        </div>
                    </div>

                    <div class="form-group">
                        <label for="GPU_NODE">GPU Support</label>
                        <div class="description">Does this node have GPUs? When enabled, ROCm will be installed and configured.</div>
                        <div class="checkbox-group">
                            <input type="checkbox" id="GPU_NODE" name="GPU_NODE" checked>
                            <label for="GPU_NODE">This node has GPUs (enable ROCm)</label>
                        </div>
                    </div>

                    <div class="form-group">
                        <label for="DOMAIN">Domain Name *</label>
                        <div class="description">Domain name for the cluster (e.g., cluster.example.com). Used for ingress configuration.</div>
                        <input type="text" id="DOMAIN" name="DOMAIN" placeholder="cluster.example.com"
                               pattern="[a-z0-9]([a-z0-9\-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9\-]*[a-z0-9])?)*"
                               title="Valid domain format: example.com or subdomain.example.com" required>
                        <small class="help-text" style="color: #666; font-size: 12px; margin-top: 4px; display: block;">
                            ✓ Format: domain.com or sub.domain.com<br>
                            ✓ Must start with alphanumeric, can contain hyphens<br>
                            ✗ No special characters except dots and hyphens
                        </small>
                    </div>
                </div>
            </div>

            <!-- Additional Node Configuration -->
            <div class="config-section conditional" id="additional-node-section">
                <div class="section-header">🔗 Additional Node Configuration</div>
                <div class="section-content">
                    <div class="form-group">
                        <label for="CONTROL_PLANE">Control Plane</label>
                        <div class="description">Should this node be a control plane node?</div>
                        <div class="checkbox-group">
                            <input type="checkbox" id="CONTROL_PLANE" name="CONTROL_PLANE">
                            <label for="CONTROL_PLANE">Make this a control plane node</label>
                        </div>
                    </div>

                    <div class="form-group">
                        <label for="SERVER_IP">Server IP Address *</label>
                        <div class="description">IP address of the RKE2 server (first node).</div>
                        <input type="text" id="SERVER_IP" name="SERVER_IP" placeholder="192.168.1.100"
                               pattern="^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$"
                               title="Valid IPv4 address format: xxx.xxx.xxx.xxx (0-255 for each octet)">
                        <small class="help-text" style="color: #666; font-size: 12px; margin-top: 4px; display: block;">
                            ✓ Format: IPv4 address (e.g., 192.168.1.100)<br>
                            ✓ Each octet: 0-255<br>
                            ✗ No IPv6 addresses yet
                        </small>
                    </div>

                    <div class="form-group">
                        <label for="JOIN_TOKEN">Join Token *</label>
                        <div class="description">Token used to join additional nodes to the cluster.</div>
                        <input type="text" id="JOIN_TOKEN" name="JOIN_TOKEN" placeholder="K10xyz...">
                    </div>
                </div>
            </div>

            <!-- Storage Configuration -->
            <div class="config-section">
                <div class="section-header">💾 Storage Configuration</div>
                <div class="section-content">
                    <div class="form-group">
                        <label for="SKIP_DISK_CHECK">Skip Disk Operations</label>
                        <div class="description">Skip disk-related operations if you don't want automatic disk setup.</div>
                        <div class="checkbox-group">
                            <input type="checkbox" id="SKIP_DISK_CHECK" name="SKIP_DISK_CHECK">
                            <label for="SKIP_DISK_CHECK">Skip disk operations</label>
                        </div>
                    </div>

                    <div class="form-group">
                        <label for="SELECTED_DISKS">Selected Disks</label>
                        <div class="description">Comma-separated list of specific disk devices to use (e.g., /dev/sdb,/dev/sdc). Leave empty for automatic selection.</div>
                        <input type="text" id="SELECTED_DISKS" name="SELECTED_DISKS" placeholder="/dev/sdb,/dev/sdc"
                               pattern="^(/dev/[a-zA-Z0-9]+)(,/dev/[a-zA-Z0-9]+)*$|^$"
                               title="Format: /dev/xxx or comma-separated /dev/xxx,/dev/yyy">
                        <small class="help-text" style="color: #666; font-size: 12px; margin-top: 4px; display: block;">
                            ✓ Format: /dev/sdb or /dev/sdb,/dev/sdc<br>
                            ✓ Must be valid block device paths<br>
                            ✓ Leave empty for auto-detection<br>
                            ✗ Ensure devices exist and are not in use
                        </small>
                    </div>

                    <div class="form-group">
                        <label for="LONGHORN_DISKS">Longhorn Disks</label>
                        <div class="description">Comma-separated list of disk paths for Longhorn storage. Leave empty for automatic configuration.</div>
                        <input type="text" id="LONGHORN_DISKS" name="LONGHORN_DISKS" placeholder="/dev/sdb,/dev/sdc"
                               pattern="^(/dev/[a-zA-Z0-9]+)(,/dev/[a-zA-Z0-9]+)*$|^$"
                               title="Format: /dev/xxx or comma-separated /dev/xxx,/dev/yyy">
                        <small class="help-text" style="color: #666; font-size: 12px; margin-top: 4px; display: block;">
                            ✓ Format: /dev/sdb or /dev/sdb,/dev/sdc<br>
                            ✓ Must be existing block devices<br>
                            ✓ Will be formatted for Longhorn storage<br>
                            ⚠️ Data on specified disks will be erased
                        </small>
                    </div>
                </div>
            </div>

            <!-- SSL/TLS Configuration -->
            <div class="config-section">
                <div class="section-header">🔒 SSL/TLS Configuration</div>
                <div class="section-content">
                    <div class="form-group">
                        <label for="USE_CERT_MANAGER">Certificate Management</label>
                        <div class="description">Use cert-manager with Let's Encrypt for automatic TLS certificates.</div>
                        <div class="checkbox-group">
                            <input type="checkbox" id="USE_CERT_MANAGER" name="USE_CERT_MANAGER" onchange="updateConditionals()">
                            <label for="USE_CERT_MANAGER">Use cert-manager with Let's Encrypt</label>
                        </div>
                    </div>

                    <div class="form-group conditional" id="cert-option-section">
                        <label for="CERT_OPTION">Certificate Option *</label>
                        <div class="description">Choose how to handle TLS certificates when not using cert-manager.</div>
                        <select id="CERT_OPTION" name="CERT_OPTION" onchange="updateConditionals()">
                            <option value="">Choose an option...</option>
                            <option value="existing">Use existing certificate files</option>
                            <option value="generate">Generate self-signed certificates</option>
                        </select>
                    </div>

                    <div class="form-group conditional" id="tls-cert-section">
                        <label for="TLS_CERT">TLS Certificate Path *</label>
                        <div class="description">Path to TLS certificate file for ingress (PEM format).</div>
                        <input type="text" id="TLS_CERT" name="TLS_CERT" placeholder="/path/to/cert.pem"
                               pattern="^(/[^/]+)+\.(pem|crt|cert)$|^$"
                               title="Must be an absolute path to a .pem, .crt, or .cert file">
                        <small class="help-text" style="color: #666; font-size: 12px; margin-top: 4px; display: block;">
                            ✓ Format: Absolute path (starts with /)<br>
                            ✓ File extensions: .pem, .crt, .cert<br>
                            ✓ Must be readable by the user
                        </small>
                    </div>

                    <div class="form-group conditional" id="tls-key-section">
                        <label for="TLS_KEY">TLS Private Key Path *</label>
                        <div class="description">Path to TLS private key file for ingress (PEM format).</div>
                        <input type="text" id="TLS_KEY" name="TLS_KEY" placeholder="/path/to/key.pem"
                               pattern="^(/[^/]+)+\.(pem|key)$|^$"
                               title="Must be an absolute path to a .pem or .key file">
                        <small class="help-text" style="color: #666; font-size: 12px; margin-top: 4px; display: block;">
                            ✓ Format: Absolute path (starts with /)<br>
                            ✓ File extensions: .pem, .key<br>
                            ✓ Must be readable and should be protected (600 permissions)
                        </small>
                    </div>
                </div>
            </div>

            <!-- Advanced Configuration -->
            <div class="config-section">
                <div class="section-header">⚙️ Advanced Configuration</div>
                <div class="section-content">
                    <div class="form-group">
                        <label for="OIDC_URL">OIDC Provider URL</label>
                        <div class="description">URL of the OIDC provider for authentication. Leave empty to skip OIDC configuration.</div>
                        <input type="url" id="OIDC_URL" name="OIDC_URL" placeholder="https://your-oidc-provider.com"
                               pattern="^https?://.*$|^$"
                               title="Must be a valid HTTP or HTTPS URL">
                        <small class="help-text" style="color: #666; font-size: 12px; margin-top: 4px; display: block;">
                            ✓ Format: https://provider.com or http://provider.local<br>
                            ✓ Must be accessible from the cluster<br>
                            ✓ Leave empty to skip OIDC setup
                        </small>
                    </div>

                    <div class="form-group">
                        <label for="CLUSTERFORGE_RELEASE">ClusterForge Release</label>
                        <div class="description">ClusterForge release URL or 'none' to skip installation.</div>
                        <input type="text" id="CLUSTERFORGE_RELEASE" name="CLUSTERFORGE_RELEASE" value="https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz">
                    </div>

                    <div class="form-group">
                        <label for="DISABLED_STEPS">Disabled Steps</label>
                        <div class="description">Comma-separated list of steps to skip (e.g., SetupLonghornStep,SetupMetallbStep).</div>
                        <input type="text" id="DISABLED_STEPS" name="DISABLED_STEPS" placeholder="SetupLonghornStep,SetupMetallbStep">
                    </div>

                    <div class="form-group">
                        <label for="ENABLED_STEPS">Enabled Steps</label>
                        <div class="description">Comma-separated list of steps to run. If specified, only these steps will run.</div>
                        <input type="text" id="ENABLED_STEPS" name="ENABLED_STEPS" placeholder="SetupRKE2Step,SetupLonghornStep">
                    </div>
                </div>
            </div>

            <div class="actions">
                <button type="button" class="btn btn-secondary" onclick="resetForm()">Reset</button>
                <button type="submit" class="btn">Generate Configuration & Start Installation</button>
            </div>
        </form>

        <div id="result" style="display: none; margin-top: 20px; padding: 20px; background: white; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);"></div>
    </div>

    <script>
        function updateConditionals() {
            const firstNode = document.getElementById('FIRST_NODE').checked;
            const useCertManager = document.getElementById('USE_CERT_MANAGER').checked;
            const certOption = document.getElementById('CERT_OPTION').value;

            // Show/hide additional node section
            const additionalNodeSection = document.getElementById('additional-node-section');
            if (firstNode) {
                additionalNodeSection.classList.remove('show');
            } else {
                additionalNodeSection.classList.add('show');
            }

            // Show/hide certificate options
            const certOptionSection = document.getElementById('cert-option-section');
            const tlsCertSection = document.getElementById('tls-cert-section');
            const tlsKeySection = document.getElementById('tls-key-section');
            const tlsCertInput = document.getElementById('TLS_CERT');
            const tlsKeyInput = document.getElementById('TLS_KEY');

            if (useCertManager) {
                certOptionSection.classList.remove('show');
                tlsCertSection.classList.remove('show');
                tlsKeySection.classList.remove('show');
                // Remove pattern validation from hidden certificate fields
                if (tlsCertInput) {
                    tlsCertInput.removeAttribute('pattern');
                    tlsCertInput.removeAttribute('required');
                    if (!window.prefilledData?.tls_cert) {
                        tlsCertInput.value = '';
                    }
                }
                if (tlsKeyInput) {
                    tlsKeyInput.removeAttribute('pattern');
                    tlsKeyInput.removeAttribute('required');
                    if (!window.prefilledData?.tls_key) {
                        tlsKeyInput.value = '';
                    }
                }
            } else {
                certOptionSection.classList.add('show');
                if (certOption === 'existing') {
                    tlsCertSection.classList.add('show');
                    tlsKeySection.classList.add('show');
                    // Restore pattern validation for visible certificate fields
                    if (tlsCertInput) {
                        tlsCertInput.setAttribute('pattern', '^(/[^/]+)+\\.(pem|crt|cert)$|^$');
                        if (window.prefilledData?.tls_cert) {
                            tlsCertInput.value = window.prefilledData.tls_cert;
                        }
                    }
                    if (tlsKeyInput) {
                        tlsKeyInput.setAttribute('pattern', '^(/[^/]+)+\\.(pem|key)$|^$');
                        if (window.prefilledData?.tls_key) {
                            tlsKeyInput.value = window.prefilledData.tls_key;
                        }
                    }
                } else {
                    tlsCertSection.classList.remove('show');
                    tlsKeySection.classList.remove('show');
                    // Remove pattern validation from hidden certificate fields
                    if (tlsCertInput) {
                        tlsCertInput.removeAttribute('pattern');
                        tlsCertInput.removeAttribute('required');
                        if (!window.prefilledData?.tls_cert) {
                            tlsCertInput.value = '';
                        }
                    }
                    if (tlsKeyInput) {
                        tlsKeyInput.removeAttribute('pattern');
                        tlsKeyInput.removeAttribute('required');
                        if (!window.prefilledData?.tls_key) {
                            tlsKeyInput.value = '';
                        }
                    }
                }
            }

            updateProgress();
        }

        function updateProgress() {
            const formData = new FormData(document.getElementById('config-form'));
            const inputs = document.querySelectorAll('#config-form input, #config-form select');
            let filled = 0;
            let total = 0;

            inputs.forEach(input => {
                if (input.type === 'checkbox') {
                    total++;
                    if (input.checked) filled++;
                } else {
                    const parent = input.closest('.form-group');
                    if (!parent.classList.contains('conditional') || parent.classList.contains('show')) {
                        total++;
                        if (input.value.trim()) filled++;
                    }
                }
            });

            const progress = Math.min(100, (filled / Math.max(total, 1)) * 100);
            document.getElementById('progress-fill').style.width = progress + '%';
        }

        function resetForm() {
            document.getElementById('config-form').reset();
            document.getElementById('FIRST_NODE').checked = true;
            document.getElementById('GPU_NODE').checked = true;
            document.getElementById('CLUSTERFORGE_RELEASE').value = 'https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz';
            updateConditionals();
        }

        document.getElementById('config-form').addEventListener('submit', function(e) {
            e.preventDefault();

            // First check if the form is valid according to HTML5 validation
            const form = e.target;
            console.log('Form validity check:', form.checkValidity());

            // If form is invalid, show which fields are invalid
            if (!form.checkValidity()) {
                console.error('Form has HTML5 validation errors');

                // Find all invalid fields
                const allInputs = form.querySelectorAll('input, select, textarea');
                let invalidFields = [];

                allInputs.forEach(input => {
                    if (!input.validity.valid) {
                        const parent = input.closest('.form-group');
                        const isVisible = parent && window.getComputedStyle(parent).display !== 'none';

                        // Set custom validation message
                        let message = '';
                        if (input.validity.patternMismatch) {
                            const helpText = input.nextElementSibling ? input.nextElementSibling.textContent : '';
                            message = 'Invalid format. ' + helpText;
                        } else if (input.validity.valueMissing) {
                            message = 'This field is required';
                        } else if (input.validity.typeMismatch) {
                            message = 'Please enter a valid value';
                        }

                        input.setCustomValidity(message);

                        invalidFields.push({
                            name: input.name || input.id,
                            value: input.value,
                            pattern: input.getAttribute('pattern'),
                            required: input.hasAttribute('required'),
                            visible: isVisible,
                            validity: {
                                patternMismatch: input.validity.patternMismatch,
                                valueMissing: input.validity.valueMissing,
                                typeMismatch: input.validity.typeMismatch
                            },
                            message: message
                        });
                    } else {
                        // Clear any custom validation message for valid fields
                        input.setCustomValidity('');
                    }
                });

                console.error('Invalid fields:', invalidFields);

                // Show the validation messages
                form.reportValidity();
                return;
            }

            const formData = new FormData(this);
            const config = {};

            // First, handle all checkboxes (including unchecked ones)
            const checkboxes = document.querySelectorAll('input[type="checkbox"]');
            checkboxes.forEach(checkbox => {
                config[checkbox.name] = checkbox.checked;
            });

            // Then handle other form fields
            for (let [key, value] of formData.entries()) {
                const input = document.querySelector(` + "`" + `[name="${key}"]` + "`" + `);
                if (input.type !== 'checkbox' && value.trim()) {
                    config[key] = value.trim();
                }
            }

            // Validate required fields
            const firstNode = config.FIRST_NODE !== false;
            const useCertManager = config.USE_CERT_MANAGER === true;
            const certOption = config.CERT_OPTION;

            let errors = [];

            // Log all form fields for debugging
            console.log('=== Form Validation Debug ===');
            console.log('Configuration:', config);

            // Function to check and log field validation
            function validateField(fieldId, fieldName) {
                const element = document.getElementById(fieldId);
                if (element) {
                    const value = element.value;
                    const pattern = element.getAttribute('pattern');
                    const isValid = element.validity.valid;
                    const visibility = window.getComputedStyle(element.parentElement).display;
                    console.log(fieldName + ':', {
                        value: value,
                        pattern: pattern,
                        valid: isValid,
                        visible: visibility !== 'none',
                        validationMessage: element.validationMessage,
                        validity: element.validity
                    });
                    return isValid;
                }
                console.log(fieldName + ': Element not found');
                return true;
            }

            // Check all form fields for validation issues
            function checkAllFormFields() {
                console.log('=== Checking ALL Form Fields ===');
                const allInputs = document.querySelectorAll('input[pattern]');
                allInputs.forEach(input => {
                    if (!input.validity.valid) {
                        const parent = input.closest('.form-group');
                        const isVisible = parent && window.getComputedStyle(parent).display !== 'none';
                        console.error('INVALID FIELD:', {
                            id: input.id,
                            name: input.name,
                            value: input.value,
                            pattern: input.getAttribute('pattern'),
                            visible: isVisible,
                            validationMessage: input.validationMessage,
                            parentClass: parent?.className
                        });
                    }
                });
                console.log('=== End Form Field Check ===');
            }

            // Enhanced validation with detailed messages and logging
            if (!config.DOMAIN) {
                errors.push('❌ Domain is required - Please enter a valid domain name (e.g., cluster.example.com)');
            } else if (!validateField('DOMAIN', 'DOMAIN')) {
                errors.push('❌ Invalid domain format - Must be a valid domain like example.com or sub.example.com');
            }

            if (!firstNode) {
                if (!config.SERVER_IP) {
                    errors.push('❌ Server IP is required - Please enter the IP address of the first node (e.g., 192.168.1.100)');
                } else if (!validateField('SERVER_IP', 'SERVER_IP')) {
                    errors.push('❌ Invalid IP address format - Must be a valid IPv4 address (e.g., 192.168.1.100)');
                }
                if (!config.JOIN_TOKEN) {
                    errors.push('❌ Join Token is required - Please enter the token from the first node installation');
                }
            }

            if (!useCertManager && !certOption) {
                errors.push('❌ Certificate option is required - Please select how to handle TLS certificates when not using cert-manager');
            }

            if (certOption === 'existing') {
                if (!config.TLS_CERT) {
                    errors.push('❌ TLS Certificate path is required - Please provide the path to your certificate file (e.g., /path/to/cert.pem)');
                } else if (!validateField('TLS_CERT', 'TLS_CERT')) {
                    errors.push('❌ Invalid certificate path - Must be an absolute path to a .pem, .crt, or .cert file');
                }
                if (!config.TLS_KEY) {
                    errors.push('❌ TLS Private Key path is required - Please provide the path to your key file (e.g., /path/to/key.pem)');
                } else if (!validateField('TLS_KEY', 'TLS_KEY')) {
                    errors.push('❌ Invalid key path - Must be an absolute path to a .pem or .key file');
                }
            }

            // Validate disk paths if provided
            if (config.SELECTED_DISKS) {
                validateField('SELECTED_DISKS', 'SELECTED_DISKS');
                if (!document.getElementById('SELECTED_DISKS').validity.valid) {
                    errors.push('❌ Invalid disk format - Must be /dev/xxx or comma-separated list like /dev/sdb,/dev/sdc');
                }
            }
            if (config.LONGHORN_DISKS) {
                validateField('LONGHORN_DISKS', 'LONGHORN_DISKS');
                if (!document.getElementById('LONGHORN_DISKS').validity.valid) {
                    errors.push('❌ Invalid Longhorn disk format - Must be /dev/xxx or comma-separated list. Note: specified disks must exist on the system');
                }
            }

            // Validate OIDC URL if provided
            if (config.OIDC_URL) {
                validateField('OIDC_URL', 'OIDC_URL');
                if (!document.getElementById('OIDC_URL').validity.valid) {
                    errors.push('❌ Invalid OIDC URL - Must be a valid HTTP or HTTPS URL');
                }
            }

            console.log('Validation errors:', errors);
            console.log('=== End Validation Debug ===');

            // Check all form fields for any HTML5 validation issues
            checkAllFormFields();

            // Double-check form validity right before submission
            const finalValidityCheck = document.getElementById('config-form').checkValidity();
            console.log('Final form validity check before submission:', finalValidityCheck);

            if (!finalValidityCheck) {
                console.error('Form became invalid between validation and submission!');
                // Find the invalid field
                const allInputs = document.querySelectorAll('input, select, textarea');
                allInputs.forEach(input => {
                    if (!input.validity.valid) {
                        console.error('INVALID FIELD FOUND:', {
                            name: input.name,
                            id: input.id,
                            value: input.value,
                            pattern: input.getAttribute('pattern'),
                            required: input.hasAttribute('required'),
                            validity: input.validity,
                            validationMessage: input.validationMessage
                        });
                    }
                });
                return;
            }

            if (errors.length > 0) {
                // Check if we're in one-shot mode
                if (window.isOneShot) {
                    // In one-shot mode, send error to server for automated handling
                    fetch('/api/validation-error', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ errors: errors })
                    });
                    return;
                } else {
                    // Show detailed error message with suggestions
                    const errorDiv = document.createElement('div');
                    errorDiv.innerHTML = '<h3 style="color: #d32f2f;">❌ Validation Failed</h3>' +
                        '<p style="color: #666;">Please fix the following issues:</p>' +
                        '<ul style="list-style: none; padding: 0;">' +
                        errors.map(e => '<li style="margin: 8px 0;">' + e + '</li>').join('') +
                        '</ul>' +
                        '<p style="color: #666; font-style: italic; margin-top: 15px;">💡 Tip: Hover over fields to see format requirements</p>';

                    // Create a better modal instead of alert
                    const modal = document.createElement('div');
                    modal.style.cssText = 'position: fixed; top: 50%; left: 50%; transform: translate(-50%, -50%); ' +
                        'background: white; padding: 25px; border-radius: 8px; box-shadow: 0 4px 20px rgba(0,0,0,0.3); ' +
                        'z-index: 10000; max-width: 600px; max-height: 80vh; overflow-y: auto;';
                    modal.appendChild(errorDiv);

                    const closeBtn = document.createElement('button');
                    closeBtn.textContent = 'OK';
                    closeBtn.style.cssText = 'background: #4CAF50; color: white; border: none; padding: 10px 20px; ' +
                        'border-radius: 4px; cursor: pointer; margin-top: 15px; font-size: 16px;';
                    closeBtn.onclick = () => {
                        document.body.removeChild(modal);
                        document.body.removeChild(overlay);
                        // Focus first invalid field
                        const firstError = errors[0];
                        if (firstError.includes('Domain')) document.getElementById('DOMAIN').focus();
                        else if (firstError.includes('Server IP')) document.getElementById('SERVER_IP').focus();
                        else if (firstError.includes('Join Token')) document.getElementById('JOIN_TOKEN').focus();
                        else if (firstError.includes('Certificate path')) document.getElementById('TLS_CERT').focus();
                        else if (firstError.includes('Key path')) document.getElementById('TLS_KEY').focus();
                    };
                    modal.appendChild(closeBtn);

                    const overlay = document.createElement('div');
                    overlay.style.cssText = 'position: fixed; top: 0; left: 0; right: 0; bottom: 0; ' +
                        'background: rgba(0,0,0,0.5); z-index: 9999;';

                    document.body.appendChild(overlay);
                    document.body.appendChild(modal);
                    return;
                }
            }

            // Submit configuration
            console.log('About to submit configuration via fetch...');
            console.log('Config object:', config);

            try {
                console.log('Creating fetch request...');
                const fetchPromise = fetch('/api/config', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(config)
                });

                console.log('Fetch request created, waiting for response...');
                fetchPromise
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    document.getElementById('result').innerHTML = ` + "`" + `
                        <div class="success">
                            <h3>✅ Configuration Saved Successfully!</h3>
                            <p>Your configuration has been saved. The installation will begin shortly...</p>
                            <p><strong>Redirecting to installation monitor...</strong></p>
                        </div>
                    ` + "`" + `;
                    document.getElementById('result').style.display = 'block';

                    // Redirect to monitor after 3 seconds
                    setTimeout(() => {
                        window.location.href = '/monitor';
                    }, 3000);
                } else {
                    document.getElementById('result').innerHTML = ` + "`" + `
                        <div class="error">
                            <h3>❌ Configuration Error</h3>
                            <p>${data.error}</p>
                        </div>
                    ` + "`" + `;
                    document.getElementById('result').style.display = 'block';
                }
            })
            .catch(error => {
                console.error('Submission error details:', {
                    message: error.message,
                    stack: error.stack,
                    type: error.name,
                    error: error
                });

                // Try to identify what's causing the error
                if (error.message && error.message.includes('pattern')) {
                    console.error('Pattern validation error detected');
                    // Check ALL form fields one more time
                    const form = document.getElementById('config-form');
                    const allInputs = form.querySelectorAll('input, select, textarea');
                    console.error('Checking all', allInputs.length, 'form inputs:');

                    allInputs.forEach((input, index) => {
                        const parent = input.closest('.form-group');
                        const isVisible = parent ? window.getComputedStyle(parent).display !== 'none' : true;
                        const isInHiddenSection = input.closest('.form-section:not(.show)') !== null;

                        console.log('Field ' + index + ': ' + (input.name || input.id), {
                            type: input.type,
                            value: input.value,
                            pattern: input.getAttribute('pattern'),
                            required: input.hasAttribute('required'),
                            visible: isVisible,
                            inHiddenSection: isInHiddenSection,
                            validity: input.validity,
                            validationMessage: input.validationMessage,
                            checkValidity: input.checkValidity()
                        });
                    });
                }

                document.getElementById('result').innerHTML = ` + "`" + `
                    <div class="error">
                        <h3>❌ Submission Failed</h3>
                        <p>Failed to submit configuration: ${error.message}</p>
                        <p style="font-size: 12px; color: #666;">Check browser console for detailed debugging information</p>
                    </div>
                ` + "`" + `;
                document.getElementById('result').style.display = 'block';
            });
            } catch (e) {
                console.error('Exception thrown before fetch:', e);
                throw e;
            }
        });

        // Add event listeners for progress tracking and clearing validation
        document.querySelectorAll('#config-form input, #config-form select').forEach(input => {
            input.addEventListener('input', function() {
                updateProgress();
                // Clear any custom validation message when user starts typing
                this.setCustomValidity('');
            });
            input.addEventListener('change', function() {
                updateProgress();
                // Clear any custom validation message when field changes
                this.setCustomValidity('');
            });
        });

        // Check for errors on page load
        function checkForErrors() {
            fetch('/api/error')
                .then(response => response.json())
                .then(data => {
                    if (data.error && data.error !== '') {
                        const banner = document.getElementById('error-banner');
                        const title = document.getElementById('error-title');
                        const message = document.getElementById('error-message');
                        const suggestion = document.getElementById('error-suggestion');

                        message.textContent = data.error;

                        switch (data.errorType) {
                            case 'os':
                                banner.style.background = '#fff3e0';
                                banner.style.border = '2px solid #f57c00';
                                title.style.color = '#f57c00';
                                title.textContent = '⚠️ Unsupported Operating System';
                                suggestion.textContent = 'This server cannot run Cluster-Bloom due to OS compatibility. Please use a supported Ubuntu version (20.04, 22.04, or 24.04).';
                                break;
                            case 'system':
                                banner.style.background = '#fff3e0';
                                banner.style.border = '2px solid #f57c00';
                                title.style.color = '#f57c00';
                                title.textContent = '⚠️ System Requirements Not Met';
                                suggestion.textContent = 'This server does not meet the minimum system requirements. Please upgrade hardware or try a different server.';
                                break;
                            case 'config':
                                banner.style.background = '#ffebee';
                                banner.style.border = '2px solid #d32f2f';
                                title.style.color = '#d32f2f';
                                title.textContent = '❌ Configuration Error';
                                suggestion.textContent = 'Please update your configuration below and try again.';
                                break;
                            default:
                                banner.style.background = '#ffebee';
                                banner.style.border = '2px solid #d32f2f';
                                title.style.color = '#d32f2f';
                                title.textContent = '❌ Installation Failed';
                                suggestion.textContent = 'Please review the error and update your configuration if needed.';
                                break;
                        }

                        banner.style.display = 'block';
                    }
                })
                .catch(err => {
                    console.log('No error to display');
                });
        }

        // Load pre-filled configuration if available
        function loadPrefilledConfig() {
            console.log('Starting loadPrefilledConfig...');
            fetch('/api/prefilled-config')
                .then(response => {
                    console.log('API response status:', response.status);
                    return response.json();
                })
                .then(data => {
                    console.log('API response data:', data);
                    if (data.hasPrefilled && data.config) {
                        console.log('Loading pre-filled configuration with', Object.keys(data.config).length, 'values');
                        window.prefilledData = data.config;  // Store for debugging

                        // Set global one-shot flag for validation error handling
                        window.isOneShot = data.oneShot;

                        // Show banner indicating pre-filled configuration
                        const banner = document.getElementById('error-banner');
                        const title = document.getElementById('error-title');
                        const message = document.getElementById('error-message');
                        const suggestion = document.getElementById('error-suggestion');

                        if (banner && title && message && suggestion) {
                            banner.style.background = '#e8f5e8';
                            banner.style.border = '2px solid #2e7d32';
                            title.style.color = '#2e7d32';
                            title.textContent = '📄 Configuration Loaded from File';
                            message.textContent = 'Configuration has been pre-filled from your config file.';

                            if (data.oneShot) {
                                suggestion.textContent = 'One-shot mode enabled - auto-proceeding...';
                            } else {
                                suggestion.textContent = 'Please review the configuration below and click "Generate Configuration & Start Installation" to proceed.';
                            }

                            banner.style.display = 'block';
                        }

                        // Fill form fields with pre-filled values
                        const config = data.config;
                        let fieldsSet = 0;
                        let fieldsNotFound = [];

                        // Boolean fields (checkboxes) - viper keys are lowercase
                        const booleanFields = {
                            'FIRST_NODE': 'first_node',
                            'GPU_NODE': 'gpu_node',
                            'CONTROL_PLANE': 'control_plane',
                            'SKIP_DISK_CHECK': 'skip_disk_check',
                            'USE_CERT_MANAGER': 'use_cert_manager'
                        };

                        Object.entries(booleanFields).forEach(([fieldId, configKey]) => {
                            const element = document.getElementById(fieldId);
                            if (element) {
                                if (config[configKey] !== undefined) {
                                    element.checked = config[configKey];
                                    console.log('Set checkbox', fieldId, 'to', config[configKey]);
                                    fieldsSet++;
                                }
                            } else {
                                fieldsNotFound.push(fieldId);
                            }
                        });

                        // String fields - map form IDs to viper keys (lowercase)
                        const stringFieldMap = {
                            'DOMAIN': 'domain',
                            'SERVER_IP': 'server_ip',
                            'JOIN_TOKEN': 'join_token',
                            'SELECTED_DISKS': 'selected_disks',
                            'LONGHORN_DISKS': 'longhorn_disks',
                            'OIDC_URL': 'oidc_url',
                            'CLUSTERFORGE_RELEASE': 'clusterforge_release',
                            'DISABLED_STEPS': 'disabled_steps',
                            'ENABLED_STEPS': 'enabled_steps',
                            'CERT_OPTION': 'cert_option',
                            'TLS_CERT': 'tls_cert',
                            'TLS_KEY': 'tls_key'
                        };

                        Object.entries(stringFieldMap).forEach(([fieldId, configKey]) => {
                            const element = document.getElementById(fieldId);
                            if (element) {
                                const value = config[configKey];
                                if (value !== undefined && value !== null && value !== '') {
                                    element.value = String(value);
                                    // Trigger change event to update any dependent fields
                                    element.dispatchEvent(new Event('change', { bubbles: true }));
                                    console.log('Set text field', fieldId, 'to', value);
                                    fieldsSet++;
                                }
                            } else {
                                fieldsNotFound.push(fieldId);
                            }
                        });

                        console.log('Fields successfully set:', fieldsSet);
                        if (fieldsNotFound.length > 0) {
                            console.warn('Fields not found in DOM:', fieldsNotFound);
                        }

                        // Update conditionals after filling
                        updateConditionals();
                        updateProgress();

                        // Auto-submit if in one-shot mode
                        if (data.oneShot) {
                            console.log('One-shot mode detected, preparing to auto-submit...');
                            setTimeout(() => {
                                const submitEvent = new Event('submit', { bubbles: true, cancelable: true });
                                document.getElementById('config-form').dispatchEvent(submitEvent);
                            }, 500);
                        }
                    } else {
                        console.log('No pre-filled configuration available');
                        // Still need to call these when no prefilled config
                        updateConditionals();
                        updateProgress();
                    }
                })
                .catch(err => {
                    console.error('Error loading pre-filled configuration:', err);
                    // Still need to call these on error
                    updateConditionals();
                    updateProgress();
                });
        }

        // Initialize when DOM is ready
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', function() {
                console.log('DOM ready, initializing...');
                // Don't call updateConditionals here - it's called inside loadPrefilledConfig
                loadPrefilledConfig();
                checkForErrors();
                // updateProgress is called inside loadPrefilledConfig
            });
        } else {
            // DOM is already loaded
            console.log('DOM already loaded, initializing...');
            // Don't call updateConditionals here - it's called inside loadPrefilledConfig
            loadPrefilledConfig();
            checkForErrors();
            // updateProgress is called inside loadPrefilledConfig
        }
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
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
	if err := os.WriteFile(filename, yamlData, 0644); err != nil {
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

		if err := os.Rename(logPath, archivedPath); err != nil {
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
		"status": "success",
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