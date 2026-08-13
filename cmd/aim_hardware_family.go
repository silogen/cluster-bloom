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
	report := aimHardwareFamilyReport{
		WasExplicit: strings.TrimSpace(configStringValue(cfg, "AIM_HARDWARE_FAMILY")) != "",
	}

	gpus, err := config.DetectAMDGPUFamilies()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Note: AMD GPU detection unavailable (%v)\n", err)
		}
	} else {
		report.Detected.GPU = gpus
		report.Detected.GPUScanSucceeded = true
	}

	epyc, model, err := config.DetectAMDEPYCCPU()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Note: AMD EPYC CPU detection unavailable (%v)\n", err)
		}
	} else {
		report.Detected.CPUScanSucceeded = true
		if epyc {
			report.Detected.EPYCModel = model
		}
	}

	if !report.WasExplicit {
		family := config.DefaultAIMHardwareFamily(report.Detected)
		cfg["AIM_HARDWARE_FAMILY"] = family
		fmt.Printf("🔎 AIM_HARDWARE_FAMILY=%s (auto-detected from host hardware)\n", family)
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
