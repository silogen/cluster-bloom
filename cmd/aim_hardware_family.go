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
	"os"
	"strings"

	"github.com/silogen/cluster-bloom/pkg/config"
)

type aimHardwareFamilyReport struct {
	Detected    config.DetectedHardware
	WasExplicit bool
}

func resolveAIMHardwareFamilyDefault(cfg config.Config) aimHardwareFamilyReport {
	wasExplicit := strings.TrimSpace(configStringValue(cfg, "AIM_HARDWARE_FAMILY")) != ""
	scan, applied := config.ApplyAIMHardwareFamilyDefault(cfg)

	report := aimHardwareFamilyReport{
		Detected:    scan.Detected,
		WasExplicit: wasExplicit,
	}

	for _, warning := range scan.Warnings() {
		fmt.Fprintf(os.Stderr, "Note: %s\n", warning)
	}

	if applied {
		family := configStringValue(cfg, "AIM_HARDWARE_FAMILY")
		fmt.Printf("🔎 AIM_HARDWARE_FAMILY=%s (auto-detected from host hardware", family)
		details := scan.Detected.Describe()
		if details != "" {
			fmt.Printf(": %s", details)
		}
		fmt.Println(")")
	}

	return report
}

func confirmAIMHardwareFamilyCompatibility(
	cfg config.Config,
	report aimHardwareFamilyReport,
) bool {
	if !report.WasExplicit {
		return true
	}

	value := configStringValue(cfg, "AIM_HARDWARE_FAMILY")
	unsupported := config.UnsupportedAIMHardwareFamilies(value, report.Detected)
	if len(unsupported) == 0 {
		return true
	}

	fmt.Println()
	fmt.Printf("⚠️  AIM_HARDWARE_FAMILY=%q includes %s, but compatible hardware was not detected on this host.\n",
		value, strings.Join(unsupported, ", "))
	fmt.Println("   Models for those families will appear undeployable in the UI unless compatible")
	fmt.Println("   hardware is available on another node in this cluster.")
	return confirmYesNo("Continue with this AIM model catalog?")
}

func shouldGateAIMHardwareFamily(dryRun bool, tags string) bool {
	if dryRun {
		return false
	}
	if strings.TrimSpace(tags) == "" {
		return true
	}
	for _, tag := range strings.Split(tags, ",") {
		if strings.TrimSpace(tag) == "deploy_clusterforge" {
			return true
		}
	}
	return false
}

func configStringValue(cfg config.Config, key string) string {
	value, _ := cfg[key].(string)
	return value
}
