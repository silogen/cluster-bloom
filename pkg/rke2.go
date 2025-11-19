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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/viper"
)

// OIDCConfig represents configuration for an OIDC provider
type OIDCConfig struct {
	URL       string   `yaml:"url"`
	Audiences []string `yaml:"audiences"`
}

var rke2ConfigContent = `
cni: cilium
cluster-cidr: 10.242.0.0/16
service-cidr: 10.243.0.0/16

disable: rke2-ingress-nginx
audit-log-path: "/var/lib/rancher/rke2/server/logs/kube-apiserver-audit.log"
audit-log-maxage: 30
audit-log-maxbackup: 10
audit-log-maxsize: 100
audit-policy-file: "/etc/rancher/rke2/audit-policy.yaml"
`

var oidcConfigTemplate = `
kube-apiserver-arg:
  - "--authentication-config=/etc/rancher/rke2/auth/auth-config.yaml"
`


var clusterRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: airm-realm-admin-binding
subjects:
- kind: Group
  name: "oidc:airm-role:Super Administrator"
  apiGroup: rbac.authorization.k8s.io
- kind: Group
  name: "oidc:airm-role:Platform Administrator"
  apiGroup: rbac.authorization.k8s.io
- kind: Group
  name: "oidc:argocd-users"
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: oidc-admin-binding
subjects:
- kind: Group
  name: "oidc:admin"
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: oidc-developer-binding
subjects:
- kind: Group
  name: "oidc:developer"
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: edit
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: oidc-viewer-binding
subjects:
- kind: Group
  name: "oidc:viewer"
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: view
  apiGroup: rbac.authorization.k8s.io
`

// AuthConfigTemplateData represents data for dynamic auth config template
type AuthConfigTemplateData struct {
	DefaultIssuer struct {
		URL         string
		Certificate string
	}
	OIDCProviders []struct {
		URL         string
		Certificate string
		Audiences   []string
	}
}

var dynamicAuthConfigTemplate = `apiVersion: apiserver.config.k8s.io/v1
kind: AuthenticationConfiguration
jwt:
- issuer:
    url: {{.DefaultIssuer.URL}}
    certificateAuthority: |
{{.DefaultIssuer.Certificate}}
    audiences:
    - k8s
  claimMappings:
    username:
      claim: preferred_username
      prefix: "oidc:"
    groups:
      claim: groups
      prefix: "oidc:"
{{range .OIDCProviders}}
- issuer:
    url: {{.URL}}
    certificateAuthority: |
{{.Certificate}}
    audiences:
{{range .Audiences}}    - {{.}}
{{end}}  claimMappings:
    username:
      claim: preferred_username
      prefix: "oidc:"
    groups:
      claim: groups
      prefix: "oidc:"
{{end}}`

func FetchAndSaveOIDCCertificate(url string, index int) error {
	// Parse URL to extract hostname
	var hostname string
	if strings.HasPrefix(url, "https://") {
		// Remove https:// prefix
		hostname = strings.TrimPrefix(url, "https://")
		// Remove path if present (e.g., "/auth/realms/main")
		if slashIndex := strings.Index(hostname, "/"); slashIndex != -1 {
			hostname = hostname[:slashIndex]
		}
	} else if strings.HasPrefix(url, "http://") {
		return fmt.Errorf("OIDC URL must use HTTPS, not HTTP: %s", url)
	} else {
		// Assume it's just a hostname
		hostname = url
		// Remove path if present
		if slashIndex := strings.Index(hostname, "/"); slashIndex != -1 {
			hostname = hostname[:slashIndex]
		}
	}
	
	if hostname == "" {
		return fmt.Errorf("could not extract hostname from URL: %s", url)
	}
	
	// Create OIDC certificates directory if it doesn't exist
	oidcCertDir := "/etc/rancher/rke2/certs"
	if err := os.MkdirAll(oidcCertDir, 0755); err != nil {
		return fmt.Errorf("failed to create OIDC cert directory: %v", err)
	}
	
	cmd := exec.Command("sh", "-c", fmt.Sprintf("openssl s_client -showcerts -connect %s:443 </dev/null | sed -n '/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/p'", hostname))
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to fetch certificate from %s (hostname: %s): %v", url, hostname, err)
	}
	
	if len(output) == 0 {
		return fmt.Errorf("no certificate data received from %s (hostname: %s)", url, hostname)
	}
	
	// Save certificate with index-based filename
	certPath := filepath.Join(oidcCertDir, fmt.Sprintf("oidc-provider-%d.crt", index))
	if err := os.WriteFile(certPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write certificate to %s: %v", certPath, err)
	}
	
	LogMessage(Info, fmt.Sprintf("Successfully saved OIDC certificate from %s (hostname: %s) to %s", url, hostname, certPath))
	return nil
}

func PrepareRKE2() error {
	commands := []struct {
		command string
		args    []string
	}{
		{"modprobe", []string{"iscsi_tcp"}},
		{"modprobe", []string{"dm_mod"}},
		{"mkdir", []string{"-p", "/etc/rancher/rke2"}},
		{"chmod", []string{"0755", "/etc/rancher/rke2"}},
	}

	for _, cmd := range commands {
		_, err := runCommand(cmd.command, cmd.args...)
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to execute command '%s %v': %v", cmd.command, cmd.args, err))
			return fmt.Errorf("failed to execute command '%s %v': %w", cmd.command, cmd.args, err)
		}
		LogMessage(Info, fmt.Sprintf("Successfully executed command: %s %v", cmd.command, cmd.args))
	}
	err := setupAudit()
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to setup audit: %v", err))
		return fmt.Errorf("failed to setup audit policy: %w", err)
	}
	rke2ConfigPath := "/etc/rancher/rke2/config.yaml"
	if err := os.WriteFile(rke2ConfigPath, []byte(rke2ConfigContent), 0644); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to write to %s: %v", rke2ConfigPath, err))
		return err
	}

	// Handle certificate and authentication configuration
	domain := viper.GetString("DOMAIN")
	if domain != "" {
		certOption := viper.GetString("CERT_OPTION")
		
		// Create persistent cert directory
		certDir := "/etc/rancher/rke2/certs"
		if err := os.MkdirAll(certDir, 0755); err != nil {
			return fmt.Errorf("failed to create cert directory: %w", err)
		}
		
		var tlsCertPath, tlsKeyPath string
		
		if certOption == "generate" {
			// CASE 1: Generate certificates to persistent location
			tlsCertPath = filepath.Join(certDir, "tls.crt")
			tlsKeyPath = filepath.Join(certDir, "tls.key") 
			
			LogMessage(Info, "Generating self-signed certificate for domain: "+domain)
			
			// Generate self-signed certificate
			cmd := exec.Command("openssl", "req", "-x509", "-nodes", "-days", "365", "-newkey", "rsa:2048",
				"-keyout", tlsKeyPath,
				"-out", tlsCertPath, 
				"-subj", fmt.Sprintf("/CN=%s", domain),
				"-addext", fmt.Sprintf("subjectAltName=DNS:%s,DNS:*.%s", domain, domain))
				
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to generate certificate: %w, output: %s", err, string(output))
			}
			
			LogMessage(Info, fmt.Sprintf("Generated certificate at %s", tlsCertPath))
			
		} else if certOption == "existing" {
			// CASE 2: Copy existing certificates to persistent location
			sourceCertPath := viper.GetString("TLS_CERT")
			sourceKeyPath := viper.GetString("TLS_KEY")
			
			// Validate source files exist
			if sourceCertPath == "" || sourceKeyPath == "" {
				return fmt.Errorf("TLS_CERT and TLS_KEY must be provided for existing certificates")
			}
			if _, err := os.Stat(sourceCertPath); os.IsNotExist(err) {
				return fmt.Errorf("TLS certificate file not found: %s", sourceCertPath)
			}
			if _, err := os.Stat(sourceKeyPath); os.IsNotExist(err) {
				return fmt.Errorf("TLS key file not found: %s", sourceKeyPath)
			}
			
			// Copy to persistent location
			tlsCertPath = filepath.Join(certDir, "tls.crt")
			tlsKeyPath = filepath.Join(certDir, "tls.key")
			
			if err := copyFile(sourceCertPath, tlsCertPath); err != nil {
				return fmt.Errorf("failed to copy certificate: %w", err)
			}
			if err := copyFile(sourceKeyPath, tlsKeyPath); err != nil {
				return fmt.Errorf("failed to copy key: %w", err)
			}
			
			LogMessage(Info, fmt.Sprintf("Copied existing certificate to %s", tlsCertPath))
			
		} else {
			return fmt.Errorf("CERT_OPTION must be 'generate' or 'existing' when DOMAIN is specified")
		}
		
		// Store paths for later use by CreateDomainConfigStep
		viper.Set("RUNTIME_TLS_CERT", tlsCertPath)
		viper.Set("RUNTIME_TLS_KEY", tlsKeyPath)
		
		// Parse OIDC_URLS configuration
		oidcConfigs, err := parseOIDCConfiguration()
		if err != nil {
			return fmt.Errorf("failed to parse OIDC configuration: %w", err)
		}
		
		// Validate OIDC URLs and fetch certificates
		if len(oidcConfigs) > 0 {
			if valid, err := validateOIDCURLs(oidcConfigs); !valid {
				return fmt.Errorf("OIDC validation failed: %w", err)
			}
		}
		
		// Create dynamic auth-config.yaml (includes default airm + OIDC providers)
		if err := createDynamicAuthConfig(domain, oidcConfigs); err != nil {
			return fmt.Errorf("failed to create auth config: %w", err)
		}
		
		// Add authentication-config to RKE2 config
		if err := addAuthConfigToRKE2(); err != nil {
			return fmt.Errorf("failed to update RKE2 config: %w", err)
		}
		
		LogMessage(Info, "Authentication configuration completed")
	}

	return nil
}

func SetupFirstRKE2() error {
	installCommand := "curl -sfL " + viper.GetString("RKE2_INSTALLATION_URL")
	rke2Version := viper.GetString("RKE2_VERSION")
	if rke2Version != "" {
		installCommand += " | INSTALL_RKE2_VERSION=\"" + rke2Version + "\" sh -"
	} else {
		installCommand += " | sh -"
	}

	commands := []struct {
		command string
		args    []string
	}{
		{"sh", []string{"-c", installCommand}},
		{"systemctl", []string{"enable", "rke2-server.service"}},
	}

	for _, cmd := range commands {
		_, err := runCommand(cmd.command, cmd.args...)
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to execute command '%s %v': %v", cmd.command, cmd.args, err))
			return fmt.Errorf("failed to execute command '%s %v': %w", cmd.command, cmd.args, err)
		}
		LogMessage(Info, fmt.Sprintf("Successfully executed command: %s %v", cmd.command, cmd.args))
	}

	if err := startServiceWithTimeout("rke2-server", 2*time.Minute); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to start rke2-server service: %v", err))
		return err
	}

	return nil
}

func startServiceWithTimeout(serviceName string, timeout time.Duration) error {
	_, err := runCommand("systemctl", "start", serviceName+".service")
	LogMessage(Info, fmt.Sprintf("Starting service %s", serviceName))
	if err != nil {
		return fmt.Errorf("failed to start service %s: %w", serviceName, err)
	}

	LogMessage(Info, fmt.Sprintf("Waiting for service %s to become active (timeout: %v)", serviceName, timeout))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// The exec.Command is fine here as it uses CombinedOutput
		isActiveCmd := exec.Command("systemctl", "is-active", serviceName+".service")
		output, err := isActiveCmd.CombinedOutput()
		status := string(output)
		if err == nil && status == "active\n" {
			LogMessage(Info, fmt.Sprintf("Service %s is now active", serviceName))
			return nil
		}
		LogMessage(Info, fmt.Sprintf("Service %s status: %s", serviceName, status))
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for service %s to become active", serviceName)
}

func SetupRKE2Additional() error {
	serverIP := viper.GetString("SERVER_IP")
	if serverIP == "" {
		return fmt.Errorf("SERVER_IP configuration item is not set")
	}
	joinToken := viper.GetString("JOIN_TOKEN")
	if joinToken == "" {
		return fmt.Errorf("JOIN_TOKEN configuration item is not set")
	}
	rke2ConfigPath := "/etc/rancher/rke2/config.yaml"

	configContent := fmt.Sprintf("\nserver: https://%s:9345\ntoken: %s\n", serverIP, joinToken)
	file, err := os.OpenFile(rke2ConfigPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to open %s for appending: %v", rke2ConfigPath, err))
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(configContent); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to append to %s: %v", rke2ConfigPath, err))
		return err
	}

	LogMessage(Info, fmt.Sprintf("Appended configuration to %s", rke2ConfigPath))
	commands := []struct {
		command string
		args    []string
	}{
		{"sh", []string{"-c", "curl -sfL " + viper.GetString("RKE2_INSTALLATION_URL") + " | INSTALL_RKE2_TYPE=agent sh -"}},
		{"systemctl", []string{"enable", "rke2-agent.service"}},
	}
	for _, cmd := range commands {
		_, err := runCommand(cmd.command, cmd.args...)
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to execute command '%s %v': %v", cmd.command, cmd.args, err))
			return fmt.Errorf("failed to execute command '%s %v': %w", cmd.command, cmd.args, err)
		}
		LogMessage(Info, fmt.Sprintf("Successfully executed command: %s %v", cmd.command, cmd.args))
	}

	if err := startServiceWithTimeout("rke2-agent", 2*time.Minute); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to start rke2-agent service: %v", err))
		return err
	}

	return nil
}

func SetupRKE2ControlPlane() error {
	serverIP := viper.GetString("SERVER_IP")
	if serverIP == "" {
		return fmt.Errorf("SERVER_IP configuration item is not set")
	}
	joinToken := viper.GetString("JOIN_TOKEN")
	if joinToken == "" {
		return fmt.Errorf("JOIN_TOKEN configuration item is not set")
	}
	rke2ConfigPath := "/etc/rancher/rke2/config.yaml"

	configContent := fmt.Sprintf("\nserver: https://%s:9345\ntoken: %s\n", serverIP, joinToken)
	file, err := os.OpenFile(rke2ConfigPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to open %s for appending: %v", rke2ConfigPath, err))
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(configContent); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to append to %s: %v", rke2ConfigPath, err))
		return err
	}

	LogMessage(Info, fmt.Sprintf("Appended configuration to %s", rke2ConfigPath))
	commands := []struct {
		command string
		args    []string
	}{
		{"sh", []string{"-c", "curl -sfL " + viper.GetString("RKE2_INSTALLATION_URL") + " | INSTALL_RKE2_TYPE=server sh -"}},
		{"systemctl", []string{"enable", "rke2-server.service"}},
	}
	for _, cmd := range commands {
		_, err := runCommand(cmd.command, cmd.args...)
		if err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to execute command '%s %v': %v", cmd.command, cmd.args, err))
			return fmt.Errorf("failed to execute command '%s %v': %w", cmd.command, cmd.args, err)
		}
		LogMessage(Info, fmt.Sprintf("Successfully executed command: %s %v", cmd.command, cmd.args))
	}

	if err := startServiceWithTimeout("rke2-server", 2*time.Minute); err != nil {
		LogMessage(Error, fmt.Sprintf("Failed to start rke2-server service: %v", err))
		return err
	}

	return nil
}

func isValidImageName(image string) bool {
	re := regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*(\/[a-z0-9]+([._-][a-z0-9]+)*)*(?::[a-z0-9]+([._-][a-z0-9]+)*)?$`)
	return re.MatchString(image)
}

func PreloadImages() error {

	LogMessage(Info, "Found PRELOAD_IMAGES configuration")
	images := strings.Split(viper.GetString("PRELOAD_IMAGES"), ",")

	var targetImages []string
	for _, image := range images {
		image = strings.TrimSpace(image)
		if image != "" {
			if isValidImageName(image) {
				targetImages = append(targetImages, image)
			} else {
				LogMessage(Info, fmt.Sprintf("Invalid image name found in PRELOAD_IMAGES: %s", image))
			}

		}

	}

	if len(targetImages) == 0 {
		LogMessage(Info, "No valid images found in PRELOAD_IMAGES")
		return nil
	}

	LogMessage(Info, fmt.Sprintf("Preloading images: %v", targetImages))
	imagesDir := "/var/lib/rancher/rke2/agent/images"
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return fmt.Errorf("failed to create images directory %s: %v", imagesDir, err)
	}
	preloadImagesList := "/var/lib/rancher/rke2/agent/images/preload_images.txt"
	if err := os.WriteFile(preloadImagesList, []byte(strings.Join(targetImages, "\n")), 0644); err != nil {
		return fmt.Errorf("failed to write preload images file %s: %v", preloadImagesList, err)
	}

	return nil
}

func copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	
	err = os.WriteFile(dst, input, 0644)
	if err != nil {
		return err
	}
	
	return nil
}


func addAuthConfigToRKE2() error {
	rke2ConfigPath := "/etc/rancher/rke2/config.yaml"
	
	rke2AuthConfig := `
kube-apiserver-arg:
  - "--authentication-config=/etc/rancher/rke2/auth/auth-config.yaml"
`
	
	file, err := os.OpenFile(rke2ConfigPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open RKE2 config file: %w", err)
	}
	defer file.Close()

	if _, err = file.WriteString(rke2AuthConfig); err != nil {
		return fmt.Errorf("failed to append to RKE2 config: %w", err)
	}

	LogMessage(Info, "Successfully added authentication-config to RKE2 configuration")
	return nil
}

func parseOIDCConfiguration() ([]OIDCConfig, error) {
	var oidcConfigs []OIDCConfig
	
	// Get OIDC_URLS from viper configuration
	oidcURLsInterface := viper.Get("OIDC_URLS")
	if oidcURLsInterface == nil {
		LogMessage(Info, "No OIDC_URLS configured, using default configuration only")
		return oidcConfigs, nil
	}
	
	// Handle different possible input formats
	switch v := oidcURLsInterface.(type) {
	case []interface{}:
		// YAML array format
		for i, item := range v {
			itemMap, ok := item.(map[interface{}]interface{})
			if !ok {
				return nil, fmt.Errorf("OIDC_URLS[%d] must be an object with 'url' and 'audiences' fields", i)
			}
			
			// Extract URL
			urlInterface, exists := itemMap["url"]
			if !exists {
				return nil, fmt.Errorf("OIDC_URLS[%d] missing required 'url' field", i)
			}
			url, ok := urlInterface.(string)
			if !ok {
				return nil, fmt.Errorf("OIDC_URLS[%d].url must be a string", i)
			}
			
			// Extract audiences
			audiencesInterface, exists := itemMap["audiences"]
			if !exists {
				return nil, fmt.Errorf("OIDC_URLS[%d] missing required 'audiences' field", i)
			}
			
			var audiences []string
			switch aud := audiencesInterface.(type) {
			case []interface{}:
				for j, audItem := range aud {
					audStr, ok := audItem.(string)
					if !ok {
						return nil, fmt.Errorf("OIDC_URLS[%d].audiences[%d] must be a string", i, j)
					}
					audiences = append(audiences, audStr)
				}
			case string:
				// Single audience as string
				audiences = append(audiences, aud)
			default:
				return nil, fmt.Errorf("OIDC_URLS[%d].audiences must be a string or array of strings", i)
			}
			
			if len(audiences) == 0 {
				return nil, fmt.Errorf("OIDC_URLS[%d] must have at least one audience", i)
			}
			
			oidcConfigs = append(oidcConfigs, OIDCConfig{
				URL:       url,
				Audiences: audiences,
			})
		}
	case []OIDCConfig:
		// Already parsed format (unlikely but handle it)
		oidcConfigs = v
	default:
		return nil, fmt.Errorf("OIDC_URLS must be an array of objects")
	}
	
	// Validate configuration
	for i, config := range oidcConfigs {
		if config.URL == "" {
			return nil, fmt.Errorf("OIDC_URLS[%d] URL cannot be empty", i)
		}
		if len(config.Audiences) == 0 {
			return nil, fmt.Errorf("OIDC_URLS[%d] must have at least one audience", i)
		}
	}
	
	LogMessage(Info, fmt.Sprintf("Parsed %d OIDC provider configurations", len(oidcConfigs)))
	return oidcConfigs, nil
}

func validateOIDCURLs(oidcConfigs []OIDCConfig) (bool, error) {
	if len(oidcConfigs) == 0 {
		LogMessage(Info, "No OIDC URLs to validate")
		return true, nil
	}
	
	LogMessage(Info, fmt.Sprintf("Validating %d OIDC provider URLs", len(oidcConfigs)))
	
	for i, config := range oidcConfigs {
		LogMessage(Info, fmt.Sprintf("Validating OIDC URL [%d]: %s", i, config.URL))
		
		// Call the modified FetchAndSaveOIDCCertificate with index
		if err := FetchAndSaveOIDCCertificate(config.URL, i); err != nil {
			LogMessage(Error, fmt.Sprintf("Failed to validate OIDC URL [%d] %s: %v", i, config.URL, err))
			return false, fmt.Errorf("OIDC URL %s is not reachable or has certificate issues: %w", config.URL, err)
		}
		
		LogMessage(Info, fmt.Sprintf("Successfully validated OIDC URL [%d]: %s", i, config.URL))
	}
	
	LogMessage(Info, "All OIDC URLs validated successfully")
	return true, nil
}

func createDynamicAuthConfig(domain string, oidcConfigs []OIDCConfig) error {
	// Read domain certificate for default airm issuer
	domainCertPath := "/etc/rancher/rke2/certs/tls.crt"
	domainCertData, err := os.ReadFile(domainCertPath)
	if err != nil {
		return fmt.Errorf("failed to read domain certificate: %w", err)
	}
	
	// Indent domain certificate properly for YAML
	domainCertLines := strings.Split(strings.TrimSpace(string(domainCertData)), "\n")
	var domainCertIndented strings.Builder
	for _, line := range domainCertLines {
		if line != "" {
			domainCertIndented.WriteString("      " + line + "\n")
		}
	}
	
	// Prepare template data
	templateData := AuthConfigTemplateData{
		DefaultIssuer: struct {
			URL         string
			Certificate string
		}{
			URL:         fmt.Sprintf("https://kc.%s/realms/airm", domain),
			Certificate: domainCertIndented.String(),
		},
	}
	
	// Add OIDC providers to template data
	for i, config := range oidcConfigs {
		// Read OIDC provider certificate
		oidcCertPath := filepath.Join("/etc/rancher/rke2/certs", fmt.Sprintf("oidc-provider-%d.crt", i))
		oidcCertData, err := os.ReadFile(oidcCertPath)
		if err != nil {
			return fmt.Errorf("failed to read OIDC certificate for provider %d: %w", i, err)
		}
		
		// Indent OIDC certificate properly for YAML
		oidcCertLines := strings.Split(strings.TrimSpace(string(oidcCertData)), "\n")
		var oidcCertIndented strings.Builder
		for _, line := range oidcCertLines {
			if line != "" {
				oidcCertIndented.WriteString("      " + line + "\n")
			}
		}
		
		templateData.OIDCProviders = append(templateData.OIDCProviders, struct {
			URL         string
			Certificate string
			Audiences   []string
		}{
			URL:         config.URL,
			Certificate: oidcCertIndented.String(),
			Audiences:   config.Audiences,
		})
	}
	
	// Parse and execute template
	tmpl, err := template.New("authConfig").Parse(dynamicAuthConfigTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse auth config template: %w", err)
	}
	
	var authConfigContent strings.Builder
	if err := tmpl.Execute(&authConfigContent, templateData); err != nil {
		return fmt.Errorf("failed to execute auth config template: %w", err)
	}
	
	// Create auth directory
	authDir := "/etc/rancher/rke2/auth"
	if err := os.MkdirAll(authDir, 0755); err != nil {
		return fmt.Errorf("failed to create auth directory: %w", err)
	}
	
	// Write auth-config.yaml
	authConfigPath := filepath.Join(authDir, "auth-config.yaml")
	if err := os.WriteFile(authConfigPath, []byte(authConfigContent.String()), 0644); err != nil {
		return fmt.Errorf("failed to write auth-config.yaml: %w", err)
	}
	
	LogMessage(Info, fmt.Sprintf("Successfully created dynamic authentication configuration with %d OIDC providers", len(oidcConfigs)+1))
	return nil
}
