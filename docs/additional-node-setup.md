# Adding Nodes to an Existing Cluster

After the first node is set up, `bloom` generates an `additional_node_command.txt` file in your bloom directory. This file contains ready-to-use commands for creating `bloom.yaml` on the new node.

## Node Types

| Node Type | Use Case | GPU Required |
|-----------|----------|--------------|
| **GPU Worker Node** | Runs GPU workloads (default) | Yes |
| **CPU Control Node** | Runs control plane workloads without GPU | No |

---

## Setup Steps

### Step 1. Create `bloom.yaml`

Choose the command that matches the node you are adding:

**For GPU Worker Node (default):**

```bash
echo -e 'FIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml
```

**For CPU Control Node:**

```bash
echo -e 'FIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>\nSKIP_RANCHER_PARTITION_CHECK: true\nGPU_NODE: false' > bloom.yaml
```

- `SKIP_RANCHER_PARTITION_CHECK: true` — Skips the Rancher disk partition validation that expects GPU node disk layout
- `GPU_NODE: false` — Tells bloom not to install GPU drivers or configure GPU-specific resources

---

### Step 2. Storage Configuration

Add a storage parameter to `bloom.yaml` based on your disk situation:

**Option A — Pre-mounted Disk** (`CLUSTER_PREMOUNTED_DISKS`):
Use when the disk is already formatted and mounted (e.g., at `/mnt/disk0`).

```bash
echo 'CLUSTER_PREMOUNTED_DISKS: /mnt/disk0' >> bloom.yaml
```

- Common in cloud VMs with pre-attached data volumes
- bloom will use the existing mount without repartitioning
- Value format: a single mount path (e.g., `/mnt/disk0`)

**Option B — Raw Disk** (`CLUSTER_DISKS`):
Use when the disk is unformatted and bloom should partition and format it.

```bash
echo 'CLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1' >> bloom.yaml
```

- Common with bare metal servers with unformatted NVMe drives
- bloom will handle partitioning and formatting
- Value format: comma-separated device paths (e.g., `/dev/nvme0n1,/dev/nvme1n1`)

---

### Step 3. Run bloom

```bash
sudo ./bloom cli bloom.yaml
```

---

## Quick Reference

| Scenario | Parameters to Add |
|----------|-------------------|
| GPU worker node | *(none beyond base parameters)* |
| CPU control node | `SKIP_RANCHER_PARTITION_CHECK: true`, `GPU_NODE: false` |
| Pre-mounted disk | + `CLUSTER_PREMOUNTED_DISKS: /mnt/disk0` |
| Raw disk | + `CLUSTER_DISKS: /dev/nvme0n1` |
| Multiple raw disks | + `CLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1` |

---

## Troubleshooting

**"Partition check failed" error on CPU node**
- Add `SKIP_RANCHER_PARTITION_CHECK: true` to your bloom.yaml

**Node joins but no storage is recognized**
- Verify either `CLUSTER_PREMOUNTED_DISKS` or `CLUSTER_DISKS` is set correctly
- For pre-mounted: confirm the path exists with `df -h`
- For raw disks: confirm the device exists with `lsblk`

**GPU driver installation fails on CPU node**
- Ensure `GPU_NODE: false` is set in bloom.yaml

---

## Related Documentation

- [Storage Management](./storage-management.md)
- [Configuration Reference](./configuration-reference.md)
- [Installation Guide](./installation-guide.md)
