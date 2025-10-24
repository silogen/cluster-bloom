# PrepareLonghornDisksStep - Test 05: New Disk Mounting - Single Unformatted Disk

## Purpose
Verify that PrepareLonghornDisksStep correctly formats and mounts a single new unformatted disk, including persisting it to fstab.

## Test Scenario
- **Configuration Mode**: New Disk Mounting (single disk)
- **Expected Behavior**: Format disk with ext4, mount at /mnt/disk0, add to fstab
- **Test Type**: Success (complete disk preparation pipeline)

## Configuration
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdb"
ENABLED_STEPS: "PrepareLonghornDisksStep"
FIRST_NODE: true
```

## Expected Behavior

### 1. Mount Preparation
- Check existing mounts: `mount | awk '/\/mnt\/disk[0-9]+/ {print $3}'`
- Result: No existing mounts found
- Determine mount point: `/mnt/disk0`

### 2. Disk Formatting
- Check filesystem: `lsblk -f /dev/sdb` → No filesystem found
- Check partitions: `lsblk -no PARTTYPE /dev/sdb` → No partitions
- **Format disk**: `mkfs.ext4 -F -F /dev/sdb` (dry-run: logged only)
- Get UUID: `blkid -s UUID -o value /dev/sdb` → Returns UUID

### 3. Mounting
- Create mount point directory: `/mnt/disk0` (dry-run: logged by fsops)
- **Mount disk**: `mount /dev/sdb /mnt/disk0` (dry-run: logged only)
- Add to mountedDiskMap: `{"/mnt/disk0": "/dev/sdb-<uuid>"}`

### 4. Persist to fstab
- Backup fstab: `sudo cp /etc/fstab /etc/fstab.bak`
- Get UUID again: `blkid -s UUID -o value /dev/sdb-<uuid>`
- Add fstab entry with bloom tag: `UUID=<uuid> /mnt/disk0 ext4 defaults,nofail 0 2 # managed by cluster-bloom`
- Verify mount: `sudo mount -a`

### 5. Final Output
- Log: "Used 1 disks: map[/mnt/disk0:/dev/sdb-<uuid>]"
- Store disk map in viper for subsequent steps

## Mock Requirements

All mock commands are required for the full disk mounting workflow:

```yaml
MountDrives.GetExistingMounts: ""              # No existing mounts
MountDrives.LsblkFilesystem: "NAME FSTYPE\nsdb"  # No filesystem
MountDrives.LsblkParttype: ""                  # No partitions
MountDrives.BlkidUUID: "<uuid>"                # UUID after format
PersistMountedDisks.BackupFstab: ""            # Backup success
PersistMountedDisks.BlkidUUID: "<uuid>"        # UUID for fstab
PersistMountedDisks.MountAll: ""               # Mount verification
```

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/04-new-disk-single/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/04-new-disk-single/mocks.yaml
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

### 2. Filesystem Checks
```
[DRY-RUN] MountDrives.LsblkFilesystem: lsblk -f /dev/sdb
[DRY-RUN] MountDrives.LsblkParttype: lsblk -no PARTTYPE /dev/sdb
```

### 3. Disk Formatting
```
Disk /dev/sdb is not partitioned. Formatting with ext4...
[DRY-RUN] MountDrives.MkfsExt4: mkfs.ext4 -F -F /dev/sdb
```

### 4. UUID Retrieval
```
[DRY-RUN] MountDrives.BlkidUUID: blkid -s UUID -o value /dev/sdb
```

### 5. Mount Point Creation and Mounting
```
[DRY-RUN] MKDIR_ALL: /mnt/disk0 (perm: 755)
[DRY-RUN] MountDrives.MountDrive: mount /dev/sdb /mnt/disk0
Mounted /dev/sdb at /mnt/disk0
```

### 6. Fstab Operations
```
[DRY-RUN] PersistMountedDisks.BackupFstab: sudo cp /etc/fstab /etc/fstab.bak
[DRY-RUN] PersistMountedDisks.BlkidUUID: blkid -s UUID -o value <device>
[DRY-RUN] CREATE_CMD: sudo tee -a /etc/fstab
[DRY-RUN] PersistMountedDisks.MountAll: sudo mount -a
```

### 7. Final Summary
```
Used 1 disks: map[/mnt/disk0:/dev/sdb-a1b2c3d4-e5f6-7890-abcd-ef1234567890]
Completed in <time>
```

## Success Criteria

- ✅ Step executes (not skipped)
- ✅ Existing mounts checked
- ✅ Filesystem check: no ext4 found
- ✅ Partition check: no partitions found
- ✅ Format command logged: `mkfs.ext4 -F -F /dev/sdb`
- ✅ UUID retrieved via blkid
- ✅ Mount point directory created: `/mnt/disk0`
- ✅ Mount command logged: `mount /dev/sdb /mnt/disk0`
- ✅ Fstab backup created
- ✅ Fstab entry added with bloom tag
- ✅ Mount verification: `mount -a` executed
- ✅ Disk map created: `{"/mnt/disk0": "/dev/sdb-<uuid>"}`
- ✅ Step completes successfully
- ✅ No errors logged

## Related Code

- Step definition: `pkg/steps.go:285` (PrepareLonghornDisksStep)
- MountDrives: `pkg/disks.go:261-340`
- PersistMountedDisks: `pkg/disks.go:342-399`

## Notes

- **Dry-run Behavior**: All disk operations (mkfs, mount) are logged but not executed
- **fsops Operations**: Directory creation via `fsops.MkdirAll()` is logged with `[DRY-RUN] MKDIR_ALL`
- **UUID Consistency**: The same UUID should be used in both MountDrives and PersistMountedDisks mocks
- **Fstab Tag**: All entries include `# managed by cluster-bloom` for cleanup tracking
- **Full Pipeline**: This test exercises the complete disk preparation workflow from raw disk to mounted and persisted
