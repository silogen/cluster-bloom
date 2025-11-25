# PrepareLonghornDisksStep Integration Test Scenarios

This document outlines test scenarios for the PrepareLonghornDisksStep, which is responsible for mounting and configuring disks for Longhorn storage.

## Overview

PrepareLonghornDisksStep performs the following operations:
1. Lists existing mount points
2. Checks filesystem types on drives
3. Checks partition types
4. Wipes existing partitions if needed
5. Formats drives with ext4
6. Gets UUIDs for drives
7. Mounts drives to `/mnt/diskN` directories
8. Persists mounts to `/etc/fstab`
9. Generates node labels for RKE2 configuration

## Test Categories

### 1. Skip Mode Tests

#### 1.1 NO_DISKS_FOR_CLUSTER Mode
**Scenario**: Skip disk mounting when NO_DISKS_FOR_CLUSTER is set

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: true
CLUSTER_DISKS: /dev/sda
```

**Expected Behavior**:
- Step skips all disk operations
- No mocks are called
- No mount points created
- Log shows: "Skipping drive mounting as NO_DISKS_FOR_CLUSTER is set."
- Step succeeds with no errors

#### 1.2 CLUSTER_PREMOUNTED_DISKS Mode
**Scenario**: Skip disk mounting when CLUSTER_PREMOUNTED_DISKS is set

**Configuration**:
```yaml
CLUSTER_PREMOUNTED_DISKS: "/mnt/disk0,/mnt/disk1"
CLUSTER_DISKS: /dev/sda,/dev/sdb
```

**Expected Behavior**:
- Step skips MountDrives operation
- Step skips PersistMountedDisks operation
- No formatting or mounting occurs
- Log shows: "Skipping drive mounting as CLUSTER_PREMOUNTED_DISKS is set."
- Step succeeds with no errors

### 2. Single Disk Tests

#### 2.1 Fresh Disk - No Filesystem
**Scenario**: Mount a fresh disk with no filesystem

**Configuration**:
```yaml
CLUSTER_DISKS: /dev/sda
NO_DISKS_FOR_CLUSTER: false
```

**Required Mocks**:
- PrepareLonghornDisksStep.ListMounts
- PrepareLonghornDisksStep.CheckFilesystem./dev/sda
- PrepareLonghornDisksStep.CheckPartitionType./dev/sda
- PrepareLonghornDisksStep.FormatDisk./dev/sda
- PrepareLonghornDisksStep.GetUUID./dev/sda
- PrepareLonghornDisksStep.MountDrive./dev/sda
- PersistMountedDisks.BackupFstab
- PersistMountedDisks.GetUUID./dev/sda-{uuid}
- PersistMountedDisks.RemountAll

**Expected Behavior**:
- Disk is formatted with ext4
- Disk is mounted at /mnt/disk0
- Entry added to /etc/fstab
- Node labels generated
- Logs show successful format and mount

#### 2.2 Disk Already Formatted with ext4
**Scenario**: Mount a disk that already has ext4 filesystem

**Configuration**:
```yaml
CLUSTER_DISKS: /dev/sdb
```

**Expected Behavior**:
- Format step is skipped
- Log shows: "Disk /dev/sdb is already formatted as ext4. Skipping format."
- Disk is mounted at /mnt/disk0
- Entry added to /etc/fstab

#### 2.3 Disk with Existing Partitions
**Scenario**: Disk has existing partitions that need to be wiped

**Configuration**:
```yaml
CLUSTER_DISKS: /dev/sdc
```

**Required Additional Mocks**:
- PrepareLonghornDisksStep.WipePartitions./dev/sdc

**Expected Behavior**:
- Partitions are wiped
- Disk is formatted with ext4
- Disk is mounted
- Logs show partition removal

### 3. Multiple Disk Tests

#### 3.1 Two Fresh Disks
**Scenario**: Mount two fresh disks sequentially

**Configuration**:
```yaml
CLUSTER_DISKS: /dev/sda,/dev/sdb
```

**Expected Behavior**:
- /dev/sda mounted at /mnt/disk0
- /dev/sdb mounted at /mnt/disk1
- Both entries in /etc/fstab
- Node labels include both disks

#### 3.2 Mixed Disk States
**Scenario**: One disk formatted, one fresh

**Configuration**:
```yaml
CLUSTER_DISKS: /dev/sda,/dev/sdb
```

**Expected Behavior**:
- /dev/sda skips format (already ext4)
- /dev/sdb gets formatted
- Both mounted successfully

### 4. Error Handling Tests

#### 4.1 Format Failure
**Scenario**: mkfs.ext4 command fails

**Mock Error**:
```yaml
PrepareLonghornDisksStep.FormatDisk./dev/sda:
  error: "device is write-protected"
```

**Expected Behavior**:
- Step fails with error
- Error message includes: "failed to format /dev/sda"
- No mount operations performed

#### 4.2 Mount Failure
**Scenario**: mount command fails

**Mock Error**:
```yaml
PrepareLonghornDisksStep.MountDrive./dev/sda:
  error: "mount point does not exist"
```

**Expected Behavior**:
- Step fails with error
- Error message includes: "failed to mount /dev/sda at /mnt/disk0"
- No fstab operations performed

#### 4.3 UUID Retrieval Failure (Non-critical)
**Scenario**: blkid fails to get UUID in PersistMountedDisks

**Mock Error**:
```yaml
PersistMountedDisks.GetUUID./dev/sda-{uuid}:
  error: "unable to read device"
```

**Expected Behavior**:
- Log message: "Could not retrieve UUID for /dev/sda-{uuid}. Skipping..."
- Step continues (does not fail)
- No fstab entry added for this disk

## Mock ID Reference

### MountDrives Mock IDs:
- `PrepareLonghornDisksStep.ListMounts` - List existing mount points
- `PrepareLonghornDisksStep.CheckFilesystem.{drive}` - Check if drive has filesystem
- `PrepareLonghornDisksStep.CheckPartitionType.{drive}` - Check partition type
- `PrepareLonghornDisksStep.WipePartitions.{drive}` - Wipe existing partitions
- `PrepareLonghornDisksStep.FormatDisk.{drive}` - Format drive with ext4
- `PrepareLonghornDisksStep.GetUUID.{drive}` - Get UUID after format
- `PrepareLonghornDisksStep.MountDrive.{drive}` - Mount drive to mount point

### PersistMountedDisks Mock IDs:
- `PersistMountedDisks.BackupFstab` - Backup /etc/fstab
- `PersistMountedDisks.GetUUID.{device}` - Get UUID for fstab entry
- `PersistMountedDisks.RemountAll` - Remount all filesystems

Note: `{drive}` = drive path (e.g., `/dev/sda`)
Note: `{device}` = device identifier from mountedMap (e.g., `/dev/sda-abc-123`)
