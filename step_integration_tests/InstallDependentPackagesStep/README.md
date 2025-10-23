# InstallDependentPackagesStep Integration Test

## Purpose
This test validates the InstallDependentPackagesStep in dry-run mode, specifically testing the scenario where the chrony package installation fails.

## Test Scenario
- **Step under test**: InstallDependentPackagesStep
- **Expected behavior**: Step should fail when chrony package installation fails
- **Mock strategy**:
  - Successfully mock installation of: open-iscsi, jq, nfs-common
  - Fail mock installation of: chrony
  - K8s tools installation should not be reached due to chrony failure

## Files
- `config.yaml` - Bloom configuration that enables only this step
- `mocks.yaml` - Mock return values for dry-run commands

## Running the Test

```bash
./cluster-bloom cli --config step_integration_tests/InstallDependentPackagesStep/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/InstallDependentPackagesStep/mocks.yaml
```

## Expected Output
```
[1/1] Install Dependent Packages
      Ensure jq, nfs-common, open-iscsi , and chrony are installed
      ❌ FAILED: setup of packages failed: failed to install chrony: ...
```

## Verification
Check `bloom.log` for:
1. Mock values loaded successfully
2. First three packages show successful dry-run execution
3. Chrony shows mock error being returned
4. Step execution stops at chrony failure
5. K8s tools installation is not attempted
