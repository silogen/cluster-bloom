# Longhorn Drive Setup and Recovery Documentation

This documentation provides comprehensive instructions forsetting up, recovering, and managing Longhorn drives on cluster-bloom nodes. It includes both manual step-by-step procedures and a sample (not officially supported) script which serves as an automation example.

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Disk Space Requirements](#disk-space-requirements)
4. [RAID Considerations](#raid-considerations)
5. [Reboot Checklist](#reboot-checklist)
6. [Manual Disk Setup Procedure](#manual-disk-setup-procedure)
7. [Automation Script](#automation-script)
8. [Longhorn UI Configuration](#longhorn-ui-configuration)
9. [Troubleshooting](#troubleshooting)
10. [Reference](#reference)

## Overview

Longhorn is a distributed block storage system for Kubernetes that requires proper disk configuration to ensure data persistence across node reboots. This documentation covers:

- **Drive Priority**: NVMe drives (preferred) → SSD drives → HDD drives
- **RAID Restriction**: Longhorn explicitly does NOT support RAID configurations
- **Special Requirements**: `/var/lib/rancher` needs dedicated mountpoint only if root partition is space-constrained
- **Mount Pattern**: Disks mounted at `/mnt/diskX` where X starts from 0 and increments by one for each additional disk
- **Filesystem**: ext4 with UUID-based mounting for reliability

## Prerequisites

- Root access to the cluster node
- Understanding of Linux disk management
- Backup of important data (formatting operations are destructive)
- Basic knowledge of Longhorn concepts

### Required Packages

Ensure these utilities are available:
```bash
sudo apt update
sudo apt install -y util-linux e2fsprogs mdadm
```

## Disk Space Requirements

Based on cluster-bloom requirements, ensure adequate disk space:

#### Disk Space Requirements
- **Root partition**: Minimum 10GB required, 20GB recommended
- **Available space**: Minimum 10GB required  
- **/var partition**: 5GB recommended for container images
- **/var/lib/rancher**: dedicated partition in the case that the root partition is constrained (and no separate /var or /var/lib mounts exist)

#### Space Validation
```bash
# Check current disk usage
df -h /
df -h /var
df -h /var/lib/rancher 2>/dev/null || echo "/var/lib/rancher not separately mounted"

# Check available space
df -h | awk '$6=="/" {print "Root partition: " $4 " available"}'
```

## RAID Considerations

**⚠️ CRITICAL**: Longhorn documentation explicitly states that **RAID configurations are NOT supported**. Longhorn provides its own replication and high availability mechanisms.

### Detecting RAID Configuration

Check if your system has software RAID that needs to be removed:

```bash
# Check for software RAID arrays
cat /proc/mdstat

# List RAID arrays
sudo mdadm --detail --scan

# Example of RAID configuration that must be removed:
```

**Example `lsblk` output showing problematic RAID setup:**
```
NAME        MAJ:MIN RM   SIZE RO TYPE  MOUNTPOINTS                UUID
nvme0n1     259:0    0   3.5T  0 disk                             
└─nvme0n1p1 259:1    0   3.5T  0 part                             
  └─md0       9:0    0    14T  0 raid0 /                         
nvme1n1     259:2    0   894G  0 disk                             
├─nvme1n1p1 259:3    0   512M  0 part  /boot/efi                 
└─nvme1n1p2 259:4    0   893G  0 part  [SWAP]                    
nvme2n1     259:5    0   3.5T  0 disk                             
└─nvme2n1p1 259:6    0   3.5T  0 part                             
  └─md0       9:0    0    14T  0 raid0 /                         
nvme3n1     259:7    0   3.5T  0 disk                             
└─nvme3n1p1 259:8    0   3.5T  0 part                             
  └─md0       9:0    0    14T  0 raid0 /                         
nvme4n1     259:9    0   3.5T  0 disk                             
└─nvme4n1p1 259:10   0   3.5T  0 part                             
  └─md0       9:0    0    14T  0 raid0 /                         
nvme5n1     259:11   0   894G  0 disk                             
```

In the above example, **md0** shows a RAID0 array using multiple NVMe drives - this must be broken apart for Longhorn use.

### RAID Removal Process

**⚠️ WARNING ** The automation [script](../experimental/longhorn-disk-setup.sh) is not robustly tested, but rather serves as a starting point for your particular use case.

The automation script (`/cluster-bloom/experimental/longhorn-disk-setup.sh`) can backup, remove, and optionally restore RAID configurations:

```bash
# Check if RAID is present
cat /proc/mdstat

# Backup and remove RAID (interactive)
sudo bash experimental/longhorn-disk-setup.sh --remove-raid

# Force RAID removal without confirmation
sudo bash experimental/longhorn-disk-setup.sh --force-raid-removal
```

#### RAID Backup and Restore

The script automatically backs up RAID configurations before removal:

**Backup Location**: `/root/longhorn-raid-backup/`
**Backup Contents**:
- `mdadm.conf.backup` - RAID configuration
- `mdstat.backup` - RAID status at backup time
- `md*_detail.backup` - Individual array details

**Manual RAID Restoration** (if needed):
```bash
# List backups
ls -la /root/longhorn-raid-backup/

# View original configuration
cat /root/longhorn-raid-backup/mdadm.conf.backup

# Restore RAID (DESTRUCTIVE - will recreate arrays)
sudo mdadm --assemble --scan --config=/root/longhorn-raid-backup/mdadm.conf.backup
```

**⚠️ Important**: RAID restoration will destroy any data written to individual disks after RAID removal.

## Reboot Checklist

After any node reboot, verify that all Longhorn storage disks are properly mounted:

### Quick Validation Commands

```bash
# 1. Check current fstab entries for Longhorn disks
sudo cat /etc/fstab | grep -E "/mnt/disk[0-9]+"

# 2. Check currently mounted disks
df -h | grep -E "/mnt/disk[0-9]+"

# 3. List all disks with UUIDs
lsblk -o +UUID

# 4. Verify all fstab entries mount correctly
sudo mount -a && echo "All mounts successful" || echo "Mount errors detected"
```

### Expected fstab Format

Your `/etc/fstab` should contain entries like:

```bash
UUID=f9134cf2-0205-4012-8e8b-ac44757a0d15 /mnt/disk0 ext4 defaults,nofail 0 2
UUID=9111f9b3-e4e5-4a50-a9cc-3258d40786f3 /mnt/disk1 ext4 defaults,nofail 0 2
UUID=e27fc7cd-356a-40de-89ae-ea1f0af59d24 /mnt/disk2 ext4 defaults,nofail 0 2
UUID=489f3576-cf3b-4319-ba9d-a07427225f81 /mnt/disk3 ext4 defaults,nofail 0 2
UUID=3206db8b-109e-4b9f-8320-7db4cca5210d /mnt/disk4 ext4 defaults,nofail 0 2
```

**Note**: The `nofail` option ensures the system boots even if a disk is unavailable.

## Manual Disk Setup Procedure

### Step 1: Identify Candidate Disks

First, examine your system's storage layout:

```bash
# List all block devices with UUIDs
lsblk -o +UUID

# Example output with 5 NVMe drives:
NAME        MAJ:MIN RM   SIZE RO TYPE  MOUNTPOINTS                UUID
nvme0n1     259:0    0   3.5T  0 disk                             
├─nvme0n1p1 259:1    0   3.5T  0 part  /                         a1b2c3d4-e5f6-7890-abcd-ef1234567890
nvme1n1     259:2    0   894G  0 disk                             
├─nvme1n1p1 259:3    0   512M  0 part  /boot/efi                 1234-5678
└─nvme1n1p2 259:4    0   893G  0 part  [SWAP]                    
nvme2n1     259:5    0   3.5T  0 disk                             
nvme3n1     259:6    0   3.5T  0 disk                             
nvme4n1     259:7    0   3.5T  0 disk                             
nvme5n1     259:8    0   894G  0 disk                             
sdb         8:16     0   256G  0 disk                             
```

**Disk Priority for Longhorn Storage**:
1. **NVMe drives** (nvme0n1, nvme2n1, nvme3n1, nvme4n1, nvme5n1) - Highest priority
2. **SSD drives** (typically sdb, sdc, etc.) - Medium priority  
3. **HDD drives** (sda usually excluded as boot drive) - Lowest priority

### Step 2: Check Current Mount Status

```bash
# Check what's currently mounted
mount | grep -E "/mnt/disk|/var/lib/rancher"

# Compare with fstab entries
sudo cat /etc/fstab
```

### Step 3: Identify Unmounted Candidate Disks

Look for disks that:
- Are not currently mounted
- Don't have a UUID (indicating they need formatting)
- Are suitable for Longhorn storage

Example identification process:

```bash
# Check if disk has UUID (formatted)
sudo blkid /dev/nvme2n1
# If no output, disk needs formatting

# Check if disk is mounted
mount | grep /dev/nvme2n1
# If no output, disk is not mounted
```

### Step 4: Format Unmounted Disks

**⚠️ WARNING**: This will destroy all data on the disk!

For each unformatted disk:

```bash
# Format with ext4 filesystem
sudo mkfs.ext4 /dev/nvme2n1

# Verify UUID was assigned
sudo blkid /dev/nvme2n1
# Output: /dev/nvme2n1: UUID="e27fc7cd-356a-40de-89ae-ea1f0af59d24" TYPE="ext4"
```

### Step 5: Create Mount Points

```bash
# Create mount directories
sudo mkdir -p /mnt/disk0
sudo mkdir -p /mnt/disk1
sudo mkdir -p /mnt/disk2
sudo mkdir -p /mnt/disk3
sudo mkdir -p /mnt/disk4
# Continue for additional disks
```

### Step 6: Add Disks to fstab

For each formatted disk, add an entry to `/etc/fstab`:

```bash
# Get the UUID for the disk
UUID=$(sudo blkid -s UUID -o value /dev/nvme2n1)

# Add entry to fstab (replace with actual UUID)
echo "UUID=$UUID /mnt/disk2 ext4 defaults,nofail 0 2" | sudo tee -a /etc/fstab
```

### Step 7: Mount All Disks

```bash
# Mount all entries in fstab
sudo mount -a

# Verify successful mounting
df -h | grep "/mnt/disk"
```

Expected output:
```
/dev/nvme2n1    3.4T   89M  3.2T   1% /mnt/disk0
/dev/nvme3n1    3.4T   89M  3.2T   1% /mnt/disk1
/dev/nvme4n1    3.4T   89M  3.2T   1% /mnt/disk2
/dev/nvme5n1    894G   77M  848G   1% /mnt/disk3
/dev/sdb        251G   65M  238G   1% /mnt/disk4
```

## Automation Script

The automation [script](../experimental/longhorn-disk-setup.sh) at `cluster-forge/experimental/longhorn-disk-setup.sh` provides comprehensive disk management capabilities, including RAID handling, disk discovery, formatting, mounting, and fstab configuration.

### Script Usage

```bash
# The automation script is available in the experimental folder
# Script location: experimental/longhorn-disk-setup.sh

# Dry run to see recommendations without making changes
sudo bash experimental/longhorn-disk-setup.sh --dry-run

# Full interactive setup (with RAID handling if needed)  
sudo bash experimental/longhorn-disk-setup.sh

# Force RAID backup and removal (if detected)
sudo bash experimental/longhorn-disk-setup.sh --remove-raid
```

### Script Capabilities

The script will:
1. **Check disk space requirements** and recommend `/var/lib/rancher` setup if needed
2. **Detect and handle software RAID** configurations safely
3. **Discover candidate disks** (prioritized by type: NVMe → SSD → HDD)
4. **Identify unformatted disks** and prompt for formatting
5. **Create mount points** with proper permissions
6. **Add fstab entries** with UUID-based mounting
7. **Validate mounts** and test reboot safety
8. **Provide summary** and next steps

### RAID Handling Features

- **RAID Detection**: Automatically detects software RAID arrays
- **Configuration Backup**: Saves RAID configuration for potential restoration
- **Safe Removal**: Properly stops and removes RAID arrays
- **Restoration Capability**: Can restore original RAID if needed

## Longhorn UI Configuration

After disks are mounted and persistent, configure them in Longhorn:

### Step 1: Access Longhorn UI

```bash
# Access the Longhorn dashboard
https://longhorn.cluster-name
```

### Step 2: Add Disks to Nodes

1. **Navigate to Nodes**: Click on the "Node" tab in the Longhorn UI
2. **Select Node**: Choose the node you want to configure
3. **Edit Disks**: In the "Operations" column (far right), click the dropdown menu and select "Edit node and disks"
4. **Add Disk**: Scroll to the bottom of the form and click "Add disk"
5. **Configure Disk**:
   - **Name**: Descriptive name (e.g., "nvme-disk-0")
   - **Disk Type**: "filesystem"
   - **Path**: Mount path (e.g., "/mnt/disk0")
   - **Storage Reserved**: Amount to reserve (bytes) - optional
6. **Enable Scheduling**: Click the "Enable" button under "Scheduling"
7. **Save**: Click "Save" to apply changes

### Step 3: Verify Disk Addition

- Check that the disk appears in the node's disk list
- Verify "Schedulable" status is "True"
- Monitor disk space and usage

## Special Requirements

### /var/lib/rancher Partition

Based on cluster-bloom requirements and available space:

- **Conditional Requirement**: `/var/lib/rancher` should have its own dedicated mount point **only if** root partition is space-constrained
- **Size Guidelines**: Refer to disk space requirements above
- **Configuration**: Can be specified via `CLUSTER_DISKS` or `CLUSTER_PREMOUNTED_DISKS` in bloom.yaml

#### When to Create Separate /var/lib/rancher:
```bash
# Check if root partition needs dedicated /var/lib/rancher
ROOT_AVAILABLE=$(df --output=avail / | tail -1)
if [ "$ROOT_AVAILABLE" -lt 20971520 ]; then  # Less than 20GB in KB
    echo "Root partition space-constrained, recommend separate /var/lib/rancher"
else
    echo "Root partition has sufficient space"
fi
```

Example setup if needed:
```bash
# If using a dedicated disk for /var/lib/rancher
UUID=12345678-90ab-cdef-1234-567890abcdef /var/lib/rancher ext4 defaults,nofail 0 2
```

## Troubleshooting

### Common Issues

**1. Disk not mounting after reboot**
```bash
# Check fstab entry syntax
sudo cat /etc/fstab | grep UUID

# Test mount manually
sudo mount UUID=your-uuid-here /mnt/disk0

# Check filesystem health
sudo fsck -f /dev/nvme2n1
```

**2. UUID not found**
```bash
# Regenerate UUID if filesystem is corrupted
sudo tune2fs -U random /dev/nvme2n1

# Update fstab with new UUID
sudo blkid /dev/nvme2n1
```

**3. Mount point permission issues**
```bash
# Fix ownership and permissions
sudo chown root:root /mnt/disk0
sudo chmod 755 /mnt/disk0
```

**4. Longhorn not detecting disks**
- Ensure disk path matches exactly in Longhorn UI
- Verify disk has sufficient space
- Check Longhorn logs for errors
```bash
kubectl logs -n longhorn-system deployment/longhorn-manager
```

### Validation Commands

```bash
# Comprehensive disk check
echo "=== Disk Status ==="
lsblk -o +UUID

echo -e "\n=== RAID Check ==="
if [[ -f /proc/mdstat ]]; then
    cat /proc/mdstat
    if grep -q "^md" /proc/mdstat; then
        echo "⚠️  RAID arrays detected - Longhorn does not support RAID!"
    else
        echo "✓ No RAID arrays found"
    fi
else
    echo "✓ No RAID support"
fi

echo -e "\n=== Mount Status ==="
df -h | grep "/mnt/disk"

echo -e "\n=== fstab Entries ==="
grep "/mnt/disk" /etc/fstab

echo -e "\n=== Mount Test ==="
sudo mount -a && echo "✓ All mounts successful" || echo "✗ Mount errors"

echo -e "\n=== Disk Space Check ==="
df -h / | tail -1 | awk '{print "Root: " $4 " available (" $5 " used)"}'
```

## Reference

### Longhorn Documentation
- [Multiple Disk Support](https://longhorn.io/docs/1.8.0/nodes-and-volumes/nodes/multidisk/)
- [Node Space Usage](https://longhorn.io/docs/1.8.0/nodes-and-volumes/nodes/node-space-usage/)

### Cluster-Bloom Configuration
- Storage options: `NO_DISKS_FOR_CLUSTER`, `CLUSTER_DISKS`, `CLUSTER_PREMOUNTED_DISKS`
- Device path format: `/dev/nvme0n1,/dev/nvme1n1` (comma-separated)
- Premounted disk format: `/mnt/disk1,/mnt/disk2` (comma-separated)

### Setup Script
  - cluster-bloom/experimental/longhorn-disk-setup.sh

### Best Practices
1. **Always backup data** before disk operations
2. **Use UUID-based mounting** for reliability
3. **Test mount operations** before rebooting
4. **Monitor disk space** regularly
5. **Keep fstab entries simple** and well-documented
6. **Use `nofail` option** to prevent boot issues
