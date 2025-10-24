# UninstallRKE2Step Integration Tests

## Purpose
Execute the RKE2 uninstall script if it exists to clean up previous RKE2 installations.

## Step Overview
- **Execution Order**: Step 7
- **Commands Executed**:
  - `/usr/local/bin/rke2-uninstall.sh`
- **Skip Conditions**: Never skipped (errors are ignored if script doesn't exist)

## What This Step Does
1. Checks if RKE2 uninstall script exists at `/usr/local/bin/rke2-uninstall.sh`
2. Executes the script if found
3. Ignores errors (script may not exist on clean systems)

## Test Scenarios

### Success Scenarios

#### 1. RKE2 Installed - Uninstall Script Runs Successfully
Tests successful execution when RKE2 is installed and uninstall script exists.

#### 2. RKE2 Not Installed - Script Doesn't Exist
Tests that step completes successfully when script doesn't exist (clean system).

#### 3. Uninstall Script Runs with Warnings
Tests that script runs but produces non-fatal warnings (still completes successfully).

### Non-Failure Scenarios

#### 4. Uninstall Script Fails
Tests that errors from the uninstall script are logged but don't stop the installation.

**Note**: This step ignores all errors - it's designed to be non-fatal for fresh installs where RKE2 isn't present.

## Configuration Requirements

- `ENABLED_STEPS: "UninstallRKE2Step"`
- No specific configuration required

## Mock Requirements

```yaml
mocks:
  # Scenario 1: Script exists and runs successfully
  "UninstallRKE2Step.RunScript":
    output: |
      Uninstalling RKE2...
      Stopping rke2-server service...
      Removing RKE2 files...
      Uninstall complete.
    error: null

  # Scenario 2: Script doesn't exist (would be real file system check)
  # No mock needed - will fall through to actual file check

  # Scenario 4: Script fails but errors ignored
  "UninstallRKE2Step.RunScript":
    output: ""
    error: "rke2-uninstall.sh: line 15: some error occurred"
```

## Running Tests

```bash
# Test 1: Successful uninstall
./cluster-bloom cli --config step_integration_tests/07_UninstallRKE2Step/01-uninstall-success/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/07_UninstallRKE2Step/01-uninstall-success/mocks.yaml

# Test 2: Script doesn't exist
./cluster-bloom cli --config step_integration_tests/07_UninstallRKE2Step/02-script-not-exists/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/07_UninstallRKE2Step/02-script-not-exists/mocks.yaml

# Test 4: Script fails (errors ignored)
./cluster-bloom cli --config step_integration_tests/07_UninstallRKE2Step/04-script-fails/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/07_UninstallRKE2Step/04-script-fails/mocks.yaml
```

## Expected Outcomes

### All Cases Complete Successfully
- ✅ Script runs if present
- ✅ Step completes even if script missing
- ✅ Step completes even if script fails
- ✅ Errors logged but not fatal
- ✅ Installation proceeds to next step

## Related Code
- Step implementation: `pkg/steps.go:1091-1103`
- Script check and execution with error suppression

## Notes
- **Non-blocking cleanup**: Designed to not fail fresh installs
- **Idempotent**: Safe to run on systems without RKE2
- **Error suppression**: All errors are ignored to support both upgrade and fresh install scenarios
- **Script location**: `/usr/local/bin/rke2-uninstall.sh` (installed by RKE2)
- **What script does**: Stops services, removes containers, deletes RKE2 files
- **Complements**: Works with CleanLonghornMountsStep to fully clean previous installations
