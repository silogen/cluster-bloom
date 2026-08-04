# Longhorn V2 Data Engine Migration Evaluation

> **Status: implemented** (Longhorn v1.12.0, V2 data engine enabled by default via `LONGHORN_V2_DATA_ENGINE`).
> Bloom deploys Longhorn v1.12.x with the V2 data engine and block-type disks when
> `LONGHORN_V2_DATA_ENGINE: true` (default). Set `LONGHORN_V2_DATA_ENGINE: false`
> to retain the V1 filesystem-disk path.

## Verdict

Migrating to the V2 data engine removes a large amount of bloom's low-level disk
code, but the savings land on the **deployment** path rather than the cleanup
path. There is no Longhorn API or CLI operation that returns a host disk to a
reusable state — Longhorn itself requires `wipefs -a` before a block disk can be
added, so at least one destructive host-level step remains permanently.

V2 also introduces a new failure mode that closely resembles the problem bloom
already had to fix: with the `nvme` or `virtio-blk` disk drivers, SPDK claims the
device in userspace and the kernel block device disappears from the host by
design. Cleanup logic must then wait for the device to return, and in some known
upstream failure cases manually unbind the PCI driver.

Net assessment: worth doing for the deployment simplification and for dropping
the iSCSI teardown hazards, but it is not a path to "cleanup is just an API
call," and it trades disk-level complexity for hugepage and PCI-driver
complexity.

## Current State

Bloom deploys the V1 data engine with filesystem-type disks. Every `CLUSTER_DISKS`
device is formatted, mounted, and registered with Longhorn through a mount point:

| Concern | Current implementation |
| --- | --- |
| Longhorn version | `pkg/ansible/runtime/manifests/longhorn/longhorn.yaml` (image tags `v1.8.0`) |
| Preflight check | `pkg/ansible/runtime/manifests/scripts/longhorn_preflight_check.sh` (downloads `longhornctl` `v1.10.0`) |
| Disk preparation | `wipefs` + `mkfs.ext4` + mount at `/mnt/diskN` + fstab entry |
| Mount index allocation | Lowest contiguous range not colliding with `CLUSTER_PREMOUNTED_DISKS` |
| Disk registration | `node.longhorn.io/default-disks-config` annotation written by the node-annotator CronJob, gated on the `node.longhorn.io/create-default-disk=config` label |
| Volume teardown | `iscsiadm` logout of Longhorn-only sessions, `pkill` of Longhorn binaries, multi-pass force unmount of CSI/kubelet paths |
| Disk teardown | fstab entry removal, unmount, `wipefs -a`, `mkfs.ext4 -F` |
| Artifact cleanup | `rm -rf` of `pvc-*`, `replicas`, `longhorn-disk.cfg` from mount points |

There is a version skew worth resolving independently of this evaluation: the
bundled manifests install v1.8.0 while the preflight script fetches `longhornctl`
v1.10.0.

## What the V2 Data Engine Changes

### Block-type disks are raw devices

The V2 data engine stores volume data on `diskType: block` disks, which are used
as raw block devices with no filesystem. V1's `diskType: filesystem` disks must be
formatted with an extent-based filesystem and mounted to a host directory. This is
the single most consequential difference for bloom, because the mount point is the
root of most of bloom's disk machinery.

A block disk is declared on the node resource as:

```yaml
spec:
  disks:
    block-disk:
      allowScheduling: true
      diskDriver: auto
      diskType: block
      evictionRequested: false
      path: /dev/disk/by-id/wwn-0x5000c500a0b1c2d3
      storageReserved: 0
      tags: []
```

`diskDriver: auto` resolves per device: `nvme` for NVMe disks, `virtio-blk` for
VirtIO block disks, and `aio` (Linux AIO) for everything else. NVMe and VirtIO
disks may be addressed by PCI BDF notation (for example `0000:05:00.0`), which is
stable across reboots; AIO disks use a conventional device path, and a persistent
symlink under `/dev/disk/by-id/` should be preferred over kernel names like
`/dev/sdb`.

### The data path moves from iSCSI to NVMe-TCP

V1 exposes volumes to the node over iSCSI, which is why bloom must log out
Longhorn iSCSI sessions carefully — an unscoped logout would detach a cloud boot
volume. V2 uses NVMe-TCP, with an experimental UBLK frontend that only works on
kernels below v6.17. The iSCSI-specific teardown and its root-disk hazard do not
apply to V2 volumes.

### Disk registration already fits

Bloom's node-annotator writes `node.longhorn.io/default-disks-config`, which
already accepts the fields a block disk needs. Moving to block disks is a change
in what the annotator emits (device path instead of mount point, plus `diskType`
and `diskDriver`), not a change in mechanism.

## Bloom Code Affected

| Current component | Status under V2 |
| --- | --- |
| `mkfs.ext4` of cluster disks | Not needed — block disks are raw |
| `/mnt/diskN` mounting | Not needed |
| fstab entry management for cluster disks | Not needed |
| Mount index allocation and premounted-index collision avoidance | Not needed for block disks |
| `PrecleanFutureMountPoints` | Not applicable — no mount point to pre-clean |
| `pvc-*` / `replicas` / `longhorn-disk.cfg` artifact sweeps | Not applicable — these are filesystem-disk artifacts |
| `logoutLonghornISCSISessions` and the `iqn.2019-10.io.longhorn` filter | Not applicable to V2 volumes |
| Multi-pass unmount of CSI and kubelet paths | Still applies to the CSI mount of the volume on the node, but the block disk itself is never mounted |
| `wipefs -a` of cluster disks | **Still required** — Longhorn refuses block disks with a filesystem or partition table |
| `RANCHER_DISK` handling (mkfs, mount, bind mount, fstab) | Unchanged — not a Longhorn disk |
| `CLUSTER_PREMOUNTED_DISKS` | Unchanged if premounted disks remain filesystem-type V1 disks |

## Longhorn-Native Teardown Operations

These are the supported API-driven equivalents of what bloom currently does at
the host level.

### Draining and removing a disk

The documented sequence is to disable scheduling for the disk, ensure no replicas
or backing images remain on it (including any in an error state), and only then
remove it from the node configuration. Eviction is requested through the node
resource:

```bash
kubectl --namespace longhorn-system edit node.longhorn.io <node-name>
# set allowScheduling: false and evictionRequested: true on the target disk
```

### Uninstalling Longhorn

Uninstallation is gated behind a confirmation setting, then performed by Helm or
by a dedicated job that detaches volumes and removes custom resources in order:

```bash
kubectl --namespace longhorn-system patch --type=merge \
  --patch '{"value": "true"}' lhs deleting-confirmation-flag
helm uninstall longhorn --namespace longhorn-system
```

Or without Helm:

```bash
kubectl create --filename https://raw.githubusercontent.com/longhorn/longhorn/v1.12.0/uninstall/uninstall.yaml
kubectl --namespace longhorn-system get job/longhorn-uninstall --watch
kubectl delete --filename https://raw.githubusercontent.com/longhorn/longhorn/v1.12.0/deploy/longhorn.yaml
kubectl delete --filename https://raw.githubusercontent.com/longhorn/longhorn/v1.12.0/uninstall/uninstall.yaml
```

This is a genuine replacement for bloom's `pkill` of Longhorn binaries and its
force-unmount sweeps — but only while the cluster is still reachable.

### Orphaned replica data

Leftover replica data is modelled as orphan resources rather than being bloom's
responsibility to `rm -rf`:

```bash
kubectl --namespace longhorn-system get orphans.longhorn.io
kubectl --namespace longhorn-system delete orphan.longhorn.io <orphan-name>
```

Automatic deletion is configured with the `orphan-resource-auto-deletion`
setting, which accepts a semicolon-separated list such as `replica-data;instance`.
Orphans on failed or unknown nodes are not auto-cleaned.

### `longhornctl` scope

`longhornctl` covers `install preflight`, `check preflight`, `export replica`,
`trim volume`, `get replica`, and in v1.12 on-demand snapshot checksum
calculation. **It has no uninstall or disk-cleanup verb.** Bloom already invokes
`longhornctl check preflight`, so the integration point exists, but it cannot
perform teardown.

## What Still Requires Host-Level Operations

1. **`wipefs` becomes mandatory, not optional.** Since v1.11.0 Longhorn refuses to
   add a block disk containing an existing filesystem or partition table, and the
   documented remedy is `wipefs -a /path/to/block/device`. Bloom keeps this step
   on the redeploy path.
2. **Disk removal does not scrub the device.** Removing a disk from `spec.disks`
   stops Longhorn managing it; the SPDK lvstore metadata remains on the device.
3. **Devices disappear from the host under the `nvme` and `virtio-blk` drivers.**
   SPDK claims the device in userspace through `vfio-pci` / `uio_pci_generic`,
   removing the kernel block device. After disk deletion the device takes up to
   roughly 30 seconds to reappear, and upstream has reported cases where deletion
   fails silently and the disk cannot be re-added without manually running
   `go-spdk-helper setup unbind <BDF>` inside the instance manager
   ([longhorn#11952](https://github.com/longhorn/longhorn/issues/11952),
   [longhorn#11860](https://github.com/longhorn/longhorn/issues/11860)). Any bloom
   cleanup implementation must poll for device reappearance rather than assume it.
   Disks that resolve to the `aio` driver keep the ordinary kernel device path and
   are not affected.
4. **The API path requires a reachable cluster.** `bloom cleanup` is expected to
   work on half-installed or broken nodes where RKE2 may already be dead. API-based
   teardown is an optimization for the healthy case; the host-level fallback has to
   remain.
5. **`RANCHER_DISK` is unaffected.** It backs `/var/lib/rancher`, not Longhorn, so
   its format, mount, bind-mount, and fstab handling stay as they are.

## New Prerequisites V2 Imposes

Node preparation gains requirements that bloom does not currently satisfy:

- **HugePages**: 2 GiB of 2 MiB pages (1024 pages) per V2 node, configured
  persistently via kernel boot parameters.
- **Kernel modules**: `vfio_pci`, `uio_pci_generic`, and `nvme_tcp` loaded and
  configured to load at boot.
- **Kernel version**: 6.7 or later recommended for NVMe/TCP support and stability.
- **CPU**: AMD64 requires SSE4.2. Longhorn v1.12.0 raised the default
  `data-engine-cpu-mask` from `0x1` to `0x3`, so two cores per node are dedicated
  to the SPDK busy-polling reactor.
- **IOMMU**: for SPDK to claim an NVMe disk through `vfio-pci`, the device must be
  in an isolatable IOMMU group; devices sharing a group with a PCIe bridge must use
  AIO mode instead.
- **ARM64 caveat**: on ARM64, NVMe-driver block disks may stall I/O when SPDK is
  configured with two or more cores. The documented workaround is to use
  AIO-backed disks.

## Feature Differences to Account For

From the V1/V2 feature parity matrix for v1.12.0:

- **Not supported in V2**: strict-local data locality, offline fast rebuilding,
  orphaned instance management, revision counters, engine live upgrade.
- **Removed in v1.12.0**: V2 backing images, replaced by the Containerized Data
  Importer (CDI). Existing V2 volumes created from backing images must be migrated
  before upgrading.
- **V2-only**: QoS.
- **Upgrade impact**: V2 volumes must be detached to upgrade between v1.12 patch
  releases; live upgrade support is planned for v1.12 → v1.13.

## Version Path

Longhorn enforces single-minor-version upgrade steps (since v1.5.0) and prevents
downgrades. Consequences for bloom:

- **Fresh deployments** can install v1.12.x directly, which is bloom's normal mode.
- **Existing clusters** cannot go from v1.8.0 to v1.12.0 in one step; that requires
  passing through v1.9, v1.10, and v1.11.
- Kubernetes v1.25 or later is required for v1.12.0 because the CSI external
  snapshotter moved to v8.2.0.

## Proposed Migration Shape

*Implemented as of this document update.*

1. **Decouple the version bump from the engine switch.** Bloom bundles Longhorn
   v1.12.0 manifests aligned with `longhornctl` v1.12.0. Set
   `LONGHORN_V2_DATA_ENGINE: false` to keep V1 filesystem disks on the same version.
2. **Make cleanup API-first with a host fallback.** `bloom cleanup` runs the
   Longhorn uninstall job when the API is reachable, then falls back to host-level
   teardown when it is not.
3. **Add V2 node preparation.** `longhorn_v2_prep.yaml` configures hugepages,
   kernel modules, and runs `longhornctl check preflight`.
4. **Add block-disk support behind `LONGHORN_V2_DATA_ENGINE`.** `block_storage.yaml`
   runs `wipefs -a` only; the node-annotator emits `diskType: block` disks.
5. **Handle device reappearance explicitly.** `CleanupBloomBlockDisks` polls for
   block devices after wipe; hot-remove via `/sys/block/.../device/delete` is not used.

## Open Questions

- Which driver do bloom's target platforms resolve to? MI300 nodes with local NVMe
  would take the `nvme`/`vfio-pci` path with its device-disappearance behavior;
  cloud VMs with virtio-scsi disks resolve to `aio` and avoid it entirely.
- Should `CLUSTER_PREMOUNTED_DISKS` remain V1 filesystem disks, given that its
  contract is explicitly about preserving an existing filesystem and fstab entry?
- Does the two-core SPDK reservation conflict with GPU-node CPU allocation for
  workloads?
- What is the recovery procedure when the uninstall job cannot run because the
  cluster is already broken — does the API-first path degrade cleanly?

## References

- [Longhorn v1.12.0 important notes](https://longhorn.io/docs/1.12.0/important-notes/)
- [V1 and V2 volume behavior and feature parity](https://longhorn.io/docs/1.12.0/v1-v2-volume-behavior-and-feature-parity/)
- [Multiple disks — adding and removing block-type disks](https://longhorn.io/docs/1.12.0/nodes-and-volumes/nodes/multidisk/)
- [Best practices — V2 block-type disks](https://longhorn.io/docs/1.12.0/best-practices/)
- [Uninstall Longhorn](https://longhorn.io/docs/1.12.0/deploy/uninstall/)
- [Upgrade path enforcement](https://longhorn.io/docs/1.12.0/deploy/upgrade/)
- [Command line tool (`longhornctl`)](https://longhorn.io/docs/1.12.0/advanced-resources/longhornctl/)

## See Also

- [Storage Management](storage-management.md) — current Longhorn configuration and cleanup sequence
- [Longhorn Drive Setup and Recovery](longhorn-drive-setup-and-recovery.md) — drive recovery and troubleshooting
- [Technical Architecture](technical-architecture.md) — system design and component interactions
