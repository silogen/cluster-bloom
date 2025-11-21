# UI Testing

Browser-based end-to-end testing for the Cluster-Bloom WebUI configuration wizard with mock support for disk auto-detection.

## Directory Structure

```
tests/ui/
├── README.md                   # This file
├── config_test.go              # Main test runner with mock support
├── go.mod                      # Go module dependencies
├── go.sum                      # Go module checksums
├── docs/                       # Documentation
│   └── TEST_CASES.md          # Comprehensive test case documentation
└── testdata/                   # Test case YAML files
    ├── valid/                  # Valid configuration test cases
    │   ├── bloom_basic_first_node.yaml
    │   ├── bloom_minimal_no_gpu.yaml
    │   ├── bloom_single_disk.yaml
    │   ├── bloom_additional_node.yaml
    │   ├── bloom_valid_subdomain.yaml
    │   └── bloom_with_hyphens.yaml
    ├── invalid/                # Validation failure test cases
    │   ├── bloom_invalid_domain.yaml
    │   ├── bloom_invalid_special_chars.yaml
    │   └── bloom_invalid_format.yaml
    ├── integration/            # End-to-end integration tests
    │   ├── bloom_e2e_first_node.yaml
    │   └── bloom_e2e_additional_node.yaml
    └── autodetect/             # Auto-detection tests with mocks
        ├── bloom_autodetect_nvme.yaml
        ├── bloom_autodetect_mixed.yaml
        ├── bloom_autodetect_virtual_filtered.yaml
        └── bloom_autodetect_no_disks.yaml
```

## Prerequisites

### Running Browser with Docker

The tests require a headless Chrome browser. Start it using Docker:

```bash
docker run -d --rm \
  --name chrome-test \
  --net=host \
  -e "PORT=9222" \
  browserless/chrome:latest
```

To stop the browser when done:

```bash
docker stop chrome-test
```

## Running Tests

```bash
cd /workspace/tests/ui
go test -v -run TestConfigBasedTests
```

This will:
- Start a separate web server for each test (or reuse if mocks unchanged)
- Load test-specific mocks for disk auto-detection
- Test all scenarios: valid, invalid, integration, and autodetect
- Automatically clean up servers when tests complete

### Run Specific Tests

```bash
# Run only autodetect tests
go test -v -run TestConfigBasedTests/.*autodetect

# Run only valid configuration tests
go test -v -run TestConfigBasedTests/bloom_basic

# Run a specific test case
go test -v -run TestConfigBasedTests/bloom_autodetect_nvme
```

## Test Case Format

Test cases use a YAML format with three sections: `input`, `mocks`, and `output`.

### Valid Configuration Test

```yaml
input:
  DOMAIN: test-basic.local
  CLUSTER_DISKS: /dev/sdb,/dev/sdc
  CERT_OPTION: generate
  FIRST_NODE: true
  GPU_NODE: true
```

### Invalid Configuration Test

```yaml
input:
  DOMAIN: -invalid-domain-format.com
  CLUSTER_DISKS: /dev/sdb
  CERT_OPTION: generate
  FIRST_NODE: true
  GPU_NODE: false

output:
  error:
    DOMAIN: "Please match the requested format"
```

### Auto-Detection Test with Mocks

```yaml
input:
  DOMAIN: autodetect-nvme.local
  FIRST_NODE: true
  GPU_NODE: false
  CERT_OPTION: generate

mocks:
  addrootdevicetoconfig.statconfigfile:
    error: "no such file or directory"
  getunmountedphysicaldisks.listblockdevices:
    output: |
      nvme0n1 disk
      nvme1n1 disk
      nvme2n1 disk
  getunmountedphysicaldisks.checkmount.nvme0n1:
    output: ""
  getunmountedphysicaldisks.checkmount.nvme1n1:
    output: ""
  getunmountedphysicaldisks.checkmount.nvme2n1:
    output: "/"

output:
  CLUSTER_DISKS: "/dev/nvme0n1,/dev/nvme1n1"
```

## Test Sections

### `input`
Contains the initial form values to fill in the WebUI. If a field is omitted, auto-detection is triggered (for CLUSTER_DISKS).

### `mocks` (optional)
Defines mock command responses for disk auto-detection testing. Available mocks:

- `addrootdevicetoconfig.statconfigfile` - Mock file existence check
- `getunmountedphysicaldisks.listblockdevices` - Mock lsblk output
- `getunmountedphysicaldisks.checkmount.<disk>` - Mock mount status per disk
- `getunmountedphysicaldisks.udevinfo.<disk>` - Mock udev properties per disk

### `output`
Specifies expected results:

- **Valid tests**: Expected values in bloom.yaml (e.g., `DOMAIN: test.local`)
- **Invalid tests**: Expected validation errors (e.g., `error: { DOMAIN: "error message" }`)
- **Auto-detect tests**: Expected auto-detected values (e.g., `CLUSTER_DISKS: "/dev/nvme0n1"`)

## Test Categories

### Valid Tests (`testdata/valid/`)
- Test successful configuration saves
- Verify bloom.yaml is created with correct values
- Cover all valid input combinations

### Invalid Tests (`testdata/invalid/`)
- Test form validation
- Verify field-specific error messages are displayed
- Ensure bloom.yaml is NOT created on validation failure
- Tests use HTML5 validation checking

### Integration Tests (`testdata/integration/`)
- End-to-end workflows
- Multi-step scenarios
- Real-world use cases

### Auto-Detection Tests (`testdata/autodetect/`)
- Test disk auto-detection with mocked system commands
- Verify NVMe and SATA/SCSI disk detection
- Test virtual disk filtering (QEMU, VMware)
- Test mount status checking
- Each test creates its own server with test-specific mocks

## Mock Scenarios

1. **NVMe Detection** - Multiple NVMe drives with mount filtering
2. **Mixed Drives** - NVMe + SCSI/SATA detection
3. **Virtual Filtering** - Exclude QEMU/VMware virtual disks
4. **Swap Filtering** - Exclude disks in use for swap
5. **No Disks** - All disks mounted or otherwise unavailable (empty detection)

## Adding New Test Cases

1. Create a YAML file in the appropriate `testdata/` subdirectory
2. Follow naming convention: `bloom_descriptive_name.yaml`
3. Use the three-section format: `input`, `mocks` (if needed), `output`
4. For validation tests, specify `output.error.<field>` with expected error message
5. For autodetect tests, include `mocks` section and `output.CLUSTER_DISKS`
6. Run tests to verify:
   ```bash
   go test -v -run TestConfigBasedTests
   ```

## CI/CD Integration

The tests are integrated into the GitHub Actions workflow. See `.github/workflows/run-tests.yml` for the complete CI/CD configuration.

To skip tests in environments without browser support:

```bash
SKIP_BROWSER_TESTS=1 go test -v
```

## Test Output

### Passing Valid Configuration Test
```
=== RUN   TestConfigBasedTests/bloom_basic_first_node
    config_test.go:100: ✅ All output values match expected
--- PASS: TestConfigBasedTests/bloom_basic_first_node (2.31s)
```

### Passing Invalid Configuration Test
```
=== RUN   TestConfigBasedTests/bloom_invalid_domain
    config_test.go:150: ✅ Validation error correctly displayed for DOMAIN
--- PASS: TestConfigBasedTests/bloom_invalid_domain (1.82s)
```

### Passing Auto-Detection Test
```
=== RUN   TestConfigBasedTests/bloom_autodetect_nvme
    config_test.go:100: ✅ All output values match expected
    config_test.go:105: ✅ Auto-detected CLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1
--- PASS: TestConfigBasedTests/bloom_autodetect_nvme (3.45s)
```

## Troubleshooting

### Chrome container not running
```bash
# Check if container is running
docker ps | grep chrome-test

# Start the container using the command from Prerequisites section
docker run -d --rm --name chrome-test --net=host -e "PORT=9222" browserless/chrome:latest
```

### Port 9222 already in use
```bash
# Stop existing Chrome container
docker stop chrome-test

# Or if running locally without Docker
pkill -9 chromium
```

### Tests timeout
- Check if Chrome is running on port 9222: `curl http://localhost:9222/json/version`
- Increase timeout in config_test.go if needed (default 30s)
- Check Docker logs: `docker logs chrome-test`

### Mock not working
- Verify mock key format matches function.command.argument pattern
- Check that all required mocks for a disk are defined
- Review test output for mock loading messages

## Contributing

When adding new test cases:
1. Document the test case in `docs/TEST_CASES.md`
2. Create YAML file in appropriate `testdata/` subdirectory
3. Use the three-section format (`input`, `mocks`, `output`)
4. For autodetect tests, define all required mocks
5. Ensure test name is descriptive
6. Run full test suite before committing

## References

- [chromedp Documentation](https://github.com/chromedp/chromedp)
- [browserless/chrome Docker Image](https://hub.docker.com/r/browserless/chrome)
- [Go Testing Package](https://pkg.go.dev/testing)
