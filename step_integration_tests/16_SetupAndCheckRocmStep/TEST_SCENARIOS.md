# SetupAndCheckRocmStep Integration Tests

## Purpose
Install and verify AMD ROCm (Radeon Open Compute) drivers and tools for GPU computing on GPU nodes.

## Step Overview
- **Execution Order**: Step 16
- **Commands Executed**:
  - `which rocm-smi` (check if already installed)
  - `grep VERSION_CODENAME /etc/os-release | cut -d= -f2`
  - `sudo apt update`
  - `uname -r`
  - `sudo apt install linux-headers-<kernel> linux-modules-extra-<kernel>`
  - `sudo apt install python3-setuptools python3-wheel`
  - `wget <ROCM_BASE_URL>/<codename>/<ROCM_DEB_PACKAGE>`
  - `sudo apt install -y ./<ROCM_DEB_PACKAGE>`
  - `sudo amdgpu-install --usecase=rocm,dkms --yes`
  - `modprobe amdgpu`
  - `cat /opt/rocm/.info/version`
  - `rocm-smi -i --json | jq -r '.[] | .["Device Name"]' | sort | uniq -c`
- **Skip Conditions**:
  - Skipped if `GPU_NODE=false`

## What This Step Does
1. Checks if `rocm-smi` already exists (skip install if present)
2. If not installed:
   - Detects Ubuntu codename (focal, jammy, noble)
   - Updates apt package lists
   - Installs kernel headers and modules for current kernel
   - Installs Python dependencies (setuptools, wheel)
   - Downloads amdgpu-install DEB package from ROCm repository
   - Installs amdgpu-install package
   - Runs `amdgpu-install --usecase=rocm,dkms --yes` to install ROCm stack
   - Loads amdgpu kernel module
3. Prints ROCm version from `/opt/rocm/.info/version`
4. Runs `rocm-smi` to list and count detected GPU devices
5. Validates that GPUs are detected (first field should be integer count)

## Test Scenarios

### Success Scenarios

#### 1. ROCm Already Installed - Skip Installation
Tests successful verification when rocm-smi already exists.

#### 2. Fresh ROCm Installation - Ubuntu 22.04 (jammy)
Tests complete installation flow on Ubuntu 22.04.

#### 3. Fresh ROCm Installation - Ubuntu 24.04 (noble)
Tests complete installation flow on Ubuntu 24.04.

#### 4. Multiple GPUs Detected
Tests successful detection and counting of multiple AMD GPUs.

#### 5. Single GPU Detected
Tests successful detection of single AMD GPU.

### Failure Scenarios

#### 6. Failed to Get Ubuntu Codename
Tests error handling when VERSION_CODENAME cannot be read.

#### 7. apt update Fails
Tests error handling when package list update fails.

#### 8. Kernel Headers Installation Fails
Tests error handling when linux-headers installation fails.

#### 9. Python Dependencies Installation Fails
Tests error handling when python3-setuptools installation fails.

#### 10. wget Download Fails
Tests error handling when ROCm DEB package download fails.

#### 11. DEB Package Installation Fails
Tests error handling when amdgpu-install package install fails.

#### 12. amdgpu-install Fails
Tests error handling when main ROCm installation fails.

#### 13. modprobe amdgpu Fails
Tests error handling when amdgpu module cannot be loaded.

#### 14. rocm-smi Shows No GPUs
Tests error handling when rocm-smi runs but detects no GPUs.

## Configuration Requirements

- `ENABLED_STEPS: "SetupAndCheckRocmStep"`
- `GPU_NODE: true` (required, otherwise step is skipped)
- `ROCM_BASE_URL: "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/"` (default)
- `ROCM_DEB_PACKAGE: "amdgpu-install_6.3.60302-1_all.deb"` (default)

## Mock Requirements

```yaml
mocks:
  # Scenario 1: Already installed
  "LookPath.rocm-smi":
    output: "/usr/bin/rocm-smi"
    error: null

  "PrintROCMVersion.CatVersion":
    output: "6.3.2.60302-1\n"
    error: null

  "SetupAndCheckRocmStep.RocmSmi":
    output: |
      2 AMD Instinct MI250X
    error: null

  # Scenario 2: Fresh installation
  "LookPath.rocm-smi":
    output: ""
    error: "rocm-smi not found in PATH"

  "CheckAndInstallROCM.GetCodename":
    output: "jammy\n"
    error: null

  "CheckAndInstallROCM.AptUpdate":
    output: |
      Hit:1 http://archive.ubuntu.com/ubuntu jammy InRelease
      Reading package lists...
    error: null

  "CheckAndInstallROCM.Uname":
    output: "5.15.0-97-generic\n"
    error: null

  "CheckAndInstallROCM.InstallKernelModules":
    output: |
      Reading package lists...
      Building dependency tree...
      linux-headers-5.15.0-97-generic is already installed
    error: null

  "CheckAndInstallROCM.InstallPythonDeps":
    output: |
      python3-setuptools is already installed
    error: null

  "CheckAndInstallROCM.WgetDebFile":
    output: |
      Saving to: 'amdgpu-install_6.3.60302-1_all.deb'
      amdgpu-install_6.3.60302-1_all.deb        100%[=====>]  28.3M  5.2MB/s    in 6s
    error: null

  "CheckAndInstallROCM.InstallDebPackage":
    output: |
      Selecting previously unselected package amdgpu-install
      Unpacking amdgpu-install...
      Setting up amdgpu-install...
    error: null

  "CheckAndInstallROCM.AmdgpuInstall":
    output: |
      Installing ROCm packages...
      Successfully installed rocm-dkms
    error: null

  "CheckAndInstallROCM.Modprobe":
    output: ""
    error: null

  "PrintROCMVersion.CatVersion":
    output: "6.3.2.60302-1\n"
    error: null

  "SetupAndCheckRocmStep.RocmSmi":
    output: |
      1 AMD Instinct MI250
    error: null

  # Scenario 14: No GPUs detected
  "SetupAndCheckRocmStep.RocmSmi":
    output: |
      No AMD GPUs detected
    error: null
```

## Running Tests

```bash
# Test 1: Already installed
./cluster-bloom cli --config step_integration_tests/16_SetupAndCheckRocmStep/01-already-installed/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/16_SetupAndCheckRocmStep/01-already-installed/mocks.yaml

# Test 2: Fresh install
./cluster-bloom cli --config step_integration_tests/16_SetupAndCheckRocmStep/02-fresh-install-jammy/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/16_SetupAndCheckRocmStep/02-fresh-install-jammy/mocks.yaml

# Test 14: No GPUs
./cluster-bloom cli --config step_integration_tests/16_SetupAndCheckRocmStep/14-no-gpus/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/16_SetupAndCheckRocmStep/14-no-gpus/mocks.yaml
```

## Expected Outcomes

### Success Cases
- ✅ ROCm installation verified or completed
- ✅ Kernel modules loaded (amdgpu)
- ✅ ROCm version displayed
- ✅ AMD GPUs detected via rocm-smi
- ✅ GPU count displayed
- ✅ Step completes successfully

### Failure Cases
- ❌ Package download failure stops execution
- ❌ Installation failures stop execution
- ❌ Module load failure stops execution
- ❌ No GPUs detected stops execution

## Related Code
- Step implementation: `pkg/steps.go:504-526`
- Main function: `pkg/rocm.go:CheckAndInstallROCM()`
- Version printer: `pkg/rocm.go:printROCMVersion()`
- ROCm version: Typically 6.3.x
- Installation location: `/opt/rocm/`

## Notes
- **ROCm**: AMD's open-source GPU compute platform (similar to NVIDIA CUDA)
- **GPU requirement**: Only runs on GPU_NODE=true nodes
- **Kernel modules**: Requires matching kernel headers for DKMS module builds
- **Installation size**: ~2GB download, significant install time
- **DEB package**: amdgpu-install is a meta-installer that downloads additional packages
- **Use cases**: `rocm` (compute stack), `dkms` (kernel module building)
- **Python dependencies**: Required by some ROCm tools and installers
- **rocm-smi**: System Management Interface for AMD GPUs (similar to nvidia-smi)
- **JSON output**: rocm-smi provides detailed GPU info in JSON format
- **Device detection**: Validates that at least one GPU is detected after install
- **Idempotent**: Skips installation if rocm-smi already exists
- **Ubuntu codename mapping**: 20.04=focal, 22.04=jammy, 24.04=noble
- **Version file**: `/opt/rocm/.info/version` contains installed ROCm version
- **Complements**: Works with UpdateModprobeStep (unblacklists amdgpu module)
