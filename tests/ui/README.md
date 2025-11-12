# UI Testing

Browser-based end-to-end testing for the Cluster-Bloom WebUI configuration wizard.

## Directory Structure

```
tests/ui/
├── README.md                   # This file
├── browser_test.go             # Main test runner using chromedp
├── go.mod                      # Go module dependencies
├── go.sum                      # Go module checksums
├── docs/                       # Documentation
│   └── TEST_CASES.md          # Comprehensive test case documentation
├── scripts/                    # Test execution scripts
│   └── run_ui_tests.sh        # Automated test runner with bloom server
└── testdata/                   # Test case YAML files
    ├── valid/                  # Valid configuration test cases
    │   ├── bloom_basic_first_node.yaml
    │   ├── bloom_minimal_no_gpu.yaml
    │   ├── bloom_single_disk.yaml
    │   ├── bloom_cert_provide.yaml
    │   ├── bloom_cert_skip.yaml
    │   ├── bloom_additional_node.yaml
    │   ├── bloom_valid_subdomain.yaml
    │   └── bloom_with_hyphens.yaml
    ├── invalid/                # Validation failure test cases
    │   ├── bloom_invalid_domain.yaml
    │   ├── bloom_invalid_special_chars.yaml
    │   ├── bloom_invalid_format.yaml
    │   ├── bloom_empty_domain.yaml
    │   └── bloom_empty_disks.yaml
    └── integration/            # End-to-end integration tests
        ├── bloom_e2e_first_node.yaml
        └── bloom_e2e_additional_node.yaml
```

## Prerequisites

1. **Chromium with remote debugging**
   ```bash
   chromium --remote-debugging-port=9222 --headless --no-sandbox
   ```

2. **Bloom binary**
   ```bash
   cd /workspace
   go build -o dist/bloom
   ```

## Running Tests

### Quick Start (Recommended)

Use the automated test runner script:

```bash
cd /workspace/tests/ui
./scripts/run_ui_tests.sh
```

This script:
- Starts a bloom web server on port 62078
- Runs all test cases from `testdata/`
- Automatically cleans up (kills server, removes temp files)
- Exits with proper error code on test failure

### Manual Testing

Run tests manually for debugging:

```bash
# 1. Start chromium with remote debugging
chromium --remote-debugging-port=9222 --headless --no-sandbox &

# 2. Start bloom server
cd /tmp
/workspace/dist/bloom &
BLOOM_PID=$!

# 3. Run tests
cd /workspace/tests/ui
BLOOM_YAML_PATH=/tmp/bloom.yaml go test -v -run TestWebFormE2E

# 4. Cleanup
kill $BLOOM_PID
```

### Run Specific Test Cases

```bash
# Run only valid configuration tests
cd /workspace/tests/ui
BLOOM_YAML_PATH=/tmp/bloom.yaml go test -v -run TestWebFormE2E/.*valid

# Run only invalid configuration tests
BLOOM_YAML_PATH=/tmp/bloom.yaml go test -v -run TestWebFormE2E/.*invalid

# Run a specific test case
BLOOM_YAML_PATH=/tmp/bloom.yaml go test -v -run TestWebFormE2E/bloom_basic_first_node.yaml
```

## Test Case Format

Each test case is a YAML file in the `testdata/` directory:

### Valid Configuration Test
```yaml
DOMAIN: test.local
CLUSTER_DISKS: /dev/sdb,/dev/sdc
CERT_OPTION: generate
FIRST_NODE: true
GPU_NODE: true
```

### Invalid Configuration Test
```yaml
DOMAIN: INVALID-DOMAIN
CLUSTER_DISKS: /dev/sdb
CERT_OPTION: generate
FIRST_NODE: true
GPU_NODE: false

expected_error: "Valid domain format"
```

## Test Categories

### Valid Tests (`testdata/valid/`)
- Test successful configuration saves
- Verify bloom.yaml is created with correct values
- Cover all valid input combinations

### Invalid Tests (`testdata/invalid/`)
- Test form validation
- Verify error messages are displayed
- Ensure bloom.yaml is NOT created on validation failure
- Cover edge cases and invalid inputs

### Integration Tests (`testdata/integration/`)
- End-to-end workflows
- Multi-step scenarios
- Real-world use cases

## Adding New Test Cases

1. Create a new YAML file in the appropriate `testdata/` subdirectory:
   - `testdata/valid/` for valid configurations
   - `testdata/invalid/` for validation tests
   - `testdata/integration/` for E2E tests

2. Follow the naming convention: `bloom_descriptive_name.yaml`

3. For invalid tests, include `expected_error` field with text that should appear in validation message

4. Run tests to verify:
   ```bash
   ./scripts/run_ui_tests.sh
   ```

## Test Output

### Passing Tests
```
=== RUN   TestWebFormE2E/bloom_basic_first_node.yaml
    browser_test.go:95: Running test case from: testdata/valid/bloom_basic_first_node.yaml
--- PASS: TestWebFormE2E/bloom_basic_first_node.yaml (4.53s)
```

### Failing Tests
```
=== RUN   TestWebFormE2E/bloom_invalid_domain.yaml
    browser_test.go:95: Running test case from: testdata/invalid/bloom_invalid_domain.yaml
    browser_test.go:228: ❌ Expected error text 'Valid domain format' not found in validation
    browser_test.go:229:    Validation info: {...}
--- FAIL: TestWebFormE2E/bloom_invalid_domain.yaml (4.06s)
```

## Debugging

### View Test Logs
```bash
# The test script shows bloom logs on failure
./scripts/run_ui_tests.sh

# For manual debugging, check bloom.log
tail -f /tmp/bloom-ui-test.*/bloom.log
```

### Interactive Browser Mode
For visual debugging, run chromium without headless mode:

```bash
# Start visible chromium
chromium --remote-debugging-port=9222 --no-sandbox &

# Run tests - you'll see the browser actions
cd /workspace/tests/ui
BLOOM_YAML_PATH=/tmp/bloom.yaml go test -v -run TestWebFormE2E
```

## CI/CD Integration

Skip tests in environments without browser support:

```bash
SKIP_BROWSER_TESTS=1 go test -v
```

## Test Coverage

Current coverage:
- ✅ Form validation (domain, disks, required fields)
- ✅ Certificate options (generate, provide, skip)
- ✅ Checkbox states (FIRST_NODE, GPU_NODE)
- ✅ Save Configuration button
- ✅ HTML5 validation error display
- ✅ Multiple disk selection
- ⚠️ Save and Install button (requires integration testing)
- ⚠️ Root device conflict detection
- ⚠️ Prefilled configuration loading

See `docs/TEST_CASES.md` for complete test case documentation.

## Troubleshooting

### Port 9222 already in use
```bash
# Kill existing chromium instance
pkill -9 chromium
chromium --remote-debugging-port=9222 --headless --no-sandbox &
```

### Port 62078 already in use
```bash
# Kill existing bloom instance
lsof -ti:62078 | xargs kill -9
```

### Tests timeout
- Increase timeout in browser_test.go (default 30s)
- Check if bloom server started successfully
- Verify chromium is running on port 9222

### bloom.yaml not created
- Check bloom server logs
- Verify form validation passed
- Check file permissions in test directory

## Contributing

When adding new test cases:
1. Document the test case in `docs/TEST_CASES.md`
2. Create the YAML file in appropriate `testdata/` subdirectory
3. Ensure test name is descriptive
4. Add comments explaining complex test scenarios
5. Run full test suite before committing

## References

- [chromedp Documentation](https://github.com/chromedp/chromedp)
- [Go Testing Package](https://pkg.go.dev/testing)
- [Conventional Commits](https://www.conventionalcommits.org/)
