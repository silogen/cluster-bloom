#!/bin/bash

#==============================================================================
# ClusterBloom Test Deployment Script
#==============================================================================
# Automates the deployment and connection to a test cluster-bloom instance
#
# Usage: ./test-deploy.sh <host> [node-name] [bloom-release]
#
#   host          SSH config alias (e.g. pd1) or bare IP address
#   node-name     Label for local kubeconfig file (default: node-<host>)
#   bloom-release GitHub release tag or "latest" (default: latest)
#
# Example: ./test-deploy.sh pd1
# Example: ./test-deploy.sh pd1 my-test-node v2.0.2
# Example: ./test-deploy.sh 10.0.255.70
#==============================================================================

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
  echo -e "${BLUE}ℹ${NC} $*"
}

log_success() {
  echo -e "${GREEN}✓${NC} $*"
}

log_warning() {
  echo -e "${YELLOW}⚠${NC} $*"
}

log_error() {
  echo -e "${RED}✗${NC} $*"
}

# Return the resolved IP for a host alias or IP string.
# Uses `ssh -G` to honour ~/.ssh/config, then falls back to DNS.
resolve_host() {
  local host="$1"
  if [[ "$host" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "$host"
    return
  fi
  local resolved
  resolved=$(ssh -G "$host" 2>/dev/null | awk '/^hostname / { print $2; exit }')
  if [[ -n "$resolved" ]]; then
    echo "$resolved"
    return
  fi
  resolved=$(getent hosts "$host" 2>/dev/null | awk '{print $1; exit}')
  echo "${resolved:-$host}"
}

is_private_ip() {
  local ip="$1"
  [[ "$ip" =~ ^10\. ]] || [[ "$ip" =~ ^192\.168\. ]] || [[ "$ip" =~ ^172\.(1[6-9]|2[0-9]|3[01])\. ]]
}

# Determine the DOMAIN for bloom.yaml.
# Tries DNS on the original alias; uses <ip>.nip.io if DNS returns a private/no result.
resolve_domain() {
  local host="$1"
  local ip="$2"
  local dns_result
  dns_result=$(getent hosts "$host" 2>/dev/null | awk '{print $1; exit}')
  if [[ -n "$dns_result" ]] && ! is_private_ip "$dns_result"; then
    echo "$host"
  else
    echo "${ip}.nip.io"
  fi
}

# Parse arguments
DISKS_OVERRIDE=""
POSITIONAL=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --disks) DISKS_OVERRIDE="$2"; shift 2 ;;
    --disks=*) DISKS_OVERRIDE="${1#--disks=}"; shift ;;
    *) POSITIONAL+=("$1"); shift ;;
  esac
done
set -- "${POSITIONAL[@]+"${POSITIONAL[@]}"}"

if [ $# -lt 1 ]; then
  log_error "Usage: $0 <host> [node-name] [bloom-release] [--disks /dev/sdb,/dev/sdc]"
  log_info "Example: $0 pd1"
  log_info "Example: $0 pd1 my-test-node v2.0.2"
  log_info "Example: $0 10.0.255.70 --disks /dev/sdb"
  exit 1
fi

HOST="$1"
NODE_NAME="${2:-node-${HOST}}"
BLOOM_RELEASE="${3:-latest}"

# Resolve actual IP (needed for nip.io and SSH when no config alias)
IP_ADDRESS=$(resolve_host "$HOST")

# If the input was a bare IP, SSH with user@ip; otherwise use the alias so
# ProxyJump and other SSH config settings are respected.
if [[ "$HOST" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  SSH_TARGET="${USER}@${IP_ADDRESS}"
else
  SSH_TARGET="$HOST"
fi

DOMAIN=$(resolve_domain "$HOST" "$IP_ADDRESS")
KUBECONFIG_LOCAL="${HOME}/.kube/${NODE_NAME}"
SSH_TUNNEL_PORT=6443

log_info "=== ClusterBloom Test Deployment ==="
log_info "Host:             $HOST"
log_info "Resolved IP:      $IP_ADDRESS"
log_info "SSH target:       $SSH_TARGET"
log_info "Domain:           $DOMAIN"
log_info "Node name:        $NODE_NAME"
log_info "Bloom release:    $BLOOM_RELEASE"
log_info "Local kubeconfig: $KUBECONFIG_LOCAL"
echo ""

# Step 1: Test SSH connectivity
log_info "Testing SSH connectivity..."
if ! ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=accept-new -o BatchMode=yes "${SSH_TARGET}" "echo 'SSH connection successful'" 2>/dev/null; then
  log_error "Cannot connect to ${SSH_TARGET}"
  log_info "Please ensure:"
  log_info "  1. The server is running"
  log_info "  2. Your SSH key is configured"
  log_info "  3. The host alias or IP address is correct"
  exit 1
fi
log_success "SSH connection verified"
echo ""

# Step 2: Determine CLUSTER_DISKS
if [ -n "$DISKS_OVERRIDE" ]; then
  CLUSTER_DISKS="$DISKS_OVERRIDE"
  log_success "CLUSTER_DISKS (provided): ${CLUSTER_DISKS}"
else
  # Discover data disks: all disks except the OS disk.
  # Empty disks are included automatically; previously formatted disks require confirmation.
  log_info "Discovering data disks on ${SSH_TARGET}..."

  disk_info=$(ssh -o StrictHostKeyChecking=accept-new "${SSH_TARGET}" bash << 'ENDSSH'
root_part=$(findmnt -n -o SOURCE /)
os_disk=$(lsblk -no PKNAME "$root_part" 2>/dev/null)
: "${os_disk:=$(basename "$root_part" | sed 's/[0-9]*$//')}"

while IFS= read -r name; do
  [ "$name" = "$os_disk" ] && continue
  dev="/dev/$name"
  size=$(lsblk -d -n -o SIZE "$dev" 2>/dev/null)
  fstype=$(lsblk -d -n -o FSTYPE "$dev" 2>/dev/null | tr -d '[:space:]')
  children=$(lsblk -n -o NAME "$dev" 2>/dev/null | grep -vc "^$name$" || true)
  if [ -n "$fstype" ] || [ "$children" -gt 0 ]; then
    echo "used $dev $size ${fstype:-partitioned}"
  else
    echo "empty $dev $size"
  fi
done < <(lsblk -d -n -o NAME,TYPE | awk '$2=="disk" {print $1}')
ENDSSH
)

  if [ -z "$disk_info" ]; then
    log_error "No data disks found on ${SSH_TARGET} (only the OS disk is present)"
    exit 1
  fi

  confirmed_disks=()
  while IFS=' ' read -r status dev size detail; do
    if [ "$status" = "empty" ]; then
      log_success "  ${dev} (${size}) — empty, will be used"
      confirmed_disks+=("$dev")
    else
      log_warning "  ${dev} (${size}, ${detail}) — previously formatted"
      read -r -p "    Use ${dev} for CLUSTER_DISKS? [y/N] " answer </dev/tty
      if [[ "$answer" =~ ^[Yy]$ ]]; then
        confirmed_disks+=("$dev")
        log_success "    ${dev} confirmed"
      else
        log_info "    ${dev} skipped"
      fi
    fi
  done <<< "$disk_info"

  if [ ${#confirmed_disks[@]} -eq 0 ]; then
    log_error "No disks selected for CLUSTER_DISKS — aborting"
    exit 1
  fi

  CLUSTER_DISKS=$(IFS=,; echo "${confirmed_disks[*]}")
  log_success "CLUSTER_DISKS: ${CLUSTER_DISKS}"
fi
echo ""

# Step 3: Create bloom.yaml on remote server
log_info "Creating bloom.yaml on remote server..."
ssh -o StrictHostKeyChecking=accept-new "${SSH_TARGET}" "cat > bloom.yaml" << EOF
DOMAIN: ${DOMAIN}
CERT_OPTION: generate
FIRST_NODE: true
GPU_NODE: true
CLUSTER_DISKS: ${CLUSTER_DISKS}
CLUSTERFORGE_RELEASE: none
EOF
log_success "bloom.yaml created"
echo ""

# Step 4: Get bloom binary — build locally if running from the repo, else download from GitHub
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOCAL_BINARY=""

if [ -f "${REPO_ROOT}/go.mod" ] && grep -q "github.com/silogen/cluster-bloom" "${REPO_ROOT}/go.mod"; then
  log_info "Running inside cluster-bloom repo — building local bloom binary..."
  (cd "${REPO_ROOT}" && CGO_ENABLED=0 go build \
    -ldflags="-X 'github.com/silogen/cluster-bloom/cmd.Version=dev-local'" \
    -o dist/bloom) || { log_error "Local build failed"; exit 1; }
  LOCAL_BINARY="${REPO_ROOT}/dist/bloom"
  log_success "Built local binary: ${LOCAL_BINARY}"

  log_info "Copying bloom binary to ${SSH_TARGET}..."
  scp -o StrictHostKeyChecking=accept-new "${LOCAL_BINARY}" "${SSH_TARGET}:bloom"
  ssh -o StrictHostKeyChecking=accept-new "${SSH_TARGET}" "chmod +x bloom && ./bloom --version || echo 'Bloom binary uploaded'"
  log_success "Local bloom binary deployed to ${SSH_TARGET}"
else
  log_info "Downloading bloom binary (${BLOOM_RELEASE}) from GitHub..."
  if [ "$BLOOM_RELEASE" = "latest" ]; then
    DOWNLOAD_URL="https://github.com/silogen/cluster-bloom/releases/latest/download/bloom"
  else
    DOWNLOAD_URL="https://github.com/silogen/cluster-bloom/releases/download/${BLOOM_RELEASE}/bloom"
  fi

  ssh -o StrictHostKeyChecking=accept-new "${SSH_TARGET}" << EOF
set -e
rm -f bloom
wget -q --show-progress "${DOWNLOAD_URL}" || {
  echo "Failed to download bloom from ${DOWNLOAD_URL}"
  exit 1
}
chmod +x bloom
./bloom --version || echo "Bloom binary downloaded"
EOF
  log_success "Bloom binary (${BLOOM_RELEASE}) downloaded to ${SSH_TARGET}"
fi
echo ""

# Step 5: Run bloom installation
log_info "Starting bloom installation (this may take several minutes)..."
log_warning "Installation output will be shown below"
echo ""
echo "========================================"

if ssh -o StrictHostKeyChecking=accept-new "${SSH_TARGET}" "sudo ./bloom cli bloom.yaml"; then
  echo "========================================"
  log_success "Bloom installation completed"
else
  echo "========================================"
  log_error "Bloom installation failed"
  exit 1
fi
echo ""

# Step 6: Wait a moment for kubeconfig to be generated
log_info "Waiting for kubeconfig to be generated..."
sleep 5

# Step 7: SCP kubeconfig back to local machine
log_info "Downloading kubeconfig to ${KUBECONFIG_LOCAL}..."
mkdir -p "${HOME}/.kube"
if scp -o StrictHostKeyChecking=accept-new "${SSH_TARGET}:.kube/config" "${KUBECONFIG_LOCAL}"; then
  log_success "Kubeconfig downloaded"
else
  log_error "Failed to download kubeconfig"
  log_info "Trying alternative path: /root/.kube/config"
  if scp -o StrictHostKeyChecking=accept-new "${SSH_TARGET}:/root/.kube/config" "${KUBECONFIG_LOCAL}"; then
    log_success "Kubeconfig downloaded from /root/.kube/config"
  else
    log_error "Could not retrieve kubeconfig"
    exit 1
  fi
fi
echo ""

# Step 8: Setup SSH tunnel
log_info "Setting up SSH tunnel (local port ${SSH_TUNNEL_PORT} -> remote port 6443)..."

# Kill any existing tunnel for this node
pkill -f "ssh.*${IP_ADDRESS}.*${SSH_TUNNEL_PORT}:127.0.0.1:6443" 2>/dev/null || true
sleep 1

# Create SSH tunnel in background
ssh -o StrictHostKeyChecking=accept-new -f -N -L "${SSH_TUNNEL_PORT}:127.0.0.1:6443" "${SSH_TARGET}"
log_success "SSH tunnel established (PID: $(pgrep -f "ssh.*${IP_ADDRESS}.*${SSH_TUNNEL_PORT}:127.0.0.1:6443"))"
echo ""

# Step 9: Modify kubeconfig
log_info "Modifying kubeconfig for local access..."

# Backup original
cp "${KUBECONFIG_LOCAL}" "${KUBECONFIG_LOCAL}.backup"

# Update server to localhost
sed -i.tmp "s|https://.*:6443|https://127.0.0.1:${SSH_TUNNEL_PORT}|g" "${KUBECONFIG_LOCAL}"

# Add insecure-skip-tls-verify and comment out certificate-authority-data
awk '
  /certificate-authority-data:/ {
    print "    insecure-skip-tls-verify: true"
    print "    # " $0
    next
  }
  { print }
' "${KUBECONFIG_LOCAL}" > "${KUBECONFIG_LOCAL}.new"
mv "${KUBECONFIG_LOCAL}.new" "${KUBECONFIG_LOCAL}"

# Clean up temp files
rm -f "${KUBECONFIG_LOCAL}.tmp"

log_success "Kubeconfig updated (backup saved to ${KUBECONFIG_LOCAL}.backup)"
echo ""

# Step 10: Test connectivity
log_info "Testing kubectl connectivity..."
export KUBECONFIG="${KUBECONFIG_LOCAL}"

if kubectl get nodes 2>/dev/null; then
  echo ""
  log_success "Connection successful! Cluster is ready."
else
  log_error "kubectl get nodes failed"
  log_info "Waiting 10 seconds for cluster to stabilize..."
  sleep 10

  if kubectl get nodes 2>/dev/null; then
    echo ""
    log_success "Connection successful! Cluster is ready."
  else
    log_error "Still unable to connect. Please check:"
    log_info "  1. SSH tunnel is running: ps aux | grep 'ssh.*${IP_ADDRESS}'"
    log_info "  2. Kubeconfig is valid: cat ${KUBECONFIG_LOCAL}"
    log_info "  3. Remote cluster is healthy: ssh ${SSH_TARGET} 'kubectl get nodes'"
    exit 1
  fi
fi

echo ""
log_info "=== Setup Complete ==="
log_success "Export kubeconfig: export KUBECONFIG=${KUBECONFIG_LOCAL}"
log_success "Test connection: kubectl get nodes"
log_info ""
log_info "To stop the SSH tunnel later:"
log_info "  pkill -f 'ssh.*${IP_ADDRESS}.*${SSH_TUNNEL_PORT}:127.0.0.1:6443'"
echo ""
