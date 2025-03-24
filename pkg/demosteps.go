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
	"time"
)

var DemoCheckUbuntuStep = Step{
	Name:        "Demo Check Ubuntu",
	Description: "Demo step to simulate Ubuntu version check",
	Action: func() StepResult {
		time.Sleep(2 * time.Second)
		return StepResult{
			Error: nil,
		}
	},
}

var DemoPackagesStep = Step{
	Name:        "Demo Install Packages",
	Description: "Demo step to simulate package installation",
	Action: func() StepResult {
		time.Sleep(3 * time.Second)
		return StepResult{
			Error: nil,
		}
	},
}

var DemoFirewallStep = Step{
	Name:        "Demo Configure Firewall",
	Description: "Demo step to simulate firewall configuration",
	Action: func() StepResult {
		time.Sleep(1 * time.Second)
		return StepResult{
			Error: nil,
		}
	},
}

var DemoMinioStep = Step{
	Name:        "Demo MinIO Setup",
	Description: "Demo step to simulate MinIO installation",
	Action: func() StepResult {
		time.Sleep(2 * time.Second)
		return StepResult{
			Error: nil,
		}
	},
}

var DemoDashboardStep = Step{
	Name:        "Demo Dashboard",
	Description: "Demo step to simulate dashboard setup",
	Action: func() StepResult {
		time.Sleep(1 * time.Second)
		return StepResult{
			Error: nil,
		}
	},
}
