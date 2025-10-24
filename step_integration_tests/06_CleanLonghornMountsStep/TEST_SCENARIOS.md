# CleanLonghornMountsStep Integration Tests

## Purpose
Clean up Longhorn PVCs and mounts before RKE2 uninstall to prevent orphaned mounts and resource leaks.

## Step Overview
- **Execution Order**: Step 6
- **Commands Executed**:
  - `sudo systemctl stop longhorn-* 2>/dev/null || true`
  - `sudo umount -lf /dev/longhorn/pvc* 2>/dev/null || true` (3 iterations)
  - `sudo umount -Af /var/lib/kubelet/pods/*/volumes/kubernetes.io~csi/pvc-* 2>/dev/null || true`
  - `sudo umount -Af /var/lib/kubelet/pods/*/volumes/kubernetes.io~csi/*/mount 2>/dev/null || true`
  - `mount | grep 'driver.longhorn.io' | awk '{print $3}' | xargs -r sudo umount -lf 2>/dev/null || true`
  - `sudo umount -Af /var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/* 2>/dev/null || true`
  - `sudo fuser -km /dev/longhorn/ 2>/dev/null || true`
  - `sudo rm -rf /dev/longhorn/pvc-* 2>/dev/null || true`
  - `sudo rm -rf /var/lib/kubelet/plugins/kubernetes.io/csi/driver.longhorn.io/* 2>/dev/null || true`
- **Skip Conditions**: Never skipped

## What This Step Does
1. Stops all longhorn-related systemd services
2. Unmounts Longhorn PVC block devices (with retry loop)
3. Unmounts kubelet CSI volume mounts
4. Unmounts Longhorn CSI driver mounts
5. Kills processes using /dev/longhorn/ devices
6. Removes Longhorn PVC device files
7. Cleans up CSI plugin directories

## Test Scenarios

### Success Scenarios

#### 1. Clean System - No Longhorn Mounts
Tests successful execution when no Longhorn mounts exist (all commands succeed with `|| true`).

#### 2. Active Longhorn Mounts - Successfully Cleaned
Tests successful cleanup of active Longhorn PVCs and CSI mounts.

#### 3. Stubborn Mounts - Multiple Unmount Attempts
Tests the retry logic for unmounting /dev/longhorn/pvc* devices (3 iterations).

### Failure Scenarios

#### 4. Commands Fail But Continue
Tests that failures are non-fatal due to `|| true` suffix on all commands.

**Note**: This step has no true failure scenarios because all commands have `|| true`, making them non-fatal. Errors are logged but don't stop execution.

## Configuration Requirements

- `ENABLED_STEPS: "CleanLonghornMountsStep"`
- Minimal configuration required

## Mock Requirements

```yaml
mocks:
  # Stop longhorn services
  "ShellCmdHelper.Exec.stop_longhorn_services":
    output: ""
    error: null

  # Unmount iterations (3x)
  "ShellCmdHelper.Exec.umount_pvc_iteration_1":
    output: ""
    error: null

  "ShellCmdHelper.Exec.umount_pvc_iteration_2":
    output: ""
    error: null

  "ShellCmdHelper.Exec.umount_pvc_iteration_3":
    output: ""
    error: null

  # Unmount kubelet CSI volumes
  "ShellCmdHelper.Exec.umount_kubelet_pvc":
    output: ""
    error: null

  "ShellCmdHelper.Exec.umount_kubelet_mount":
    output: ""
    error: null

  # Unmount longhorn driver mounts
  "ShellCmdHelper.Exec.grep_and_umount_longhorn":
    output: ""
    error: null

  "ShellCmdHelper.Exec.umount_csi_plugin":
    output: ""
    error: null

  # Kill processes
  "ShellCmdHelper.Exec.fuser_kill":
    output: ""
    error: null

  # Remove device files and directories
  "ShellCmdHelper.Exec.rm_pvc_devices":
    output: ""
    error: null

  "ShellCmdHelper.Exec.rm_csi_plugin_dir":
    output: ""
    error: null
```

## Running Tests

```bash
# Test 1: Clean system
./cluster-bloom cli --config step_integration_tests/06_CleanLonghornMountsStep/01-clean-system/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/06_CleanLonghornMountsStep/01-clean-system/mocks.yaml

# Test 2: Active mounts
./cluster-bloom cli --config step_integration_tests/06_CleanLonghornMountsStep/02-active-mounts/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/06_CleanLonghornMountsStep/02-active-mounts/mocks.yaml
```

## Expected Outcomes

### Success Cases
- ✅ All cleanup commands execute
- ✅ Longhorn services stopped
- ✅ PVC mounts unmounted
- ✅ CSI mounts cleaned up
- ✅ Device files removed
- ✅ Step completes successfully

### Non-Fatal Error Cases
- ⚠️ Commands fail but are logged only
- ⚠️ Step continues despite individual command failures
- ✅ Step still completes successfully

## Related Code
- Step implementation: `pkg/steps.go:998-1089`
- Shell command execution uses `ShellCmdHelper.Exec()` with `|| true` suffix

## Notes
- **All commands are non-fatal**: Every command has `|| true` suffix
- **Idempotent**: Safe to run multiple times
- **Aggressive cleanup**: Uses force unmount (`-lf` and `-Af` flags)
- **Process killing**: Uses `fuser -km` to kill blocking processes
- **Multiple unmount attempts**: Retries PVC unmount 3 times
- **Critical for reinstall**: Prevents "device busy" errors on fresh install
