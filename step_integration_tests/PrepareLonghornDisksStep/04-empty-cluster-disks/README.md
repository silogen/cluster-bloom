# PrepareLonghornDisksStep - Test 04: New Disk Mounting - Empty CLUSTER_DISKS

## Purpose
Verify that ValidateArgsStep correctly detects and rejects empty CLUSTER_DISKS configuration when NO_DISKS_FOR_CLUSTER is false, failing with a clear error message.

## Test Scenario
- **Configuration Mode**: Invalid Configuration (empty CLUSTER_DISKS)
- **Expected Behavior**: Fail with error "either CLUSTER_PREMOUNTED_DISKS or CLUSTER_DISKS must be set"
- **Test Type**: Failure (configuration validation)

## Configuration
```yaml
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: ""
ENABLED_STEPS: "ValidateArgsStep,PrepareLonghornDisksStep"
FIRST_NODE: true
```

## Expected Behavior

### 1. ValidateArgsStep Execution
- `NO_DISKS_FOR_CLUSTER` is `false`, so disk configuration is required
- `CLUSTER_DISKS` is empty string
- `CLUSTER_PREMOUNTED_DISKS` is also empty
- ValidateArgsStep detects invalid configuration

### 2. Error Detection
- ValidateArgsStep checks disk configuration requirements
- Error message: `"either CLUSTER_PREMOUNTED_DISKS or CLUSTER_DISKS must be set"`
- Validation fails before PrepareLonghornDisksStep executes

### 3. No Disk Operations Executed
- PrepareLonghornDisksStep never runs (failed at validation)
- No mount checks
- No lsblk operations
- No formatting or mounting
- No fstab operations

## Mock Requirements

No mocks required - error occurs during configuration validation before any system commands are executed.

```yaml
mocks: {}
```

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/PrepareLonghornDisksStep/05-empty-disks-error/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/PrepareLonghornDisksStep/05-empty-disks-error/mocks.yaml
```

## Expected Output

```
🚀 Starting installation with 2 steps
════════════════════════════════════════

[1/2] Validate Configuration
      Validate all configuration arguments
      ❌ FAILED: configuration validation failed: configuration validation failed:
- CLUSTER_PREMOUNTED_DISKS: either CLUSTER_PREMOUNTED_DISKS or CLUSTER_DISKS must be set
════════════════════════════════════════
🏁 Installation Summary
════════════════════════════════════════
[❌] Validate Configuration
[❌] Prepare Longhorn Disks

❌ Execution failed: configuration validation failed: configuration validation failed:
- CLUSTER_PREMOUNTED_DISKS: either CLUSTER_PREMOUNTED_DISKS or CLUSTER_DISKS must be set
```

## Verification

Check `bloom.log` for the error flow:

### 1. ValidateArgsStep Execution
```
Starting step: Validate Configuration
```

### 2. Configuration Validation
```
NO_DISKS_FOR_CLUSTER=false but no disk parameters specified
```

### 3. Error Logged
```
level=error msg="Execution failed: configuration validation failed: configuration validation failed:
- CLUSTER_PREMOUNTED_DISKS: either CLUSTER_PREMOUNTED_DISKS or CLUSTER_DISKS must be set"
```

### 4. No Disk Commands Logged
- PrepareLonghornDisksStep marked as failed but never executes
- No `[DRY-RUN]` entries for mount, lsblk, mkfs, or fstab operations
- Error occurs during validation, before PrepareLonghornDisksStep runs

## Success Criteria

- ✅ ValidateArgsStep starts execution
- ✅ Configuration validation detects empty CLUSTER_DISKS and CLUSTER_PREMOUNTED_DISKS
- ✅ Error message: "either CLUSTER_PREMOUNTED_DISKS or CLUSTER_DISKS must be set"
- ✅ ValidateArgsStep fails with appropriate error level
- ✅ PrepareLonghornDisksStep never executes (marked failed)
- ✅ No disk operations attempted
- ✅ No disk map created
- ✅ No mounts attempted

## Related Code

- Step definition: `pkg/steps.go:43-55` (ValidateArgsStep)
- Args validation: `pkg/args/args.go` (ValidateArgs function)
- Disk configuration check validates that when `NO_DISKS_FOR_CLUSTER` is false, either `CLUSTER_DISKS` or `CLUSTER_PREMOUNTED_DISKS` must be set

## Notes

- **ValidateArgsStep**: This test demonstrates the importance of the ValidateArgsStep which catches configuration errors before any system operations
- **Fail-Fast**: Error is detected during validation, preventing invalid disk operations
- **No Side Effects**: Empty configuration cannot cause accidental system modifications
- **Error Clarity**: Clear error message helps users identify missing disk configuration
- **Multi-Error Reporting**: ValidateArgsStep can report multiple configuration errors at once if present
- **Defense in Depth**: Validates that disk configuration is complete when NO_DISKS_FOR_CLUSTER is false
