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
	"strings"

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
	}
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

	response := map[string]interface{}{
		"config":   h.prefilledConfig,
		"oneShot":  h.oneShot,
		"hasPrefilled": len(h.prefilledConfig) > 0,
	}


	json.NewEncoder(w).Encode(response)
}

func (h *WebHandlerService) DashboardHandler(w http.ResponseWriter, r *http.Request) {
	if h.configMode {
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
                <button class="btn btn-secondary" id="reconfigure-btn" onclick="window.location.href='/'">Reconfigure</button>
            </div>
            <div id="os-error-banner" style="display: none; background: #fff3e0; border: 2px solid #f57c00; border-radius: 4px; padding: 15px; margin-top: 15px;">
                <h3 style="color: #f57c00; margin: 0 0 10px 0;">⚠️ Unsupported Operating System</h3>
                <p id="os-error-message" style="margin: 0; color: #666;"></p>
                <p style="margin: 10px 0 0 0; color: #666; font-style: italic;">This server cannot run Cluster-Bloom. Please use a supported Ubuntu version (20.04, 22.04, or 24.04).</p>
            </div>
        </div>

        <div class="overview" id="overview"></div>

        <div class="section">
            <div class="section-header">📋 Installation Steps</div>
            <div class="section-content">
                <div class="progress-bar">
                    <div class="progress-fill" id="overall-progress"></div>
                </div>
                <div class="progress-text" id="progress-text"></div>
                <div id="steps"></div>
            </div>
        </div>

        <div class="section">
            <div class="section-header">🔧 Variables</div>
            <div class="section-content" id="variables"></div>
        </div>

        <div class="section">
            <div class="section-header">📋 Recent Logs</div>
            <div class="section-content" id="logs"></div>
        </div>
    </div>

    <script>
        let lastRefresh = 0;

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

        <form id="config-form">
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
                        <input type="text" id="DOMAIN" name="DOMAIN" placeholder="cluster.example.com" required>
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
                        <input type="text" id="SERVER_IP" name="SERVER_IP" placeholder="192.168.1.100">
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
                        <input type="text" id="SELECTED_DISKS" name="SELECTED_DISKS" placeholder="/dev/sdb,/dev/sdc">
                    </div>

                    <div class="form-group">
                        <label for="LONGHORN_DISKS">Longhorn Disks</label>
                        <div class="description">Comma-separated list of disk paths for Longhorn storage. Leave empty for automatic configuration.</div>
                        <input type="text" id="LONGHORN_DISKS" name="LONGHORN_DISKS" placeholder="/dev/sdb,/dev/sdc">
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
                        <input type="text" id="TLS_CERT" name="TLS_CERT" placeholder="/path/to/cert.pem">
                    </div>

                    <div class="form-group conditional" id="tls-key-section">
                        <label for="TLS_KEY">TLS Private Key Path *</label>
                        <div class="description">Path to TLS private key file for ingress (PEM format).</div>
                        <input type="text" id="TLS_KEY" name="TLS_KEY" placeholder="/path/to/key.pem">
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
                        <input type="url" id="OIDC_URL" name="OIDC_URL" placeholder="https://your-oidc-provider.com">
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

            if (useCertManager) {
                certOptionSection.classList.remove('show');
                tlsCertSection.classList.remove('show');
                tlsKeySection.classList.remove('show');
            } else {
                certOptionSection.classList.add('show');
                if (certOption === 'existing') {
                    tlsCertSection.classList.add('show');
                    tlsKeySection.classList.add('show');
                } else {
                    tlsCertSection.classList.remove('show');
                    tlsKeySection.classList.remove('show');
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

            const formData = new FormData(this);
            const config = {};

            // Convert form data to config object
            for (let [key, value] of formData.entries()) {
                const input = document.querySelector(` + "`" + `[name="${key}"]` + "`" + `);
                if (input.type === 'checkbox') {
                    config[key] = input.checked;
                } else if (value.trim()) {
                    config[key] = value.trim();
                }
            }

            // Validate required fields
            const firstNode = config.FIRST_NODE !== false;
            const useCertManager = config.USE_CERT_MANAGER === true;
            const certOption = config.CERT_OPTION;

            let errors = [];
            if (!config.DOMAIN) errors.push('Domain is required');
            if (!firstNode) {
                if (!config.SERVER_IP) errors.push('Server IP is required for additional nodes');
                if (!config.JOIN_TOKEN) errors.push('Join Token is required for additional nodes');
            }
            if (!useCertManager && !certOption) {
                errors.push('Certificate option is required when not using cert-manager');
            }
            if (certOption === 'existing') {
                if (!config.TLS_CERT) errors.push('TLS Certificate path is required for existing certificates');
                if (!config.TLS_KEY) errors.push('TLS Private Key path is required for existing certificates');
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
                    alert('Please fix the following errors:\\n\\n' + errors.join('\\n'));
                    return;
                }
            }

            // Submit configuration
            fetch('/api/config', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(config)
            })
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
                document.getElementById('result').innerHTML = ` + "`" + `
                    <div class="error">
                        <h3>❌ Submission Failed</h3>
                        <p>Failed to submit configuration: ${error.message}</p>
                    </div>
                ` + "`" + `;
                document.getElementById('result').style.display = 'block';
            });
        });

        // Add event listeners for progress tracking
        document.querySelectorAll('#config-form input, #config-form select').forEach(input => {
            input.addEventListener('input', updateProgress);
            input.addEventListener('change', updateProgress);
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
            fetch('/api/prefilled-config')
                .then(response => response.json())
                .then(data => {
                    if (data.hasPrefilled) {
                        console.log('Loading pre-filled configuration:', data.config);

                        // Set global one-shot flag for validation error handling
                        window.isOneShot = data.oneShot;

                        // Show banner indicating pre-filled configuration
                        const banner = document.getElementById('error-banner');
                        const title = document.getElementById('error-title');
                        const message = document.getElementById('error-message');
                        const suggestion = document.getElementById('error-suggestion');

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

                        // Fill form fields with pre-filled values
                        const config = data.config;

                        // Boolean fields (checkboxes) - viper keys are lowercase
                        if (config.first_node !== undefined) {
                            document.getElementById('FIRST_NODE').checked = config.first_node;
                        }
                        if (config.gpu_node !== undefined) {
                            document.getElementById('GPU_NODE').checked = config.gpu_node;
                        }
                        if (config.control_plane !== undefined) {
                            document.getElementById('CONTROL_PLANE').checked = config.control_plane;
                        }
                        if (config.skip_disk_check !== undefined) {
                            document.getElementById('SKIP_DISK_CHECK').checked = config.skip_disk_check;
                        }
                        if (config.use_cert_manager !== undefined) {
                            document.getElementById('USE_CERT_MANAGER').checked = config.use_cert_manager;
                        }

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
                            if (element && config[configKey] !== undefined && config[configKey] !== '') {
                                element.value = config[configKey];
                            }
                        });

                        // Update conditionals after filling
                        updateConditionals();
                        updateProgress();

                        // Auto-submit if in one-shot mode
                        if (data.oneShot) {
                            // Auto-submit after form is fully populated
                            const submitEvent = new Event('submit', { bubbles: true, cancelable: true });
                            document.getElementById('config-form').dispatchEvent(submitEvent);
                        }
                    }
                })
                .catch(err => {
                    console.log('No pre-filled configuration available');
                });
        }

        // Initialize
        loadPrefilledConfig();
        checkForErrors();
        updateConditionals();
        updateProgress();
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