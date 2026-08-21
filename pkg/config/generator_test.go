package config

import (
	"strings"
	"testing"
)

func TestGenerateYAMLAppliesAIMHardwareFamilyDefault(t *testing.T) {
	SetHardwareDetectHooksForTest(t,
		func() (string, error) { return "", nil },
		func() (string, error) {
			return "vendor_id : AuthenticAMD\nmodel name : AMD EPYC 9124 16-Core Processor\n", nil
		},
	)

	yaml := GenerateYAML(Config{})
	if !strings.Contains(yaml, `AIM_HARDWARE_FAMILY: "epyc"`) &&
		!strings.Contains(yaml, "AIM_HARDWARE_FAMILY: epyc") {
		t.Fatalf("yaml = %q", yaml)
	}
}

func TestPrepareGeneratedConfigPreservesExplicitAIMFamily(t *testing.T) {
	cfg := Config{"AIM_HARDWARE_FAMILY": "instinct"}
	prepared := PrepareGeneratedConfig(cfg)
	if prepared["AIM_HARDWARE_FAMILY"] != "instinct" {
		t.Fatalf("prepared = %#v", prepared["AIM_HARDWARE_FAMILY"])
	}
}
