# PrepareLonghornDisksStep - Test 02: Pre-mounted Disks - Single Disk

## Purpose
Verify that PrepareLonghornDisksStep correctly handles a single pre-mounted disk without attempting to mount it.

## Test Scenario
- **Configuration Mode**: Pre-mounted Mode (single disk)
- **Expected Behavior**: Use existing mount point, create disk map, check for Longhorn backup files
- **Test Type**: Success (pre-mounted disk handling)

## Configuration
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_PREMOUNTED_DISKS: "/mnt/storage1"
ENABLED_STEPS: "PrepareLonghornDisksStep"
FIRST_NODE: true
```

## Expected Behavior

1. **Disk Map Creation**:
   - Parse `CLUSTER_PREMOUNTED_DISKS` value
   - Create mountedDiskMap: `{"/mnt/storage1": "0"}`
   - Store in viper as `mounted_disk_map`

2. **No Mounting Operations**:
   - Skip `MountDrives()` function
   - Skip `PersistMountedDisks()` function
   - No lsblk, mkfs, mount, or fstab operations

3. **Backup Check**:
   - Check for `/mnt/storage1/longhorn-disk.cfg` (won't exist in dry-run)
   - Check for `/mnt/storage1/replicas/` directory (won't exist in dry-run)
   - In real scenario, would backup if found

4. **Log Messages**:
   - "CLUSTER_PREMOUNTED_DISKS is set, populating mounted disk map from mount points"
   - "/mnt/storage1" (mount point logged)
   - Disk map logged: `map[/mnt/storage1:0]`
   - "Used 1 disks: map[/mnt/storage1:0]"

5. **Result**: Step completes successfully

## Mock Requirements
**None** - Pre-mounted mode doesn't execute commands.

**Note on File System Operations**:
- The step checks for existing `longhorn-disk.cfg` and `replicas/` using `os.Stat()`
- In dry-run mode in a container, these files don't exist, so no backups are performed
- If backups were needed, `fsops.Rename()` would log `[DRY-RUN] RENAME` operations
- To test the backup scenario specifically, `os.Stat()` calls would need to be moved to fsops package with mock support

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/02-premounted-single/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/02-premounted-single/mocks.yaml
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

Check `bloom.log` for:

1. Step execution starts:
   ```
   Starting step: Prepare Longhorn Disks
   ```

2. Pre-mounted disk mode detected:
   ```
   CLUSTER_PREMOUNTED_DISKS is set, populating mounted disk map from mount points
   ```

3. Mount point logged:
   ```
   /mnt/storage1
   ```

4. Disk map created and logged:
   ```
   map[/mnt/storage1:0]
   Used 1 disks: map[/mnt/storage1:0]
   ```

5. No mounting commands executed (no lsblk, mkfs, mount in logs)

6. Step completes successfully:
   ```
   Completed in <time>
   ```

## Success Criteria

- ✅ Step executes (not skipped)
- ✅ Pre-mounted mode detected
- ✅ Disk map created with correct structure: `{"/mnt/storage1": "0"}`
- ✅ Mount point logged correctly
- ✅ No mount/format commands executed
- ✅ Disk map stored in viper for later steps
- ✅ Step completes successfully
- ✅ No errors logged

## Related Code

- Step definition: `pkg/steps.go:285` (PrepareLonghornDisksStep)
- Pre-mounted logic: `pkg/steps.go:298-312`
- Backup logic: `pkg/steps.go:339-363`

## Notes

- In dry-run mode, the file system checks for `longhorn-disk.cfg` and `replicas` will not find files (they don't exist in the container environment)
- The disk map is the primary output of this test
- This mode is useful for clusters where disks are managed externally
