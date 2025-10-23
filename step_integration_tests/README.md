# Step Integration Tests

This directory contains integration tests for individual Bloom installation steps using the dry-run mode with mock return values.

## Purpose

These tests allow you to:
- Test individual steps in isolation without executing actual system commands
- Validate step behavior with controlled command outputs and errors
- Verify error handling and edge cases
- Test the installation flow without requiring a full cluster setup

## Structure

Each step has its own subdirectory containing:
- `config.yaml` - Bloom configuration that enables only the specific step
- `mocks.yaml` - Mock return values for commands executed during the step
- `README.md` - Documentation for the specific test scenario

## Available Tests

### ValidateSystemRequirementsStep
Tests the system validation step with successful validation of resources, OS version, and kernel modules.

### InstallDependentPackagesStep
Tests the package installation step with a scenario where chrony installation fails.

## Running Tests

General format:
```bash
./cluster-bloom cli --config step_integration_tests/<StepName>/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/<StepName>/mocks.yaml
```

Example:
```bash
./cluster-bloom cli --config step_integration_tests/InstallDependentPackagesStep/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/InstallDependentPackagesStep/mocks.yaml
```

## Creating New Tests

To add a new step integration test:

1. Create a subdirectory: `mkdir -p step_integration_tests/<StepName>`

2. Create `config.yaml` with:
   ```yaml
   # Enable only your step
   ENABLED_STEPS: "<StepName>"

   # Add required configuration
   FIRST_NODE: true
   # ... other required config
   ```

3. Create `mocks.yaml` with mock command return values:
   ```yaml
   mocks:
     "StepName.CommandName":
       output: "expected output"
       error: null  # or "error message" for failures
   ```

4. Create `README.md` documenting the test scenario

5. Test your configuration:
   ```bash
   ./cluster-bloom cli --config step_integration_tests/<StepName>/config.yaml \
                       --dry-run \
                       --dry-run-mocks step_integration_tests/<StepName>/mocks.yaml
   ```

## Mock Value Format

Mock values are defined in YAML format:

```yaml
mocks:
  # Successful command
  "StepName.Operation":
    output: "command output here"
    error: null

  # Failed command
  "StepName.FailingOperation":
    output: ""
    error: "error message here"
```

The command name must match the `name` parameter passed to the command execution functions in the code.

## Finding Command Names

To find the correct command names to mock:

1. Run the test without mocks to see what commands are executed
2. Check `bloom.log` for `[DRY-RUN]` entries showing command names
3. Look at the step implementation in `pkg/steps.go` or related files

## Tips

- Start with minimal mocks and add more as needed based on log output
- Use `grep "[DRY-RUN]" bloom.log` to see which commands need mocking
- Mock values are optional - commands without mocks return empty results
- Test both success and failure scenarios for comprehensive coverage
