# PrepareLonghornDisksStep - Test 01: Skip Mode - NO_DISKS_FOR_CLUSTER

## Purpose
Verify that PrepareLonghornDisksStep is completely skipped when `NO_DISKS_FOR_CLUSTER` is set to true.

## Test Scenario
- **Configuration Mode**: Skip Mode
- **Expected Behavior**: Step should be skipped entirely, no disk operations performed
- **Test Type**: Success (skip scenario)

## Configuration
```yaml
NO_DISKS_FOR_CLUSTER: true
ENABLED_STEPS: "PrepareLonghornDisksStep"
FIRST_NODE: true
```

## Expected Behavior

1. **Step Execution**: Step is skipped before Action runs
2. **Skip Function**: Returns true due to `NO_DISKS_FOR_CLUSTER: true`
3. **Log Message**: "Skipping drive mounting as NO_DISKS_FOR_CLUSTER is set."
4. **Operations**: No disk mounting, formatting, or fstab modifications
5. **Result**: Step marked as skipped in summary

## Mock Requirements
**None** - The step is skipped before any commands are executed.

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/01-skip-mode/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/01-skip-mode/mocks.yaml
```

## Expected Output

```
🚀 Starting installation with 1 steps
════════════════════════════════════════

[1/1] Prepare Longhorn Disks
      Mount selected disks or populate disk map from CLUSTER_PREMOUNTED_DISKS configuration
      ⏭️  SKIPPED
```

## Verification

Check `bloom.log` for:

1. Step appears in execution plan:
   ```
   Total steps to execute: 1
   Starting step: Prepare Longhorn Disks
   ```

2. Skip message logged:
   ```
   Skipping drive mounting as NO_DISKS_FOR_CLUSTER is set.
   ```

3. No disk-related commands executed (no lsblk, mkfs, mount, etc.)

4. Step marked as skipped in summary

## Success Criteria

- ✅ Step appears in the execution plan
- ✅ Skip function returns true
- ✅ Skip log message appears
- ✅ No mount/format commands executed
- ✅ Step marked as SKIPPED in output
- ✅ No errors logged
- ✅ Overall execution succeeds

## Related Code

- Step definition: `pkg/steps.go:285` (PrepareLonghornDisksStep)
- Skip function: `pkg/steps.go:289-294`
