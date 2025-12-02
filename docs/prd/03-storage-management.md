# Storage Management with Longhorn

## Overview

ClusterBloom automates the deployment and configuration of Longhorn distributed storage system, providing persistent block storage for Kubernetes workloads.

## Components

### Disk Detection
Automatically identifies and selects available NVMe drives:
- **Detection Method**: Scans for NVMe block devices via /sys/block/
- **Filtering**: Excludes boot disks, mounted partitions, and swap devices
- **Virtual Disk Filtering**: Excludes QEMU, VMware virtual disks
- **Size Validation**: Ensures sufficient disk space for storage workloads

**Detection Process**:
```bash
# List NVMe devices
lsblk -d -o NAME,TYPE,SIZE | grep nvme

# Check mount status
mount | grep nvme

# Verify not swap
swapon --show
```

### Interactive Disk Selection
TUI interface for manual disk selection:
- **Visual Interface**: Terminal-based UI showing available disks
- **Disk Information**: Displays size, model, serial number
- **Multi-selection**: Select multiple disks for Longhorn storage pool
- **Confirmation**: Warns about data loss before formatting

**Selection Features**:
- Color-coded disk status (available, mounted, system)
- Keyboard navigation for disk selection
- Real-time disk information updates
- Safe abort option

### Automated Mounting
Formats and mounts selected drives with persistence:
- **Filesystem**: ext4 filesystem format
- **Mount Points**: `/mnt/disk0`, `/mnt/disk1`, etc.
- **fstab Entries**: UUID-based mounting for reliability
- **Mount Options**: `defaults,nofail` for robustness

**Mounting Process**:
1. Wipe existing filesystem signatures
2. Format disk with ext4
3. Get disk UUID
4. Create mount point directory
5. Add fstab entry with UUID
6. Mount disk

**fstab Entry Format**:
```
UUID=<disk-uuid> /mnt/disk0 ext4 defaults,nofail 0 2
```

### Longhorn Integration
Configures Longhorn distributed storage system:
- **Version**: v1.8.0
- **Storage Class**: `mlstorage` (default)
- **Replica Count**: 3 (configurable)
- **Data Locality**: Configurable (disabled, best-effort, strict)

**Longhorn Features**:
- **Distributed Storage**: Replicated block storage across nodes
- **Snapshots**: Volume snapshots for backup and recovery
- **Backups**: S3/NFS backup support
- **CSI Driver**: Container Storage Interface compliance
- **Volume Encryption**: Optional volume encryption
- **Volume Cloning**: Clone volumes for testing

**Longhorn Configuration**:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: longhorn-default-setting
  namespace: longhorn-system
data:
  create-default-disk-labeled-nodes: "true"
  default-data-path: "/mnt/disk0"
  replica-soft-anti-affinity: "true"
  disable-revision-counter: "true"
  priority-class: "longhorn-critical"
```

### Storage Class Configuration
Default storage class for PVC provisioning:
```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: mlstorage
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: driver.longhorn.io
allowVolumeExpansion: true
reclaimPolicy: Delete
volumeBindingMode: Immediate
parameters:
  numberOfReplicas: "3"
  staleReplicaTimeout: "2880"
  fromBackup: ""
  fsType: "ext4"
```

## Architecture

```mermaid
graph TB
    subgraph "Disk Discovery & Preparation"
        A[Scan NVMe Devices] --> B[Filter Virtual Disks]
        B --> C[Exclude Mounted/Swap]
        C --> D{Auto or Manual?}
        D -->|Auto| E[Select First Available]
        D -->|Manual| F[TUI Selection Interface]
        E --> G[Format ext4]
        F --> G
        G --> H[Create Mount Point]
        H --> I[Add fstab Entry]
        I --> J[Mount Disk]
    end
    
    subgraph "Longhorn Deployment"
        J --> K[Deploy Longhorn CRDs]
        K --> L[Deploy Longhorn Operators]
        L --> M[Deploy CSI Driver]
        M --> N[Configure Default Settings]
        N --> O[Create Storage Class]
    end
    
    subgraph "Storage Operations"
        O --> P[PVC Creation]
        P --> Q[Volume Provisioning]
        Q --> R[Replica Distribution]
        R --> S[Volume Attachment]
        S --> T[Pod Mount]
    end
    
    subgraph "Data Protection"
        Q --> U[Snapshot System]
        Q --> V[Backup System]
        U --> W[Snapshot Restore]
        V --> X[S3/NFS Backup Target]
    end
    
    style A fill:#4CAF50
    style G fill:#FF9800
    style K fill:#2196F3
    style Q fill:#9C27B0
```
