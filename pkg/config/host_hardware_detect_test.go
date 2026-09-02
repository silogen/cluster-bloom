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

import (
	"fmt"
	"testing"
)

func TestApplyAIMHardwareFamilyDefaultPreservesExplicitValue(t *testing.T) {
	cfg := Config{"AIM_HARDWARE_FAMILY": "instinct"}
	_, applied := ApplyAIMHardwareFamilyDefault(cfg)
	if applied {
		t.Fatal("explicit AIM_HARDWARE_FAMILY must not be overwritten")
	}
	if cfg["AIM_HARDWARE_FAMILY"] != "instinct" {
		t.Fatalf("cfg = %#v", cfg["AIM_HARDWARE_FAMILY"])
	}
}

func TestApplyAIMHardwareFamilyDefaultDetectsHost(t *testing.T) {
	SetHardwareDetectHooksForTest(t,
		func() (string, error) {
			return "0000:03:00.0 Processing accelerators [1200]: AMD/ATI Device [1002:74a1]\n", nil
		},
		func() (string, error) {
			return "vendor_id : AuthenticAMD\nmodel name : AMD EPYC 9124 16-Core Processor\n", nil
		},
	)

	cfg := Config{}
	scan, applied := ApplyAIMHardwareFamilyDefault(cfg)
	if !applied {
		t.Fatal("empty AIM_HARDWARE_FAMILY must be auto-detected")
	}
	if cfg["AIM_HARDWARE_FAMILY"] != "epyc,instinct" {
		t.Fatalf("cfg = %q, want epyc,instinct", cfg["AIM_HARDWARE_FAMILY"])
	}
	if !scan.Detected.GPUScanSucceeded || !scan.Detected.CPUScanSucceeded {
		t.Fatalf("scan = %+v", scan)
	}
}

func TestHostHardwareScanWarnings(t *testing.T) {
	scan := HostHardwareScan{
		GPUErr: fmt.Errorf("lspci: not found"),
		Detected: DetectedHardware{
			GPU: DetectedGPUFamilies{UnmappedDeviceIDs: []string{"abcd"}},
		},
	}
	warnings := scan.Warnings()
	if len(warnings) != 2 {
		t.Fatalf("warnings = %#v", warnings)
	}
}
