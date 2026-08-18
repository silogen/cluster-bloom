# GPU Driver Support

> **Version scope:** This document applies to ClusterBloom versions
> `>= v2.3.0-rcx`. It does not describe the GPU driver behavior in the
> `v2.2.1` latest release.

ClusterBloom manages the host AMD GPU kernel driver without installing the ROCm
runtime, HIP, SDK, or workload libraries. Host ROCm is not required for
containerized AIM, AIWB, AIRM, NFD, or the AMD GPU Operator.

By default Bloom also installs AMD-SMI as a standalone host diagnostic. That
small optional userspace component does not turn the host into a full ROCm
runtime installation.

## Table of contents

- [Supported version matrix](#supported-version-matrix)
- [Installation behavior](#installation-behavior)
- [Standalone AMD-SMI](#standalone-amd-smi)
- [Configuration](#configuration)
- [Blacklisting](#blacklisting)
- [Verification](#verification)

## Supported version matrix

Bloom detects `amdgpu-dkms` package metadata and DKMS registrations before
changing an AMD repository or driver.

The exact supported tuples are:

| AMD driver | DKMS package/module
|---|---|---|
| `30.10.2` | `6.14.14.30100200-2226257` |
| `30.20.1` | `6.16.6.30200100-2255209`  |
| `30.30.3` | `6.16.13.30300300-2327507` |
| `30.30.4` | `6.16.13.30300400-2341068` |
| `31.30.0` | `6.19.4.31300000-2337710`  |
| `31.40.0` | `6.19.14.31400000-2364437` |

## Installation behavior

Bloom applies this policy:

1. If one supported DKMS tuple is detected, Bloom retains it and continues.
2. If no `amdgpu-dkms` package or manual DKMS registration exists, a stock
   Ubuntu inbox `amdgpu` module is treated as a fresh node. Bloom downloads
   `amdgpu-install_31.40.314000-1_all.deb` and runs
   `amdgpu-install --usecase=dkms`. This installs AMD GPU Driver `31.40.0` with
   DKMS module `6.19.14`.
3. If host ROCm runtime packages exist but Bloom cannot identify a supported
   DKMS release, Bloom halts rather than replacing an inbox or unknown driver
   beneath that userspace installation.
4. If an unrecognized or mixed out-of-tree DKMS release is detected, Bloom
   halts before changing the GPU repository or driver. Its output explains how
   to either install a supported driver paired with existing host ROCm, or
   remove the out-of-tree driver and let Bloom install `31.40.0`.
5. Bloom verifies that DKMS built the effective release for the running kernel,
   `modprobe` selects `updates/dkms/amdgpu.ko*`, and the selected module version
   matches the supported tuple.
6. If the old inbox or DKMS module remains active, Bloom writes
   `/var/lib/bloom/reboot-required.json` and ends the play before GPU-dependent
   deployment. The CLI offers to reboot on exit; `--yes` auto-confirms it.
7. On the post-reboot rerun, Bloom verifies the active module and then handles
   standalone AMD-SMI. The steps are idempotent.
8. If Bloom already rebooted for the marker and the reboot requirement remains,
   it stops with manual diagnostics instead of entering a reboot loop.

Existing host ROCm userspace is left untouched. Driver compatibility, not the
presence or absence of `/opt/rocm`, controls whether installation continues.

## Standalone AMD-SMI

`GPU_INSTALL_HOST_TOOLS` defaults to `true`.

Bloom first searches PATH and the legacy and Core SDK `/opt/rocm*` layouts. If
`amd-smi` already exists, Bloom retains it. If it is absent, Bloom installs the
package matched to the effective driver:

- Drivers `30.10.2`, `30.20.1`, `30.30.3`, and `30.30.4`: install the exact
  `amd-smi-lib` build from `repo.radeon.com/rocm/apt/7.0.2`, `7.1.1`, `7.2.3`,
  or `7.2.4`.
- Driver `31.30.0`: install `amdrocm-amdsmi7.13` from the versioned Core SDK
  package repository.
- Driver `31.40.0`: install `amdrocm-amdsmi7.14` from the Core SDK multi-arch
  package repository.

Bloom writes a dedicated `bloom-amd-smi.list` apt source, installs only AMD-SMI
and the package's minimal system dependencies, and exposes the resolved binary
as `/usr/local/bin/amd-smi`. It verifies both `amd-smi version` and
`amd-smi list`.

Set the following for a strict kernel-driver-only host:

```yaml
GPU_INSTALL_HOST_TOOLS: false
```

## Configuration

`GPU_DRIVER_SKIP_INSTALL` defaults to `false`. Setting it to `true` skips driver
compatibility validation, driver installation, and standalone AMD-SMI, leaving
the node's GPU stack untouched.

The fresh-node production default is:

```yaml
GPU_DRIVER_VERSION: ""
GPU_DRIVER_BUILD: ""
GPU_INSTALL_HOST_TOOLS: true
```

The empty driver values resolve to `31.40` and `314000-1`. These fields identify
the `amdgpu-install` package, not the resulting AMD driver:

| Installer package version (`GPU_DRIVER_VERSION`) | Installer build (`GPU_DRIVER_BUILD`) | Resulting AMD driver |
|---|---|---|
| `7.0.2` | `70002-1` | `30.10.2` |
| `7.1.1` | `70101-1` | `30.20.1` |
| `7.2.3` | `70203-1` | `30.30.3` |
| `7.2.4` | `70204-1` | `30.30.4` |
| `31.30` | `313000-1` | `31.30.0` |
| `31.40` | `314000-1` | `31.40.0` |

The jump from `7.2.4` to `31.30` is intentional. AMD's older
`amdgpu-install` packages in this allowlist use ROCm-aligned `7.x` package
versions, while the newer packages use the `31.x` AMD driver release stream.
These values therefore are not one continuous semantic-version sequence.

When Bloom reads `bloom.yaml`, it requires both override fields together and
checks the pair against this table before starting Ansible or connecting to the
node. Ansible repeats the tuple check as defense in depth. Detection and
validation of the driver actually installed on the host remain in Ansible
because they require access to package, DKMS, kernel-module, and ROCm state on
the target node.

`GPU_STACK_FAMILY` selects the ClusterForge GPU Operator and DeviceConfig
profile only. Both families share this host-driver policy and default to driver
`31.40.0` on fresh nodes.

See the [Configuration Reference](configuration-reference.md) for all GPU
configuration fields.

## Blacklisting

Bloom does not blacklist `amdgpu`. Both the inbox and DKMS drivers use the same
module name, so blacklisting `amdgpu` blocks both rather than selecting the DKMS
module.

If Bloom finds an active `blacklist amdgpu` directive, it automatically comments
out only the matching directive, creates an Ansible backup of the file, rebuilds
the current kernel's initramfs, and verifies that no active directive remains.
This remediation is noninteractive and behaves the same in direct and exported
playbook runs.

## Verification

Run the following checks on the GPU host after Bloom completes.

### 1. Confirm the installed packages

This prints the installed `amdgpu-install` and `amdgpu-dkms` package versions.
Both package names should resolve instead of reporting that no package was
found.

```bash
dpkg-query --show amdgpu-install amdgpu-dkms
```

### 2. Confirm DKMS built the driver for this kernel

The result should contain an `amdgpu` entry with status `installed` for the
kernel returned by `uname --kernel-release`.

```bash
uname --kernel-release
dkms status --module amdgpu
```

### 3. Confirm the selected module is the DKMS module

The path should be under
`/lib/modules/<kernel>/updates/dkms/amdgpu.ko*`. A path under the stock kernel
tree means the inbox driver is still selected and a reboot may be required.

```bash
modinfo --filename amdgpu
modinfo --field version amdgpu
```

### 4. Confirm the selected module is active

The first command reads the source version of the currently loaded module. The
second reads the source version of the module selected for the running kernel.
The two values should match.

```bash
cat /sys/module/amdgpu/srcversion
modinfo --field srcversion amdgpu
```

### 5. Confirm standalone AMD-SMI works

These commands report the AMD-SMI version and list the GPUs visible to the
host. Skip this check when `GPU_INSTALL_HOST_TOOLS: false`.

```bash
amd-smi version
amd-smi list
```

### 6. Confirm the required device files exist

`/dev/kfd` provides compute access and `/dev/dri/renderD*` provides render
devices. At least one render device must be present.

```bash
ls -l /dev/kfd /dev/dri/renderD*
```

### 7. Confirm Kubernetes advertises the GPU

Run this from a host with cluster access. The output should include an AMD GPU
label from Node Feature Discovery and a non-zero `amd.com/gpu` allocatable
resource. First list the nodes, then replace the example value with the GPU
node's Kubernetes name.

```bash
kubectl get nodes
GPU_NODE_NAME="replace-with-gpu-node-name"
kubectl get node "$GPU_NODE_NAME" --show-labels | grep -i amd
kubectl get node "$GPU_NODE_NAME" \
  --output jsonpath='{.status.allocatable.amd\.com/gpu}{"\n"}'
```
