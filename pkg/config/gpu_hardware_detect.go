package config

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// Supported AMD GPU product families, shared by GPU_STACK_FAMILY and
// AIM_HARDWARE_FAMILY detection.
const (
	FamilyInstinct = "instinct"
	FamilyRadeon   = "radeon"
)

// amdGPUDevice classifies one AMD PCI device ID into a product family, with a
// short marketing name for prompt/log text.
type amdGPUDevice struct {
	Family string
	Model  string
}

// amdGPUDevicesByID maps AMD (vendor 1002) PCI device IDs to their product
// family. Mirrored from cluster-forge's amd-gpu NFD rule
// (sources/amd-gpu-operator/*/templates/gpu-nfd-default-rule.yaml) so bloom's
// pre-install detection and cluster-forge's in-cluster node labeling agree on
// the same hardware taxonomy. Keep these two lists in sync when either one
// gains new device IDs.
var amdGPUDevicesByID = map[string]amdGPUDevice{
	// AMD Instinct (data-center accelerators), physical and MxGPU VF ids.
	"7410": {FamilyInstinct, "MI210 VF"},
	"74b5": {FamilyInstinct, "MI300X VF"},
	"74bd": {FamilyInstinct, "MI300X HF VF"},
	"74b6": {FamilyInstinct, "MI308X VF"},
	"74bc": {FamilyInstinct, "MI308X HF VF"},
	"74b9": {FamilyInstinct, "MI325X VF"},
	"75b8": {FamilyInstinct, "MI350P VF"},
	"75b0": {FamilyInstinct, "MI350X VF"},
	"75b3": {FamilyInstinct, "MI355X VF"},
	"75a3": {FamilyInstinct, "MI355X"},
	"75a0": {FamilyInstinct, "MI350X"},
	"75a8": {FamilyInstinct, "MI350P"},
	"74a5": {FamilyInstinct, "MI325X"},
	"74a2": {FamilyInstinct, "MI308X"},
	"74a8": {FamilyInstinct, "MI308X HF"},
	"74a0": {FamilyInstinct, "MI300A"},
	"74a1": {FamilyInstinct, "MI300X"},
	"74a9": {FamilyInstinct, "MI300X HF"},
	"740f": {FamilyInstinct, "MI210"},
	"7408": {FamilyInstinct, "MI250X"},
	"740c": {FamilyInstinct, "MI250/MI250X"},
	"738c": {FamilyInstinct, "MI100"},
	"738e": {FamilyInstinct, "MI100"},

	// AMD Radeon (Pro + consumer), including MxGPU VF ids.
	"7461": {FamilyRadeon, "Radeon Pro V710 MxGPU"},
	"73ae": {FamilyRadeon, "Radeon Pro V620 MxGPU"},
	"7460": {FamilyRadeon, "V710"},
	"7448": {FamilyRadeon, "W7900"},
	"744b": {FamilyRadeon, "W7900D"},
	"744a": {FamilyRadeon, "W7900 Dual Slot"},
	"7449": {FamilyRadeon, "W7800 48GB"},
	"745e": {FamilyRadeon, "W7800"},
	"73a2": {FamilyRadeon, "W6900X"},
	"73a3": {FamilyRadeon, "W6800 GL-XL"},
	"73ab": {FamilyRadeon, "W6800X / W6800X Duo"},
	"73a1": {FamilyRadeon, "V620"},
	"7551": {FamilyRadeon, "AI PRO R9700 / R9700S / R9600D"},
	"7550": {FamilyRadeon, "RX 9070 / 9070 XT"},
	"744c": {FamilyRadeon, "RX 7900 XT / 7900 XTX / 7900 GRE / 7900M"},
	"73af": {FamilyRadeon, "RX 6900 XT"},
	"73bf": {FamilyRadeon, "RX 6800 / 6800 XT / 6900 XT"},
	"7590": {FamilyRadeon, "RX 9060 XT"},
}

// gpuPCIClasses are the lspci class codes that represent a display/3D/GPU
// accelerator function, as opposed to a GPU board's *other* PCI functions
// (HDMI audio, USB-C, PCI bridge) which also carry AMD's vendor ID and would
// otherwise show up as spurious "unknown AMD device" noise.
var gpuPCIClasses = map[string]bool{
	"0300": true, // VGA controller
	"0302": true, // 3D controller
	"1200": true, // Processing accelerator (e.g. Instinct OAM modules)
}

// amdPCIDeviceLine matches one AMD (vendor 1002) `lspci -nn` line and
// captures the PCI class code and device ID, e.g.:
//
//	0000:03:00.0 Processing accelerators [1200]: AMD/ATI Aldebaran/MI210 [1002:740f]
var amdPCIDeviceLine = regexp.MustCompile(`\[([0-9a-fA-F]{4})]:.*\[1002:([0-9a-fA-F]{4})]\s*$`)

// DetectedGPUFamilies is the classification of AMD GPU hardware physically
// present on a node, from a local PCI scan.
type DetectedGPUFamilies struct {
	// Families lists the distinct product families found, sorted (a subset
	// of {FamilyInstinct, FamilyRadeon}). Empty means no known AMD GPU was
	// found — no GPU present, an unrecognized device ID, or lspci/pciutils
	// unavailable.
	Families []string
	// Models maps each family to the human-readable model names seen (e.g.
	// FamilyInstinct -> ["MI300X"]), for prompt/log text.
	Models map[string][]string
}

// Ambiguous reports whether the node has GPUs from more than one family,
// which host ROCm (a single family per node) cannot install for
// simultaneously.
func (d DetectedGPUFamilies) Ambiguous() bool {
	return len(d.Families) > 1
}

// DescribeFamily returns a human-readable "Model, Model2" summary of the
// models detected for one family, for prompt/log text. Empty if none.
func (d DetectedGPUFamilies) DescribeFamily(family string) string {
	return strings.Join(d.Models[family], ", ")
}

// lspciOutput is overridden in tests to avoid shelling out to real hardware.
var lspciOutput = func() (string, error) {
	out, err := exec.Command("lspci", "-nn", "-d", "1002:").Output()
	return string(out), err
}

// DetectAMDGPUFamilies runs a local PCI scan (via lspci) and classifies any
// AMD GPUs found into product families, using the same device-ID taxonomy as
// cluster-forge's amd-gpu NFD rule. Detection is best-effort: any failure
// (lspci/pciutils not installed, no permission, etc.) is returned as an error
// so callers can skip auto-detection rather than block an install that would
// have succeeded before this feature existed.
func DetectAMDGPUFamilies() (DetectedGPUFamilies, error) {
	out, err := lspciOutput()
	if err != nil {
		return DetectedGPUFamilies{}, fmt.Errorf("lspci: %w", err)
	}
	return ParseLspciAMDOutput(out), nil
}

// ParseLspciAMDOutput classifies `lspci -nn -d 1002:` output into GPU
// families. Exported (pure, no I/O) so the classification logic can be unit
// tested against captured lspci output without needing real hardware.
func ParseLspciAMDOutput(output string) DetectedGPUFamilies {
	result := DetectedGPUFamilies{Models: map[string][]string{}}
	seen := map[string]bool{} // dedupe by family+model across multiple identical cards
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		m := amdPCIDeviceLine.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		class, deviceID := strings.ToLower(m[1]), strings.ToLower(m[2])
		if !gpuPCIClasses[class] {
			continue
		}
		dev, ok := amdGPUDevicesByID[deviceID]
		if !ok {
			continue
		}
		key := dev.Family + "/" + dev.Model
		if seen[key] {
			continue
		}
		seen[key] = true
		result.Models[dev.Family] = append(result.Models[dev.Family], dev.Model)
	}
	for family := range result.Models {
		result.Families = append(result.Families, family)
	}
	sort.Strings(result.Families)
	return result
}
