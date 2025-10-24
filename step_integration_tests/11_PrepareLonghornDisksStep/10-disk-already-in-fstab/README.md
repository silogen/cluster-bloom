# PrepareLonghornDisksStep - Test 10: Disk Already in Fstab

## Purpose
Verify that PrepareLonghornDisksStep correctly detects and reports an error when a disk's UUID already exists in /etc/fstab, preventing duplicate mounts.

## Test Scenario
- **Configuration Mode**: New Disk Mounting (single formatted disk)
- **Special Condition**: Disk UUID already exists in /etc/fstab
- **Expected Behavior**: Fail with error indicating disk is already in fstab
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
- Check existing mounts: `mount | awk '/\/mnt\/disk[0-9]+/ {print $3}'`
- Result: No existing mounts found
- Read /etc/fstab to check for existing entries
- Determine mount point: `/mnt/disk0`

### 2. Disk Processing
- Check filesystem: `lsblk -f /dev/sdb` → ext4 already present
- Skip formatting (disk already has ext4)
- Get UUID: `blkid -s UUID -o value /dev/sdb` → Returns UUID e5f6a7b8...

### 3. Fstab Check
- Read /etc/fstab content
- Find that UUID=e5f6a7b8-c9d0-1234-ef01-23456789abcd already exists
- **FAIL** with error: "disk /dev/sdb is already in /etc/fstab - please remove it first"

### 4. No Mount Operations
- No mkdir operation (failed before mount)
- No mount operation (failed before mount)
- No fstab modifications (detected duplicate)

## Mock Requirements

The key mock for this test is the fstab content that contains the duplicate UUID:

```yaml
MountDrives.GetExistingMounts: ""
ReadFile./etc/fstab: |
  # /etc/fstab: static file system information.
  UUID=1234-5678 /boot/efi vfat defaults 0 1
  UUID=e5f6a7b8-c9d0-1234-ef01-23456789abcd / ext4 defaults 0 1
  UUID=e5f6a7b8-c9d0-1234-ef01-23456789abcd /mnt/disk0 ext4 defaults,nofail 0 2 # managed by cluster-bloom
MountDrives.LsblkFilesystem./dev/sdb: "NAME FSTYPE\nsdb  ext4"
MountDrives.BlkidUUID./dev/sdb: "e5f6a7b8-c9d0-1234-ef01-23456789abcd"
```

**Note**: The fstab mock shows the UUID appearing twice - once for root filesystem and once for a bloom-managed mount point.

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/10-disk-already-in-fstab/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/10-disk-already-in-fstab/mocks.yaml
```

## Expected Output

```
🚀 Starting installation with 1 steps
════════════════════════════════════════

[1/1] Prepare Longhorn Disks
      Mount selected disks or populate disk map from CLUSTER_PREMOUNTED_DISKS configuration
      ❌ FAILED: error mounting disks: disk /dev/sdb is already in /etc/fstab - please remove it first
```

## Verification

Check `bloom.log` for the duplicate detection:

```
[DRY-RUN] MountDrives.GetExistingMounts: sh -c mount | awk '/\/mnt\/disk[0-9]+/ {print $3}'
[DRY-RUN] READ_FILE (mocked): /etc/fstab (XXX bytes)
[DRY-RUN] MountDrives.LsblkFilesystem./dev/sdb: lsblk -f /dev/sdb
Disk /dev/sdb is already formatted as ext4. Skipping format.
[DRY-RUN] MountDrives.BlkidUUID./dev/sdb: blkid -s UUID -o value /dev/sdb
Execution failed: error mounting disks: disk /dev/sdb is already in /etc/fstab - please remove it first
```

**Key verification points:**
- Fstab read successfully from mock
- Filesystem check performed
- UUID retrieved
- **Duplicate UUID detected in fstab**
- **Error logged before any mount operations**
- No mkdir, mount, or fstab append operations

## Success Criteria

- ✅ Step executes (not skipped)
- ✅ Existing mounts checked
- ✅ **Fstab content read from mock**
- ✅ Filesystem check performed (ext4 found)
- ✅ Format skipped (already ext4)
- ✅ UUID retrieved successfully
- ✅ **Duplicate UUID detected in fstab**
- ✅ **Error thrown: "disk /dev/sdb is already in /etc/fstab"**
- ✅ No mkdir operation logged
- ✅ No mount operation logged
- ✅ No fstab append operation logged
- ✅ Step fails with expected error
- ✅ Error message is clear and actionable

## Related Code

- Step definition: `pkg/steps.go:285` (PrepareLonghornDisksStep)
- MountDrives function: `pkg/disks.go:247-340`
- Fstab read (now using fsops): `pkg/disks.go:282`
  ```go
  fstabContent, err := fsops.ReadFile("/etc/fstab")
  if err != nil {
      return nil, fmt.Errorf("failed to read /etc/fstab: %w", err)
  }
  ```
- Mount point collision check: `pkg/disks.go:319-323`
  ```go
  mountPoint := fmt.Sprintf("/mnt/disk%d", i)
  for usedMountPoints[mountPoint] || strings.Contains(string(fstabContent), mountPoint) {
      i++
      mountPoint = fmt.Sprintf("/mnt/disk%d", i)
  }
  ```
- Duplicate UUID check: Looking for where this check happens in the code
  - **Note**: Need to verify this check exists or if test will reveal missing validation

## Notes

- **Idempotency**: This test verifies that cluster-bloom won't create duplicate fstab entries
- **Safety**: Prevents mounting the same disk twice at different mount points
- **fsops.ReadFile**: This test validates the new `fsops.ReadFile` function with mock support
- **Mock Pattern**: Uses `ReadFile./etc/fstab` as the mock key
- **Real-world Scenario**: Simulates running cluster-bloom on a node that already has the disk configured
- **Error Message**: Should guide user to manually remove the existing fstab entry
- **Comparison to Test 06**: Test 06 handles already-formatted disk successfully; this test handles already-mounted disk (in fstab) as an error

## Code Validation

The duplicate UUID check exists in `pkg/disks.go:316-317`:

```go
uuid := strings.TrimSpace(string(uuidOutput))
if uuid != "" && strings.Contains(string(fstabContent), fmt.Sprintf("UUID=%s", uuid)) {
    return mountedMap, fmt.Errorf("disk %s is already in /etc/fstab - please remove it first", drive)
}
```

This validation:
- Retrieves the UUID from the disk using blkid
- Checks if that UUID already exists anywhere in /etc/fstab
- Returns an error before any mount operations if found
- Provides clear guidance to the user to manually remove the existing entry
