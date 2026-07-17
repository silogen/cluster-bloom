package config

import "fmt"

// GPU stack version pins, by GPU family. These are the qualified host ROCm,
// GPU Operator chart, and DeviceConfig ROCm-driver versions that move together
// as one matrix row.
//
// The OperatorPath pins are real: instinct uses the qualified v1.4.1 chart and
// radeon uses the v1.5.1-beta.0 tech-preview chart, both vendored under
// cluster-forge sources/amd-gpu-operator. The radeon host ROCm and DeviceConfig
// driver versions are sourced outside cluster-bloom / cluster-forge (ROCm PMO /
// EAI-5906); the strings below mirror those values and are not authoritative
// pins owned here.
const (
	// Instinct: the existing qualified defaults (no behavior change).
	instinctHostRocmVersion    = "7.2.3"
	instinctHostRocmDebBuild   = "70203-1"
	// instinctHostRocmMinPatch is the minimum 7.2.x patch Bloom accepts on GPU
	// nodes when GPU_STACK_FAMILY is instinct (or empty). Newer patches such as
	// 7.2.4 are allowed without triggering amdgpu-install.
	instinctHostRocmMinPatch = 3
	instinctOperatorPath       = "amd-gpu-operator/v1.4.1"
	instinctOperatorConfigPath = "amd-gpu-operator-config/v1.4.1"
	instinctDriverVersion      = "7.0"

	// Radeon: ROCm 7.13 tech preview.
	radeonHostRocmVersion    = "7.13.0"
	radeonHostRocmDebBuild   = "71300-1"
	radeonOperatorPath       = "amd-gpu-operator/v1.5.1-beta.0"
	radeonOperatorConfigPath = "amd-gpu-operator-config/v1.5.1-beta.0"
	radeonDriverVersion      = "7.13"

	// Radeon ROCm 7.13 is a "TheRock" preview-stream release. It is NOT published
	// on repo.radeon.com's legacy amdgpu-install/<rocm-version>/ path; the ROCm
	// packages are served from repo.amd.com (repo.amd.com/rocm/packages/<ubuntuXXYY>).
	//
	// amdgpu-install 31.30 is BROKEN for this tree: it builds malformed package
	// names (the release tag glued after the gfx family, e.g.
	// "amdrocm-gfx110x7.13.0") and every apt-get 404s. So the ansible therock path
	// does NOT run `amdgpu-install --rocmrelease` to install; it registers the
	// repo.amd.com apt source itself and `apt install`s the correctly-named
	// "amdrocm-core-sdk<major.minor>-<gfx-family>" meta-package directly. The
	// amdgpu-install 31.x .deb is still downloaded/installed and used ONLY for its
	// GPU->gfx-family auto-detector (which prints "gfx suffix: -gfxNNNx"). These
	// installer coordinates are decoupled from radeonHostRocmVersion (7.13.0),
	// used for detection and version-acceptability. Sourced from AMD's ROCm
	// 7.13.0 preview install docs and the AMD HPCTrainingDock rocm_setup.sh
	// preview path (documents the amdgpu-install 31.30 bug); reconcile with EAI-5906.
	radeonInstallerBaseURL = "https://repo.radeon.com/amdgpu-install/31.30/ubuntu/"
	radeonInstallerDeb     = "amdgpu-install_31.30.313000-1_all.deb"
	// radeonRocmRelease is passed to the amdgpu-install auto-detector (as
	// --rocmrelease) and used to derive the major.minor package suffix (7.13.0 ->
	// 7.13) for the repo.amd.com meta-package. Must match radeonHostRocmVersion.
	radeonRocmRelease = "7.13.0"

	// installModelLegacy is the repo.radeon.com amdgpu-install path used for the
	// ROCm 5.x–7.2 stream (instinct); installModelTheRock is the 7.12+ preview path.
	installModelLegacy  = "legacy"
	installModelTheRock = "therock"
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
// passes OperatorPath + OperatorConfigPath + DeviceConfigDriverVersion through
// to cluster-forge so the GPU Operator, its config chart, and the DeviceConfig
// all match the same family.
type StackProfile struct {
	Family                    string
	HostRocmVersion           string
	HostRocmDebBuild          string
	OperatorPath              string
	OperatorConfigPath        string
	DeviceConfigDriverVersion string
	TechPreview               bool
	// InstallModel selects the ansible ROCm install path: "legacy" (repo.radeon.com
	// amdgpu-install for the ROCm 5.x–7.2 stream) or "therock" (7.12+ preview stream).
	InstallModel string
	// InstallerBaseURL / InstallerDeb locate the amdgpu-install .deb to download.
	// Empty means "use the legacy rocm_base_url / rocm_deb_package defaults", which
	// keeps the ROCM_BASE_URL / ROCM_DEB_PACKAGE overrides working for instinct.
	InstallerBaseURL string
	InstallerDeb     string
	// RocmRelease is the full ROCm version for the therock model (e.g. "7.13.0"):
	// fed to the amdgpu-install gfx auto-detector and used to derive the
	// major.minor repo.amd.com package suffix. Empty for the legacy model.
	RocmRelease string
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
			OperatorConfigPath:        instinctOperatorConfigPath,
			DeviceConfigDriverVersion: instinctDriverVersion,
			TechPreview:               false,
			InstallModel:              installModelLegacy,
		}, nil
	case "radeon":
		profile := StackProfile{
			Family:                    "radeon",
			HostRocmVersion:           radeonHostRocmVersion,
			HostRocmDebBuild:          radeonHostRocmDebBuild,
			OperatorPath:              radeonOperatorPath,
			OperatorConfigPath:        radeonOperatorConfigPath,
			DeviceConfigDriverVersion: radeonDriverVersion,
			TechPreview:               true,
			InstallModel:              installModelTheRock,
			InstallerBaseURL:          radeonInstallerBaseURL,
			InstallerDeb:              radeonInstallerDeb,
			RocmRelease:               radeonRocmRelease,
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
	cfg["rocm_version_exact_required"] = false
	cfg["rocm_instinct_min_patch"] = instinctHostRocmMinPatch
	// ROCm install model + installer coordinates. For radeon (therock) these point
	// the ansible download at the amdgpu-install 31.x series (used only for gfx
	// auto-detection) and carry the ROCm release used to build the repo.amd.com
	// package name. For instinct they are left unset so the
	// ansible defaults (rocm_base_url / rocm_deb_package, honoring ROCM_BASE_URL /
	// ROCM_DEB_PACKAGE overrides) apply unchanged.
	cfg["rocm_install_model"] = profile.InstallModel
	cfg["rocm_release"] = profile.RocmRelease
	if profile.InstallerBaseURL != "" {
		cfg["amdgpu_install_base_url"] = profile.InstallerBaseURL
	}
	if profile.InstallerDeb != "" {
		cfg["amdgpu_install_deb"] = profile.InstallerDeb
	}
	// Forge-bound selections consumed by the deploy_clusterforge tasks.
	cfg["gpu_operator_path"] = profile.OperatorPath
	cfg["gpu_operator_config_path"] = profile.OperatorConfigPath
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

// HostRocmVersionAcceptable reports whether an installed host ROCm version may
// be left in place for the given GPU stack family. Instinct accepts 7.2.x with
// patch >= instinctHostRocmMinPatch (e.g. 7.2.3, 7.2.4). Radeon accepts the
// pinned major.minor train with patch >= the pinned patch (e.g. 7.13.0,
// 7.13.1 when required is 7.13.0).
func HostRocmVersionAcceptable(family, installed, required string) (bool, error) {
	major, minor, patch, err := parseRocmVersion(installed)
	if err != nil {
		return false, err
	}
	switch family {
	case "", "instinct":
		return major == 7 && minor == 2 && patch >= instinctHostRocmMinPatch, nil
	case "radeon":
		reqMajor, reqMinor, reqPatch, err := parseRocmVersion(required)
		if err != nil {
			return false, err
		}
		return major == reqMajor && minor == reqMinor && patch >= reqPatch, nil
	default:
		return false, fmt.Errorf("unknown GPU stack family %q", family)
	}
}

// parseRocmVersion parses ROCm versions like "7.2.3" or "7.13.0-preview".
func parseRocmVersion(version string) (major, minor, patch int, err error) {
	n, _ := fmt.Sscanf(version, "%d.%d.%d", &major, &minor, &patch)
	if n < 3 {
		return 0, 0, 0, fmt.Errorf("invalid ROCm version %q", version)
	}
	return major, minor, patch, nil
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
