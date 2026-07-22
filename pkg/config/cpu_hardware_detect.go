package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// FamilyEPYC is the AIM_HARDWARE_FAMILY value for AMD EPYC CPU-targeted AIM
// images. Unlike FamilyInstinct/FamilyRadeon, this is never a valid
// GPU_STACK_FAMILY value (ResolveStackProfile rejects it) — EPYC detection
// only ever feeds AIM_HARDWARE_FAMILY.
const FamilyEPYC = "epyc"

// cpuInfoContents is overridden in tests to avoid depending on the real
// /proc/cpuinfo of whatever machine runs the test.
var cpuInfoContents = func() (string, error) {
	data, err := os.ReadFile("/proc/cpuinfo")
	return string(data), err
}

// DetectAMDEPYCCPU checks whether this node's CPU is an AMD EPYC processor,
// via /proc/cpuinfo. Best-effort like DetectAMDGPUFamilies: any failure
// (unreadable /proc/cpuinfo, non-Linux, etc.) is returned as an error so
// callers can skip auto-detection rather than block an install.
func DetectAMDEPYCCPU() (detected bool, model string, err error) {
	contents, err := cpuInfoContents()
	if err != nil {
		return false, "", fmt.Errorf("read /proc/cpuinfo: %w", err)
	}
	detected, model = ParseCPUInfoForEPYC(contents)
	return detected, model, nil
}

// ParseCPUInfoForEPYC classifies /proc/cpuinfo contents as an AMD EPYC CPU or
// not, returning the reported model name for prompt/log text. Exported
// (pure, no I/O) so the classification logic can be unit tested against
// captured /proc/cpuinfo content. Only the vendor_id/model name of the first
// CPU entry is consulted: a node's CPUs are homogeneous, so checking one is
// sufficient.
func ParseCPUInfoForEPYC(cpuinfo string) (detected bool, model string) {
	isAMD := false
	modelName := ""
	scanner := bufio.NewScanner(strings.NewReader(cpuinfo))
	for scanner.Scan() {
		line := scanner.Text()
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "vendor_id":
			if value == "AuthenticAMD" {
				isAMD = true
			}
		case "model name":
			if modelName == "" {
				modelName = value
			}
		}
		if isAMD && modelName != "" {
			break
		}
	}
	// "EPYC" in the model name is itself an unambiguous AMD signal — no other
	// vendor uses the name — so we do NOT hard-require vendor_id ==
	// AuthenticAMD. Some hypervisors mask or omit vendor_id on virtualized
	// EPYC parts (including custom cloud SKUs such as "AMD EPYC 9J14"), and
	// gating on it there would wrongly skip a node whose model name plainly
	// says it's an EPYC. vendor_id (or "AMD" in the model string) is still
	// required as a guard so a stray "EPYC" elsewhere can't false-positive.
	modelUpper := strings.ToUpper(modelName)
	if strings.Contains(modelUpper, "EPYC") && (isAMD || strings.Contains(modelUpper, "AMD")) {
		return true, modelName
	}
	return false, ""
}
