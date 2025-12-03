# Manual Installation Guide

## Overview

This guide documents the manual steps that ClusterBloom automates. Use this for:
- Understanding ClusterBloom's automated process
- Troubleshooting installation issues
- Performing custom installations
- Adapting the process for non-Ubuntu systems

## Prerequisites Verification

### System Requirements Check

**Verify Ubuntu Version**:
```bash
lsb_release -a
# Must be Ubuntu 20.04, 22.04, or 24.04
```

**Check Disk Space**:
```bash
df -h /
# Root partition: minimum 20GB
# /var/lib/rancher: recommended 500GB
df -h /var
# /var partition: recommended 5GB for container images
```

**Verify Memory and CPU**:
```bash
free -h
# Minimum 4GB RAM, recommended 8GB
nproc
# Minimum 2 cores, recommended 4 cores
```

**Check Kernel Modules**:
```bash
lsmod | grep overlay
lsmod | grep br_netfilter
# For GPU nodes:
lsmod | grep amdgpu
```

## First Node Installation

### Phase 1: System Preparation

**Update System and Install Dependencies**:
```bash
sudo apt update
sudo apt install -y jq nfs-common open-iscsi chrony curl wget
```

**Configure Firewall Ports**:
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

**Configure inotify Limits**:
```bash
echo "fs.inotify.max_user_instances = 8192" | sudo tee -a /etc/sysctl.conf
echo "fs.inotify.max_user_watches = 524288" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

**Install Kubernetes Tools**:
```bash
# Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Install k9s
wget https://github.com/derailed/k9s/releases/latest/download/k9s_Linux_amd64.tar.gz
tar -xzf k9s_Linux_amd64.tar.gz
sudo mv k9s /usr/local/bin/
```

### Phase 2: Storage Configuration

**Configure Multipath**:
```bash
sudo apt install -y multipath-tools

# Create multipath blacklist configuration
cat <<EOF | sudo tee /etc/multipath.conf
blacklist {
    devnode "^(ram|raw|loop|fd|md|dm-|sr|scd|st)[0-9]*"
    devnode "^hd[a-z]"
    devnode "^sd[a-z]"
}
EOF

sudo systemctl restart multipathd
```

**Load Required Kernel Modules**:
```bash
sudo modprobe overlay
sudo modprobe br_netfilter

# Make persistent
cat <<EOF | sudo tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF
```

**Identify and Prepare Disks for Longhorn**:
```bash
# List available NVMe drives
lsblk | grep nvme

# For each disk (e.g., /dev/nvme0n1):
# WARNING: This will erase all data on the disk
sudo wipefs -a /dev/nvme0n1
sudo mkfs.ext4 /dev/nvme0n1

# Get UUID
UUID=$(sudo blkid -s UUID -o value /dev/nvme0n1)

# Create mount point
sudo mkdir -p /mnt/disk0

# Add to fstab for persistence
echo "UUID=$UUID /mnt/disk0 ext4 defaults,nofail 0 2" | sudo tee -a /etc/fstab

# Mount the disk
sudo mount -a
```

**Configure rsyslog**:
```bash
cat <<EOF | sudo tee /etc/rsyslog.d/30-ratelimit.conf
# Limit iSCSI messages to prevent log flooding
:msg, contains, "iSCSI" stop
EOF

sudo systemctl restart rsyslog
```

**Configure logrotate**:
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

### Phase 3: GPU Setup (GPU Nodes Only)

**Install ROCm Drivers**:
```bash
# Get Ubuntu codename
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

**Configure GPU Permissions**:
```bash
cat <<EOF | sudo tee /etc/udev/rules.d/70-amdgpu.rules
KERNEL=="kfd", MODE="0666"
SUBSYSTEM=="drm", KERNEL=="renderD*", MODE="0666"
EOF

sudo udevadm control --reload-rules
sudo udevadm trigger
```

### Phase 4: RKE2 Kubernetes Installation

**Install RKE2**:
```bash
# Download RKE2 installation script
curl -sfL https://get.rke2.io | sudo sh -

# Create RKE2 configuration directory
sudo mkdir -p /etc/rancher/rke2
```

**Configure RKE2 for First Node**:
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
EOF

# If GPU node, add GPU labels
# node-label:
#   - "gpu=true"
#   - "amd.com/gpu=true"
```

**Create Audit Policy**:
```bash
sudo mkdir -p /etc/rancher/rke2/audit
cat <<EOF | sudo tee /etc/rancher/rke2/audit/policy.yaml
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
  - level: Metadata
EOF

# Update RKE2 config to enable audit logging
cat <<EOF | sudo tee -a /etc/rancher/rke2/config.yaml
audit-policy-file: /etc/rancher/rke2/audit/policy.yaml
EOF
```

**Start RKE2 Service**:
```bash
sudo systemctl enable rke2-server.service
sudo systemctl start rke2-server.service

# Wait for RKE2 to start (may take 2-5 minutes)
sudo systemctl status rke2-server.service

# Check logs if needed
sudo journalctl -u rke2-server -f
```

**Configure kubectl Access**:
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

**Get Join Token for Additional Nodes**:
```bash
sudo cat /var/lib/rancher/rke2/server/node-token
# Save this token for additional nodes

# Get server IP
hostname -I | awk '{print $1}'
```

### Phase 5: Storage and Networking Setup

**Deploy Longhorn Storage**:
```bash
# Create manifests directory
sudo mkdir -p /var/lib/rancher/rke2/server/manifests

# Download Longhorn manifests
# (ClusterBloom includes pre-configured Longhorn manifests)
# Apply standard Longhorn installation or custom configuration

# Wait for Longhorn pods to be ready
kubectl wait --for=condition=ready pod -l app=longhorn-manager -n longhorn-system --timeout=600s
```

**Deploy MetalLB Load Balancer**:
```bash
# Get node IP for MetalLB pool
NODE_IP=$(hostname -I | awk '{print $1}')

# Create MetalLB namespace and install
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

**Configure Chrony Time Synchronization**:
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

**Configure Domain and TLS** (if using custom domain):
```bash
DOMAIN="your.domain.com"

# Option A: Using cert-manager with Let's Encrypt
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

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

# Option B: Using existing certificates
kubectl create secret tls cluster-tls \
  --cert=/path/to/tls.crt \
  --key=/path/to/tls.key \
  -n default

# Option C: Generate self-signed certificates
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout tls.key -out tls.crt \
  -subj "/CN=$DOMAIN/O=$DOMAIN"

kubectl create secret tls cluster-tls \
  --cert=tls.crt \
  --key=tls.key \
  -n default
```

**Deploy ClusterForge** (Optional):
```bash
# Download ClusterForge release
wget https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz
tar -xzf deploy-release.tar.gz
cd deploy-release

# Deploy ClusterForge
./deploy.sh
```

## Additional Node Installation

### Worker Node Setup

**Perform System Preparation** (Steps 1-11 from First Node):
- Update system and install dependencies
- Configure firewall
- Configure multipath
- Load kernel modules
- Prepare disks (if storage node)
- Install ROCm (if GPU node)

**Install RKE2 Agent**:
```bash
curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE="agent" sudo sh -

# Configure RKE2 agent
sudo mkdir -p /etc/rancher/rke2
cat <<EOF | sudo tee /etc/rancher/rke2/config.yaml
server: https://\<FIRST_NODE_IP\>:9345
token: <JOIN_TOKEN>
EOF

# Start agent service
sudo systemctl enable rke2-agent.service
sudo systemctl start rke2-agent.service
```

### Additional Control Plane Node Setup

**Install RKE2 Server**:
```bash
curl -sfL https://get.rke2.io | sudo sh -

# Configure RKE2 server
sudo mkdir -p /etc/rancher/rke2
cat <<EOF | sudo tee /etc/rancher/rke2/config.yaml
server: https://\<FIRST_NODE_IP\>:9345
token: <JOIN_TOKEN>
write-kubeconfig-mode: "0644"
tls-san:
  - $(hostname -I | awk '{print $1}')
EOF

# Start server service
sudo systemctl enable rke2-server.service
sudo systemctl start rke2-server.service
```

**Configure Chrony** (sync with first node):
```bash
cat <<EOF | sudo tee /etc/chrony/chrony.conf
server <FIRST_NODE_IP> iburst
EOF

sudo systemctl restart chrony
```

**Verify Node Joined**:
```bash
# On first node, check new node status
kubectl get nodes
```

## Post-Installation Verification

**Verify All Pods Running**:
```bash
kubectl get pods -A
```

**Check Longhorn Status**:
```bash
kubectl get pods -n longhorn-system
```

**Verify MetalLB**:
```bash
kubectl get pods -n metallb-system
```

**Test PVC Creation** (Longhorn validation):
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

kubectl get pvc test-pvc
kubectl delete pvc test-pvc
```

**Check GPU Access** (GPU nodes):
```bash
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
  restartPolicy: Never
EOF

kubectl logs rocm-test
kubectl delete pod rocm-test
```

## Key Differences: Manual vs Automated

ClusterBloom automates all of the above steps and provides:

1. **Interactive UI**: TUI and Web UI for configuration and monitoring
2. **Validation**: Pre-flight checks before any system modifications
3. **Error Recovery**: Automatic retry and reconfiguration on failures
4. **State Management**: Tracks progress and resumes on interruption
5. **Configuration Management**: YAML-based configuration with validation
6. **Disk Auto-detection**: Intelligent disk selection and formatting
7. **Integration**: Seamless ClusterForge and 1Password Connect integration
8. **Monitoring**: Real-time progress tracking and detailed logging
9. **Multi-node Coordination**: Automatic generation of join commands
10. **Best Practices**: Built-in configurations following Kubernetes best practices

See [PRD.md](../../PRD.md) for product overview and [06-technical-architecture.md](06-technical-architecture.md) for technical details.
