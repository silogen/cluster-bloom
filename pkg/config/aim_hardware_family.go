package config

import (
	"sort"
	"strings"
)

// DetectedHardware records both the detection result and whether each scan
// completed reliably. A failed best-effort scan must not be treated as proof
// that configured hardware is absent.
type DetectedHardware struct {
	GPU              DetectedGPUFamilies
	GPUScanSucceeded bool
	EPYCModel        string
	CPUScanSucceeded bool
}

func (d DetectedHardware) HasEPYC() bool {
	return d.EPYCModel != ""
}

// DefaultAIMFamilies returns the most specific model families supported by
// this host. Generic CPU is the safe fallback when no optimized AMD family is
// detected.
func (d DetectedHardware) DefaultAIMFamilies() []string {
	families := append([]string{}, d.GPU.Families...)
	if d.HasEPYC() {
		families = append(families, FamilyEPYC)
	}
	if len(families) == 0 {
		return []string{FamilyCPU}
	}
	sort.Strings(families)
	return families
}

func DefaultAIMHardwareFamily(detected DetectedHardware) string {
	return strings.Join(detected.DefaultAIMFamilies(), ",")
}

// UnsupportedAIMHardwareFamilies returns explicitly requested model families
// for which this host has no compatible hardware. Generic CPU models are
// compatible with every host. Unknown scan results are not reported as
// mismatches.
func UnsupportedAIMHardwareFamilies(value string, detected DetectedHardware) []string {
	detectedGPU := make(map[string]bool, len(detected.GPU.Families))
	for _, family := range detected.GPU.Families {
		detectedGPU[family] = true
	}

	unsupported := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		family := strings.TrimSpace(item)
		switch family {
		case FamilyCPU, "":
			continue
		case FamilyEPYC:
			if detected.CPUScanSucceeded && !detected.HasEPYC() {
				unsupported[family] = true
			}
		case FamilyInstinct, FamilyRadeon:
			if detected.GPUScanSucceeded && !detectedGPU[family] {
				unsupported[family] = true
			}
		}
	}

	result := make([]string, 0, len(unsupported))
	for family := range unsupported {
		result = append(result, family)
	}
	sort.Strings(result)
	return result
}
