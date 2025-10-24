# PrepareLonghornDisksStep - Test 12: Backup Existing Replicas Directory

## Purpose
Verify that PrepareLonghornDisksStep correctly detects and backs up an existing Longhorn replicas directory before continuing with disk setup.

## Test Scenario
- **Configuration Mode**: Pre-mounted Disks (single mount point)
- **Special Condition**: replicas directory exists at mount point
- **Expected Behavior**: Detect directory, rename to backup with timestamp, log operation
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
- **File does not exist** (via mock error)
- Skip config backup

### 3. Check for Replicas Directory
- Check for replicas: `fsops.Stat(/mnt/storage1/replicas)`
- **Directory exists** (via mock)
- Verify it's a directory: `info.IsDir()` returns true

### 4. Backup Replicas Directory
- Create backup path: `/mnt/storage1/replicas.backup-<timestamp>`
- Log: "Found replicas directory at /mnt/storage1/replicas, backing up to /mnt/storage1/replicas.backup-<timestamp>"
- Rename directory: `fsops.Rename(replicasPath, backupPath)`
- Log: "Backed up and removed replicas directory"

### 5. Complete Successfully
- Step completes with no errors
- Backup operation logged

## Mock Requirements

```yaml
# Simulate that longhorn-disk.cfg does not exist
Stat./mnt/storage1/longhorn-disk.cfg:
  output: ""
  error: "stat /mnt/storage1/longhorn-disk.cfg: no such file or directory"

# Simulate that replicas directory exists
Stat./mnt/storage1/replicas:
  output: "dir"
  error: null
```

**Mock Pattern**: `Stat.<full-path>`
- Returns `"file"` for regular files
- Returns `"dir"` for directories (triggers `IsDir()` check)
- Returns error for non-existent files

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/12-backup-replicas-directory/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/12-backup-replicas-directory/mocks.yaml
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
Found replicas directory at /mnt/storage1/replicas, backing up to /mnt/storage1/replicas.backup-20250123-140530
[DRY-RUN] RENAME: /mnt/storage1/replicas -> /mnt/storage1/replicas.backup-20250123-140530
Backed up and removed replicas directory
Completed in <time>
```

**Key verification points:**
- Disk map populated from CLUSTER_PREMOUNTED_DISKS
- Stat operation checked for longhorn-disk.cfg (mocked as not exists)
- No config backup attempted
- Stat operation checked for replicas directory (mocked as exists + isDir)
- Backup path created with timestamp
- Rename operation logged (dry-run)
- Success message logged

## Success Criteria

- ✅ Step executes (not skipped)
- ✅ CLUSTER_PREMOUNTED_DISKS parsed correctly
- ✅ Disk map populated: `map[/mnt/storage1:0]`
- ✅ **fsops.Stat called for longhorn-disk.cfg**
- ✅ **Mock returned error (not exists)**
- ✅ No config backup logged
- ✅ **fsops.Stat called for replicas directory**
- ✅ **Mock returned "dir" (directory exists)**
- ✅ **IsDir() check passed**
- ✅ Backup path generated with timestamp
- ✅ **Rename operation logged**: replicas -> backup
- ✅ Success message: "Backed up and removed replicas directory"
- ✅ Step completes successfully
- ✅ No errors logged

## Related Code

- Step definition: `pkg/steps.go:285` (PrepareLonghornDisksStep)
- Backup logic: `pkg/steps.go:339-363`
- Replicas directory check: `pkg/steps.go:354-362`
  ```go
  replicasPath := filepath.Join(mountPoint, "replicas")
  if info, err := fsops.Stat(replicasPath); err == nil && info.IsDir() {
      backupPath := filepath.Join(mountPoint, fmt.Sprintf("replicas.backup-%s", timestamp))
      LogMessage(Info, fmt.Sprintf("Found replicas directory at %s, backing up to %s", replicasPath, backupPath))
      if err := fsops.Rename(replicasPath, backupPath); err != nil {
          LogMessage(Warn, fmt.Sprintf("Failed to backup replicas directory: %v", err))
      } else {
          LogMessage(Info, fmt.Sprintf("Backed up and removed replicas directory"))
      }
  }
  ```
- fsops.Stat implementation: `pkg/fsops/fsops.go:235-259`
  - Returns `mockFileInfo` with `isDir: true` when mock output is "dir"
- mockFileInfo.IsDir(): `pkg/fsops/fsops.go:271`
  ```go
  func (m *mockFileInfo) IsDir() bool { return m.isDir }
  ```
- fsops.Rename implementation: `pkg/fsops/fsops.go:89-95`

## Notes

- **Timestamp Format**: `20060102-150405` (YYYYMMDD-HHMMSS)
- **Backup Path**: Original directory is renamed, not copied
- **fsops.Stat Mocking**:
  - Mock output "dir" creates mockFileInfo with isDir=true
  - IsDir() check in code properly validates it's a directory
- **fsops.Rename**: Already supports dry-run logging for directories
- **Non-destructive**: In dry-run mode, rename is logged but not executed
- **Real-world Scenario**: Simulates re-running cluster-bloom on a node with existing Longhorn replicas
- **Comparison to Test 11**: Test 11 backs up config file; this test backs up replicas directory
- **Data Preservation**: Backup ensures we don't lose existing Longhorn replica data

## mockFileInfo IsDir Implementation

The mock returns different FileInfo based on output:
- `output: "file"` → `mockFileInfo{name: name, isDir: false}`
- `output: "dir"` → `mockFileInfo{name: name, isDir: true}`

This allows the code to properly distinguish between files and directories:
```go
if info, err := fsops.Stat(replicasPath); err == nil && info.IsDir() {
    // This block only executes if mock output was "dir"
}
```
