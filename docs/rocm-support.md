# AMD GPU Driver and Container ROCm Support

## Overview

ClusterBloom prepares AMD GPU nodes for containerized ROCm workloads without
installing a host ROCm runtime, HIP SDK, or workload libraries. It manages an
allowlisted out-of-tree `amdgpu` DKMS driver, configures device access, and
integrates the node with the AMD GPU Operator.

Bloom installs standalone AMD-SMI diagnostics by default. This does not turn
the host into a full ROCm environment. Existing host ROCm userspace is left
untouched; the detected driver tuple determines compatibility.

## Supported drivers

For the full GPU driver compatibility table, see
[GPU Driver Support](gpu-driver-support.md#supported-version-matrix).

The default fresh-node installation is driver `31.40.0`, using
`amdgpu-install_31.40.314000-1_all.deb` and
`amdgpu-install --usecase=dkms`.

See [GPU Driver Support](gpu-driver-support.md) for package
selection, detection, fail-safe behavior, and verification details.

## Installation behavior

For each GPU node, Bloom:

1. Detects `amdgpu-dkms` package metadata and DKMS registrations.
2. Retains the driver when it exactly matches a supported tuple.
3. Installs the production-default `31.40.0` driver on a fresh node that has no
   out-of-tree AMD DKMS package or registration.
4. Stops before repository or package changes when it detects an unsupported,
   ambiguous, or mixed out-of-tree driver.
5. Verifies that DKMS built the expected module for the running kernel and that
   `modprobe` resolves to `/lib/modules/<kernel>/updates/dkms/amdgpu.ko*`.
6. Requests a reboot when the selected DKMS module is not yet active, then
   verifies the active module on the next run.
7. Installs and verifies the standalone AMD-SMI package matched to the driver,
   unless `GPU_INSTALL_HOST_TOOLS` is `false`.

Set `GPU_DRIVER_SKIP_INSTALL: true` to leave the host GPU stack untouched,
including compatibility validation and standalone AMD-SMI installation.

## GPU family and operator selection

`GPU_STACK_FAMILY` selects ClusterForge's vendored AMD GPU Operator and
DeviceConfig profile. It does not select or install host ROCm.

| Family | GPU Operator path | DeviceConfig driver train | Status |
|---|---|---|---|
| `instinct` (default) | `amd-gpu-operator/v1.4.1` | `7.0` | qualified |
| `radeon` | `amd-gpu-operator/v1.5.1-beta.0` | `7.13` | tech preview |

The host driver default remains `31.40.0` for both families. The family setting
is independent of `AIM_HARDWARE_FAMILY`, which selects the AIM model catalog.

## Standalone AMD-SMI

When `GPU_INSTALL_HOST_TOOLS` is `true`, Bloom retains an existing `amd-smi`
binary or installs the package associated with the effective driver:

- Drivers `30.10.2` through `30.30.4`: exact `amd-smi-lib` build from the
  corresponding ROCm repository.
- Driver `31.30.0`: `amdrocm-amdsmi7.13`.
- Driver `31.40.0`: `amdrocm-amdsmi7.14`.

Bloom verifies both `amd-smi version` and `amd-smi list`.

## Device validation

Bloom verifies:

- the active `amdgpu` kernel module and its source version;
- `/dev/kfd` and at least one `/dev/dri/renderD*` device;
- the NFD AMD GPU label; and
- the `amd.com/gpu` allocatable Kubernetes resource.

Useful host checks:

```bash
dpkg-query -W amdgpu-install amdgpu-dkms
dkms status -m amdgpu
modinfo -n amdgpu
modinfo -F version amdgpu
amd-smi version
amd-smi list
```

## Containerized ROCm workloads

ROCm workload libraries belong in the container image. A GPU pod requests the
resource exposed by the AMD device plugin:

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
```
