# PrepareLonghornDisksStep Test Scenarios

## Overview
The PrepareLonghornDisksStep handles disk mounting for Longhorn storage. It supports three main modes and includes backup functionality for existing Longhorn configurations.

## Configuration Modes

### Mode 1: NO_DISKS_FOR_CLUSTER (Skip Mode)
Step is completely skipped when `NO_DISKS_FOR_CLUSTER: true`

### Mode 2: CLUSTER_PREMOUNTED_DISKS (Pre-mounted Mode)
Uses existing mount points without mounting new disks

### Mode 3: CLUSTER_DISKS (New Disk Mounting Mode)
Mounts and formats new disks for Longhorn

---

## Test Scenarios

### 1. Skip Mode - NO_DISKS_FOR_CLUSTER
**Purpose**: Verify step is skipped when no disks are configured for the cluster

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: true
FIRST_NODE: true
```

**Expected Behavior**:
- Step skipped entirely
- No disk operations performed
- Log message: "Skipping drive mounting as NO_DISKS_FOR_CLUSTER is set."

**Mock Requirements**: None (step is skipped)

---

### 2. Pre-mounted Disks - Single Disk
**Purpose**: Test using a single pre-mounted disk

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_PREMOUNTED_DISKS: "/mnt/storage1"
FIRST_NODE: true
```

**Expected Behavior**:
- Populate mountedDiskMap with: `{"/mnt/storage1": "0"}`
- No mounting operations performed
- Check for existing longhorn-disk.cfg and replicas directory
- Backup any existing Longhorn files

**Mock Requirements**:
- File system stat operations (dry-run compatible)

---

### 3. Pre-mounted Disks - Multiple Disks
**Purpose**: Test using multiple pre-mounted disks

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_PREMOUNTED_DISKS: "/mnt/storage1, /mnt/storage2, /mnt/nvme0"
FIRST_NODE: true
```

**Expected Behavior**:
- Populate mountedDiskMap with:
  - `{"/mnt/storage1": "0", "/mnt/storage2": "1", "/mnt/nvme0": "2"}`
- Trim whitespace from mount point names
- Check all mount points for existing Longhorn files

**Mock Requirements**:
- File system stat operations for all mount points

---

### 4. New Disk Mounting - Empty CLUSTER_DISKS
**Purpose**: Test error handling when no disks specified

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: ""
FIRST_NODE: true
```

**Expected Behavior**:
- **FAIL** with error: "no disks selected for mounting"
- No disk operations performed

**Mock Requirements**: None

---

### 5. New Disk Mounting - Single Unformatted Disk
**Purpose**: Test formatting and mounting a single new disk

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdb"
FIRST_NODE: true
```

**Expected Behavior**:
- Check existing mounts via `mount | awk '/\/mnt\/disk[0-9]+/ {print $3}'`
- Check filesystem: `lsblk -f /dev/sdb` (no ext4 found)
- Check partitions: `lsblk -no PARTTYPE /dev/sdb` (empty)
- Format disk: `mkfs.ext4 -F -F /dev/sdb`
- Get UUID: `blkid -s UUID -o value /dev/sdb`
- Create mount point: `/mnt/disk0`
- Mount: `mount /dev/sdb /mnt/disk0`
- Update fstab with bloom tag
- Run `mount -a` to verify

**Mock Requirements**:
```yaml
MountDrives.GetExistingMounts: ""
MountDrives.LsblkFilesystem: "NAME FSTYPE\nsdb  "
MountDrives.LsblkParttype: ""
MountDrives.BlkidUUID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
PersistMountedDisks.BlkidUUID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
```

---

### 6. New Disk Mounting - Already Formatted Disk
**Purpose**: Test mounting a disk that already has ext4 filesystem

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdc"
FIRST_NODE: true
```

**Expected Behavior**:
- Check filesystem: `lsblk -f /dev/sdc` (ext4 found)
- Skip formatting
- Get UUID and mount
- Update fstab

**Mock Requirements**:
```yaml
MountDrives.GetExistingMounts: ""
MountDrives.LsblkFilesystem: "NAME FSTYPE\nsdc  ext4"
MountDrives.BlkidUUID: "b2c3d4e5-f6a7-8901-bcde-f12345678901"
PersistMountedDisks.BlkidUUID: "b2c3d4e5-f6a7-8901-bcde-f12345678901"
```

---

### 7. New Disk Mounting - Disk with Existing Partitions
**Purpose**: Test handling disk with existing partition table

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdd"
FIRST_NODE: true
```

**Expected Behavior**:
- Check filesystem: no ext4
- Check partitions: `lsblk -no PARTTYPE /dev/sdd` (returns partition type)
- Wipe partitions: `wipefs -a /dev/sdd`
- Format with ext4
- Mount and persist

**Mock Requirements**:
```yaml
MountDrives.GetExistingMounts: ""
MountDrives.LsblkFilesystem: "NAME FSTYPE\nsdd  "
MountDrives.LsblkParttype: "0fc63daf-8483-4772-8e79-3d69d8477de4"
MountDrives.BlkidUUID: "c3d4e5f6-a7b8-9012-cdef-123456789012"
PersistMountedDisks.BlkidUUID: "c3d4e5f6-a7b8-9012-cdef-123456789012"
```

---

### 8. New Disk Mounting - Multiple Disks
**Purpose**: Test mounting multiple disks with sequential mount points

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdb,/dev/sdc,/dev/sdd"
FIRST_NODE: true
```

**Expected Behavior**:
- Mount /dev/sdb at /mnt/disk0
- Mount /dev/sdc at /mnt/disk1
- Mount /dev/sdd at /mnt/disk2
- Add all to fstab with bloom tags
- Run mount -a

**Mock Requirements**:
```yaml
MountDrives.GetExistingMounts: ""
# Separate mocks for each disk's lsblk/blkid operations
MountDrives.LsblkFilesystem: "NAME FSTYPE\nsdX  "  # (for each)
MountDrives.BlkidUUID: "<unique-uuid-for-each-disk>"
```

---

### 9. Mount Point Collision - Existing /mnt/disk0
**Purpose**: Test handling when mount point already exists

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdb"
FIRST_NODE: true
```

**Expected Behavior**:
- Detect /mnt/disk0 is in use
- Use /mnt/disk1 instead
- Mount successfully at /mnt/disk1

**Mock Requirements**:
```yaml
MountDrives.GetExistingMounts: "/mnt/disk0"
MountDrives.LsblkFilesystem: "NAME FSTYPE\nsdb  "
MountDrives.BlkidUUID: "d4e5f6a7-b8c9-0123-def0-123456789abc"
```

---

### 10. Disk Already in Fstab
**Purpose**: Test error when disk UUID already exists in fstab

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdb"
FIRST_NODE: true
```

**Expected Behavior**:
- Get UUID: returns existing UUID
- Check fstab: UUID found
- **FAIL** with error: "disk /dev/sdb is already in /etc/fstab - please remove it first"

**Mock Requirements**:
```yaml
MountDrives.GetExistingMounts: ""
MountDrives.LsblkFilesystem: "NAME FSTYPE\nsdb  ext4"
MountDrives.BlkidUUID: "e5f6a7b8-c9d0-1234-ef01-23456789abcd"
# Note: Would need to mock fstab content reading (file operation in dry-run)
```

---

### 11. Backup Existing Longhorn Config - Single Mount Point
**Purpose**: Test backing up existing longhorn-disk.cfg

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_PREMOUNTED_DISKS: "/mnt/storage1"
FIRST_NODE: true
```

**Setup**:
- `/mnt/storage1/longhorn-disk.cfg` exists

**Expected Behavior**:
- Detect longhorn-disk.cfg at mount point
- Rename to `longhorn-disk.cfg.backup-<timestamp>`
- Log backup operation
- Continue successfully

**Mock Requirements**:
- File stat and rename operations (fsops package)

---

### 12. Backup Existing Replicas Directory
**Purpose**: Test backing up existing Longhorn replicas directory

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_PREMOUNTED_DISKS: "/mnt/storage1"
FIRST_NODE: true
```

**Setup**:
- `/mnt/storage1/replicas/` directory exists

**Expected Behavior**:
- Detect replicas directory
- Rename to `replicas.backup-<timestamp>`
- Log backup operation
- Continue successfully

**Mock Requirements**:
- Directory stat and rename operations

---

### 13. Mount Failure - mkfs.ext4 Fails
**Purpose**: Test error handling when disk formatting fails

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdb"
FIRST_NODE: true
```

**Expected Behavior**:
- Attempt to format disk
- **FAIL** with error: "failed to format /dev/sdb: ..."
- No mount point created
- No fstab entry added

**Mock Requirements**:
```yaml
MountDrives.GetExistingMounts: ""
MountDrives.LsblkFilesystem: "NAME FSTYPE\nsdb  "
MountDrives.LsblkParttype: ""
MountDrives.MkfsExt4: error: "mkfs.ext4 failed - disk error"
```

---

### 14. Mount Failure - mount Command Fails
**Purpose**: Test error handling when mount command fails

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdb"
FIRST_NODE: true
```

**Expected Behavior**:
- Format disk successfully
- Create mount point directory
- Attempt mount
- **FAIL** with error: "failed to mount /dev/sdb at /mnt/disk0: ..."

**Mock Requirements**:
```yaml
MountDrives.GetExistingMounts: ""
MountDrives.LsblkFilesystem: "NAME FSTYPE\nsdb  "
MountDrives.BlkidUUID: "f6a7b8c9-d0e1-2345-f012-3456789abcde"
MountDrives.MountDrive: error: "mount failed - device busy"
```

---

### 15. Persist Failure - No UUID Retrieved
**Purpose**: Test handling when blkid cannot retrieve UUID

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdb"
FIRST_NODE: true
```

**Expected Behavior**:
- Mount disk successfully
- Attempt to persist to fstab
- Cannot get UUID
- Log: "Could not retrieve UUID for <device>. Skipping..."
- Continue (not fatal)

**Mock Requirements**:
```yaml
MountDrives.GetExistingMounts: ""
MountDrives.LsblkFilesystem: "NAME FSTYPE\nsdb  "
MountDrives.BlkidUUID: "a1b2c3d4-uuid"
PersistMountedDisks.BlkidUUID: error: "blkid: cannot open /dev/sdb"
```

---

### 16. Persist Failure - mount -a Fails
**Purpose**: Test error handling when final mount -a fails

**Configuration**:
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdb"
FIRST_NODE: true
```

**Expected Behavior**:
- Mount and add to fstab successfully
- Run `mount -a` to verify
- **FAIL** with error: "failed to remount filesystems: ..."

**Mock Requirements**:
```yaml
MountDrives.GetExistingMounts: ""
MountDrives.LsblkFilesystem: "NAME FSTYPE\nsdb  ext4"
MountDrives.BlkidUUID: "b2c3d4e5-uuid"
PersistMountedDisks.BlkidUUID: "b2c3d4e5-uuid"
PersistMountedDisks.MountAll: error: "mount: mounting failed"
```

---

## Test Priority Recommendations

### High Priority (Must Test)
1. Skip Mode - NO_DISKS_FOR_CLUSTER
2. Pre-mounted Disks - Single Disk
3. Pre-mounted Disks - Multiple Disks
4. New Disk Mounting - Single Unformatted Disk
5. Empty CLUSTER_DISKS Error
6. Backup Existing Longhorn Config

### Medium Priority (Should Test)
7. Already Formatted Disk
8. Disk with Existing Partitions
9. Multiple Disks Mounting
10. Mount Point Collision

### Low Priority (Nice to Have)
11. Backup Replicas Directory
12. Disk Already in Fstab
13. Various failure scenarios (mkfs, mount, UUID, mount -a)

---

## Mock Command Reference

### Common Commands to Mock:
- `MountDrives.GetExistingMounts`: `mount | awk '/\/mnt\/disk[0-9]+/ {print $3}'`
- `MountDrives.LsblkFilesystem`: `lsblk -f <device>`
- `MountDrives.LsblkParttype`: `lsblk -no PARTTYPE <device>`
- `MountDrives.WipefsPartitions`: `wipefs -a <device>`
- `MountDrives.MkfsExt4`: `mkfs.ext4 -F -F <device>`
- `MountDrives.BlkidUUID`: `blkid -s UUID -o value <device>`
- `MountDrives.MountDrive`: `mount <device> <mountpoint>`
- `PersistMountedDisks.BackupFstab`: `cp /etc/fstab /etc/fstab.bak`
- `PersistMountedDisks.BlkidUUID`: `blkid -s UUID -o value <device>`
- `PersistMountedDisks.MountAll`: `mount -a`

### File System Operations (fsops package):
These now support dry-run mocking:
- `fsops.Stat()` for checking file/directory existence (mock pattern: `Stat.<path>`)
- `fsops.Rename()` for backing up config files (dry-run logging)
- `fsops.MkdirAll()` for creating mount point directories (dry-run logging)
- `fsops.ReadFile()` for reading /etc/fstab (mock pattern: `ReadFile.<path>`)

---

## Notes

1. **Timestamp Variability**: Backup filenames include timestamps - tests should be flexible about exact names

2. **File System Operations**: Some operations use `fsops` package which may need special dry-run handling beyond command mocking

3. **Fstab Reading**: The step reads `/etc/fstab` directly via `os.ReadFile()` - this needs to work in dry-run or be mocked differently

4. **Concurrent Mounts**: The step handles existing mount points intelligently by incrementing the disk number

5. **Bloom Tag**: All fstab entries are tagged with `# managed by cluster-bloom` for later cleanup

6. **Non-Fatal UUID Failures**: If UUID cannot be retrieved during persist, it logs a warning but continues

7. **Idempotency**: The step checks for existing mounts and fstab entries to avoid duplicate operations
