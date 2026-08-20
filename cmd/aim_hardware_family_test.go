package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/silogen/cluster-bloom/pkg/config"
)

func TestResolveAIMHardwareFamilyDefaultAutoDetects(t *testing.T) {
	config.SetHardwareDetectHooksForTest(t,
		func() (string, error) {
			return "0000:03:00.0 Processing accelerators [1200]: AMD/ATI Device [1002:74a1]\n", nil
		},
		func() (string, error) {
			return "", nil
		},
	)

	cfg := config.Config{}
	report := resolveAIMHardwareFamilyDefault(cfg)
	if report.WasExplicit {
		t.Fatal("expected implicit catalog")
	}
	if cfg["AIM_HARDWARE_FAMILY"] != "instinct" {
		t.Fatalf("cfg = %q, want instinct", cfg["AIM_HARDWARE_FAMILY"])
	}
}

func TestResolveAIMHardwareFamilyDefaultPreservesExplicitValue(t *testing.T) {
	config.SetHardwareDetectHooksForTest(t,
		func() (string, error) { return "", nil },
		func() (string, error) { return "", nil },
	)

	cfg := config.Config{"AIM_HARDWARE_FAMILY": "radeon"}
	report := resolveAIMHardwareFamilyDefault(cfg)
	if !report.WasExplicit {
		t.Fatal("expected explicit catalog")
	}
	if cfg["AIM_HARDWARE_FAMILY"] != "radeon" {
		t.Fatalf("cfg = %q, want radeon", cfg["AIM_HARDWARE_FAMILY"])
	}
}

func TestConfirmAIMHardwareFamilyCompatibility(t *testing.T) {
	detected := config.DetectedHardware{
		GPU: config.DetectedGPUFamilies{
			Families: []string{config.FamilyRadeon},
		},
		GPUScanSucceeded: true,
		CPUScanSucceeded: true,
	}

	tests := []struct {
		name        string
		cfg         config.Config
		report      aimHardwareFamilyReport
		autoConfirm bool
		stdin       string
		want        bool
	}{
		{
			name:   "implicit catalog",
			cfg:    config.Config{"AIM_HARDWARE_FAMILY": "instinct"},
			report: aimHardwareFamilyReport{WasExplicit: false},
			want:   true,
		},
		{
			name:   "explicit compatible catalog",
			cfg:    config.Config{"AIM_HARDWARE_FAMILY": "cpu,radeon"},
			report: aimHardwareFamilyReport{Detected: detected, WasExplicit: true},
			want:   true,
		},
		{
			name:        "explicit incompatible auto-confirmed",
			cfg:         config.Config{"AIM_HARDWARE_FAMILY": "instinct"},
			report:      aimHardwareFamilyReport{Detected: detected, WasExplicit: true},
			autoConfirm: true,
			want:        true,
		},
		{
			name:   "explicit incompatible declined",
			cfg:    config.Config{"AIM_HARDWARE_FAMILY": "instinct"},
			report: aimHardwareFamilyReport{Detected: detected, WasExplicit: true},
			stdin:  "n\n",
			want:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			prevAutoConfirm := autoConfirm
			autoConfirm = test.autoConfirm
			t.Cleanup(func() { autoConfirm = prevAutoConfirm })

			if test.stdin != "" {
				prevStdin := os.Stdin
				r, w, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				os.Stdin = r
				t.Cleanup(func() { os.Stdin = prevStdin })
				if _, err := io.Copy(w, bytes.NewBufferString(test.stdin)); err != nil {
					t.Fatal(err)
				}
				w.Close()
			}

			if got := confirmAIMHardwareFamilyCompatibility(test.cfg, test.report); got != test.want {
				t.Fatalf("confirmAIMHardwareFamilyCompatibility() = %t, want %t", got, test.want)
			}
		})
	}
}
