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
	"strings"
	"testing"
)

func TestGenerateYAMLAppliesAIMHardwareFamilyDefault(t *testing.T) {
	SetHardwareDetectHooksForTest(t,
		func() (string, error) { return "", nil },
		func() (string, error) {
			return "vendor_id : AuthenticAMD\nmodel name : AMD EPYC 9124 16-Core Processor\n", nil
		},
	)

	yaml := GenerateYAML(Config{})
	if !strings.Contains(yaml, `AIM_HARDWARE_FAMILY: "epyc"`) &&
		!strings.Contains(yaml, "AIM_HARDWARE_FAMILY: epyc") {
		t.Fatalf("yaml = %q", yaml)
	}
}

func TestPrepareGeneratedConfigPreservesExplicitAIMFamily(t *testing.T) {
	cfg := Config{"AIM_HARDWARE_FAMILY": "instinct"}
	prepared := PrepareGeneratedConfig(cfg)
	if prepared["AIM_HARDWARE_FAMILY"] != "instinct" {
		t.Fatalf("prepared = %#v", prepared["AIM_HARDWARE_FAMILY"])
	}
}
