package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/silogen/cluster-bloom/pkg/config"
)

// resolveGPUFamilyDefaults auto-detects the AMD hardware family/families
// physically present on this node — GPUs via a local PCI scan, plus an AMD
// EPYC CPU via /proc/cpuinfo — and fills in GPU_STACK_FAMILY /
// AIM_HARDWARE_FAMILY when the installer left them unset in bloom.yaml.
// Explicitly-set values are never overridden.
//
// Detection is best-effort: any failure (no lspci/pciutils, no permission,
// unreadable /proc/cpuinfo, running inside a stripped-down container, etc.)
// is treated as "nothing detected" for that half of the scan and skipped
// silently, so this can never turn a previously successful install into a
// failure.
//
// GPU_STACK_FAMILY is single-select per node (host ROCm + the GPU Operator
// can only target one family), so a node with GPUs from more than one family
// cannot be resolved automatically: the installer is asked to choose
// explicitly instead of bloom guessing — see decideAmbiguousGPUStackFamily.
// EPYC has no bearing on this choice at all: it is not a valid
// GPU_STACK_FAMILY value, since it names a CPU-only AIM target rather than a
// ROCm/GPU Operator stack.
//
// AIM_HARDWARE_FAMILY is multi-select by design (the AIM model catalog can
// be heterogeneous), so a mixed-hardware node simply gets every detected
// family listed there (GPU families plus "epyc"), with no prompt required.
// When AIM_HARDWARE_FAMILY was set explicitly and detection finds hardware
// the installer didn't list (e.g. AIM_HARDWARE_FAMILY: "epyc" on a box that
// also has a Radeon GPU), the explicit value still wins as-is, but an
// informational notice is printed so an incomplete config isn't silently
// missed.
//
// This detection isn't gated on GPU_NODE: an EPYC CPU can be present (and
// worth auto-detecting for AIM_HARDWARE_FAMILY) on a node with no AMD GPU at
// all, and the GPU PCI scan itself is a cheap no-op on such a node anyway.
//
// Runs in the top-level bloom process on the operator's real terminal,
// before RunPlaybook re-execs into the namespaced container that drives
// ansible-playbook over an SSH loopback with no TTY — so, unlike the
// ROCM_ALLOW_VERSION_MISMATCH guard deep in the ansible run, an interactive
// prompt here is safe.
func resolveGPUFamilyDefaults(cfg config.Config) error {
	detected := config.DetectedHardware{}

	gpuDetected, err := config.DetectAMDGPUFamilies()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Note: skipping AMD GPU auto-detection (%v)\n", err)
		}
	} else {
		detected.GPU = gpuDetected
	}

	epycDetected, epycModel, err := config.DetectAMDEPYCCPU()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Note: skipping AMD EPYC CPU auto-detection (%v)\n", err)
		}
	} else if epycDetected {
		detected.EPYCModel = epycModel
	}

	defaults := config.ComputeFamilyDefaults(cfg, detected)

	if defaults.Ambiguous {
		family, err := decideAmbiguousGPUStackFamily(detected.GPU)
		if err != nil {
			return err
		}
		defaults.GPUStackFamily = family
	}

	if defaults.GPUStackFamily != "" {
		fmt.Printf("🔎 Detected %s GPU (%s) — setting GPU_STACK_FAMILY=%s\n",
			defaults.GPUStackFamily, detected.GPU.DescribeFamily(defaults.GPUStackFamily), defaults.GPUStackFamily)
		cfg["GPU_STACK_FAMILY"] = defaults.GPUStackFamily
	}
	if defaults.AIMHardwareFamily != "" {
		fmt.Printf("🔎 Detected AMD hardware (%s) — setting AIM_HARDWARE_FAMILY=%s\n",
			describeAllHardware(detected), defaults.AIMHardwareFamily)
		cfg["AIM_HARDWARE_FAMILY"] = defaults.AIMHardwareFamily
	}
	if len(defaults.UnconfiguredDetectedAIMFamilies) > 0 {
		fmt.Printf("ℹ️  AIM_HARDWARE_FAMILY is set to %q, but this node also has %s not included there.\n",
			configString(cfg, "AIM_HARDWARE_FAMILY"), describeFamilyList(detected, defaults.UnconfiguredDetectedAIMFamilies))
		fmt.Println("   Using your explicit AIM_HARDWARE_FAMILY as-is. Add the family/families above too if you want AIM models for them on this cluster.")
	}

	return nil
}

// configString reads a string config value, tolerating an absent, nil, or
// non-string entry (returns "" rather than panicking on a type assertion).
func configString(cfg config.Config, key string) string {
	v, ok := cfg[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

// decideAmbiguousGPUStackFamily is called when a node has AMD GPUs from more
// than one product family (e.g. an Instinct accelerator and a Radeon card in
// the same box) and GPU_STACK_FAMILY was left unset. Host ROCm and the GPU
// Operator can only target one family per node, so bloom cannot safely guess
// which one is intended — silently picking either risks installing the wrong
// ROCm/driver stack for hardware the installer didn't mean to run AI
// workloads on.
//
// In non-interactive runs (--yes/--auto-confirm-prompts, or no readable
// stdin) this hard-fails instead of guessing, matching the existing
// ROCM_ALLOW_VERSION_MISMATCH fail-fast convention: the installer must set
// GPU_STACK_FAMILY explicitly in bloom.yaml to proceed.
func decideAmbiguousGPUStackFamily(detected config.DetectedGPUFamilies) (string, error) {
	fmt.Println()
	fmt.Println("⚠️  Multiple AMD GPU families detected on this node")
	fmt.Println()
	fmt.Println("This node has GPUs from more than one AMD product family:")
	for _, family := range detected.Families {
		fmt.Printf("  - %-9s %s\n", family+":", detected.DescribeFamily(family))
	}
	fmt.Println()
	fmt.Println("GPU_STACK_FAMILY controls the host ROCm install and GPU Operator chart, and")
	fmt.Println("both are single-select: only one family's stack can be installed on this")
	fmt.Println("node. Bloom cannot guess which one you intend to run AI workloads on, so it")
	fmt.Println("needs an explicit choice here rather than picking one for you. (The AIM model")
	fmt.Println("catalog is separate and will still include both families' models.)")
	fmt.Println()

	const errHint = "set GPU_STACK_FAMILY explicitly to \"radeon\" or \"instinct\" in bloom.yaml to proceed"

	if autoConfirm {
		return "", fmt.Errorf("ambiguous GPU hardware detected (%s) and --yes/--auto-confirm-prompts was set; %s",
			strings.Join(detected.Families, " + "), errHint)
	}

	reader := bufio.NewReader(os.Stdin)
	for attempt := 0; attempt < 3; attempt++ {
		fmt.Println("Which GPU family should this node's ROCm host install + GPU Operator target?")
		fmt.Println("  [1] instinct")
		fmt.Println("  [2] radeon")
		fmt.Print("Enter 1 or 2: ")

		line, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("no interactive input available to resolve ambiguous GPU family; %s", errHint)
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "1", "instinct":
			return "instinct", nil
		case "2", "radeon":
			return "radeon", nil
		default:
			fmt.Println("Please enter 1 or 2.")
			fmt.Println()
		}
	}
	return "", fmt.Errorf("no valid selection after 3 attempts; %s", errHint)
}

// describeAllHardware renders every detected GPU family (with models) plus
// the EPYC CPU, if any, for informational log text, e.g.
// "instinct: MI300X; radeon: RX 9070; epyc: AMD EPYC 9354 32-Core Processor".
func describeAllHardware(detected config.DetectedHardware) string {
	parts := make([]string, 0, len(detected.GPU.Families)+1)
	for _, family := range detected.GPU.Families {
		parts = append(parts, fmt.Sprintf("%s: %s", family, detected.GPU.DescribeFamily(family)))
	}
	if detected.HasEPYC() {
		parts = append(parts, fmt.Sprintf("%s: %s", config.FamilyEPYC, detected.EPYCModel))
	}
	return strings.Join(parts, "; ")
}

// describeFamilyList renders a subset of detected families (e.g.
// FamilyDefaults.UnconfiguredDetectedAIMFamilies) with their model/CPU
// descriptions, e.g. "radeon (RX 9070)".
func describeFamilyList(detected config.DetectedHardware, families []string) string {
	parts := make([]string, 0, len(families))
	for _, family := range families {
		if family == config.FamilyEPYC {
			parts = append(parts, fmt.Sprintf("%s (%s)", family, detected.EPYCModel))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", family, detected.GPU.DescribeFamily(family)))
	}
	return strings.Join(parts, ", ")
}
