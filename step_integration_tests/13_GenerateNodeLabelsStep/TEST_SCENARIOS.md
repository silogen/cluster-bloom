# GenerateNodeLabelsStep Integration Tests

## Purpose
Generate and configure Kubernetes node labels based on GPU availability and mounted disk information for Longhorn storage configuration.

## Step Overview
- **Execution Order**: Step 13
- **Commands Executed**:
  - Appends to `/etc/rancher/rke2/config.yaml`
- **Skip Conditions**:
  - Skipped if `NO_DISKS_FOR_CLUSTER=true`

## What This Step Does
1. Retrieves `mounted_disk_map` from viper (set by PrepareLonghornDisksStep)
2. Appends GPU node label if `GPU_NODE=true`
3. If disks are configured:
   - Appends Longhorn storage configuration template
   - Adds node-selector label for each mounted disk path
   - Creates storage tags for Longhorn disk management
4. Configures node labels in RKE2 config for automatic application

## Test Scenarios

### Success Scenarios

#### 1. GPU Node with Disk Labels
Tests successful label generation for GPU node with mounted disks.

#### 2. CPU Node with Disk Labels
Tests successful label generation for CPU node with mounted disks.

#### 3. GPU Node - No Disks (NO_DISKS_FOR_CLUSTER)
Tests that only GPU label is added when NO_DISKS_FOR_CLUSTER=true.

#### 4. Multiple Disks - Multiple Labels
Tests generation of multiple node-selector labels for multiple mounted disks.

#### 5. Empty Mounted Disk Map
Tests handling when mounted_disk_map is empty (GPU label only).

### Failure Scenarios

#### 6. Failed to Append GPU Node Label
Tests error handling when GPU label cannot be appended to config.yaml.

#### 7. Failed to Append Longhorn Template
Tests error handling when Longhorn configuration template write fails.

#### 8. Failed to Append Disk Labels
Tests error handling when disk node-selector labels cannot be written.

## Configuration Requirements

- `ENABLED_STEPS: "GenerateNodeLabelsStep"`
- `GPU_NODE: true` or `false`
- `NO_DISKS_FOR_CLUSTER: false` (for disk labels)
- `mounted_disk_map` in viper (from PrepareLonghornDisksStep)

## Mock Requirements

```yaml
mocks:
  # GPU label append
  "AppendToFile./etc/rancher/rke2/config.yaml":
    output: ""
    error: null

  # Longhorn template append
  "AppendToFile./etc/rancher/rke2/config.yaml":
    output: ""
    error: null

  # Disk label appends (one per disk)
  "AppendToFile./etc/rancher/rke2/config.yaml":
    output: ""
    error: null

  # Note: mounted_disk_map set via viper, not mocked
  # Example: map[/mnt/disk0:/dev/sdb-uuid /mnt/disk1:/dev/sdc-uuid]
```

## Running Tests

```bash
# Test 1: GPU node with disks
./cluster-bloom cli --config step_integration_tests/13_GenerateNodeLabelsStep/01-gpu-with-disks/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/13_GenerateNodeLabelsStep/01-gpu-with-disks/mocks.yaml

# Test 2: CPU node with disks
./cluster-bloom cli --config step_integration_tests/13_GenerateNodeLabelsStep/02-cpu-with-disks/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/13_GenerateNodeLabelsStep/02-cpu-with-disks/mocks.yaml

# Test 3: GPU node no disks
./cluster-bloom cli --config step_integration_tests/13_GenerateNodeLabelsStep/03-gpu-no-disks/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/13_GenerateNodeLabelsStep/03-gpu-no-disks/mocks.yaml
```

## Expected Outcomes

### Success Cases
- ✅ GPU node label added (if GPU_NODE=true)
- ✅ Longhorn template appended (if disks configured)
- ✅ Node-selector label for each disk mount point
- ✅ Storage tags configured in Longhorn template
- ✅ Step completes successfully

### Failure Cases
- ❌ Config file append failure stops execution
- ❌ Template write failure stops execution
- ❌ Disk label append failure stops execution

## Related Code
- Step implementation: `pkg/steps.go:369-388`
- Label generation logic uses mounted_disk_map from viper
- Appends to existing config.yaml created by PrepareRKE2Step

## Notes
- **GPU node label**: `node.kubernetes.io/instance-type=gpu-node`
- **Longhorn configuration**: Uses `node-selector` to assign specific disks to nodes
- **Storage tags**: Each disk gets tagged with its mount point path
- **mounted_disk_map format**: `map[mount_point]device` (e.g., `map[/mnt/disk0:/dev/sdb-uuid]`)
- **RKE2 config location**: `/etc/rancher/rke2/config.yaml`
- **Label application**: RKE2 automatically applies labels from config during node registration
- **Dependencies**: Requires PrepareLonghornDisksStep to populate mounted_disk_map
- **Skipped for no-disk clusters**: Saves resources when storage isn't needed
- **Multiple disks**: Each mount point gets its own node-selector entry
