# PrepareLonghornDisksStep - Test 09: Mount Point Collision

## Purpose
Verify that PrepareLonghornDisksStep correctly handles the case when the default mount point /mnt/disk0 is already in use, by incrementing to /mnt/disk1.

## Test Scenario
- **Configuration Mode**: New Disk Mounting (single unformatted disk)
- **Special Condition**: /mnt/disk0 already exists and is in use
- **Expected Behavior**: Mount disk at /mnt/disk1 instead of /mnt/disk0
- **Test Type**: Success (mount point collision handled gracefully)

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
- Result: `/mnt/disk0` is already in use
- Determine next available mount point: `/mnt/disk1`

### 2. Disk Processing
- Check filesystem: `lsblk -f /dev/sdb` → No filesystem
- Check partitions: `lsblk -no PARTTYPE /dev/sdb` → No partitions
- Format: `mkfs.ext4 -F -F /dev/sdb`
- Get UUID: `blkid -s UUID -o value /dev/sdb` → Returns UUID d4e5f6a7...

### 3. Mounting
- Skip /mnt/disk0 (already in use)
- Create mount point: `/mnt/disk1`
- Mount: `mount /dev/sdb /mnt/disk1`
- Log: "Mounted /dev/sdb at /mnt/disk1"

### 4. Persist to fstab
- Backup fstab: `sudo cp /etc/fstab /etc/fstab.bak`
- Get UUID: `blkid -s UUID -o value <device>`
- Add fstab entry for /mnt/disk1 with bloom tag
- Verify: `sudo mount -a`

### 5. Final Output
- Log: "Used 1 disks: map[/mnt/disk1:/dev/sdb-d4e5f6a7...]"
- Store disk map in viper for subsequent steps

## Mock Requirements

```yaml
MountDrives.GetExistingMounts: "/mnt/disk0"
MountDrives.LsblkFilesystem./dev/sdb: "NAME FSTYPE\nsdb"
MountDrives.LsblkParttype./dev/sdb: ""
MountDrives.BlkidUUID./dev/sdb: "d4e5f6a7-b8c9-0123-def0-123456789abc"
PersistMountedDisks.BackupFstab: ""
PersistMountedDisks.BlkidUUID./dev/sdb-d4e5f6a7...: "d4e5f6a7-b8c9-0123-def0-123456789abc"
PersistMountedDisks.MountAll: ""
```

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/09-mount-point-collision/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/09-mount-point-collision/mocks.yaml
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

Check `bloom.log` for the mount point collision handling:

```
[DRY-RUN] MountDrives.GetExistingMounts: sh -c mount | awk '/\/mnt\/disk[0-9]+/ {print $3}'
[DRY-RUN] MountDrives.LsblkFilesystem./dev/sdb: lsblk -f /dev/sdb
[DRY-RUN] MountDrives.LsblkParttype./dev/sdb: lsblk -no PARTTYPE /dev/sdb
Disk /dev/sdb is not partitioned. Formatting with ext4...
[DRY-RUN] MountDrives.MkfsExt4./dev/sdb: mkfs.ext4 -F -F /dev/sdb
[DRY-RUN] MountDrives.BlkidUUID./dev/sdb: blkid -s UUID -o value /dev/sdb
[DRY-RUN] MKDIR_ALL: /mnt/disk1 (perm: 755)
[DRY-RUN] MountDrives.MountDrive./dev/sdb: mount /dev/sdb /mnt/disk1
Mounted /dev/sdb at /mnt/disk1
[DRY-RUN] PersistMountedDisks.BackupFstab: sudo cp /etc/fstab /etc/fstab.bak
[DRY-RUN] PersistMountedDisks.BlkidUUID./dev/sdb-d4e5f6a7-b8c9-0123-def0-123456789abc: blkid -s UUID -o value /dev/sdb-d4e5f6a7-b8c9-0123-def0-123456789abc
[DRY-RUN] CREATE_CMD: sudo tee -a /etc/fstab
[DRY-RUN] PersistMountedDisks.MountAll: sudo mount -a
Used 1 disks: map[/mnt/disk1:/dev/sdb-d4e5f6a7-b8c9-0123-def0-123456789abc]
Completed in <time>
```

**Key verification points:**
- Mount point created at `/mnt/disk1` (NOT /mnt/disk0)
- Disk successfully mounted despite collision
- Fstab entry uses /mnt/disk1

## Success Criteria

- ✅ Step executes (not skipped)
- ✅ Existing mounts checked - finds /mnt/disk0 in use
- ✅ **Mount point collision detected**
- ✅ **Next mount point /mnt/disk1 used instead**
- ✅ Filesystem check performed
- ✅ Partition check performed
- ✅ Disk formatted with ext4
- ✅ UUID retrieved
- ✅ **mkdir for /mnt/disk1 (not /mnt/disk0)**
- ✅ **Mount operation at /mnt/disk1**
- ✅ Fstab backup created
- ✅ **Fstab entry added for /mnt/disk1**
- ✅ Mount verification: `mount -a` executed
- ✅ Disk map contains /mnt/disk1
- ✅ Step completes successfully
- ✅ No errors logged

## Related Code

- Step definition: `pkg/steps.go:285` (PrepareLonghornDisksStep)
- Mount point collision logic: `pkg/disks.go:319-323`
  ```go
  mountPoint := fmt.Sprintf("/mnt/disk%d", i)
  for usedMountPoints[mountPoint] || strings.Contains(string(fstabContent), mountPoint) {
      i++
      mountPoint = fmt.Sprintf("/mnt/disk%d", i)
  }
  ```
- GetExistingMounts: `pkg/disks.go:247-254`
  ```go
  output, err := command.Output("MountDrives.GetExistingMounts", true, "sh", "-c",
      "mount | awk '/\\/mnt\\/disk[0-9]+/ {print $3}'")
  ```

## Notes

- **Collision Detection**: The code checks both the usedMountPoints map and the fstab content to avoid collisions
- **Counter Increment**: The counter `i` starts at 0 and increments until a free mount point is found
- **Multiple Collisions**: The while loop handles multiple consecutive collisions (e.g., if both /mnt/disk0 and /mnt/disk1 exist)
- **Dry-run Behavior**: All disk operations (mkfs, mount) are logged but not executed
- **Real-world Scenario**: This test simulates a node that already has a disk mounted, ensuring new disks don't conflict
- **Comparison to Test 05**: Test 05 uses /mnt/disk0; this test uses /mnt/disk1 due to collision
