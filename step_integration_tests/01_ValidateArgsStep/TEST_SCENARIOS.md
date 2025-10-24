# ValidateArgsStep Integration Tests

## Purpose
Validates all configuration arguments before installation begins. This step ensures configuration integrity and catches errors early before any system modifications occur.

## Step Overview
- **Execution Order**: Step 1 (First step in installation)
- **Commands Executed**: None (pure validation logic)
- **Skip Conditions**: Never skipped (always runs first)

## What This Step Does
1. Validates all required arguments are non-empty
2. Checks deprecated arguments and provides migration warnings
3. Validates enum fields against allowed options
4. Validates IP addresses (format and not loopback/unspecified)
5. Validates file paths (existence and absolute paths)
6. Validates URL format (http/https with valid host)
7. Validates JOIN_TOKEN format and length (32-512 characters)
8. Checks for conflicting configurations

## Test Scenarios

### Success Scenarios

#### 1. Valid Configuration - All Required Fields
Tests successful validation with all required fields populated correctly.

#### 2. Valid Optional Fields
Tests validation with optional fields like OIDC_URL, TLS_CERT, etc.

#### 3. Valid Enum Values
Tests CERT_OPTION with "generate" and "existing" values.

### Failure Scenarios

#### 4. Empty Required Field
Tests validation failure when required non-empty field is missing.

#### 5. Invalid IP Address
Tests validation failure with malformed IP address.

#### 6. Invalid URL Format
Tests validation failure with invalid URL (missing scheme, invalid host).

#### 7. Non-existent File Path
Tests validation failure when TLS_CERT or TLS_KEY file doesn't exist.

#### 8. Relative File Path
Tests validation failure when file path is not absolute.

#### 9. Invalid Enum Value
Tests validation failure when CERT_OPTION has invalid value.

#### 10. Deprecated Arguments Warning
Tests detection of deprecated arguments (SKIP_DISK_CHECK, LONGHORN_DISKS, SELECTED_DISKS).

#### 11. Conflicting Configuration
Tests validation failure when both CLUSTER_PREMOUNTED_DISKS and CLUSTER_DISKS are set.

#### 12. Invalid JOIN_TOKEN Length
Tests validation failure when JOIN_TOKEN is too short (<32) or too long (>512).

#### 13. Invalid JOIN_TOKEN Characters
Tests validation failure when JOIN_TOKEN contains invalid characters.

## Configuration Requirements

### Required Arguments
- `CLUSTER_NAME`
- `DOMAIN`
- `JOIN_TOKEN` (for additional nodes)
- At least one of: `CLUSTER_DISKS`, `CLUSTER_PREMOUNTED_DISKS`, or `NO_DISKS_FOR_CLUSTER=true`

### Optional Arguments
- All other configuration parameters

## Mock Requirements

No mocks required - this step performs pure validation logic without executing commands.

## Running Tests

```bash
# Test 1: Valid configuration
./cluster-bloom cli --config step_integration_tests/01_ValidateArgsStep/01-valid-all-required/config.yaml \
                    --dry-run

# Test 4: Empty required field
./cluster-bloom cli --config step_integration_tests/01_ValidateArgsStep/04-empty-required-field/config.yaml \
                    --dry-run
```

## Expected Outcomes

### Success Cases
- ✅ Validation passes
- ✅ No errors logged
- ✅ Proceeds to next step

### Failure Cases
- ❌ Validation fails with descriptive error
- ❌ Installation stops before any system modifications
- ❌ Error message indicates which field(s) failed validation

## Related Code
- Validation implementation: `pkg/args/args.go` (ValidateArgs function)
- Step definition: `pkg/steps.go:43-55`

## Notes
- This is a **fail-fast** validation step
- Catches configuration errors before any system changes
- Multiple validation errors reported together
- Provides clear migration guidance for deprecated arguments
