package cmd

import (
	"fmt"
	"strings"

	"github.com/silogen/cluster-bloom/pkg/config"
)

// hardwareMismatch is a single discrepancy between bloom.yaml and what bloom
// detected on this node.
type hardwareMismatch struct {
	Title  string
	Detail string
}

// checkHardwareConfigMismatch runs the "Check for hardware autodetection and
// configuration mismatch" pre-flight step. It consolidates discrepancies
// between the configuration and what bloom detected on this node:
//
//   - GPU_NODE vs. whether an AMD GPU is physically present,
//   - GPU_STACK_FAMILY (explicit) vs. the detected GPU family,
//   - the installed host ROCm version vs. the version the family requires,
//   - AIM_HARDWARE_FAMILY (explicit) vs. detected hardware,
//
// prints the findings, and prompts the operator to continue or abort.
//
// It is non-mutational (reads + prompts only), so it is safe under
// `--tags validate_node`. It runs in the top-level bloom process, which owns the
// operator's real TTY — unlike ansible, which bloom drives over an SSH loopback
// with its stdout/stderr piped through the OutputProcessor, so an interactive
// `pause` there would be swallowed and hang. That is exactly why the interactive
// mismatch prompt lives here in Go rather than in the playbook.
//
// The ROCm-version dimension is skipped when ROCM_ALLOW_VERSION_MISMATCH is set,
// matching the authoritative ansible ROCm guard. --yes/--auto-confirm-prompts
// auto-continues the whole step (via confirmYesNo). Returns a non-nil error to
// abort the run when the operator declines.
//
// Must run after config.ApplyGPUStackVars, which resolves the effective family
// (cfg["gpu_stack_family_resolved"]) and required host ROCm version
// (cfg["rocm_required_version"]).
func checkHardwareConfigMismatch(cfg config.Config, report gpuFamilyDetectionReport) error {
	detected := report.Detected
	gpuDetected := len(detected.GPU.Families) > 0
	gpuNode := configBool(cfg, "GPU_NODE", true)

	var mismatches []hardwareMismatch

	// GPU_NODE vs. detected GPU presence.
	switch {
	case gpuNode && !gpuDetected:
		mismatches = append(mismatches, hardwareMismatch{
			Title:  "GPU_NODE=true but no AMD GPU detected",
			Detail: "This node is marked as a GPU node, but no AMD GPU was found on the PCI bus. GPU/ROCm setup will run and may fail. Set GPU_NODE=false for a CPU-only node.",
		})
	case !gpuNode && gpuDetected:
		mismatches = append(mismatches, hardwareMismatch{
			Title:  "GPU_NODE=false but AMD GPU detected",
			Detail: fmt.Sprintf("Detected %s, but GPU_NODE is false, so GPU/ROCm will not be set up. Set GPU_NODE=true to enable GPU support.", describeFamilyList(detected, detected.GPU.Families)),
		})
	}

	// GPU_STACK_FAMILY explicitly set to a family this node doesn't have.
	if report.Defaults.GPUStackFamilyConflict {
		mismatches = append(mismatches, hardwareMismatch{
			Title:  fmt.Sprintf("GPU_STACK_FAMILY=%q does not match the detected GPU(s)", configString(cfg, "GPU_STACK_FAMILY")),
			Detail: fmt.Sprintf("Detected %s. Host ROCm and the GPU Operator target GPU_STACK_FAMILY, so the wrong driver stack would be installed. Fix GPU_STACK_FAMILY (or remove it to auto-detect).", describeFamilyList(detected, detected.GPU.Families)),
		})
	}

	// Installed ROCm version incompatible with the resolved family. Not a
	// mismatch when ROCm is simply absent (bloom installs the right one), and
	// skipped entirely under ROCM_ALLOW_VERSION_MISMATCH.
	if gpuNode && !configBool(cfg, "ROCM_ALLOW_VERSION_MISMATCH", false) {
		if m := rocmVersionMismatch(cfg); m != nil {
			mismatches = append(mismatches, *m)
		}
	}

	// AIM_HARDWARE_FAMILY: explicit list missing detected hardware, or listing
	// hardware not present here.
	if fams := report.Defaults.UnconfiguredDetectedAIMFamilies; len(fams) > 0 {
		mismatches = append(mismatches, hardwareMismatch{
			Title:  "AIM_HARDWARE_FAMILY is missing detected hardware",
			Detail: fmt.Sprintf("Set to %q, but this node also has %s. Your explicit value is kept as-is; add the family/families if you want AIM models for them.", configString(cfg, "AIM_HARDWARE_FAMILY"), describeFamilyList(detected, fams)),
		})
	}
	if fams := report.Defaults.ConfiguredAIMFamiliesNotDetected; len(fams) > 0 {
		mismatches = append(mismatches, hardwareMismatch{
			Title:  "AIM_HARDWARE_FAMILY lists hardware not detected here",
			Detail: fmt.Sprintf("Lists %s, which was not detected on this node. Expected if that hardware lives on other nodes; otherwise check for a typo.", strings.Join(fams, ", ")),
		})
	}

	fmt.Println()
	fmt.Println("🔧 Check for hardware autodetection and configuration mismatch")
	if len(mismatches) == 0 {
		fmt.Println("   ✅ No mismatches between detected hardware and configuration.")
		fmt.Println()
		return nil
	}

	for _, m := range mismatches {
		fmt.Printf("   ⚠️  %s\n", m.Title)
		fmt.Printf("       %s\n", m.Detail)
	}
	fmt.Println()

	if !confirmYesNo("Continue despite the mismatch(es) above?") {
		return fmt.Errorf("aborted by operator: hardware autodetection / configuration mismatch")
	}
	return nil
}

// rocmVersionMismatch reports an installed host ROCm version that is
// incompatible with the resolved GPU stack family, reusing
// config.HostRocmVersionAcceptable (the same logic and version pins the
// authoritative ansible ROCm guard uses) so the two never disagree. Returns nil
// when no ROCm is installed (normal install path) or the installed version is
// acceptable.
func rocmVersionMismatch(cfg config.Config) *hardwareMismatch {
	family := configString(cfg, "gpu_stack_family_resolved")
	required := configString(cfg, "rocm_required_version")

	installed, found := config.DetectInstalledROCmVersion()
	if !found {
		return nil
	}
	acceptable, err := config.HostRocmVersionAcceptable(family, installed, required)
	if err != nil || acceptable {
		return nil
	}

	needs := "ROCm 7.2.x (>= 7.2.3)"
	if family == "radeon" {
		needs = fmt.Sprintf("the ROCm %s train", required)
	}
	return &hardwareMismatch{
		Title: fmt.Sprintf("Installed ROCm %s is incompatible with GPU_STACK_FAMILY=%s", installed, familyOrDefault(family)),
		Detail: fmt.Sprintf("This node has ROCm %s, but %s requires %s. Set ROCM_ALLOW_VERSION_MISMATCH=true to proceed with the installed ROCm anyway.",
			installed, familyOrDefault(family), needs),
	}
}

// configBool reads a bool config value, tolerating an absent/nil entry (returns
// def) and a string form ("true"/"1"/"yes", case-insensitive) as produced when
// the value comes from an environment variable rather than parsed YAML.
func configBool(cfg config.Config, key string, def bool) bool {
	v, ok := cfg[key]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	return def
}

func familyOrDefault(family string) string {
	if family == "" {
		return "instinct"
	}
	return family
}
