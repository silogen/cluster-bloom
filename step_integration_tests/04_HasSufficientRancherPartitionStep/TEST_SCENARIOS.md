# HasSufficientRancherPartitionStep Integration Tests

## Purpose
Check if `/var/lib/rancher` partition has at least 500GB of available space for GPU nodes.

## Step Overview
- **Execution Order**: Step 4
- **Commands Executed**:
  - `mkdir -p /var/lib/rancher`
  - `df -BG /var/lib/rancher`
- **Skip Conditions**:
  - `GPU_NODE=false`
  - `SKIP_RANCHER_PARTITION_CHECK=true`

## What This Step Does
1. Creates `/var/lib/rancher` directory if it doesn't exist
2. Runs `df -BG` to check available space
3. Parses output to extract available GB
4. Fails if available space < 500GB on GPU nodes

## Test Scenarios

### Success Scenarios

#### 1. GPU Node with Sufficient Space (600GB)
Tests successful validation when partition has 600GB available.

#### 2. CPU Node (Step Skipped)
Tests that step is skipped when `GPU_NODE=false`.

#### 3. Skip Check Enabled
Tests that step is skipped when `SKIP_RANCHER_PARTITION_CHECK=true`.

### Failure Scenarios

#### 4. GPU Node with Insufficient Space (400GB)
Tests validation failure when partition only has 400GB available.

#### 5. mkdir Command Fails
Tests error handling when directory creation fails.

#### 6. df Command Fails
Tests error handling when disk space check fails.

## Configuration Requirements

- `GPU_NODE: true` (for step to run)
- `SKIP_RANCHER_PARTITION_CHECK: false` (default)
- `ENABLED_STEPS: "HasSufficientRancherPartitionStep"`

## Mock Requirements

```yaml
mocks:
  "HasSufficientRancherPartition.Mkdir":
    output: ""
    error: null

  "HasSufficientRancherPartition.Df":
    output: |
      Filesystem     1G-blocks  Used Available Use% Mounted on
      /dev/sda1           700G  100G      600G  15% /var/lib/rancher
    error: null
```

## Running Tests

```bash
# Test 1: Sufficient space
./cluster-bloom cli --config step_integration_tests/04_HasSufficientRancherPartitionStep/01-sufficient-space/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/04_HasSufficientRancherPartitionStep/01-sufficient-space/mocks.yaml
```

## Expected Outcomes

### Success Cases
- ✅ Step completes successfully
- ✅ Logs available space
- ✅ Proceeds to next step

### Failure Cases
- ❌ Step fails with error about insufficient space
- ❌ Error message shows required vs available space
- ❌ Installation stops

## Related Code
- Step implementation: `pkg/steps.go:487-548`

## Notes
- Only applies to GPU nodes (large model storage requirements)
- Can be skipped with `SKIP_RANCHER_PARTITION_CHECK=true`
- 500GB minimum ensures space for models and training data
