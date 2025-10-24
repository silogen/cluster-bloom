# InotifyInstancesStep Integration Tests

## Purpose
Verify and update the system's inotify max_user_instances setting to ensure sufficient file watching capability for Kubernetes components.

## Step Overview
- **Execution Order**: Step 15
- **Commands Executed**:
  - `sysctl -n fs.inotify.max_user_instances` (check current value)
  - `sudo sysctl -w fs.inotify.max_user_instances=512` (if update needed)
  - Writes/updates `/etc/sysctl.conf` (if update needed)
- **Skip Conditions**: Never skipped

## What This Step Does
1. Reads current value of `fs.inotify.max_user_instances` from sysctl
2. Compares current value against target value (512)
3. If current value < 512:
   - Sets new value via `sysctl -w`
   - Updates `/etc/sysctl.conf` to persist across reboots
   - If sysctl.conf doesn't exist, creates it
   - If parameter already exists in file, updates it
   - If parameter doesn't exist, appends it
4. If current value >= 512, no changes needed

## Test Scenarios

### Success Scenarios

#### 1. Current Value Sufficient - No Update
Tests when current inotify instances (512+) already meets requirement.

#### 2. Current Value Low - Update Successfully
Tests successful update from low value (e.g., 128) to target (512).

#### 3. sysctl.conf Doesn't Exist - Create New File
Tests creation of new sysctl.conf when file doesn't exist.

#### 4. Parameter Exists in sysctl.conf - Update Entry
Tests updating existing `fs.inotify.max_user_instances` entry in sysctl.conf.

#### 5. Parameter Missing in sysctl.conf - Append Entry
Tests appending new parameter when sysctl.conf exists but doesn't have entry.

### Failure Scenarios

#### 6. Failed to Read Current Value
Tests error handling when sysctl command to read current value fails.

#### 7. Failed to Set New Value
Tests error handling when sysctl -w command to set new value fails.

#### 8. Failed to Update sysctl.conf
Tests error handling when writing to /etc/sysctl.conf fails (permissions).

## Configuration Requirements

- `ENABLED_STEPS: "InotifyInstancesStep"`
- No specific configuration required
- Target value hardcoded to 512

## Mock Requirements

```yaml
mocks:
  # Scenario 1: Value already sufficient
  "GetCurrentInotifyValue.Sysctl":
    output: "1024\n"
    error: null
  # No additional commands needed

  # Scenario 2: Update needed
  "GetCurrentInotifyValue.Sysctl":
    output: "128\n"
    error: null

  "SetInotifyValue.SysctlSet":
    output: "fs.inotify.max_user_instances = 512\n"
    error: null

  "ReadFile./etc/sysctl.conf":
    output: |
      # System settings
      net.ipv4.ip_forward=1
    error: null

  "WriteFile./etc/sysctl.conf":
    output: ""
    error: null

  # Scenario 6: Read fails
  "GetCurrentInotifyValue.Sysctl":
    output: ""
    error: "sysctl: cannot stat /proc/sys/fs/inotify/max_user_instances"

  # Scenario 7: Set fails
  "SetInotifyValue.SysctlSet":
    output: ""
    error: "sysctl: permission denied on key 'fs.inotify.max_user_instances'"
```

## Running Tests

```bash
# Test 1: Value sufficient
./cluster-bloom cli --config step_integration_tests/15_InotifyInstancesStep/01-value-sufficient/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/15_InotifyInstancesStep/01-value-sufficient/mocks.yaml

# Test 2: Update needed
./cluster-bloom cli --config step_integration_tests/15_InotifyInstancesStep/02-update-needed/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/15_InotifyInstancesStep/02-update-needed/mocks.yaml

# Test 6: Read fails
./cluster-bloom cli --config step_integration_tests/15_InotifyInstancesStep/06-read-fails/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/15_InotifyInstancesStep/06-read-fails/mocks.yaml
```

## Expected Outcomes

### Success Cases
- ✅ Current value read successfully
- ✅ Value updated to 512 if needed
- ✅ sysctl.conf created or updated
- ✅ Setting persists across reboots
- ✅ Step completes successfully

### Failure Cases
- ❌ sysctl read failure stops execution
- ❌ sysctl write failure stops execution
- ❌ File write failure stops execution

## Related Code
- Step implementation: `pkg/steps.go:490-502`
- Function: `pkg/os-setup.go:VerifyInotifyInstances()`
- Target value constant: 512
- Sysctl parameter: `fs.inotify.max_user_instances`
- Config file: `/etc/sysctl.conf`

## Notes
- **Purpose**: Kubernetes components (kubelet, containerd) use inotify to watch files
- **Default values**: Often 128 or 256 on Ubuntu systems
- **Required value**: 512 for reliable cluster operation
- **Why needed**: Insufficient inotify instances cause file watching failures
- **Symptoms of low value**: "too many open files" or "no space left on device" errors
- **Persistence**: Updates both runtime (sysctl -w) and boot config (/etc/sysctl.conf)
- **Idempotent**: Safe to run multiple times, skips update if value already sufficient
- **File handling**: Creates sysctl.conf if missing, updates existing entries, appends if needed
