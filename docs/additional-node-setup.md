# Cluster Setup and Node Management

This guide covers the complete workflow for setting up a highly available cluster with bloom, from the first control plane node to the final cluster configuration.

> This guide covers setting up a **large** size cluster (`CLUSTER_SIZE: large`). Large clusters support both highly available (HA) configurations and non-HA multi-node setups, such as a single control plane with worker nodes or multiple GPU control nodes.

## Setup Workflow Overview

Follow this sequence for proper cluster setup:

1. **First Control Plane Node** - Initialize your cluster
2. **Additional Control Plane Nodes** - Add remaining control plane nodes for HA (minimum 3 total, always odd number)
3. **Worker Nodes** - Add GPU and CPU worker nodes as needed
4. **ClusterForge** - Return to first control plane node and run clusterforge to complete setup

---

## Recommended Large Cluster Storage Architecture

For optimal large cluster performance with GPU workloads, use dedicated storage separation:

### Example GPU-Heavy Cluster Setup

```yaml
# First Control Plane Node
FIRST_NODE: true
CLUSTER_DISKS: /dev/nvme0n1          # Application storage (Longhorn)
# RANCHER_DISK optional for control plane

# GPU Worker Nodes (High-Performance) - repeat for each worker
FIRST_NODE: false
CONTROL_PLANE: false
GPU_NODE: true
CLUSTER_DISKS: /dev/nvme0n1          # Application storage
RANCHER_DISK: /dev/nvme1n1           # Dedicated /var/lib/rancher for GPU workloads
JOIN_TOKEN: <token>
SERVER_IP: <first-node-ip>
```

**Benefits:**
- **GPU worker nodes** get dedicated fast storage for intensive workloads
- Separates heavy kubelet/container data from application storage
- Better performance for nodes with large container images and logs
- Control plane nodes can optionally use RANCHER_DISK if desired

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
| **GPU Worker Node** | Runs GPU workloads | Yes |
| **CPU Worker Node** | Runs workloads without GPU | No |
| **GPU Control Node** | Runs control plane workloads with GPU | Yes |
| **CPU Control Node** | Runs control plane workloads without GPU | No |

---

## Step 2: Additional Control Plane Nodes (HA Setup)

For high availability, add additional control plane nodes **before** adding worker nodes. The total number of control plane nodes must be odd (3, 5, 7, etc.) with a minimum of 3 nodes.

### Create `bloom.yaml` for Additional Control Plane Nodes

Copy the join token and server IP from `additional_node_command.txt` generated on your first control plane node to each additional node, then create the configuration on the additional node:

**For CPU Control Plane Node:**

```bash
echo -e 'CLUSTER_SIZE: large\nCONTROL_PLANE: true\nGPU_NODE: false\nFIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml
```

**For GPU Control Plane Node:**

```bash
echo -e 'CLUSTER_SIZE: large\nCONTROL_PLANE: true\nGPU_NODE: true\nFIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml
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
echo -e 'CLUSTER_SIZE: large\nCONTROL_PLANE: false\nGPU_NODE: true\nFIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml
```

**For CPU Worker Node:**

```bash
echo -e 'CLUSTER_SIZE: large\nCONTROL_PLANE: false\nGPU_NODE: false\nFIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml
```

- `CONTROL_PLANE: true` — Designates this node as part of the control plane
- `CONTROL_PLANE: false` — Designates this node as a worker node
- `GPU_NODE: true` — Enables GPU drivers and GPU-specific resources (for GPU nodes)
- `GPU_NODE: false` — Disables GPU drivers and resources (for CPU-only nodes)

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

**Option C — Dedicated /var/lib/rancher Storage** (`RANCHER_DISK`):

- **Primary use**: GPU worker nodes with intensive container workloads
- Provides dedicated fast storage for kubelet and container runtime data
- Improves performance for nodes with heavy GPU workloads and large logs
- Can also be used on control plane nodes if dedicated RKE2 storage is desired
- Can be combined with CLUSTER_DISKS/CLUSTER_PREMOUNTED_DISKS
- Value format: single device path (e.g., `/dev/nvme2n1`)

```bash
echo 'RANCHER_DISK: /dev/nvme2n1' >> bloom.yaml
```
Use to dedicate a separate disk for `/var/lib/rancher` directory.
**Highly recommended for GPU worker nodes** with heavy workloads.

### Run bloom

```bash
sudo ./bloom cli bloom.yaml
```

---

## Step 4: ClusterForge Bootstrap

After all nodes (control plane and worker nodes) have been added to your cluster, return to your **first control plane node** to run the ClusterForge bootstrap.

This is designed as a **two-part deployment**:

- **Part 1** — the initial `bloom cli` run deploys the cluster infrastructure. To defer ClusterForge until all nodes are ready, set `CLUSTERFORGE_RELEASE: none` in your `bloom.yaml` before running it.
- **Part 2** — once all nodes have joined, run the ClusterForge bootstrap on its own using the `deploy_clusterforge` tag:

```bash
sudo ./bloom cli bloom.yaml --tags deploy_clusterforge
```

This step configures cluster-wide services, networking, and other essential components that require all nodes to be present. The same `bloom.yaml` used for the initial deployment is reused — `CLUSTERFORGE_RELEASE` must be set to the desired release (not `none`) when running this step.

---

## Quick Reference

### Commands Summary

| Node Type | Commands |
|-----------|----------|
| **CPU Control Plane** | `echo -e 'CLUSTER_SIZE: large\nCONTROL_PLANE: true\nGPU_NODE: false\nFIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml` |
| **GPU Control Plane** | `echo -e 'CLUSTER_SIZE: large\nCONTROL_PLANE: true\nGPU_NODE: true\nFIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml` |
| **GPU Worker Node** | `echo -e 'CLUSTER_SIZE: large\nCONTROL_PLANE: false\nGPU_NODE: true\nFIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml` |

| **CPU Worker Node** | `echo -e 'CLUSTER_SIZE: large\nCONTROL_PLANE: false\nGPU_NODE: false\nFIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml` |

### Storage Options

| Storage Type | Command |
|-------------|---------|
| Pre-mounted disk | `echo 'CLUSTER_PREMOUNTED_DISKS: /mnt/disk0' >> bloom.yaml` |
| Single raw disk | `echo 'CLUSTER_DISKS: /dev/nvme0n1' >> bloom.yaml` |
| Multiple raw disks | `echo 'CLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1' >> bloom.yaml` |
| No rancher disk for gpu worker | `echo 'CLUSTER_DISKS: /dev/nvme0n1' >> bloom.yaml` |
| With rancher disk for gpu worker | `echo -e 'CLUSTER_DISKS: /dev/nvme0n1\nRANCHER_DISK: /dev/nvme2n1' >> bloom.yaml` |

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

**ClusterForge bootstrap fails**
- Ensure all intended nodes have successfully joined the cluster first
- Verify you are running on the first control plane node
- Confirm `CLUSTERFORGE_RELEASE` in `bloom.yaml` is set to a valid release (not `none`)
- Re-run with: `sudo ./bloom cli bloom.yaml --tags deploy_clusterforge`
- Check cluster status with `kubectl get nodes`

**Cluster not highly available**
- Ensure you have an odd number of control plane nodes (3, 5, 7, etc.)
- Add control plane nodes before worker nodes for proper HA setup

---

## Related Documentation

- [Storage Management](./storage-management.md)
- [Configuration Reference](./configuration-reference.md)
- [Installation Guide](./installation-guide.md)
