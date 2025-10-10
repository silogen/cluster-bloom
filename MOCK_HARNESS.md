# Mock Harness Documentation

## Overview

The mock harness allows you to test and develop the Cluster-Bloom UI and final output without making any actual system changes. All steps return success and generate realistic mock variables that would normally be created during a real installation.

## Quick Start

### 1. Build the Application

```bash
go build -o bloom
```

### 2. Run Mock Installation

#### First Node (Interactive Mode)
```bash
sudo ./bloom mock --config mock-config.yaml
```

#### Additional Node
```bash
# Edit mock-config.yaml and set:
# FIRST_NODE: false
# JOIN_TOKEN: "K10abc123xyz456def789ghi012jkl345::server:mock-token"
# SERVER_IP: "10.0.100.50"

sudo ./bloom mock --config mock-config.yaml
```

## Configuration

The mock harness uses the same configuration format as the real installation. See `mock-config.yaml` for a complete example.

### Key Configuration Options

#### Node Type
- `FIRST_NODE: true` - Simulates first node installation (includes control plane setup)
- `FIRST_NODE: false` - Simulates additional node joining existing cluster

#### GPU Configuration
- `GPU_NODE: true` - Simulates GPU node with ROCm installation
- `GPU_NODE: false` - Simulates CPU-only node

#### Domain and TLS
- `DOMAIN: "cluster.example.com"` - Domain for ingress configuration
- `USE_CERT_MANAGER: false` - Whether to use cert-manager
- `CERT_OPTION: "generate"` - Generate self-signed cert or use existing (`"existing"`)

## Mock Behavior

### What Gets Mocked

All steps are mocked to:
1. **Return success immediately** - No actual system changes
2. **Generate realistic variables** - Creates mock data like:
   - Join tokens: `K10abc123xyz456def789ghi012jkl345::server:mock-token-data`
   - IP addresses: `10.0.100.50`
   - Disk selections: `/dev/nvme0n1`, `/dev/nvme1n1`
   - GPU information: `8x AMD Instinct MI250X`
   - Storage capacity: `3.8TB`
3. **Simulate timing** - Each step has a small delay to simulate real execution
4. **Log realistic output** - Creates detailed logs as if installation was running

### Generated Variables

The mock harness sets these variables (accessible via viper):

**For First Node:**
- `join_token` - Mock join token for additional nodes
- `server_ip` - Mock server IP address (e.g., "10.0.100.50")
- `selected_disks` - Mock disk selection
- `mounted_disks` - Mock mount points

**For GPU Nodes:**
- `gpu_count` - Mock GPU count (e.g., 8)
- `gpu_model` - Mock GPU model (e.g., "AMD Instinct MI250X")

### Skip Logic

Mock steps respect the same skip logic as real steps:
- ROCm steps skip when `GPU_NODE: false`
- Disk mounting skips when `SKIP_DISK_CHECK: true` or `LONGHORN_DISKS` is set
- First-node-only steps skip when `FIRST_NODE: false`

## Testing Scenarios

### Scenario 1: First GPU Node
```yaml
FIRST_NODE: true
GPU_NODE: true
DOMAIN: "gpu-cluster.local"
USE_CERT_MANAGER: false
CERT_OPTION: "generate"
METALLB_IP_RANGE: "192.168.1.240-192.168.1.250"
```

**Expected Output:**
- Full installation with ROCm setup
- Mock GPU detection (8x MI250X)
- MetalLB and Longhorn configuration
- Join command generated in `additional_node_command.txt`

### Scenario 2: Additional GPU Node
```yaml
FIRST_NODE: false
GPU_NODE: true
JOIN_TOKEN: "K10abc123xyz456def789ghi012jkl345::server:mock-token"
SERVER_IP: "10.0.100.50"
```

**Expected Output:**
- Worker node joining cluster
- ROCm setup for GPU
- Longhorn drive setup instructions in `longhorn_drive_setup.txt`

### Scenario 3: CPU-Only First Node
```yaml
FIRST_NODE: true
GPU_NODE: false
SKIP_DISK_CHECK: true
DOMAIN: "cpu-cluster.local"
USE_CERT_MANAGER: true
```

**Expected Output:**
- Control plane setup without ROCm
- No disk selection/mounting (skipped)
- Cert-manager will handle TLS

## Development Workflow

### Testing UI Changes

1. Make changes to web UI templates or handlers
2. Run mock installation:
   ```bash
   sudo ./bloom mock --config mock-config.yaml
   ```
3. Open browser to `http://127.0.0.1:62078`
4. Watch the UI update in real-time as mock steps execute

### Testing Final Output

1. Configure mock-config.yaml with desired scenario
2. Run mock installation
3. Check generated files:
   - `bloom.log` - Full execution log
   - `additional_node_command.txt` - Join command (first node only)
   - Monitor web UI for final status

### Testing Skip Logic

Test step skipping by setting configuration:

```yaml
# Skip ROCm steps
GPU_NODE: false

# Skip disk mounting
SKIP_DISK_CHECK: true

# Skip specific steps
DISABLED_STEPS: "SetupClusterForgeStep,CreateDomainConfigStep"

# Only run specific steps
ENABLED_STEPS: "ValidateArgsStep,CheckUbuntuStep,MockFinalOutput"
```

## File Structure

### Mock Implementation Files

- **`pkg/mocksteps.go`** - All mock step implementations
  - Each step has a `Mock` prefix (e.g., `MockSetupRKE2Step`)
  - Returns success with simulated delays
  - Sets appropriate viper variables

- **`cmd/mock.go`** - Mock command implementation
  - Builds step arrays based on `FIRST_NODE` flag
  - Runs steps with UI

- **`mock-config.yaml`** - Sample configuration
  - Pre-configured for first GPU node
  - Includes comments for all options

## Extending the Mock Harness

### Adding New Mock Steps

1. Add step to `pkg/mocksteps.go`:
```go
var MockNewFeatureStep = Step{
    Id:          "NewFeatureStep",
    Name:        "New Feature",
    Description: "Does something new (MOCK)",
    Action: func() StepResult {
        time.Sleep(300 * time.Millisecond)
        LogMessage(Info, "Mock: New feature configured")
        viper.Set("new_feature_var", "mock-value")
        return StepResult{Error: nil}
    },
}
```

2. Add to step array in `cmd/mock.go`:
```go
steps = []pkg.Step{
    // ... existing steps ...
    pkg.MockNewFeatureStep,
    // ... more steps ...
}
```

### Customizing Mock Data

Edit the mock step actions in `pkg/mocksteps.go` to change:
- Sleep durations (simulate longer/shorter operations)
- Log messages (add more detail or change formatting)
- Generated variables (different IPs, tokens, etc.)
- Skip conditions (change when steps are skipped)

## Troubleshooting

### Mock Command Not Found

```bash
go build -o bloom
./bloom mock --help
```

### Variables Not Appearing in Final Output

Check that:
1. Mock steps are setting variables with `viper.Set()`
2. FinalOutput step is included in steps array
3. Configuration has `FIRST_NODE` set correctly

### Steps Being Skipped Unexpectedly

Check skip logic in mock steps:
- GPU_NODE affects ROCm and GPU-related steps
- FIRST_NODE affects control plane steps
- SKIP_DISK_CHECK affects disk-related steps

## Comparison: Mock vs Real

| Aspect | Mock | Real |
|--------|------|------|
| System Changes | None | Full installation |
| Execution Time | ~30 seconds | 10-30 minutes |
| Prerequisites | None | Sudo, Ubuntu, hardware |
| Variables | Simulated | Actual system data |
| Idempotent | Yes | Depends on steps |
| Use Case | Development/testing | Production deployment |

## Tips

1. **Fast iteration**: Mock runs complete in seconds, perfect for UI development
2. **Safe testing**: No risk of breaking your system during development
3. **Realistic simulation**: Mock data closely matches real installation output
4. **Full coverage**: Tests all code paths without requiring specific hardware
5. **Easy debugging**: Add log statements to mock steps to trace execution

## Next Steps

After testing with mock:
1. Validate UI displays all information correctly
2. Test with real installation on test hardware
3. Compare real vs mock output to ensure consistency
4. Update mock data if real installation output changes
