# WebUI Test Cases

This document outlines test cases for validating the Cluster-Bloom WebUI configuration wizard.

## Test Case Categories

### 1. Form Validation Tests

#### TC-001: Valid Domain Name
**Objective:** Verify that valid domain names are accepted

**Test Data:**
- `test.local`
- `subdomain.example.com`
- `multi-level.sub.domain.com`
- `test-123.example.com`

**Expected Result:**
- Form validation passes
- Configuration can be saved
- bloom.yaml contains correct DOMAIN value

**YAML File:** `bloom_valid_domain.yaml`

---

#### TC-002: Invalid Domain Name - Uppercase Characters
**Objective:** Verify uppercase characters are rejected

**Test Data:**
- `INVALID-DOMAIN-WITH-CAPS`
- `Test.Local`
- `EXAMPLE.COM`

**Expected Result:**
- HTML5 validation error displayed
- Error message contains "Valid domain format"
- bloom.yaml is NOT created
- Save/Submit buttons are blocked by browser validation

**YAML File:** `bloom_invalid_domain.yaml` (existing)

---

#### TC-003: Invalid Domain Name - Special Characters
**Objective:** Verify special characters (except dots and hyphens) are rejected

**Test Data:**
- `domain_with_underscore.com`
- `domain@example.com`
- `domain#test.com`
- `domain!.com`

**Expected Result:**
- HTML5 validation error displayed
- bloom.yaml is NOT created

**YAML File:** `bloom_invalid_special_chars.yaml`

---

#### TC-004: Invalid Domain Name - Invalid Format
**Objective:** Verify malformed domains are rejected

**Test Data:**
- `-starts-with-hyphen.com`
- `ends-with-hyphen-.com`
- `.starts-with-dot.com`
- `ends-with-dot.com.`
- `double..dots.com`

**Expected Result:**
- HTML5 validation error displayed
- bloom.yaml is NOT created

**YAML File:** `bloom_invalid_format.yaml`

---

#### TC-005: Empty Domain Field
**Objective:** Verify empty required field is rejected

**Test Data:**
- DOMAIN: ""

**Expected Result:**
- HTML5 required field validation error
- bloom.yaml is NOT created

**YAML File:** `bloom_empty_domain.yaml`

---

### 2. Disk Selection Tests

#### TC-006: Single Disk Selection
**Objective:** Verify single disk can be selected

**Test Data:**
- CLUSTER_DISKS: `/dev/sdb`
- CLUSTER_DISKS: `/dev/nvme0n1`

**Expected Result:**
- Configuration saved successfully
- bloom.yaml contains correct disk

**YAML File:** `bloom_single_disk.yaml`

---

#### TC-007: Multiple Disk Selection
**Objective:** Verify multiple disks can be selected

**Test Data:**
- CLUSTER_DISKS: `/dev/sdb,/dev/sdc`
- CLUSTER_DISKS: `/dev/nvme0n1,/dev/nvme0n2,/dev/nvme0n3`

**Expected Result:**
- Configuration saved successfully
- bloom.yaml contains all disks in comma-separated format

**YAML File:** `bloom_basic_first_node.yaml` (existing)

---

#### TC-008: Root Device Conflict
**Objective:** Verify selecting root device shows warning

**Test Data:**
- CLUSTER_DISKS: `/dev/sda` (assuming sda is root device)

**Expected Result:**
- Error modal displayed
- Warning about root device conflict
- bloom.yaml is NOT created

**YAML File:** `bloom_root_device_conflict.yaml`

---

#### TC-009: Empty Disk Selection
**Objective:** Verify empty disk field is rejected

**Test Data:**
- CLUSTER_DISKS: ""

**Expected Result:**
- HTML5 required field validation error
- bloom.yaml is NOT created

**YAML File:** `bloom_empty_disks.yaml`

---

### 3. Certificate Option Tests

#### TC-010: Generate Certificate Option
**Objective:** Verify "generate" certificate option

**Test Data:**
- CERT_OPTION: `generate`

**Expected Result:**
- Configuration saved successfully
- bloom.yaml contains `CERT_OPTION: generate`

**YAML File:** `bloom_basic_first_node.yaml` (existing)

---

#### TC-011: Provide Certificate Option
**Objective:** Verify "provide" certificate option

**Test Data:**
- CERT_OPTION: `provide`

**Expected Result:**
- Configuration saved successfully
- bloom.yaml contains `CERT_OPTION: provide`

**YAML File:** `bloom_cert_provide.yaml`

---

#### TC-012: Skip Certificate Option
**Objective:** Verify "skip" certificate option

**Test Data:**
- CERT_OPTION: `skip`

**Expected Result:**
- Configuration saved successfully
- bloom.yaml contains `CERT_OPTION: skip`

**YAML File:** `bloom_cert_skip.yaml`

---

### 4. Checkbox Tests

#### TC-013: First Node Checkbox - Checked
**Objective:** Verify FIRST_NODE checkbox can be checked

**Test Data:**
- FIRST_NODE: `true`

**Expected Result:**
- Configuration saved successfully
- bloom.yaml contains `FIRST_NODE: true`

**YAML File:** `bloom_basic_first_node.yaml` (existing)

---

#### TC-014: First Node Checkbox - Unchecked
**Objective:** Verify FIRST_NODE checkbox can be unchecked

**Test Data:**
- FIRST_NODE: `false`

**Expected Result:**
- Configuration saved successfully
- bloom.yaml contains `FIRST_NODE: false`

**YAML File:** `bloom_additional_node.yaml`

---

#### TC-015: GPU Node Checkbox - Checked
**Objective:** Verify GPU_NODE checkbox can be checked

**Test Data:**
- GPU_NODE: `true`

**Expected Result:**
- Configuration saved successfully
- bloom.yaml contains `GPU_NODE: true`

**YAML File:** `bloom_basic_first_node.yaml` (existing)

---

#### TC-016: GPU Node Checkbox - Unchecked
**Objective:** Verify GPU_NODE checkbox can be unchecked

**Test Data:**
- GPU_NODE: `false`

**Expected Result:**
- Configuration saved successfully
- bloom.yaml contains `GPU_NODE: false`

**YAML File:** `bloom_minimal_no_gpu.yaml` (existing)

---

### 5. Button Functionality Tests

#### TC-017: Save Configuration Button
**Objective:** Verify Save Configuration button saves without starting installation

**Test Data:**
- Valid configuration

**Expected Result:**
- bloom.yaml is created
- Success message displayed
- Installation does NOT start
- Web interface remains on configuration page

**YAML File:** `bloom_basic_first_node.yaml` (existing)

---

#### TC-018: Save and Install Button
**Objective:** Verify Save and Install button saves and starts installation

**Test Data:**
- Valid configuration

**Expected Result:**
- bloom.yaml is created
- Installation process starts
- UI switches to monitoring view
- Progress steps are displayed

**Note:** This requires integration testing, not just browser automation

---

#### TC-019: Reset Form Button
**Objective:** Verify form can be reset to defaults

**Test Data:**
- Fill form with custom values
- Click reset/clear button (if available)

**Expected Result:**
- Form returns to default/empty state
- No configuration is saved

---

### 6. Prefilled Configuration Tests

#### TC-020: Prefilled Domain
**Objective:** Verify prefilled domain values are loaded

**Test Data:**
- Start bloom with existing bloom.yaml containing DOMAIN

**Expected Result:**
- DOMAIN field is prefilled with value from bloom.yaml
- User can modify the value
- User can save with modified value

---

#### TC-021: Prefilled Disk Selection
**Objective:** Verify prefilled disk values are loaded

**Test Data:**
- Start bloom with existing bloom.yaml containing CLUSTER_DISKS

**Expected Result:**
- CLUSTER_DISKS field is prefilled
- Disks are properly displayed (comma-separated)
- User can modify selection

---

### 7. Error Handling Tests

#### TC-022: Network Error During Save
**Objective:** Verify error handling when network fails during save

**Test Data:**
- Valid configuration
- Simulate network failure

**Expected Result:**
- Error message displayed to user
- bloom.yaml is NOT created
- User can retry

**Note:** Requires manual testing or network simulation

---

#### TC-023: Invalid Server Response
**Objective:** Verify error handling for invalid server responses

**Test Data:**
- Valid configuration
- Server returns 500 error

**Expected Result:**
- Error message displayed
- bloom.yaml may not be created
- Error details shown to user

**Note:** Requires server error simulation

---

### 8. UI/UX Tests

#### TC-024: Form Field Labels
**Objective:** Verify all form fields have clear labels

**Expected Result:**
- All fields have visible labels
- Labels accurately describe the field
- Required fields are marked with *

---

#### TC-025: Help Text Display
**Objective:** Verify help text is shown for complex fields

**Expected Result:**
- DOMAIN field shows format examples
- CLUSTER_DISKS shows selection instructions
- Help text is readable and helpful

---

#### TC-026: Responsive Design
**Objective:** Verify form works on different screen sizes

**Test Data:**
- Desktop (1920x1080)
- Tablet (768x1024)
- Mobile (375x667)

**Expected Result:**
- Form is usable on all screen sizes
- No horizontal scrolling required
- Buttons are accessible

**Note:** Requires viewport testing

---

#### TC-027: Error Modal Display
**Objective:** Verify error modal appears correctly

**Test Data:**
- Trigger validation error

**Expected Result:**
- Modal appears centered on screen
- Error message is readable
- Modal can be dismissed
- Background is dimmed

---

### 9. Integration Tests

#### TC-028: End-to-End First Node Setup
**Objective:** Complete first node setup workflow

**Test Data:**
- DOMAIN: `cluster.test.local`
- CLUSTER_DISKS: `/dev/sdb,/dev/sdc`
- CERT_OPTION: `generate`
- FIRST_NODE: `true`
- GPU_NODE: `true`

**Expected Result:**
- Configuration saved
- Installation starts
- All setup steps complete successfully
- Cluster is operational

**YAML File:** `bloom_e2e_first_node.yaml`

---

#### TC-029: End-to-End Additional Node Setup
**Objective:** Complete additional node setup workflow

**Test Data:**
- DOMAIN: `cluster.test.local`
- CLUSTER_DISKS: `/dev/sdb`
- CERT_OPTION: `generate`
- FIRST_NODE: `false`
- GPU_NODE: `false`

**Expected Result:**
- Configuration saved
- Node joins existing cluster
- All setup steps complete successfully

**YAML File:** `bloom_e2e_additional_node.yaml`

---

### 10. Security Tests

#### TC-030: Localhost-Only Access
**Objective:** Verify web interface is only accessible from localhost

**Test Data:**
- Access from 127.0.0.1
- Access from external IP

**Expected Result:**
- Localhost access succeeds
- External access is blocked
- Appropriate error message shown

**Note:** Requires network testing

---

#### TC-031: Input Sanitization
**Objective:** Verify inputs are sanitized to prevent XSS

**Test Data:**
- DOMAIN: `<script>alert('xss')</script>.com`
- DOMAIN: `test';DROP TABLE users;--.com`

**Expected Result:**
- Input is rejected by validation
- Or input is properly escaped/sanitized
- No script execution occurs

---

## Test Execution Priority

### P0 - Critical (Must Pass)
- TC-001: Valid Domain Name
- TC-002: Invalid Domain Name - Uppercase
- TC-006: Single Disk Selection
- TC-007: Multiple Disk Selection
- TC-017: Save Configuration Button
- TC-030: Localhost-Only Access

### P1 - High (Should Pass)
- TC-003 through TC-005: Domain validation
- TC-008: Root Device Conflict
- TC-010 through TC-012: Certificate options
- TC-013 through TC-016: Checkbox tests
- TC-028: End-to-End First Node

### P2 - Medium (Nice to Have)
- TC-020 through TC-021: Prefilled configuration
- TC-024 through TC-027: UI/UX tests
- TC-029: Additional node setup

### P3 - Low (Future)
- TC-022 through TC-023: Error handling
- TC-026: Responsive design
- TC-031: Input sanitization

## Test Case Format

Each test case YAML file should follow this format:

```yaml
# Test case description and ID
DOMAIN: <value>
CLUSTER_DISKS: <value>
CERT_OPTION: <value>
FIRST_NODE: <boolean>
GPU_NODE: <boolean>

# Optional: For validation failure tests
expected_error: "<error text to verify>"

# Optional: For specific behavior tests
expected_behavior: "<expected behavior description>"
```

## Running Tests

```bash
# Run all UI tests
./run_ui_tests.sh

# Run specific test case
BLOOM_YAML_PATH=/tmp/bloom.yaml go test -v -run TestWebFormE2E/bloom_specific_test.yaml

# Run with verbose output
BLOOM_YAML_PATH=/tmp/bloom.yaml go test -v -run TestWebFormE2E
```

## Test Coverage Goals

- **Form Validation:** 100% of validation rules covered
- **User Workflows:** All primary user journeys tested
- **Error Scenarios:** All expected error conditions tested
- **UI Components:** All interactive elements tested

## Notes

- Test cases are implemented as YAML files matching pattern `bloom_*.yaml`
- Each test case is run as a subtest using Go's `t.Run()`
- Browser automation uses chromedp connecting to remote Chrome on port 9222
- Tests should be idempotent and independent
- Cleanup happens automatically via test script
