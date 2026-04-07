# Adding Nodes to an Existing Cluster

After the first node is set up, `bloom` generates an `additional_node_command.txt` file in your bloom directory. This file contains ready-to-use commands for creating `bloom.yaml` on the new node.

> This guide covers adding nodes to a **large** size cluster (`CLUSTER_SIZE: large`).

## Node Types

| Node Type | Use Case | GPU Required |
|-----------|----------|--------------|
| **GPU Worker Node** | Runs GPU workloads (default) | Yes |
| **CPU Worker Node** | Runs workloads without GPU | No |
| **CPU Control Node** | Runs control plane workloads without GPU | No |

---

## Setup Steps

### Step 1. Create `bloom.yaml`

Choose the command that matches the node you are adding:

**For GPU Worker Node (default):**

```bash
echo -e 'FIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>\nCLUSTER_SIZE: large' > bloom.yaml
```

**For CPU Worker Node:**

```bash
echo -e 'FIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>\nCLUSTER_SIZE: large\nGPU_NODE: false\nCONTROL_PLANE: false' > bloom.yaml
```

**For CPU Control Node:**

```bash
echo -e 'FIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>\nGPU_NODE: false\nCLUSTER_SIZE: large\nCONTROL_PLANE: true' > bloom.yaml
```

- `GPU_NODE: false` — Tells bloom not to install GPU drivers or configure GPU-specific resources
- `CONTROL_PLANE: true` — Designates this node as part of the control plane

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
| GPU worker node | `GPU_NODE: true`, `CONTROL_PLANE: false` |
| CPU worker node | `GPU_NODE: false`, `CONTROL_PLANE: false` |
| CPU control node | `GPU_NODE: false`, `CONTROL_PLANE: true` |
| Pre-mounted disk | + `CLUSTER_PREMOUNTED_DISKS: /mnt/disk0` |
| Raw disk | + `CLUSTER_DISKS: /dev/nvme0n1` |
| Multiple raw disks | + `CLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1` |

---

## Troubleshooting

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
