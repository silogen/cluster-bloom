# WebUI Test Cases

This document outlines test cases for validating the Cluster-Bloom WebUI configuration wizard.

## Test Case Format

All test cases use a three-section YAML format:

```yaml
input:
  DOMAIN: <value>
  CLUSTER_DISKS: <value>  # Optional - omit to trigger auto-detection
  CERT_OPTION: <value>
  FIRST_NODE: <boolean>
  GPU_NODE: <boolean>

mocks:  # Optional - for auto-detection tests
  <function>.<command>.<argument>:
    output: "<command output>"
    error: "<error message>"

output:
  <field>: <expected_value>
  error:  # For validation tests
    <field>: "<expected error message>"
```

## Test Categories

### 1. Valid Configuration Tests (`testdata/valid/`)

These tests verify that valid inputs are accepted and correctly saved to bloom.yaml.

#### TC-001: Basic First Node Configuration
**File:** `bloom_basic_first_node.yaml`

**Input:**
```yaml
input:
  DOMAIN: test-basic.local
  CLUSTER_DISKS: /dev/sdb,/dev/sdc
  CERT_OPTION: generate
  FIRST_NODE: true
  GPU_NODE: true
```

**Expected Result:**
- Configuration saved successfully
- bloom.yaml contains all input values
- No validation errors

---

#### TC-002: Minimal No GPU Configuration
**File:** `bloom_minimal_no_gpu.yaml`

**Input:**
```yaml
input:
  DOMAIN: minimal.local
  CLUSTER_DISKS: /dev/sdb
  CERT_OPTION: generate
  FIRST_NODE: true
  GPU_NODE: false
```

**Expected Result:**
- Configuration saved with GPU_NODE: false
- Single disk accepted

---

#### TC-003: Single Disk Selection
**File:** `bloom_single_disk.yaml`

**Input:**
```yaml
input:
  DOMAIN: single-disk.local
  CLUSTER_DISKS: /dev/nvme0n1
  CERT_OPTION: generate
  FIRST_NODE: true
  GPU_NODE: false
```

**Expected Result:**
- Single disk configuration accepted
- NVMe device path stored correctly

---

#### TC-004: Additional Node Configuration
**File:** `bloom_additional_node.yaml`

**Input:**
```yaml
input:
  DOMAIN: cluster.local
  CLUSTER_DISKS: /dev/sdb
  SERVER_IP: 192.168.1.100
  CERT_OPTION: generate
  FIRST_NODE: false
  GPU_NODE: false
```

**Expected Result:**
- FIRST_NODE: false accepted
- SERVER_IP field visible and saved

---

#### TC-005: Valid Subdomain
**File:** `bloom_valid_subdomain.yaml`

**Input:**
```yaml
input:
  DOMAIN: sub.domain.example.com
  CLUSTER_DISKS: /dev/sdb
  CERT_OPTION: generate
  FIRST_NODE: true
  GPU_NODE: false
```

**Expected Result:**
- Multi-level subdomain accepted
- Configuration saved successfully

---

#### TC-006: Domain with Hyphens
**File:** `bloom_with_hyphens.yaml`

**Input:**
```yaml
input:
  DOMAIN: test-cluster-123.local
  CLUSTER_DISKS: /dev/sdb
  CERT_OPTION: generate
  FIRST_NODE: true
  GPU_NODE: false
```

**Expected Result:**
- Hyphens in domain name accepted
- Numbers in domain name accepted

---

### 2. Invalid Configuration Tests (`testdata/invalid/`)

These tests verify that invalid inputs are rejected with appropriate field-specific error messages.

#### TC-007: Invalid Domain Name
**File:** `bloom_invalid_domain.yaml`

**Input:**
```yaml
input:
  DOMAIN: INVALID-DOMAIN-WITH-CAPS
  CLUSTER_DISKS: /dev/sdb
  CERT_OPTION: generate
  FIRST_NODE: true
  GPU_NODE: false

output:
  error:
    DOMAIN: "Please match the requested format"
```

**Expected Result:**
- HTML5 validation error displayed on DOMAIN field
- Error message contains "Please match the requested format"
- bloom.yaml is NOT created
- Save button is blocked by browser validation

---

#### TC-008: Invalid Special Characters
**File:** `bloom_invalid_special_chars.yaml`

**Input:**
```yaml
input:
  DOMAIN: domain_with_underscore.com
  CLUSTER_DISKS: /dev/sdb
  CERT_OPTION: generate
  FIRST_NODE: true
  GPU_NODE: false

output:
  error:
    DOMAIN: "Please match the requested format"
```

**Expected Result:**
- Underscore in domain name rejected
- HTML5 validation error on DOMAIN field
- bloom.yaml is NOT created

---

#### TC-009: Invalid Domain Format
**File:** `bloom_invalid_format.yaml`

**Input:**
```yaml
input:
  DOMAIN: -starts-with-hyphen.com
  CLUSTER_DISKS: /dev/sdb
  CERT_OPTION: generate
  FIRST_NODE: true
  GPU_NODE: false

output:
  error:
    DOMAIN: "Please match the requested format"
```

**Expected Result:**
- Domain starting with hyphen rejected
- HTML5 validation error displayed
- bloom.yaml is NOT created

---

#### TC-010: Missing Server IP for Additional Node
**File:** `bloom_additional_node_missing_server_ip.yaml`

**Input:**
```yaml
input:
  DOMAIN: cluster.local
  CLUSTER_DISKS: /dev/sdb
  CERT_OPTION: generate
  FIRST_NODE: false
  GPU_NODE: false

output:
  error:
    SERVER_IP: "Please fill out this field"
```

**Expected Result:**
- Required field validation error on SERVER_IP
- Error message: "Please fill out this field"
- bloom.yaml is NOT created

---

### 3. Auto-Detection Tests (`testdata/autodetect/`)

These tests verify disk auto-detection functionality using mocked system commands.

#### TC-011: NVMe Disk Auto-Detection
**File:** `bloom_autodetect_nvme.yaml`

**Input:**
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

**Expected Result:**
- nvme0n1 and nvme1n1 detected (unmounted)
- nvme2n1 excluded (mounted on /)
- CLUSTER_DISKS auto-filled in UI
- Configuration saves with detected disks

---

#### TC-012: Mixed Drive Auto-Detection
**File:** `bloom_autodetect_mixed.yaml`

**Input:**
```yaml
input:
  DOMAIN: autodetect-mixed.local
  FIRST_NODE: true
  GPU_NODE: false
  CERT_OPTION: generate

mocks:
  addrootdevicetoconfig.statconfigfile:
    error: "no such file or directory"
  getunmountedphysicaldisks.listblockdevices:
    output: |
      nvme0n1 disk
      sda disk
      sdb disk
  getunmountedphysicaldisks.checkmount.nvme0n1:
    output: ""
  getunmountedphysicaldisks.checkmount.sda:
    output: ""
  getunmountedphysicaldisks.checkmount.sdb:
    output: ""
  getunmountedphysicaldisks.udevinfo.sda:
    output: |
      ID_BUS=scsi
      ID_MODEL=Samsung_SSD_870
  getunmountedphysicaldisks.udevinfo.sdb:
    output: |
      ID_BUS=scsi
      ID_MODEL=WD_Blue_1TB

output:
  CLUSTER_DISKS: "/dev/nvme0n1,/dev/sda,/dev/sdb"
```

**Expected Result:**
- NVMe and SCSI/SATA drives both detected
- All unmounted disks included
- Correct ordering in output

---

#### TC-013: Virtual Disk Filtering
**File:** `bloom_autodetect_virtual_filtered.yaml`

**Input:**
```yaml
input:
  DOMAIN: autodetect-virtual.local
  FIRST_NODE: true
  GPU_NODE: false
  CERT_OPTION: generate

mocks:
  addrootdevicetoconfig.statconfigfile:
    error: "no such file or directory"
  getunmountedphysicaldisks.listblockdevices:
    output: |
      sda disk
      sdb disk
      sdc disk
  getunmountedphysicaldisks.checkmount.sda:
    output: "/"
  getunmountedphysicaldisks.checkmount.sdb:
    output: ""
  getunmountedphysicaldisks.checkmount.sdc:
    output: ""
  getunmountedphysicaldisks.udevinfo.sdb:
    output: |
      ID_BUS=scsi
      ID_MODEL=QEMU_HARDDISK
  getunmountedphysicaldisks.udevinfo.sdc:
    output: |
      ID_BUS=scsi
      ID_MODEL=Samsung_SSD_870

output:
  CLUSTER_DISKS: "/dev/sdc"
```

**Expected Result:**
- sda excluded (mounted)
- sdb excluded (QEMU virtual disk)
- sdc included (real Samsung SSD)
- Only physical disks detected

---

#### TC-014: Swap Disk Filtering
**File:** `bloom_autodetect_swap.yaml`

**Input:**
```yaml
input:
  DOMAIN: autodetect-swap.local
  FIRST_NODE: true
  GPU_NODE: false
  CERT_OPTION: generate

mocks:
  addrootdevicetoconfig.statconfigfile:
    error: "no such file or directory"
  getunmountedphysicaldisks.listblockdevices:
    output: |
      nvme0n1 disk
      sda disk
      sdb disk
  getunmountedphysicaldisks.checkmount.nvme0n1:
    output: ""
  getunmountedphysicaldisks.checkmount.sda:
    output: ""
  getunmountedphysicaldisks.checkmount.sdb:
    output: "[SWAP]"
  getunmountedphysicaldisks.udevinfo.sda:
    output: |
      ID_BUS=scsi
      ID_MODEL=Samsung_SSD_870
  getunmountedphysicaldisks.udevinfo.sdb:
    output: |
      ID_BUS=scsi
      ID_MODEL=WD_Blue_1TB

output:
  CLUSTER_DISKS: "/dev/nvme0n1,/dev/sda"
```

**Expected Result:**
- sdb excluded (in use for swap)
- nvme0n1 and sda included (available)
- Swap disks filtered out

---

#### TC-015: No Available Disks
**File:** `bloom_autodetect_no_disks.yaml`

**Input:**
```yaml
input:
  DOMAIN: autodetect-none.local
  FIRST_NODE: true
  GPU_NODE: false
  CERT_OPTION: generate

mocks:
  addrootdevicetoconfig.statconfigfile:
    error: "no such file or directory"
  getunmountedphysicaldisks.listblockdevices:
    output: |
      sda disk
      sdb disk
  getunmountedphysicaldisks.checkmount.sda:
    output: "/"
  getunmountedphysicaldisks.checkmount.sdb:
    output: "/mnt/data"

output:
  CLUSTER_DISKS: ""
```

**Expected Result:**
- All disks mounted, none available
- CLUSTER_DISKS field empty
- User must manually specify disks or validation fails

---

### 4. Integration Tests (`testdata/integration/`)

These tests verify end-to-end workflows.

#### TC-016: Complete First Node Setup
**File:** `bloom_e2e_first_node.yaml`

**Input:**
```yaml
input:
  DOMAIN: cluster.test.local
  CLUSTER_DISKS: /dev/sdb,/dev/sdc
  CERT_OPTION: generate
  FIRST_NODE: true
  GPU_NODE: true
```

**Expected Result:**
- Configuration saved successfully
- All fields validated
- bloom.yaml created with all values
- Ready for installation

---

#### TC-017: Complete Additional Node Setup
**File:** `bloom_e2e_additional_node.yaml`

**Input:**
```yaml
input:
  DOMAIN: cluster.test.local
  CLUSTER_DISKS: /dev/sdb
  SERVER_IP: 192.168.1.100
  CERT_OPTION: generate
  FIRST_NODE: false
  GPU_NODE: false
```

**Expected Result:**
- Additional node configuration accepted
- SERVER_IP field validated and saved
- FIRST_NODE: false saved correctly
- Ready to join cluster

---

## Running Specific Test Categories

```bash
# Run all valid configuration tests
go test -v -run TestConfigBasedTests/.*valid

# Run all invalid/validation tests
go test -v -run TestConfigBasedTests/.*invalid

# Run all auto-detection tests
go test -v -run TestConfigBasedTests/.*autodetect

# Run all integration tests
go test -v -run TestConfigBasedTests/.*e2e

# Run a specific test
go test -v -run TestConfigBasedTests/bloom_autodetect_nvme
```

## Test Execution Priority

### P0 - Critical (Must Pass)
- TC-001: Basic First Node Configuration
- TC-002: Minimal No GPU Configuration
- TC-007: Invalid Domain Name
- TC-011: NVMe Disk Auto-Detection

### P1 - High (Should Pass)
- TC-003 through TC-006: Valid configuration variations
- TC-008 through TC-010: Validation tests
- TC-012 through TC-014: Auto-detection scenarios
- TC-016: Complete First Node Setup

### P2 - Medium (Nice to Have)
- TC-015: No Available Disks
- TC-017: Complete Additional Node Setup

## Adding New Test Cases

1. Choose appropriate directory: `valid/`, `invalid/`, `autodetect/`, or `integration/`
2. Create YAML file following naming: `bloom_descriptive_name.yaml`
3. Use three-section format:
   - `input`: Form fields to fill
   - `mocks`: Command mocks (if auto-detection)
   - `output`: Expected values or errors
4. For validation tests, specify `output.error.<field>`
5. For auto-detection, specify `output.CLUSTER_DISKS`
6. Document the test case in this file
7. Run tests to verify:
   ```bash
   go test -v -run TestConfigBasedTests/bloom_your_new_test
   ```

## Mock System Reference

### Mock Key Format
```
<function>.<command>.<argument>
```

### Available Mock Functions

- `addrootdevicetoconfig.statconfigfile` - File existence check
  ```yaml
  error: "no such file or directory"  # Triggers auto-detection
  ```

- `getunmountedphysicaldisks.listblockdevices` - List all block devices
  ```yaml
  output: |
    nvme0n1 disk
    sda disk
  ```

- `getunmountedphysicaldisks.checkmount.<disk>` - Check mount status
  ```yaml
  output: ""              # Not mounted (available)
  output: "/"             # Mounted (unavailable)
  output: "[SWAP]"        # Swap (unavailable)
  ```

- `getunmountedphysicaldisks.udevinfo.<disk>` - Get device properties
  ```yaml
  output: |
    ID_BUS=scsi
    ID_MODEL=Samsung_SSD_870    # Physical disk
    # or
    ID_MODEL=QEMU_HARDDISK      # Virtual disk (filtered)
  ```

## Validation Error Checking

The test framework checks for field-specific HTML5 validation errors:

```yaml
output:
  error:
    DOMAIN: "Please match the requested format"
    SERVER_IP: "Please fill out this field"
```

Tests verify:
- Error is associated with correct form field
- Error message matches expected text
- Form submission is blocked
- bloom.yaml is NOT created

## Test Coverage Goals

- **Form Validation:** 100% of validation rules covered
- **User Workflows:** All primary user journeys tested
- **Auto-Detection:** All disk detection scenarios tested
- **Error Scenarios:** All expected error conditions tested

## References

- Main README: `/workspace/tests/ui/README.md`
- Test Runner: `/workspace/tests/ui/config_test.go`
- Test Data: `/workspace/tests/ui/testdata/`
