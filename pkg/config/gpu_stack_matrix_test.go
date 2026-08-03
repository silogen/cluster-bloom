package config

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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
			if profile.DriverPackageVersion == "" || profile.OperatorPath == "" || profile.DeviceConfigDriverVersion == "" {
				t.Errorf("profile has empty pins: %+v", profile)
			}
		})
	}
}

func TestInstinctUsesProductionDriverDefault(t *testing.T) {
	profile, err := ResolveStackProfile("instinct")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.DriverPackageVersion != "31.40" {
		t.Errorf("instinct driver package version: got %q, want 31.40", profile.DriverPackageVersion)
	}
	if profile.DriverPackageBuild != "314000-1" {
		t.Errorf("instinct driver package build: got %q, want 314000-1", profile.DriverPackageBuild)
	}
	if profile.OperatorPath != "amd-gpu-operator/v1.4.1" {
		t.Errorf("instinct operator path: got %q, want amd-gpu-operator/v1.4.1", profile.OperatorPath)
	}
	if profile.OperatorConfigPath != "amd-gpu-operator-config/v1.4.1" {
		t.Errorf("instinct operator config path: got %q, want amd-gpu-operator-config/v1.4.1", profile.OperatorConfigPath)
	}
	if profile.DeviceConfigDriverVersion != "7.0" {
		t.Errorf("instinct DeviceConfig driver: got %q, want 7.0", profile.DeviceConfigDriverVersion)
	}
}

func TestRadeonSelectsBetaOperator(t *testing.T) {
	// Radeon must select the v1.5.1-beta.0 tech-preview chart, while the
	// instinct default stays on the qualified v1.4.1 chart.
	profile, err := ResolveStackProfile("radeon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if profile.OperatorPath != "amd-gpu-operator/v1.5.1-beta.0" {
		t.Errorf("radeon operator path: got %q, want amd-gpu-operator/v1.5.1-beta.0", profile.OperatorPath)
	}
	if profile.OperatorConfigPath != "amd-gpu-operator-config/v1.5.1-beta.0" {
		t.Errorf("radeon operator config path: got %q, want amd-gpu-operator-config/v1.5.1-beta.0", profile.OperatorConfigPath)
	}
	if profile.DriverPackageVersion != "31.40" {
		t.Errorf("radeon driver package version: got %q, want 31.40", profile.DriverPackageVersion)
	}
	if profile.DriverPackageBuild != "314000-1" {
		t.Errorf("radeon driver package build: got %q, want 314000-1", profile.DriverPackageBuild)
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
	if cfg["gpu_driver_default_version"] != "31.40" {
		t.Errorf("gpu_driver_default_version: got %v, want 31.40", cfg["gpu_driver_default_version"])
	}
	if cfg["gpu_driver_default_build"] != "314000-1" {
		t.Errorf("gpu_driver_default_build: got %v, want 314000-1", cfg["gpu_driver_default_build"])
	}
	if cfg["gpu_operator_path"] != "amd-gpu-operator/v1.4.1" {
		t.Errorf("gpu_operator_path: got %v", cfg["gpu_operator_path"])
	}
	if cfg["gpu_operator_config_path"] != "amd-gpu-operator-config/v1.4.1" {
		t.Errorf("gpu_operator_config_path: got %v", cfg["gpu_operator_config_path"])
	}
	if cfg["gpu_stack_family_resolved"] != "instinct" {
		t.Errorf("gpu_stack_family_resolved: got %v", cfg["gpu_stack_family_resolved"])
	}
}

func TestApplyGPUStackVarsRadeon(t *testing.T) {
	cfg := Config{"GPU_STACK_FAMILY": "radeon"}
	if err := ApplyGPUStackVars(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["gpu_driver_default_version"] != "31.40" {
		t.Errorf("gpu_driver_default_version: got %v, want 31.40", cfg["gpu_driver_default_version"])
	}
	if cfg["gpu_driver_default_build"] != "314000-1" {
		t.Errorf("gpu_driver_default_build: got %v, want 314000-1", cfg["gpu_driver_default_build"])
	}
}

func TestSupportedGPUDriversIncludesValidatedTuples(t *testing.T) {
	want := map[string]string{
		"30.10.2": "7.0.2",
		"30.20.1": "7.1.1",
		"30.30.3": "7.2.3",
		"30.30.4": "7.2.4",
		"31.30.0": "7.13.0",
		"31.40.0": "7.14.0",
	}
	for _, driver := range supportedGPUDrivers {
		if paired, ok := want[driver.DriverRelease]; !ok {
			t.Errorf("unexpected driver release in support table: %s", driver.DriverRelease)
		} else if driver.PairedROCm != paired {
			t.Errorf("%s paired ROCm: got %q, want %q", driver.DriverRelease, driver.PairedROCm, paired)
		}
		if driver.DKMSModuleVersion == "" || driver.DKMSPackageCode == "" || driver.HostToolsPackage == "" {
			t.Errorf("incomplete support tuple: %+v", driver)
		}
		delete(want, driver.DriverRelease)
	}
	if len(want) != 0 {
		t.Errorf("support table is missing tuples: %v", want)
	}
}

func TestValidateGPUDriverInstallerTuple(t *testing.T) {
	for _, driver := range supportedGPUDrivers {
		t.Run(driver.DriverRelease, func(t *testing.T) {
			errors := Validate(Config{
				"GPU_NODE":           true,
				"GPU_DRIVER_VERSION": driver.InstallerVersion,
				"GPU_DRIVER_BUILD":   driver.InstallerBuild,
			})
			if len(errors) != 0 {
				t.Fatalf("validated tuple returned errors: %v", errors)
			}
		})
	}
}

func TestValidateGPUDriverInstallerTupleRejectsPartialOverride(t *testing.T) {
	errors := Validate(Config{
		"GPU_NODE":           true,
		"GPU_DRIVER_VERSION": "31.40",
	})
	if len(errors) == 0 || !strings.Contains(strings.Join(errors, "\n"), "must be set together") {
		t.Fatalf("expected companion-field error, got: %v", errors)
	}
}

func TestValidateGPUDriverInstallerTupleRejectsUnsupportedPair(t *testing.T) {
	errors := Validate(Config{
		"GPU_NODE":           true,
		"GPU_DRIVER_VERSION": "7.2.4",
		"GPU_DRIVER_BUILD":   "314000-1",
	})
	combined := strings.Join(errors, "\n")
	if !strings.Contains(combined, "unsupported GPU driver installer tuple") {
		t.Fatalf("expected unsupported-tuple error, got: %v", errors)
	}
	if !strings.Contains(combined, "31.40 / 314000-1 (AMD driver 31.40.0)") {
		t.Errorf("error should list supported tuples, got: %v", errors)
	}
}

func TestValidateGPUDriverInstallerTupleAcceptsEmptyDefaults(t *testing.T) {
	errors := Validate(Config{
		"GPU_NODE":           true,
		"GPU_DRIVER_VERSION": "",
		"GPU_DRIVER_BUILD":   "",
	})
	if len(errors) != 0 {
		t.Fatalf("empty overrides should select defaults, got: %v", errors)
	}
}

func TestDriverCompatibilityPreservesAnsibleFieldNames(t *testing.T) {
	driver := supportedGPUDrivers[0]

	jsonBytes, err := json.Marshal(driver)
	if err != nil {
		t.Fatalf("marshal driver as JSON: %v", err)
	}
	var jsonFields map[string]any
	if err := json.Unmarshal(jsonBytes, &jsonFields); err != nil {
		t.Fatalf("unmarshal driver JSON: %v", err)
	}

	yamlBytes, err := yaml.Marshal(driver)
	if err != nil {
		t.Fatalf("marshal driver as YAML: %v", err)
	}
	var yamlFields map[string]any
	if err := yaml.Unmarshal(yamlBytes, &yamlFields); err != nil {
		t.Fatalf("unmarshal driver YAML: %v", err)
	}

	for _, fields := range []map[string]any{jsonFields, yamlFields} {
		if fields["DriverRelease"] != driver.DriverRelease {
			t.Errorf("DriverRelease missing after serialization: %#v", fields)
		}
		if fields["DKMSModuleVersion"] != driver.DKMSModuleVersion {
			t.Errorf("DKMSModuleVersion missing after serialization: %#v", fields)
		}
		if _, exists := fields["driverrelease"]; exists {
			t.Errorf("unexpected lowercase driverrelease key after serialization: %#v", fields)
		}
	}

	cfg := Config{}
	if err := ApplyGPUStackVars(cfg); err != nil {
		t.Fatalf("apply GPU stack vars: %v", err)
	}
	exportedVars, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal exported vars: %v", err)
	}
	var exportedConfig map[string]any
	if err := yaml.Unmarshal(exportedVars, &exportedConfig); err != nil {
		t.Fatalf("unmarshal exported vars: %v", err)
	}
	exportedDrivers, ok := exportedConfig["gpu_driver_supported"].([]any)
	if !ok || len(exportedDrivers) == 0 {
		t.Fatalf("exported driver tuples missing: %#v", exportedConfig["gpu_driver_supported"])
	}
	exportedDriver, ok := exportedDrivers[0].(map[string]any)
	if !ok {
		t.Fatalf("exported driver tuple has unexpected type: %#v", exportedDrivers[0])
	}
	if exportedDriver["DriverRelease"] != driver.DriverRelease {
		t.Errorf("exported DriverRelease missing: %#v", exportedDriver)
	}
	if _, exists := exportedDriver["driverrelease"]; exists {
		t.Errorf("exported tuple contains lowercase driverrelease: %#v", exportedDriver)
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
