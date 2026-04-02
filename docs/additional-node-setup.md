# Adding Nodes to an Existing Cluster

After the first node is set up, `bloom` generates an `additional_node_command.txt` file in your bloom directory. This file contains ready-to-use commands for adding more nodes to the cluster.

## Node Types

| Node Type | Use Case | GPU Required |
|-----------|----------|--------------|
| **GPU Worker Node** | Runs GPU workloads (default) | Yes |
| **CPU Control Node** | Runs control plane workloads without GPU | No |

---

## Using additional_node_command.txt

The file contains two commands — pick the one that matches the node you are adding.

### For GPU Worker Node (default)

```bash
echo -e 'FIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml && sudo ./bloom cli bloom.yaml
```

Use this when the new node has a GPU and will serve as a worker node.

### For CPU Control Node

```bash
echo -e 'FIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>\nSKIP_RANCHER_PARTITION_CHECK: true\nGPU_NODE: false' > bloom.yaml && sudo ./bloom cli bloom.yaml
```

Use this when the new node has no GPU and will serve as a control plane node.

**Key parameters for CPU control nodes:**

- `SKIP_RANCHER_PARTITION_CHECK: true` — Skips the Rancher disk partition validation that expects GPU node disk layout
- `GPU_NODE: false` — Tells bloom not to install GPU drivers or configure GPU-specific resources

---

## Storage Configuration

CPU control nodes typically need storage configured. Add one of the following parameters to the command depending on your disk situation.

### Option 1: Pre-mounted Disk (`CLUSTER_PREMOUNTED_DISKS`)

Use this when the disk is already formatted and mounted on the node.

```bash
echo -e 'FIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>\nSKIP_RANCHER_PARTITION_CHECK: true\nGPU_NODE: false\nCLUSTER_PREMOUNTED_DISKS: /mnt/disk0' > bloom.yaml && sudo ./bloom cli bloom.yaml
```

**When to use:**
- The disk is already formatted (e.g., ext4, xfs) and mounted at a path like `/mnt/disk0`
- You want bloom to use the existing mount without repartitioning
- Common in cloud VMs with pre-attached data volumes

**Value format:** A single mount path (e.g., `/mnt/disk0`)

---

### Option 2: Raw Disk (`CLUSTER_DISKS`)

Use this when the disk is unformatted and bloom should partition and format it.

```bash
echo -e 'FIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>\nSKIP_RANCHER_PARTITION_CHECK: true\nGPU_NODE: false\nCLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1' > bloom.yaml && sudo ./bloom cli bloom.yaml
```

**When to use:**
- The disk is a raw block device with no filesystem
- You want bloom to handle partitioning and formatting
- Common with bare metal servers that have unformatted NVMe drives

**Value format:** Comma-separated device paths (e.g., `/dev/nvme0n1,/dev/nvme1n1`)

---

## Quick Reference

| Scenario | Parameters to Add |
|----------|-------------------|
| GPU worker node | *(none — use default command)* |
| CPU control node, no extra storage | `SKIP_RANCHER_PARTITION_CHECK: true`, `GPU_NODE: false` |
| CPU control node, pre-mounted disk | + `CLUSTER_PREMOUNTED_DISKS: /mnt/disk0` |
| CPU control node, raw disk | + `CLUSTER_DISKS: /dev/nvme0n1` |
| CPU control node, multiple raw disks | + `CLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1` |

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
