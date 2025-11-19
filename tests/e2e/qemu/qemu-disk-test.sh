#!/bin/bash
set -e

# Check if required arguments are provided
if [ $# -lt 3 ]; then
    echo "ERROR: Insufficient arguments"
    echo "Usage: $0 <vm-name> <profile-yaml> <bloom-binary-path>"
    echo "Example: $0 nvme-test-vm ./vm-profile.yaml ./cluster-bloom"
    echo ""
    echo "Note: bloom.yaml config files are now specified in the profile.yaml under 'bloom_configs' key"
    exit 1
fi

VM_NAME="$1"
PROFILE_YAML="$2"
BLOOM_BINARY="$3"

# Verify profile file exists
if [ ! -f "$PROFILE_YAML" ]; then
    echo "ERROR: Profile YAML not found at $PROFILE_YAML"
    exit 1
fi

# Parse profile YAML using basic grep/awk (works without yq)
VM_CPUS=$(grep "^cpus:" "$PROFILE_YAML" | awk '{print $2}' | tr -d '"' || echo "2")
VM_MEMORY=$(grep "^memory:" "$PROFILE_YAML" | awk '{print $2}' | tr -d '"' || echo "10G")
ROOT_DISK_SIZE=$(grep "^root_disk_size:" "$PROFILE_YAML" | awk '{print $2}' | tr -d '"' || echo "100G")
DISK_COUNT=$(grep -A 100 "^disks:" "$PROFILE_YAML" | grep "  - size:" | wc -l)

# Parse bloom config files from profile
BLOOM_CONFIGS=()
while IFS= read -r line; do
    # Stop if we hit another top-level key (no leading spaces)
    if [[ "$line" =~ ^[a-zA-Z] ]]; then
        break
    fi
    # Extract config path after "  - " or "- " prefix (only direct list items)
    if [[ "$line" =~ ^[[:space:]]*-[[:space:]]+ ]] && [[ ! "$line" =~ : ]]; then
        config=$(echo "$line" | sed 's/^[[:space:]]*-[[:space:]]*//' | tr -d '"' | tr -d "'")
        if [ -n "$config" ]; then
            # Resolve relative paths from profile directory
            PROFILE_DIR=$(dirname "$PROFILE_YAML")
            if [[ "$config" != /* ]]; then
                config="$PROFILE_DIR/$config"
            fi
            BLOOM_CONFIGS+=("$config")
        fi
    fi
done < <(grep -A 100 "^bloom_configs:" "$PROFILE_YAML" | tail -n +2)

# Verify all bloom config files exist
if [ ${#BLOOM_CONFIGS[@]} -eq 0 ]; then
    echo "ERROR: No bloom_configs specified in $PROFILE_YAML"
    exit 1
fi

for config in "${BLOOM_CONFIGS[@]}"; do
    if [ ! -f "$config" ]; then
        echo "ERROR: Bloom config file not found at $config"
        exit 1
    fi
done

echo "Bloom config files (${#BLOOM_CONFIGS[@]}):"
for config in "${BLOOM_CONFIGS[@]}"; do
    echo "  - $config"
done

# Parse individual disk configurations
DISK_SIZES=()
DISK_TYPES=()
DISK_FORMATS=()

if [ $DISK_COUNT -gt 0 ]; then
    while IFS= read -r line; do
        DISK_SIZES+=("$(echo "$line" | awk '{print $3}' | tr -d '"')")
    done < <(grep -A 100 "^disks:" "$PROFILE_YAML" | grep "  - size:")

    while IFS= read -r line; do
        DISK_TYPES+=("$(echo "$line" | awk '{print $2}' | tr -d '"')")
    done < <(grep -A 100 "^disks:" "$PROFILE_YAML" | grep "    type:")

    while IFS= read -r line; do
        DISK_FORMATS+=("$(echo "$line" | awk '{print $2}' | tr -d '"')")
    done < <(grep -A 100 "^disks:" "$PROFILE_YAML" | grep "    format:")
else
    # Default to 8 NVMe drives if none specified
    DISK_COUNT=8
    for i in {0..7}; do
        DISK_SIZES+=("1M")
        DISK_TYPES+=("nvme")
        DISK_FORMATS+=("raw")
    done
fi

echo "VM Profile Configuration:"
echo "  CPUs: $VM_CPUS"
echo "  Memory: $VM_MEMORY"
echo "  Root disk size: $ROOT_DISK_SIZE"
echo "  Number of disks: $DISK_COUNT"

echo "Setting up QEMU VM '$VM_NAME' with $DISK_COUNT drives (Linux KVM - Clean Setup)..."

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

# Download or copy Ubuntu 24.04 AMD64 cloud image
CI_IMAGE_CACHE="$HOME/ci/noble-server-cloudimg-amd64.img"
if [ ! -f "$CI_IMAGE_CACHE" ]; then
    echo "Downloading Ubuntu 24.04 AMD64 cloud image to cache (~700MB)..."
    mkdir -p "$(dirname "$CI_IMAGE_CACHE")"
    curl -L -s -o "$CI_IMAGE_CACHE" \
        https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img
fi
echo "Copying Ubuntu cloud image to VM directory..."
cp "$CI_IMAGE_CACHE" "$VM_NAME/noble-server-cloudimg-amd64.img"

# Copy OVMF files for UEFI boot
echo "Setting up UEFI firmware..."
# Try different OVMF file locations (varies by distro/version)
if [ -f /usr/share/OVMF/OVMF_CODE.fd ] && [ -f /usr/share/OVMF/OVMF_VARS.fd ]; then
    OVMF_CODE="/usr/share/OVMF/OVMF_CODE.fd"
    cp /usr/share/OVMF/OVMF_VARS.fd "$VM_NAME/"
elif [ -f /usr/share/OVMF/OVMF_CODE_4M.fd ] && [ -f /usr/share/OVMF/OVMF_VARS_4M.fd ]; then
    OVMF_CODE="/usr/share/OVMF/OVMF_CODE_4M.fd"
    cp /usr/share/OVMF/OVMF_VARS_4M.fd "$VM_NAME/OVMF_VARS.fd"
else
    echo "ERROR: OVMF firmware files not found. Install ovmf package."
    exit 1
fi

# Create OS disk with size from profile
echo "Creating OS disk ($ROOT_DISK_SIZE)..."
qemu-img create -f qcow2 -F qcow2 -b "noble-server-cloudimg-amd64.img" "$VM_NAME/os-disk.qcow2" "$ROOT_DISK_SIZE"

# Create disk images based on profile
echo "Creating $DISK_COUNT disk images from profile..."
for i in $(seq 0 $((DISK_COUNT - 1))); do
    DISK_SIZE="${DISK_SIZES[$i]}"
    DISK_TYPE="${DISK_TYPES[$i]}"
    DISK_FORMAT="${DISK_FORMATS[$i]}"
    echo "  Creating disk $i: type=$DISK_TYPE, size=$DISK_SIZE, format=$DISK_FORMAT"
    qemu-img create -f "$DISK_FORMAT" "$VM_NAME/${DISK_TYPE}${i}.img" "$DISK_SIZE"
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

# Build QEMU disk arguments first
QEMU_DISK_ARGS=""
for i in $(seq 0 $((DISK_COUNT - 1))); do
    DISK_TYPE="${DISK_TYPES[$i]}"
    DISK_FORMAT="${DISK_FORMATS[$i]}"
    SERIAL_NUM=$(printf "%08d" $((i + 1)))

    if [ "$DISK_TYPE" = "nvme" ]; then
        QEMU_DISK_ARGS="${QEMU_DISK_ARGS}  -drive file=${DISK_TYPE}${i}.img,if=none,id=${DISK_TYPE}${i},format=${DISK_FORMAT} \\
  -device nvme,serial=NVME${SERIAL_NUM},drive=${DISK_TYPE}${i} \\
"
    elif [ "$DISK_TYPE" = "virtio" ]; then
        QEMU_DISK_ARGS="${QEMU_DISK_ARGS}  -drive file=${DISK_TYPE}${i}.img,if=virtio,format=${DISK_FORMAT} \\
"
    elif [ "$DISK_TYPE" = "scsi" ]; then
        QEMU_DISK_ARGS="${QEMU_DISK_ARGS}  -drive file=${DISK_TYPE}${i}.img,if=none,id=${DISK_TYPE}${i},format=${DISK_FORMAT} \\
  -device scsi-hd,drive=${DISK_TYPE}${i} \\
"
    else
        QEMU_DISK_ARGS="${QEMU_DISK_ARGS}  -drive file=${DISK_TYPE}${i}.img,if=ide,format=${DISK_FORMAT} \\
"
    fi
done

# Create startup script with direct variable expansion
cat > "$VM_NAME/start-vm.sh" << STARTEOF
#!/bin/bash
SCRIPT_DIR="\$(cd "\$(dirname "\$0")" && pwd)"
cd "\$SCRIPT_DIR"

echo "Starting x86_64 VM with $DISK_COUNT devices in background..."
echo "Output will be logged to \$SCRIPT_DIR/startup.log"
echo "Wait ~90 seconds for cloud-init to complete."
echo ""
echo "To monitor boot progress:"
echo "  tail -f \$SCRIPT_DIR/startup.log"
echo ""
echo "To connect via SSH:"
echo "  \$SCRIPT_DIR/ssh-vm.sh"
echo ""

qemu-system-x86_64 \\
  -machine q35,accel=kvm \\
  -cpu host \\
  -smp $VM_CPUS \\
  -m $VM_MEMORY \\
  -drive if=pflash,format=raw,readonly=on,file=$OVMF_CODE \\
  -drive if=pflash,format=raw,file="\$SCRIPT_DIR/OVMF_VARS.fd" \\
  -drive file=os-disk.qcow2,if=virtio,format=qcow2 \\
  -drive file=seed.img,if=virtio,format=raw \\
$QEMU_DISK_ARGS  -netdev user,id=net0,hostfwd=tcp::2222-:22 \\
  -device virtio-net-pci,netdev=net0 \\
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

# Config files already verified earlier in the script

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

# # Clean up VM
# echo ""
# echo "Cleaning up VM..."
# bash "$VM_NAME/stop-vm.sh" || killall qemu-system-x86_64 2>/dev/null || true
# sleep 2

# echo "Removing $VM_NAME directory..."
# rm -rf "$VM_NAME"

# echo ""
# echo "✓ VM deleted and cleaned up"
