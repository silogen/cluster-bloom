package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/silogen/cluster-bloom/pkg/config"
)

// resolveGPUFamilyDefaults auto-detects the AMD GPU hardware family/families
// physically present on this node and fills in GPU_STACK_FAMILY /
// AIM_HARDWARE_FAMILY when the installer left them unset in bloom.yaml.
// Explicitly-set values are never overridden.
//
// Detection is best-effort: any failure (no lspci/pciutils, no permission,
// running inside a stripped-down container, etc.) is treated as "nothing
// detected" and skipped silently, so this can never turn a previously
// successful install into a failure.
//
// GPU_STACK_FAMILY is single-select per node (host ROCm + the GPU Operator
// can only target one family), so a node with GPUs from more than one family
// cannot be resolved automatically: the installer is asked to choose
// explicitly instead of bloom guessing — see decideAmbiguousGPUStackFamily.
// AIM_HARDWARE_FAMILY is multi-select by design (the AIM model catalog can be
// heterogeneous), so a mixed-family node simply gets every detected family
// listed there, with no prompt required.
//
// Runs in the top-level bloom process on the operator's real terminal,
// before RunPlaybook re-execs into the namespaced container that drives
// ansible-playbook over an SSH loopback with no TTY — so, unlike the
// ROCM_ALLOW_VERSION_MISMATCH guard deep in the ansible run, an interactive
// prompt here is safe.
func resolveGPUFamilyDefaults(cfg config.Config) error {
	gpuNode, _ := cfg["GPU_NODE"].(bool)
	if !gpuNode {
		return nil
	}

	detected, err := config.DetectAMDGPUFamilies()
	if err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "Note: skipping GPU family auto-detection (%v)\n", err)
		}
		return nil
	}

	defaults := config.ComputeFamilyDefaults(cfg, detected)

	if defaults.Ambiguous {
		family, err := decideAmbiguousGPUStackFamily(detected)
		if err != nil {
			return err
		}
		defaults.GPUStackFamily = family
	}

	if defaults.GPUStackFamily != "" {
		fmt.Printf("🔎 Detected %s GPU (%s) — setting GPU_STACK_FAMILY=%s\n",
			defaults.GPUStackFamily, detected.DescribeFamily(defaults.GPUStackFamily), defaults.GPUStackFamily)
		cfg["GPU_STACK_FAMILY"] = defaults.GPUStackFamily
	}
	if defaults.AIMHardwareFamily != "" {
		fmt.Printf("🔎 Detected AMD GPU families (%s) — setting AIM_HARDWARE_FAMILY=%s\n",
			describeAllFamilies(detected), defaults.AIMHardwareFamily)
		cfg["AIM_HARDWARE_FAMILY"] = defaults.AIMHardwareFamily
	}

	return nil
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

// describeAllFamilies renders every detected family and its models for
// informational log text, e.g. "instinct: MI300X; radeon: RX 9070".
func describeAllFamilies(detected config.DetectedGPUFamilies) string {
	parts := make([]string, 0, len(detected.Families))
	for _, family := range detected.Families {
		parts = append(parts, fmt.Sprintf("%s: %s", family, detected.DescribeFamily(family)))
	}
	return strings.Join(parts, "; ")
}
