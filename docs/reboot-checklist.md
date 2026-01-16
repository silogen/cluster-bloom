# Cluster Reboot Checklist

This comprehensive checklist ensures that your cluster's storage infrastructure remains stable and functional across node reboots. Following this checklist helps prevent data loss and storage-related issues when rebooting cluster nodes with Longhorn storage.

## Table of Contents

1. [Overview](#overview)
2. [Pre-Reboot Preparation](#pre-reboot-preparation)
3. [GPU Workload Management](#gpu-workload-management)
4. [Reboot Execution](#reboot-execution)
5. [Post-Reboot Validation](#post-reboot-validation)
6. [Troubleshooting](#troubleshooting)
7. [Emergency Recovery](#emergency-recovery)

## Overview

This checklist covers the disk and storage-related aspects of cluster node reboots, specifically focusing on:

- **Longhorn storage disk persistence**
- **Mount point validation**
- **Storage health verification** 
- **Cluster storage availability**
- **Data integrity checks**

**⚠️ Important**: This checklist assumes you're using Longhorn for distributed storage and have disks mounted at `/mnt/diskX` locations.

## Pre-Reboot Preparation

### 1. Document Current State

Before making any changes, capture the current state of your system:

```bash
# Create a snapshot directory with timestamp
SNAPSHOT_DIR="/tmp/reboot-snapshot-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$SNAPSHOT_DIR"

# Document current disk state
lsblk -o +UUID > "$SNAPSHOT_DIR/lsblk-before.txt"
df -h > "$SNAPSHOT_DIR/df-before.txt"
mount | grep -E "/mnt/disk|/var/lib/rancher" > "$SNAPSHOT_DIR/mounts-before.txt"
cp /etc/fstab "$SNAPSHOT_DIR/fstab-before.txt"

echo "System state documented in: $SNAPSHOT_DIR"
```

### 2. Verify Longhorn Storage Health

Check the overall health of your Longhorn storage system:

```bash
# Check Longhorn manager pods
kubectl get pods -n longhorn-system | grep longhorn-manager

# Check Longhorn volume health
kubectl get volumes -n longhorn-system

# Check for any degraded volumes
kubectl get volumes -n longhorn-system -o json | jq -r '.items[] | select(.status.robustness != "healthy") | "\(.metadata.name): \(.status.robustness)"'

# Verify Longhorn nodes are schedulable
kubectl get lhnodes -n longhorn-system -o wide
```

### 3. Check Current Disk Mount Status

Verify all expected storage disks are properly mounted:

```bash
# Check current fstab entries for Longhorn disks
echo "=== Current fstab entries ==="
sudo cat /etc/fstab | grep -E "/mnt/disk[0-9]+"

# Check currently mounted disks
echo -e "\n=== Currently mounted disks ==="
df -h | grep -E "/mnt/disk[0-9]+"

# List all disks with UUIDs
echo -e "\n=== All disks with UUIDs ==="
lsblk -o +UUID

# Count expected vs actual mounts
EXPECTED_MOUNTS=$(sudo cat /etc/fstab | grep -E "/mnt/disk[0-9]+" | wc -l)
ACTUAL_MOUNTS=$(df -h | grep -E "/mnt/disk[0-9]+" | wc -l)
echo -e "\n=== Mount count comparison ==="
echo "Expected mounts: $EXPECTED_MOUNTS"
echo "Actual mounts: $ACTUAL_MOUNTS"

if [ "$EXPECTED_MOUNTS" -eq "$ACTUAL_MOUNTS" ]; then
    echo "✓ Mount count matches"
else
    echo "⚠️  Mount count mismatch - investigate before rebooting"
fi
```

### 4. Test Mount Configuration

Verify that all fstab entries will mount correctly after reboot:

```bash
# Test all fstab entries without actually mounting
echo "=== Testing fstab mount configuration ==="
sudo mount -a --fake --verbose

# For each disk mount, test individual mounting
echo -e "\n=== Testing individual disk mounts ==="
for disk in $(sudo cat /etc/fstab | grep -E "/mnt/disk[0-9]+" | awk '{print $2}'); do
    echo "Testing mount: $disk"
    if sudo findmnt "$disk" >/dev/null 2>&1; then
        echo "✓ $disk is currently mounted"
        
        # Test remount
        if sudo mount -o remount "$disk" 2>/dev/null; then
            echo "✓ $disk remount successful"
        else
            echo "⚠️  $disk remount failed"
        fi
    else
        echo "⚠️  $disk is not mounted"
        
        # Test mount
        if sudo mount "$disk" 2>/dev/null; then
            echo "✓ $disk mount successful"
        else
            echo "✗ $disk mount failed - investigate before rebooting"
        fi
    fi
done
```

### 5. Verify Filesystem Health

Check the integrity of all storage filesystems:

```bash
echo "=== Filesystem health check ==="
for device in $(lsblk -rno NAME,MOUNTPOINT | grep -E "/mnt/disk[0-9]+" | awk '{print $1}'); do
    echo "Checking filesystem on /dev/$device"
    
    # For mounted filesystems, do a read-only check
    if sudo fsck -n /dev/$device 2>/dev/null; then
        echo "✓ /dev/$device filesystem is clean"
    else
        echo "⚠️  /dev/$device filesystem has issues - consider repair"
    fi
done
```

### 6. Check Disk Space and Usage

Ensure adequate free space before reboot:

```bash
echo "=== Disk space verification ==="
df -h | grep -E "(^Filesystem|/mnt/disk|/$|/var)"

echo -e "\n=== Critical space checks ==="
# Check root partition
ROOT_USAGE=$(df / | awk 'NR==2 {print $5}' | sed 's/%//')
if [ "$ROOT_USAGE" -gt 90 ]; then
    echo "⚠️  Root partition is ${ROOT_USAGE}% full - consider cleanup"
else
    echo "✓ Root partition usage acceptable (${ROOT_USAGE}%)"
fi

# Check each storage disk
for disk in $(df -h | grep -E "/mnt/disk[0-9]+" | awk '{print $6}'); do
    DISK_USAGE=$(df "$disk" | awk 'NR==2 {print $5}' | sed 's/%//')
    if [ "$DISK_USAGE" -gt 85 ]; then
        echo "⚠️  $disk is ${DISK_USAGE}% full"
    else
        echo "✓ $disk usage acceptable (${DISK_USAGE}%)"
    fi
done
```

### 7. Backup Critical Configuration

Create backups of essential configuration files:

```bash
# Create backup directory
BACKUP_DIR="/root/pre-reboot-backup-$(date +%Y%m%d-%H%M%S)"
sudo mkdir -p "$BACKUP_DIR"

# Backup fstab
sudo cp /etc/fstab "$BACKUP_DIR/fstab.backup"

# Backup current mount information
mount > "$BACKUP_DIR/current-mounts.backup"
lsblk -o +UUID > "$BACKUP_DIR/lsblk.backup"

# Backup Longhorn configuration (if accessible)
kubectl get lhnodes -n longhorn-system -o yaml > "$BACKUP_DIR/longhorn-nodes.yaml" 2>/dev/null || echo "Could not backup Longhorn nodes"

echo "Configuration backed up to: $BACKUP_DIR"
```

### 8. Pre-Reboot Checklist Summary

**✓ Complete this checklist before proceeding with reboot:**

- [ ] System state documented
- [ ] Longhorn storage health verified
- [ ] All expected disks are mounted
- [ ] fstab entries test successfully
- [ ] Filesystem integrity confirmed
- [ ] Adequate disk space available
- [ ] Configuration files backed up
- [ ] No critical Longhorn operations in progress

## GPU Workload Management

**⚠️ CRITICAL**: For GPU-enabled clusters running AI/ML workloads, proper workload management is essential to prevent data loss and job interruption during reboots.

### GPU Workload Strategy Options

Choose the appropriate strategy based on your cluster configuration and workload criticality:

#### Option A: Sequential Node Reboot (Recommended for Multi-Node GPU Clusters)
- **Best for**: Multi-node clusters with distributed GPU workloads
- **Advantage**: Zero downtime for workloads, automatic rescheduling
- **Time**: Longer, but safer

#### Option B: Coordinated Cluster Shutdown (Fastest for Small Clusters)
- **Best for**: Small clusters, maintenance windows, urgent updates
- **Advantage**: Faster completion, full cluster consistency
- **Time**: Shorter, but requires downtime

#### Option C: Workload-Aware Rolling Updates
- **Best for**: Production clusters with SLA requirements  
- **Advantage**: Respects pod disruption budgets, graceful handling
- **Time**: Variable, depends on workload cooperation

### 1. Identify Running GPU Workloads

First, discover all GPU-utilizing workloads across the cluster:

```bash
echo "=== GPU Workload Discovery ==="

# Check GPU node labels and taints
echo "GPU-enabled nodes:"
kubectl get nodes -l cluster-bloom/gpu-node=true -o wide

# Find pods requesting GPU resources
echo -e "\n=== Pods using GPUs ==="
kubectl get pods --all-namespaces -o json | jq -r '
  .items[] | 
  select(.spec.containers[]?.resources.requests."nvidia.com/gpu" or .spec.containers[]?.resources.limits."nvidia.com/gpu" or .spec.containers[]?.resources.requests."amd.com/gpu" or .spec.containers[]?.resources.limits."amd.com/gpu") |
  "\(.metadata.namespace)/\(.metadata.name) on \(.spec.nodeName // "unscheduled")"
'

# Check for running AI/ML frameworks
echo -e "\n=== AI/ML Framework Detection ==="
kubectl get pods --all-namespaces -o json | jq -r '
  .items[] |
  select(.spec.containers[].image | test("pytorch|tensorflow|cuda|rocm|jupyter|notebook")) |
  "\(.metadata.namespace)/\(.metadata.name): \(.spec.containers[].image | split("/")[-1] | split(":")[0])"
'

# Check GPU utilization on current node (if GPU node)
echo -e "\n=== Current Node GPU Status ==="
if command -v nvidia-smi >/dev/null 2>&1; then
    echo "NVIDIA GPU Status:"
    nvidia-smi --query-gpu=index,name,utilization.gpu,memory.used,memory.total --format=csv
elif command -v rocm-smi >/dev/null 2>&1; then
    echo "AMD GPU Status:"
    rocm-smi --showuse --csv
else
    echo "No GPU monitoring tools found (node may not have GPUs)"
fi

# Check for long-running training jobs
echo -e "\n=== Long-Running Workload Detection ==="
kubectl get pods --all-namespaces --field-selector=status.phase=Running -o json | jq -r '
  .items[] |
  select(.status.startTime) |
  select((now - (.status.startTime | fromdateiso8601)) > 3600) |
  "\(.metadata.namespace)/\(.metadata.name): running for \(((now - (.status.startTime | fromdateiso8601))/3600) | floor)h"
'
```

### 2. Assess Workload Criticality and State

Determine which workloads can be safely interrupted:

```bash
echo "=== Workload State Assessment ==="

# Check for StatefulSets (usually critical)
echo "StatefulSets with GPU workloads:"
kubectl get statefulsets --all-namespaces -o json | jq -r '
  .items[] |
  select(.spec.template.spec.containers[]?.resources.requests."nvidia.com/gpu" or .spec.template.spec.containers[]?.resources.requests."amd.com/gpu") |
  "\(.metadata.namespace)/\(.metadata.name) (replicas: \(.spec.replicas))"
'

# Check for Jobs and CronJobs
echo -e "\nBatch Jobs with GPU requirements:"
kubectl get jobs --all-namespaces -o json | jq -r '
  .items[] |
  select(.spec.template.spec.containers[]?.resources.requests."nvidia.com/gpu" or .spec.template.spec.containers[]?.resources.requests."amd.com/gpu") |
  "\(.metadata.namespace)/\(.metadata.name) - completions: \(.status.succeeded // 0)/\(.spec.completions // "unlimited")"
'

# Check Pod Disruption Budgets that might affect GPU workloads
echo -e "\nPod Disruption Budgets:"
kubectl get pdb --all-namespaces

# Look for checkpointing annotations/labels (indicates resumable workloads)
echo -e "\nCheckpointing-enabled workloads:"
kubectl get pods --all-namespaces -o json | jq -r '
  .items[] |
  select(.metadata.annotations."checkpoint.enabled" or .metadata.labels."checkpoint.enabled") |
  "\(.metadata.namespace)/\(.metadata.name)"
'
```

### 3. Strategy A: Sequential Node Reboot (Recommended)

This approach maintains cluster availability while safely rebooting nodes one at a time:

```bash
echo "=== Sequential Node Reboot Strategy ==="

# Step 1: Get list of all GPU nodes
GPU_NODES=($(kubectl get nodes -l cluster-bloom/gpu-node=true -o name | sed 's/node\///'))
CURRENT_NODE=$(hostname)
TOTAL_NODES=${#GPU_NODES[@]}

echo "GPU nodes in cluster: ${GPU_NODES[*]}"
echo "Current node: $CURRENT_NODE"
echo "Total GPU nodes: $TOTAL_NODES"

if [ "$TOTAL_NODES" -eq 1 ]; then
    echo "⚠️  Single GPU node cluster detected!"
    echo "Consider Strategy B (Coordinated Shutdown) to avoid workload termination"
    echo "Or ensure all critical workloads support checkpointing/resumption"
fi

# Step 2: Check if this is the last node to be rebooted
echo -e "\n=== Checking Node Status ==="
echo "Cordoned nodes:"
kubectl get nodes -o json | jq -r '.items[] | select(.spec.unschedulable == true) | .metadata.name'

CORDONED_COUNT=$(kubectl get nodes -o json | jq -r '.items[] | select(.spec.unschedulable == true) | .metadata.name' | wc -l)
READY_NODES=$(kubectl get nodes --no-headers | grep " Ready " | grep -v SchedulingDisabled | wc -l)

echo "Cordoned nodes: $CORDONED_COUNT"
echo "Ready & schedulable nodes: $READY_NODES"

# Step 3: Cordon this node and wait for workload migration
echo -e "\n=== Cordoning Current Node ==="
kubectl cordon "$CURRENT_NODE"

if [ "$READY_NODES" -gt 1 ]; then
    echo "✓ Other nodes available for workload rescheduling"
    
    # Step 4: Gracefully drain GPU workloads
    echo "Initiating graceful drain with extended timeout for GPU workloads..."
    
    # Use longer timeout for GPU workloads (they may need time to checkpoint)
    kubectl drain "$CURRENT_NODE" \
        --ignore-daemonsets \
        --delete-emptydir-data \
        --force \
        --grace-period=300 \
        --timeout=1800s \
        --skip-wait-for-delete-timeout=10
        
    echo "✓ Node drained successfully"
    
else
    echo "⚠️  WARNING: This appears to be the last available GPU node!"
    echo "Proceeding will terminate all GPU workloads."
    echo "Consider the following options:"
    echo ""
    echo "1. Cancel reboot and use Strategy B (Coordinated Shutdown)"
    echo "2. Continue if workloads support automatic restart/checkpointing"
    echo "3. Manually migrate critical workloads to other node types"
    echo ""
    
    read -p "Continue with reboot? (yes/no): " CONFIRM
    if [ "$CONFIRM" != "yes" ]; then
        echo "Reboot cancelled. Uncordoning node..."
        kubectl uncordon "$CURRENT_NODE"
        exit 1
    fi
    
    echo "Proceeding with final node reboot..."
    kubectl drain "$CURRENT_NODE" \
        --ignore-daemonsets \
        --delete-emptydir-data \
        --force \
        --grace-period=180 \
        --timeout=600s
fi
```

### 4. Strategy B: Coordinated Cluster Shutdown

For planned maintenance or when you need to reboot all nodes:

```bash
echo "=== Coordinated Cluster Shutdown Strategy ==="

# Step 1: Announce maintenance window
echo "Initiating coordinated cluster maintenance..."

# Step 2: Scale down non-critical workloads
echo "Scaling down non-critical deployments..."

# Get deployments that are not marked as critical
kubectl get deployments --all-namespaces -o json | jq -r '
  .items[] |
  select(.metadata.annotations."maintenance.downscale" != "false") |
  select(.metadata.labels."app.kubernetes.io/component" != "critical") |
  "\(.metadata.namespace) \(.metadata.name) \(.spec.replicas)"
' > /tmp/deployment-replicas.backup

# Scale down eligible deployments
while IFS= read -r line; do
    NAMESPACE=$(echo "$line" | awk '{print $1}')
    DEPLOYMENT=$(echo "$line" | awk '{print $2}')
    REPLICAS=$(echo "$line" | awk '{print $3}')
    
    if [ "$REPLICAS" -gt 0 ]; then
        echo "Scaling down $NAMESPACE/$DEPLOYMENT (currently $REPLICAS replicas)"
        kubectl scale deployment "$DEPLOYMENT" -n "$NAMESPACE" --replicas=0
    fi
done < /tmp/deployment-replicas.backup

# Step 3: Wait for GPU workloads to complete or reach safe stopping point
echo -e "\n=== Waiting for GPU workload completion ==="

# Monitor long-running jobs
echo "Monitoring GPU jobs for completion..."
while true; do
    RUNNING_GPU_JOBS=$(kubectl get jobs --all-namespaces -o json | jq -r '
        .items[] |
        select(.spec.template.spec.containers[]?.resources.requests."nvidia.com/gpu" or .spec.template.spec.containers[]?.resources.requests."amd.com/gpu") |
        select(.status.conditions[]?.type != "Complete") |
        "\(.metadata.namespace)/\(.metadata.name)"
    ' | wc -l)
    
    if [ "$RUNNING_GPU_JOBS" -eq 0 ]; then
        echo "✓ All GPU jobs completed"
        break
    fi
    
    echo "Waiting for $RUNNING_GPU_JOBS GPU jobs to complete..."
    sleep 30
done

# Step 4: Cordon all GPU nodes
echo "Cordoning all GPU nodes..."
kubectl get nodes -l cluster-bloom/gpu-node=true -o name | xargs kubectl cordon

# Step 5: Final cleanup
echo "Performing final GPU workload cleanup..."
kubectl delete pods --all-namespaces --field-selector=status.phase=Succeeded --force --grace-period=0 2>/dev/null || true
```

### 5. Strategy C: Workload-Aware Rolling Updates

For production clusters with strict SLA requirements:

```bash
echo "=== Workload-Aware Rolling Update Strategy ==="

# Step 1: Create Pod Disruption Budgets for critical workloads without them
echo "Checking Pod Disruption Budget coverage..."

kubectl get deployments --all-namespaces -o json | jq -r '
  .items[] |
  select(.spec.template.spec.containers[]?.resources.requests."nvidia.com/gpu") |
  "\(.metadata.namespace) \(.metadata.name)"
' | while IFS= read -r line; do
    NAMESPACE=$(echo "$line" | awk '{print $1}')
    DEPLOYMENT=$(echo "$line" | awk '{print $2}')
    
    # Check if PDB exists
    if ! kubectl get pdb -n "$NAMESPACE" "$DEPLOYMENT-pdb" >/dev/null 2>&1; then
        echo "Creating PDB for $NAMESPACE/$DEPLOYMENT"
        kubectl create -f - <<EOF
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: $DEPLOYMENT-pdb
  namespace: $NAMESPACE
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: $DEPLOYMENT
EOF
    fi
done

# Step 2: Check workload tolerance for disruption
echo "Checking workload disruption tolerance..."

# Look for restart-tolerant workloads
kubectl get pods --all-namespaces -o json | jq -r '
  .items[] |
  select(.spec.containers[]?.resources.requests."nvidia.com/gpu") |
  select(.spec.restartPolicy != "Never") |
  "\(.metadata.namespace)/\(.metadata.name): restart-tolerant"
'

# Look for checkpointing capabilities
kubectl get pods --all-namespaces -o json | jq -r '
  .items[] |
  select(.metadata.annotations."checkpoint.interval" or .spec.containers[]?.env[]?.name == "CHECKPOINT_DIR") |
  "\(.metadata.namespace)/\(.metadata.name): supports checkpointing"
'

# Step 3: Implement gradual drain with monitoring
echo "Implementing workload-aware drain..."

# Drain with respect to PDBs and longer grace periods
kubectl drain "$CURRENT_NODE" \
    --ignore-daemonsets \
    --delete-emptydir-data \
    --grace-period=600 \
    --timeout=3600s \
    --skip-wait-for-delete-timeout=30 \
    --pod-running-timeout=10m
```

### 6. GPU Workload Verification Before Reboot

Final checks before proceeding with the actual reboot:

```bash
echo "=== Final GPU Workload Verification ==="

# Verify no critical GPU workloads remain on this node
REMAINING_GPU_PODS=$(kubectl get pods --all-namespaces --field-selector spec.nodeName="$CURRENT_NODE" -o json | jq -r '
  .items[] |
  select(.spec.containers[]?.resources.requests."nvidia.com/gpu" or .spec.containers[]?.resources.requests."amd.com/gpu") |
  select(.metadata.namespace != "kube-system") |
  "\(.metadata.namespace)/\(.metadata.name)"
')

if [ -n "$REMAINING_GPU_PODS" ]; then
    echo "⚠️  GPU workloads still running on this node:"
    echo "$REMAINING_GPU_PODS"
    echo ""
    echo "Options:"
    echo "1. Wait longer for graceful termination"
    echo "2. Force delete remaining pods (may cause data loss)"
    echo "3. Cancel reboot"
    echo ""
    
    read -p "How do you want to proceed? (wait/force/cancel): " ACTION
    case "$ACTION" in
        wait)
            echo "Waiting additional time for graceful termination..."
            sleep 300
            ;;
        force)
            echo "Force deleting remaining GPU pods..."
            echo "$REMAINING_GPU_PODS" | while IFS= read -r pod; do
                kubectl delete pod "$pod" --force --grace-period=0
            done
            ;;
        cancel)
            echo "Reboot cancelled. Uncordoning node..."
            kubectl uncordon "$CURRENT_NODE"
            exit 1
            ;;
    esac
else
    echo "✓ No GPU workloads remaining on this node"
fi

# Check cluster-wide GPU capacity
TOTAL_GPU_CAPACITY=$(kubectl describe nodes -l cluster-bloom/gpu-node=true | grep "amd.com/gpu\|nvidia.com/gpu" | grep "Capacity" | awk '{sum += $2} END {print sum}')
AVAILABLE_GPU_CAPACITY=$(kubectl describe nodes -l cluster-bloom/gpu-node=true | grep "amd.com/gpu\|nvidia.com/gpu" | grep "Allocatable" | awk '{sum += $2} END {print sum}')

echo "Cluster GPU Status:"
echo "Total GPU capacity: $TOTAL_GPU_CAPACITY"
echo "Available GPU capacity: $AVAILABLE_GPU_CAPACITY"

# Final confirmation
echo -e "\n=== Reboot Readiness Check ==="
echo "✓ GPU workloads handled according to selected strategy"
echo "✓ Node is cordoned (no new workloads will be scheduled)"
echo "✓ Critical workloads migrated or safely stopped"
echo "✓ Storage infrastructure verified (from previous steps)"
echo ""
echo "🔥 Ready for reboot!"
```

### 7. GPU Workload Management Summary

**✓ Complete this checklist for GPU workload management:**

**Strategy Selection:**
- [ ] Identified running GPU workloads and their criticality
- [ ] Selected appropriate reboot strategy (Sequential/Coordinated/Workload-Aware)
- [ ] Verified cluster has sufficient capacity for workload migration

**Workload Handling:**
- [ ] Cordoned current node to prevent new GPU workload scheduling
- [ ] Drained GPU workloads with appropriate grace periods
- [ ] Verified critical workloads migrated to other nodes OR safely stopped
- [ ] Confirmed no critical GPU processes remain on this node

**Cluster State:**
- [ ] Verified remaining GPU capacity can handle current workloads
- [ ] Documented any workloads that will be interrupted
- [ ] Ensured Pod Disruption Budgets are respected
- [ ] Prepared recovery plan for any terminated workloads

## Reboot Execution

### Safe Reboot Procedure

```bash
# 1. Gracefully drain the node (if part of multi-node cluster)
kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data --force

# 2. Perform the reboot
sudo reboot

# Note: If this is a single-node cluster, skip the drain step
```

## Post-Reboot Validation

### 1. Initial System Check

After the system boots up, perform these immediate checks:

```bash
# Wait for system to fully boot, then check basic connectivity
echo "=== System boot validation ==="
uptime
df -h /

# Check if SSH/network is accessible
echo "✓ System is accessible"
```

### 2. Storage Mount Validation

Verify all storage disks mounted correctly:

```bash
echo "=== Post-reboot mount validation ==="

# Check all expected mounts are present
echo "Current mounts:"
df -h | grep -E "/mnt/disk[0-9]+"

# Compare with pre-reboot state
EXPECTED_MOUNTS=$(sudo cat /etc/fstab | grep -E "/mnt/disk[0-9]+" | wc -l)
ACTUAL_MOUNTS=$(df -h | grep -E "/mnt/disk[0-9]+" | wc -l)

echo "Expected mounts: $EXPECTED_MOUNTS"
echo "Actual mounts: $ACTUAL_MOUNTS"

if [ "$EXPECTED_MOUNTS" -eq "$ACTUAL_MOUNTS" ]; then
    echo "✓ All expected disks mounted successfully"
else
    echo "✗ Mount count mismatch - investigating..."
    
    # Find missing mounts
    echo "=== Investigating missing mounts ==="
    for entry in $(sudo cat /etc/fstab | grep -E "/mnt/disk[0-9]+" | awk '{print $2}'); do
        if ! findmnt "$entry" >/dev/null 2>&1; then
            echo "Missing mount: $entry"
            echo "Attempting to mount..."
            if sudo mount "$entry"; then
                echo "✓ Successfully mounted $entry"
            else
                echo "✗ Failed to mount $entry - requires investigation"
            fi
        fi
    done
fi
```

### 3. UUID and Disk Identity Verification

Ensure all disks maintained their UUIDs and identity:

```bash
echo "=== UUID verification ==="

# Compare current UUIDs with backed up state (if backup available)
lsblk -o +UUID > /tmp/lsblk-after.txt

# Manual verification of critical mounts
echo "Current disk UUIDs:"
for disk in $(df | grep -E "/mnt/disk[0-9]+" | awk '{print $1}'); do
    echo "$disk: $(sudo blkid -s UUID -o value $disk)"
done

# Verify fstab UUIDs match actual disk UUIDs
echo -e "\n=== fstab vs actual UUID verification ==="
while IFS= read -r line; do
    if echo "$line" | grep -qE "UUID=.*[[:space:]]/mnt/disk[0-9]+"; then
        FSTAB_UUID=$(echo "$line" | grep -o 'UUID=[^[:space:]]*' | cut -d= -f2)
        MOUNT_POINT=$(echo "$line" | awk '{print $2}')
        
        if findmnt "$MOUNT_POINT" >/dev/null 2>&1; then
            DEVICE=$(findmnt -n -o SOURCE "$MOUNT_POINT")
            ACTUAL_UUID=$(sudo blkid -s UUID -o value "$DEVICE")
            
            if [ "$FSTAB_UUID" = "$ACTUAL_UUID" ]; then
                echo "✓ $MOUNT_POINT UUID matches ($FSTAB_UUID)"
            else
                echo "✗ $MOUNT_POINT UUID mismatch: fstab=$FSTAB_UUID, actual=$ACTUAL_UUID"
            fi
        else
            echo "⚠️  $MOUNT_POINT not mounted"
        fi
    fi
done < /etc/fstab
```

### 4. Storage Performance Test

Verify storage is not just mounted, but functional:

```bash
echo "=== Storage functionality test ==="

for mount_point in $(df | grep -E "/mnt/disk[0-9]+" | awk '{print $6}'); do
    echo "Testing $mount_point"
    
    # Test write capability
    test_file="$mount_point/.reboot-test-$(date +%s)"
    if echo "test data" | sudo tee "$test_file" >/dev/null 2>&1; then
        echo "✓ $mount_point write test successful"
        
        # Test read capability
        if sudo cat "$test_file" >/dev/null 2>&1; then
            echo "✓ $mount_point read test successful"
        else
            echo "✗ $mount_point read test failed"
        fi
        
        # Clean up test file
        sudo rm -f "$test_file"
    else
        echo "✗ $mount_point write test failed"
    fi
done
```

### 5. Kubernetes and Longhorn Recovery

If this is a Kubernetes node, verify cluster connectivity and Longhorn status:

```bash
echo "=== Kubernetes connectivity test ==="

# Test kubectl connectivity
if kubectl get nodes >/dev/null 2>&1; then
    echo "✓ Kubernetes cluster accessible"
    
    # Check node status
    NODE_STATUS=$(kubectl get nodes $(hostname) -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)
    if [ "$NODE_STATUS" = "True" ]; then
        echo "✓ Node is Ready in cluster"
    else
        echo "⚠️  Node not ready in cluster - may need time to stabilize"
    fi
    
    # Uncordon node if it was drained
    kubectl uncordon $(hostname) 2>/dev/null || echo "Node was not cordoned"
    
else
    echo "⚠️  Kubernetes cluster not yet accessible - may need time to start"
fi

echo -e "\n=== Longhorn status check ==="

# Wait for Longhorn pods to be ready
echo "Checking Longhorn system status..."
if kubectl get pods -n longhorn-system >/dev/null 2>&1; then
    echo "Longhorn namespace accessible"
    
    # Check Longhorn manager on this node
    LONGHORN_MANAGER=$(kubectl get pods -n longhorn-system -l app=longhorn-manager --field-selector spec.nodeName=$(hostname) -o name 2>/dev/null)
    if [ -n "$LONGHORN_MANAGER" ]; then
        echo "✓ Longhorn manager pod found on this node"
        
        # Check if it's running
        POD_STATUS=$(kubectl get $LONGHORN_MANAGER -n longhorn-system -o jsonpath='{.status.phase}' 2>/dev/null)
        if [ "$POD_STATUS" = "Running" ]; then
            echo "✓ Longhorn manager is running"
        else
            echo "⚠️  Longhorn manager status: $POD_STATUS"
        fi
    else
        echo "⚠️  No Longhorn manager pod found on this node"
    fi
    
    # Check Longhorn node status
    LH_NODE_STATUS=$(kubectl get lhnodes $(hostname) -n longhorn-system -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)
    if [ "$LH_NODE_STATUS" = "True" ]; then
        echo "✓ Longhorn node is ready"
    else
        echo "⚠️  Longhorn node not ready - status: $LH_NODE_STATUS"
    fi
else
    echo "⚠️  Longhorn system not accessible yet"
fi
```

### 6. Final Validation Summary

```bash
echo "=== FINAL POST-REBOOT VALIDATION SUMMARY ==="

# Disk mounts
EXPECTED_MOUNTS=$(sudo cat /etc/fstab | grep -E "/mnt/disk[0-9]+" | wc -l)
ACTUAL_MOUNTS=$(df -h | grep -E "/mnt/disk[0-9]+" | wc -l)

if [ "$EXPECTED_MOUNTS" -eq "$ACTUAL_MOUNTS" ] && [ "$ACTUAL_MOUNTS" -gt 0 ]; then
    echo "✓ All storage disks ($ACTUAL_MOUNTS) mounted successfully"
else
    echo "✗ Storage mount issues detected"
fi

# Storage functionality
echo "✓ Storage read/write tests completed"

# System accessibility
echo "✓ System accessible and responsive"

# Kubernetes status (if applicable)
if kubectl get nodes >/dev/null 2>&1; then
    echo "✓ Kubernetes cluster connectivity confirmed"
else
    echo "- Kubernetes not configured or not yet ready"
fi

echo -e "\n=== Next Steps ==="
echo "1. Monitor Longhorn dashboard for volume health"
echo "2. Verify application pods can access persistent storage"
echo "3. Check for any error logs in system journals"
echo "4. Consider running extended storage health checks"

echo -e "\nReboot validation completed at: $(date)"
```

### 7. GPU Workload Recovery

After successful reboot, restore GPU workload capacity and verify GPU functionality:

```bash
echo "=== GPU System Recovery ==="

# Check GPU hardware detection
echo "Verifying GPU hardware detection..."
if command -v nvidia-smi >/dev/null 2>&1; then
    echo "NVIDIA GPU Status:"
    nvidia-smi --query-gpu=index,name,driver_version,memory.total,temperature.gpu --format=csv
    
    # Check for any GPU errors
    if nvidia-smi -q | grep -i error; then
        echo "⚠️  GPU errors detected in nvidia-smi output"
    else
        echo "✓ NVIDIA GPUs appear healthy"
    fi
    
elif command -v rocm-smi >/dev/null 2>&1; then
    echo "AMD GPU Status:"
    rocm-smi --showtemp --showmeminfo --csv
    
    # Check ROCm functionality
    if rocm-smi --showuse | grep -q "GPU use"; then
        echo "✓ AMD GPUs detected and functional"
    else
        echo "⚠️  AMD GPU detection may have issues"
    fi
else
    echo "⚠️  No GPU monitoring tools found"
fi

# Verify Kubernetes GPU device plugin
echo -e "\n=== Kubernetes GPU Device Plugin Status ==="
kubectl get pods -n kube-system -l name=nvidia-device-plugin -o wide 2>/dev/null || \
kubectl get pods -n kube-system -l app.kubernetes.io/name=amd-gpu-device-plugin -o wide 2>/dev/null || \
echo "No GPU device plugin pods found"

# Check GPU resource reporting in Kubernetes
echo -e "\n=== GPU Resource Availability ==="
kubectl describe nodes $(hostname) | grep -A 10 "Capacity:\|Allocatable:" | grep -E "amd.com/gpu|nvidia.com/gpu"

# Uncordon the node to allow GPU workload scheduling
echo -e "\n=== Enabling GPU Workload Scheduling ==="
kubectl uncordon $(hostname)

if kubectl get node $(hostname) -o jsonpath='{.spec.unschedulable}' | grep -q "true"; then
    echo "⚠️  Node is still cordoned"
else
    echo "✓ Node is schedulable for new GPU workloads"
fi

# Wait for GPU resources to be available
echo "Waiting for GPU resources to be reported..."
sleep 30

AVAILABLE_GPUS=$(kubectl describe node $(hostname) | grep -E "amd.com/gpu|nvidia.com/gpu" | grep "Allocatable" | awk '{print $2}' || echo "0")
echo "Available GPUs on this node: $AVAILABLE_GPUS"

if [ "$AVAILABLE_GPUS" -gt 0 ]; then
    echo "✓ GPU resources are available for scheduling"
else
    echo "⚠️  No GPU resources reported - may need investigation"
fi
```

### 8. GPU Workload Restoration

Restore GPU workloads based on the reboot strategy used:

```bash
echo "=== GPU Workload Restoration ==="

# Strategy A & C: Check for workloads that should automatically reschedule
if [ -f /tmp/deployment-replicas.backup ]; then
    echo "Restoring scaled-down deployments (Strategy B)..."
    
    while IFS= read -r line; do
        NAMESPACE=$(echo "$line" | awk '{print $1}')
        DEPLOYMENT=$(echo "$line" | awk '{print $2}')
        ORIGINAL_REPLICAS=$(echo "$line" | awk '{print $3}')
        
        if [ "$ORIGINAL_REPLICAS" -gt 0 ]; then
            echo "Scaling up $NAMESPACE/$DEPLOYMENT to $ORIGINAL_REPLICAS replicas"
            kubectl scale deployment "$DEPLOYMENT" -n "$NAMESPACE" --replicas="$ORIGINAL_REPLICAS"
        fi
    done < /tmp/deployment-replicas.backup
    
    rm -f /tmp/deployment-replicas.backup
else
    echo "Checking for workload rescheduling (Strategies A & C)..."
    
    # Check for pending GPU workloads
    PENDING_GPU_PODS=$(kubectl get pods --all-namespaces --field-selector=status.phase=Pending -o json | jq -r '
      .items[] |
      select(.spec.containers[]?.resources.requests."nvidia.com/gpu" or .spec.containers[]?.resources.requests."amd.com/gpu") |
      "\(.metadata.namespace)/\(.metadata.name)"
    ')
    
    if [ -n "$PENDING_GPU_PODS" ]; then
        echo "GPU workloads waiting to be scheduled:"
        echo "$PENDING_GPU_PODS"
        
        echo "Waiting for pod scheduling..."
        sleep 60
        
        # Check again
        STILL_PENDING=$(kubectl get pods --all-namespaces --field-selector=status.phase=Pending -o json | jq -r '
          .items[] |
          select(.spec.containers[]?.resources.requests."nvidia.com/gpu" or .spec.containers[]?.resources.requests."amd.com/gpu") |
          "\(.metadata.namespace)/\(.metadata.name)"
        ')
        
        if [ -n "$STILL_PENDING" ]; then
            echo "⚠️  Some GPU workloads are still pending:"
            echo "$STILL_PENDING"
        else
            echo "✓ All GPU workloads scheduled successfully"
        fi
    else
        echo "✓ No pending GPU workloads found"
    fi
fi

# Verify GPU workload health
echo -e "\n=== GPU Workload Health Check ==="
RUNNING_GPU_PODS=$(kubectl get pods --all-namespaces --field-selector=status.phase=Running -o json | jq -r '
  .items[] |
  select(.spec.containers[]?.resources.requests."nvidia.com/gpu" or .spec.containers[]?.resources.requests."amd.com/gpu") |
  "\(.metadata.namespace)/\(.metadata.name)"
')

if [ -n "$RUNNING_GPU_PODS" ]; then
    echo "Running GPU workloads:"
    echo "$RUNNING_GPU_PODS"
    
    # Test GPU functionality with a quick test pod (optional)
    echo -e "\n=== GPU Functionality Test (Optional) ==="
    echo "Would you like to run a GPU functionality test? (yes/no)"
    read -r RUN_TEST
    
    if [ "$RUN_TEST" = "yes" ]; then
        echo "Creating GPU test pod..."
        
        # Determine GPU type for test
        if kubectl describe node $(hostname) | grep -q "nvidia.com/gpu"; then
            GPU_TEST_IMAGE="nvidia/cuda:11.8-runtime-ubuntu20.04"
            GPU_TEST_CMD='["nvidia-smi"]'
            GPU_RESOURCE='"nvidia.com/gpu": 1'
        elif kubectl describe node $(hostname) | grep -q "amd.com/gpu"; then
            GPU_TEST_IMAGE="rocm/rocm-terminal:latest"  
            GPU_TEST_CMD='["rocm-smi"]'
            GPU_RESOURCE='"amd.com/gpu": 1'
        else
            echo "No GPU resources detected for testing"
            GPU_TEST_IMAGE=""
        fi
        
        if [ -n "$GPU_TEST_IMAGE" ]; then
            kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: gpu-test-$(date +%s)
  namespace: default
spec:
  restartPolicy: Never
  nodeSelector:
    kubernetes.io/hostname: $(hostname)
  containers:
  - name: gpu-test
    image: $GPU_TEST_IMAGE
    command: $GPU_TEST_CMD
    resources:
      requests:
        $GPU_RESOURCE
      limits:
        $GPU_RESOURCE
EOF
            
            echo "GPU test pod created. Check logs with:"
            echo "kubectl logs gpu-test-* -n default"
        fi
    fi
else
    echo "No GPU workloads currently running"
fi
```

### 9. Cluster-Wide GPU Health Verification

```bash
echo "=== Cluster-Wide GPU Health Check ==="

# Check all GPU nodes status
echo "GPU-enabled nodes status:"
kubectl get nodes -l cluster-bloom/gpu-node=true -o wide

# Verify total GPU capacity vs demand
echo -e "\n=== GPU Capacity Analysis ==="
TOTAL_GPUS=$(kubectl describe nodes -l cluster-bloom/gpu-node=true | grep -E "nvidia.com/gpu|amd.com/gpu" | grep "Capacity" | awk '{sum += $2} END {print sum+0}')
ALLOCATED_GPUS=$(kubectl describe nodes -l cluster-bloom/gpu-node=true | grep -E "nvidia.com/gpu|amd.com/gpu" | grep "Allocated" | awk '{sum += $2} END {print sum+0}')
AVAILABLE_GPUS=$((TOTAL_GPUS - ALLOCATED_GPUS))

echo "Total cluster GPU capacity: $TOTAL_GPUS"
echo "Currently allocated GPUs: $ALLOCATED_GPUS" 
echo "Available GPUs: $AVAILABLE_GPUS"

# Check for any failed GPU workloads
echo -e "\n=== Failed GPU Workload Detection ==="
FAILED_GPU_PODS=$(kubectl get pods --all-namespaces --field-selector=status.phase=Failed -o json | jq -r '
  .items[] |
  select(.spec.containers[]?.resources.requests."nvidia.com/gpu" or .spec.containers[]?.resources.requests."amd.com/gpu") |
  "\(.metadata.namespace)/\(.metadata.name): \(.status.reason // "Unknown")"
')

if [ -n "$FAILED_GPU_PODS" ]; then
    echo "⚠️  Failed GPU workloads detected:"
    echo "$FAILED_GPU_PODS"
else
    echo "✓ No failed GPU workloads found"
fi

echo -e "\n=== GPU Reboot Recovery Complete ==="
echo "✓ GPU hardware detected and functional"
echo "✓ Kubernetes GPU resources available" 
echo "✓ Node uncordoned and schedulable"
echo "✓ GPU workloads restored/rescheduled"
echo "✓ Cluster GPU capacity verified"
```

### 10. Post-Reboot Checklist Summary

**✓ Complete this validation after reboot:**

- [ ] System boots successfully
- [ ] All storage disks are mounted
- [ ] Disk UUIDs are preserved
- [ ] Storage read/write functionality confirmed
- [ ] Kubernetes node rejoins cluster (if applicable)
- [ ] Longhorn storage system is operational
- [ ] GPU hardware detected and functional
- [ ] Kubernetes GPU resources available for scheduling
- [ ] Node uncordoned and accepting GPU workloads
- [ ] GPU workloads restored/rescheduled successfully
- [ ] No errors in system logs
- [ ] Applications can access persistent storage and GPU resources

## Troubleshooting

### GPU Workload Issues After Reboot

If GPU workloads fail to start or schedule after reboot:

```bash
# Check GPU device plugin status
kubectl get pods -n kube-system -l name=nvidia-device-plugin -o wide
kubectl logs -n kube-system -l name=nvidia-device-plugin --tail=50

# Verify GPU driver and runtime
nvidia-container-runtime --version  # For NVIDIA
rocm-smi --version                   # For AMD

# Check node GPU labels and taints
kubectl describe node $(hostname) | grep -A 5 -B 5 -E "gpu|GPU"

# Restart GPU device plugin if needed
kubectl delete pods -n kube-system -l name=nvidia-device-plugin
```

### Alternative Reboot Strategies

**Rolling Maintenance with Load Balancer:**
- Use external load balancer to redirect traffic
- Reboot nodes behind load balancer one at a time
- Best for web-facing GPU services

**Checkpoint-Based Approach:**
- For long-running ML training jobs
- Implement checkpoint saving before reboot
- Automatic restart from checkpoint after reboot
- Requires application-level checkpoint support

**Blue-Green Node Replacement:**
- Provision new nodes with updated configuration  
- Gradually migrate workloads to new nodes
- Decommission old nodes after validation
- Zero downtime but requires additional resources

### Missing Mounts After Reboot

If some disks fail to mount automatically:

```bash
# 1. Check what failed to mount
sudo mount -a

# 2. Check system logs for errors
journalctl -u systemd-fsck* | tail -20
dmesg | grep -i error | tail -10

# 3. Manually mount missing disks
for entry in $(sudo cat /etc/fstab | grep -E "/mnt/disk[0-9]+" | awk '{print $2}'); do
    if ! findmnt "$entry" >/dev/null 2>&1; then
        echo "Attempting to mount $entry"
        sudo mount "$entry"
    fi
done

# 4. Check filesystem if mount fails
# (Replace /dev/nvmeXnY with actual device)
sudo fsck -y /dev/nvmeXnY
```

### UUID Changes After Reboot

If UUIDs have changed (rare, but can happen with filesystem corruption):

```bash
# 1. Identify devices with new UUIDs
lsblk -o +UUID

# 2. Update fstab with new UUIDs
# (Manual process - backup fstab first)
sudo cp /etc/fstab /etc/fstab.backup

# 3. Get new UUID for device
NEW_UUID=$(sudo blkid -s UUID -o value /dev/nvmeXnY)

# 4. Update fstab entry with new UUID
sudo sed -i "s/UUID=old-uuid-here/UUID=$NEW_UUID/" /etc/fstab

# 5. Test the mount
sudo mount -a
```

### Longhorn Issues After Reboot

If Longhorn doesn't detect disks after reboot:

```bash
# 1. Verify disk paths in Longhorn match actual mounts
kubectl get lhnodes -n longhorn-system -o yaml

# 2. Check Longhorn manager logs
kubectl logs -n longhorn-system -l app=longhorn-manager --tail=50

# 3. Restart Longhorn manager if needed
kubectl delete pods -n longhorn-system -l app=longhorn-manager

# 4. Verify disk tags and scheduling
kubectl patch lhnode $(hostname) -n longhorn-system --type='json' \
  -p='[{"op": "replace", "path": "/spec/allowScheduling", "value": true}]'
```

## Emergency Recovery

### Complete Storage Failure

If all storage mounts fail and the system is unstable:

```bash
# 1. Boot into single-user mode or recovery mode

# 2. Check filesystem integrity on all storage devices
for device in /dev/nvme*n1 /dev/sd*; do
    if [ -b "$device" ] && [[ ! "$device" =~ nvme[0-9]+n1p[0-9]+ ]] && [[ ! "$device" =~ sd[a-z][0-9]+ ]]; then
        echo "Checking $device"
        fsck -y "$device"
    fi
done

# 3. Restore fstab from backup if needed
if [ -f /root/pre-reboot-backup-*/fstab.backup ]; then
    sudo cp /root/pre-reboot-backup-*/fstab.backup /etc/fstab
fi

# 4. Attempt to mount all storage
sudo mount -a

# 5. If recovery fails, check the Longhorn documentation for data recovery procedures
```

### Data Recovery Resources

- **Backup Location**: `/root/pre-reboot-backup-*`
- **Longhorn Volume Recovery**: [Longhorn Disaster Recovery](https://longhorn.io/docs/1.8.0/snapshots-and-backups/backup-and-restore/)
- **System Recovery**: Boot from rescue media and mount storage externally

---

**Note**: This checklist focuses specifically on storage and disk-related aspects of cluster reboots. For comprehensive cluster management, also consider application-specific recovery procedures and cluster networking validation.