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
- **`radeon` uses a different install model.** ROCm 7.13 is a "TheRock" preview-stream release that is **not** published on repo.radeon.com's legacy `amdgpu-install/<rocm-version>/` path; its ROCm packages are served from repo.amd.com. Bloom registers the repo.amd.com apt source itself and installs the `amdrocm-core-sdk<major.minor>-<gfx-family>` meta-package directly with `apt`, whereas `instinct` uses the legacy repo.radeon.com path. See [radeon ROCm 7.13 install model](#radeon-rocm-713-install-model).

### GPU family auto-detection and ambiguous hardware

When `GPU_STACK_FAMILY` and/or `AIM_HARDWARE_FAMILY` are left unset (the default), bloom runs a local hardware scan on the node before starting the install:

- A PCI scan (`lspci -nn -d 1002:`) classifies any AMD GPUs found by product family, using the same device-ID taxonomy as cluster-forge's `amd-gpu` NFD rule (`pkg/config/gpu_hardware_detect.go`, kept in sync with `sources/amd-gpu-operator/*/templates/gpu-nfd-default-rule.yaml` in cluster-forge).
- A `/proc/cpuinfo` check (`pkg/config/cpu_hardware_detect.go`) detects whether the node's CPU is an AMD EPYC part, for `cpu`-family AIM models that target EPYC rather than a GPU.

Both scans run unconditionally (not just on `GPU_NODE: true` nodes), since an EPYC CPU can be present — and worth detecting for `AIM_HARDWARE_FAMILY` — on a node with no AMD GPU at all. Detection is best-effort: if `lspci`/`pciutils` isn't available, `/proc/cpuinfo` isn't readable, or either scan otherwise fails, bloom silently skips that half of the scan and falls back to today's behavior (empty `GPU_STACK_FAMILY` resolves to `instinct`) — this can never turn a previously successful install into a failure.

What happens with the result:

- **One GPU family detected, `GPU_STACK_FAMILY` unset** — bloom sets it to that family automatically (e.g. a Radeon-only box gets `GPU_STACK_FAMILY: radeon` without being asked). An EPYC CPU has no bearing on this choice at all — `epyc` is not a valid `GPU_STACK_FAMILY` value.
- **Any hardware detected (GPU family and/or EPYC CPU), `AIM_HARDWARE_FAMILY` unset** — bloom sets it to the comma-separated list of everything detected (e.g. a Radeon GPU + EPYC CPU box gets `AIM_HARDWARE_FAMILY: epyc,radeon`). This is never ambiguous: the AIM model catalog is multi-select by design, so mixed hardware is valid here and just gets every matching family's models.
- **GPUs from more than one family detected (the "node ambiguity" case), `GPU_STACK_FAMILY` unset** — host ROCm and the GPU Operator are single-select per node, so bloom cannot guess which stack you intend to run AI workloads on. It prints the detected models for each family and an explanation of why it's asking, then prompts you to pick `instinct` or `radeon` interactively. Running with `--yes`/`--auto-confirm-prompts` (or with no readable stdin) hard-fails instead of guessing, telling you to set `GPU_STACK_FAMILY` explicitly in `bloom.yaml`. EPYC plays no part in this prompt.
- **Anything explicitly set in `bloom.yaml`** (either variable) is never overridden by detection, regardless of what hardware is found.
- **`AIM_HARDWARE_FAMILY` explicitly set, but detection finds hardware not in that list** — e.g. `AIM_HARDWARE_FAMILY: "epyc"` on a box that also has a Radeon GPU. The explicit value is used as-is (never overridden), but bloom surfaces it in the [hardware / configuration mismatch check](#hardware--configuration-mismatch-check) below, so a deliberately narrow config doesn't silently hide hardware you may have wanted included.

This prompt is safe from the "no TTY" constraint that applies to the [version compatibility guard](#version-compatibility-guard-fail-fast) below: it runs in the top-level `bloom` process directly on the operator's terminal, *before* `bloom` re-execs itself into the namespaced container that drives `ansible-playbook` over an SSH loopback connection (where a `pause`-style prompt genuinely has no TTY and would hang). By the time any ansible task runs, both variables are already resolved.

**Hardware detection summary**: every `bloom cli`/`bloom run` invocation always prints a short readout of what was found and what it resolved to, regardless of `--tags` (this runs before `--tags` filtering reaches ansible), and regardless of whether anything was auto-detected or auto-assigned:

```
🔎 Hardware detection summary
   GPU: radeon (RX 9070)
   CPU: AMD EPYC detected (AMD EPYC 9124 16-Core Processor)
   -> GPU_STACK_FAMILY=radeon (auto-detected)
   -> AIM_HARDWARE_FAMILY=epyc,radeon (auto-detected)
```

`GPU`/`CPU` report `none detected` / `no AMD EPYC CPU detected` when nothing was found. The `->` lines report the final resolved value for each variable and its source — `explicit in bloom.yaml`, `auto-detected`, or (for `GPU_STACK_FAMILY` only) the `instinct` default when nothing was detected and nothing was configured. This is the easiest way to sanity-check detection on a node, e.g. via `bloom cli bloom.yaml --tags validate_node`.

### Hardware / configuration mismatch check

Immediately after the detection summary, and before any playbook task runs (including a `--destroy-data` wipe), bloom runs a consolidated, **non-mutational** pre-flight step:

```
🔧 Check for hardware autodetection and configuration mismatch
```

It compares your `bloom.yaml` against what was actually detected on the node and reports any of the following discrepancies:

- **`GPU_NODE` vs. GPU presence** — `GPU_NODE: true` but no AMD GPU was found (GPU/ROCm setup will run and likely fail — set `GPU_NODE: false` for a CPU-only node), or `GPU_NODE: false` while an AMD GPU is present (GPU/ROCm will not be set up).
- **`GPU_STACK_FAMILY` vs. detected GPU** — `GPU_STACK_FAMILY` is explicitly set to a family this node doesn't have (host ROCm and the GPU Operator target that family, so the wrong driver stack would be installed).
- **Installed host ROCm version vs. the family's required train** — an *incompatible* ROCm is already installed (e.g. ROCm 7.13 on an `instinct` node, or a pre-7.2.3 ROCm when `instinct` needs 7.2.3). This is **not** flagged when no ROCm is installed at all (that is the normal install path — bloom installs the right version). The check reuses the exact version logic and pins of the authoritative [version compatibility guard](#version-compatibility-guard-fail-fast), so the two never disagree; this dimension is **skipped** when `ROCM_ALLOW_VERSION_MISMATCH` is set.
- **`AIM_HARDWARE_FAMILY` vs. detected hardware** — the explicit list is missing detected hardware, or lists hardware not present on this node (the latter is expected when that hardware lives on other nodes).

If there are no discrepancies the step prints `✅ No mismatches between detected hardware and configuration.` and continues without prompting. If there are, it prints each finding and asks:

```
Continue despite the mismatch(es) above? [y/N]:
```

Answering `n` (the default) aborts the run before anything is changed. This step runs in the top-level `bloom` process on the operator's real terminal — the same reason the [ambiguous-hardware prompt](#gpu-family-auto-detection-and-ambiguous-hardware) above is safe, and the reason it lives in Go rather than in a playbook `pause` (ansible has no TTY here and would hang). Running with `--yes`/`--auto-confirm-prompts` auto-continues past every mismatch (use with caution). There is no separate `AIM_ALLOW_VERSION_MISMATCH`: an `AIM_HARDWARE_FAMILY` discrepancy is a catalog difference rather than a version conflict, so it is governed by this interactive prompt (and `--yes`), not by a version-named flag.

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
8. (instinct/ROCm 7.2.x) Apply AMD's [post-install environment configuration](https://rocm.docs.amd.com/projects/install-on-linux/en/docs-7.2.3/install/post-install.html): register ROCm's `lib`/`lib64` with the system linker (`/etc/ld.so.conf.d/rocm.conf` + `ldconfig`), add ROCm `bin` to `PATH` for login shells (`/etc/profile.d/rocm.sh`), and verify with `amd-smi version`. These are written as persistent, system-wide config (not the doc's session-only `export`s) and are driven off the resolved ROCm root, so they stay correct for `/opt/rocm`, `/opt/rocm-7.2.3`, etc. The radeon/ROCm 7.13 packages register their tools/libraries via `update-alternatives` at install time, so this legacy finalization is not applied there.
9. (radeon/ROCm 7.13 tech preview) Run AMD's [post-installation validation](https://rocm.docs.amd.com/en/7.13.0-preview/install/rocm.html#post-installation) — `rocminfo` (checked for an `HSA Agents` section) and `amd-smi version` (checked for a `ROCm version` line). Both checks are bounded so an unhealthy driver stack can't stall the run: each is launched fully detached (`nohup setsid`) under `timeout -s KILL 120` and writes its exit code to a marker file, which bloom polls for with a lightweight loop (30 × 5s = 150s). This self-driven marker pattern (the same one used by the ROCm install tasks) is used instead of Ansible `async`/`poll`, whose `async_status` reaping can wedge in bloom's namespaced execution and hang the play on `ASYNC OK ... jid=...` even after the command finished. A check stuck *uninterruptibly* in a kernel ioctl on a wedged `/dev/kfd` (D state) never writes its marker, so the poll loop simply exhausts its retries and the check counts as a failure rather than hanging the run. Unlike the informational instinct check above, a failure here fails the bloom run outright with a message telling the user to install ROCm manually and re-run bloom, since the therock apt workaround (see [radeon ROCm 7.13 install model](#radeon-rocm-713-install-model)) can silently "succeed" without a working driver stack.

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
| `amdgpu-install` .deb | `repo.radeon.com/amdgpu-install/<rocm-version>/ubuntu/<codename>/` | `repo.radeon.com/amdgpu-install/31.30/ubuntu/<codename>/amdgpu-install_31.30.313000-1_all.deb` (used **only** for GPU→gfx-family auto-detection) |
| ROCm packages | repo.radeon.com (pinned above Ubuntu universe) | `repo.amd.com/rocm/packages/<ubuntuXXYY>` (apt source registered by bloom, key at `/etc/apt/keyrings/amdrocm.gpg`) |
| Install command | `amdgpu-install --usecase=rocm,dkms --yes --allow-downgrades` | `apt install amdrocm-core-sdk<major.minor>-<gfx-family>` (e.g. `amdrocm-core-sdk7.13-gfx110x`) |
| Detected version | 7.2.x (`/opt/rocm-7.2.3`) | 7.13.x (`/opt/rocm/core-7.13`, plus a `/opt/rocm-7.13.0` compat symlink) |

**Why bloom does not run `amdgpu-install` to install the radeon packages.** The `amdgpu-install` 31.30 utility is broken for the 7.13 preview tree: it constructs malformed package names — the release tag is glued *after* the gfx family (e.g. `amdrocm-gfx110x7.13.0`) — so every `apt-get` it issues fails with `Unable to locate package amdrocm-gfx110x7.13.0`. The real repo.amd.com packages put the major.minor *between* the family root and the gfx suffix (`amdrocm-core-sdk7.13-gfx110x`). Bloom therefore:

1. registers the repo.amd.com apt source and signing key itself (a keyring distinct from the legacy `rocm.gpg` so both repos can coexist),
2. reuses `amdgpu-install`'s own auto-detector purely to read the GPU's gfx family suffix (it prints `gfx suffix: -gfxNNNx` before it fails), keeping AMD's device→gfx mapping as the source of truth,
3. `apt install`s the correctly-named `amdrocm-core-sdk<major.minor>-<gfx-family>` meta-package (whose Depends chain pulls the runtime, dev libraries, and developer-tools including `amd-smi`), and
4. adds a `/opt/rocm-7.13.0 → /opt/rocm/core-7.13` compatibility symlink so ROCm-root detection and SMI verification (which search `/opt/rocm-*/bin` and `/opt/rocm/core-*/bin`) resolve the preview Core SDK layout.

The legacy-only steps (repo.radeon.com pin, `rocm.list` conffile restore/verify) are skipped for the therock model, since its repo comes from repo.amd.com.

> The `31.30.313000-1` installer version and the `7.13` release are sourced from AMD's [ROCm 7.13.0 preview install guide](https://rocm.docs.amd.com/en/7.13.0-preview/install/rocm.html) and the AMD HPCTrainingDock `rocm_setup.sh` preview path (which documents the same `amdgpu-install` 31.30 package-name bug), and should be reconciled with the authoritative pins in EAI-5906. They live in `pkg/config/gpu_stack_matrix.go` (`radeonInstaller*` / `radeonRocmRelease`).

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

The same incompatibility is also surfaced earlier and interactively by the top-level [hardware / configuration mismatch check](#hardware--configuration-mismatch-check), which does a best-effort read of the installed ROCm version (via `amd-smi` or `/opt/rocm*/.info/version`) and lets you abort at a `[y/N]` prompt before the playbook starts. Both use the same version logic and pins, so they agree; this ansible guard remains the authoritative, non-interactive backstop for `--tags`-scoped or automated runs. Setting `ROCM_ALLOW_VERSION_MISMATCH` suppresses both.

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
