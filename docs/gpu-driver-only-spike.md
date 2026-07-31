# GPU Driver-Only Host Policy (EAI-5657)

ClusterBloom manages the host AMD GPU kernel driver without installing the ROCm
runtime, HIP, SDK, or workload libraries. Host ROCm is not required for
containerized AIM, AIWB, AIRM, NFD, or the AMD GPU Operator.

By default Bloom also installs AMD-SMI as a standalone host diagnostic. That
small optional userspace component does not turn the host into a full ROCm
runtime installation.

## Supported version matrix

Bloom detects `amdgpu-dkms` package metadata and DKMS registrations before
changing an AMD repository or driver.

The exact supported tuples are:

| AMD driver | DKMS package/module | Associated ROCm |
|---|---|---|
| `30.10.2` | `6.14.14.30100200-2226257` | `7.0.2` |
| `30.20.1` | `6.16.6.30200100-2255209` | `7.1.1` |
| `30.30.3` | `6.16.13.30300300-2327507` | `7.2.3` |
| `30.30.4` | `6.16.13.30300400-2341068` | `7.2.4` |
| `31.30.0` | `6.19.4.31300000-2337710` | `7.13.0` |
| `31.40.0` | `6.19.14.31400000-2364437` | `7.14.0` |

The paired ROCm release documents AMD's coordinated release train and selects a
matching AMD-SMI package. Bloom does not require or install that ROCm release.

## Installation synopsis

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

The empty driver values resolve to `31.40` and `314000-1`. Advanced overrides
are accepted only when they form one of the six exact supported installer
tuples:

| `GPU_DRIVER_VERSION` | `GPU_DRIVER_BUILD` | Resulting AMD driver |
|---|---|---|
| `7.0.2` | `70002-1` | `30.10.2` |
| `7.1.1` | `70101-1` | `30.20.1` |
| `7.2.3` | `70203-1` | `30.30.3` |
| `7.2.4` | `70204-1` | `30.30.4` |
| `31.30` | `313000-1` | `31.30.0` |
| `31.40` | `314000-1` | `31.40.0` |

Unsupported override combinations fail before download.

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

Useful host checks are:

```bash
dpkg-query -W amdgpu-install amdgpu-dkms
dkms status -m amdgpu
modinfo -n amdgpu
modinfo -F version amdgpu
cat /sys/module/amdgpu/srcversion
modinfo -F srcversion amdgpu
amd-smi version
amd-smi list
```

`/lib/modules/<kernel>/updates/dkms/amdgpu.ko*` identifies the selected DKMS
module. Matching active and selected `srcversion` values confirms that the
selected module is active.

The host must also expose `/dev/kfd` and at least one `/dev/dri/renderD*`
device. Kubernetes validation should confirm the NFD AMD GPU label and the
`amd.com/gpu` allocatable resource.
