# CheckUbuntuStep Integration Tests

## Purpose
Verify that the system is running a supported Ubuntu version (20.04, 22.04, or 24.04) before proceeding with installation.

## Step Overview
- **Execution Order**: Step 3
- **Commands Executed**: Reads `/etc/os-release` file
- **Skip Conditions**: Never skipped

## What This Step Does
1. Reads `/etc/os-release` file
2. Checks if `ID=ubuntu`
3. Verifies `VERSION_ID` is one of: "20.04", "22.04", "24.04"
4. Fails installation if OS is not supported Ubuntu version

## Test Scenarios

### Success Scenarios

#### 1. Ubuntu 20.04 Detected
Tests successful validation on Ubuntu 20.04 LTS (Focal Fossa).

#### 2. Ubuntu 22.04 Detected
Tests successful validation on Ubuntu 22.04 LTS (Jammy Jellyfish).

#### 3. Ubuntu 24.04 Detected
Tests successful validation on Ubuntu 24.04 LTS (Noble Numbat).

### Failure Scenarios

#### 4. Not Ubuntu (Debian)
Tests validation failure when running on Debian instead of Ubuntu.

#### 5. Unsupported Ubuntu Version (18.04)
Tests validation failure on Ubuntu 18.04 (unsupported).

#### 6. Unsupported Ubuntu Version (23.04)
Tests validation failure on Ubuntu 23.04 (non-LTS, unsupported).

#### 7. Missing os-release File
Tests validation failure when `/etc/os-release` doesn't exist.

#### 8. Missing VERSION_ID
Tests validation failure when `VERSION_ID` field is missing from os-release.

## Configuration Requirements

- `ENABLED_STEPS: "CheckUbuntuStep"`
- Minimal configuration to allow step execution

## Mock Requirements

```yaml
mocks:
  # Mock /etc/os-release content
  "Stat./etc/os-release":
    output: "file"
    error: null

  "ReadFile./etc/os-release":
    output: |
      NAME="Ubuntu"
      VERSION="22.04.3 LTS (Jammy Jellyfish)"
      ID=ubuntu
      ID_LIKE=debian
      PRETTY_NAME="Ubuntu 22.04.3 LTS"
      VERSION_ID="22.04"
      HOME_URL="https://www.ubuntu.com/"
      SUPPORT_URL="https://help.ubuntu.com/"
      BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
    error: null
```

## Running Tests

```bash
# Test 1: Ubuntu 20.04
./cluster-bloom cli --config step_integration_tests/03_CheckUbuntuStep/01-ubuntu-20.04/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/03_CheckUbuntuStep/01-ubuntu-20.04/mocks.yaml

# Test 2: Ubuntu 22.04
./cluster-bloom cli --config step_integration_tests/03_CheckUbuntuStep/02-ubuntu-22.04/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/03_CheckUbuntuStep/02-ubuntu-22.04/mocks.yaml

# Test 5: Unsupported version
./cluster-bloom cli --config step_integration_tests/03_CheckUbuntuStep/05-ubuntu-18.04-unsupported/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/03_CheckUbuntuStep/05-ubuntu-18.04-unsupported/mocks.yaml
```

## Expected Outcomes

### Success Cases
- ✅ Step completes successfully
- ✅ Logs detected Ubuntu version
- ✅ Proceeds to next step

### Failure Cases
- ❌ Step fails with error message
- ❌ Error indicates unsupported OS or version
- ❌ Installation stops

## Related Code
- Step implementation: `pkg/steps.go:71-84`

## Notes
- This check ensures compatibility with Ubuntu-specific package repositories
- Only LTS versions are officially supported
- Non-LTS versions (like 23.04) are explicitly rejected
