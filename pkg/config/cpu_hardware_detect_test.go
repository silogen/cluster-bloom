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

package config

import "testing"

func TestParseCPUInfoForEPYC(t *testing.T) {
	tests := []struct {
		name     string
		cpuinfo  string
		detected bool
		model    string
	}{
		{
			name: "physical EPYC",
			cpuinfo: "vendor_id : AuthenticAMD\n" +
				"model name : AMD EPYC 9124 16-Core Processor\n",
			detected: true,
			model:    "AMD EPYC 9124 16-Core Processor",
		},
		{
			name: "virtualized EPYC without vendor",
			cpuinfo: "model name : AMD EPYC 9J14\n",
			detected: true,
			model:    "AMD EPYC 9J14",
		},
		{
			name: "non EPYC AMD CPU",
			cpuinfo: "vendor_id : AuthenticAMD\n" +
				"model name : AMD Ryzen 9 9950X\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detected, model := ParseCPUInfoForEPYC(test.cpuinfo)
			if detected != test.detected || model != test.model {
				t.Fatalf("got (%t, %q), want (%t, %q)",
					detected, model, test.detected, test.model)
			}
		})
	}
}
