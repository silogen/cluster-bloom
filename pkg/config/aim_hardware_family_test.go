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
	"reflect"
	"testing"
)

func TestFormatAIMHardwareFamily(t *testing.T) {
	tests := []struct {
		name     string
		detected DetectedHardware
		want     string
	}{
		{
			name: "generic CPU fallback",
			want: FamilyCPU,
		},
		{
			name: "EPYC",
			detected: DetectedHardware{
				EPYCModel: "AMD EPYC 9124 16-Core Processor",
			},
			want: FamilyEPYC,
		},
		{
			name: "GPU and EPYC",
			detected: DetectedHardware{
				GPU: DetectedGPUFamilies{
					Families: []string{FamilyRadeon},
				},
				EPYCModel: "AMD EPYC 9124 16-Core Processor",
			},
			want: "epyc,radeon",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FormatAIMHardwareFamily(test.detected); got != test.want {
				t.Fatalf("FormatAIMHardwareFamily() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestUnsupportedAIMHardwareFamilies(t *testing.T) {
	detected := DetectedHardware{
		GPU: DetectedGPUFamilies{
			Families: []string{FamilyRadeon},
		},
		GPUScanSucceeded: true,
		CPUScanSucceeded: true,
	}

	got := UnsupportedAIMHardwareFamilies(
		"cpu,epyc,instinct,radeon,instinct",
		detected,
	)
	want := []string{FamilyEPYC, FamilyInstinct}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UnsupportedAIMHardwareFamilies() = %v, want %v", got, want)
	}
}

func TestUnsupportedAIMHardwareFamiliesDoesNotGuessAfterFailedScan(t *testing.T) {
	got := UnsupportedAIMHardwareFamilies(
		"epyc,instinct",
		DetectedHardware{},
	)
	if len(got) != 0 {
		t.Fatalf("failed scans must not prove incompatibility, got %v", got)
	}
}
