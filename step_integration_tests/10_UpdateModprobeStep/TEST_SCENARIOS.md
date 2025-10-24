# UpdateModprobeStep Integration Tests

## Purpose
Unblacklist the amdgpu kernel module by commenting out any blacklist entries in modprobe configuration files, then load the amdgpu module.

## Step Overview
- **Execution Order**: Step 10
- **Commands Executed**:
  - `sudo sed -i '/^blacklist amdgpu/s/^/# /' /etc/modprobe.d/*.conf`
  - `modprobe amdgpu`
- **Skip Conditions**: Never skipped

## What This Step Does
1. Searches all files in `/etc/modprobe.d/` for lines starting with `blacklist amdgpu`
2. Comments out any found blacklist lines by prepending `# `
3. Loads the amdgpu kernel module
4. Enables AMD GPU functionality

## Test Scenarios

### Success Scenarios

#### 1. No Blacklist Entries - Load Module
Tests successful module loading when no blacklist entries exist.

#### 2. Blacklist Entry Exists - Comment and Load
Tests commenting out existing `blacklist amdgpu` entry and loading module.

#### 3. Multiple Blacklist Entries - Comment All
Tests commenting out multiple blacklist entries across different conf files.

#### 4. Module Already Loaded
Tests that modprobe handles already-loaded module gracefully.

### Failure Scenarios

#### 5. sed Command Fails
Tests error handling when sed command to comment blacklist fails.

#### 6. modprobe Fails - Module Not Available
Tests error handling when amdgpu module cannot be loaded (not available).

#### 7. modprobe Fails - Hardware Not Present
Tests error handling when module load fails due to no AMD GPU hardware.

## Configuration Requirements

- `ENABLED_STEPS: "UpdateModprobeStep"`
- No specific configuration required
- Typically runs on GPU nodes but not conditional

## Mock Requirements

```yaml
mocks:
  # Scenario 1: No blacklist, module loads
  "UpdateModprobe.CommentBlacklist":
    output: ""
    error: null

  "UpdateModprobe.LoadModule":
    output: ""
    error: null

  # Scenario 2: Blacklist exists
  "UpdateModprobe.CommentBlacklist":
    output: |
      # Modified /etc/modprobe.d/amdgpu-blacklist.conf
    error: null

  "UpdateModprobe.LoadModule":
    output: ""
    error: null

  # Scenario 6: modprobe fails
  "UpdateModprobe.CommentBlacklist":
    output: ""
    error: null

  "UpdateModprobe.LoadModule":
    output: ""
    error: "modprobe: FATAL: Module amdgpu not found in directory /lib/modules/5.15.0-97-generic"
```

## Running Tests

```bash
# Test 1: No blacklist
./cluster-bloom cli --config step_integration_tests/10_UpdateModprobeStep/01-no-blacklist/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/10_UpdateModprobeStep/01-no-blacklist/mocks.yaml

# Test 2: Blacklist exists
./cluster-bloom cli --config step_integration_tests/10_UpdateModprobeStep/02-blacklist-exists/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/10_UpdateModprobeStep/02-blacklist-exists/mocks.yaml

# Test 6: modprobe fails
./cluster-bloom cli --config step_integration_tests/10_UpdateModprobeStep/06-modprobe-fails/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/10_UpdateModprobeStep/06-modprobe-fails/mocks.yaml
```

## Expected Outcomes

### Success Cases
- ✅ Blacklist lines commented out (if present)
- ✅ amdgpu module loaded successfully
- ✅ GPU functionality enabled
- ✅ Step completes successfully

### Failure Cases
- ❌ sed command failure stops execution
- ❌ modprobe failure stops execution
- ❌ Error indicates module not available or hardware issue

## Related Code
- Step implementation: `pkg/steps.go:270-283`
- sed command: In-place edit with pattern replacement
- modprobe: Kernel module loading

## Notes
- **Blacklist reason**: Some systems blacklist amdgpu due to compatibility issues
- **sed pattern**: `/^blacklist amdgpu/s/^/# /` finds lines starting with "blacklist amdgpu" and prepends "# "
- **Wildcard match**: `*.conf` processes all modprobe config files
- **Common files**: `/etc/modprobe.d/blacklist.conf`, `/etc/modprobe.d/amdgpu-blacklist.conf`
- **modprobe behavior**: Silently succeeds if module already loaded
- **GPU requirement**: amdgpu module only available with AMD GPU drivers installed
- **Complements**: Works with SetupAndCheckRocmStep (which installs drivers)
- **Non-conditional**: Runs on all nodes (not just GPU_NODE=true)
