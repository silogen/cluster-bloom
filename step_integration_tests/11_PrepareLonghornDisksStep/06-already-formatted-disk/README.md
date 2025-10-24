# PrepareLonghornDisksStep - Test 06: New Disk Mounting - Already Formatted Disk

## Purpose
Verify that PrepareLonghornDisksStep correctly handles a disk that already has an ext4 filesystem, skipping the formatting step and proceeding directly to mounting.

## Test Scenario
- **Configuration Mode**: New Disk Mounting (disk with existing ext4)
- **Expected Behavior**: Skip formatting, mount at /mnt/disk0, add to fstab
- **Test Type**: Success (mount existing formatted disk)

## Configuration
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdc"
ENABLED_STEPS: "PrepareLonghornDisksStep"
FIRST_NODE: true
```

## Expected Behavior

### 1. Mount Preparation
- Check existing mounts: `mount | awk '/\/mnt\/disk[0-9]+/ {print $3}'`
- Result: No existing mounts found
- Determine mount point: `/mnt/disk0`

### 2. Filesystem Check
- Check filesystem: `lsblk -f /dev/sdc` → ext4 found
- Log: "Disk /dev/sdc is already formatted as ext4. Skipping format."
- **Skip formatting step** (no mkfs.ext4 command)

### 3. UUID Retrieval and Mounting
- Get UUID: `blkid -s UUID -o value /dev/sdc` → Returns UUID
- Create mount point directory: `/mnt/disk0` (dry-run: logged by fsops)
- **Mount disk**: `mount /dev/sdc /mnt/disk0` (dry-run: logged only)
- Add to mountedDiskMap: `{"/mnt/disk0": "/dev/sdc-<uuid>"}`

### 4. Persist to fstab
- Backup fstab: `sudo cp /etc/fstab /etc/fstab.bak`
- Get UUID again: `blkid -s UUID -o value /dev/sdc-<uuid>`
- Add fstab entry with bloom tag: `UUID=<uuid> /mnt/disk0 ext4 defaults,nofail 0 2 # managed by cluster-bloom`
- Verify mount: `sudo mount -a`

### 5. Final Output
- Log: "Used 1 disks: map[/mnt/disk0:/dev/sdc-<uuid>]"
- Store disk map in viper for subsequent steps

## Mock Requirements

All mock commands except mkfs.ext4 (formatting is skipped):

```yaml
MountDrives.GetExistingMounts: ""              # No existing mounts
MountDrives.LsblkFilesystem: "NAME FSTYPE\nsdc  ext4"  # ext4 found
MountDrives.BlkidUUID: "<uuid>"                # UUID for mounting
PersistMountedDisks.BackupFstab: ""            # Backup success
PersistMountedDisks.BlkidUUID: "<uuid>"        # UUID for fstab
PersistMountedDisks.MountAll: ""               # Mount verification
```

**Note**: No mock for `MountDrives.MkfsExt4` because formatting is skipped when ext4 is detected.

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/06-already-formatted-disk/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/06-already-formatted-disk/mocks.yaml
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

### 2. Filesystem Check (ext4 detected)
```
[DRY-RUN] MountDrives.LsblkFilesystem: lsblk -f /dev/sdc
Disk /dev/sdc is already formatted as ext4. Skipping format.
```

### 3. No Formatting Command
```
# NO mkfs.ext4 command should appear in the log
# Formatting is skipped because ext4 was detected
```

### 4. UUID Retrieval and Mounting
```
[DRY-RUN] MountDrives.BlkidUUID: blkid -s UUID -o value /dev/sdc
[DRY-RUN] MKDIR_ALL: /mnt/disk0 (perm: 755)
[DRY-RUN] MountDrives.MountDrive: mount /dev/sdc /mnt/disk0
Mounted /dev/sdc at /mnt/disk0
```

### 5. Fstab Operations
```
[DRY-RUN] PersistMountedDisks.BackupFstab: sudo cp /etc/fstab /etc/fstab.bak
[DRY-RUN] PersistMountedDisks.BlkidUUID: blkid -s UUID -o value <device>
[DRY-RUN] CREATE_CMD: sudo tee -a /etc/fstab
[DRY-RUN] PersistMountedDisks.MountAll: sudo mount -a
```

### 6. Final Summary
```
Used 1 disks: map[/mnt/disk0:/dev/sdc-b2c3d4e5-f6a7-8901-bcde-f12345678901]
Completed in <time>
```

## Success Criteria

- ✅ Step executes (not skipped)
- ✅ Existing mounts checked
- ✅ Filesystem check: ext4 found
- ✅ Log message: "Disk /dev/sdc is already formatted as ext4. Skipping format."
- ✅ **No formatting command executed** (mkfs.ext4 not called)
- ✅ Partition check skipped (only done when no ext4 found)
- ✅ UUID retrieved via blkid
- ✅ Mount point directory created: `/mnt/disk0`
- ✅ Mount command logged: `mount /dev/sdc /mnt/disk0`
- ✅ Fstab backup created
- ✅ Fstab entry added with bloom tag
- ✅ Mount verification: `mount -a` executed
- ✅ Disk map created: `{"/mnt/disk0": "/dev/sdc-<uuid>"}`
- ✅ Step completes successfully
- ✅ No errors logged

## Related Code

- Step definition: `pkg/steps.go:285` (PrepareLonghornDisksStep)
- MountDrives: `pkg/disks.go:261-340`
- Filesystem check (line 292-294):
  ```go
  if strings.Contains(string(output), "ext4") {
      LogMessage(Info, fmt.Sprintf("Disk %s is already formatted as ext4. Skipping format.", drive))
  }
  ```
- PersistMountedDisks: `pkg/disks.go:342-399`

## Notes

- **Format Skipping**: This test validates that the code correctly detects existing ext4 filesystems and avoids reformatting
- **Efficiency**: Skipping format saves time and preserves any existing data structure on the disk
- **Idempotency**: Re-running the step on already-formatted disks should be safe
- **UUID Consistency**: The same UUID is used throughout (no new UUID from mkfs.ext4)
- **Dry-run Behavior**: All disk operations (mount) are logged but not executed; mkfs.ext4 is not even attempted
- **Comparison to Test 05**: Unlike test 05 which formats a blank disk, this test mounts an existing formatted disk
