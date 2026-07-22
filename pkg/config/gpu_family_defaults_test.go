package config

import "testing"

func TestComputeFamilyDefaultsNoDetectionIsNoOp(t *testing.T) {
	cfg := Config{}
	got := ComputeFamilyDefaults(cfg, DetectedGPUFamilies{})

	if got.GPUStackFamily != "" || got.AIMHardwareFamily != "" || got.Ambiguous {
		t.Errorf("expected a no-op result when nothing was detected, got %+v", got)
	}
}

func TestComputeFamilyDefaultsSingleFamilyAutoAssignsBoth(t *testing.T) {
	cfg := Config{"GPU_STACK_FAMILY": "", "AIM_HARDWARE_FAMILY": ""}
	detected := DetectedGPUFamilies{Families: []string{FamilyRadeon}, Models: map[string][]string{FamilyRadeon: {"RX 9070"}}}

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
	detected := DetectedGPUFamilies{Families: []string{FamilyInstinct, FamilyRadeon}}

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
	detected := DetectedGPUFamilies{Families: []string{FamilyInstinct, FamilyRadeon}}

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
	detected := DetectedGPUFamilies{Families: []string{FamilyRadeon}}

	got := ComputeFamilyDefaults(cfg, detected)

	if got.AIMHardwareFamily != "" {
		t.Errorf("AIMHardwareFamily must stay empty (no override) when already set, got %q", got.AIMHardwareFamily)
	}
	if got.GPUStackFamily != FamilyRadeon {
		t.Errorf("GPUStackFamily should still auto-assign independently, got %q", got.GPUStackFamily)
	}
}

func TestComputeFamilyDefaultsMissingKeysTreatedAsUnset(t *testing.T) {
	// A cfg map that never had the keys inserted (rather than set to "")
	// must behave identically to an explicit empty string.
	cfg := Config{}
	detected := DetectedGPUFamilies{Families: []string{FamilyInstinct}}

	got := ComputeFamilyDefaults(cfg, detected)

	if got.GPUStackFamily != FamilyInstinct {
		t.Errorf("GPUStackFamily = %q, want %q", got.GPUStackFamily, FamilyInstinct)
	}
	if got.AIMHardwareFamily != FamilyInstinct {
		t.Errorf("AIMHardwareFamily = %q, want %q", got.AIMHardwareFamily, FamilyInstinct)
	}
}
