package config

import (
	"errors"
	"testing"
)

func TestParseCPUInfoForEPYCDetectsEPYC(t *testing.T) {
	cpuinfo := "processor\t: 0\n" +
		"vendor_id\t: AuthenticAMD\n" +
		"cpu family\t: 25\n" +
		"model name\t: AMD EPYC 9354 32-Core Processor\n" +
		"stepping\t: 1\n"

	detected, model := ParseCPUInfoForEPYC(cpuinfo)

	if !detected {
		t.Fatal("expected AMD EPYC to be detected")
	}
	if model != "AMD EPYC 9354 32-Core Processor" {
		t.Errorf("model = %q, want %q", model, "AMD EPYC 9354 32-Core Processor")
	}
}

func TestParseCPUInfoForEPYCDetectsCustomCloudSKU(t *testing.T) {
	// Semi-custom / hyperscaler EPYC SKUs (e.g. "9J14") have non-retail model
	// numbers but still carry "EPYC" in the name — must be detected, since we
	// match the name substring rather than a model-number whitelist.
	cpuinfo := "vendor_id\t: AuthenticAMD\n" +
		"model name\t: AMD EPYC 9J14 96-Core Processor\n"

	detected, model := ParseCPUInfoForEPYC(cpuinfo)

	if !detected || model != "AMD EPYC 9J14 96-Core Processor" {
		t.Errorf("detected=%v model=%q, want a custom EPYC SKU to be detected", detected, model)
	}
}

func TestParseCPUInfoForEPYCDetectsVirtualizedMaskedVendor(t *testing.T) {
	// Some hypervisors mask/omit vendor_id on virtualized EPYC parts. The
	// model name still identifies it as EPYC, so detection must not depend on
	// vendor_id == AuthenticAMD.
	cpuinfo := "processor\t: 0\n" +
		"vendor_id\t: KVMKVMKVM\n" +
		"model name\t: AMD EPYC 9J14 96-Core Processor\n"

	detected, model := ParseCPUInfoForEPYC(cpuinfo)

	if !detected || model != "AMD EPYC 9J14 96-Core Processor" {
		t.Errorf("detected=%v model=%q, want a masked-vendor virtualized EPYC to be detected", detected, model)
	}
}

func TestParseCPUInfoForEPYCIgnoresNonEPYCAMD(t *testing.T) {
	// An AMD CPU that isn't EPYC (e.g. Ryzen/Threadripper) must not match.
	cpuinfo := "vendor_id\t: AuthenticAMD\n" +
		"model name\t: AMD Ryzen 9 7950X 16-Core Processor\n"

	detected, model := ParseCPUInfoForEPYC(cpuinfo)

	if detected {
		t.Errorf("non-EPYC AMD CPU must not be detected as EPYC, got model %q", model)
	}
}

func TestParseCPUInfoForEPYCIgnoresIntel(t *testing.T) {
	cpuinfo := "vendor_id\t: GenuineIntel\n" +
		"model name\t: Intel(R) Xeon(R) Platinum 8358 CPU @ 2.60GHz\n"

	detected, _ := ParseCPUInfoForEPYC(cpuinfo)

	if detected {
		t.Error("Intel CPU must not be detected as AMD EPYC")
	}
}

func TestParseCPUInfoForEPYCEmpty(t *testing.T) {
	detected, model := ParseCPUInfoForEPYC("")
	if detected || model != "" {
		t.Errorf("empty input should detect nothing, got detected=%v model=%q", detected, model)
	}
}

func TestParseCPUInfoForEPYCOnlyChecksFirstCPUEntry(t *testing.T) {
	// Multiple "processor" blocks in /proc/cpuinfo (one per core/thread) —
	// only the first entry's vendor/model should be consulted.
	cpuinfo := "processor\t: 0\n" +
		"vendor_id\t: AuthenticAMD\n" +
		"model name\t: AMD EPYC 7763 64-Core Processor\n" +
		"\n" +
		"processor\t: 1\n" +
		"vendor_id\t: AuthenticAMD\n" +
		"model name\t: AMD EPYC 7763 64-Core Processor\n"

	detected, model := ParseCPUInfoForEPYC(cpuinfo)

	if !detected || model != "AMD EPYC 7763 64-Core Processor" {
		t.Errorf("detected=%v model=%q, want detected=true model=%q", detected, model, "AMD EPYC 7763 64-Core Processor")
	}
}

func TestDetectAMDEPYCCPUPropagatesReadFailure(t *testing.T) {
	original := cpuInfoContents
	defer func() { cpuInfoContents = original }()

	cpuInfoContents = func() (string, error) {
		return "", errors.New("open /proc/cpuinfo: no such file or directory")
	}

	_, _, err := DetectAMDEPYCCPU()
	if err == nil {
		t.Fatal("expected an error when /proc/cpuinfo is unreadable, got nil")
	}
}

func TestDetectAMDEPYCCPUUsesParsedContents(t *testing.T) {
	original := cpuInfoContents
	defer func() { cpuInfoContents = original }()

	cpuInfoContents = func() (string, error) {
		return "vendor_id\t: AuthenticAMD\nmodel name\t: AMD EPYC 9124 16-Core Processor\n", nil
	}

	detected, model, err := DetectAMDEPYCCPU()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detected || model != "AMD EPYC 9124 16-Core Processor" {
		t.Errorf("detected=%v model=%q", detected, model)
	}
}
