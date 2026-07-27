# GPU Driver-Only Spike (EAI-5657)

> **Branch scope**: this describes the `EAI-5657_gpudriver_over_rocm_spike` branch, which
> replaces cluster-bloom's normal ROCm(+driver) install with a driver-only install. It
> exists to test whether the AIM/AIWB/AIRM reference stack works correctly against a
> host that has only the `amdgpu` kernel driver, with **no host ROCm userspace** at all.
> This is not merged to `main`.

## What changed

On a GPU node (`GPU_NODE: true`), bloom now installs **only** the `amdgpu` kernel driver
via `amdgpu-install --usecase=dkms`, instead of the full ROCm+driver install
(`amdgpu-install --usecase=rocm,dkms`) that `main` performs. No ROCm userspace
(`/opt/rocm*`, `amd-smi`, `rocm-smi`, `rocm-*` packages) is installed by this branch.

Everything downstream is unchanged: the AMD GPU Operator, its `DeviceConfig`, and the
AIM/AIWB/AIRM deploy steps still run exactly as they do on `main`.

## Config flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `GPU_DRIVER_SKIP_INSTALL` | bool | `false` | Skip the amdgpu driver install step entirely (e.g. pre-provisioned node, or you want to manage the driver yourself). Also skips the guard that fails when ROCm is already installed. |
| `GPU_DRIVER_VERSION` | str | `""` | Force a specific `amdgpu-install` version (e.g. `7.2.3` or `31.30`) instead of the `GPU_STACK_FAMILY` default. Must be paired with `GPU_DRIVER_BUILD`. |
| `GPU_DRIVER_BUILD` | str | `""` | Build suffix paired with `GPU_DRIVER_VERSION` (e.g. `70203-1` or `313000-1`), forming the `amdgpu-install_<version>.<build>_all.deb` filename. |

`GPU_STACK_FAMILY` (`instinct` | `radeon`, existing flag) still selects the default
driver package when `GPU_DRIVER_VERSION`/`GPU_DRIVER_BUILD` are unset, and still selects
the GPU Operator chart + `DeviceConfig` driver-version pin downstream — that part is
unrelated to the host driver and unchanged from `main`.

| `GPU_STACK_FAMILY` | Default `amdgpu-install` version.build |
|---|---|
| `instinct` (default) | `7.2.3.70203-1` (legacy `repo.radeon.com` series) |
| `radeon` | `31.30.313000-1` (newer series with better RDNA/radeon card detection) |

The `radeon` default is a **starting point for this spike**, not a validated pin — it
was carried over from the EAI-7530 branch's radeon installer selection (chosen there for
GPU auto-detection reasons, not driver-only testing). Override it with
`GPU_DRIVER_VERSION`/`GPU_DRIVER_BUILD` if it turns out to be wrong for the hardware
under test.

## What's out of scope for this branch

- **No ROCm install at all.** All ROCm-adjacent code from `main`/EAI-7530 (ROCm version
  detection, the `ROCM_ALLOW_VERSION_MISMATCH` guard, `ROCM_BASE_URL`/`ROCM_DEB_PACKAGE`)
  has been removed — there's no ROCm version to detect or gate on.
- **No reconciliation of a node that already has ROCm.** If ROCm is already installed,
  the driver-only install step fails fast with a clear message. This branch only handles
  a clean node with no ROCm and (optionally) no driver yet. Uninstalling an existing
  ROCm install and replacing it is explicitly out of scope here.
- **No changes to the GPU Operator / cluster-forge deploy.** `DeviceConfig`,
  `driver.enable`, and the operator chart selection are all unchanged from `main`.

## Reboot handling

Installing the `amdgpu` DKMS kernel module can leave a node needing a reboot before the
driver is actually usable. Bloom detects the OS's own `/var/run/reboot-required` signal
and directly compares the active module with the mapped DKMS module right after the
driver install. When a reboot is required, Bloom ends the play successfully at that
point—before later node preparation or cluster deployment—and offers to reboot:

```bash
sudo ./bloom cli bloom.yaml             # normal full installation
sudo ./bloom cli bloom.yaml -y          # auto-confirms and reboots
sudo ./bloom cli bloom.yaml --tags gpu  # targeted test on an existing cluster
```

Re-running bloom after a reboot is idempotent: the driver-only step verifies that the
installed `amdgpu-dkms` package is the exact candidate from the family-mapped installer
repository, that DKMS built it for the running kernel, and that the active module matches
the mapped module on disk. A preloaded Ubuntu inbox `amdgpu` module does **not** satisfy
this check. If the mapped package/build is already correct, installation is skipped and
bloom proceeds to verification. A loop-guard prevents bloom from offering — or
automatically triggering — a reboot more than once for the same unresolved condition.

## Manual test plan

1. **Fresh instinct node**, no ROCm/driver present. Set `GPU_STACK_FAMILY: instinct` (or
   leave unset) and run bloom. Verify:
   - `lsmod | grep amdgpu` shows the module loaded (after a reboot if one was required).
   - `/dev/kfd` and `/dev/dri/renderD*` are present.
   - The AMD GPU Operator registers the node (`amd.com/gpu` resource,
     `feature.node.kubernetes.io/amd-gpu=true` label) — no host ROCm required.
   - Deploy an AIM model pod (e.g. an `amdenterpriseai/aim-*` image) and confirm inference
     works end-to-end.
2. **Fresh radeon node**, same as above with `GPU_STACK_FAMILY: radeon`.
3. **AIWB / AIRM**: deploy against the cluster and verify their GPU-dependent
   functionality (metrics collection, scheduling, etc.) works without host ROCm.
4. **Guard check**: confirm bloom refuses to run the driver-only install (clear failure
   message) on a node that already has ROCm installed, and that
   `GPU_DRIVER_SKIP_INSTALL: true` cleanly skips the step instead.

## Result

Document the yes/no outcome (does the reference stack need host ROCm beyond the
`amdgpu` driver?) back on [EAI-5657](https://amd.atlassian.net/browse/EAI-5657). If host
ROCm turns out to be required for any component, note exactly which one and why.
