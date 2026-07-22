package config

import "testing"

func TestComputeFamilyDefaultsNoDetectionIsNoOp(t *testing.T) {
	cfg := Config{}
	got := ComputeFamilyDefaults(cfg, DetectedHardware{})

	if got.GPUStackFamily != "" || got.AIMHardwareFamily != "" || got.Ambiguous {
		t.Errorf("expected a no-op result when nothing was detected, got %+v", got)
	}
}

func TestComputeFamilyDefaultsSingleFamilyAutoAssignsBoth(t *testing.T) {
	cfg := Config{"GPU_STACK_FAMILY": "", "AIM_HARDWARE_FAMILY": ""}
	detected := DetectedHardware{GPU: DetectedGPUFamilies{Families: []string{FamilyRadeon}, Models: map[string][]string{FamilyRadeon: {"RX 9070"}}}}

	got := ComputeFamilyDefaults(cfg, detected)

	if got.GPUStackFamily != FamilyRadeon {
		t.Errorf("GPUStackFamily = %q, want %q", got.GPUStackFamily, FamilyRadeon)
	}
	if got.AIMHardwareFamily != FamilyRadeon {
		t.Errorf("AIMHardwareFamily = %q, want %q", got.AIMHardwareFamily, FamilyRadeon)
	}
	if got.Ambiguous {
		t.Error("single detected family must not be ambiguous")
	}
}

func TestComputeFamilyDefaultsMixedFamilyIsAmbiguousForStackOnly(t *testing.T) {
	cfg := Config{"GPU_STACK_FAMILY": "", "AIM_HARDWARE_FAMILY": ""}
	detected := DetectedHardware{GPU: DetectedGPUFamilies{Families: []string{FamilyInstinct, FamilyRadeon}}}

	got := ComputeFamilyDefaults(cfg, detected)

	if !got.Ambiguous {
		t.Error("mixed families with GPU_STACK_FAMILY unset must be ambiguous")
	}
	if got.GPUStackFamily != "" {
		t.Errorf("GPUStackFamily must stay empty when ambiguous, got %q", got.GPUStackFamily)
	}
	// AIM_HARDWARE_FAMILY is multi-select, so a mixed box is never
	// ambiguous there — it should just list every detected family.
	if got.AIMHardwareFamily != "instinct,radeon" {
		t.Errorf("AIMHardwareFamily = %q, want %q", got.AIMHardwareFamily, "instinct,radeon")
	}
}

func TestComputeFamilyDefaultsExplicitGPUStackFamilyNeverOverridden(t *testing.T) {
	cfg := Config{"GPU_STACK_FAMILY": "instinct", "AIM_HARDWARE_FAMILY": ""}
	// Even ambiguous hardware must not touch an explicitly-set GPU_STACK_FAMILY.
	detected := DetectedHardware{GPU: DetectedGPUFamilies{Families: []string{FamilyInstinct, FamilyRadeon}}}

	got := ComputeFamilyDefaults(cfg, detected)

	if got.GPUStackFamily != "" {
		t.Errorf("GPUStackFamily must stay empty (no override) when already set, got %q", got.GPUStackFamily)
	}
	if got.Ambiguous {
		t.Error("an explicitly-set GPU_STACK_FAMILY must never be reported as ambiguous")
	}
	if got.AIMHardwareFamily != "instinct,radeon" {
		t.Errorf("AIMHardwareFamily should still auto-populate independently, got %q", got.AIMHardwareFamily)
	}
}

func TestComputeFamilyDefaultsExplicitAIMHardwareFamilyNeverOverridden(t *testing.T) {
	cfg := Config{"GPU_STACK_FAMILY": "", "AIM_HARDWARE_FAMILY": "epyc"}
	detected := DetectedHardware{GPU: DetectedGPUFamilies{Families: []string{FamilyRadeon}}}

	got := ComputeFamilyDefaults(cfg, detected)

	if got.AIMHardwareFamily != "" {
		t.Errorf("AIMHardwareFamily must stay empty (no override) when already set, got %q", got.AIMHardwareFamily)
	}
	if got.GPUStackFamily != FamilyRadeon {
		t.Errorf("GPUStackFamily should still auto-assign independently, got %q", got.GPUStackFamily)
	}
	// Radeon was detected but the installer's explicit AIM_HARDWARE_FAMILY
	// only mentions epyc — this should be flagged, not silently dropped.
	if got.UnconfiguredDetectedAIMFamilies == nil || got.UnconfiguredDetectedAIMFamilies[0] != FamilyRadeon {
		t.Errorf("UnconfiguredDetectedAIMFamilies = %v, want [%q]", got.UnconfiguredDetectedAIMFamilies, FamilyRadeon)
	}
}

func TestComputeFamilyDefaultsMissingKeysTreatedAsUnset(t *testing.T) {
	// A cfg map that never had the keys inserted (rather than set to "")
	// must behave identically to an explicit empty string.
	cfg := Config{}
	detected := DetectedHardware{GPU: DetectedGPUFamilies{Families: []string{FamilyInstinct}}}

	got := ComputeFamilyDefaults(cfg, detected)

	if got.GPUStackFamily != FamilyInstinct {
		t.Errorf("GPUStackFamily = %q, want %q", got.GPUStackFamily, FamilyInstinct)
	}
	if got.AIMHardwareFamily != FamilyInstinct {
		t.Errorf("AIMHardwareFamily = %q, want %q", got.AIMHardwareFamily, FamilyInstinct)
	}
}

func TestComputeFamilyDefaultsEPYCOnlyAutoAssignsAIMFamilyOnly(t *testing.T) {
	// A CPU-only node (no AMD GPU) must still get AIM_HARDWARE_FAMILY
	// auto-populated with "epyc", and must never touch GPU_STACK_FAMILY
	// (epyc is not a valid ROCm/GPU Operator stack value).
	cfg := Config{}
	detected := DetectedHardware{EPYCModel: "AMD EPYC 9354 32-Core Processor"}

	got := ComputeFamilyDefaults(cfg, detected)

	if got.GPUStackFamily != "" || got.Ambiguous {
		t.Errorf("EPYC-only detection must never touch GPU_STACK_FAMILY, got GPUStackFamily=%q Ambiguous=%v", got.GPUStackFamily, got.Ambiguous)
	}
	if got.AIMHardwareFamily != FamilyEPYC {
		t.Errorf("AIMHardwareFamily = %q, want %q", got.AIMHardwareFamily, FamilyEPYC)
	}
}

func TestComputeFamilyDefaultsRadeonPlusEPYCAutoAssignsBothToAIMFamily(t *testing.T) {
	cfg := Config{}
	detected := DetectedHardware{
		GPU:       DetectedGPUFamilies{Families: []string{FamilyRadeon}, Models: map[string][]string{FamilyRadeon: {"RX 9070"}}},
		EPYCModel: "AMD EPYC 9124 16-Core Processor",
	}

	got := ComputeFamilyDefaults(cfg, detected)

	if got.GPUStackFamily != FamilyRadeon {
		t.Errorf("GPUStackFamily = %q, want %q (epyc has no bearing on the ROCm stack choice)", got.GPUStackFamily, FamilyRadeon)
	}
	if got.AIMHardwareFamily != "epyc,radeon" {
		t.Errorf("AIMHardwareFamily = %q, want %q", got.AIMHardwareFamily, "epyc,radeon")
	}
	if len(got.UnconfiguredDetectedAIMFamilies) != 0 {
		t.Errorf("no explicit AIM_HARDWARE_FAMILY was set, so nothing should be flagged as unconfigured, got %v", got.UnconfiguredDetectedAIMFamilies)
	}
}

func TestComputeFamilyDefaultsExplicitEPYCOnlyFlagsUndetectedRadeon(t *testing.T) {
	// The installer explicitly configured AIM_HARDWARE_FAMILY: "epyc" only,
	// but this box also has a Radeon GPU. Explicit config must win as-is
	// (AIMHardwareFamily stays "" — no override), but the gap must be
	// surfaced so the caller can notify the installer.
	cfg := Config{"AIM_HARDWARE_FAMILY": "epyc"}
	detected := DetectedHardware{
		GPU:       DetectedGPUFamilies{Families: []string{FamilyRadeon}, Models: map[string][]string{FamilyRadeon: {"RX 9070"}}},
		EPYCModel: "AMD EPYC 9124 16-Core Processor",
	}

	got := ComputeFamilyDefaults(cfg, detected)

	if got.AIMHardwareFamily != "" {
		t.Errorf("AIMHardwareFamily must stay empty (explicit config wins), got %q", got.AIMHardwareFamily)
	}
	if len(got.UnconfiguredDetectedAIMFamilies) != 1 || got.UnconfiguredDetectedAIMFamilies[0] != FamilyRadeon {
		t.Errorf("UnconfiguredDetectedAIMFamilies = %v, want [%q]", got.UnconfiguredDetectedAIMFamilies, FamilyRadeon)
	}
}

func TestComputeFamilyDefaultsExplicitConfigMatchingDetectionFlagsNothing(t *testing.T) {
	cfg := Config{"AIM_HARDWARE_FAMILY": "epyc,radeon"}
	detected := DetectedHardware{
		GPU:       DetectedGPUFamilies{Families: []string{FamilyRadeon}},
		EPYCModel: "AMD EPYC 9124 16-Core Processor",
	}

	got := ComputeFamilyDefaults(cfg, detected)

	if len(got.UnconfiguredDetectedAIMFamilies) != 0 {
		t.Errorf("explicit config already covers everything detected, want no notice, got %v", got.UnconfiguredDetectedAIMFamilies)
	}
}

func TestComputeFamilyDefaultsExplicitConfigWithExtraSpacesMatches(t *testing.T) {
	// AIM_HARDWARE_FAMILY: "epyc, radeon" (space after comma) must be
	// treated the same as "epyc,radeon" — no spurious notice.
	cfg := Config{"AIM_HARDWARE_FAMILY": "epyc, radeon"}
	detected := DetectedHardware{
		GPU:       DetectedGPUFamilies{Families: []string{FamilyRadeon}},
		EPYCModel: "AMD EPYC 9124 16-Core Processor",
	}

	got := ComputeFamilyDefaults(cfg, detected)

	if len(got.UnconfiguredDetectedAIMFamilies) != 0 {
		t.Errorf("whitespace around commas must not cause a spurious notice, got %v", got.UnconfiguredDetectedAIMFamilies)
	}
}

func TestDetectedHardwareAIMFamiliesSortedAndDeduped(t *testing.T) {
	d := DetectedHardware{
		GPU:       DetectedGPUFamilies{Families: []string{FamilyRadeon, FamilyInstinct}},
		EPYCModel: "AMD EPYC 7763 64-Core Processor",
	}

	got := d.AIMFamilies()
	want := []string{FamilyEPYC, FamilyInstinct, FamilyRadeon}

	if len(got) != len(want) {
		t.Fatalf("AIMFamilies() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AIMFamilies()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDetectedHardwareHasEPYC(t *testing.T) {
	if (DetectedHardware{}).HasEPYC() {
		t.Error("zero-value DetectedHardware must not report an EPYC CPU")
	}
	if !(DetectedHardware{EPYCModel: "AMD EPYC 9354 32-Core Processor"}).HasEPYC() {
		t.Error("expected HasEPYC to be true when EPYCModel is set")
	}
}
