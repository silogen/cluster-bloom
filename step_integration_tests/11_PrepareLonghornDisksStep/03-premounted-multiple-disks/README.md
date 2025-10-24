# PrepareLonghornDisksStep - Test 03: Pre-mounted Disks - Multiple Disks

## Purpose
Verify that PrepareLonghornDisksStep correctly handles multiple pre-mounted disks from a comma-separated list, including proper whitespace trimming.

## Test Scenario
- **Configuration Mode**: Pre-mounted Mode (multiple disks)
- **Expected Behavior**: Parse comma-separated list, trim whitespace, create disk map with sequential indices
- **Test Type**: Success (multiple pre-mounted disk handling)

## Configuration
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_PREMOUNTED_DISKS: "/mnt/storage1, /mnt/storage2, /mnt/nvme0"
ENABLED_STEPS: "PrepareLonghornDisksStep"
FIRST_NODE: true
```

## Expected Behavior

1. **Disk Map Creation**:
   - Parse `CLUSTER_PREMOUNTED_DISKS` value (comma-separated)
   - Split on commas: `["/mnt/storage1", " /mnt/storage2", " /mnt/nvme0"]`
   - Trim whitespace from each entry
   - Create mountedDiskMap with indices:
     - `{"/mnt/storage1": "0", "/mnt/storage2": "1", "/mnt/nvme0": "2"}`
   - Store in viper as `mounted_disk_map`

2. **No Mounting Operations**:
   - Skip `MountDrives()` function
   - Skip `PersistMountedDisks()` function
   - No lsblk, mkfs, mount, or fstab operations

3. **Backup Check**:
   - Check each mount point for `longhorn-disk.cfg`
   - Check each mount point for `replicas/` directory
   - In dry-run container environment, files won't exist

4. **Log Messages**:
   - "CLUSTER_PREMOUNTED_DISKS is set, populating mounted disk map from mount points"
   - "/mnt/storage1" (first mount point)
   - "/mnt/storage2" (second mount point)
   - "/mnt/nvme0" (third mount point)
   - Disk map: `map[/mnt/nvme0:2 /mnt/storage1:0 /mnt/storage2:1]` (order may vary in map)
   - "Used 3 disks: map[...]"

5. **Result**: Step completes successfully

## Mock Requirements
**None** - Pre-mounted mode doesn't execute commands.

**Note on File System Operations**:
- The step checks for existing `longhorn-disk.cfg` and `replicas/` on each mount point
- In dry-run mode in a container, these files don't exist, so no backups are performed
- If backups were needed, `fsops.Rename()` would log `[DRY-RUN] RENAME` operations

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/03-premounted-multiple/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/03-premounted-multiple/mocks.yaml
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

3. All mount points logged (in order):
   ```
   /mnt/storage1
   /mnt/storage2
   /mnt/nvme0
   ```

4. Disk map created with all three disks:
   ```
   map[/mnt/nvme0:2 /mnt/storage1:0 /mnt/storage2:1]
   Used 3 disks: map[/mnt/nvme0:2 /mnt/storage1:0 /mnt/storage2:1]
   ```
   (Note: Go maps don't guarantee order in string representation)

5. No mounting commands executed

6. Step completes successfully

## Success Criteria

- ✅ Step executes (not skipped)
- ✅ Pre-mounted mode detected
- ✅ Comma-separated list parsed correctly
- ✅ Whitespace trimmed from each entry
- ✅ Disk map created with 3 entries and sequential indices (0, 1, 2)
- ✅ All three mount points logged individually
- ✅ No mount/format commands executed
- ✅ Disk map stored in viper for later steps
- ✅ Step completes successfully
- ✅ No errors logged

## Related Code

- Step definition: `pkg/steps.go:285` (PrepareLonghornDisksStep)
- Pre-mounted logic: `pkg/steps.go:298-312`
- String split and trim: `pkg/steps.go:304-310`
- Backup logic: `pkg/steps.go:339-363`

## Notes

- **Whitespace Handling**: The configuration intentionally includes spaces after commas to test trimming
- **Map Order**: Go maps are unordered, so log output may show entries in different order
- **Index Assignment**: Indices are assigned sequentially (0, 1, 2) based on array order after split
- **Multiple Checks**: Each mount point is checked independently for Longhorn files
