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
	"os/exec"
	"time"

	"github.com/spf13/viper"
)

var DemoCheckUbuntuStep = Step{
	Name:        "Check Ubuntu Version",
	Description: "Verify running on supported Ubuntu version",
	Action: func() error {
		LogMessage(Debug, "Config value for 'demo': "+viper.GetString("demo"))
		LogMessage(Info, "OS version is compatible")
		time.Sleep(2 * time.Second)
		return nil
	},
}

var DemoPackagesStep = Step{
	Name:        "Packages",
	Description: "Install required packages",
	Action: func() error {
		LogMessage(Info, "APT install Packages")
		time.Sleep(2 * time.Second)
		return nil
	},
}

var DemoFirewallStep = Step{
	Name:        "Configure Firewall",
	Description: "Open required ports",
	Action: func() error {
		LogMessage(Info, "Adding iptables ports")
		time.Sleep(2 * time.Second)
		return nil
	},
}

var DemoInotifyStep = Step{
	Name:        "Inotify Configuration",
	Description: "Verify inotify instances",
	Action: func() error {
		LogMessage(Info, "Configuring number of dir/file watchers")
		cmd := exec.Command("/bin/cat", "/etc/hosts")
		output, err := cmd.CombinedOutput()

		if err != nil {
			LogMessage(Error, "Failed to execute command: "+err.Error())
			return err
		}
		LogCommand("/bin/cat /etc/hosts", string(output))
		return nil
	},
}

var DemoRocmStep = Step{
	Name:        "ROCm installation",
	Description: "Verify ROCm installation",
	Action: func() error {
		LogMessage(Info, "installing and checking ROCm")
		time.Sleep(2 * time.Second)
		return nil
	},
}

var DemoDiskMountsStep = Step{
	Name:        "Disk mounts",
	Description: "Verify Disk mounts",
	Action: func() error {
		LogMessage(Info, "Disks available")
		time.Sleep(2 * time.Second)
		return nil
	},
}

var DemoRke2Step = Step{
	Name:        "RKE2 installation",
	Description: "Verify RKE2 installation",
	Action: func() error {
		LogMessage(Info, "Installing RKE2")
		time.Sleep(2 * time.Second)
		return nil
	},
}

var DemoLonghornStep = Step{
	Name:        "Longhorn",
	Description: "Verify Longhorn setup",
	Action: func() error {
		LogMessage(Info, "Installing Longhorn nodes")
		time.Sleep(2 * time.Second)
		return nil
	},
}

var DemoIngressStep = Step{
	Name:        "Ingress",
	Description: "Verify Ingress configuration",
	Action: func() error {
		LogMessage(Info, "Configuring ingress")
		time.Sleep(2 * time.Second)
		return nil
	},
}

var DemoMinioStep = Step{
	Name:        "Minio",
	Description: "Verify Minio configuration",
	Action: func() error {
		LogMessage(Info, "Setting up Minio tenant and access")
		time.Sleep(2 * time.Second)
		return nil
	},
}

var DemoDashboardStep = Step{
	Name:        "Dashboard",
	Description: "Setup monitoring dashboard",
	Action: func() error {
		LogMessage(Info, "Setting up monitoring dashboard")
		time.Sleep(2 * time.Second)
		return nil
	},
}
