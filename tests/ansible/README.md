# Ansible Playbook Tests

Automated tests for cluster-bloom Ansible playbooks, focusing on domain update and RKE2 configuration file manipulation.

## Overview

These tests verify critical file operations that could cause cluster failures if incorrect:
- RKE2 configuration updates (`/etc/rancher/rke2/config.yaml`)
- OIDC authentication configuration (`/etc/rancher/rke2/auth/auth-config.yaml`)
- TLS SAN preservation and extraction
- YAML structure and indentation correctness

### Mock Modules

Tests use **mock Ansible modules** to simulate infrastructure operations that can't run in test containers:

**Mocked modules**:
- `systemd` - Service restarts (no systemd in container)
- `command` - kubectl commands (no Kubernetes cluster)
- `shell` - kubectl commands (mocked), sed/grep (real)

**Real modules**:
- `blockinfile` - File manipulation we're testing
- `copy` - File generation we're testing
- `file` - File deletion (safe in container)

This allows tests to verify file content while skipping infrastructure checks that belong in integration tests.

## Test Structure

```
tests/ansible/
├── unit/                      # Fast, isolated tests (< 5 seconds)
│   ├── test_rke2_config.py   # RKE2 config.yaml manipulation
│   ├── test_auth_config.py   # OIDC auth-config.yaml generation
│   └── ...
├── integration/               # Comprehensive workflow tests (future)
├── fixtures/                  # Test data
├── conftest.py               # Shared pytest fixtures
└── requirements.txt          # Python dependencies
```

## Setup

### Install Dependencies

```bash
cd /home/pwistbac/dev/cluster-bloom
pip install -r tests/ansible/requirements.txt
```

Dependencies:
- `pytest` - Test framework
- `ansible-runner` - Programmatic Ansible execution
- `PyYAML` - YAML parsing and validation
- `pytest-httpserver` - HTTP API mocking

## Running Tests

### Run Tests in Docker (Recommended)

Tests run in Docker containers for isolation and to safely use `/etc/rancher/rke2` paths:

```bash
cd /home/pwistbac/dev/cluster-bloom/tests/ansible
./run_tests_docker.sh
```

Run specific tests:

```bash
./run_tests_docker.sh pytest unit/test_rke2_config.py -v
```

### Run Tests Locally (Advanced)

**Warning**: Tests modify `/etc/rancher/rke2/` - only run on test systems!

```bash
cd tests/ansible
pytest unit/ -v
```

### Run Specific Test File

```bash
pytest unit/test_rke2_config.py -v
```

### Run Specific Test

```bash
pytest unit/test_rke2_config.py::test_blockinfile_preserves_additional_sans -v
```

### Run with Detailed Output

```bash
pytest unit/ -vv --tb=long
```

### Run with Coverage

```bash
pytest unit/ --cov=../../pkg/ansible/runtime/playbooks --cov-report=html
```

Coverage report will be in `htmlcov/index.html`

## Test Categories

### Unit Tests (unit/)

**Fast, isolated tests** that verify individual file operations:

- **test_rke2_config.py**: Tests for `/etc/rancher/rke2/config.yaml` manipulation
  - Preservation of additional TLS SANs
  - YAML indentation correctness
  - Missing marker block handling
  - Shell command SAN extraction

- **test_auth_config.py**: Tests for `/etc/rancher/rke2/auth/auth-config.yaml` generation
  - Template substitution
  - YAML structure (jwt: field placement)
  - claimMappings not nested under issuer
  - Domain with special characters

### Integration Tests (integration/)

**Comprehensive workflow tests** (future implementation):
- Full domain update workflow
- Dry-run mode verification
- Multi-node scenarios

## Writing New Tests

### Using Fixtures

The `conftest.py` file provides reusable fixtures:

```python
def test_my_feature(fake_rke2_root, ansible_runner_factory):
    """Test description"""
    # fake_rke2_root provides a mock /etc/rancher/rke2 directory
    # ansible_runner_factory runs playbooks with custom variables
    
    result = ansible_runner_factory(
        'tasks/update/update_rke2.yaml',
        extravars={
            'NEW_DOMAIN': 'test.com',
            'DRY_RUN': False,
        }
    )
    
    assert result.rc == 0
    
    # Verify file content
    config = yaml.safe_load((fake_rke2_root / 'config.yaml').read_text())
    assert 'expected-value' in config
```

### Available Fixtures

- **playbook_dir**: Path to `pkg/ansible/runtime/playbooks`
- **ansible_runner_factory**: Function to run playbooks programmatically
- **fake_rke2_root**: Mock `/etc/rancher/rke2` directory with default config
- **rke2_config_fixtures**: Multiple config states (clean, multi_san, no_marker, etc.)

## Common Test Patterns

### Testing File Content

```python
config_file = fake_rke2_root / 'config.yaml'
content = config_file.read_text()
assert 'expected-string' in content
```

### Testing YAML Structure

```python
config = yaml.safe_load(config_file.read_text())
assert config['field'] == 'expected-value'
assert isinstance(config['list-field'], list)
```

### Testing Indentation

```python
for line in content.split('\n'):
    if line.strip().startswith('- item'):
        assert line.startswith('  - '), "Should have 2-space indent"
```

### Testing Shell Commands

```python
import subprocess

result = subprocess.run(
    ['sed', '-n', '/pattern/p', str(config_file)],
    capture_output=True,
    text=True
)
assert 'expected' in result.stdout
```

## Bugs These Tests Catch

### Real Bugs Caught

1. **Invalid TLS SANs** (test_san_extraction_filters_node_labels)
   - Shell command read too many lines, included node-label entries
   - Result: kube-apiserver failed with "runtime core not ready"

2. **Missing jwt: Field** (test_auth_config_jwt_field_not_indented)
   - auth-config.yaml missing root-level `jwt:` field
   - Result: "yaml: line 3: mapping values are not allowed"

3. **claimMappings Nested** (test_auth_config_claim_mappings_not_nested)
   - claimMappings incorrectly nested under issuer
   - Result: "unknown field 'jwt[0].issuer.claimMappings'"

4. **Lost Additional SANs** (test_blockinfile_preserves_additional_sans)
   - Additional TLS SANs not preserved during update
   - Result: Custom load balancer DNS names lost, causing connection failures

### Edge Cases Tested

- Missing marker blocks
- Malformed YAML
- Same domain updates (idempotency)
- Dry-run mode (no modifications)
- Domains with hyphens and numbers
- File overwriting (recovery scenarios)

## CI/CD Integration

Tests run automatically in GitHub Actions:

```yaml
# .github/workflows/run-tests.yml
ansible-playbook-tests:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - name: Setup Python
      uses: actions/setup-python@v4
      with:
        python-version: '3.11'
    - name: Install dependencies
      run: pip install -r tests/ansible/requirements.txt
    - name: Run tests
      run: |
        cd tests/ansible
        pytest unit/ -v --tb=short
```

## Troubleshooting

### Tests Fail with "playbook not found"

Ensure you're running from the `tests/ansible/` directory, or use absolute paths.

### Tests Fail with "ansible-runner not found"

Install dependencies: `pip install -r requirements.txt`

### Tests Modify Real System Files

Tests use `tmp_path` fixtures - they NEVER touch real `/etc/rancher/rke2` files. If a test appears to modify system files, the playbook needs to be parameterized to accept a config path override.

### YAML Parsing Errors

If YAML parsing fails in tests, the playbook is generating invalid YAML. Check:
- Indentation (should be 2 spaces)
- Field placement (jwt: at root level, not nested)
- Quotes around string values

## Future Improvements

- [ ] Integration tests with Docker containers
- [ ] Tests for certificate generation
- [ ] Tests for Gitea API integration
- [ ] Tests for dry-run mode across all playbooks
- [ ] Coverage target: 80%+
- [ ] Performance benchmarks for playbook execution time

## References

- [Ansible Runner Documentation](https://ansible-runner.readthedocs.io/)
- [pytest Documentation](https://docs.pytest.org/)
- [RKE2 Configuration Reference](https://docs.rke2.io/reference/server_config)
