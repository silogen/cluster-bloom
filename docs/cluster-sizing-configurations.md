# Cluster Size Profiles

## Summary Table

| Tier   | Target Use Case                              | Nodes (typ.)        | vCPU / node | RAM / node   | GPUs (total)                  | Storage                                        | Control Plane                       |
|--------|----------------------------------------------|---------------------|-------------|--------------|-------------------------------|------------------------------------------------|-------------------------------------|
| Small  | Workstation, gaming PC, single developer     | 1 node              | 16–32 vCPU  | 64–128 GB    | 1–2 GPUs (full)               | 1–4 TB NVMe                                    | Single-node (server + etcd + worker)|
| Medium | Small team, shared environment               | 1–3 nodes           | 32–64 vCPU  | 128–256 GB   | Up to 8 GPUs (partitionable)  | 4–16 TB NVMe + 2–10 TB internal S3             | 1–3 CP nodes (optional light HA)    |
| Large  | Production path, HA, scalable to 100s nodes  | 6+ nodes (scalable) | 32–96 vCPU  | 256–1024 GB  | 8–N GPUs, mixed/partitioned   | 10–100 TB NVMe + external HA object storage    | 3–5 dedicated CP nodes (full HA)    |

## Tier Details (RKE2-specific)

### 1. Small — Workstation / Gaming PC / Single Developer

#### Intent

Local "full stack in a box"

Perfect for PoC, experimentation, reproducible bug repros

#### Specs

Nodes: 1 physical node

Runs control-plane + etcd + worker

CPU: 16–32 logical cores

Memory: 64–128 GB

GPU: 1–2 GPUs, no partitioning needed

Disk: 1–4 TB NVMe (single or mirrored)

Storage: Optional small MinIO; otherwise skip internal S3

#### RKE2 Notes:

Single rke2-server

No HA (expected)

Great for local dev/solo work

### 2. Medium — Team Cluster

#### Intent

Shared environment for a small team (5–20 users)

Supports concurrent jobs, serving workloads, larger datasets

#### Specs

Nodes: 1–3 nodes

Option A: 1×control plane + 1–2 GPU workers

Option B: 3×control-plane (HA) + optional GPU workers

CPU: 32–64 vCPU per GPU node

Memory: 128–256 GB RAM per GPU node

GPU: Up to 8 GPUs total, partitioning optional

Storage:

4–16 TB total NVMe

Internal S3: 2–10 TB via MinIO or similar

#### RKE2 Notes:

Path to HA (3 cp nodes) is documented

Use taints/labels for GPU worker separation

Networking: 10 GbE recommended

### 3. Large — Production-Path / Scale-Out

#### Intent

Deployment that can transition into production

Supports 10s–100s of nodes, mixed workloads, HA everywhere

#### Specs

Nodes:

Control Plane: 3–5 dedicated servers

Workers: 3–6 GPU nodes to start, scale to 100s

CPU:

Workers: 32–96 vCPU

CP nodes: 8–16 vCPU

Memory:

Workers: 256–1024 GB

CP nodes: 32–64 GB

GPU:

8+ GPUs baseline

Mixed families, heterogenous, partitionable

Storage:

10–100+ TB NVMe

External HA S3 object storage (recommended)

#### RKE2 Notes:

Dedicated, tainted CP nodes

Etcd snapshots + full backup/restore plan

Ingress via MetalLB or cloud LB

Networking:

25 GbE or more

Optional separate storage network

## Sizing your cluster

### Small (Workstation)

A single-node rke2 cluster for individual developers.
1–2 GPUs, 64–128 GB RAM, 1–4 TB SSD.
Best for demos, experimentation, and local development.

### Medium (Team)

A 1–3 node cluster for a small team.
Up to 8 GPUs, 128–256 GB RAM per GPU node, internal S3 up to 10 TB.
Optional light HA of the control plane.

### Large (Production Path)

A scalable, HA rke2 cluster with dedicated control plane nodes.
8+ GPUs, large NVMe pools, external HA object storage.
Designed for growth to 100+ nodes.