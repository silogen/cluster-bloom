# SetupMultipathStep Integration Tests

## Purpose
Configure multipath to blacklist standard devices (sd[a-z0-9]+) to prevent multipath from interfering with normal disk operations.

## Step Overview
- **Execution Order**: Step 9
- **Commands Executed**:
  - Reads `/etc/multipath.conf` (if exists)
  - Writes blacklist entry to `/etc/multipath.conf`
  - `systemctl restart multipathd.service`
  - `multipath -t` (verification)
- **Skip Conditions**: Never skipped

## What This Step Does
1. Checks if `/etc/multipath.conf` exists
2. If exists, reads current configuration and checks for existing blacklist
3. If blacklist doesn't exist, adds standard device blacklist:
   ```
   blacklist {
       devnode "^sd[a-z0-9]+"
   }
   ```
4. If file doesn't exist, creates it with blacklist configuration
5. Restarts multipathd service
6. Verifies configuration with `multipath -t`

## Test Scenarios

### Success Scenarios

#### 1. No Multipath Config - Create New
Tests creation of new `/etc/multipath.conf` with blacklist when file doesn't exist.

#### 2. Existing Config Without Blacklist - Add Blacklist
Tests adding blacklist to existing multipath.conf that doesn't have one.

#### 3. Existing Config With Blacklist - No Changes
Tests that step completes without modification when blacklist already exists.

#### 4. Service Restart Success
Tests successful restart of multipathd service after configuration update.

### Failure Scenarios

#### 5. Failed to Read Existing Config
Tests error handling when multipath.conf exists but cannot be read.

#### 6. Failed to Write Config
Tests error handling when multipath.conf cannot be written.

#### 7. Service Restart Fails
Tests error handling when multipathd service restart fails.

#### 8. Multipath Verification Fails
Tests error handling when `multipath -t` verification command fails.

## Configuration Requirements

- `ENABLED_STEPS: "SetupMultipathStep"`
- No specific configuration required

## Mock Requirements

```yaml
mocks:
  # Scenario 1: No config file exists
  "Stat./etc/multipath.conf":
    output: ""
    error: "stat /etc/multipath.conf: no such file or directory"

  "WriteFile./etc/multipath.conf":
    output: ""
    error: null

  "SetupMultipath.RestartService":
    output: ""
    error: null

  "SetupMultipath.VerifyConfig":
    output: |
      multipath configuration valid
    error: null

  # Scenario 2: Existing config without blacklist
  "Stat./etc/multipath.conf":
    output: "file"
    error: null

  "ReadFile./etc/multipath.conf":
    output: |
      defaults {
          user_friendly_names yes
      }
    error: null

  "WriteFile./etc/multipath.conf":
    output: ""
    error: null

  # Scenario 3: Existing config with blacklist
  "Stat./etc/multipath.conf":
    output: "file"
    error: null

  "ReadFile./etc/multipath.conf":
    output: |
      defaults {
          user_friendly_names yes
      }
      blacklist {
          devnode "^sd[a-z0-9]+"
      }
    error: null
  # No write needed - already has blacklist
```

## Running Tests

```bash
# Test 1: No config file
./cluster-bloom cli --config step_integration_tests/09_SetupMultipathStep/01-no-config-file/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/09_SetupMultipathStep/01-no-config-file/mocks.yaml

# Test 2: Add blacklist
./cluster-bloom cli --config step_integration_tests/09_SetupMultipathStep/02-add-blacklist/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/09_SetupMultipathStep/02-add-blacklist/mocks.yaml

# Test 3: Already has blacklist
./cluster-bloom cli --config step_integration_tests/09_SetupMultipathStep/03-has-blacklist/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/09_SetupMultipathStep/03-has-blacklist/mocks.yaml
```

## Expected Outcomes

### Success Cases
- ✅ Multipath.conf created or updated with blacklist
- ✅ Blacklist pattern `^sd[a-z0-9]+` added
- ✅ multipathd service restarted
- ✅ Configuration verified with `multipath -t`
- ✅ Step completes successfully

### Failure Cases
- ❌ File read failure stops execution
- ❌ File write failure stops execution
- ❌ Service restart failure stops execution
- ❌ Configuration verification failure stops execution

## Related Code
- Step implementation: `pkg/steps.go:255-268`
- Multipath configuration: Check for existing blacklist pattern

## Notes
- **Blacklist purpose**: Prevents multipath from claiming standard disks (sda, sdb, etc.)
- **Pattern**: `^sd[a-z0-9]+` matches sda, sdb, sdc, sda1, sdb2, etc.
- **Why needed**: Multipath can interfere with single-path disk operations
- **Service restart**: Required for configuration changes to take effect
- **Verification**: `multipath -t` tests config without applying
- **Idempotent**: Detects existing blacklist and skips if present
- **Config location**: `/etc/multipath.conf` (system-wide multipath configuration)
