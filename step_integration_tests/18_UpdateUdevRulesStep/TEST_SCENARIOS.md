# UpdateUdevRulesStep Integration Tests

## Purpose
Configure udev rules to set proper permissions for AMD GPU devices (KFD and render devices) to enable non-root GPU access.

## Step Overview
- **Execution Order**: Step 18
- **Commands Executed**:
  - Writes `/etc/udev/rules.d/70-amdgpu.rules`
  - `sudo udevadm control --reload-rules`
  - `sudo udevadm trigger`
- **Skip Conditions**:
  - Skipped if `GPU_NODE=false`

## What This Step Does
1. Creates udev rules file `/etc/udev/rules.d/70-amdgpu.rules` with content:
   ```
   KERNEL=="kfd", MODE="0666"
   SUBSYSTEM=="drm", KERNEL=="renderD*", MODE="0666"
   ```
2. Reloads udev rules to pick up the new configuration
3. Triggers udev to apply rules to existing devices
4. Enables non-root users to access GPU devices
5. Required for ROCm applications and containerized GPU workloads

## Test Scenarios

### Success Scenarios

#### 1. Fresh Rule Creation - File Doesn't Exist
Tests successful creation of new udev rules file.

#### 2. Rule File Already Exists - Overwrite
Tests overwriting existing rules file with correct content.

#### 3. Rules Reload Successfully
Tests successful reload of udev rules.

#### 4. Trigger Apply Successfully
Tests successful triggering of udev to apply rules.

### Failure Scenarios

#### 5. Failed to Write Rules File
Tests error handling when rules file cannot be written (permissions).

#### 6. udevadm reload-rules Fails
Tests error handling when reload-rules command fails.

#### 7. udevadm trigger Fails
Tests error handling when trigger command fails.

#### 8. GPU_NODE=false - Step Skipped
Tests that step is properly skipped on non-GPU nodes.

## Configuration Requirements

- `ENABLED_STEPS: "UpdateUdevRulesStep"`
- `GPU_NODE: true` (required, otherwise step is skipped)

## Mock Requirements

```yaml
mocks:
  # Scenario 1: Fresh rule creation
  "WriteFile./etc/udev/rules.d/70-amdgpu.rules":
    output: ""
    error: null

  "UpdateUdevRulesStep.ReloadRules":
    output: ""
    error: null

  "UpdateUdevRulesStep.Trigger":
    output: ""
    error: null

  # Scenario 2: Overwrite existing
  "WriteFile./etc/udev/rules.d/70-amdgpu.rules":
    output: ""
    error: null

  "UpdateUdevRulesStep.ReloadRules":
    output: ""
    error: null

  "UpdateUdevRulesStep.Trigger":
    output: ""
    error: null

  # Scenario 5: Write fails
  "WriteFile./etc/udev/rules.d/70-amdgpu.rules":
    output: ""
    error: "open /etc/udev/rules.d/70-amdgpu.rules: permission denied"

  # Scenario 6: Reload fails
  "WriteFile./etc/udev/rules.d/70-amdgpu.rules":
    output: ""
    error: null

  "UpdateUdevRulesStep.ReloadRules":
    output: ""
    error: "Failed to send reload request: No such file or directory"

  # Scenario 7: Trigger fails
  "WriteFile./etc/udev/rules.d/70-amdgpu.rules":
    output: ""
    error: null

  "UpdateUdevRulesStep.ReloadRules":
    output: ""
    error: null

  "UpdateUdevRulesStep.Trigger":
    output: ""
    error: "error executing trigger: operation not permitted"
```

## Running Tests

```bash
# Test 1: Fresh creation
./cluster-bloom cli --config step_integration_tests/18_UpdateUdevRulesStep/01-fresh-creation/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/18_UpdateUdevRulesStep/01-fresh-creation/mocks.yaml

# Test 2: Overwrite existing
./cluster-bloom cli --config step_integration_tests/18_UpdateUdevRulesStep/02-overwrite-existing/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/18_UpdateUdevRulesStep/02-overwrite-existing/mocks.yaml

# Test 5: Write fails
./cluster-bloom cli --config step_integration_tests/18_UpdateUdevRulesStep/05-write-fails/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/18_UpdateUdevRulesStep/05-write-fails/mocks.yaml

# Test 8: GPU_NODE=false
./cluster-bloom cli --config step_integration_tests/18_UpdateUdevRulesStep/08-non-gpu-node/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/18_UpdateUdevRulesStep/08-non-gpu-node/mocks.yaml
```

## Expected Outcomes

### Success Cases
- ✅ Udev rules file created/updated
- ✅ Rules reloaded successfully
- ✅ Rules applied to existing devices
- ✅ GPU devices accessible with mode 0666
- ✅ Step completes successfully

### Failure Cases
- ❌ File write failure stops execution
- ❌ Rules reload failure stops execution
- ❌ Trigger failure stops execution
- ❌ Step skipped on non-GPU nodes

## Related Code
- Step implementation: `pkg/steps.go:539-566`
- Rules file path: `/etc/udev/rules.d/70-amdgpu.rules`
- File content hardcoded in step
- Two udev rules defined

## Notes
- **udev**: Linux device manager that handles device nodes in /dev
- **KFD (Kernel Fusion Driver)**: AMD GPU compute interface device
- **renderD devices**: GPU render nodes (e.g., /dev/dri/renderD128)
- **MODE="0666"**: Sets device permissions to read/write for all users
- **Default permissions**: Usually 0660 (root:video group only)
- **Why needed**: Allows non-root containers and users to access GPU
- **Security consideration**: 0666 is permissive - consider restricting in production
- **File naming**: 70- prefix determines rule priority (higher = later execution)
- **udevadm control --reload-rules**: Tells udev to reload all rule files
- **udevadm trigger**: Replays device events to apply new rules
- **Without trigger**: Rules only apply to newly connected devices
- **DRM subsystem**: Direct Rendering Manager for GPU devices
- **KERNEL match**: Matches device kernel name pattern
- **SUBSYSTEM match**: Matches device subsystem type
- **Kubernetes requirement**: GPU operator and device plugins need device access
- **Container runtime**: containerd/docker need access to pass GPU to containers
- **ROCm requirement**: ROCm tools and libraries require KFD access
- **Complements**: Works with SetupAndCheckRocmStep (ROCm installation)
- **Idempotent**: Safe to run multiple times, overwrites with same content
- **Alternative**: Could use udev group membership instead of 0666 permissions
