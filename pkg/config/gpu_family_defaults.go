package config

import (
	"sort"
	"strings"
)

// DetectedHardware combines the AMD GPU PCI scan with AMD EPYC CPU
// detection: the full picture ComputeFamilyDefaults needs to fill in
// AIM_HARDWARE_FAMILY. GPU_STACK_FAMILY only ever depends on the GPU side —
// "epyc" is not a valid GPU_STACK_FAMILY value (ResolveStackProfile rejects
// it), since it names a CPU-only AIM target, not a ROCm/GPU Operator stack.
type DetectedHardware struct {
	GPU DetectedGPUFamilies
	// EPYCModel is the reported CPU model name if this node's CPU is an AMD
	// EPYC part, or "" if not detected.
	EPYCModel string
}

// HasEPYC reports whether an AMD EPYC CPU was detected.
func (d DetectedHardware) HasEPYC() bool {
	return d.EPYCModel != ""
}

// AIMFamilies returns every family suitable for AIM_HARDWARE_FAMILY
// auto-population — the detected GPU families plus "epyc" if this node's
// CPU is an AMD EPYC part — sorted for deterministic output.
func (d DetectedHardware) AIMFamilies() []string {
	if !d.HasEPYC() {
		return d.GPU.Families
	}
	families := append(append([]string{}, d.GPU.Families...), FamilyEPYC)
	sort.Strings(families)
	return families
}

// FamilyDefaults is the outcome of applying detected hardware to the family
// config values an installer left unset. GPU_STACK_FAMILY is single-select
// (host ROCm targets one family per node), so a node with GPUs from more
// than one family cannot be resolved here — Ambiguous is set instead, and
// the caller must obtain an explicit choice (e.g. an interactive prompt)
// before setting GPU_STACK_FAMILY.
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
	// design, so a mixed-family node just gets every detected family listed
	// (GPU families plus "epyc" when an AMD EPYC CPU was detected).
	AIMHardwareFamily string
	// UnconfiguredDetectedAIMFamilies lists hardware families that were
	// detected but are NOT present in an explicitly-set AIM_HARDWARE_FAMILY.
	// Explicit config always wins and is never overridden, but a
	// deliberately narrow config (e.g. AIM_HARDWARE_FAMILY: "epyc" on a box
	// that also has a Radeon GPU) may be an oversight, so callers should
	// surface this as an informational notice rather than silently
	// proceeding. Empty when AIM_HARDWARE_FAMILY was unset (nothing to
	// compare against) or when detection matches the explicit config.
	UnconfiguredDetectedAIMFamilies []string
	// GPUStackFamilyConflict is true when GPU_STACK_FAMILY was set explicitly
	// to a family that is NOT among the GPU families actually detected on this
	// node (and at least one GPU family WAS detected). Host ROCm and the GPU
	// Operator target this single family, so a mismatch most likely installs
	// the wrong driver stack for the hardware present — callers should surface
	// this as a warning. Never set when no GPU was detected: detection is
	// best-effort, so an empty scan (container, missing pciutils, etc.) is not
	// proof of a conflict.
	GPUStackFamilyConflict bool
	// ConfiguredAIMFamiliesNotDetected lists families present in an
	// explicitly-set AIM_HARDWARE_FAMILY that were NOT detected on this node.
	// Unlike a GPU_STACK_FAMILY conflict this is only informational:
	// AIM_HARDWARE_FAMILY is a cluster-wide model catalog and may legitimately
	// list hardware that lives on *other* nodes. Empty when AIM_HARDWARE_FAMILY
	// was unset or nothing was detected (best-effort: an empty scan is not
	// proof the configured hardware is absent).
	ConfiguredAIMFamiliesNotDetected []string
	// Detected is the hardware scan this decision was computed from, kept
	// around so callers can build prompt/log text without re-running
	// detection.
	Detected DetectedHardware
}

// ComputeFamilyDefaults decides how detected hardware should fill in
// GPU_STACK_FAMILY / AIM_HARDWARE_FAMILY, given the config values the
// installer already set in cfg (which are never overridden here). Pure
// function: takes an already-run detection result so the decision logic is
// unit-testable without shelling out to lspci or reading /proc/cpuinfo.
func ComputeFamilyDefaults(cfg Config, detected DetectedHardware) FamilyDefaults {
	result := FamilyDefaults{Detected: detected}

	existingGPUStack := configString(cfg, "GPU_STACK_FAMILY")
	if len(detected.GPU.Families) > 0 {
		switch {
		case existingGPUStack == "":
			if len(detected.GPU.Families) == 1 {
				result.GPUStackFamily = detected.GPU.Families[0]
			} else {
				result.Ambiguous = true
			}
		case !contains(detected.GPU.Families, existingGPUStack):
			// Explicit value is never overridden, but a family the node
			// doesn't actually have means the wrong host ROCm/GPU Operator
			// stack — worth a warning.
			result.GPUStackFamilyConflict = true
		}
	}

	aimFamilies := detected.AIMFamilies()
	if len(aimFamilies) == 0 {
		return result
	}

	existingAIM := configString(cfg, "AIM_HARDWARE_FAMILY")
	if existingAIM == "" {
		result.AIMHardwareFamily = strings.Join(aimFamilies, ",")
		return result
	}

	configured := map[string]bool{}
	for _, f := range strings.Split(existingAIM, ",") {
		if f = strings.TrimSpace(f); f != "" {
			configured[f] = true
		}
	}
	detectedAIM := map[string]bool{}
	for _, f := range aimFamilies {
		detectedAIM[f] = true
		if !configured[f] {
			result.UnconfiguredDetectedAIMFamilies = append(result.UnconfiguredDetectedAIMFamilies, f)
		}
	}
	for _, f := range sortedKeys(configured) {
		if !detectedAIM[f] {
			result.ConfiguredAIMFamiliesNotDetected = append(result.ConfiguredAIMFamiliesNotDetected, f)
		}
	}

	return result
}

// sortedKeys returns the keys of a set in deterministic order, so warning
// text (e.g. ConfiguredAIMFamiliesNotDetected) is stable across runs.
func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
