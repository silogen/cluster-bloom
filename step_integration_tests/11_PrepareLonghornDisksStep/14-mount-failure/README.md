# PrepareLonghornDisksStep - Test 14: Mount Failure - mount Command Fails

## Purpose
Verify that PrepareLonghornDisksStep properly handles and reports errors when the mount command fails after successful formatting.

## Test Scenario
- **Configuration Mode**: New Disk Mounting (single unformatted disk)
- **Failure Point**: mount command fails
- **Expected Behavior**: Fail with clear error message after successful format
- **Test Type**: Failure (expected error condition)

## Configuration
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/sdb"
ENABLED_STEPS: "PrepareLonghornDisksStep"
FIRST_NODE: true
```

## Expected Behavior

### 1. Mount Preparation
- Check existing mounts: No existing mounts
- Read /etc/fstab: Success
- Determine mount point: /mnt/disk0

### 2. Disk Processing - Successful Steps
- Check filesystem: `lsblk -f /dev/sdb` → No filesystem
- Check partitions: `lsblk -no PARTTYPE /dev/sdb` → No partitions
- Format disk: `mkfs.ext4 -F -F /dev/sdb` → **Success**
- Get UUID: `blkid -s UUID -o value /dev/sdb` → Returns UUID
- Create mount point: `/mnt/disk0` → Success

### 3. Disk Processing - Failure Point
- Attempt mount: `mount /dev/sdb /mnt/disk0`
- **Mount fails**: Returns error "wrong fs type, bad option, bad superblock"

### 4. Error Handling
- **FAIL** with error: "failed to mount /dev/sdb at /mnt/disk0: mount: wrong fs type..."
- No fstab modifications attempted

## Mock Requirements

```yaml
MountDrives.GetExistingMounts: ""
ReadFile./etc/fstab: "..."
MountDrives.LsblkFilesystem./dev/sdb: "NAME FSTYPE\nsdb"
MountDrives.LsblkParttype./dev/sdb: ""
MountDrives.BlkidUUID./dev/sdb: "f6a7b8c9-d0e1-2345-f012-3456789abcde"
MountDrives.MountDrive./dev/sdb:
  error: "mount: /mnt/disk0: wrong fs type, bad option, bad superblock"
```

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/14-mount-failure/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/14-mount-failure/mocks.yaml
```

## Expected Output

```
🚀 Starting installation with 1 steps
════════════════════════════════════════

[1/1] Prepare Longhorn Disks
      Mount selected disks or populate disk map from CLUSTER_PREMOUNTED_DISKS configuration
      ❌ FAILED: error mounting disks: failed to mount /dev/sdb at /mnt/disk0: mount: /mnt/disk0: wrong fs type, bad option, bad superblock
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
Execution failed: error mounting disks: failed to mount /dev/sdb at /mnt/disk0: mount: /mnt/disk0: wrong fs type, bad option, bad superblock
```

**Key verification points:**
- Format completed successfully
- UUID retrieved successfully
- Mount point directory created
- **Mount attempted**
- **Mount failed with mock error**
- No fstab operations (failure before persist step)
- Clear error message returned

## Success Criteria

- ✅ Step executes (not skipped)
- ✅ Existing mounts checked
- ✅ Fstab read successfully
- ✅ Filesystem check performed
- ✅ Partition check performed
- ✅ Format succeeded
- ✅ UUID retrieved successfully
- ✅ Mount point directory created
- ✅ **Mount attempted**
- ✅ **Mount failed with error from mock**
- ✅ **Error propagated correctly**
- ✅ No fstab backup logged
- ✅ No fstab append logged
- ✅ Step fails with expected error
- ✅ Error message includes disk, mount point, and reason

## Related Code

- MountDrives function: `pkg/disks.go:247-340`
- Mount operation: `pkg/disks.go:327`
  ```go
  if err := command.SimpleRun("MountDrives.MountDrive."+drive, false, "mount", drive, mountPoint); err != nil {
      return mountedMap, fmt.Errorf("failed to mount %s at %s: %w", drive, mountPoint, err)
  }
  ```

## Notes

- **Partial Success**: Format succeeds but mount fails
- **Error Propagation**: Error from mount is wrapped with context (disk + mount point)
- **Early Exit**: Failure prevents fstab persistence
- **Real-world Scenario**: Filesystem corruption, kernel module issues, or mount point in use
- **Dry-run Behavior**: Mock error simulates mount failure without system changes
- **User Guidance**: Error message provides disk, mount point, and system error
