package config

import (
	"strings"
	"testing"
)

func TestResolveStackProfile(t *testing.T) {
	tests := []struct {
		name       string
		family     string
		wantFamily string
		wantTP     bool
		wantErr    bool
	}{
		{name: "empty defaults to instinct", family: "", wantFamily: "instinct", wantTP: false},
		{name: "instinct explicit", family: "instinct", wantFamily: "instinct", wantTP: false},
		{name: "radeon tech preview", family: "radeon", wantFamily: "radeon", wantTP: true},
		{name: "unknown family errors", family: "epyc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := ResolveStackProfile(tt.family)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for family %q, got none", tt.family)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if profile.Family != tt.wantFamily {
				t.Errorf("family: got %q, want %q", profile.Family, tt.wantFamily)
			}
			if profile.TechPreview != tt.wantTP {
				t.Errorf("techPreview: got %v, want %v", profile.TechPreview, tt.wantTP)
			}
			if profile.HostRocmVersion == "" || profile.OperatorPath == "" || profile.DeviceConfigDriverVersion == "" {
				t.Errorf("profile has empty pins: %+v", profile)
			}
		})
	}
}

func TestInstinctMatchesExistingDefaults(t *testing.T) {
	// The instinct row must reproduce today's hardcoded defaults so existing
	// installs see no change.
	profile, err := ResolveStackProfile("instinct")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.HostRocmVersion != "7.1.1" {
		t.Errorf("instinct host ROCm: got %q, want 7.1.1", profile.HostRocmVersion)
	}
	if profile.HostRocmDebBuild != "70101-1" {
		t.Errorf("instinct deb build: got %q, want 70101-1", profile.HostRocmDebBuild)
	}
	if profile.OperatorPath != "amd-gpu-operator/v1.4.1" {
		t.Errorf("instinct operator path: got %q, want amd-gpu-operator/v1.4.1", profile.OperatorPath)
	}
	if profile.DeviceConfigDriverVersion != "7.0" {
		t.Errorf("instinct DeviceConfig driver: got %q, want 7.0", profile.DeviceConfigDriverVersion)
	}
}

func TestCheckRadeonSupportedRejectsTooOldRocm(t *testing.T) {
	// Guards the EAI-6030 unsupported-combination rule: radeon on ROCm 7.2.
	err := checkRadeonSupported(StackProfile{Family: "radeon", DeviceConfigDriverVersion: "7.2"})
	if err == nil {
		t.Fatal("expected radeon + ROCm 7.2 to be rejected")
	}
	if !strings.Contains(err.Error(), "radeon") || !strings.Contains(err.Error(), "too old") {
		t.Errorf("error should name radeon and 'too old': %v", err)
	}
}

func TestCheckRadeonSupportedAcceptsMinimum(t *testing.T) {
	if err := checkRadeonSupported(StackProfile{Family: "radeon", DeviceConfigDriverVersion: "7.13"}); err != nil {
		t.Errorf("radeon + ROCm 7.13 should be supported, got: %v", err)
	}
}

func TestApplyGPUStackVars(t *testing.T) {
	cfg := Config{"GPU_STACK_FAMILY": "instinct"}
	if err := ApplyGPUStackVars(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["rocm_required_version"] != "7.1.1" {
		t.Errorf("rocm_required_version: got %v, want 7.1.1", cfg["rocm_required_version"])
	}
	if cfg["gpu_operator_path"] != "amd-gpu-operator/v1.4.1" {
		t.Errorf("gpu_operator_path: got %v", cfg["gpu_operator_path"])
	}
	if cfg["gpu_stack_family_resolved"] != "instinct" {
		t.Errorf("gpu_stack_family_resolved: got %v", cfg["gpu_stack_family_resolved"])
	}
}

func TestApplyGPUStackVarsEmptyDefaultsInstinct(t *testing.T) {
	cfg := Config{}
	if err := ApplyGPUStackVars(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["gpu_stack_family_resolved"] != "instinct" {
		t.Errorf("empty family should resolve to instinct, got %v", cfg["gpu_stack_family_resolved"])
	}
}
