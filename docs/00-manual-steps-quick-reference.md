# Manual Steps Quick Reference

## Overview

This document collates all manual installation steps from the detailed documentation into a single sequential reference. Use this for quick lookup when performing manual installations or troubleshooting.

For complete context and explanations, refer to the detailed documentation:
- [01-rke2-deployment.md](./01-rke2-deployment.md)
- [02-rocm-support.md](./02-rocm-support.md)
- [03-storage-management.md](./03-storage-management.md)
- [04-network-configuration.md](./04-network-configuration.md)
- [05-certificate-management.md](./05-certificate-management.md)
- [08-installation-guide.md](./08-installation-guide.md)

---

## Prerequisites Verification

### System Requirements
```bash
# Verify Ubuntu version (must be 20.04, 22.04, or 24.04)
lsb_release -a

# Check disk space (root: 20GB+, /var: 5GB+, /var/lib/rancher: 500GB+ recommended)
df -h / /var

# Verify memory (4GB+ minimum, 8GB+ recommended) and CPU (2+ cores, 4+ recommended)
free -h
nproc

# Check kernel modules
lsmod | grep overlay
lsmod | grep br_netfilter
lsmod | grep amdgpu  # For GPU nodes only
```

---

## First Node Installation

### 1. System Preparation

**Update System and Install Dependencies**
```bash
sudo apt update
sudo apt install -y jq nfs-common open-iscsi chrony curl wget
```

**Configure Firewall Ports**
```bash
# RKE2 required ports
sudo ufw allow 6443/tcp     # Kubernetes API
sudo ufw allow 9345/tcp     # RKE2 supervisor
sudo ufw allow 10250/tcp    # kubelet
sudo ufw allow 2379:2380/tcp # etcd
sudo ufw allow 30000:32767/tcp # NodePort services
sudo ufw allow 8472/udp     # Cilium VXLAN
sudo ufw allow 4240/tcp     # Cilium health checks
```

**Configure inotify Limits**
```bash
echo "fs.inotify.max_user_instances = 8192" | sudo tee -a /etc/sysctl.conf
echo "fs.inotify.max_user_watches = 524288" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

**Install Kubernetes Tools**
```bash
# Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Install k9s
wget https://github.com/derailed/k9s/releases/latest/download/k9s_Linux_amd64.tar.gz
tar -xzf k9s_Linux_amd64.tar.gz
sudo mv k9s /usr/local/bin/
```

### 2. Storage Configuration

**Configure Multipath**
```bash
sudo apt install -y multipath-tools

cat <<EOF | sudo tee /etc/multipath.conf
blacklist {
    devnode "^(ram|raw|loop|fd|md|dm-|sr|scd|st)[0-9]*"
    devnode "^hd[a-z]"
    devnode "^sd[a-z]"
}
EOF

sudo systemctl restart multipathd
```

**Load Required Kernel Modules**
```bash
sudo modprobe overlay
sudo modprobe br_netfilter

# Make persistent
cat <<EOF | sudo tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF
```

**Prepare Disks for Longhorn**
```bash
# List available NVMe drives
lsblk | grep nvme

# For each disk (e.g., /dev/nvme0n1) - WARNING: This erases all data!
sudo wipefs -a /dev/nvme0n1
sudo mkfs.ext4 /dev/nvme0n1

# Get UUID and create mount point
UUID=$(sudo blkid -s UUID -o value /dev/nvme0n1)
sudo mkdir -p /mnt/disk0

# Add to fstab for persistence
echo "UUID=$UUID /mnt/disk0 ext4 defaults,nofail 0 2" | sudo tee -a /etc/fstab

# Mount the disk
sudo mount -a
```

**Configure rsyslog**
```bash
cat <<EOF | sudo tee /etc/rsyslog.d/30-ratelimit.conf
# Limit iSCSI messages to prevent log flooding
:msg, contains, "iSCSI" stop
EOF

sudo systemctl restart rsyslog
```

**Configure logrotate**
```bash
cat <<EOF | sudo tee /etc/logrotate.d/bloom
/var/log/bloom.log {
    daily
    rotate 7
    compress
    missingok
    notifempty
}
EOF
```

### 3. GPU Setup (GPU Nodes Only)

**Install ROCm Drivers**
```bash
# Get Ubuntu codename and kernel version
CODENAME=$(grep VERSION_CODENAME /etc/os-release | cut -d= -f2)
KERNEL_VERSION=$(uname -r)

# Install kernel headers
sudo apt install -y linux-headers-$KERNEL_VERSION linux-modules-extra-$KERNEL_VERSION

# Install Python dependencies
sudo apt install -y python3-setuptools python3-wheel

# Download and install amdgpu-install
wget https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/$CODENAME/amdgpu-install_6.3.60302-1_all.deb
sudo apt install -y ./amdgpu-install_6.3.60302-1_all.deb

# Install ROCm
sudo amdgpu-install --usecase=rocm,dkms --yes

# Load amdgpu module
sudo modprobe amdgpu

# Verify installation
rocm-smi
```

**Configure GPU Permissions**
```bash
cat <<EOF | sudo tee /etc/udev/rules.d/70-amdgpu.rules
KERNEL=="kfd", MODE="0666"
SUBSYSTEM=="drm", KERNEL=="renderD*", MODE="0666"
EOF

sudo udevadm control --reload-rules
sudo udevadm trigger
```

### 4. RKE2 Kubernetes Installation

**Install RKE2**
```bash
curl -sfL https://get.rke2.io | sudo sh -
sudo mkdir -p /etc/rancher/rke2
```

**Configure RKE2 for First Node**
```bash
cat <<EOF | sudo tee /etc/rancher/rke2/config.yaml
write-kubeconfig-mode: "0644"
cni: cilium
disable:
  - rke2-ingress-nginx
tls-san:
  - $(hostname -I | awk '{print $1}')
node-label:
  - "node.longhorn.io/create-default-disk=config"
  - "node.longhorn.io/instance-manager=true"
EOF

# Add GPU labels for GPU nodes:
# node-label:
#   - "gpu=true"
#   - "amd.com/gpu=true"

# Add Longhorn disk labels (for each mounted disk):
# node-label:
#   - "silogen.ai/longhorndisks=disk0xxxdisk1xxx..."
```

**Create Audit Policy**
```bash
sudo mkdir -p /etc/rancher/rke2
cat <<EOF | sudo tee /etc/rancher/rke2/audit-policy.yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: Metadata
EOF

# Audit policy and logs are configured automatically in RKE2 config
# No manual config update needed
```

**Start RKE2 Service**
```bash
sudo systemctl enable rke2-server.service
sudo systemctl start rke2-server.service

# Wait for RKE2 to start (may take 2-5 minutes)
sudo systemctl status rke2-server.service

# Check logs if needed
sudo journalctl -u rke2-server -f
```

**Configure kubectl Access**
```bash
# Copy kubeconfig to user directory
mkdir -p ~/.kube
sudo cp /etc/rancher/rke2/rke2.yaml ~/.kube/config
sudo chown $(id -u):$(id -g) ~/.kube/config

# Add to PATH
echo 'export PATH=$PATH:/var/lib/rancher/rke2/bin' >> ~/.bashrc
export PATH=$PATH:/var/lib/rancher/rke2/bin

# Verify cluster is running
kubectl get nodes
```

**Get Join Information for Additional Nodes**
```bash
# Get join token
sudo cat /var/lib/rancher/rke2/server/node-token

# Get server IP
hostname -I | awk '{print $1}'
```

### 5. Storage and Networking Setup

**Deploy Longhorn Storage**
```bash
# Create Longhorn namespace
kubectl create namespace longhorn-system

# Apply Longhorn manifests
kubectl apply -f https://raw.githubusercontent.com/longhorn/longhorn/v1.8.0/deploy/longhorn.yaml

# Wait for Longhorn pods to be ready
kubectl wait --for=condition=ready pod -l app=longhorn-manager -n longhorn-system --timeout=600s

# Create default storage class
cat <<EOF | kubectl apply -f -
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: mlstorage
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: driver.longhorn.io
allowVolumeExpansion: true
reclaimPolicy: Delete
volumeBindingMode: Immediate
parameters:
  numberOfReplicas: "3"
  staleReplicaTimeout: "2880"
  fromBackup: ""
  fsType: "ext4"
EOF
```

**Deploy MetalLB Load Balancer**
```bash
# Get node IP for MetalLB pool
NODE_IP=$(hostname -I | awk '{print $1}')

# Install MetalLB
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.12/config/manifests/metallb-native.yaml

# Wait for MetalLB to be ready
kubectl wait --namespace metallb-system \
  --for=condition=ready pod \
  --selector=app=metallb \
  --timeout=90s

# Create IP address pool
cat <<EOF | kubectl apply -f -
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: cluster-bloom-ip-pool
  namespace: metallb-system
spec:
  addresses:
  - $NODE_IP/32
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: cluster-bloom-l2-adv
  namespace: metallb-system
spec:
  ipAddressPools:
  - cluster-bloom-ip-pool
EOF
```

**Configure Chrony Time Synchronization**
```bash
cat <<EOF | sudo tee /etc/chrony/chrony.conf
server 0.ubuntu.pool.ntp.org iburst
server 1.ubuntu.pool.ntp.org iburst
server 2.ubuntu.pool.ntp.org iburst
server 3.ubuntu.pool.ntp.org iburst
allow 0.0.0.0/0
local stratum 10
EOF

sudo systemctl restart chrony
```

### 6. TLS Certificate Configuration

**Option A: Using cert-manager with Let's Encrypt (Recommended for Production)**
```bash
DOMAIN="your.domain.com"

# Install cert-manager
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Wait for cert-manager to be ready
kubectl wait --for=condition=ready pod -l app=cert-manager -n cert-manager --timeout=300s

# Create domain configuration
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-domain
  namespace: default
data:
  domain: "$DOMAIN"
  use-cert-manager: "true"
EOF
```

**Option B: Using Existing Certificates**
```bash
DOMAIN="your.domain.com"
CERT_PATH="/path/to/tls.crt"
KEY_PATH="/path/to/tls.key"

# Create TLS secret from existing certificates
kubectl create secret tls cluster-tls \
  --cert=$CERT_PATH \
  --key=$KEY_PATH \
  -n default

# Create domain configuration
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-domain
  namespace: default
data:
  domain: "$DOMAIN"
  use-cert-manager: "false"
EOF
```

**Option C: Generate Self-Signed Certificates (Development/Testing)**
```bash
DOMAIN="your.domain.com"

# Generate self-signed certificate
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout tls.key -out tls.crt \
  -subj "/CN=$DOMAIN/O=$DOMAIN" \
  -addext "subjectAltName=DNS:$DOMAIN,DNS:*.$DOMAIN"

# Create TLS secret
kubectl create secret tls cluster-tls \
  --cert=tls.crt \
  --key=tls.key \
  -n default

# Create domain configuration
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-domain
  namespace: default
data:
  domain: "$DOMAIN"
  use-cert-manager: "false"
EOF
```

---

## Additional Node Installation

### Worker Node Setup

**1. Perform System Preparation (same as First Node)**
- Update system and install dependencies
- Configure firewall
- Configure multipath
- Load kernel modules
- Prepare disks (if storage node)
- Install ROCm (if GPU node)

**2. Install RKE2 Agent**
```bash
# Install RKE2 agent
curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE="agent" sudo sh -

# Configure RKE2 agent
sudo mkdir -p /etc/rancher/rke2
cat <<EOF | sudo tee /etc/rancher/rke2/config.yaml
server: https://<FIRST_NODE_IP>:9345
token: <JOIN_TOKEN>
node-label:
  - "node.longhorn.io/create-default-disk=config"
  - "node.longhorn.io/instance-manager=true"
EOF

# Add GPU labels for GPU nodes:
# node-label:
#   - "gpu=true"
#   - "amd.com/gpu=true"

# Add Longhorn disk labels (for each mounted disk):
# node-label:
#   - "silogen.ai/longhorndisks=disk0xxxdisk1xxx..."

# Start agent service
sudo systemctl enable rke2-agent.service
sudo systemctl start rke2-agent.service
```

**3. Configure Chrony to Sync with First Node**
```bash
cat <<EOF | sudo tee /etc/chrony/chrony.conf
server <FIRST_NODE_IP> iburst
EOF

sudo systemctl restart chrony
```

### Additional Control Plane Node Setup

**1. Perform System Preparation (same as First Node)**

**2. Install RKE2 Server**
```bash
# Install RKE2 server
curl -sfL https://get.rke2.io | sudo sh -

# Configure RKE2 server
sudo mkdir -p /etc/rancher/rke2
cat <<EOF | sudo tee /etc/rancher/rke2/config.yaml
server: https://<FIRST_NODE_IP>:9345
token: <JOIN_TOKEN>
write-kubeconfig-mode: "0644"
tls-san:
  - $(hostname -I | awk '{print $1}')
node-label:
  - "node.longhorn.io/create-default-disk=config"
  - "node.longhorn.io/instance-manager=true"
EOF

# Start server service
sudo systemctl enable rke2-server.service
sudo systemctl start rke2-server.service
```

**3. Configure Chrony**
```bash
cat <<EOF | sudo tee /etc/chrony/chrony.conf
server <FIRST_NODE_IP> iburst
EOF

sudo systemctl restart chrony
```

---

## Post-Installation Verification

**Verify All Pods Running**
```bash
kubectl get pods -A
```

**Check Node Status**
```bash
kubectl get nodes -o wide
```

**Verify Longhorn**
```bash
kubectl get pods -n longhorn-system
kubectl get storageclass
```

**Verify MetalLB**
```bash
kubectl get pods -n metallb-system
kubectl get ipaddresspool -n metallb-system
```

**Test PVC Creation (Longhorn Validation)**
```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
EOF

# Wait and verify
kubectl get pvc test-pvc

# Clean up
kubectl delete pvc test-pvc
```

**Check GPU Access (GPU Nodes)**
```bash
# Deploy test pod
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: rocm-test
spec:
  containers:
  - name: rocm-test
    image: rocm/pytorch:latest
    command: ["rocm-smi"]
    resources:
      limits:
        amd.com/gpu: 1
  restartPolicy: Never
EOF

# Check output
kubectl logs rocm-test

# Clean up
kubectl delete pod rocm-test
```

**Verify Time Synchronization**
```bash
# On all nodes
chronyc sources
chronyc tracking
```

**Test Certificate (if configured)**
```bash
# Check certificate secret
kubectl get secret cluster-tls -n default

# Test HTTPS endpoint (if ingress configured)
curl -k https://<DOMAIN>
```

---

## Troubleshooting Quick Reference

**RKE2 Service Not Starting**
```bash
# Check logs
sudo journalctl -u rke2-server -n 100 --no-pager

# Verify configuration
sudo cat /etc/rancher/rke2/config.yaml

# Check disk space
df -h /var/lib/rancher
```

**Node Not Joining**
```bash
# Verify token and server IP
sudo cat /etc/rancher/rke2/config.yaml

# Check firewall rules
sudo ufw status

# Test connectivity to first node
nc -zv <FIRST_NODE_IP> 9345

# Check agent logs
sudo journalctl -u rke2-agent -n 100 --no-pager
```

**Longhorn Pods Not Starting**
```bash
# Check disk mounts
df -h | grep /mnt/disk

# Verify node labels
kubectl get nodes --show-labels | grep longhorn

# Check Longhorn logs
kubectl logs -n longhorn-system -l app=longhorn-manager --tail=100
```

**GPU Not Detected**
```bash
# Verify amdgpu module loaded
lsmod | grep amdgpu

# Check ROCm installation
rocm-smi

# Verify device plugin
kubectl get pods -n kube-system | grep amd-gpu
```

**Time Sync Issues**
```bash
# Check chrony status
chronyc tracking

# Restart chrony
sudo systemctl restart chrony

# Verify NTP sources
chronyc sources -v
```

---

## Environment Variables for ClusterBloom

If using ClusterBloom automation instead of manual steps, configure via environment variables or `bloom.yaml`:

```bash
# Node configuration
export FIRST_NODE=true              # false for additional nodes
export JOIN_TOKEN="<token>"         # Required for additional nodes
export SERVER_IP="<ip>"             # Required for additional nodes

# Storage configuration
export NO_DISKS_FOR_CLUSTER=false   # Set true to skip disk setup
export CLUSTER_DISKS="/dev/nvme0n1,/dev/nvme1n1"  # Pre-selected disks
export CLUSTER_PREMOUNTED_DISKS="/mnt/disk0,/mnt/disk1"  # Pre-mounted paths

# GPU configuration
export GPU_NODE=true                # Enable GPU support

# Certificate configuration
export USE_CERT_MANAGER=true        # Use cert-manager for automatic certificates
export DOMAIN="cluster.example.com" # Cluster domain
export CERT_OPTION="generate"       # "existing" or "generate" if not using cert-manager
export TLS_CERT="/path/to/cert.pem" # Path to certificate (if existing)
export TLS_KEY="/path/to/key.pem"   # Path to key (if existing)

# Network configuration
export METALLB_IP_RANGE="192.168.1.100-192.168.1.110"  # Optional custom IP range
```

---

## Related Documentation

- **[PRD.md](./PRD.md)** - Product overview and features
- **[01-rke2-deployment.md](./01-rke2-deployment.md)** - Detailed RKE2 deployment documentation
- **[02-rocm-support.md](./02-rocm-support.md)** - AMD GPU and ROCm configuration
- **[03-storage-management.md](./03-storage-management.md)** - Longhorn storage setup
- **[04-network-configuration.md](./04-network-configuration.md)** - Network and load balancing
- **[05-certificate-management.md](./05-certificate-management.md)** - TLS certificate management
- **[08-installation-guide.md](./08-installation-guide.md)** - Complete manual installation guide
- **[10-configuration-reference.md](./10-configuration-reference.md)** - Configuration variable reference

---

## Key Differences: Manual vs Automated

ClusterBloom automates all of the above steps and provides:

1. **Interactive UI** - TUI and Web UI for configuration and monitoring
2. **Validation** - Pre-flight checks before any system modifications
3. **Error Recovery** - Automatic retry and reconfiguration on failures
4. **State Management** - Tracks progress and resumes on interruption
5. **Configuration Management** - YAML-based configuration with validation
6. **Disk Auto-detection** - Intelligent disk selection and formatting
7. **Integration** - Seamless ClusterForge and 1Password Connect integration
8. **Monitoring** - Real-time progress tracking and detailed logging
9. **Multi-node Coordination** - Automatic generation of join commands
10. **Best Practices** - Built-in configurations following Kubernetes best practices
