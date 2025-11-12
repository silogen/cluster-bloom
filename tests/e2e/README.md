# End-to-End (E2E) Tests

End-to-end tests for Cluster-Bloom that validate complete workflows in real or virtualized environments.

## Directory Structure

```
tests/e2e/
├── README.md                   # This file
├── qemu/                       # QEMU-based VM tests
│   └── qemu-disk-test.sh       # Script for testing with QEMU VMs
└── test_case_1/                # Test case configurations
    └── bloom.yaml              # Configuration for test case 1
```

## QEMU VM Tests

### Prerequisites

The QEMU disk test script requires the following dependencies:

#### Ubuntu/Debian
```bash
# Install QEMU and KVM
sudo apt update
sudo apt install qemu-system-x86 qemu-utils qemu-kvm

# Install ISO creation tools (required for cloud-init)
sudo apt install genisoimage

# Optional: Install KVM acceleration support (recommended)
sudo apt install qemu-kvm libvirt-daemon-system libvirt-clients bridge-utils

# Install other dependencies
sudo apt install curl ssh-keygen
```

#### Verify Installation
```bash
qemu-system-x86_64 --version
mkisofs --version || genisoimage --version
```

### OVMF Firmware

The script requires OVMF (UEFI firmware for QEMU):

```bash
sudo apt install ovmf
```

This provides:
- `/usr/share/OVMF/OVMF_CODE.fd` or `/usr/share/OVMF/OVMF_CODE_4M.fd` - Read-only UEFI firmware
- `/usr/share/OVMF/OVMF_VARS.fd` or `/usr/share/OVMF/OVMF_VARS_4M.fd` - UEFI variables template

### KVM Permissions

To use KVM acceleration (required for reasonable performance), add your user to the `kvm` group:

```bash
# Add user to kvm group
sudo usermod -a -G kvm $USER

# Logout and login, or run this in current shell:
newgrp kvm

# Verify group membership
groups | grep kvm
```

**Note:** Without KVM permissions, tests will fail with "Permission denied" errors.

### Running QEMU Tests

#### Basic Usage
```bash
# Run with a single test configuration
bash tests/e2e/qemu/qemu-disk-test.sh <vm-name> <bloom-binary> <bloom-yaml>

# Example
bash tests/e2e/qemu/qemu-disk-test.sh nvme-test-vm dist/bloom tests/e2e/test_case_1/bloom.yaml
```

#### Multiple Test Configurations
```bash
# Run multiple configurations sequentially
bash tests/e2e/qemu/qemu-disk-test.sh nvme-test-vm dist/bloom \
    tests/e2e/test_case_1/bloom.yaml \
    tests/e2e/test_case_2/bloom.yaml
```

### What the QEMU Test Does

1. **VM Setup**
   - Creates a QEMU VM with 8 NVMe drives
   - Uses Ubuntu 24.04 cloud image
   - Configures cloud-init for SSH access
   - Sets up UEFI boot with OVMF

2. **Test Execution**
   - Copies bloom binary to VM
   - Runs bloom with provided configuration(s)
   - Captures test results
   - Outputs structured YAML results

3. **Cleanup**
   - Stops QEMU VM
   - Preserves test results in `<vm-name>-test-results.yaml`

### Test Results

After running, check the results file:
```bash
cat nvme-test-vm-test-results.yaml
```

Results include:
- Overall success/failure status
- Individual step results
- Duration metrics
- Error messages (if any)

### CI/CD Integration

The workflow in `.github/workflows/run-tests.yml` includes QEMU tests:

```yaml
- name: QEMU Disk Test
  run: |
    bash tests/e2e/qemu/qemu-disk-test.sh nvme-test-vm dist/bloom tests/e2e/test_case_1/bloom.yaml

- name: Check Test Results
  run: |
    SUCCESS=$(yq '.overall_summary.success' nvme-test-vm-test-results.yaml)
    if [ "$SUCCESS" != "true" ]; then
      echo "❌ Tests failed"
      cat nvme-test-vm-test-results.yaml
      exit 1
    fi
```

## Creating New Test Cases

1. Create a new directory under `tests/e2e/`:
   ```bash
   mkdir tests/e2e/test_case_2
   ```

2. Add a `bloom.yaml` configuration:
   ```yaml
   DOMAIN: test.local
   FIRST_NODE: true
   GPU_NODE: false
   CLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1
   CERT_OPTION: generate
   ```

3. Run the test:
   ```bash
   bash tests/e2e/qemu/qemu-disk-test.sh test-vm dist/bloom tests/e2e/test_case_2/bloom.yaml
   ```

## Troubleshooting

### QEMU not found
```bash
sudo apt install qemu-system-x86 qemu-utils
```

### mkisofs not found
```bash
sudo apt install genisoimage
```

### OVMF files not found
```bash
sudo apt install ovmf
ls /usr/share/OVMF/
```

### VM fails to boot
- Check KVM support: `kvm-ok` (install with `sudo apt install cpu-checker`)
- Verify user is in `kvm` group: `sudo usermod -a -G kvm $USER` (logout/login required)
- Check QEMU process logs in the VM directory

### SSH connection issues
- VM may still be booting (wait 30-60 seconds)
- Check SSH key permissions: `chmod 600 <vm-name>/qemu-login`
- Verify cloud-init completed: Check VM console output

### Disk test failures
- Ensure NVMe devices are properly created
- Check disk permissions in VM
- Verify bloom binary has correct permissions
- Review test results YAML for specific errors

## Performance Notes

- QEMU tests can take 2-5 minutes depending on hardware
- KVM acceleration significantly improves performance
- Cloud image is cached at `/home/ubuntu/ci/` to speed up repeated runs
- Each test creates fresh disk images to ensure isolation
