package config

import (
	"fmt"
	"strings"
	"testing"
)

// HostHardwareScan records hardware detection results and any scan errors.
type HostHardwareScan struct {
	Detected DetectedHardware
	GPUErr   error
	CPUErr   error
}

// ScanHostHardware detects AMD GPU and EPYC CPU families from the local host.
func ScanHostHardware() HostHardwareScan {
	scan := HostHardwareScan{}

	gpus, err := DetectAMDGPUFamilies()
	if err != nil {
		scan.GPUErr = err
	} else {
		scan.Detected.GPU = gpus
		scan.Detected.GPUScanSucceeded = true
	}

	epyc, model, err := DetectAMDEPYCCPU()
	if err != nil {
		scan.CPUErr = err
	} else {
		scan.Detected.CPUScanSucceeded = true
		if epyc {
			scan.Detected.EPYCModel = model
		}
	}

	return scan
}

// Warnings returns user-visible notes about incomplete or ambiguous detection.
func (s HostHardwareScan) Warnings() []string {
	var warnings []string
	if s.GPUErr != nil {
		warnings = append(warnings, fmt.Sprintf("AMD GPU detection unavailable (%v)", s.GPUErr))
	}
	if s.CPUErr != nil {
		warnings = append(warnings, fmt.Sprintf("AMD EPYC CPU detection unavailable (%v)", s.CPUErr))
	}
	if len(s.Detected.GPU.UnmappedDeviceIDs) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"AMD GPU device IDs are present but not mapped to an AIM family: %s",
			strings.Join(s.Detected.GPU.UnmappedDeviceIDs, ", "),
		))
	}
	return warnings
}

// FormatAIMHardwareFamily renders the default AIM model catalog for a host.
func FormatAIMHardwareFamily(detected DetectedHardware) string {
	return strings.Join(detected.DefaultAIMFamilies(), ",")
}

// ApplyAIMHardwareFamilyDefault sets AIM_HARDWARE_FAMILY when unset or empty.
func ApplyAIMHardwareFamilyDefault(cfg Config) (HostHardwareScan, bool) {
	value, _ := cfg["AIM_HARDWARE_FAMILY"].(string)
	if strings.TrimSpace(value) != "" {
		return ScanHostHardware(), false
	}

	scan := ScanHostHardware()
	cfg["AIM_HARDWARE_FAMILY"] = FormatAIMHardwareFamily(scan.Detected)
	return scan, true
}

// SetHardwareDetectHooksForTest overrides PCI and CPU detection for tests.
func SetHardwareDetectHooksForTest(
	t *testing.T,
	lspci func() (string, error),
	cpuinfo func() (string, error),
) {
	t.Helper()
	prevLspci := lspciOutput
	prevCPU := cpuInfoContents
	if lspci != nil {
		lspciOutput = lspci
	}
	if cpuinfo != nil {
		cpuInfoContents = cpuinfo
	}
	t.Cleanup(func() {
		lspciOutput = prevLspci
		cpuInfoContents = prevCPU
	})
}
