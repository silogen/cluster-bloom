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
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

type amdGPUDevice struct {
	Family string
	Model  string
}

// Keep this taxonomy aligned with cluster-forge's amd-gpu NFD rules.
var amdGPUDevicesByID = map[string]amdGPUDevice{
	"7410": {FamilyInstinct, "MI210 VF"},
	"74b5": {FamilyInstinct, "MI300X VF"},
	"74bd": {FamilyInstinct, "MI300X HF VF"},
	"74b6": {FamilyInstinct, "MI308X VF"},
	"74bc": {FamilyInstinct, "MI308X HF VF"},
	"74b9": {FamilyInstinct, "MI325X VF"},
	"75b8": {FamilyInstinct, "MI350P VF"},
	"75b0": {FamilyInstinct, "MI350X VF"},
	"75b3": {FamilyInstinct, "MI355X VF"},
	"75a3": {FamilyInstinct, "MI355X"},
	"75a0": {FamilyInstinct, "MI350X"},
	"75a8": {FamilyInstinct, "MI350P"},
	"74a5": {FamilyInstinct, "MI325X"},
	"74a2": {FamilyInstinct, "MI308X"},
	"74a8": {FamilyInstinct, "MI308X HF"},
	"74a0": {FamilyInstinct, "MI300A"},
	"74a1": {FamilyInstinct, "MI300X"},
	"74a9": {FamilyInstinct, "MI300X HF"},
	"740f": {FamilyInstinct, "MI210"},
	"7408": {FamilyInstinct, "MI250X"},
	"740c": {FamilyInstinct, "MI250/MI250X"},
	"738c": {FamilyInstinct, "MI100"},
	"738e": {FamilyInstinct, "MI100"},

	"7461": {FamilyRadeon, "Radeon Pro V710 MxGPU"},
	"73ae": {FamilyRadeon, "Radeon Pro V620 MxGPU"},
	"7460": {FamilyRadeon, "V710"},
	"7448": {FamilyRadeon, "W7900"},
	"744b": {FamilyRadeon, "W7900D"},
	"744a": {FamilyRadeon, "W7900 Dual Slot"},
	"7449": {FamilyRadeon, "W7800 48GB"},
	"745e": {FamilyRadeon, "W7800"},
	"73a2": {FamilyRadeon, "W6900X"},
	"73a3": {FamilyRadeon, "W6800 GL-XL"},
	"73ab": {FamilyRadeon, "W6800X / W6800X Duo"},
	"73a1": {FamilyRadeon, "V620"},
	"7551": {FamilyRadeon, "AI PRO R9700 / R9700S / R9600D"},
	"7550": {FamilyRadeon, "RX 9070 / 9070 XT"},
	"744c": {FamilyRadeon, "RX 7900 XT / 7900 XTX / 7900 GRE / 7900M"},
	"73af": {FamilyRadeon, "RX 6900 XT"},
	"73bf": {FamilyRadeon, "RX 6800 / 6800 XT / 6900 XT"},
	"7590": {FamilyRadeon, "RX 9060 XT"},
}

// Match a known AMD device ID on an lspci device line. Device IDs, rather
// than PCI classes, are authoritative because GPU VFs and headless devices
// can report classes other than VGA or 3D controller.
var amdPCIDeviceLine = regexp.MustCompile(`\[[0-9a-fA-F]{4}]:.*\[1002:([0-9a-fA-F]{4})]`)

type DetectedGPUFamilies struct {
	Families          []string
	Models            map[string][]string
	UnmappedDeviceIDs []string
}

func (d DetectedGPUFamilies) DescribeFamily(family string) string {
	return strings.Join(d.Models[family], ", ")
}

var lspciOutput = func() (string, error) {
	out, err := exec.Command("lspci", "-nn", "-d", "1002:").Output()
	return string(out), err
}

// DetectAMDGPUFamilies identifies supported AMD GPU families from PCI
// hardware. It does not require an installed amdgpu driver or host ROCm.
func DetectAMDGPUFamilies() (DetectedGPUFamilies, error) {
	out, err := lspciOutput()
	if err != nil {
		return DetectedGPUFamilies{}, fmt.Errorf("lspci: %w", err)
	}
	return ParseLspciAMDOutput(out), nil
}

func ParseLspciAMDOutput(output string) DetectedGPUFamilies {
	result := DetectedGPUFamilies{Models: map[string][]string{}}
	seen := map[string]bool{}
	unmapped := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		match := amdPCIDeviceLine.FindStringSubmatch(scanner.Text())
		if match == nil {
			continue
		}
		deviceID := strings.ToLower(match[1])
		device, ok := amdGPUDevicesByID[deviceID]
		if !ok {
			unmapped[deviceID] = true
			continue
		}
		key := device.Family + "/" + device.Model
		if seen[key] {
			continue
		}
		seen[key] = true
		result.Models[device.Family] = append(result.Models[device.Family], device.Model)
	}
	for deviceID := range unmapped {
		result.UnmappedDeviceIDs = append(result.UnmappedDeviceIDs, deviceID)
	}
	sort.Strings(result.UnmappedDeviceIDs)
	for family := range result.Models {
		result.Families = append(result.Families, family)
	}
	sort.Strings(result.Families)
	return result
}
