# PrepareLonghornDisksStep - Test 11: Backup Existing Longhorn Config

## Purpose
Verify that PrepareLonghornDisksStep correctly detects and backs up an existing longhorn-disk.cfg file before continuing with disk setup.

## Test Scenario
- **Configuration Mode**: Pre-mounted Disks (single mount point)
- **Special Condition**: longhorn-disk.cfg file exists at mount point
- **Expected Behavior**: Detect file, rename to backup with timestamp, log operation
- **Test Type**: Success (backup operation completes successfully)

## Configuration
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_PREMOUNTED_DISKS: "/mnt/storage1"
ENABLED_STEPS: "PrepareLonghornDisksStep"
FIRST_NODE: true
```

## Expected Behavior

### 1. Populate Disk Map from Pre-mounted Disks
- Parse CLUSTER_PREMOUNTED_DISKS: "/mnt/storage1"
- Create disk map: `{"/mnt/storage1": "0"}`
- Log: "Used 1 disks: map[/mnt/storage1:0]"

### 2. Check for Existing Longhorn Files
- Generate timestamp for backup names
- Check for longhorn-disk.cfg: `fsops.Stat(/mnt/storage1/longhorn-disk.cfg)`
- **File exists** (via mock)

### 3. Backup longhorn-disk.cfg
- Create backup path: `/mnt/storage1/longhorn-disk.cfg.backup-<timestamp>`
- Log: "Found longhorn-disk.cfg at /mnt/storage1/longhorn-disk.cfg, backing up to /mnt/storage1/longhorn-disk.cfg.backup-<timestamp>"
- Rename file: `fsops.Rename(longhornConfigPath, backupPath)`
- Log: "Backed up and removed longhorn-disk.cfg"

### 4. Check for Replicas Directory
- Check for replicas: `fsops.Stat(/mnt/storage1/replicas)`
- **Directory does not exist** (via mock error)
- Skip replicas backup

### 5. Complete Successfully
- Step completes with no errors
- Backup operation logged

## Mock Requirements

```yaml
# Simulate that longhorn-disk.cfg exists
Stat./mnt/storage1/longhorn-disk.cfg:
  output: "file"
  error: null

# Simulate that replicas directory does not exist
Stat./mnt/storage1/replicas:
  output: ""
  error: "stat /mnt/storage1/replicas: no such file or directory"
```

**Mock Pattern**: `Stat.<full-path-to-file>`
- Returns `"file"` for regular files
- Returns `"dir"` for directories
- Returns error for non-existent files

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/11-backup-longhorn-config/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/11-backup-longhorn-config/mocks.yaml
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

Check `bloom.log` for the backup operations:

```
CLUSTER_PREMOUNTED_DISKS is set, populating mounted disk map from mount points
/mnt/storage1
map[/mnt/storage1:0]
Used 1 disks: map[/mnt/storage1:0]
Found longhorn-disk.cfg at /mnt/storage1/longhorn-disk.cfg, backing up to /mnt/storage1/longhorn-disk.cfg.backup-20250123-140530
[DRY-RUN] RENAME: /mnt/storage1/longhorn-disk.cfg -> /mnt/storage1/longhorn-disk.cfg.backup-20250123-140530
Backed up and removed longhorn-disk.cfg
Completed in <time>
```

**Key verification points:**
- Disk map populated from CLUSTER_PREMOUNTED_DISKS
- Stat operation checked for longhorn-disk.cfg (mocked as exists)
- Backup path created with timestamp
- Rename operation logged (dry-run)
- Success message logged
- Stat operation checked for replicas directory (mocked as not exists)
- No replicas backup attempted

## Success Criteria

- ✅ Step executes (not skipped)
- ✅ CLUSTER_PREMOUNTED_DISKS parsed correctly
- ✅ Disk map populated: `map[/mnt/storage1:0]`
- ✅ **fsops.Stat called for longhorn-disk.cfg**
- ✅ **Mock returned "file exists"**
- ✅ Backup path generated with timestamp
- ✅ **Rename operation logged**: config -> backup
- ✅ Success message: "Backed up and removed longhorn-disk.cfg"
- ✅ **fsops.Stat called for replicas directory**
- ✅ **Mock returned error (not exists)**
- ✅ No replicas backup logged
- ✅ Step completes successfully
- ✅ No errors logged

## Related Code

- Step definition: `pkg/steps.go:285` (PrepareLonghornDisksStep)
- Backup logic: `pkg/steps.go:339-363`
- Config file check: `pkg/steps.go:343`
  ```go
  if _, err := fsops.Stat(longhornConfigPath); err == nil {
      backupPath := filepath.Join(mountPoint, fmt.Sprintf("longhorn-disk.cfg.backup-%s", timestamp))
      LogMessage(Info, fmt.Sprintf("Found longhorn-disk.cfg at %s, backing up to %s", longhornConfigPath, backupPath))
      if err := fsops.Rename(longhornConfigPath, backupPath); err != nil {
          LogMessage(Warn, fmt.Sprintf("Failed to backup longhorn-disk.cfg: %v", err))
      } else {
          LogMessage(Info, fmt.Sprintf("Backed up and removed longhorn-disk.cfg"))
      }
  }
  ```
- Replicas directory check: `pkg/steps.go:354`
  ```go
  if info, err := fsops.Stat(replicasPath); err == nil && info.IsDir() {
      // backup replicas directory
  }
  ```
- fsops.Stat implementation: `pkg/fsops/fsops.go:235-259`
- fsops.Rename implementation: `pkg/fsops/fsops.go:89-95` (already supports dry-run)

## Notes

- **Timestamp Format**: `20060102-150405` (YYYYMMDD-HHMMSS)
- **Backup Path**: Original file is renamed, not copied
- **fsops.Stat Mocking**: New feature implemented for this test
  - Returns mock FileInfo when mock value is "file" or "dir"
  - Returns os.ErrNotExist when mock has error
  - Falls back to actual os.Stat if no mock provided
- **fsops.Rename**: Already supports dry-run logging
- **Non-destructive**: In dry-run mode, rename is logged but not executed
- **Real-world Scenario**: Simulates re-running cluster-bloom on a node with existing Longhorn installation
- **Comparison to Test 02**: Test 02 uses pre-mounted disk without backup; this test includes backup scenario
- **Idempotency**: Backup ensures we don't lose existing Longhorn data

## Mock FileInfo Implementation

The `fsops.Stat` function uses a simple mock FileInfo structure:

```go
type mockFileInfo struct {
    name  string
    isDir bool
}
```

This implements the `fs.FileInfo` interface with sensible defaults:
- `Name()`: Returns the file name
- `Size()`: Returns 0 (not needed for existence checks)
- `Mode()`: Returns 0644 (standard file permissions)
- `ModTime()`: Returns current time
- `IsDir()`: Returns true for "dir" mock, false for "file" mock
- `Sys()`: Returns nil
