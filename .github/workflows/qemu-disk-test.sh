#!/bin/bash
set -e

# Check if required arguments are provided
if [ $# -lt 3 ]; then
    echo "ERROR: Insufficient arguments"
    echo "Usage: $0 <vm-name> <bloom-binary-path> <bloom-yaml-path> [additional-yaml-paths...]"
    echo "Example: $0 nvme-test-vm ./cluster-bloom ./test/bloom.yaml ./test/bloom2.yaml"
    exit 1
fi

VM_NAME="$1"
BLOOM_BINARY="$2"
shift 2
BLOOM_CONFIGS=("$@")

echo "Setting up QEMU VM '$VM_NAME' with 8 NVMe drives (Linux KVM - Clean Setup)..."

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

# Completely remove and recreate working directory
echo "Creating fresh working directory..."
rm -rf "$VM_NAME"
mkdir -p "$VM_NAME"

# Download Ubuntu 24.04 AMD64 cloud image
if [ ! -f noble-server-cloudimg-amd64.img ]; then
    echo "Downloading Ubuntu 24.04 AMD64 cloud image (~700MB)..."
    curl -L --progress-bar -o noble-server-cloudimg-amd64.img \
        https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
fi

# Copy OVMF VARS for writable UEFI variables
echo "Setting up UEFI firmware..."
cp /usr/share/OVMF/OVMF_VARS.fd "$VM_NAME/"

# Create OS disk (40GB)
echo "Creating OS disk..."
qemu-img create -f qcow2 -F qcow2 -b "$(pwd)/noble-server-cloudimg-amd64.img" "$VM_NAME/os-disk.qcow2" 40G

# Create 8 NVMe disk images (1MB each)
echo "Creating 8 NVMe disk images..."
for i in {0..7}; do
    qemu-img create -f raw "$VM_NAME/nvme${i}.img" 1M
done

# Create cloud-init configuration with proper user setup
echo "Creating cloud-init configuration..."
mkdir -p "$VM_NAME/seed-content"

# Generate SSH key if it doesn't exist
if [ ! -f "$VM_NAME/qemu-login" ]; then
    echo "Generating SSH key (qemu-login)..."
    ssh-keygen -t rsa -b 4096 -f "$VM_NAME/qemu-login" -N ""
fi

cat > "$VM_NAME/seed-content/user-data" << EOF
#cloud-config

# Enable password authentication (as fallback)
ssh_pwauth: True
disable_root: false

# Create ubuntu user with SSH key
users:
  - name: ubuntu
    plain_text_passwd: ubuntu
    lock_passwd: false
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    groups: [users, admin, sudo]
    ssh_authorized_keys:
      - $(cat "$VM_NAME/qemu-login.pub")

# Set password explicitly (fallback)
chpasswd:
  list: |
    ubuntu:ubuntu
  expire: False

# Run commands after boot
runcmd:
  - sleep 10
  - echo "ubuntu:ubuntu" | chpasswd
  - echo "System ready at $(date)" > /home/ubuntu/boot-complete.txt
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

# Create ISO seed image
echo "Creating cloud-init seed ISO..."
if command -v mkisofs &> /dev/null; then
    mkisofs -output "$VM_NAME/seed.img" -volid cidata -joliet -rock "$VM_NAME/seed-content/user-data" "$VM_NAME/seed-content/meta-data" 2>/dev/null
elif command -v genisoimage &> /dev/null; then
    genisoimage -output "$VM_NAME/seed.img" -volid cidata -joliet -rock "$VM_NAME/seed-content/user-data" "$VM_NAME/seed-content/meta-data" 2>/dev/null
fi

# Create startup script
cat > "$VM_NAME/start-vm.sh" << STARTEOF
#!/bin/bash
SCRIPT_DIR="\$(cd "\$(dirname "\$0")" && pwd)"
cd "\$SCRIPT_DIR"

echo "Starting x86_64 VM with 8 NVMe devices in background..."
echo "Output will be logged to \$SCRIPT_DIR/startup.log"
echo "Wait ~90 seconds for cloud-init to complete."
echo ""
echo "To monitor boot progress:"
echo "  tail -f \$SCRIPT_DIR/startup.log"
echo ""
echo "To connect via SSH:"
echo "  \$SCRIPT_DIR/ssh-vm.sh"
echo ""

qemu-system-x86_64 \
  -machine q35,accel=kvm \
  -cpu host \
  -smp 2 \
  -m 10G \
  -drive if=pflash,format=raw,readonly=on,file=/usr/share/OVMF/OVMF_CODE.fd \
  -drive if=pflash,format=raw,file="\$SCRIPT_DIR/OVMF_VARS.fd" \
  -drive file=os-disk.qcow2,if=virtio,format=qcow2 \
  -drive file=seed.img,if=virtio,format=raw \
  -drive file=nvme0.img,if=none,id=nvme0,format=raw \
  -device nvme,serial=NVME000001,drive=nvme0 \
  -drive file=nvme1.img,if=none,id=nvme1,format=raw \
  -device nvme,serial=NVME000002,drive=nvme1 \
  -drive file=nvme2.img,if=none,id=nvme2,format=raw \
  -device nvme,serial=NVME000003,drive=nvme2 \
  -drive file=nvme3.img,if=none,id=nvme3,format=raw \
  -device nvme,serial=NVME000004,drive=nvme3 \
  -drive file=nvme4.img,if=none,id=nvme4,format=raw \
  -device nvme,serial=NVME000005,drive=nvme4 \
  -drive file=nvme5.img,if=none,id=nvme5,format=raw \
  -device nvme,serial=NVME000006,drive=nvme5 \
  -drive file=nvme6.img,if=none,id=nvme6,format=raw \
  -device nvme,serial=NVME000007,drive=nvme6 \
  -drive file=nvme7.img,if=none,id=nvme7,format=raw \
  -device nvme,serial=NVME000008,drive=nvme7 \
  -netdev user,id=net0,hostfwd=tcp::2222-:22 \
  -device virtio-net-pci,netdev=net0 \
  -nographic > "\$SCRIPT_DIR/startup.log" 2>&1 &

VM_PID=\$!
echo "VM started with PID \$VM_PID"
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
cat > "$VM_NAME/stop-vm.sh" << 'EOF'
#!/bin/bash
killall qemu-system-x86_64
EOF

chmod +x "$VM_NAME/stop-vm.sh"

# Create SSH helper script
cat > "$VM_NAME/ssh-vm.sh" << EOF
#!/bin/bash
cd "\$(dirname "\$0")"
ssh -i qemu-login -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -p 2222 ubuntu@localhost
EOF

chmod +x "$VM_NAME/ssh-vm.sh"

echo ""
echo "=========================================="
echo "✓ Clean Setup Complete!"
echo "=========================================="
echo ""
echo "Step 1: Start the VM"
echo "  cd $VM_NAME && ./start-vm.sh"
echo ""
echo "Step 2: Wait ~90 seconds for boot + cloud-init"
echo ""
echo "Step 3: In a NEW TERMINAL, SSH to the VM:"
echo "  cd $VM_NAME && ./ssh-vm.sh"
echo "  (passwordless SSH with qemu-login key)"
echo ""
echo "Step 4: Inside VM, verify NVMe devices:"
echo "  lsblk"
echo "  ls -l /dev/nvme*"
echo "  cat ~/boot-complete.txt"
echo ""
echo "To stop the VM:"
echo "  cd $VM_NAME && ./stop-vm.sh"
echo "  or press Ctrl+A then X in the console"
echo ""

# Start the VM automatically
echo "Starting the VM..."
bash "$VM_NAME/start-vm.sh"

# Verify bloom binary exists
if [ ! -f "$BLOOM_BINARY" ]; then
    echo "ERROR: Bloom binary not found at $BLOOM_BINARY"
    exit 1
fi

# Verify all config files exist
for config in "${BLOOM_CONFIGS[@]}"; do
    if [ ! -f "$config" ]; then
        echo "ERROR: Config file not found at $config"
        exit 1
    fi
done

echo ""
echo "Copying bloom binary and config files to VM..."

# Wait a bit more to ensure VM is fully ready for SSH
sleep 10

# Build file list for scp - bloom binary first
FILES_TO_COPY="$BLOOM_BINARY"

# Add all config files
for config in "${BLOOM_CONFIGS[@]}"; do
    FILES_TO_COPY="$FILES_TO_COPY $config"
done

# Copy all files at once
scp -i "$VM_NAME/qemu-login" -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -P 2222 $FILES_TO_COPY ubuntu@localhost:~/

echo "Files copied successfully"
echo "Making bloom executable and running test..."

# Build the bloom test command with all config files
BLOOM_BINARY_NAME=$(basename "$BLOOM_BINARY")
CONFIG_NAMES=""
for config in "${BLOOM_CONFIGS[@]}"; do
    CONFIG_NAMES="$CONFIG_NAMES $(basename "$config")"
done

# Make bloom executable and run the test
ssh -i "$VM_NAME/qemu-login" -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -p 2222 ubuntu@localhost chmod +x $BLOOM_BINARY_NAME
echo "Running: sudo ./$BLOOM_BINARY_NAME test$CONFIG_NAMES"
ssh -i "$VM_NAME/qemu-login" -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -p 2222 ubuntu@localhost sudo ./$BLOOM_BINARY_NAME test$CONFIG_NAMES | tee "$VM_NAME-test-results.yaml"

echo ""
echo "Test execution completed"
echo "Results saved to: test-results.yaml"

# Clean up VM
echo ""
echo "Cleaning up VM..."
bash "$VM_NAME/stop-vm.sh" || killall qemu-system-x86_64 2>/dev/null || true
sleep 2

echo "Removing $VM_NAME directory..."
rm -rf "$VM_NAME"

echo ""
echo "✓ VM deleted and cleaned up"
