package config

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseLspciAMDOutputInstinctOnly(t *testing.T) {
	output := "0000:03:00.0 Processing accelerators [1200]: Advanced Micro Devices, Inc. [AMD/ATI] Aldebaran/MI210 [1002:740f]\n" +
		"0000:04:00.0 Audio device [0403]: Advanced Micro Devices, Inc. [AMD/ATI] Device [1002:1640]\n"

	got := ParseLspciAMDOutput(output)

	if !reflect.DeepEqual(got.Families, []string{FamilyInstinct}) {
		t.Fatalf("Families = %v, want [%s]", got.Families, FamilyInstinct)
	}
	if got.Ambiguous() {
		t.Error("single family should not be ambiguous")
	}
	if got.DescribeFamily(FamilyInstinct) != "MI210" {
		t.Errorf("DescribeFamily(instinct) = %q, want %q", got.DescribeFamily(FamilyInstinct), "MI210")
	}
}

func TestParseLspciAMDOutputRadeonOnly(t *testing.T) {
	output := "0000:0a:00.0 VGA compatible controller [0300]: Advanced Micro Devices, Inc. [AMD/ATI] Navi 48 [Radeon RX 9070/9070 XT] [1002:7550]\n"

	got := ParseLspciAMDOutput(output)

	if !reflect.DeepEqual(got.Families, []string{FamilyRadeon}) {
		t.Fatalf("Families = %v, want [%s]", got.Families, FamilyRadeon)
	}
	if got.Ambiguous() {
		t.Error("single family should not be ambiguous")
	}
}

func TestParseLspciAMDOutputMixedFamiliesIsAmbiguous(t *testing.T) {
	// The exact "node ambiguity" scenario: an Instinct accelerator and a
	// Radeon card physically present in the same box.
	output := "0000:03:00.0 Processing accelerators [1200]: Advanced Micro Devices, Inc. [AMD/ATI] Aldebaran/MI300X [1002:74a1]\n" +
		"0000:0a:00.0 VGA compatible controller [0300]: Advanced Micro Devices, Inc. [AMD/ATI] Navi 48 [Radeon RX 9070/9070 XT] [1002:7550]\n"

	got := ParseLspciAMDOutput(output)

	if !got.Ambiguous() {
		t.Fatalf("expected mixed instinct+radeon hardware to be ambiguous, got Families=%v", got.Families)
	}
	if len(got.Families) != 2 {
		t.Fatalf("Families = %v, want both instinct and radeon", got.Families)
	}
	if got.DescribeFamily(FamilyInstinct) != "MI300X" {
		t.Errorf("DescribeFamily(instinct) = %q, want %q", got.DescribeFamily(FamilyInstinct), "MI300X")
	}
	if got.DescribeFamily(FamilyRadeon) != "RX 9070 / 9070 XT" {
		t.Errorf("DescribeFamily(radeon) = %q, want %q", got.DescribeFamily(FamilyRadeon), "RX 9070 / 9070 XT")
	}
}

func TestParseLspciAMDOutputMatchesRevisionSuffix(t *testing.T) {
	// lspci -nn appends "(rev NN)" for any non-zero PCI revision, which is
	// common on AMD GPUs. A trailing revision must not defeat detection (a
	// previous `\s*$`-anchored regex silently missed these, falling back to
	// the instinct default even on a radeon node).
	output := "0000:03:00.0 VGA compatible controller [0300]: Advanced Micro Devices, Inc. [AMD/ATI] Navi 31 [Radeon RX 7900 XT/7900 XTX/7900 GRE/7900M] [1002:744c] (rev c8)\n"

	got := ParseLspciAMDOutput(output)

	if !reflect.DeepEqual(got.Families, []string{FamilyRadeon}) {
		t.Fatalf("Families = %v, want [%s] (a trailing '(rev NN)' must not defeat detection)", got.Families, FamilyRadeon)
	}
}

func TestParseLspciAMDOutputClassifiesByDeviceIDNotPCIClass(t *testing.T) {
	// A known GPU device ID must be classified from the amdGPUDevicesByID
	// table regardless of the PCI class lspci reports for it — e.g. an SR-IOV
	// VF or a headless "Display controller [0380]" function. This guards
	// against re-introducing a hardcoded PCI-class allowlist that would
	// silently drop valid GPUs whose function reports an unexpected class.
	output := "0000:01:00.0 Display controller [0380]: Advanced Micro Devices, Inc. [AMD/ATI] [1002:74b5]\n"

	got := ParseLspciAMDOutput(output)

	if !reflect.DeepEqual(got.Families, []string{FamilyInstinct}) {
		t.Fatalf("Families = %v, want [%s] (device-ID table must be authoritative over PCI class)", got.Families, FamilyInstinct)
	}
	if got.DescribeFamily(FamilyInstinct) != "MI300X VF" {
		t.Errorf("DescribeFamily(instinct) = %q, want %q", got.DescribeFamily(FamilyInstinct), "MI300X VF")
	}
}

func TestParseLspciAMDOutputNoKnownGPU(t *testing.T) {
	// A non-GPU AMD PCI function (e.g. chipset/bridge) must not be
	// misclassified just because it shares the AMD vendor ID.
	output := "0000:00:14.3 ISA bridge [0601]: Advanced Micro Devices, Inc. [AMD] FCH LPC Bridge [1002:790e]\n"

	got := ParseLspciAMDOutput(output)

	if len(got.Families) != 0 {
		t.Errorf("Families = %v, want none", got.Families)
	}
	if got.Ambiguous() {
		t.Error("no GPU detected should not be ambiguous")
	}
}

func TestParseLspciAMDOutputEmpty(t *testing.T) {
	got := ParseLspciAMDOutput("")
	if len(got.Families) != 0 {
		t.Errorf("Families = %v, want none", got.Families)
	}
}

func TestParseLspciAMDOutputDedupesMultipleIdenticalCards(t *testing.T) {
	// A common 8-way Instinct box should collapse to one model entry, not 8.
	output := ""
	for i := 0; i < 8; i++ {
		output += "0000:0" + string(rune('0'+i)) + ":00.0 Processing accelerators [1200]: Advanced Micro Devices, Inc. [AMD/ATI] Aldebaran/MI300X [1002:74a1]\n"
	}

	got := ParseLspciAMDOutput(output)

	if len(got.Models[FamilyInstinct]) != 1 {
		t.Errorf("Models[instinct] = %v, want a single deduped MI300X entry", got.Models[FamilyInstinct])
	}
}

func TestDetectAMDGPUFamiliesPropagatesLspciFailure(t *testing.T) {
	original := lspciOutput
	defer func() { lspciOutput = original }()

	lspciOutput = func() (string, error) {
		return "", errors.New("exec: \"lspci\": executable file not found in $PATH")
	}

	_, err := DetectAMDGPUFamilies()
	if err == nil {
		t.Fatal("expected an error when lspci is unavailable, got nil")
	}
}

func TestDetectAMDGPUFamiliesUsesParsedOutput(t *testing.T) {
	original := lspciOutput
	defer func() { lspciOutput = original }()

	lspciOutput = func() (string, error) {
		return "0000:03:00.0 Processing accelerators [1200]: Advanced Micro Devices, Inc. [AMD/ATI] Aldebaran/MI210 [1002:740f]\n", nil
	}

	got, err := DetectAMDGPUFamilies()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got.Families, []string{FamilyInstinct}) {
		t.Fatalf("Families = %v, want [%s]", got.Families, FamilyInstinct)
	}
}
