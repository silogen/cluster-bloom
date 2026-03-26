# AMD GPU Support with ROCm

## Overview

ClusterBloom provides automated AMD GPU support through ROCm driver installation and configuration, enabling GPU-accelerated workloads on Kubernetes clusters.

## Components

### ROCm Installation
Automated installation of ROCm drivers and runtime components:
- **Driver Version**: Configurable via `ROCM_BASE_URL` (default: 7.1.1)
- **Components**: amdgpu kernel driver, ROCm runtime, ROCm libraries
- **Dependencies**: Linux kernel headers, Python setuptools
- **Installation Method**: amdgpu-install utility from AMD repositories
- **Management Tool**: amd-smi (ROCm 7.x) replaces deprecated rocm-smi

**Installation Process**:
1. Detect Ubuntu version and kernel version
2. Install required kernel headers and modules
3. Download amdgpu-install package from AMD repository
4. Execute installation with ROCm and DKMS use cases
5. Load amdgpu kernel module
6. Verify installation with amd-smi

### GPU Detection
Validates GPU availability and configuration:
- **Hardware Detection**: Identifies AMD GPU devices via PCI bus
- **Driver Verification**: Checks amdgpu kernel module loading
- **Device Validation**: Verifies /dev/kfd and /dev/dri/renderD* devices
- **amd-smi Check**: Validates ROCm software stack functionality (ROCm 7.x)

**Detection Methods**:
```bash
# PCI device detection
lspci | grep -i 'vga\|display\|3d'

# Kernel module verification
lsmod | grep amdgpu

# Device node verification
ls -l /dev/kfd /dev/dri/renderD*

# ROCm validation (ROCm 7.x)
amd-smi list

# Detailed GPU information
amd-smi list --json
```

### Version Verification
Ensures correct ROCm version is installed:
- **Supported Version**: ROCm 7.1.1 exactly
- **Version Check**: Validates installed version matches requirements
- **Out-of-Date Detection**: Identifies 6.x versions requiring upgrade
- **Unsupported Warning**: Flags 7.2+ versions not yet supported

**Version Check Commands**:
```bash
# Check ROCm version (displayed in amd-smi header)
amd-smi
# Look for "ROCm version: X.X.X" in the first line

# Example output:
# +------------------------------------------------------------------------------+
# | AMD-SMI 26.0.2+39589fda  amdgpu version: 6.14.14  ROCm version: 7.1.1    |
# +------------------------------------------------------------------------------+

# Expected: ROCm version: 7.1.1
```

**Version Status Guide**:
- ✅ **7.1.1** - Correct, required and fully supported
- ⚠️ **Other** - Version mismatch: WARNING issued; install 7.1.1

**Install ROCm 7.1.1**:
```bash
# 1. Remove old installation
sudo amdgpu-uninstall
sudo apt remove --purge amdgpu-install

# 2. Reinstall with 7.1.1
CODENAME=$(grep VERSION_CODENAME /etc/os-release | cut -d= -f2)
wget https://repo.radeon.com/amdgpu-install/7.1.1/ubuntu/$CODENAME/amdgpu-install_7.1.1.70002-1_all.deb
sudo apt install -y ./amdgpu-install_7.1.1.70002-1_all.deb
sudo amdgpu-install --usecase=rocm,dkms --yes

# 3. Reboot and verify
sudo reboot
# After reboot, check version in header:
amd-smi
# Should show: ROCm version: 7.1.1
```

### Device Rules
Configures udev rules for GPU access permissions:
- **Permission Mode**: 0666 for /dev/kfd and /dev/dri/renderD* devices
- **Udev Rules Location**: `/etc/udev/rules.d/70-amdgpu.rules`
- **Access Control**: Allows non-root container access to GPU devices

**Udev Rule Configuration**:
```
KERNEL=="kfd", MODE="0666"
SUBSYSTEM=="drm", KERNEL=="renderD*", MODE="0666"
```

**Rule Application**:
```bash
sudo udevadm control --reload-rules
sudo udevadm trigger
```

### Kernel Module Management
Handles amdgpu module loading and configuration:
- **Module Loading**: Automatic amdgpu module loading at boot
- **Module Parameters**: Configurable via /etc/modprobe.d/
- **Persistence**: Configuration persists across reboots
- **Dependency Management**: Ensures required modules are loaded

**Module Configuration**:
```bash
# Load module
sudo modprobe amdgpu

# Make persistent
echo "amdgpu" | sudo tee -a /etc/modules
```

### Kubernetes Integration
GPU resource exposure and scheduling:
- **Node Labels**: `gpu=true`, `amd.com/gpu=true`
- **Device Plugin**: AMD GPU device plugin for Kubernetes
- **Resource Limits**: GPU resource scheduling (`amd.com/gpu: 1`)
- **Pod Scheduling**: GPU-aware pod placement

**GPU Pod Example**:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: gpu-workload
spec:
  containers:
  - name: rocm-container
    image: rocm/pytorch:latest
    resources:
      limits:
        amd.com/gpu: 1
  nodeSelector:
    gpu: "true"
```

## Architecture

```mermaid
graph TB
    subgraph "ROCm Installation Flow"
        A[Check GPU Node Flag] --> B{GPU_NODE=true?}
        B -->|Yes| C[Detect Ubuntu Version]
        B -->|No| Z[Skip GPU Setup]
        C --> D[Install Kernel Headers]
        D --> E[Download amdgpu-install]
        E --> F[Install ROCm + DKMS]
        F --> G[Load amdgpu Module]
        G --> H[Configure udev Rules]
        H --> I[Verify with rocm-smi]
    end
    
    subgraph "GPU Device Access"
        I --> J[/dev/kfd Device]
        I --> K[/dev/dri/renderD* Devices]
        J --> L[Container GPU Access]
        K --> L
    end
    
    subgraph "Kubernetes GPU Scheduling"
        L --> M[AMD Device Plugin]
        M --> N[GPU Resource Advertisement]
        N --> O[Node Labels]
        O --> P[GPU Pod Scheduling]
    end
    
    style A fill:#4CAF50
    style I fill:#2196F3
    style M fill:#FF9800
    style P fill:#9C27B0
```
