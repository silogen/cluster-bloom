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

func TestParseLspciAMDOutput(t *testing.T) {
	output := `0000:03:00.0 Processing accelerators [1200]: AMD/ATI Device [1002:74a1]
0000:03:00.1 Audio device [0403]: AMD/ATI Device [1002:1640]
0000:0a:00.0 VGA compatible controller [0300]: AMD/ATI Device [1002:7550] (rev c1)
0000:0b:00.0 Display controller [0380]: AMD/ATI Device [1002:7550]
`

	got := ParseLspciAMDOutput(output)
	if !reflect.DeepEqual(got.Families, []string{FamilyInstinct, FamilyRadeon}) {
		t.Fatalf("families = %v", got.Families)
	}
	if !reflect.DeepEqual(got.Models[FamilyInstinct], []string{"MI300X"}) {
		t.Fatalf("instinct models = %v", got.Models[FamilyInstinct])
	}
	if !reflect.DeepEqual(got.Models[FamilyRadeon], []string{"RX 9070 / 9070 XT"}) {
		t.Fatalf("radeon models = %v", got.Models[FamilyRadeon])
	}
}

func TestParseLspciAMDOutputIgnoresUnknownAMDDevices(t *testing.T) {
	got := ParseLspciAMDOutput(
		"0000:03:00.0 VGA compatible controller [0300]: AMD/ATI Device [1002:ffff]\n",
	)
	if len(got.Families) != 0 {
		t.Fatalf("families = %v, want none", got.Families)
	}
	if !reflect.DeepEqual(got.UnmappedDeviceIDs, []string{"ffff"}) {
		t.Fatalf("unmapped = %v, want [ffff]", got.UnmappedDeviceIDs)
	}
}
