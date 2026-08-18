# Cluster Teardown

This guide covers tearing down a multi-node Bloom cluster safely. It complements [Cluster Setup and Node Management](additional-node-setup.md) (setup order) and [Storage Management](storage-management.md) (per-node cleanup internals).

Bloom cleanup is **local to each node**. There is no single cluster-wide command. Run `sudo ./bloom cleanup bloom.yaml` on every node that participated in the cluster, using **that node's** `bloom.yaml`.

## Teardown Order Overview

Teardown is the **reverse** of the setup workflow documented in [additional-node-setup.md](additional-node-setup.md):

| Setup step | Teardown step |
|------------|---------------|
| 1. First control plane | **Last** — bootstrap control plane |
| 2. Additional control planes (HA, odd count) | Before bootstrap — additional control planes |
| 3. Worker nodes | **First** — workers (if any) |
| 4. ClusterForge (on first CP) | No separate step — removed with control plane nodes |

For a **3-node HA control plane** cluster (no separate workers):

1. Run preflight on all three nodes.
2. Clean up **additional** control plane nodes (CP2, CP3).
3. Clean up the **bootstrap / first** control plane node last.

For clusters with workers, clean workers before any control plane node.

## Why Order Matters

Each `bloom cleanup` run:

- Drains **only the local node** (matched by hostname) when the Kubernetes API is reachable.
- Performs Longhorn teardown for **local** artifacts (mounts, iSCSI sessions, kubelet CSI state).
- Uninstalls RKE2 and wipes that node's bloom-managed disks.

While two or more control plane nodes remain, etcd keeps quorum and the API stays available for remaining nodes to self-drain. Cleaning the bootstrap control plane first removes an etcd member and may leave fewer nodes able to coordinate drains and Longhorn volume detaches.

Do **not** run cleanup on multiple nodes in parallel. Process one node at a time.

## Prerequisites

- Root access on each node (`sudo`).
- Each node's `bloom.yaml` with correct storage settings (`CLUSTER_DISKS`, `CLUSTER_PREMOUNTED_DISKS`, `RANCHER_DISK` as deployed).
- Optional but recommended: `kubectl` access from any control plane node to inspect workloads before teardown.

## Step 0: Preflight on Every Node

Run non-destructive checks on **each** node before tearing anything down:

```bash
sudo ./bloom cleanup bloom.yaml --preflight-only
```

Preflight validates that `bloom.yaml`, bloom-managed `/etc/fstab` entries, live mounts, and block-device identities agree. Any mismatch aborts cleanup before RKE2, Longhorn, or disk changes. Fix reported issues on that node before proceeding.

See [Storage Management — Cleanup Behaviour](storage-management.md#cleanup-behaviour) for preflight rules.

## Step 1: Optional Cluster-Level Preparation

From any node with a working kubeconfig (`/etc/rancher/rke2/rke2.yaml`):

```bash
# Inspect remaining workloads
kubectl get pods -A -o wide

# Remove test or non-essential workloads (example)
kubectl delete pod bloom-cleanup-test-pod --ignore-not-found
kubectl delete pvc bloom-cleanup-test-pvc --ignore-not-found
```

On large clusters (`CLUSTER_SIZE: large`), Longhorn runs in the **`longhorn`** namespace (not `longhorn-system`):

```bash
kubectl get volumes.longhorn.io -n longhorn
kubectl get pods -n longhorn
```

Nodes that still have Longhorn-backed volumes attached benefit from being cleaned **before** control plane nodes that hold no volume attachments. Bloom's per-node drain is best-effort (~30s timeout, `--force --disable-eviction`); evacuating workloads manually reduces stuck-volume risk.

## Step 2: Worker Nodes (If Any)

On each worker node:

```bash
sudo ./bloom cleanup bloom.yaml
```

Review the disk wipe preview and confirm when prompted. Use `--force` (or `--yes`) only when you accept the preview without an interactive prompt.

Repeat for every worker before touching control plane nodes.

## Step 3: Additional Control Plane Nodes

On each control plane node **except** the bootstrap (first) node:

```bash
sudo ./bloom cleanup bloom.yaml
```

Identify the bootstrap node: it was deployed with `FIRST_NODE: true` and holds the initial join token source (`additional_node_command.txt` was generated there).

## Step 4: Bootstrap Control Plane (Last)

On the first control plane node:

```bash
sudo ./bloom cleanup bloom.yaml
```

After this completes, the Kubernetes cluster no longer exists.

## What Each Cleanup Run Does

Every `bloom cleanup` executes the same per-node sequence (also described in `bloom cleanup --help` and [Storage Management](storage-management.md#cleanup-behaviour)):

1. Destructive storage preflight (config, fstab, live mounts, protected OS devices).
2. Longhorn cleanup when local Longhorn artifacts are detected — skipped on `CLUSTER_SIZE: small`/`medium` clusters with no Longhorn present:
   - Best-effort cordon and drain of the local node (~30s).
   - Longhorn-only iSCSI logout (`iqn.2019-10.io.longhorn:*`).
   - Stop Longhorn processes and force-unmount CSI/kubelet volumes.
3. RKE2 uninstall and removal of RKE2 directories.
4. Pre-clean bloom artifacts from the future mount range.
5. Clean premounted disks (preserve filesystem and fstab).
6. Remove bloom-managed fstab entries and wipe `CLUSTER_DISKS`.

`bloom cli bloom.yaml --destroy-data` runs the same cleanup logic on that node, then redeploys. It cannot be combined with `--export`.

## Redeploying After Teardown

To wipe and redeploy a single node in place:

```bash
sudo ./bloom cli bloom.yaml --destroy-data
```

For a full multi-node redeploy, tear down all nodes in the order above, then follow [additional-node-setup.md](additional-node-setup.md) setup order from the first control plane node.

## 3-Node HA Control Plane Example

Assuming three control plane nodes and no workers:

```
Node A — bootstrap CP (FIRST_NODE: true)     → clean LAST
Node B — additional CP                       → clean 2nd
Node C — additional CP                       → clean 1st (or 2nd; order between B and C is flexible)
```

```bash
# All nodes: preflight
sudo ./bloom cleanup bloom.yaml --preflight-only

# Node C
sudo ./bloom cleanup bloom.yaml

# Node B
sudo ./bloom cleanup bloom.yaml

# Node A (bootstrap)
sudo ./bloom cleanup bloom.yaml
```

## Troubleshooting

**Preflight fails on a node**

- Ensure you are using that node's `bloom.yaml`, not a copy from another host.
- Compare `CLUSTER_DISKS` / `CLUSTER_PREMOUNTED_DISKS` with `grep cluster-bloom /etc/fstab` and `lsblk`.
- See [Storage Management](storage-management.md) for fstab tag and mount-index rules.

**Drain skipped or API unreachable**

- Cleanup continues with local Longhorn and RKE2 teardown. This is expected if you already removed other control plane nodes and the API is down.
- For graceful Longhorn detach, clean nodes while the API still has quorum.

**Longhorn cleanup skipped ("No Longhorn detected")**

- Typical on small/medium clusters without Longhorn, or on nodes with no local Longhorn artifacts.
- On large clusters, Longhorn artifacts appear on nodes running attached volumes (check `/dev/longhorn/`, kubelet CSI paths, and mounts — not only `iscsiadm`).

**Interrupted cleanup**

- Bloom defers a single Ctrl-C until the current disk wipe finishes. A second Ctrl-C force-exits and may leave a disk mid-operation.
- See [Storage Management — Interrupted Cleanup Safety](storage-management.md#cleanup-behaviour).

## Related Documentation

- [Cluster Setup and Node Management](additional-node-setup.md) — Setup order (reverse for teardown)
- [Storage Management](storage-management.md) — Per-node cleanup sequence, preflight, Longhorn iSCSI behaviour
- [Configuration Reference — Cleanup Command](configuration-reference.md#cleanup-command) — Flags and command reference
- [Installation Guide](installation-guide.md) — Deployment verification commands
