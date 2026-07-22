package config

import "strings"

// FamilyDefaults is the outcome of applying detected GPU hardware to the
// family config values an installer left unset. GPU_STACK_FAMILY is
// single-select (host ROCm targets one family per node), so a node with GPUs
// from more than one family cannot be resolved here — Ambiguous is set
// instead, and the caller must obtain an explicit choice (e.g. an
// interactive prompt) before setting GPU_STACK_FAMILY.
type FamilyDefaults struct {
	// GPUStackFamily is the value to auto-assign when non-empty. Empty means
	// "leave GPU_STACK_FAMILY as-is": either the installer already set it
	// explicitly, nothing was detected, or the detection was ambiguous (see
	// Ambiguous).
	GPUStackFamily string
	// Ambiguous is true when GPU_STACK_FAMILY was left unset by the
	// installer AND the node has GPUs from more than one family: an explicit
	// choice is required, since silently picking one risks installing the
	// wrong ROCm/GPU Operator stack for hardware the installer didn't intend
	// to target.
	Ambiguous bool
	// AIMHardwareFamily is the value to auto-assign when non-empty. Empty
	// means "leave AIM_HARDWARE_FAMILY as-is". Unlike GPU_STACK_FAMILY, this
	// is never ambiguous: the AIM model catalog is a comma-separated list by
	// design, so a mixed-family node just gets every detected family listed.
	AIMHardwareFamily string
	// Detected is the hardware scan this decision was computed from, kept
	// around so callers can build prompt/log text without re-running
	// detection.
	Detected DetectedGPUFamilies
}

// ComputeFamilyDefaults decides how detected hardware should fill in
// GPU_STACK_FAMILY / AIM_HARDWARE_FAMILY, given the config values the
// installer already set in cfg (which are never overridden here). Pure
// function: takes an already-run detection result so the decision logic is
// unit-testable without shelling out to lspci.
func ComputeFamilyDefaults(cfg Config, detected DetectedGPUFamilies) FamilyDefaults {
	result := FamilyDefaults{Detected: detected}
	if len(detected.Families) == 0 {
		return result
	}

	if configString(cfg, "GPU_STACK_FAMILY") == "" {
		if len(detected.Families) == 1 {
			result.GPUStackFamily = detected.Families[0]
		} else {
			result.Ambiguous = true
		}
	}

	if configString(cfg, "AIM_HARDWARE_FAMILY") == "" {
		result.AIMHardwareFamily = strings.Join(detected.Families, ",")
	}

	return result
}

// configString reads a string config value, tolerating an absent, nil, or
// non-string entry (returns "" rather than panicking on a type assertion).
func configString(cfg Config, key string) string {
	v, ok := cfg[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
