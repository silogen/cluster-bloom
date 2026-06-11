package config

import "fmt"

// GPU stack version pins, by GPU family. These are the qualified host ROCm,
// GPU Operator chart, and DeviceConfig ROCm-driver versions that move together
// as one matrix row.
//
// TODO(EAI-5906): the radeon row carries placeholder pins until the ROCm 7.13
// tech-preview GPU Operator build and amdgpu-install package are published.
// Replace the radeonHostRocm* / radeonOperatorPath / radeonDriverVersion
// constants with the real strings from EAI-5906 / the ROCm PMO sync.
const (
	// Instinct: the existing qualified defaults (no behavior change).
	instinctHostRocmVersion  = "7.1.1"
	instinctHostRocmDebBuild = "70101-1"
	instinctOperatorPath     = "amd-gpu-operator/v1.4.1"
	instinctDriverVersion    = "7.0"

	// Radeon: ROCm 7.13 tech preview. PLACEHOLDER pins, see TODO above.
	radeonHostRocmVersion  = "7.13.0"                  // TODO(EAI-5906): real 7.13 TP host ROCm version
	radeonHostRocmDebBuild = "71300-1"                 // TODO(EAI-5906): real amdgpu-install deb build id
	radeonOperatorPath     = "amd-gpu-operator/v1.4.1" // TODO(EAI-5906): vendored 7.13 TP operator chart path
	radeonDriverVersion    = "7.13"                    // TODO(EAI-5906): DeviceConfig driver.version for 7.13 TP
)

// minRadeonRocmMajor / minRadeonRocmMinor express the unsupported-combination
// rule from EAI-6030: Radeon requires the ROCm 7.13 tech-preview train; ROCm 7.2
// (and older) is too old and must block the install.
const (
	minRadeonRocmMajor = 7
	minRadeonRocmMinor = 13
)

// StackProfile is the resolved per-family ROCm / GPU Operator stack. Bloom owns
// host ROCm (the HostRocm* fields drive the ansible amdgpu-install vars) and
// passes OperatorPath + DeviceConfigDriverVersion through to cluster-forge so
// the GPU Operator and its DeviceConfig match the same family.
type StackProfile struct {
	Family                    string
	HostRocmVersion           string
	HostRocmDebBuild          string
	OperatorPath              string
	DeviceConfigDriverVersion string
	TechPreview               bool
}

// ResolveStackProfile maps a GPU_STACK_FAMILY value to its qualified stack.
// Empty resolves to instinct (the current defaults), so existing installs are
// unchanged. Unsupported combinations return an error naming the incompatible
// component, which the caller surfaces as a fail-fast validation error.
func ResolveStackProfile(family string) (StackProfile, error) {
	switch family {
	case "", "instinct":
		return StackProfile{
			Family:                    "instinct",
			HostRocmVersion:           instinctHostRocmVersion,
			HostRocmDebBuild:          instinctHostRocmDebBuild,
			OperatorPath:              instinctOperatorPath,
			DeviceConfigDriverVersion: instinctDriverVersion,
			TechPreview:               false,
		}, nil
	case "radeon":
		profile := StackProfile{
			Family:                    "radeon",
			HostRocmVersion:           radeonHostRocmVersion,
			HostRocmDebBuild:          radeonHostRocmDebBuild,
			OperatorPath:              radeonOperatorPath,
			DeviceConfigDriverVersion: radeonDriverVersion,
			TechPreview:               true,
		}
		if err := checkRadeonSupported(profile); err != nil {
			return StackProfile{}, err
		}
		return profile, nil
	default:
		return StackProfile{}, fmt.Errorf(
			"GPU_STACK_FAMILY %q is not a supported GPU family (expected radeon or instinct)", family)
	}
}

// ApplyGPUStackVars resolves GPU_STACK_FAMILY and injects the derived ansible
// vars into cfg. These override the play-level defaults in cluster-bloom.yaml
// (host ROCm) and carry the resolved GPU Operator path + DeviceConfig ROCm
// version through to the cluster-forge deploy tasks. Call after Validate, which
// guarantees the family resolves; any resolution error here is returned so the
// caller can fail loudly rather than install a mismatched stack.
func ApplyGPUStackVars(cfg Config) error {
	family := ""
	if v, ok := cfg["GPU_STACK_FAMILY"]; ok && v != nil {
		if s, isStr := v.(string); isStr {
			family = s
		}
	}
	profile, err := ResolveStackProfile(family)
	if err != nil {
		return err
	}
	// Host ROCm: override the cluster-bloom.yaml play vars via extra-vars.
	cfg["rocm_required_version"] = profile.HostRocmVersion
	cfg["rocm_deb_build"] = profile.HostRocmDebBuild
	// Forge-bound selections consumed by the deploy_clusterforge tasks.
	cfg["gpu_operator_path"] = profile.OperatorPath
	cfg["gpu_deviceconfig_driver_version"] = profile.DeviceConfigDriverVersion
	cfg["gpu_stack_family_resolved"] = profile.Family
	cfg["gpu_stack_tech_preview"] = profile.TechPreview
	return nil
}

// checkRadeonSupported enforces the Radeon minimum-ROCm rule. It guards against
// a future edit pinning the radeon row to a too-old ROCm train (e.g. 7.2).
func checkRadeonSupported(p StackProfile) error {
	major, minor, err := parseRocmMajorMinor(p.DeviceConfigDriverVersion)
	if err != nil {
		return fmt.Errorf("GPU stack radeon: cannot parse DeviceConfig ROCm version %q: %w",
			p.DeviceConfigDriverVersion, err)
	}
	if major < minRadeonRocmMajor || (major == minRadeonRocmMajor && minor < minRadeonRocmMinor) {
		return fmt.Errorf(
			"unsupported GPU stack: family=radeon resolves to GPU Operator ROCm %s, which is too old; "+
				"radeon requires the ROCm %d.%d tech-preview train or newer",
			p.DeviceConfigDriverVersion, minRadeonRocmMajor, minRadeonRocmMinor)
	}
	return nil
}

// parseRocmMajorMinor parses a "MAJOR" or "MAJOR.MINOR[.PATCH]" ROCm version.
// A bare major (e.g. "7") yields minor 0.
func parseRocmMajorMinor(version string) (int, int, error) {
	var major, minor int
	if n, _ := fmt.Sscanf(version, "%d.%d", &major, &minor); n >= 1 {
		return major, minor, nil
	}
	return 0, 0, fmt.Errorf("invalid ROCm version %q", version)
}
