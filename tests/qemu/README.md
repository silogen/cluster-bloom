# QEMU Testing for ClusterBloom

Quick automated testing of ClusterBloom in isolated QEMU VMs.

## Usage

```bash
# Run test with 2 NVMe drives
./qemu/manual-qemu-test.sh <vm-name> <profile-yaml> <bloom-binary> <bloom-config>
./qemu/manual-qemu-test.sh qemu-test-vm ./qemu/profile_2_nvme.yaml ./bloom ./qemu/bloom.yaml

# SSH into running VM for manual verification
cd qemu-test-vm && ./ssh-vm.sh

# Stop VM when done
cd qemu-test-vm && ./stop-vm.sh
```

**Arguments:**
- `vm-name`: Directory name for VM files (e.g., qemu-test-vm)
- `profile-yaml`: VM hardware config (CPU, RAM, disks)  
- `bloom-binary`: Path to ClusterBloom executable
- `bloom-config`: ClusterBloom YAML configuration file