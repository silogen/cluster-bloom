# Mock Harness Quick Start

## What is the Mock Harness?

A testing tool that simulates the entire Cluster-Bloom installation without making any real system changes. Perfect for:
- Testing the web UI
- Developing the final output screen
- Validating configuration flows
- Testing without requiring real hardware

## Usage

### 1. Build
```bash
go build -o bloom
```

### 2. Run Mock Installation

**First Node (GPU):**
```bash
sudo ./bloom mock --config mock-config.yaml
```

**Additional Node:**
Edit `mock-config.yaml`:
```yaml
FIRST_NODE: false
JOIN_TOKEN: "K10abc123xyz456def789ghi012jkl345::server:mock-token"
SERVER_IP: "10.0.100.50"
```
Then run:
```bash
sudo ./bloom mock --config mock-config.yaml
```

### 3. View Results

**Web UI:** `http://127.0.0.1:62078`

**Generated Files:**
- `bloom.log` - Full execution log
- `additional_node_command.txt` - Join command (first node only)

## Mock vs Real

| Feature | Mock | Real |
|---------|------|------|
| Time | ~30 sec | 10-30 min |
| System Changes | None | Full install |
| Hardware Required | No | Yes |
| Variables Generated | Yes | Yes |

## Example Configurations

### GPU Cluster First Node
```yaml
FIRST_NODE: true
GPU_NODE: true
DOMAIN: "gpu.example.com"
USE_CERT_MANAGER: false
CERT_OPTION: "generate"
```

### CPU-Only Node
```yaml
FIRST_NODE: true
GPU_NODE: false
SKIP_DISK_CHECK: true
```

### Additional Worker
```yaml
FIRST_NODE: false
GPU_NODE: true
JOIN_TOKEN: "K10abc123..."
SERVER_IP: "10.0.100.50"
```

## Mock Variables Generated

The mock harness generates realistic variables:

- **Join Token:** `K10abc123xyz456def789ghi012jkl345::server:mock-token-data`
- **Server IP:** `10.0.100.50`
- **GPUs:** `8x AMD Instinct MI250X`
- **Disks:** `/dev/nvme0n1`, `/dev/nvme1n1`
- **Storage:** `3.8TB`
- **Versions:** RKE2 v1.28.3, Longhorn v1.5.3, MetalLB v0.13.12

## Files Created

**Implementation:**
- `pkg/mocksteps.go` - All mock step implementations
- `cmd/mock.go` - Mock command
- `mock-config.yaml` - Example configuration

**Documentation:**
- `MOCK_HARNESS.md` - Full documentation
- `MOCK_QUICKSTART.md` - This file

## Common Use Cases

### 1. Test UI Changes
```bash
# Make changes to web templates
vim pkg/webhandlers.go

# Run mock to see changes
sudo ./bloom mock --config mock-config.yaml

# Open http://127.0.0.1:62078
```

### 2. Test Final Output
```bash
# Run mock installation
sudo ./bloom mock --config mock-config.yaml

# Check generated command
cat additional_node_command.txt
```

### 3. Test Skip Logic
```yaml
# Test with GPU node
GPU_NODE: true
# vs
GPU_NODE: false
```

### 4. Test Step Filtering
```yaml
# Only run specific steps
ENABLED_STEPS: "ValidateArgsStep,CheckUbuntuStep,MockFinalOutput"
```

## Troubleshooting

**Q: Command not found?**
```bash
go build -o bloom
./bloom mock --help
```

**Q: Variables not showing?**
- Check `FIRST_NODE` is set correctly
- Verify mock steps are setting viper variables
- Look at `bloom.log` for details

**Q: Steps skipping?**
- Check skip conditions in mock steps
- GPU_NODE affects ROCm steps
- FIRST_NODE affects control plane steps
- SKIP_DISK_CHECK affects disk steps

## Next Steps

1. Run mock and verify UI displays correctly
2. Check final output format
3. Test different scenarios (GPU/CPU, first/additional)
4. When ready, test with real installation

For full documentation, see `MOCK_HARNESS.md`
