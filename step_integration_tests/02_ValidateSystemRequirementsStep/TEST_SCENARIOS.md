# ValidateSystemRequirementsStep Integration Tests

## Purpose
Validate system resources (disk space, memory, CPU cores, OS version, and kernel modules) to ensure the system meets minimum requirements for Kubernetes cluster installation.

## Step Overview
- **Execution Order**: Step 2
- **Commands Executed**:
  - Reads `/proc/meminfo` (memory check)
  - Reads `/proc/cpuinfo` (CPU check)
  - Reads `/etc/os-release` (Ubuntu version check)
  - `syscall.Statfs("/")` (root partition disk space)
  - `syscall.Statfs("/var")` (var partition disk space if separate)
  - `lsmod` (check loaded kernel modules)
  - `modinfo <module>` (check module availability)
- **Skip Conditions**: Never skipped

## What This Step Does
1. **Validates disk space**:
   - Root partition: Minimum 20GB total, 10GB available required
   - /var partition: Minimum 5GB available recommended (if separate)
2. **Validates memory**:
   - Minimum: 4GB required
   - Recommended: 8GB
3. **Validates CPU**:
   - Minimum: 2 cores required
   - Recommended: 4 cores
4. **Validates Ubuntu version**:
   - Supported: 20.04, 22.04, 24.04
   - Warns if not Ubuntu or unsupported version
5. **Validates kernel modules** (non-fatal warnings):
   - Required: `overlay`, `br_netfilter`
   - GPU nodes: `amdgpu`

## Test Scenarios

### Success Scenarios

#### 1. Meets All Requirements - Recommended Specs
Tests successful validation with recommended specs (8GB RAM, 4 cores, 20GB+ disk).

#### 2. Meets Minimum Requirements
Tests successful validation with minimum specs (4GB RAM, 2 cores, 10GB available).

#### 3. Ubuntu 22.04 - Fully Supported
Tests validation on Ubuntu 22.04 (supported version).

#### 4. Ubuntu 24.04 - Fully Supported
Tests validation on Ubuntu 24.04 (supported version).

#### 5. Separate /var Partition - Sufficient Space
Tests validation when /var is on separate partition with sufficient space.

#### 6. Kernel Modules Already Loaded
Tests validation when required kernel modules (overlay, br_netfilter) already loaded.

#### 7. GPU Node - amdgpu Module Available
Tests validation on GPU node with amdgpu module available.

### Failure Scenarios

#### 8. Insufficient Disk Space - Root Partition
Tests error when root partition has less than 10GB available.

#### 9. Insufficient Memory - Below 4GB
Tests error when system has less than 4GB RAM.

#### 10. Insufficient CPU Cores - Single Core
Tests error when system has only 1 CPU core.

#### 11. Low Memory Warning - Below 8GB
Tests warning (non-fatal) when memory is between 4-8GB.

#### 12. Low CPU Warning - Below 4 Cores
Tests warning (non-fatal) when CPU has 2-3 cores.

#### 13. Small Root Partition Warning - Below 20GB
Tests warning (non-fatal) when root partition is below 20GB total.

#### 14. Low /var Space Warning
Tests warning (non-fatal) when /var has less than 5GB available.

#### 15. Unsupported Ubuntu Version
Tests warning (non-fatal) when running unsupported Ubuntu version (e.g., 18.04).

#### 16. Not Ubuntu OS
Tests warning (non-fatal) when running on non-Ubuntu OS (e.g., Debian).

#### 17. Kernel Module Not Loaded - overlay
Tests warning (non-fatal) when overlay module not loaded or available.

#### 18. Kernel Module Not Loaded - br_netfilter
Tests warning (non-fatal) when br_netfilter module not loaded or available.

#### 19. GPU Node - amdgpu Module Missing
Tests warning (non-fatal) when GPU_NODE=true but amdgpu module unavailable.

## Configuration Requirements

- `ENABLED_STEPS: "ValidateSystemRequirementsStep"`
- Optional: `GPU_NODE: true` (enables GPU module checks)

## Mock Requirements

```yaml
mocks:
  # Scenario 1: Meets all requirements
  "ReadFile./proc/meminfo":
    output: |
      MemTotal:        8388608 kB
      MemFree:         4194304 kB
    error: null

  "ReadFile./proc/cpuinfo":
    output: |
      processor       : 0
      processor       : 1
      processor       : 2
      processor       : 3
    error: null

  "ReadFile./etc/os-release":
    output: |
      ID=ubuntu
      VERSION_ID="22.04"
    error: null

  "Statfs./":
    # Mock returns 50GB total, 30GB available
    # Implementation uses syscall.Statfs_t struct
    error: null

  "IsModuleLoaded.Lsmod":
    output: |
      Module                  Size  Used by
      overlay               151552  0
      br_netfilter           28672  0
    error: null

  # Scenario 8: Insufficient disk space
  "Statfs./":
    # Mock returns 8GB available (below 10GB minimum)
    error: null

  # Scenario 9: Insufficient memory
  "ReadFile./proc/meminfo":
    output: |
      MemTotal:        3145728 kB
    error: null

  # Scenario 10: Insufficient CPU
  "ReadFile./proc/cpuinfo":
    output: |
      processor       : 0
    error: null

  # Scenario 15: Unsupported Ubuntu
  "ReadFile./etc/os-release":
    output: |
      ID=ubuntu
      VERSION_ID="18.04"
    error: null

  # Scenario 16: Not Ubuntu
  "ReadFile./etc/os-release":
    output: |
      ID=debian
      VERSION_ID="11"
    error: null

  # Scenario 17: Module not loaded
  "IsModuleLoaded.Lsmod":
    output: |
      Module                  Size  Used by
      br_netfilter           28672  0
    error: null  # overlay missing

  "IsModuleAvailable.Modinfo":
    output: ""
    error: "modinfo: ERROR: Module overlay not found."
```

## Running Tests

```bash
# Test 1: Meets all requirements
./cluster-bloom cli --config step_integration_tests/02_ValidateSystemRequirementsStep/01-meets-all-requirements/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/02_ValidateSystemRequirementsStep/01-meets-all-requirements/mocks.yaml

# Test 8: Insufficient disk
./cluster-bloom cli --config step_integration_tests/02_ValidateSystemRequirementsStep/08-insufficient-disk/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/02_ValidateSystemRequirementsStep/08-insufficient-disk/mocks.yaml

# Test 9: Insufficient memory
./cluster-bloom cli --config step_integration_tests/02_ValidateSystemRequirementsStep/09-insufficient-memory/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/02_ValidateSystemRequirementsStep/09-insufficient-memory/mocks.yaml
```

## Expected Outcomes

### Success Cases
- ✅ Disk space >= 10GB available (root), >= 5GB (/var recommended)
- ✅ Memory >= 4GB (8GB recommended)
- ✅ CPU >= 2 cores (4 recommended)
- ✅ Ubuntu 20.04, 22.04, or 24.04
- ✅ Kernel modules available (warnings only)
- ✅ Step completes successfully

### Failure Cases
- ❌ Insufficient disk space stops execution
- ❌ Insufficient memory stops execution
- ❌ Insufficient CPU cores stops execution
- ⚠️ Non-fatal warnings for:
  - Below recommended specs
  - Unsupported Ubuntu version
  - Non-Ubuntu OS
  - Missing kernel modules

## Related Code
- Step implementation: `pkg/steps.go:76-87`
- Main function: `pkg/sysvalidation/sysvalidation.go:ValidateResourceRequirements()`
- Disk validation: `pkg/sysvalidation/sysvalidation.go:validateDiskSpace()`
- Resource validation: `pkg/sysvalidation/sysvalidation.go:validateSystemResources()`
- Ubuntu validation: `pkg/sysvalidation/sysvalidation.go:validateUbuntuVersion()`
- Module validation: `pkg/sysvalidation/sysvalidation.go:validateKernelModules()`

## Notes
- **Minimum requirements**: Based on Kubernetes baseline (4GB RAM, 2 cores)
- **Recommended specs**: For production workloads (8GB RAM, 4 cores)
- **Disk space**:
  - Root: 20GB recommended for OS, Kubernetes binaries, container images
  - /var: 5GB+ for container logs and data
- **Memory check**: Reads /proc/meminfo MemTotal field
- **CPU check**: Counts "processor" lines in /proc/cpuinfo
- **Ubuntu versions**: 20.04 (Focal), 22.04 (Jammy), 24.04 (Noble)
- **Kernel modules**:
  - `overlay`: Container filesystem (overlay2 storage driver)
  - `br_netfilter`: Bridge netfilter for Kubernetes networking
  - `amdgpu`: AMD GPU driver (GPU nodes only)
- **Non-fatal checks**: Module checks and below-recommended specs generate warnings only
- **Fatal checks**: Disk space, memory, and CPU below minimum stop installation
- **GPU considerations**: Additional amdgpu module check when GPU_NODE=true
- **Validation order**: Disk → Memory/CPU → Ubuntu → Modules
- **Early exit**: Stops at first fatal error (disk/memory/CPU)
- **syscall.Statfs**: Low-level disk space check (more reliable than df command)
