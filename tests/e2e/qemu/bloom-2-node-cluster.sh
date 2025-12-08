#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
WORK_DIR="$(pwd)"

echo "=========================================="
echo "Deploying 2-node RKE2 cluster with Longhorn"
echo "=========================================="
echo ""
echo "Working directory: $WORK_DIR"
echo ""

# Check that VM directories exist in current working directory
if [ ! -d "$WORK_DIR/longhorn-node1" ]; then
    echo "ERROR: longhorn-node1 directory not found in current directory"
    echo "Run this script from the directory containing longhorn-node1 and longhorn-node2"
    exit 1
fi

if [ ! -d "$WORK_DIR/longhorn-node2" ]; then
    echo "ERROR: longhorn-node2 directory not found in current directory"
    echo "Run this script from the directory containing longhorn-node1 and longhorn-node2"
    exit 1
fi

# Check that VMs are running
if ! pgrep -f "longhorn-node1" > /dev/null; then
    echo "ERROR: longhorn-node1 VM is not running"
    echo "Start it with: cd longhorn-node1 && ./start-vm.sh"
    exit 1
fi

if ! pgrep -f "longhorn-node2" > /dev/null; then
    echo "ERROR: longhorn-node2 VM is not running"
    echo "Start it with: cd longhorn-node2 && ./start-vm.sh"
    exit 1
fi

# Check bloom binary exists
BLOOM_BINARY="$SCRIPT_DIR/../../../dist/bloom"
if [ ! -f "$BLOOM_BINARY" ]; then
    echo "ERROR: Bloom binary not found at $BLOOM_BINARY"
    echo "Expected location: $BLOOM_BINARY"
    echo "Build it with: make build"
    exit 1
fi

echo "✓ Both VMs are running"
echo "✓ Bloom binary found"
echo ""

# Create bloom configs
echo "Creating bloom configuration files..."

# Node 1 config (first node)
cat > "$WORK_DIR/node1-bloom.yaml" << 'EOF'
CLUSTERFORGE_RELEASE: none
DOMAIN: longhorn-test.local
FIRST_NODE: true
GPU_NODE: false
CLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1
NO_DISKS_FOR_CLUSTER: false
USE_CERT_MANAGER: false
CERT_OPTION: generate
PRELOAD_IMAGES: ""
EOF

echo "✓ Created node1-bloom.yaml"
echo ""

# Copy bloom binary and config to node1
echo "=========================================="
echo "Step 1: Deploying first node (longhorn-node1)"
echo "=========================================="
echo ""

echo "Copying bloom binary and config to node1..."
if ! scp -i "$WORK_DIR/longhorn-node1/qemu-login" \
    -o UserKnownHostsFile=/dev/null \
    -o StrictHostKeyChecking=no \
    -P 2222 \
    "$BLOOM_BINARY" \
    "$WORK_DIR/node1-bloom.yaml" \
    ubuntu@localhost:~/; then
    echo "ERROR: Failed to copy files to node1"
    exit 1
fi

echo "Verifying files copied successfully..."
ssh -i "$WORK_DIR/longhorn-node1/qemu-login" \
    -o UserKnownHostsFile=/dev/null \
    -o StrictHostKeyChecking=no \
    -p 2222 \
    ubuntu@localhost \
    'ls -lh ~/bloom ~/node1-bloom.yaml'

echo "Making bloom executable..."
ssh -i "$WORK_DIR/longhorn-node1/qemu-login" \
    -o UserKnownHostsFile=/dev/null \
    -o StrictHostKeyChecking=no \
    -p 2222 \
    ubuntu@localhost \
    'chmod +x ~/bloom'

echo ""
echo "Running bloom on node1 (this will take 5-10 minutes)..."
echo "Progress will be shown below..."
echo ""

ssh -i "$WORK_DIR/longhorn-node1/qemu-login" \
    -o UserKnownHostsFile=/dev/null \
    -o StrictHostKeyChecking=no \
    -p 2222 \
    ubuntu@localhost \
    'sudo ~/bloom cli --config ~/node1-bloom.yaml' | tee "$WORK_DIR/node1-bloom.log"

echo ""
echo "✓ Node1 deployment complete!"
echo ""

# Get join token and server IP from node1
echo "Retrieving cluster join information from node1..."

JOIN_TOKEN=$(ssh -i "$WORK_DIR/longhorn-node1/qemu-login" \
    -o UserKnownHostsFile=/dev/null \
    -o StrictHostKeyChecking=no \
    -p 2222 \
    ubuntu@localhost \
    'sudo cat /var/lib/rancher/rke2/server/node-token' 2>/dev/null || echo "")

if [ -z "$JOIN_TOKEN" ]; then
    echo "ERROR: Failed to retrieve join token from node1"
    exit 1
fi

echo "✓ Retrieved join token"
echo ""

# Create node2 config with join information
cat > "$WORK_DIR/node2-bloom.yaml" << EOF
CLUSTERFORGE_RELEASE: none
DOMAIN: longhorn-test.local
FIRST_NODE: false
GPU_NODE: false
SERVER_IP: 10.100.100.11
JOIN_TOKEN: $JOIN_TOKEN
CLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1
NO_DISKS_FOR_CLUSTER: false
USE_CERT_MANAGER: false
CERT_OPTION: generate
PRELOAD_IMAGES: ""
EOF

echo "✓ Created node2-bloom.yaml with join token"
echo ""

# Copy bloom binary and config to node2
echo "=========================================="
echo "Step 2: Joining second node (longhorn-node2)"
echo "=========================================="
echo ""

echo "Copying bloom binary and config to node2..."
if ! scp -i "$WORK_DIR/longhorn-node2/qemu-login" \
    -o UserKnownHostsFile=/dev/null \
    -o StrictHostKeyChecking=no \
    -P 2223 \
    "$BLOOM_BINARY" cli\
    "$WORK_DIR/node2-bloom.yaml" \
    ubuntu@localhost:~/; then
    echo "ERROR: Failed to copy files to node2"
    exit 1
fi

echo "Verifying files copied successfully..."
ssh -i "$WORK_DIR/longhorn-node2/qemu-login" \
    -o UserKnownHostsFile=/dev/null \
    -o StrictHostKeyChecking=no \
    -p 2223 \
    ubuntu@localhost \
    'ls -lh ~/bloom ~/node2-bloom.yaml'

echo "Making bloom executable..."
ssh -i "$WORK_DIR/longhorn-node2/qemu-login" \
    -o UserKnownHostsFile=/dev/null \
    -o StrictHostKeyChecking=no \
    -p 2223 \
    ubuntu@localhost \
    'chmod +x ~/bloom'

echo ""
echo "Running bloom on node2 (this will take 3-5 minutes)..."
echo "Progress will be shown below..."
echo ""

ssh -i "$WORK_DIR/longhorn-node2/qemu-login" \
    -o UserKnownHostsFile=/dev/null \
    -o StrictHostKeyChecking=no \
    -p 2223 \
    ubuntu@localhost \
    'sudo ~/bloom --config ~/node2-bloom.yaml' | tee "$WORK_DIR/node2-bloom.log"

echo ""
echo "✓ Node2 joined the cluster!"
echo ""

# Verify cluster
echo "=========================================="
echo "Step 3: Verifying cluster"
echo "=========================================="
echo ""

echo "Checking cluster nodes..."
ssh -i "$WORK_DIR/longhorn-node1/qemu-login" \
    -o UserKnownHostsFile=/dev/null \
    -o StrictHostKeyChecking=no \
    -p 2222 \
    ubuntu@localhost \
    'sudo kubectl get nodes -o wide'

echo ""
echo "Checking Longhorn status..."
ssh -i "$WORK_DIR/longhorn-node1/qemu-login" \
    -o UserKnownHostsFile=/dev/null \
    -o StrictHostKeyChecking=no \
    -p 2222 \
    ubuntu@localhost \
    'sudo kubectl get pods -n longhorn-system'

echo ""
echo "=========================================="
echo "✓ 2-node cluster deployment complete!"
echo "=========================================="
echo ""
echo "Cluster access:"
echo "  SSH to node1: cd $WORK_DIR/longhorn-node1 && ./ssh-vm.sh"
echo "  SSH to node2: cd $WORK_DIR/longhorn-node2 && ./ssh-vm.sh"
echo ""
echo "Kubectl access (from node1):"
echo "  sudo kubectl get nodes"
echo "  sudo kubectl get pods -A"
echo ""
echo "Logs saved to:"
echo "  Node1: $WORK_DIR/node1-bloom.log"
echo "  Node2: $WORK_DIR/node2-bloom.log"
echo ""
echo "Next: Test Longhorn volume migration (issue #561)"
echo ""
