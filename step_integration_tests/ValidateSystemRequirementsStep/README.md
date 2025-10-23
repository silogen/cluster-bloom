# ValidateSystemRequirementsStep Integration Test

## Purpose
This test validates the ValidateSystemRequirementsStep in dry-run mode with successful validation.

## Test Scenario
- **Step under test**: ValidateSystemRequirementsStep
- **Expected behavior**: Step should pass all validation checks
- **Mock strategy**:
  - Mock kernel module checks (lsmod, modinfo) to show required modules are available

## Files
- `config.yaml` - Bloom configuration that enables only this step
- `mocks.yaml` - Mock return values for dry-run commands

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/ValidateSystemRequirementsStep/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/ValidateSystemRequirementsStep/mocks.yaml
```

## Expected Output
```
[1/1] Validate System Requirements
      Validate system resources (disk, memory, CPU, OS version, kernel modules)
      ✅ COMPLETED in <time>
```

## Verification
Check `bloom.log` for:
1. Mock values loaded successfully
2. System validation checks executed
3. Step completes successfully
