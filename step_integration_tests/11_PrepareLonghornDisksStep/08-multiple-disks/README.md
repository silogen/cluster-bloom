# PrepareLonghornDisksStep - Test 08: New Disk Mounting - Multiple Disks

## Purpose
Verify that PrepareLonghornDisksStep correctly mounts multiple disks with sequential mount points and unique UUIDs.

## Test Scenario
- **Configuration Mode**: New Disk Mounting (multiple unformatted disks)
- **Expected Behavior**: Mount three disks at /mnt/disk0, /mnt/disk1, /mnt/disk2 with unique UUIDs, add all to fstab
- **Test Type**: Success (complete multi-disk mount workflow)

## Configuration
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdb,/dev/sdc,/dev/sdd"
ENABLED_STEPS: "PrepareLonghornDisksStep"
FIRST_NODE: true
```

## Expected Behavior

### 1. Mount Preparation
- Check existing mounts: `mount | awk '/\/mnt\/disk[0-9]+/ {print $3}'`
- Result: No existing mounts found
- Determine mount points: `/mnt/disk0`, `/mnt/disk1`, `/mnt/disk2`

### 2. Process Each Disk Sequentially

#### Disk 1: /dev/sdb → /mnt/disk0
- Check filesystem: `lsblk -f /dev/sdb` → No filesystem
- Check partitions: `lsblk -no PARTTYPE /dev/sdb` → No partitions
- Format: `mkfs.ext4 -F -F /dev/sdb`
- Get UUID: `blkid -s UUID -o value /dev/sdb` → Returns UUID a1111111...
- Create mount point: `/mnt/disk0`
- Mount: `mount /dev/sdb /mnt/disk0`

#### Disk 2: /dev/sdc → /mnt/disk1
- Check filesystem: `lsblk -f /dev/sdc` → No filesystem
- Check partitions: `lsblk -no PARTTYPE /dev/sdc` → No partitions
- Format: `mkfs.ext4 -F -F /dev/sdc`
- Get UUID: `blkid -s UUID -o value /dev/sdc` → Returns UUID b2222222...
- Create mount point: `/mnt/disk1`
- Mount: `mount /dev/sdc /mnt/disk1`

#### Disk 3: /dev/sdd → /mnt/disk2
- Check filesystem: `lsblk -f /dev/sdd` → No filesystem
- Check partitions: `lsblk -no PARTTYPE /dev/sdd` → No partitions
- Format: `mkfs.ext4 -F -F /dev/sdd`
- Get UUID: `blkid -s UUID -o value /dev/sdd` → Returns UUID c3333333...
- Create mount point: `/mnt/disk2`
- Mount: `mount /dev/sdd /mnt/disk2`

### 3. Persist All Disks to fstab
- Backup fstab: `sudo cp /etc/fstab /etc/fstab.bak` (once)
- For each disk:
  - Get UUID: `blkid -s UUID -o value <device>-<uuid>`
  - Add fstab entry with bloom tag
- Verify: `sudo mount -a`

### 4. Final Output
- Log: "Used 3 disks: map[/mnt/disk0:/dev/sdb-a111... /mnt/disk1:/dev/sdc-b222... /mnt/disk2:/dev/sdd-c333...]"
- Store disk map in viper for subsequent steps

## Mock Requirements

Separate mock values for each disk (note the drive suffix appended to mock names):

```yaml
MountDrives.GetExistingMounts: ""

# Disk 1: /dev/sdb
MountDrives.LsblkFilesystem./dev/sdb: "NAME FSTYPE\nsdb"
MountDrives.LsblkParttype./dev/sdb: ""
MountDrives.BlkidUUID./dev/sdb: "a1111111-1111-1111-1111-111111111111"

# Disk 2: /dev/sdc
MountDrives.LsblkFilesystem./dev/sdc: "NAME FSTYPE\nsdc"
MountDrives.LsblkParttype./dev/sdc: ""
MountDrives.BlkidUUID./dev/sdc: "b2222222-2222-2222-2222-222222222222"

# Disk 3: /dev/sdd
MountDrives.LsblkFilesystem./dev/sdd: "NAME FSTYPE\nsdd"
MountDrives.LsblkParttype./dev/sdd: ""
MountDrives.BlkidUUID./dev/sdd: "c3333333-3333-3333-3333-333333333333"

# Persist operations
PersistMountedDisks.BackupFstab: ""
PersistMountedDisks.BlkidUUID./dev/sdb-a111...: "a1111111-1111-1111-1111-111111111111"
PersistMountedDisks.BlkidUUID./dev/sdc-b222...: "b2222222-2222-2222-2222-222222222222"
PersistMountedDisks.BlkidUUID./dev/sdd-c333...: "c3333333-3333-3333-3333-333333333333"
PersistMountedDisks.MountAll: ""
```

**Note**: This test requires the code changes that append the drive name to mock keys (e.g., `"MountDrives.LsblkFilesystem."+drive`).

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/08-multiple-disks/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/08-multiple-disks/mocks.yaml
```

## Expected Output

```
🚀 Starting installation with 1 steps
════════════════════════════════════════

[1/1] Prepare Longhorn Disks
      Mount selected disks or populate disk map from CLUSTER_PREMOUNTED_DISKS configuration
      ✅ COMPLETED in <time>
```

## Verification

Check `bloom.log` for the complete workflow showing all three disks processed:

### Disk 1: /dev/sdb → /mnt/disk0
```
[DRY-RUN] MountDrives.LsblkFilesystem./dev/sdb: lsblk -f /dev/sdb
[DRY-RUN] MountDrives.LsblkParttype./dev/sdb: lsblk -no PARTTYPE /dev/sdb
Disk /dev/sdb is not partitioned. Formatting with ext4...
[DRY-RUN] MountDrives.MkfsExt4./dev/sdb: mkfs.ext4 -F -F /dev/sdb
[DRY-RUN] MountDrives.BlkidUUID./dev/sdb: blkid -s UUID -o value /dev/sdb
[DRY-RUN] MKDIR_ALL: /mnt/disk0 (perm: 755)
[DRY-RUN] MountDrives.MountDrive./dev/sdb: mount /dev/sdb /mnt/disk0
Mounted /dev/sdb at /mnt/disk0
```

### Disk 2: /dev/sdc → /mnt/disk1
```
[DRY-RUN] MountDrives.LsblkFilesystem./dev/sdc: lsblk -f /dev/sdc
[DRY-RUN] MountDrives.LsblkParttype./dev/sdc: lsblk -no PARTTYPE /dev/sdc
Disk /dev/sdc is not partitioned. Formatting with ext4...
[DRY-RUN] MountDrives.MkfsExt4./dev/sdc: mkfs.ext4 -F -F /dev/sdc
[DRY-RUN] MountDrives.BlkidUUID./dev/sdc: blkid -s UUID -o value /dev/sdc
[DRY-RUN] MKDIR_ALL: /mnt/disk1 (perm: 755)
[DRY-RUN] MountDrives.MountDrive./dev/sdc: mount /dev/sdc /mnt/disk1
Mounted /dev/sdc at /mnt/disk1
```

### Disk 3: /dev/sdd → /mnt/disk2
```
[DRY-RUN] MountDrives.LsblkFilesystem./dev/sdd: lsblk -f /dev/sdd
[DRY-RUN] MountDrives.LsblkParttype./dev/sdd: lsblk -no PARTTYPE /dev/sdd
Disk /dev/sdd is not partitioned. Formatting with ext4...
[DRY-RUN] MountDrives.MkfsExt4./dev/sdd: mkfs.ext4 -F -F /dev/sdd
[DRY-RUN] MountDrives.BlkidUUID./dev/sdd: blkid -s UUID -o value /dev/sdd
[DRY-RUN] MKDIR_ALL: /mnt/disk2 (perm: 755)
[DRY-RUN] MountDrives.MountDrive./dev/sdd: mount /dev/sdd /mnt/disk2
Mounted /dev/sdd at /mnt/disk2
```

### Fstab Operations (All Disks)
```
[DRY-RUN] PersistMountedDisks.BackupFstab: sudo cp /etc/fstab /etc/fstab.bak
[DRY-RUN] PersistMountedDisks.BlkidUUID./dev/sdb-a1111111-1111-1111-1111-111111111111: blkid -s UUID -o value /dev/sdb-a1111111-1111-1111-1111-111111111111
[DRY-RUN] CREATE_CMD: sudo tee -a /etc/fstab
[DRY-RUN] PersistMountedDisks.BlkidUUID./dev/sdc-b2222222-2222-2222-2222-222222222222: blkid -s UUID -o value /dev/sdc-b2222222-2222-2222-2222-222222222222
[DRY-RUN] CREATE_CMD: sudo tee -a /etc/fstab
[DRY-RUN] PersistMountedDisks.BlkidUUID./dev/sdd-c3333333-3333-3333-3333-333333333333: blkid -s UUID -o value /dev/sdd-c3333333-3333-3333-3333-333333333333
[DRY-RUN] CREATE_CMD: sudo tee -a /etc/fstab
[DRY-RUN] PersistMountedDisks.MountAll: sudo mount -a
```

### Final Summary
```
Used 3 disks: map[/mnt/disk0:/dev/sdb-a1111111-1111-1111-1111-111111111111 /mnt/disk1:/dev/sdc-b2222222-2222-2222-2222-222222222222 /mnt/disk2:/dev/sdd-c3333333-3333-3333-3333-333333333333]
Completed in <time>
```

## Success Criteria

- ✅ Step executes (not skipped)
- ✅ Existing mounts checked
- ✅ **Three disks processed sequentially**
- ✅ Each disk: filesystem check, partition check, format, UUID retrieval
- ✅ **Sequential mount points**: /mnt/disk0, /mnt/disk1, /mnt/disk2
- ✅ **Unique UUIDs** for each disk
- ✅ Three mkdir operations for mount points
- ✅ Three mount operations
- ✅ Fstab backup created (once)
- ✅ **Three fstab entries** added (one per disk)
- ✅ Mount verification: `mount -a` executed
- ✅ Disk map contains all three disks with unique UUIDs
- ✅ Step completes successfully
- ✅ No errors logged

## Related Code

- Step definition: `pkg/steps.go:285` (PrepareLonghornDisksStep)
- MountDrives loop: `pkg/disks.go:287-340`
- Sequential mount point logic (lines 319-323):
  ```go
  mountPoint := fmt.Sprintf("/mnt/disk%d", i)
  for usedMountPoints[mountPoint] || strings.Contains(string(fstabContent), mountPoint) {
      i++
      mountPoint = fmt.Sprintf("/mnt/disk%d", i)
  }
  ```
- Per-disk mock names (line 288):
  ```go
  output, err := command.Output("MountDrives.LsblkFilesystem."+drive, true, "lsblk", "-f", drive)
  ```
- PersistMountedDisks loop: `pkg/disks.go:363-392`

## Notes

- **Sequential Processing**: Disks are mounted one at a time in the order specified in CLUSTER_DISKS
- **Mount Point Increment**: The counter `i` increments for each disk, creating /mnt/disk0, /mnt/disk1, /mnt/disk2
- **Unique UUIDs**: Each disk gets its own UUID from blkid after formatting
- **Per-Disk Mocks**: The mock system appends the drive name to keys to support unique values per disk
- **Code Change Required**: This test requires the code changes that append `+drive` to mock names in pkg/disks.go
- **Dry-run Behavior**: All disk operations (mkfs, mount) are logged but not executed for all disks
- **Comparison to Test 05**: Test 05 mounts one disk; this test mounts three with sequential mount points

## Code Changes Required

This test required modifications to `pkg/disks.go` to append the drive name to mock keys:

```go
// Before:
output, err := command.Output("MountDrives.LsblkFilesystem", true, "lsblk", "-f", drive)

// After:
output, err := command.Output("MountDrives.LsblkFilesystem."+drive, true, "lsblk", "-f", drive)
```

This change was applied to all command calls in the drive processing loop:
- `MountDrives.LsblkFilesystem.`+drive
- `MountDrives.LsblkParttype.`+drive
- `MountDrives.WipefsPartitions.`+drive
- `MountDrives.MkfsExt4.`+drive
- `MountDrives.BlkidUUID.`+drive
- `MountDrives.MountDrive.`+drive
- `PersistMountedDisks.BlkidUUID.`+device

All previous tests (05, 06, 07) were updated to use the new naming convention.
