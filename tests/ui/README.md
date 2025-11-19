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

**Chromium with remote debugging**
```bash
chromium --remote-debugging-port=9222 --headless --no-sandbox &
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
go test -v -run TestConfigBasedTests/bloom_autodetect_nvme.yaml
```

## Test Case Format

### Valid Configuration Test
```yaml
DOMAIN: test.local
CLUSTER_DISKS: /dev/sdb,/dev/sdc
CERT_OPTION: generate
FIRST_NODE: true
GPU_NODE: true
```

### Auto-Detection Test with Mocks
```yaml
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

expected_cluster_disks: "/dev/nvme0n1,/dev/nvme1n1"
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

## Mock System

The mock system allows testing disk auto-detection without real hardware:

### Available Mocks

- `addrootdevicetoconfig.statconfigfile` - Mock file existence check
- `getunmountedphysicaldisks.listblockdevices` - Mock lsblk output
- `getunmountedphysicaldisks.checkmount.<disk>` - Mock mount status per disk
- `getunmountedphysicaldisks.udevinfo.<disk>` - Mock udev properties per disk

### Mock Scenarios

1. **NVMe Detection** - Multiple NVMe drives with mount filtering
2. **Mixed Drives** - NVMe + SCSI/SATA detection
3. **Virtual Filtering** - Exclude QEMU/VMware virtual disks
4. **No Disks** - All disks mounted (empty detection)

## Adding New Test Cases

1. Create a YAML file in the appropriate `testdata/` subdirectory
2. Follow naming convention: `bloom_descriptive_name.yaml`
3. For autodetect tests, include `mocks` section and `expected_cluster_disks`
4. Run tests to verify:
   ```bash
   go test -v -run TestConfigBasedTests
   ```

## CI/CD Integration

Skip tests in environments without browser support:

```bash
SKIP_BROWSER_TESTS=1 go test -v
```

## Test Output

### Passing Auto-Detection Test
```
=== RUN   TestConfigBasedTests/bloom_autodetect_nvme.yaml
    config_test.go:73: Running test: testdata/autodetect/bloom_autodetect_nvme.yaml
    config_test.go:75: Expected CLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1
    config_test.go:137: ✅ Auto-detected CLUSTER_DISKS correctly: /dev/nvme0n1,/dev/nvme1n1
    config_test.go:221: ✅ Browser form field correctly shows: /dev/nvme0n1,/dev/nvme1n1
--- PASS: TestConfigBasedTests/bloom_autodetect_nvme.yaml (4.62s)
```

## Troubleshooting

### Port 9222 already in use
```bash
pkill -9 chromium
chromium --remote-debugging-port=9222 --headless --no-sandbox &
```

### Tests timeout
- Check if chromium is running on port 9222
- Increase timeout in config_test.go (default 30s)

## Contributing

When adding new test cases:
1. Document the test case in `docs/TEST_CASES.md`
2. Create YAML file in appropriate `testdata/` subdirectory
3. For autodetect tests, define all required mocks
4. Ensure test name is descriptive
5. Run full test suite before committing

## References

- [chromedp Documentation](https://github.com/chromedp/chromedp)
- [Go Testing Package](https://pkg.go.dev/testing)
