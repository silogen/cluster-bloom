# AMD GPU Support with ROCm

## Overview

ClusterBloom provides automated AMD GPU support through ROCm driver installation and configuration, enabling GPU-accelerated workloads on Kubernetes clusters.

## Components

### GPU-family install defaults (`GPU_STACK_FAMILY`)

The host ROCm version and the cluster-forge GPU Operator are selected together per GPU family via the `GPU_STACK_FAMILY` flag (`radeon` | `instinct`; empty resolves to `instinct`). The selection is a single qualified matrix row, host ROCm, GPU Operator chart path, and the operator DeviceConfig ROCm driver version move together.

| Family | Host ROCm | GPU Operator path | DeviceConfig ROCm driver | Tech preview |
|--------|-----------|-------------------|--------------------------|--------------|
| `instinct` (default) | 7.2.3 / 70203-1 | amd-gpu-operator/v1.4.1 | 7.0 | no |
| `radeon` | 7.13.0 | amd-gpu-operator/v1.5.1-beta.0 | 7.13 | yes |

Notes:
- `instinct` reproduces the existing defaults exactly, so existing installs are unchanged.
- `radeon` selects the ROCm 7.13 tech-preview stack. bloom prints a tech-preview notice at install time, these components are not production qualified for this release.
- Single-select by design: host ROCm is one version per node. The AIM model catalog (`AIM_HARDWARE_FAMILY`) can still be heterogeneous.
- Unsupported combinations (e.g. a Radeon stack resolving to ROCm 7.2.0, which is too old) fail validation before install with an error naming the incompatible component. See [Version Compatibility Guard](#version-compatibility-guard-fail-fast) for the fail-fast behavior and how to override it.
- The real ROCm 7.13 tech-preview version strings and the vendored GPU Operator chart are tracked in EAI-5906; the `radeon` row carries placeholder pins until then.
- **`radeon` uses a different install model.** ROCm 7.13 is a "TheRock" preview-stream release that is **not** published on repo.radeon.com's legacy `amdgpu-install/<rocm-version>/` path. Bloom installs it via the `amdgpu-install` 31.x installer series plus `amdgpu-install --rocmrelease=7.13.0` (which pulls ROCm packages from repo.amd.com), whereas `instinct` uses the legacy repo.radeon.com path. See [radeon ROCm 7.13 install model](#radeon-rocm-713-install-model).

### ROCm Installation
Automated installation of ROCm drivers and runtime components:
- **Driver Version**: Selected by `GPU_STACK_FAMILY` (default family `instinct` → ROCm 7.2.3); base URL still overridable via `ROCM_BASE_URL`
- **Components**: amdgpu kernel driver, ROCm runtime, ROCm libraries
- **Dependencies**: Linux kernel headers, Python setuptools
- **Installation Method**: amdgpu-install utility from AMD repositories
- **Management Tool**: amd-smi (ROCm 7.x) replaces deprecated rocm-smi

**Installation Process**:
1. Detect Ubuntu version and kernel version
2. Install required kernel headers and modules
3. Purge any leftover `amdgpu-install` package and stale repo lists when a fresh install is needed (see [Recovering after `amdgpu-install --uninstall`](#recovering-after-amdgpu-install---uninstall))
4. Download amdgpu-install package from AMD repository
5. Execute installation with ROCm and DKMS use cases
6. Load amdgpu kernel module
7. Verify installation with amd-smi

### Recovering after `amdgpu-install --uninstall`

`amdgpu-install --uninstall` (and `amdgpu-uninstall`) removes the ROCm runtime — `amd-smi`, `/opt/rocm`, and the `rocm` metapackages — but **leaves the `amdgpu-install` package itself installed**, along with its `/etc/apt/sources.list.d/amdgpu.list` and `rocm.list` conffiles pinned to whatever ROCm train was last configured.

That leftover state previously broke a subsequent bloom install in several ways:
- When the leftover `amdgpu-install` was an equal or newer version than the train bloom targets, `apt` refused to reinstall the pinned `.deb` and aborted with `A later version is already installed`, so ROCm was never installed.
- On an upgrade path, `dpkg` preserved the old (modified) `rocm.list` conffile, so `amdgpu-install --usecase=rocm` resolved against the previous train's repo.
- The ROCm runtime packages (`rocm`, `amd-smi`, `hip-runtime-amd`, …) were frequently left marked *installed* in dpkg even though their files had been removed or renamed aside (e.g. a `/opt/rocm-<ver>.stale` tree). `amdgpu-install --usecase=rocm` then treated ROCm as already present and did nothing — the step "succeeded" but no tooling landed, and the SMI-presence guard failed.

Bloom now resets this leftover state to a clean slate during the ROCm install step, but only when a fresh install is required (no acceptable ROCm tooling present — a healthy, acceptable install never enters this path). It:
- purges the leftover `amdgpu-install` **and** any orphaned ROCm runtime packages (`rocm*`, `amd-smi*`, `hip-runtime-amd*`),
- removes the stale `amdgpu.list`/`rocm.list` repo lists, and
- deletes renamed-aside `/opt/rocm-*.stale` / `/opt/rocm-*.bak` trees,

before laying down the target train's `amdgpu-install` `.deb` and running the ROCm install. As a result, re-running bloom after `amdgpu-install --uninstall` installs ROCm cleanly for both the `instinct` (7.2 train) and `radeon` (7.13 preview) paths.

ROCm-root detection also ignores renamed-aside siblings: only real version-numbered directories such as `/opt/rocm-7.2.3` are considered, so a tool-less `/opt/rocm-7.13.0.stale` tree can no longer be picked as the ROCm root and shadow the real install. If detection still finds no `amd-smi`/`rocm-smi` after install, the failure now prints the dpkg package state, `/opt/rocm*` layout, and apt policy for `rocm`/`amd-smi` to make the cause obvious.

### radeon ROCm 7.13 install model

ROCm 7.13 (the `radeon` stack) is a **"TheRock" preview-stream release** with a different distribution model from the ROCm 5.x–7.2 stream used for `instinct`. Bloom selects the model automatically from `GPU_STACK_FAMILY` (`rocm_install_model` = `legacy` for instinct, `therock` for radeon):

| | `instinct` (legacy) | `radeon` (therock) |
|---|---|---|
| `amdgpu-install` .deb | `repo.radeon.com/amdgpu-install/<rocm-version>/ubuntu/<codename>/` | `repo.radeon.com/amdgpu-install/31.30/ubuntu/<codename>/amdgpu-install_31.30.313000-1_all.deb` |
| ROCm packages | repo.radeon.com (pinned above Ubuntu universe) | `repo.amd.com/rocm/packages/...`, registered on the fly by `amdgpu-install --rocmrelease` |
| Install command | `amdgpu-install --usecase=rocm,dkms --yes --allow-downgrades` | `amdgpu-install --usecase=rocm,dkms --rocmrelease=7.13.0 --yes --allow-downgrades` |
| Detected version | 7.2.x (`/opt/rocm-7.2.3`) | 7.13.x (`/opt/rocm-7.13`) |

The installer coordinates for radeon are intentionally decoupled from the ROCm version used for detection: the `amdgpu-install` **installer** comes from the `31.x` series, while `--rocmrelease=7.13.0` selects the ROCm **packages** (the flag requires the full `X.Y.Z` version — `7.13` is rejected). The legacy-only steps (repo.radeon.com pin, `rocm.list` conffile restore/verify) are skipped for the therock model, since its repo comes from repo.amd.com.

> The `31.30.313000-1` installer version and the `7.13` release are sourced from AMD's [ROCm 7.13.0 preview install guide](https://rocm.docs.amd.com/en/7.13.0-preview/install/rocm.html) and should be reconciled with the authoritative pins in EAI-5906. They live in `pkg/config/gpu_stack_matrix.go` (`radeonInstaller*` / `radeonRocmRelease`).

### GPU Detection
Validates GPU availability and configuration:
- **Hardware Detection**: Identifies AMD GPU devices via PCI bus
- **Driver Verification**: Checks amdgpu kernel module loading
- **Device Validation**: Verifies /dev/kfd and /dev/dri/renderD* devices
- **amd-smi Check**: Validates ROCm software stack functionality (ROCm 7.x)

**Detection Methods**:
```bash
# PCI device detection
lspci | grep -i 'vga\|display\|3d'

# Kernel module verification
lsmod | grep amdgpu

# Device node verification
ls -l /dev/kfd /dev/dri/renderD*

# ROCm validation (ROCm 7.x)
amd-smi list

# Detailed GPU information
amd-smi list --json
```

### Version Verification
Ensures correct ROCm version is installed:
- **Supported Version**: ROCm 7.2.3 exactly
- **Version Check**: Validates installed version matches requirements
- **Out-of-Date Detection**: Identifies 6.x versions requiring upgrade
- **Unsupported Warning**: Flags 7.2+ versions not yet supported

**Version Check Commands**:
```bash
# Check ROCm version (displayed in amd-smi header)
amd-smi
# Look for "ROCm version: X.X.X" in the first line

# Example output:
# +------------------------------------------------------------------------------+
# | AMD-SMI 26.0.2+39589fda  amdgpu version: 6.14.14  ROCm version: 7.2.3    |
# +------------------------------------------------------------------------------+

# Expected: ROCm version: 7.2.3 (instinct) or 7.13.0 (radeon)

# Fallback: read version file (layout depends on ROCm stream)
cat /opt/rocm/.info/version                    # legacy (e.g. 7.2.3)
cat /opt/rocm/core-7.13/.info/version          # ROCm 7.13 Core SDK (radeon)
```

**Version Status Guide**:
- ✅ **7.2.3** - Correct, required and fully supported
- ⚠️ **Other** - Version mismatch: WARNING issued; install 7.2.3

### Version Compatibility Guard (fail-fast)

Before doing any package, kernel, or repository work, bloom detects the ROCm already installed on each GPU node and checks it against the version train required by the selected `GPU_STACK_FAMILY`:

- `instinct` (default) requires host ROCm on the **7.2** train (>= 7.2.3).
- `radeon` requires host ROCm on the **7.13** train.

If a functional ROCm install (amd-smi / rocm-smi present) is found whose version does not match the required train — for example `radeon` selected on a host that already has ROCm 7.2.3 — bloom **aborts early during the node validation phase** with an "Unsupported ROCm version" message. This runs as early as the installed version can be known, so the deploy stops before any GPU work rather than finishing with a mismatched, unsupported stack.

This guard is a **hard fail with no interactive prompt**: bloom pipes the ansible-playbook output through its own processor over an SSH connection, so there is no TTY for a `[y/N]` prompt (it would hang the run). The escape hatch is the `ROCM_ALLOW_VERSION_MISMATCH` config option instead.

**Override (proceed anyway)** — keep the currently installed ROCm and skip the guard by setting this in `bloom.yaml`:

```yaml
ROCM_ALLOW_VERSION_MISMATCH: true   # accepts true|TRUE|1
```

`ROCM_ALLOW_VERSION_MISMATCH` is a `bloom.yaml` config key (default `false`), so it works with `bloom cli bloom.yaml`. With `bloom run` you can also pass it as an extra-var:

```bash
sudo bloom run -e ROCM_ALLOW_VERSION_MISMATCH=true ...
```

**Install ROCm 7.2.3**:
```bash
# 1. Remove old installation
sudo amdgpu-uninstall
sudo apt remove --purge amdgpu-install

# 2. Reinstall with 7.2.3
CODENAME=$(grep VERSION_CODENAME /etc/os-release | cut -d= -f2)
wget https://repo.radeon.com/amdgpu-install/7.2.3/ubuntu/$CODENAME/amdgpu-install_7.2.3.70002-1_all.deb
sudo apt install -y ./amdgpu-install_7.2.3.70002-1_all.deb
sudo amdgpu-install --usecase=rocm,dkms --yes

# 3. Reboot and verify
sudo reboot
# After reboot, check version in header:
amd-smi
# Should show: ROCm version: 7.2.3
```

### Device Rules
Configures udev rules for GPU access permissions:
- **Permission Mode**: 0666 for /dev/kfd and /dev/dri/renderD* devices
- **Udev Rules Location**: `/etc/udev/rules.d/70-amdgpu.rules`
- **Access Control**: Allows non-root container access to GPU devices

**Udev Rule Configuration**:
```
KERNEL=="kfd", MODE="0666"
SUBSYSTEM=="drm", KERNEL=="renderD*", MODE="0666"
```

**Rule Application**:
```bash
sudo udevadm control --reload-rules
sudo udevadm trigger
```

### Kernel Module Management
Handles amdgpu module loading and configuration:
- **Module Loading**: Automatic amdgpu module loading at boot
- **Module Parameters**: Configurable via /etc/modprobe.d/
- **Persistence**: Configuration persists across reboots
- **Dependency Management**: Ensures required modules are loaded

**Module Configuration**:
```bash
# Load module
sudo modprobe amdgpu

# Make persistent
echo "amdgpu" | sudo tee -a /etc/modules
```

### Kubernetes Integration
GPU resource exposure and scheduling:
- **Node Labels**: `gpu=true`, `amd.com/gpu=true`
- **Device Plugin**: AMD GPU device plugin for Kubernetes
- **Resource Limits**: GPU resource scheduling (`amd.com/gpu: 1`)
- **Pod Scheduling**: GPU-aware pod placement

**GPU Pod Example**:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: gpu-workload
spec:
  containers:
  - name: rocm-container
    image: rocm/pytorch:latest
    resources:
      limits:
        amd.com/gpu: 1
  nodeSelector:
    gpu: "true"
```

## Architecture

```mermaid
graph TB
    subgraph "ROCm Installation Flow"
        A[Check GPU Node Flag] --> B{GPU_NODE=true?}
        B -->|Yes| C[Detect Ubuntu Version]
        B -->|No| Z[Skip GPU Setup]
        C --> D[Install Kernel Headers]
        D --> E[Download amdgpu-install]
        E --> F[Install ROCm + DKMS]
        F --> G[Load amdgpu Module]
        G --> H[Configure udev Rules]
        H --> I[Verify with rocm-smi]
    end
    
    subgraph "GPU Device Access"
        I --> J[/dev/kfd Device]
        I --> K[/dev/dri/renderD* Devices]
        J --> L[Container GPU Access]
        K --> L
    end
    
    subgraph "Kubernetes GPU Scheduling"
        L --> M[AMD Device Plugin]
        M --> N[GPU Resource Advertisement]
        N --> O[Node Labels]
        O --> P[GPU Pod Scheduling]
    end
    
    style A fill:#4CAF50
    style I fill:#2196F3
    style M fill:#FF9800
    style P fill:#9C27B0
```
