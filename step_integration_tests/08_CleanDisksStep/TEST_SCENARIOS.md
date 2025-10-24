# CleanDisksStep Integration Tests

## Purpose
Remove any previous Longhorn temporary drives and clean disk state by unmounting bloom-tagged fstab entries, cleaning CSI mounts, wiping filesystems, and removing stale devices.

## Step Overview
- **Execution Order**: Step 8
- **Commands Executed**:
  - Calls `UnmountPriorLonghornDisks()`:
    - Reads `/etc/fstab`
    - `sudo cp /etc/fstab /etc/fstab.bak.<timestamp>`
    - `sudo umount <mount_point>` for each bloom-tagged entry
    - Writes updated `/etc/fstab` without bloom entries
  - `mount` - list current mounts
  - `sudo umount -lf <mount_point>` - unmount longhorn CSI mounts
  - `lsblk -o NAME,MOUNTPOINT` - list block devices
  - `sudo wipefs -a /dev/<device>` - wipe filesystem signatures
  - `sudo rm -rf /var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/*`
  - `lsblk -nd -o NAME,TYPE,MOUNTPOINT` - list disks
  - `sudo tee /sys/block/<device>/device/delete` - delete unmounted disks
- **Skip Conditions**: Never skipped

## What This Step Does
1. Unmounts all bloom-tagged fstab entries (entries with `# managed by cluster-bloom`)
2. Backs up and updates /etc/fstab to remove bloom entries
3. Identifies and unmounts longhorn CSI mounts from kubelet
4. Wipes filesystem signatures from disks
5. Removes CSI driver plugin directories
6. Deletes unmounted disk devices from the system

## Test Scenarios

### Success Scenarios

#### 1. Clean System - No Bloom Entries
Tests successful execution when no bloom-tagged entries exist in fstab.

#### 2. Bloom Entries in Fstab - Successfully Unmounted
Tests successful unmounting and removal of bloom-tagged fstab entries.

#### 3. Multiple Bloom Disks with CSI Mounts
Tests cleanup of multiple disks with both fstab entries and CSI mounts.

#### 4. Longhorn CSI Mounts - Successfully Cleaned
Tests identification and cleanup of longhorn CSI kubelet mounts.

### Failure Scenarios

#### 5. Failed to Backup Fstab
Tests error handling when fstab backup fails.

#### 6. Failed to Read Fstab
Tests error handling when /etc/fstab cannot be read.

#### 7. Unmount Command Fails
Tests that unmount failures are logged as warnings but don't stop execution.

#### 8. Wipefs Command Fails
Tests that filesystem wipe failures are logged as warnings but don't stop execution.

## Configuration Requirements

- `ENABLED_STEPS: "CleanDisksStep"`
- No specific configuration required

## Mock Requirements

```yaml
mocks:
  # Unmount bloom-tagged entries
  "UnmountPriorLonghornDisks.BackupFstab":
    output: ""
    error: null

  "ReadFile./etc/fstab":
    output: |
      # /etc/fstab: static file system information.
      UUID=1234-5678 /boot/efi vfat defaults 0 1
      UUID=a1b2c3d4-e5f6-7890 /mnt/disk0 ext4 defaults,nofail 0 2 # managed by cluster-bloom
      UUID=b2c3d4e5-f6a7-8901 /mnt/disk1 ext4 defaults,nofail 0 2 # managed by cluster-bloom
    error: null

  "UnmountPriorLonghornDisks.UnmountPoint./mnt/disk0":
    output: ""
    error: null

  "UnmountPriorLonghornDisks.UnmountPoint./mnt/disk1":
    output: ""
    error: null

  "WriteFile./etc/fstab":
    output: ""
    error: null

  # Clean CSI mounts
  "CleanDisks.Mount":
    output: |
      /dev/sda1 on / type ext4 (rw,relatime)
      /dev/longhorn/pvc-123 on /var/lib/kubelet/pods/abc/volumes/kubernetes.io~csi/pvc-123/mount type ext4 (rw)
    error: null

  "CleanDisks.UnmountCSI":
    output: ""
    error: null

  # Wipe disks
  "CleanDisks.LsblkMountpoint":
    output: |
      NAME MOUNTPOINT
      sda
      sdb
      sdc  /mnt/disk0
    error: null

  "CleanDisks.Wipefs./dev/sdb":
    output: |
      /dev/sdb: 2 bytes were erased at offset 0x00000438 (ext4): 53 ef
    error: null

  # Remove CSI plugin directory
  "CleanDisks.RmCSIPlugin":
    output: ""
    error: null

  # Delete unmounted devices
  "CleanDisks.LsblkDisks":
    output: |
      sda disk
      sdb disk
      sr0 rom
    error: null

  "CleanDisks.DeleteDevice.sdb":
    output: ""
    error: null
```

## Running Tests

```bash
# Test 1: Clean system
./cluster-bloom cli --config step_integration_tests/08_CleanDisksStep/01-clean-system/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/08_CleanDisksStep/01-clean-system/mocks.yaml

# Test 2: Bloom entries in fstab
./cluster-bloom cli --config step_integration_tests/08_CleanDisksStep/02-bloom-fstab-entries/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/08_CleanDisksStep/02-bloom-fstab-entries/mocks.yaml

# Test 5: Fstab backup fails
./cluster-bloom cli --config step_integration_tests/08_CleanDisksStep/05-backup-fails/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/08_CleanDisksStep/05-backup-fails/mocks.yaml
```

## Expected Outcomes

### Success Cases
- ✅ Bloom-tagged fstab entries identified and unmounted
- ✅ Fstab backed up and updated
- ✅ CSI mounts cleaned up
- ✅ Filesystems wiped from disks
- ✅ CSI plugin directory removed
- ✅ Unmounted devices deleted
- ✅ Step completes successfully

### Failure Cases
- ❌ Fstab backup failure stops execution
- ❌ Fstab read failure stops execution
- ⚠️ Unmount failures logged as warnings (non-fatal)
- ⚠️ Wipefs failures logged as warnings (non-fatal)
- ⚠️ Device deletion failures logged as warnings (non-fatal)

## Related Code
- Step implementation: `pkg/steps.go:242-253`
- UnmountPriorLonghornDisks: `pkg/disks.go:34-90`
- Disk cleaning logic: `pkg/disks.go:92-169`

## Notes
- **Bloom tag identification**: Looks for `# managed by cluster-bloom` in fstab
- **Fstab backup**: Creates timestamped backup before modification
- **Partial error tolerance**: Unmount/wipe/delete errors are warnings only
- **Critical errors**: Fstab backup/read failures stop execution
- **Device deletion**: Uses `/sys/block/<device>/device/delete` to remove SCSI devices
- **CSI cleanup**: Removes longhorn CSI plugin directories
- **Idempotent**: Safe to run multiple times
- **Complements**: Works with CleanLonghornMountsStep for complete cleanup
