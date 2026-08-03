# AMD GPU Driver and Container ROCm Support

ClusterBloom manages the AMD GPU kernel driver for containerized ROCm workloads.
It does not install a host ROCm runtime, HIP SDK, or workload libraries.
Standalone AMD-SMI diagnostics are installed by default.

For the full GPU driver compatibility table, see
[GPU Driver Support](docs/gpu-driver-support.md#supported-version-matrix).

The associated ROCm version identifies AMD's coordinated release train and the
matching standalone AMD-SMI package. ClusterBloom does not require or install
that ROCm version on the host.

## Installation policy

1. Detect `amdgpu-dkms` package metadata and DKMS registrations.
2. Retain an existing driver only when it matches one exact supported tuple.
3. On a fresh node, install production driver `31.40.0` from
   `amdgpu-install_31.40.314000-1_all.deb` with:

   ```bash
   amdgpu-install --usecase=dkms
   ```

4. Stop before repository or package changes when an unknown, ambiguous, or
   mixed out-of-tree driver is detected.
5. Verify that DKMS built the effective module for the running kernel and that
   `modprobe` selects `/lib/modules/<kernel>/updates/dkms/amdgpu.ko*`.
6. When the selected DKMS module is not yet active, write
   `/var/lib/bloom/reboot-required.json` and end the play before GPU-dependent
   deployment. The CLI offers to reboot; `--yes` auto-confirms.
7. After rebooting, rerun Bloom. It verifies the active module and clears the
   marker. A persistent requirement after an attempted reboot stops with manual
   diagnostics instead of entering a reboot loop.
8. Install and verify standalone AMD-SMI matched to the effective driver unless
   `GPU_INSTALL_HOST_TOOLS` is `false`.

## Configuration

### `GPU_NODE`

Enables GPU driver preparation and Kubernetes GPU integration.

### `GPU_DRIVER_SKIP_INSTALL`

Default: `false`

Set to `true` to skip driver validation, driver installation, and standalone
AMD-SMI, leaving the host GPU stack untouched.

### `GPU_INSTALL_HOST_TOOLS`

Default: `true`

Set to `false` for a strict kernel-driver-only host without standalone AMD-SMI.

### `GPU_DRIVER_VERSION` and `GPU_DRIVER_BUILD`

Default: empty, resolving to `31.40` and `314000-1`.

Both values must be set together and match an installer pair in the
[full GPU driver compatibility table](docs/gpu-driver-support.md#configuration).

### `GPU_STACK_FAMILY`

Selects the ClusterForge AMD GPU Operator and DeviceConfig profile. It does not
select or install host ROCm. Empty resolves to `instinct`; `radeon` selects the
tech-preview operator profile.

## Verification

Run these checks on the host:

```bash
dpkg-query --show amdgpu-install amdgpu-dkms
dkms status --module amdgpu
modinfo --filename amdgpu
modinfo --field version amdgpu
amd-smi version
amd-smi list
```

The host must expose `/dev/kfd` and at least one `/dev/dri/renderD*` device.
Kubernetes must report the NFD AMD GPU label and `amd.com/gpu` allocatable
resource.

For detailed behavior and recovery guidance, see:

- [GPU Driver Support](docs/gpu-driver-support.md)
- [AMD GPU Driver and Container ROCm Support](docs/rocm-support.md)
- [Configuration Reference](docs/configuration-reference.md)
