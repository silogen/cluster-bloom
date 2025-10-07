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

package cmd

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

// validateBool validates boolean input
func validateBool(input string) error {
	lower := strings.ToLower(strings.TrimSpace(input))
	validValues := []string{"true", "false", "t", "f", "yes", "no", "y", "n", "1", "0"}
	for _, v := range validValues {
		if lower == v {
			return nil
		}
	}
	return fmt.Errorf("invalid boolean value. Please enter: true/false, yes/no, y/n, or 1/0")
}

// validateURL validates that a string is a proper URL with http/https scheme
func validateURL(urlStr, paramName string) error {
	if urlStr == "" {
		return nil // Empty URLs are allowed for optional parameters
	}

	// Handle special case for CLUSTERFORGE_RELEASE
	if paramName == "CLUSTERFORGE_RELEASE" && strings.ToLower(urlStr) == "none" {
		return nil
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format for %s: %v", paramName, err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme for %s: must be http or https, got %s", paramName, parsedURL.Scheme)
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("invalid URL for %s: missing host", paramName)
	}

	return nil
}

// validateIPAddress validates that a string is a valid IPv4 or IPv6 address
func validateIPAddress(ipStr, paramName string) error {
	if ipStr == "" {
		return nil // Empty IPs are allowed for optional parameters
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return fmt.Errorf("invalid IP address for %s: %s", paramName, ipStr)
	}

	// Check for disallowed IP addresses
	if ip.IsLoopback() {
		return fmt.Errorf("loopback IP address not allowed for %s: %s", paramName, ipStr)
	}

	if ip.IsUnspecified() {
		return fmt.Errorf("unspecified IP address (0.0.0.0 or ::) not allowed for %s: %s", paramName, ipStr)
	}

	// For SERVER_IP, we want to allow private/internal IPs since clusters often use internal networks
	// We only reject clearly invalid addresses like loopback and unspecified
	return nil
}

// validateAllIPs validates all IP address configuration parameters
func validateAllIPs() error {
	// Only validate SERVER_IP if it's required (when FIRST_NODE is false)
	if !viper.GetBool("FIRST_NODE") {
		serverIP := viper.GetString("SERVER_IP")
		if err := validateIPAddress(serverIP, "SERVER_IP"); err != nil {
			return err
		}
	}

	return nil
}

// validateToken validates token format based on the token type
func validateToken(token, paramName string) error {
	if token == "" {
		return nil // Empty tokens are allowed for optional parameters
	}

	switch paramName {
	case "JOIN_TOKEN":
		return validateJoinToken(token)
	default:
		return fmt.Errorf("unknown token type: %s", paramName)
	}
}

// validateJoinToken validates RKE2/K3s join token format
func validateJoinToken(token string) error {
	// RKE2/K3s tokens are typically:
	// - Base64-encoded or hex strings
	// - Usually 64+ characters long
	// - Contain alphanumeric characters, +, /, =

	// Empty tokens are handled by validateToken function
	if token == "" {
		return nil
	}

	if len(token) < 32 {
		return fmt.Errorf("JOIN_TOKEN is too short (minimum 32 characters), got %d characters", len(token))
	}

	if len(token) > 512 {
		return fmt.Errorf("JOIN_TOKEN is too long (maximum 512 characters), got %d characters", len(token))
	}

	// Allow base64 characters, hex characters, and common separators including colons
	validTokenPattern := regexp.MustCompile(`^[a-zA-Z0-9+/=_.:-]+$`)
	if !validTokenPattern.MatchString(token) {
		return fmt.Errorf("JOIN_TOKEN contains invalid characters (only alphanumeric, +, /, =, _, ., :, - allowed)")
	}

	return nil
}
