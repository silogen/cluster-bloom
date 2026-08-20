package config

import (
	"reflect"
	"testing"
)

func TestFormatAIMHardwareFamily(t *testing.T) {
	tests := []struct {
		name     string
		detected DetectedHardware
		want     string
	}{
		{
			name: "generic CPU fallback",
			want: FamilyCPU,
		},
		{
			name: "EPYC",
			detected: DetectedHardware{
				EPYCModel: "AMD EPYC 9124 16-Core Processor",
			},
			want: FamilyEPYC,
		},
		{
			name: "GPU and EPYC",
			detected: DetectedHardware{
				GPU: DetectedGPUFamilies{
					Families: []string{FamilyRadeon},
				},
				EPYCModel: "AMD EPYC 9124 16-Core Processor",
			},
			want: "epyc,radeon",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FormatAIMHardwareFamily(test.detected); got != test.want {
				t.Fatalf("FormatAIMHardwareFamily() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestUnsupportedAIMHardwareFamilies(t *testing.T) {
	detected := DetectedHardware{
		GPU: DetectedGPUFamilies{
			Families: []string{FamilyRadeon},
		},
		GPUScanSucceeded: true,
		CPUScanSucceeded: true,
	}

	got := UnsupportedAIMHardwareFamilies(
		"cpu,epyc,instinct,radeon,instinct",
		detected,
	)
	want := []string{FamilyEPYC, FamilyInstinct}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("UnsupportedAIMHardwareFamilies() = %v, want %v", got, want)
	}
}

func TestUnsupportedAIMHardwareFamiliesDoesNotGuessAfterFailedScan(t *testing.T) {
	got := UnsupportedAIMHardwareFamilies(
		"epyc,instinct",
		DetectedHardware{},
	)
	if len(got) != 0 {
		t.Fatalf("failed scans must not prove incompatibility, got %v", got)
	}
}
