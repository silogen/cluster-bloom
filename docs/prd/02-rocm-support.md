# AMD GPU Support with ROCm

## Overview

ClusterBloom provides automated AMD GPU support through ROCm driver installation and configuration, enabling GPU-accelerated workloads on Kubernetes clusters.

## Components

### ROCm Installation
Automated installation of ROCm drivers and runtime components:
- **Driver Version**: Configurable via `ROCM_BASE_URL` (default: 6.3.2)
- **Components**: amdgpu kernel driver, ROCm runtime, ROCm libraries
- **Dependencies**: Linux kernel headers, Python setuptools
- **Installation Method**: amdgpu-install utility from AMD repositories

**Installation Process**:
1. Detect Ubuntu version and kernel version
2. Install required kernel headers and modules
3. Download amdgpu-install package from AMD repository
4. Execute installation with ROCm and DKMS use cases
5. Load amdgpu kernel module
6. Verify installation with rocm-smi

### GPU Detection
Validates GPU availability and configuration:
- **Hardware Detection**: Identifies AMD GPU devices via PCI bus
- **Driver Verification**: Checks amdgpu kernel module loading
- **Device Validation**: Verifies /dev/kfd and /dev/dri/renderD* devices
- **rocm-smi Check**: Validates ROCm software stack functionality

**Detection Methods**:
```bash
# PCI device detection
lspci | grep -i 'vga\|display\|3d'

# Kernel module verification
lsmod | grep amdgpu

# Device node verification
ls -l /dev/kfd /dev/dri/renderD*

# ROCm validation
rocm-smi
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
