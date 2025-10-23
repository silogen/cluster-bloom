# PrepareLonghornDisksStep - Test 16: Persist Failure - mount -a Fails

## Purpose
Verify that PrepareLonghornDisksStep properly handles and reports errors when the final `mount -a` verification command fails after adding entries to fstab.

## Test Scenario
- **Configuration Mode**: New Disk Mounting (single unformatted disk)
- **Failure Point**: mount -a verification fails after fstab update
- **Expected Behavior**: Fail with clear error message after fstab modification
- **Test Type**: Failure (expected error condition)

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
- Get UUID: **Success**
- Create mount point: /mnt/disk0
- Mount: **Success**
- Disk added to mountedMap

### 2. Persist Phase - Success Until Verification
- Backup fstab: **Success**
- Get UUID for persist: **Success**
- Check UUID not already in fstab: **Success**
- Append fstab entry: **Success**
- Verify with mount -a: **FAIL**

### 3. Error Handling
- **FAIL** with error: "failed to remount filesystems: mount: mounting UUID=... failed: Invalid argument"
- Fstab has been modified but verification failed
- System may be in inconsistent state

## Mock Requirements

```yaml
MountDrives.GetExistingMounts: ""
ReadFile./etc/fstab: "..."
MountDrives.LsblkFilesystem./dev/sdb: "NAME FSTYPE\nsdb"
MountDrives.LsblkParttype./dev/sdb: ""
MountDrives.BlkidUUID./dev/sdb: "b2c3d4e5-f6a7-8901-bcde-f12345678901"
PersistMountedDisks.BackupFstab: ""
PersistMountedDisks.BlkidUUID./dev/sdb-b2c3d4e5...: "b2c3d4e5-f6a7-8901-bcde-f12345678901"
PersistMountedDisks.MountAll:
  error: "mount: mounting UUID=b2c3d4e5... on /mnt/disk0 failed: Invalid argument"
```

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/16-mount-a-failure/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/16-mount-a-failure/mocks.yaml
```

## Expected Output

```
🚀 Starting installation with 1 steps
════════════════════════════════════════

[1/1] Prepare Longhorn Disks
      Mount selected disks or populate disk map from CLUSTER_PREMOUNTED_DISKS configuration
      ❌ FAILED: failed to remount filesystems: mount: mounting UUID=b2c3d4e5-f6a7-8901-bcde-f12345678901 on /mnt/disk0 failed: Invalid argument
```

## Verification

Check `bloom.log` for the failure sequence:

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
[DRY-RUN] PersistMountedDisks.BlkidUUID./dev/sdb-b2c3d4e5-f6a7-8901-bcde-f12345678901: blkid -s UUID -o value /dev/sdb-b2c3d4e5-f6a7-8901-bcde-f12345678901
[DRY-RUN] CREATE_CMD: sudo tee -a /etc/fstab
[DRY-RUN] PersistMountedDisks.MountAll: sudo mount -a
Execution failed: failed to remount filesystems: mount: mounting UUID=b2c3d4e5-f6a7-8901-bcde-f12345678901 on /mnt/disk0 failed: Invalid argument
```

**Key verification points:**
- Mount operations completed successfully
- Fstab backup completed
- UUID retrieved for persist
- Fstab entry appended (logged)
- **mount -a attempted**
- **mount -a failed with error**
- Error message propagated

## Success Criteria

- ✅ Step executes (not skipped)
- ✅ Mount phase completes successfully
- ✅ Disk formatted, mounted, and added to mountedMap
- ✅ Fstab backup completed
- ✅ UUID retrieved for persist
- ✅ Fstab entry appended
- ✅ **mount -a verification attempted**
- ✅ **mount -a failed with error from mock**
- ✅ **Error propagated correctly**
- ✅ Step fails with expected error
- ✅ Error message includes mount -a output

## Related Code

- PersistMountedDisks function: `pkg/disks.go:349-395`
- mount -a verification: `pkg/disks.go:391-394`
  ```go
  if err := command.SimpleRun("PersistMountedDisks.MountAll", false, "sudo", "mount", "-a"); err != nil {
      return fmt.Errorf("failed to remount filesystems: %w", err)
  }
  ```

## Notes

- **Final Verification**: mount -a verifies all fstab entries are valid
- **Inconsistent State**: Fstab modified but verification failed
- **Backup Available**: Fstab backup exists if manual recovery needed
- **Error Propagation**: Error from mount -a wrapped with context
- **Real-world Scenario**: Invalid fstab syntax, UUID mismatch, or filesystem incompatibility
- **Recovery**: User can restore from /etc/fstab.bak
- **Dry-run Behavior**: Mock error simulates verification failure
- **User Impact**: Installation fails but system state is known (fstab modified, backup available)
