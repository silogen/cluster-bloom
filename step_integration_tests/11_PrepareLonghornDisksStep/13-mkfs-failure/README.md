# PrepareLonghornDisksStep - Test 13: Mount Failure - mkfs.ext4 Fails

## Purpose
Verify that PrepareLonghornDisksStep properly handles and reports errors when the mkfs.ext4 command fails during disk formatting.

## Test Scenario
- **Configuration Mode**: New Disk Mounting (single unformatted disk)
- **Failure Point**: mkfs.ext4 command fails
- **Expected Behavior**: Fail with clear error message, no mount attempted
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

### 2. Disk Processing
- Check filesystem: `lsblk -f /dev/sdb` → No filesystem
- Check partitions: `lsblk -no PARTTYPE /dev/sdb` → No partitions
- Attempt format: `mkfs.ext4 -F -F /dev/sdb`
- **Format fails**: Returns error "Device or resource busy"

### 3. Error Handling
- **FAIL** with error: "failed to format /dev/sdb: mkfs.ext4: Device or resource busy"
- No UUID retrieval attempted
- No mount point created
- No mount operation attempted
- No fstab modifications

## Mock Requirements

```yaml
MountDrives.GetExistingMounts: ""
ReadFile./etc/fstab: "..."
MountDrives.LsblkFilesystem./dev/sdb: "NAME FSTYPE\nsdb"
MountDrives.LsblkParttype./dev/sdb: ""
MountDrives.MkfsExt4./dev/sdb:
  error: "mkfs.ext4: Device or resource busy"
```

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/13-mkfs-failure/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/13-mkfs-failure/mocks.yaml
```

## Expected Output

```
🚀 Starting installation with 1 steps
════════════════════════════════════════

[1/1] Prepare Longhorn Disks
      Mount selected disks or populate disk map from CLUSTER_PREMOUNTED_DISKS configuration
      ❌ FAILED: error mounting disks: failed to format /dev/sdb: mkfs.ext4: Device or resource busy
```

## Verification

Check `bloom.log` for the failure sequence:

```
[DRY-RUN] MountDrives.GetExistingMounts: sh -c mount | awk '/\/mnt\/disk[0-9]+/ {print $3}'
[DRY-RUN] MountDrives.LsblkFilesystem./dev/sdb: lsblk -f /dev/sdb
[DRY-RUN] MountDrives.LsblkParttype./dev/sdb: lsblk -no PARTTYPE /dev/sdb
Disk /dev/sdb is not partitioned. Formatting with ext4...
[DRY-RUN] MountDrives.MkfsExt4./dev/sdb: mkfs.ext4 -F -F /dev/sdb
Execution failed: error mounting disks: failed to format /dev/sdb: mkfs.ext4: Device or resource busy
```

**Key verification points:**
- Filesystem and partition checks completed
- Format attempted
- **Format failed with mock error**
- No UUID retrieval logged
- No mkdir operation
- No mount operation
- No fstab operations
- Clear error message returned

## Success Criteria

- ✅ Step executes (not skipped)
- ✅ Existing mounts checked
- ✅ Fstab read successfully
- ✅ Filesystem check performed
- ✅ Partition check performed
- ✅ **Format attempted**
- ✅ **Format failed with error from mock**
- ✅ **Error propagated correctly**
- ✅ No UUID retrieval attempted
- ✅ No mkdir logged
- ✅ No mount logged
- ✅ No fstab backup logged
- ✅ No fstab append logged
- ✅ Step fails with expected error
- ✅ Error message is clear and actionable

## Related Code

- MountDrives function: `pkg/disks.go:247-340`
- Format operation: `pkg/disks.go:308`
  ```go
  if err := command.SimpleRun("MountDrives.MkfsExt4."+drive, false, "mkfs.ext4", "-F", "-F", drive); err != nil {
      return mountedMap, fmt.Errorf("failed to format %s: %w", drive, err)
  }
  ```

## Notes

- **Error Propagation**: Error from mkfs.ext4 is wrapped with context
- **Early Exit**: Failure prevents any subsequent mount operations
- **Real-world Scenario**: Disk is locked by another process or has hardware issues
- **Dry-run Behavior**: Mock error simulates the failure without needing actual disk issues
- **User Guidance**: Error message indicates which disk failed and why
