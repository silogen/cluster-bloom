# PrepareLonghornDisksStep - Test 07: New Disk Mounting - Disk with Existing Partitions

## Purpose
Verify that PrepareLonghornDisksStep correctly handles a disk with existing partition table by wiping partitions before formatting with ext4.

## Test Scenario
- **Configuration Mode**: New Disk Mounting (disk with existing partitions)
- **Expected Behavior**: Detect partitions, wipe them, format with ext4, mount at /mnt/disk0, add to fstab
- **Test Type**: Success (complete partition wipe and mount workflow)

## Configuration
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdd"
ENABLED_STEPS: "PrepareLonghornDisksStep"
FIRST_NODE: true
```

## Expected Behavior

### 1. Mount Preparation
- Check existing mounts: `mount | awk '/\/mnt\/disk[0-9]+/ {print $3}'`
- Result: No existing mounts found
- Determine mount point: `/mnt/disk0`

### 2. Filesystem and Partition Checks
- Check filesystem: `lsblk -f /dev/sdd` → No filesystem found
- Check partitions: `lsblk -no PARTTYPE /dev/sdd` → Returns partition type GUID
- Log: "Disk /dev/sdd has existing partitions. Removing partitions..."

### 3. Partition Wiping
- **Wipe partitions**: `sudo wipefs -a /dev/sdd` (dry-run: logged only)
- This removes all partition table signatures from the disk

### 4. Disk Formatting
- Log: "Disk /dev/sdd is not partitioned. Formatting with ext4..."
- **Format disk**: `mkfs.ext4 -F -F /dev/sdd` (dry-run: logged only)
- Get UUID: `blkid -s UUID -o value /dev/sdd` → Returns UUID

### 5. Mounting
- Create mount point directory: `/mnt/disk0` (dry-run: logged by fsops)
- **Mount disk**: `mount /dev/sdd /mnt/disk0` (dry-run: logged only)
- Add to mountedDiskMap: `{"/mnt/disk0": "/dev/sdd-<uuid>"}`
- Log: "Mounted /dev/sdd at /mnt/disk0"

### 6. Persist to fstab
- Backup fstab: `sudo cp /etc/fstab /etc/fstab.bak`
- Get UUID again: `blkid -s UUID -o value /dev/sdd-<uuid>`
- Add fstab entry with bloom tag: `UUID=<uuid> /mnt/disk0 ext4 defaults,nofail 0 2 # managed by cluster-bloom`
- Verify mount: `sudo mount -a`

### 7. Final Output
- Log: "Used 1 disks: map[/mnt/disk0:/dev/sdd-<uuid>]"
- Store disk map in viper for subsequent steps

## Mock Requirements

All mock commands including wipefs for partition removal:

```yaml
MountDrives.GetExistingMounts: ""              # No existing mounts
MountDrives.LsblkFilesystem: "NAME FSTYPE\nsdd"  # No filesystem
MountDrives.LsblkParttype: "0fc63daf-8483-4772-8e79-3d69d8477de4"  # Linux partition GUID
MountDrives.WipefsPartitions: ""               # Wipe success
MountDrives.BlkidUUID: "<uuid>"                # UUID after format
PersistMountedDisks.BackupFstab: ""            # Backup success
PersistMountedDisks.BlkidUUID: "<uuid>"        # UUID for fstab
PersistMountedDisks.MountAll: ""               # Mount verification
```

**Note**: This test includes the `MountDrives.WipefsPartitions` mock which is not needed in tests 05 or 06.

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/07-disk-with-partitions/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/07-disk-with-partitions/mocks.yaml
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

Check `bloom.log` for the complete workflow:

### 1. Mount Preparation
```
[DRY-RUN] MountDrives.GetExistingMounts: sh -c mount | awk '/\/mnt\/disk[0-9]+/ {print $3}'
```

### 2. Filesystem Check (no filesystem)
```
[DRY-RUN] MountDrives.LsblkFilesystem: lsblk -f /dev/sdd
```

### 3. Partition Check (partitions found)
```
[DRY-RUN] MountDrives.LsblkParttype: lsblk -no PARTTYPE /dev/sdd
Disk /dev/sdd has existing partitions. Removing partitions...
```

### 4. Partition Wiping
```
[DRY-RUN] MountDrives.WipefsPartitions: sudo wipefs -a /dev/sdd
```

### 5. Disk Formatting
```
Disk /dev/sdd is not partitioned. Formatting with ext4...
[DRY-RUN] MountDrives.MkfsExt4: mkfs.ext4 -F -F /dev/sdd
```

### 6. UUID Retrieval and Mounting
```
[DRY-RUN] MountDrives.BlkidUUID: blkid -s UUID -o value /dev/sdd
[DRY-RUN] MKDIR_ALL: /mnt/disk0 (perm: 755)
[DRY-RUN] MountDrives.MountDrive: mount /dev/sdd /mnt/disk0
Mounted /dev/sdd at /mnt/disk0
```

### 7. Fstab Operations
```
[DRY-RUN] PersistMountedDisks.BackupFstab: sudo cp /etc/fstab /etc/fstab.bak
[DRY-RUN] PersistMountedDisks.BlkidUUID: blkid -s UUID -o value <device>
[DRY-RUN] CREATE_CMD: sudo tee -a /etc/fstab
[DRY-RUN] PersistMountedDisks.MountAll: sudo mount -a
```

### 8. Final Summary
```
Used 1 disks: map[/mnt/disk0:/dev/sdd-c3d4e5f6-a7b8-9012-cdef-123456789012]
Completed in <time>
```

## Success Criteria

- ✅ Step executes (not skipped)
- ✅ Existing mounts checked
- ✅ Filesystem check: no ext4 found
- ✅ Partition check: partition type GUID found
- ✅ Log message: "Disk /dev/sdd has existing partitions. Removing partitions..."
- ✅ **Wipefs command executed**: `sudo wipefs -a /dev/sdd`
- ✅ Format command logged: `mkfs.ext4 -F -F /dev/sdd`
- ✅ UUID retrieved via blkid
- ✅ Mount point directory created: `/mnt/disk0`
- ✅ Mount command logged: `mount /dev/sdd /mnt/disk0`
- ✅ Fstab backup created
- ✅ Fstab entry added with bloom tag
- ✅ Mount verification: `mount -a` executed
- ✅ Disk map created: `{"/mnt/disk0": "/dev/sdd-<uuid>"}`
- ✅ Step completes successfully
- ✅ No errors logged

## Related Code

- Step definition: `pkg/steps.go:285` (PrepareLonghornDisksStep)
- MountDrives: `pkg/disks.go:261-340`
- Partition check and wipe (lines 295-303):
  ```go
  output, err = command.Output("MountDrives.LsblkParttype", true, "lsblk", "-no", "PARTTYPE", drive)
  if err != nil {
      return mountedMap, fmt.Errorf("failed to check partition type for %s: %w", drive, err)
  }
  if strings.TrimSpace(string(output)) != "" {
      LogMessage(Info, fmt.Sprintf("Disk %s has existing partitions. Removing partitions...", drive))
      if err := command.SimpleRun("MountDrives.WipefsPartitions", false, "sudo", "wipefs", "-a", drive); err != nil {
          return mountedMap, fmt.Errorf("failed to wipe partitions on %s: %w", drive, err)
      }
  }
  ```
- PersistMountedDisks: `pkg/disks.go:342-399`

## Notes

- **Partition Wiping**: This test validates proper cleanup of existing partition tables before formatting
- **Safety**: The code uses `wipefs -a` which removes all filesystem and partition signatures
- **Two-Step Process**: First wipe partitions, then format with ext4
- **Dry-run Behavior**: All disk operations (wipefs, mkfs, mount) are logged but not executed
- **Comparison to Test 05**: Test 05 has no partitions; this test has partitions that must be wiped first
- **Comparison to Test 06**: Test 06 has ext4 and skips formatting; this test has no ext4 and must format
- **GUID Format**: The partition type GUID "0fc63daf-8483-4772-8e79-3d69d8477de4" is the standard Linux filesystem partition type
