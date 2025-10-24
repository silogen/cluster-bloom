# PrepareLonghornDisksStep - Test 15: Persist Failure - No UUID Retrieved

## Purpose
Verify that PrepareLonghornDisksStep handles UUID retrieval failure during fstab persistence gracefully (non-fatal warning).

## Test Scenario
- **Configuration Mode**: New Disk Mounting (single unformatted disk)
- **Failure Point**: UUID retrieval fails during fstab persist (after successful mount)
- **Expected Behavior**: Log warning and skip that disk, but continue overall
- **Test Type**: Partial Success (mount succeeds, persist skipped with warning)

## Configuration
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdb"
ENABLED_STEPS: "PrepareLonghornDisksStep"
FIRST_NODE: true
```

## Expected Behavior

### 1. Mount Phase - Success
- Check existing mounts: No existing mounts
- Read /etc/fstab: Success
- Check filesystem: No filesystem
- Check partitions: No partitions
- Format disk: **Success**
- Get UUID: **Success** (returns a1b2c3d4...)
- Create mount point: /mnt/disk0
- Mount: **Success**
- Log: "Mounted /dev/sdb at /mnt/disk0"
- Disk added to mountedMap

### 2. Persist Phase - UUID Retrieval Fails
- Backup fstab: Success
- For each disk in mountedMap:
  - Attempt to get UUID: `blkid -s UUID -o value /dev/sdb-<uuid>`
  - **UUID retrieval fails**: Returns error
  - Log: "Could not retrieve UUID for /dev/sdb-a1b2c3d4.... Skipping..."
  - **Continue** (non-fatal)

### 3. Complete Successfully
- Step completes with success (mount succeeded)
- Warning logged about UUID retrieval
- Disk is mounted but not in fstab (won't persist across reboots)

## Mock Requirements

```yaml
MountDrives.GetExistingMounts: ""
ReadFile./etc/fstab: "..."
MountDrives.LsblkFilesystem./dev/sdb: "NAME FSTYPE\nsdb"
MountDrives.LsblkParttype./dev/sdb: ""
MountDrives.BlkidUUID./dev/sdb: "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
PersistMountedDisks.BackupFstab: ""
PersistMountedDisks.BlkidUUID./dev/sdb-a1b2c3d4...:
  error: "blkid: cannot open /dev/sdb-a1b2c3d4...: No such file or directory"
```

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/15-uuid-retrieval-failure/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/15-uuid-retrieval-failure/mocks.yaml
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

Check `bloom.log` for the warning:

```
[DRY-RUN] MountDrives.GetExistingMounts: sh -c mount | awk '/\/mnt\/disk[0-9]+/ {print $3}'
[DRY-RUN] MountDrives.LsblkFilesystem./dev/sdb: lsblk -f /dev/sdb
[DRY-RUN] MountDrives.LsblkParttype./dev/sdb: lsblk -no PARTTYPE /dev/sdb
Disk /dev/sdb is not partitioned. Formatting with ext4...
[DRY-RUN] MountDrives.MkfsExt4./dev/sdb: mkfs.ext4 -F -F /dev/sdb
[DRY-RUN] MountDrives.BlkidUUID./dev/sdb: blkid -s UUID -o value /dev/sdb
[DRY-RUN] MKDIR_ALL: /mnt/disk0 (perm: 755)
[DRY-RUN] MountDrives.MountDrive./dev/sdb: mount /dev/sdb /mnt/disk0
Mounted /dev/sdb at /mnt/disk0
[DRY-RUN] PersistMountedDisks.BackupFstab: sudo cp /etc/fstab /etc/fstab.bak
[DRY-RUN] PersistMountedDisks.BlkidUUID./dev/sdb-a1b2c3d4-e5f6-7890-abcd-ef1234567890: blkid -s UUID -o value /dev/sdb-a1b2c3d4-e5f6-7890-abcd-ef1234567890
Could not retrieve UUID for /dev/sdb-a1b2c3d4-e5f6-7890-abcd-ef1234567890. Skipping...
Used 1 disks: map[/mnt/disk0:/dev/sdb-a1b2c3d4-e5f6-7890-abcd-ef1234567890]
Completed in <time>
```

**Key verification points:**
- Mount operations completed successfully
- Disk added to mountedMap
- Fstab backup attempted
- UUID retrieval during persist failed
- **Warning logged (not error)**
- **Step completed successfully** (non-fatal)
- No fstab append operation
- No mount -a verification

## Success Criteria

- ✅ Step executes (not skipped)
- ✅ Mount phase completes successfully
- ✅ Disk formatted and mounted
- ✅ Disk added to mountedMap
- ✅ Fstab backup attempted
- ✅ **UUID retrieval during persist failed**
- ✅ **Warning logged**: "Could not retrieve UUID for <device>. Skipping..."
- ✅ **Step continues** (non-fatal error)
- ✅ No fstab entry created for this disk
- ✅ No mount -a called (no fstab changes)
- ✅ Step completes with success status
- ✅ mountedMap still contains the disk

## Related Code

- PersistMountedDisks function: `pkg/disks.go:349-395`
- UUID retrieval failure handling: `pkg/disks.go:366-373`
  ```go
  uuidOutput, err := command.Output("PersistMountedDisks.BlkidUUID."+device, true, "blkid", "-s", "UUID", "-o", "value", device)
  if err != nil {
      LogMessage(Info, fmt.Sprintf("Could not retrieve UUID for %s. Skipping...", device))
      continue
  }
  uuid := strings.TrimSpace(string(uuidOutput))
  if uuid == "" {
      LogMessage(Info, fmt.Sprintf("Could not retrieve UUID for %s. Skipping...", device))
      continue
  }
  ```

## Notes

- **Non-fatal**: UUID retrieval failure during persist is treated as a warning, not an error
- **Degraded State**: Disk is mounted for current session but won't remount on reboot
- **Continue Loop**: Error skips this disk but processes remaining disks
- **Real-world Scenario**: Transient blkid failure, race condition, or device disappeared
- **User Impact**: Disk is usable now but needs manual fstab entry for persistence
- **Comparison to Mount Phase**: UUID retrieval during mount is required; during persist is optional
