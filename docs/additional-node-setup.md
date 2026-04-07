# Cluster Setup and Node Management

This guide covers the complete workflow for setting up a highly available cluster with bloom, from the first control plane node to the final cluster configuration.

> This guide covers setting up a **large** size cluster (`CLUSTER_SIZE: large`). Large clusters are typically used for HA setups but can also be single-node deployments.

## Setup Workflow Overview

Follow this sequence for proper cluster setup:

1. **First Control Plane Node** - Initialize your cluster
2. **Additional Control Plane Nodes** - Add remaining control plane nodes for HA (minimum 3 total, always odd number)
3. **Worker Nodes** - Add GPU and CPU worker nodes as needed
4. **ClusterForge** - Return to first control plane node and run clusterforge to complete setup

---

## Step 1: First Control Plane Node Setup

Start by setting up your first control plane node with bloom. This node will serve as the foundation of your cluster.

1. Create your initial `bloom.yaml` configuration
2. Run `sudo ./bloom cli bloom.yaml` to initialize the cluster
3. After setup completes, bloom will generate an `additional_node_command.txt` file in your bloom directory
4. This file contains the join token and server IP needed for additional nodes

---

## Node Types

| Node Type | Use Case | GPU Required |
|-----------|----------|--------------|
| **GPU Worker Node** | Runs GPU workloads (default) | Yes |
| **CPU Worker Node** | Runs workloads without GPU | No |
| **CPU Control Node** | Runs control plane workloads without GPU | No |

---

## Step 2: Additional Control Plane Nodes (HA Setup)

For high availability, add additional control plane nodes **before** adding worker nodes. The total number of control plane nodes must be odd (3, 5, 7, etc.) with a minimum of 3 nodes.

### Create `bloom.yaml` for Additional Control Plane Nodes

Copy the join token and server IP from `additional_node_command.txt` generated on your first control plane node, then create the configuration:

```bash
echo -e 'CLUSTER_SIZE: large\nCONTROL_PLANE: true\nFIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml
```

### Add Storage Configuration

Add storage configuration based on your setup. See the [Storage Configuration](#storage-configuration) section below for detailed options.

### Run bloom

```bash
sudo ./bloom cli bloom.yaml
```

**Repeat this process for each additional control plane node until you have an odd total (minimum 3).**

---

## Step 3: Worker Nodes

After all control plane nodes are set up, add worker nodes for your workloads.

### Create `bloom.yaml` for Worker Nodes

Choose the command that matches the worker node you are adding:

**For GPU Worker Node:**

```bash
echo -e 'CLUSTER_SIZE: large\nGPU_NODE: true\nFIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml
```

**For CPU Worker Node:**

```bash
echo -e 'CLUSTER_SIZE: large\nGPU_NODE: false\nFIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml
```

- `GPU_NODE: true` — Enables GPU drivers and GPU-specific resources (only needed for GPU worker nodes)

### Storage Configuration

Add a storage parameter to `bloom.yaml` based on your disk situation. **One of `CLUSTER_PREMOUNTED_DISKS` or `CLUSTER_DISKS` is mandatory** for proper cluster storage:

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

### Run bloom

```bash
sudo ./bloom cli bloom.yaml
```

---

## Step 4: ClusterForge Configuration

After all nodes (control plane and worker nodes) have been added to your cluster, return to your **first control plane node** to complete the cluster setup.

### Manual ClusterForge Setup

Create the clusterforge directory and download the ClusterForge Enterprise AI package:

```bash
mkdir clusterforge
chmod 755 clusterforge
wget -O "./clusterforge/clusterforge.tar.gz" https://github.com/silogen/cluster-forge/releases/download/v2.0.2/release-enterprise-ai-v2.0.2.tar.gz
tar -xzf "./clusterforge/clusterforge.tar.gz" -C ./clusterforge --no-same-owner
cd clusterforge/cluster-forge
```

### Run Bootstrap Script

Execute the bootstrap script to complete the ClusterForge installation:

```bash
./scripts/bootstrap.sh <your-domain> --cluster-size=large
```

This step configures cluster-wide services, networking, and other essential components that require all nodes to be present.

---

## Quick Reference

### Commands Summary

| Node Type | Commands |
|-----------|----------|
| **Additional Control Plane** | `echo -e 'CLUSTER_SIZE: large\nCONTROL_PLANE: true\nFIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml` |
| **GPU Worker Node** | `echo -e 'CLUSTER_SIZE: large\nGPU_NODE: true\nFIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml` |
| **CPU Worker Node** | `echo -e 'CLUSTER_SIZE: large\nGPU_NODE: false\nFIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml` |

### Storage Options

| Storage Type | Command |
|-------------|---------|
| Pre-mounted disk | `echo 'CLUSTER_PREMOUNTED_DISKS: /mnt/disk0' >> bloom.yaml` |
| Single raw disk | `echo 'CLUSTER_DISKS: /dev/nvme0n1' >> bloom.yaml` |
| Multiple raw disks | `echo 'CLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1' >> bloom.yaml` |

### Setup Order

1. First control plane node (initial setup)
2. Additional control plane nodes (minimum 3 total, odd numbers)
3. Worker nodes (GPU and CPU)
4. ClusterForge (run on first control plane node)

---

## Troubleshooting

**Control plane node fails to join**
- Verify the join token and server IP from `additional_node_command.txt`
- Ensure `CONTROL_PLANE: true` is set in bloom.yaml
- Check network connectivity between nodes

**Node joins but no storage is recognized**
- Verify either `CLUSTER_PREMOUNTED_DISKS` or `CLUSTER_DISKS` is set correctly
- For pre-mounted: confirm the path exists with `df -h`
- For raw disks: confirm the device exists with `lsblk`

**GPU driver installation fails on CPU node**
- Remove `GPU_NODE: true` from bloom.yaml (only needed for GPU worker nodes)
- CPU worker nodes and control plane nodes don't need GPU_NODE parameter

**ClusterForge fails to run**
- Ensure all intended nodes have successfully joined the cluster first
- Verify you're running clusterforge on the first control plane node
- Check cluster status with `kubectl get nodes`

**Cluster not highly available**
- Ensure you have an odd number of control plane nodes (3, 5, 7, etc.)
- Add control plane nodes before worker nodes for proper HA setup

---

## Related Documentation

- [Storage Management](./storage-management.md)
- [Configuration Reference](./configuration-reference.md)
- [Installation Guide](./installation-guide.md)
