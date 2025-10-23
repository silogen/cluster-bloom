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

	"github.com/silogen/cluster-bloom/pkg/command"
	"github.com/silogen/cluster-bloom/pkg/fsops"
	"github.com/spf13/viper"
)

// Define chrony configuration templates as package-level variables
var (
    chronyTemplateFirst = `pool 0.pool.ntp.org iburst maxsources 2
server time.google.com iburst
server time.cloudflare.com iburst

pool pool.ntp.org iburst maxsources 4

allow 10.0.0.0/8
`

    chronyTemplateAdditional = `pool pool.ntp.org iburst maxsources 4
server time.google.com iburst
server time.cloudflare.com iburst

server %s iburst prefer

pool 0.pool.ntp.org iburst maxsources 2
`
)

// GenerateChronyConfFirst creates a chrony.conf for first node
func GenerateChronyConfFirst() error {
	  firstNode := viper.GetBool("FIRST_NODE")
	  if !firstNode {
			return fmt.Errorf("This is not FIRST_NODE")
		}
    chronyConf := chronyTemplateFirst

		LogMessage(Info, "Create chrony.conf for first node")
    if err := writeChronyConf(chronyConf); err != nil {
        return err
    }

    // Restart chronyd service
    if output, err := command.CombinedOutput(false, "systemctl", "restart", "chronyd"); err != nil {
        return fmt.Errorf("failed to restart chronyd: %w, output: %s", err, string(output))
    }

		LogMessage(Info, "Chronyd restarted")
    return nil
}

// GenerateChronyConfAdditional creates a chrony.conf for additional node.
func GenerateChronyConfAdditional() error {
		serverIP := viper.GetString("SERVER_IP")
	  if serverIP == "" {
		    return fmt.Errorf("SERVER_IP configuration item is not set")
		}
    chronyConf := fmt.Sprintf(chronyTemplateAdditional, serverIP)
		LogMessage(Info, "Create chrony.conf for additional node")

    if err := writeChronyConf(chronyConf); err != nil {
        return err
    }

    // Restart chronyd service
    if output, err := command.CombinedOutput(false, "systemctl", "restart", "chronyd"); err != nil {
        return fmt.Errorf("failed to restart chronyd: %w, output: %s", err, string(output))
    }

		LogMessage(Info, "Chronyd restarted")
    return nil
}

// writeChronyConf writes the chrony configuration to the specified file.
func writeChronyConf(chronyConf string) error {
    if err := command.SimpleRun(false, "cp", "/etc/chrony/chrony.conf", "/etc/chrony/chrony.conf.bak"); err != nil {
        return fmt.Errorf("failed to backup chrony.conf: %w", err)
    }
    targetPath := "/etc/chrony/chrony.conf"
		LogMessage(Info, "Original chrony.conf saved as chrony.conf.bak")

    err := fsops.WriteFile(targetPath, []byte(chronyConf), 0644)
    if err != nil {
        return fmt.Errorf("failed to write %s: %w", targetPath, err)
    }
		LogMessage(Info, "Chrony.conf created")

    return nil
}
