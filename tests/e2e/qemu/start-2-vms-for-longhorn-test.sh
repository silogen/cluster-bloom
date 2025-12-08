#!/bin/bash
set -e

echo "Starting 2 VMs for Longhorn volume migration testing"
echo "Each VM will have 2x 1GB NVMe disks"
echo ""

# Check dependencies
if ! command -v qemu-system-x86_64 &> /dev/null; then
    echo "ERROR: QEMU not found."
    exit 1
fi

if ! command -v mkisofs &> /dev/null && ! command -v genisoimage &> /dev/null; then
    echo "ERROR: mkisofs not found."
    exit 1
fi

# Kill any existing QEMU processes
echo "Cleaning up any existing QEMU processes..."
killall qemu-system-x86_64 2>/dev/null && echo "✓ Killed existing QEMU" || true
sleep 2

# Download Ubuntu cloud image if not cached
CI_IMAGE_CACHE="$HOME/ci/noble-server-cloudimg-amd64.img"
if [ ! -f "$CI_IMAGE_CACHE" ]; then
    echo "Downloading Ubuntu 24.04 AMD64 cloud image to cache (~700MB)..."
    mkdir -p "$(dirname "$CI_IMAGE_CACHE")"
    curl -L -s -o "$CI_IMAGE_CACHE" \
        https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
fi

# Find OVMF firmware files
if [ -f /usr/share/OVMF/OVMF_CODE.fd ] && [ -f /usr/share/OVMF/OVMF_VARS.fd ]; then
    OVMF_CODE="/usr/share/OVMF/OVMF_CODE.fd"
    OVMF_VARS="/usr/share/OVMF/OVMF_VARS.fd"
elif [ -f /usr/share/OVMF/OVMF_CODE_4M.fd ] && [ -f /usr/share/OVMF/OVMF_VARS_4M.fd ]; then
    OVMF_CODE="/usr/share/OVMF/OVMF_CODE_4M.fd"
    OVMF_VARS="/usr/share/OVMF/OVMF_VARS_4M.fd"
else
    echo "ERROR: OVMF firmware files not found. Install ovmf package."
    exit 1
fi

# Shared socket for VM-to-VM networking
SHARED_SOCKET="/tmp/qemu-vlan-longhorn.sock"
rm -f "$SHARED_SOCKET"

# Function to create a VM
create_vm() {
    local VM_NAME="$1"
    local SSH_PORT="$2"
    local VM_IP="$3"
    local VM_CPUS="2"
    local VM_MEMORY="4G"
    local ROOT_DISK_SIZE="50G"

    echo "=========================================="
    echo "Creating VM: $VM_NAME (SSH port: $SSH_PORT)"
    echo "=========================================="

    # Remove and recreate working directory
    rm -rf "$VM_NAME"
    mkdir -p "$VM_NAME"

    # Copy Ubuntu cloud image
    echo "Copying Ubuntu cloud image..."
    cp "$CI_IMAGE_CACHE" "$VM_NAME/noble-server-cloudimg-amd64.img"

    # Copy OVMF vars
    echo "Setting up UEFI firmware..."
    cp "$OVMF_VARS" "$VM_NAME/OVMF_VARS.fd"

    # Create OS disk
    echo "Creating OS disk ($ROOT_DISK_SIZE)..."
    qemu-img create -f qcow2 -F qcow2 -b "noble-server-cloudimg-amd64.img" "$VM_NAME/os-disk.qcow2" "$ROOT_DISK_SIZE"

    # Create 2x 1GB NVMe disks
    echo "Creating 2x 1GB NVMe disks..."
    qemu-img create -f raw "$VM_NAME/nvme0.img" 1G
    qemu-img create -f raw "$VM_NAME/nvme1.img" 1G

    # Generate SSH key
    echo "Generating SSH key..."
    ssh-keygen -t rsa -b 4096 -f "$VM_NAME/qemu-login" -N ""

    # Create cloud-init configuration
    echo "Creating cloud-init configuration..."
    mkdir -p "$VM_NAME/seed-content"

    cat > "$VM_NAME/seed-content/user-data" << EOF
#cloud-config

ssh_pwauth: True
disable_root: false

users:
  - name: ubuntu
    plain_text_passwd: ubuntu
    lock_passwd: false
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    groups: [users, admin, sudo]
    ssh_authorized_keys:
      - $(cat "$VM_NAME/qemu-login.pub")

chpasswd:
  list: |
    ubuntu:ubuntu
  expire: False

write_files:
  - path: /etc/netplan/60-cluster-net.yaml
    content: |
      network:
        version: 2
        ethernets:
          enp0s5:
            addresses:
              - $VM_IP/24
    permissions: '0600'

runcmd:
  - sleep 10
  - echo "ubuntu:ubuntu" | chpasswd
  - netplan apply
  - echo "System ready at \$(date)" > /home/ubuntu/boot-complete.txt
  - echo "" >> /home/ubuntu/boot-complete.txt
  - echo "=== Network Interfaces ===" >> /home/ubuntu/boot-complete.txt
  - ip addr >> /home/ubuntu/boot-complete.txt
  - echo "" >> /home/ubuntu/boot-complete.txt
  - echo "=== NVMe Devices ===" >> /home/ubuntu/boot-complete.txt
  - lsblk >> /home/ubuntu/boot-complete.txt
  - echo "" >> /home/ubuntu/boot-complete.txt
  - echo "=== Device List ===" >> /home/ubuntu/boot-complete.txt
  - ls -l /dev/nvme* >> /home/ubuntu/boot-complete.txt 2>&1 || echo "No /dev/nvme* found" >> /home/ubuntu/boot-complete.txt
  - chown ubuntu:ubuntu /home/ubuntu/boot-complete.txt

final_message: "Cloud-init complete! System is ready."
EOF

    cat > "$VM_NAME/seed-content/meta-data" << EOF
instance-id: $VM_NAME-001
local-hostname: $VM_NAME
EOF

    # Create cloud-init ISO
    echo "Creating cloud-init seed ISO..."
    if command -v mkisofs &> /dev/null; then
        mkisofs -output "$VM_NAME/seed.img" -volid cidata -joliet -rock "$VM_NAME/seed-content/user-data" "$VM_NAME/seed-content/meta-data" 2>/dev/null
    elif command -v genisoimage &> /dev/null; then
        genisoimage -output "$VM_NAME/seed.img" -volid cidata -joliet -rock "$VM_NAME/seed-content/user-data" "$VM_NAME/seed-content/meta-data" 2>/dev/null
    fi

    # Create start script
    cat > "$VM_NAME/start-vm.sh" << STARTEOF
#!/bin/bash
SCRIPT_DIR="\$(cd "\$(dirname "\$0")" && pwd)"
cd "\$SCRIPT_DIR"

echo "Starting $VM_NAME in background..."
echo "Output: \$SCRIPT_DIR/startup.log"

qemu-system-x86_64 \\
  -machine q35,accel=kvm \\
  -cpu host \\
  -smp $VM_CPUS \\
  -m $VM_MEMORY \\
  -drive if=pflash,format=raw,readonly=on,file=$OVMF_CODE \\
  -drive if=pflash,format=raw,file="\$SCRIPT_DIR/OVMF_VARS.fd" \\
  -drive file=os-disk.qcow2,if=virtio,format=qcow2 \\
  -drive file=seed.img,if=virtio,format=raw \\
  -drive file=nvme0.img,if=none,id=nvme0,format=raw \\
  -device nvme,serial=NVME00000001,drive=nvme0 \\
  -drive file=nvme1.img,if=none,id=nvme1,format=raw \\
  -device nvme,serial=NVME00000002,drive=nvme1 \\
  -netdev user,id=net0,hostfwd=tcp::$SSH_PORT-:22 \\
  -device virtio-net-pci,netdev=net0 \\
  -netdev socket,id=net1,mcast=230.0.0.1:1234 \\
  -device virtio-net-pci,netdev=net1,mac=52:54:00:12:34:$(printf '%02x' $((SSH_PORT - 2220))) \\
  -nographic > "\$SCRIPT_DIR/startup.log" 2>&1 &

VM_PID=\$!
echo "VM started with PID \$VM_PID (SSH port: $SSH_PORT)"
echo "Waiting for login prompt (timeout: 2 minutes)..."

elapsed=0
while [ \$elapsed -lt 120 ]; do
  if grep -q "$VM_NAME login:" "\$SCRIPT_DIR/startup.log" 2>/dev/null; then
    echo "✓ VM is ready! (login prompt found after \${elapsed}s)"
    exit 0
  fi
  sleep 2
  elapsed=\$((elapsed + 2))
done

echo "✓ Timeout reached (2 minutes). VM may still be booting."
echo "Check logs: tail -f \$SCRIPT_DIR/startup.log"
STARTEOF

    chmod +x "$VM_NAME/start-vm.sh"

    # Create stop script
    cat > "$VM_NAME/stop-vm.sh" << 'STOPEOF'
#!/bin/bash
killall qemu-system-x86_64
STOPEOF

    chmod +x "$VM_NAME/stop-vm.sh"

    # Create SSH helper
    cat > "$VM_NAME/ssh-vm.sh" << SSHEOF
#!/bin/bash
cd "\$(dirname "\$0")"
ssh -i qemu-login -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -p $SSH_PORT ubuntu@localhost
SSHEOF

    chmod +x "$VM_NAME/ssh-vm.sh"

    echo "✓ $VM_NAME setup complete"
    echo ""
}

# Create both VMs
create_vm "longhorn-node1" "2222" "10.100.100.11"
create_vm "longhorn-node2" "2223" "10.100.100.12"

echo "=========================================="
echo "✓ Both VMs created successfully!"
echo "=========================================="
echo ""
echo "VM 1: longhorn-node1 (SSH port 2222, IP: 10.100.100.11)"
echo "  cd longhorn-node1 && ./start-vm.sh"
echo "  cd longhorn-node1 && ./ssh-vm.sh"
echo ""
echo "VM 2: longhorn-node2 (SSH port 2223, IP: 10.100.100.12)"
echo "  cd longhorn-node2 && ./start-vm.sh"
echo "  cd longhorn-node2 && ./ssh-vm.sh"
echo ""
echo "VMs can communicate with each other on 10.100.100.0/24 network"
echo "  From node1: ssh ubuntu@10.100.100.12"
echo "  From node2: ssh ubuntu@10.100.100.11"
echo ""
echo "To stop all VMs:"
echo "  killall qemu-system-x86_64"
echo ""
echo "Starting both VMs now..."
echo ""

# Start both VMs
(cd longhorn-node1 && ./start-vm.sh) &
NODE1_PID=$!

sleep 5

(cd longhorn-node2 && ./start-vm.sh) &
NODE2_PID=$!

echo ""
echo "=========================================="
echo "✓ Both VMs are starting..."
echo "=========================================="
echo "longhorn-node1: SSH on port 2222, IP: 10.100.100.11"
echo "longhorn-node2: SSH on port 2223, IP: 10.100.100.12"
echo ""
echo "Wait ~90 seconds for cloud-init to complete, then:"
echo "  cd longhorn-node1 && ./ssh-vm.sh"
echo "  cd longhorn-node2 && ./ssh-vm.sh"
echo ""
echo "Test VM-to-VM connectivity:"
echo "  From node1: ping 10.100.100.12"
echo "  From node2: ping 10.100.100.11"
echo ""
