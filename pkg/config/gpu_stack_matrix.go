package config

import "fmt"

// GPU stack version pins, by GPU family.
//
// EAI-5657 installs no host ROCm runtime. It manages an allowlisted amdgpu DKMS
// driver and, by default, the standalone AMD-SMI diagnostics package. ROCm
// versions in DriverCompatibility document the release paired with each driver;
// ROCm itself is neither required nor installed by the driver flow.
//
// The OperatorPath pins are unrelated to the host driver: instinct uses the
// qualified v1.4.1 chart and radeon uses the v1.5.1-beta.0 tech-preview chart,
// both vendored under cluster-forge sources/amd-gpu-operator. These still
// drive the (unchanged) GPU Operator + DeviceConfig deploy in cluster-forge.
const (
	defaultDriverPackageVersion = "31.40"
	defaultDriverPackageBuild   = "314000-1"

	instinctOperatorPath       = "amd-gpu-operator/v1.4.1"
	instinctOperatorConfigPath = "amd-gpu-operator-config/v1.4.1"
	instinctDriverVersion      = "7.0"

	radeonOperatorPath       = "amd-gpu-operator/v1.5.1-beta.0"
	radeonOperatorConfigPath = "amd-gpu-operator-config/v1.5.1-beta.0"
	radeonDriverVersion      = "7.13"
)

// DriverCompatibility is an exact, validated driver tuple. Do not replace this
// allowlist with a >= comparison: the driver release, DKMS source and paired
// ROCm train need to be reviewed together. PairedROCm is informational and is
// also used to select a matching standalone AMD-SMI package.
type DriverCompatibility struct {
	DriverRelease     string
	InstallerVersion  string
	InstallerBuild    string
	DKMSModuleVersion string
	DKMSBuild         string
	DKMSPackageCode   string
	PairedROCm        string
	HostToolsChannel  string
	HostToolsPackage  string
}

var supportedGPUDrivers = []DriverCompatibility{
	{
		DriverRelease: "30.20.1", InstallerVersion: "7.1.1", InstallerBuild: "70101-1",
		DKMSModuleVersion: "6.16.6", DKMSBuild: "2255209", DKMSPackageCode: "30200100", PairedROCm: "7.1.1",
		HostToolsChannel: "legacy", HostToolsPackage: "amd-smi-lib",
	},
	{
		DriverRelease: "30.30.3", InstallerVersion: "7.2.3", InstallerBuild: "70203-1",
		DKMSModuleVersion: "6.16.13", DKMSBuild: "2327507", DKMSPackageCode: "30300300", PairedROCm: "7.2.3",
		HostToolsChannel: "legacy", HostToolsPackage: "amd-smi-lib",
	},
	{
		DriverRelease: "31.30.0", InstallerVersion: "31.30", InstallerBuild: "313000-1",
		DKMSModuleVersion: "6.19.4", DKMSBuild: "2337710", DKMSPackageCode: "31300000", PairedROCm: "7.13",
		HostToolsChannel: "core", HostToolsPackage: "amdrocm-amdsmi7.13",
	},
	{
		DriverRelease: "31.40.0", InstallerVersion: "31.40", InstallerBuild: "314000-1",
		DKMSModuleVersion: "6.19.14", DKMSBuild: "2364437", DKMSPackageCode: "31400000", PairedROCm: "7.14",
		HostToolsChannel: "core-multiarch", HostToolsPackage: "amdrocm-amdsmi7.14",
	},
}

// minRadeonRocmMajor / minRadeonRocmMinor express the unsupported-combination
// rule from EAI-6030: Radeon requires the ROCm 7.13 tech-preview
// GPU-Operator/DeviceConfig train; anything older is too old and must block.
// This still applies here even though this branch installs no host ROCm,
// because DeviceConfigDriverVersion still selects the GPU Operator chart.
const (
	minRadeonRocmMajor = 7
	minRadeonRocmMinor = 13
)

// StackProfile is the resolved per-family GPU stack. DriverPackage* drives the
// ansible amdgpu-install (driver-only) task; OperatorPath + OperatorConfigPath
// + DeviceConfigDriverVersion pass through to cluster-forge so the GPU
// Operator and its DeviceConfig match the same family.
type StackProfile struct {
	Family                    string
	DriverPackageVersion      string
	DriverPackageBuild        string
	OperatorPath              string
	OperatorConfigPath        string
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
			DriverPackageVersion:      defaultDriverPackageVersion,
			DriverPackageBuild:        defaultDriverPackageBuild,
			OperatorPath:              instinctOperatorPath,
			OperatorConfigPath:        instinctOperatorConfigPath,
			DeviceConfigDriverVersion: instinctDriverVersion,
			TechPreview:               false,
		}, nil
	case "radeon":
		profile := StackProfile{
			Family:                    "radeon",
			DriverPackageVersion:      defaultDriverPackageVersion,
			DriverPackageBuild:        defaultDriverPackageBuild,
			OperatorPath:              radeonOperatorPath,
			OperatorConfigPath:        radeonOperatorConfigPath,
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
// vars into cfg. gpu_driver_default_version / gpu_driver_default_build are the
// family-based defaults for the driver-only install task; they are only used
// by ansible when GPU_DRIVER_VERSION / GPU_DRIVER_BUILD are left unset. Call
// after Validate, which guarantees the family resolves; any resolution error
// here is returned so the caller can fail loudly rather than install a
// mismatched stack.
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
	// Driver-only install defaults, overridable via GPU_DRIVER_VERSION /
	// GPU_DRIVER_BUILD (handled in the ansible task, not here).
	cfg["gpu_driver_default_version"] = profile.DriverPackageVersion
	cfg["gpu_driver_default_build"] = profile.DriverPackageBuild
	cfg["gpu_driver_supported"] = supportedGPUDrivers
	// Forge-bound selections consumed by the deploy_clusterforge tasks.
	cfg["gpu_operator_path"] = profile.OperatorPath
	cfg["gpu_operator_config_path"] = profile.OperatorConfigPath
	cfg["gpu_deviceconfig_driver_version"] = profile.DeviceConfigDriverVersion
	cfg["gpu_stack_family_resolved"] = profile.Family
	cfg["gpu_stack_tech_preview"] = profile.TechPreview
	return nil
}

// checkRadeonSupported enforces the Radeon minimum-DeviceConfig rule. It
// guards against a future edit pinning the radeon row to a too-old GPU
// Operator DeviceConfig train (e.g. 7.2).
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

// parseRocmMajorMinor parses a "MAJOR" or "MAJOR.MINOR[.PATCH]" version string
// (used for the GPU Operator DeviceConfig version, not a host ROCm install).
// A bare major (e.g. "7") yields minor 0.
func parseRocmMajorMinor(version string) (int, int, error) {
	var major, minor int
	if n, _ := fmt.Sscanf(version, "%d.%d", &major, &minor); n >= 1 {
		return major, minor, nil
	}
	return 0, 0, fmt.Errorf("invalid version %q", version)
}
