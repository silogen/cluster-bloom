# End-to-End (E2E) Tests

End-to-end tests for Cluster-Bloom that validate complete workflows in real or virtualized environments.

## Directory Structure

```
tests/e2e/
├── README.md                   # This file
├── qemu/                       # QEMU-based VM infrastructure
│   ├── qemu-disk-test.sh       # Script for testing with QEMU VMs
│   └── prepull-images.sh       # Pre-pull container images for faster testing
└── nvme_8_disks/               # 8-disk NVMe test configuration
    ├── profile.yaml            # VM hardware profile (8 CPUs, 10GB RAM, 8 NVMe disks)
    ├── first_bloom.yaml        # Initial cluster setup configuration
    └── rebloom_bloom.yaml      # Re-bloom test configuration
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

#### Profile-Based Testing (Recommended)
The new profile-based approach allows configuring VM hardware and multiple test scenarios in YAML:

```bash
# Run tests using a profile configuration
bash tests/e2e/qemu/qemu-disk-test.sh <vm-name> <profile.yaml> <bloom-binary>

# Example: 8-disk NVMe test
bash tests/e2e/qemu/qemu-disk-test.sh nvme-test-vm tests/e2e/nvme_8_disks/profile.yaml ./cluster-bloom
```

The profile.yaml defines:
- VM hardware (CPUs, memory, root disk size)
- Disk configuration (count, type, size, format)
- Bloom test configurations to run sequentially

#### Legacy Single Configuration (Still Supported)
```bash
# Run with a single bloom.yaml configuration
bash tests/e2e/qemu/qemu-disk-test.sh <vm-name> <bloom.yaml> <bloom-binary>

# Example
bash tests/e2e/qemu/qemu-disk-test.sh nvme-test-vm tests/e2e/nvme_8_disks/first_bloom.yaml ./cluster-bloom
```

### What the QEMU Test Does

1. **VM Setup**
   - Creates a QEMU VM with configurable hardware (CPUs, memory, disks)
   - Supports multiple disk types: NVMe, VirtIO, SCSI, IDE
   - Uses Ubuntu 24.04 cloud image (cached at `~/ci/` for speed)
   - Configures cloud-init for passwordless SSH access
   - Sets up UEFI boot with OVMF firmware

2. **Test Execution**
   - Copies bloom binary and configuration files to VM
   - Runs bloom test command for each configuration in profile
   - Captures structured test results in YAML format
   - Preserves VM for debugging (cleanup is disabled by default)

3. **Results & Debugging**
   - Test results saved to `<vm-name>-test-results.yaml`
   - VM remains running for post-test inspection
   - Access VM via: `cd <vm-name> && ./ssh-vm.sh`
   - View logs: `tail -f <vm-name>/startup.log`
   - Stop VM: `cd <vm-name> && ./stop-vm.sh`

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

Example workflow integration:

```yaml
- name: Build Cluster-Bloom
  run: go build -v .

- name: Run QEMU E2E Tests
  run: |
    bash tests/e2e/qemu/qemu-disk-test.sh nvme-test-vm \
      tests/e2e/nvme_8_disks/profile.yaml \
      ./cluster-bloom

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

### Profile-Based Test Case

1. Create a new test directory:
   ```bash
   mkdir tests/e2e/my_test_case
   cd tests/e2e/my_test_case
   ```

2. Create `profile.yaml` with VM hardware configuration:
   ```yaml
   # VM hardware configuration
   cpus: 4
   memory: 8G
   root_disk_size: 50G

   # Bloom configurations to run
   bloom_configs:
     - first_bloom.yaml
     - second_bloom.yaml

   # Disk configuration
   disks:
     - size: 10G
       type: nvme
       format: raw
     - size: 10G
       type: nvme
       format: raw
   ```

3. Create bloom configuration files (`first_bloom.yaml`, etc.):
   ```yaml
   CLUSTERFORGE_RELEASE: none
   DOMAIN: test.silogen.ai
   FIRST_NODE: true
   GPU_NODE: false
   CLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1
   NO_DISKS_FOR_CLUSTER: false
   USE_CERT_MANAGER: false
   CERT_OPTION: generate
   PRELOAD_IMAGES: ""
   ```

4. Run the test:
   ```bash
   bash tests/e2e/qemu/qemu-disk-test.sh my-vm \
     tests/e2e/my_test_case/profile.yaml \
     ./cluster-bloom
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
