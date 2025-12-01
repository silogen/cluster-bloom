# Product Requirements Document: ClusterBloom

## Executive Summary

ClusterBloom is an automated Kubernetes cluster deployment and configuration tool specifically designed for AMD GPU environments. It streamlines the complex process of setting up production-ready Kubernetes clusters using RKE2, with specialized support for ROCm, Longhorn storage, and multi-node cluster configurations.

## Product Overview

### Purpose
ClusterBloom automates the deployment of Kubernetes clusters with AMD GPU support, eliminating the manual complexity of configuring ROCm, storage management, networking, and cluster joining procedures.

### Target Users
- DevOps Engineers managing AMD GPU workloads
- Platform Teams deploying Kubernetes infrastructure
- Organizations requiring automated cluster provisioning with AMD GPU support
- Teams needing reliable storage configuration with Longhorn

## Core Features

### 1. Automated RKE2 Kubernetes Deployment
- **First Node Setup**: Initializes the primary cluster node with all necessary configurations
- **Additional Node Joining**: Automated process for adding worker nodes or additional control plane nodes
- **Cilium CNI Integration**: Pre-configured with Cilium for advanced networking
- **Audit Logging**: Built-in audit policy configuration for compliance

### 2. AMD GPU Support with ROCm
- **ROCm Installation**: Automated ROCm driver and runtime installation
- **GPU Detection**: Validates GPU availability and configuration
- **Device Rules**: Configures udev rules for GPU access permissions
- **Kernel Module Management**: Handles amdgpu module loading and configuration

### 3. Storage Management with Longhorn
- **Disk Detection**: Automatically identifies and selects available NVMe drives
- **Interactive Disk Selection**: TUI interface for manual disk selection
- **Automated Mounting**: Formats and mounts selected drives with persistence
- **Longhorn Integration**: Configures Longhorn distributed storage system

### 4. Network Configuration
- **MetalLB Load Balancer**: Automated MetalLB installation and configuration
- **IP Address Pool Management**: Dynamic IP pool configuration for services
- **Firewall Configuration**: Opens required ports for cluster communication
- **Multi-path Configuration**: Sets up multipath for storage reliability

### 5. Interactive Terminal UI
- **Real-time Progress Tracking**: Visual progress indicators for all installation steps
- **Live Log Streaming**: Real-time log output with color coding
- **Interactive Options**: Selection screens for disk management and configuration
- **Error Handling**: Clear error reporting with continuation options

### 6. Configuration Management
- **YAML Configuration**: Flexible configuration through YAML files
- **Environment Variables**: Support for environment-based configuration
- **CLI Flags**: Command-line parameter support
- **Validation**: Configuration validation with clear error messages
- **Interactive Wizard**: Step-by-step configuration file generation
- **ConfigMap Creation**: Automatic Kubernetes ConfigMap from bloom configuration

### 7. Node Validation and Testing
- **Proof Command**: Pre-deployment validation of node readiness
- **Connectivity Testing**: Verification of package repository access
- **GPU Availability Check**: Validation of GPU hardware for GPU nodes
- **Port Validation**: Firewall and network port verification
- **Test Mode**: Safe testing of disk operations and UI components

### 8. TLS Certificate Management
- **Cert-Manager Integration**: Automatic TLS certificates via Let's Encrypt
- **Manual Certificate Support**: Option to provide existing certificates
- **Self-Signed Generation**: Automatic self-signed certificate creation
- **Domain Configuration**: Ingress configuration with custom domains

### 9. Web UI and Monitoring Interface
- **Configuration Wizard**: Browser-based configuration form with validation
- **Real-time Monitoring**: Web dashboard showing installation progress and status
- **Error Recovery Interface**: Configuration reconfiguration after failed installations
- **Responsive Design**: Mobile-friendly interface for remote management
- **Form Validation**: Client-side validation with HTML5 patterns and JavaScript
- **Automatic Redirects**: Seamless flow between configuration and monitoring modes

### 10. Comprehensive Configuration Validation
- **Pre-flight Validation**: Validates all configuration before any system modifications
- **URL Validation**: Validates OIDC, ClusterForge, ROCm, and RKE2 installation URLs
- **Network Validation**: IP address and token format validation
- **Step Name Validation**: Ensures step control parameters reference valid steps
- **Conflict Detection**: Identifies incompatible configuration combinations
- **Resource Validation**: Verifies sufficient disk space, memory, and CPU cores
- **OS Compatibility**: Checks Ubuntu version and kernel module availability
- **Detailed Error Messages**: Clear, actionable error messages with suggested fixes

## Technical Architecture

### Core Components

#### Command Structure
- **Root Command** (`cmd/root.go`): Main application entry point with configuration management
- **Demo Command** (`cmd/demo.go`): UI demonstration and testing functionality
- **Version Command** (`cmd/version.go`): Version information display
- **Wizard Command** (`cmd/wizard.go`): Interactive configuration wizard for generating bloom.yaml files
- **Proof Command** (`cmd/proof.go`): Node readiness validation and prerequisite checking
- **Test Command** (`cmd/test.go`): Testing functionality for UI components and disk operations

#### Package Organization
- **Installation Steps** (`pkg/steps.go`): Modular installation step definitions
- **Disk Management** (`pkg/disks.go`): Storage detection and management
- **RKE2 Integration** (`pkg/rke2.go`): Kubernetes cluster setup
- **ROCm Support** (`pkg/rocm.go`): AMD GPU driver management
- **UI Framework** (`pkg/view.go`): Terminal user interface implementation
- **Web Handlers** (`pkg/webhandlers.go`): HTTP handlers for web interface functionality
- **Configuration Maps** (`pkg/configmaps.go`): Kubernetes ConfigMap creation for bloom configuration
- **Package Management** (`pkg/packages.go`): System package installation and management
- **OS Setup** (`pkg/os-setup.go`): Operating system configuration and validation
- **Demo Steps** (`pkg/demosteps.go`): Demonstration steps for testing and UI showcase
- **Validation** (`cmd/validation.go`): Input validation functions for configuration parameters

#### Web UI Architecture

##### Application Modes
The web interface operates in two distinct modes:

1. **Configuration Mode**: Used when no installation is currently running
   - Displays configuration wizard form
   - Handles form validation and submission
   - Triggers installation after configuration save

2. **Monitoring Mode**: Used when monitoring existing installation status
   - Shows real-time installation progress
   - Displays step-by-step execution status (Total, Completed, Running, Failed)
   - Provides "Reconfigure" option for retrying failed installations

##### Key Components (`pkg/webhandlers.go`)

**WebHandlerService Struct**:
```go
type WebHandlerService struct {
    configFile       string           // Path to bloom.yaml configuration file
    prefilledConfig  map[string]interface{}  // Configuration loaded from bloom.log
    steps            []Step           // Installation steps for progress tracking
    startInstallation func() error    // Callback to trigger installation process
}
```

**Critical Handler Functions**:
- `DashboardHandler`: Routes to appropriate interface (config vs monitoring)
- `ConfigWizardHandler`: Serves configuration form with pre-filled values
- `MonitorHandler`: Displays real-time installation monitoring interface
- `ReconfigureHandler`: Switches from monitoring to configuration mode
- `ConfigAPIHandler`: Processes form submissions and triggers installations
- `PrefilledConfigAPIHandler`: Returns pre-filled configuration data from bloom.log

##### Form Validation System
- **HTML5 Pattern Validation**: Uses regex patterns for client-side validation
- **Domain Validation**: Pattern `[a-z0-9]([a-z0-9\-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9\-]*[a-z0-9])?)*`
- **URL Validation**: Standard HTTP/HTTPS URL pattern validation
- **Required Field Validation**: Enforces mandatory configuration parameters
- **JavaScript Validation**: Additional client-side validation before submission

##### Configuration Flow Logic
1. **Application Startup**: Detects existing bloom.log from previous failed installations
2. **Mode Detection**: Automatically enters monitoring mode if bloom.log exists with errors
3. **Configuration Loading**: Pre-fills form with values parsed from bloom.log
4. **Form Submission**: Validates input, saves to bloom.yaml, triggers installation
5. **Installation Monitoring**: Shows real-time progress and error status
6. **Error Recovery**: Provides reconfigure option to retry with modified settings

##### HTTP Routes and Endpoints
- `/`: Main dashboard (redirects based on current mode)
- `/config`: Configuration wizard interface
- `/monitor`: Installation monitoring dashboard
- `/reconfigure`: Switches to configuration mode for retry scenarios
- `/api/config`: REST endpoint for configuration form submission
- `/api/prefilled-config`: Returns pre-filled configuration data
- `/api/steps`: Real-time installation step status
- `/api/variables`: Current configuration variables

##### Installation Integration
The web interface integrates with the core installation system through:
- **Installation Callbacks**: WebHandlerService receives installation trigger functions
- **Progress Monitoring**: Real-time step status updates via API endpoints
- **Log Integration**: Parses bloom.log for configuration recovery
- **State Management**: Coordinates between configuration and monitoring modes

#### Installation Pipeline
The system executes a sequential pipeline of installation steps:

1. **Pre-Kubernetes Steps**:
   - Ubuntu version validation
   - Partition size verification
   - NVMe drive availability check
   - Dependency package installation
   - Longhorn cleanup and disk preparation
   - Multipath and kernel module configuration
   - Disk selection and mounting
   - ROCm installation and validation
   - Network configuration

2. **Kubernetes Setup**:
   - RKE2 installation and configuration
   - Cluster initialization or node joining

3. **Post-Kubernetes Steps**:
   - Longhorn storage system deployment
   - MetalLB load balancer setup
   - Kubeconfig configuration
   - Bloom ConfigMap creation
   - Domain configuration for ingress
   - TLS certificate setup (cert-manager or manual)
   - ClusterForge integration (optional)
   - 1Password secrets integration (optional)

### Configuration System

#### Supported Configuration Variables
- `FIRST_NODE`: Designates first node vs additional nodes
- `CONTROL_PLANE`: Indicates if additional node should be control plane (when FIRST_NODE is false)
- `GPU_NODE`: Enables/disables GPU-specific configurations
- `SERVER_IP`/`JOIN_TOKEN`: Required for additional node joining
- `NO_DISKS_FOR_CLUSTER`: Bypasses disk-related operations
- `CLUSTER_PREMOUNTED_DISKS`: Manual disk specification
- `CLUSTERFORGE_RELEASE`: ClusterForge version specification
- `CF_VALUES`: ClusterForge values file path specification (optional)
- `DISABLED_STEPS`: Comma-separated list of step IDs to skip during installation (mutually exclusive with `ENABLED_STEPS`)
- `ENABLED_STEPS`: Comma-separated list of step IDs to execute (if set, all other steps are skipped; mutually exclusive with `DISABLED_STEPS`)
- `CLUSTER_DISKS`: Pre-selected disk devices (also skips NVME drive checks)
- `DOMAIN`: Domain name for cluster ingress configuration
- `USE_CERT_MANAGER`: Enable cert-manager with Let's Encrypt for TLS
- `CERT_OPTION`: Certificate handling when cert-manager disabled ('existing' or 'generate')
- `TLS_CERT`/`TLS_KEY`: Paths to TLS certificate files for ingress
- `OIDC_URL`: OIDC provider URL for authentication
- `RKE2_EXTRA_CONFIG`: Additional RKE2 configuration in YAML format to append to /etc/rancher/rke2/config.yaml
- `PRELOAD_IMAGES`: Comma-separated list of container images to preload into the cluster
- `SKIP_RANCHER_PARTITION_CHECK`: Skip validation of /var/lib/rancher partition size (useful for CPU-only nodes)
- `ONEPASSWORD_CONNECT_TOKEN`: Token for 1Password Connect integration (optional)
- `ONEPASSWORD_CONNECT_HOST`: Host URL for 1Password Connect service (optional)
- `CERT_MANAGER_EMAIL`: Email address for Let's Encrypt certificate notifications (required when `USE_CERT_MANAGER: true`)

#### Configuration Sources (Priority Order)
1. Command-line flags
2. Configuration file (YAML)
3. Environment variables
4. Default values

## User Experience

### Installation Workflow

#### Configuration Wizard
```bash
./bloom wizard
```
- Interactive wizard for generating bloom.yaml configuration files
- Guides through all configuration options with validation
- Supports both first node and additional node configurations
- Validates inputs including domains, IPs, URLs, and file paths
- Optionally launches bloom with generated configuration

#### Node Validation (Proof Command)
```bash
sudo ./bloom proof
```
- Validates node readiness before cluster deployment
- Checks Ubuntu version compatibility
- Verifies package installation connectivity
- Tests GPU availability (for GPU nodes)
- Validates firewall and port configurations
- Checks inotify configuration

#### First Node Setup
```bash
sudo ./bloom
```
- Interactive TUI guides through all installation steps
- Real-time progress tracking with visual indicators
- Automated disk detection and selection interface
- Generates join command for additional nodes

#### Additional Node Setup
```bash
echo -e 'FIRST_NODE: false\nJOIN_TOKEN: <token>\nSERVER_IP: <ip>' > bloom.yaml
sudo ./bloom --config bloom.yaml
```

#### Demo Mode
```bash
sudo ./bloom demo-ui
```
- Demonstrates UI capabilities without system modifications
- Useful for testing and familiarization

#### Test Mode
```bash
./bloom test [config-file...]
```
- Runs multiple configuration files in sequence for integration testing
- Executes enabled steps with mocked commands for each config
- Outputs structured YAML results showing pass/fail status
- Useful for validating installation steps without system modifications
- Example: `./bloom test tests/integration/step/*/bloom.yaml`

#### UI Testing Framework
```bash
# Start Chrome container for browser-based testing
docker run -d --rm --name chrome-test --net=host -e "PORT=9222" browserless/chrome:latest

# Run all UI tests
go test -v ./tests/ui

# Run specific test categories
go test -v -run TestConfigBasedTests/.*autodetect ./tests/ui
go test -v -run TestConfigBasedTests/.*invalid ./tests/ui
```

**Test Infrastructure Features:**
- **Browser Automation**: chromedp-based testing with headless Chrome
- **Mock System**: Comprehensive command mocking for disk detection and system calls
- **Test Categories**:
  - Valid configuration tests (7 cases)
  - Invalid/validation error tests (4 cases)
  - Disk auto-detection tests (6 cases)
  - End-to-end integration tests (2 cases)
  - Additional node validation tests (3 cases)
- **Structured Test Format**: YAML-based test definitions with `input`/`mocks`/`output` sections
- **Environment Portability**: Tests pass in containerized, development, and bare-metal environments
- **CI/CD Integration**: Automated browser setup in GitHub Actions workflow

**Test Coverage Areas:**
- Form validation with HTML5 patterns
- Field-specific error message display
- Disk auto-detection with various hardware configurations
- Virtual disk filtering (QEMU, VMware)
- Swap partition detection and exclusion
- Dynamic form behavior (conditional field visibility)
- Configuration save vs save-and-install workflows

### System Requirements Validation
ClusterBloom validates system requirements before installation:

1. **Disk Space Requirements**:
   - Root partition: Minimum 20GB required
   - Available space: Minimum 10GB required
   - /var partition: 5GB recommended for container images
   - /var/lib/ partition: 500GB recommended (for rancher directory, can be skipped with `SKIP_RANCHER_PARTITION_CHECK`)

2. **System Resources**:
   - Memory: Minimum 4GB required, 8GB recommended for Kubernetes
   - CPU: Minimum 2 cores required, 4 cores recommended

3. **Ubuntu Version Compatibility**:
   - Supported versions: 20.04, 22.04, 24.04
   - Other distributions may work but are not officially supported

4. **Kernel Module Requirements**:
   - Required: overlay, br_netfilter
   - For GPU nodes: amdgpu module

See [VALIDATION.md](VALIDATION.md) for complete validation system documentation.

#### Web UI Installation Workflow

##### Initial Setup via Web Interface
1. **Access Web Interface**: Navigate to `http://localhost:62078` in browser
2. **Configuration Wizard**: Fill out cluster configuration form with:
   - Node type selection (first node, additional node, control plane)
   - GPU support options
   - Domain configuration for ingress
   - Storage and networking settings
   - Optional ClusterForge and certificate management
3. **Form Validation**: Real-time validation ensures correct input formats
4. **Installation Trigger**: Submit form to generate bloom.yaml and start installation
5. **Automatic Redirect**: Browser redirects to monitoring dashboard after 3 seconds

##### Error Recovery Workflow
1. **Failed Installation Detection**: Application detects existing bloom.log with errors
2. **Monitoring Mode**: Automatically shows installation status with error details
3. **Reconfigure Option**: Click "Reconfigure" button to retry installation
4. **Pre-filled Form**: Configuration form loads with values from previous attempt
5. **Modify and Retry**: Adjust configuration as needed and resubmit
6. **Monitoring Dashboard**: Real-time progress tracking with step-by-step status

##### Monitoring Interface Features
- **Progress Cards**: Visual display of Total Steps, Completed, Running, Failed counts
- **Step Details**: Expandable sections showing individual step progress
- **Error Information**: Clear error messages with troubleshooting hints
- **Log Access**: Recent logs tab for debugging installation issues
- **Refresh Capability**: Manual refresh button for latest status updates

### Error Handling and Recovery
- **Graceful Failures**: Clear error messages with recovery suggestions
- **Step Isolation**: Failed steps don't prevent manual retry
- **State Persistence**: Configuration state maintained across restarts
- **Cleanup Operations**: Automated cleanup of partial installations

## Integration Capabilities

### External Integrations
- **1Password Connect**: Secure secrets management integration
- **ClusterForge**: Automated application deployment platform
- **OIDC Providers**: Authentication provider integration support

### Kubernetes Ecosystem
- **Helm Charts**: Ready for Helm-based application deployment
- **Kubectl Access**: Automated kubeconfig setup for cluster access
- **K9s Integration**: Terminal-based Kubernetes management interface

### CI/CD Pipeline
- **GitHub Actions Integration**: Automated build and release workflow
- **Devbox Build System**: Consistent development environment using Devbox
- **Release Automation**: Automatic binary creation on GitHub release events
- **Version Injection**: Build-time version injection from Git tags

## Current Limitations and Known Issues

### Missing Components
1. **Backup and Recovery**: No automated backup solution for cluster state
2. **Monitoring Stack**: No built-in monitoring (Prometheus/Grafana) deployment
3. **Certificate Management**: Limited certificate lifecycle management
4. **Multi-Cloud Support**: Currently Ubuntu-only, no cloud provider integration
5. **Rolling Updates**: No automated cluster upgrade mechanism

### Incomplete Areas
1. **Network Policies**: Basic networking without advanced policy management
2. **Resource Quotas**: No default resource management policies
3. **High Availability**: Limited HA configuration for etcd/control plane
4. **Disaster Recovery**: No disaster recovery procedures or automation
5. **Scaling Automation**: Manual process for cluster scaling operations

### Technical Debt
1. **Testing Coverage**: Comprehensive browser-based UI testing implemented (22 test cases). Integration testing framework established with mock-based validation. Additional unit test coverage needed for backend components.
2. **Documentation**: Missing detailed operational procedures
3. **Configuration Validation**: Basic validation without comprehensive checks
4. **Log Management**: Basic logging without centralized log aggregation
5. **Performance Tuning**: No performance optimization configurations

### Recently Resolved Issues (Web UI)
1. **Domain Validation Regex**: Fixed HTML5 pattern compatibility for domain validation
   - **Issue**: Browser JavaScript errors with complex regex patterns in HTML5 form validation
   - **Solution**: Simplified domain pattern to `[a-z0-9]([a-z0-9\-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9\-]*[a-z0-9])?)*`
   - **Location**: `pkg/webhandlers.go:798`

2. **Missing API Endpoints in Monitoring Mode**: API routes not available during monitoring
   - **Issue**: Form submissions failed when application was in monitoring mode
   - **Solution**: Added `/api/config` endpoint to monitoring mode server routing
   - **Location**: `cmd/root.go` monitoring mode server setup

3. **Installation Trigger Integration**: Configuration saves didn't trigger installations
   - **Issue**: Web form saved configuration but didn't start installation process
   - **Solution**: Enhanced WebHandlerService with installation callback system
   - **Location**: `pkg/webhandlers.go` WebHandlerService structure and methods

4. **JavaScript Redirect Flow**: Post-submission flow didn't show installation status
   - **Issue**: Users saw configuration screen instead of monitoring dashboard after submission
   - **Solution**: Implemented 3-second JavaScript redirect to `/monitor` endpoint
   - **Location**: `pkg/webhandlers.go` ConfigAPIHandler response

5. **Configuration Recovery**: Failed installations couldn't be easily retried
   - **Issue**: Users had to manually recreate configuration after installation failures
   - **Solution**: Automatic configuration pre-filling from bloom.log parsing
   - **Location**: `pkg/webhandlers.go` PrefilledConfigAPIHandler and configuration loading

## Success Metrics

### Primary Metrics
- **Installation Success Rate**: Target >95% successful first-time installations
- **Time to Cluster**: Target <30 minutes for complete cluster setup
- **User Experience**: Minimal manual intervention required
- **Error Recovery**: Clear error messages with actionable solutions

### Secondary Metrics
- **Node Addition Time**: Target <10 minutes for additional node joining
- **Storage Performance**: Longhorn performance meeting baseline requirements
- **GPU Utilization**: Successful ROCm workload execution
- **Operational Stability**: 99.9% cluster uptime after initial setup

## Future Roadmap

### Near-term Enhancements (3-6 months)
1. **Enhanced Testing**: ✅ Browser-based UI testing complete (22 test cases). Next: Backend unit tests and E2E installation testing
2. **Backup Integration**: Automated backup solution implementation
3. **Monitoring Stack**: Built-in Prometheus/Grafana deployment
4. **Documentation**: Comprehensive operational documentation

### Medium-term Goals (6-12 months)
1. **Multi-OS Support**: CentOS/RHEL compatibility
2. **Cloud Integration**: AWS/Azure/GCP provider support
3. **HA Configuration**: Advanced high-availability setup
4. **Scaling Automation**: Automated cluster scaling capabilities

### Long-term Vision (12+ months)
1. **GitOps Integration**: ArgoCD/Flux integration for application delivery
2. **Multi-Cluster Management**: Centralized multi-cluster management
3. **Advanced Security**: Security policy automation and compliance
4. **Machine Learning Optimization**: ML-driven performance optimization

## Manual Installation Guide

For reference, this section documents the manual steps that ClusterBloom automates. This is useful for:
- Understanding what ClusterBloom does under the hood
- Troubleshooting installation issues
- Performing custom installations outside of ClusterBloom
- Adapting the process for non-Ubuntu systems

### Prerequisites Verification

#### System Requirements Check
1. **Verify Ubuntu Version**:
   ```bash
   lsb_release -a
   # Must be Ubuntu 20.04, 22.04, or 24.04
   ```

2. **Check Disk Space**:
   ```bash
   df -h /
   # Root partition: minimum 20GB
   # /var/lib/rancher: recommended 500GB
   df -h /var
   # /var partition: recommended 5GB for container images
   ```

3. **Verify Memory and CPU**:
   ```bash
   free -h
   # Minimum 4GB RAM, recommended 8GB
   nproc
   # Minimum 2 cores, recommended 4 cores
   ```

4. **Check Kernel Modules**:
   ```bash
   lsmod | grep overlay
   lsmod | grep br_netfilter
   # For GPU nodes:
   lsmod | grep amdgpu
   ```

### First Node Installation

#### Phase 1: System Preparation

1. **Update System and Install Dependencies**:
   ```bash
   sudo apt update
   sudo apt install -y jq nfs-common open-iscsi chrony curl wget
   ```

2. **Configure Firewall Ports**:
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

3. **Configure inotify Limits** (for monitoring many files):
   ```bash
   echo "fs.inotify.max_user_instances = 8192" | sudo tee -a /etc/sysctl.conf
   echo "fs.inotify.max_user_watches = 524288" | sudo tee -a /etc/sysctl.conf
   sudo sysctl -p
   ```

4. **Install Kubernetes Tools**:
   ```bash
   # Install kubectl
   curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
   sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
   
   # Install k9s
   wget https://github.com/derailed/k9s/releases/latest/download/k9s_Linux_amd64.tar.gz
   tar -xzf k9s_Linux_amd64.tar.gz
   sudo mv k9s /usr/local/bin/
   ```

#### Phase 2: Storage Configuration

5. **Configure Multipath** (for disk reliability):
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

6. **Load Required Kernel Modules**:
   ```bash
   sudo modprobe overlay
   sudo modprobe br_netfilter
   
   # Make persistent
   cat <<EOF | sudo tee /etc/modules-load.d/k8s.conf
   overlay
   br_netfilter
   EOF
   ```

7. **Identify and Prepare Disks for Longhorn**:
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

8. **Configure rsyslog** (prevent iSCSI log flooding):
   ```bash
   cat <<EOF | sudo tee /etc/rsyslog.d/30-ratelimit.conf
   # Limit iSCSI messages to prevent log flooding
   :msg, contains, "iSCSI" stop
   EOF
   
   sudo systemctl restart rsyslog
   ```

9. **Configure logrotate**:
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

#### Phase 3: GPU Setup (GPU Nodes Only)

10. **Install ROCm Drivers**:
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

11. **Configure GPU Permissions**:
    ```bash
    cat <<EOF | sudo tee /etc/udev/rules.d/70-amdgpu.rules
    KERNEL=="kfd", MODE="0666"
    SUBSYSTEM=="drm", KERNEL=="renderD*", MODE="0666"
    EOF
    
    sudo udevadm control --reload-rules
    sudo udevadm trigger
    ```

#### Phase 4: RKE2 Kubernetes Installation

12. **Install RKE2**:
    ```bash
    # Download RKE2 installation script
    curl -sfL https://get.rke2.io | sudo sh -
    
    # Create RKE2 configuration directory
    sudo mkdir -p /etc/rancher/rke2
    ```

13. **Configure RKE2 for First Node**:
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

14. **Create Audit Policy** (for compliance):
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

15. **Start RKE2 Service**:
    ```bash
    sudo systemctl enable rke2-server.service
    sudo systemctl start rke2-server.service
    
    # Wait for RKE2 to start (may take 2-5 minutes)
    sudo systemctl status rke2-server.service
    
    # Check logs if needed
    sudo journalctl -u rke2-server -f
    ```

16. **Configure kubectl Access**:
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

17. **Get Join Token for Additional Nodes**:
    ```bash
    sudo cat /var/lib/rancher/rke2/server/node-token
    # Save this token for additional nodes
    
    # Get server IP
    hostname -I | awk '{print $1}'
    ```

#### Phase 5: Storage and Networking Setup

18. **Deploy Longhorn Storage** (First Node Only):
    ```bash
    # Create manifests directory
    sudo mkdir -p /var/lib/rancher/rke2/server/manifests
    
    # Download Longhorn manifests
    # (ClusterBloom includes pre-configured Longhorn manifests)
    # Apply standard Longhorn installation or custom configuration
    
    # Wait for Longhorn pods to be ready
    kubectl wait --for=condition=ready pod -l app=longhorn-manager -n longhorn-system --timeout=600s
    ```

19. **Deploy MetalLB Load Balancer**:
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

20. **Configure Chrony Time Synchronization**:
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

21. **Configure Domain and TLS** (if using custom domain):
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

22. **Deploy ClusterForge** (Optional):
    ```bash
    # Download ClusterForge release
    wget https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz
    tar -xzf deploy-release.tar.gz
    cd deploy-release
    
    # Deploy ClusterForge
    ./deploy.sh
    ```

### Additional Node Installation

For adding worker or control plane nodes to an existing cluster:

1. **Perform System Preparation** (Steps 1-11 from First Node, excluding cluster-specific steps)

2. **Install RKE2 Agent** (for worker nodes):
   ```bash
   curl -sfL https://get.rke2.io | INSTALL_RKE2_TYPE="agent" sudo sh -
   
   # Configure RKE2 agent
   sudo mkdir -p /etc/rancher/rke2
   cat <<EOF | sudo tee /etc/rancher/rke2/config.yaml
   server: https://<FIRST_NODE_IP>:9345
   token: <JOIN_TOKEN>
   EOF
   
   # Start agent service
   sudo systemctl enable rke2-agent.service
   sudo systemctl start rke2-agent.service
   ```

3. **Install RKE2 Server** (for additional control plane nodes):
   ```bash
   curl -sfL https://get.rke2.io | sudo sh -
   
   # Configure RKE2 server
   sudo mkdir -p /etc/rancher/rke2
   cat <<EOF | sudo tee /etc/rancher/rke2/config.yaml
   server: https://<FIRST_NODE_IP>:9345
   token: <JOIN_TOKEN>
   write-kubeconfig-mode: "0644"
   tls-san:
     - $(hostname -I | awk '{print $1}')
   EOF
   
   # Start server service
   sudo systemctl enable rke2-server.service
   sudo systemctl start rke2-server.service
   ```

4. **Configure Chrony** (sync with first node):
   ```bash
   cat <<EOF | sudo tee /etc/chrony/chrony.conf
   server <FIRST_NODE_IP> iburst
   EOF
   
   sudo systemctl restart chrony
   ```

5. **Verify Node Joined**:
   ```bash
   # On first node, check new node status
   kubectl get nodes
   ```

### Post-Installation Verification

1. **Verify All Pods Running**:
   ```bash
   kubectl get pods -A
   ```

2. **Check Longhorn Status**:
   ```bash
   kubectl get pods -n longhorn-system
   ```

3. **Verify MetalLB**:
   ```bash
   kubectl get pods -n metallb-system
   ```

4. **Test PVC Creation** (Longhorn validation):
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

5. **Check GPU Access** (GPU nodes):
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

### Key Differences Between Manual and Automated Installation

ClusterBloom automates all of the above steps and provides additional benefits:

1. **Interactive UI**: TUI and Web UI for configuration and monitoring
2. **Validation**: Pre-flight checks before any system modifications
3. **Error Recovery**: Automatic retry and reconfiguration on failures
4. **State Management**: Tracks installation progress and resumes on interruption
5. **Configuration Management**: YAML-based configuration with validation
6. **Disk Auto-detection**: Intelligent disk selection and formatting
7. **Integration**: Seamless ClusterForge and 1Password Connect integration
8. **Monitoring**: Real-time progress tracking and detailed logging
9. **Multi-node Coordination**: Automatic generation of join commands
10. **Best Practices**: Built-in configurations following Kubernetes best practices

## Cloud Platform Compatibility

### Overview

ClusterBloom is designed for on-premises bare-metal Kubernetes deployments with RKE2. However, applications deployed on top of ClusterBloom may need to run on managed Kubernetes services like EKS, AKS, or GKE. This section outlines the infrastructure dependencies, potential conflicts, and configuration variables needed to make applications portable across platforms.

### Core Infrastructure Components

ClusterBloom installs and configures the following infrastructure components that have cloud-managed equivalents:

#### 1. Kubernetes Distribution: RKE2
**ClusterBloom Default**: RKE2 (Rancher Kubernetes Engine 2) with Cilium CNI
- **Cluster CIDR**: `10.242.0.0/16`
- **Service CIDR**: `10.243.0.0/16`
- **CNI**: Cilium with VXLAN (port 8472/udp)
- **Ingress**: rke2-ingress-nginx disabled (applications provide their own)

**Cloud Equivalents**:
- **EKS**: AWS-managed control plane with AWS VPC CNI
- **AKS**: Azure-managed control plane with Azure CNI or Kubenet
- **GKE**: Google-managed control plane with GKE CNI

**Configuration Variables for Application Portability**:
```yaml
# Application should detect and adapt to these differences
KUBERNETES_SERVICE_HOST: # Auto-populated by Kubernetes
KUBERNETES_SERVICE_PORT: # Auto-populated by Kubernetes
CLUSTER_CIDR: # May differ per platform
SERVICE_CIDR: # May differ per platform
CNI_PROVIDER: # "cilium" (ClusterBloom), "aws-vpc-cni" (EKS), "azure-cni" (AKS), etc.
```

#### 2. Storage: Longhorn Distributed Storage
**ClusterBloom Default**: Longhorn v1.8.0
- **Storage Class**: `mlstorage` (default)
- **Provisioner**: `driver.longhorn.io`
- **Features**: Distributed block storage, snapshots, backups, replicas
- **Node Requirements**: Disks at `/mnt/diskX` or `CLUSTER_PREMOUNTED_DISKS` paths
- **Default Settings**: 
  - `create-default-disk-labeled-nodes: true`
  - `priority-class: longhorn-critical`
  - `disable-revision-counter: true`

**Cloud Equivalents**:
- **EKS**: AWS EBS CSI driver (`ebs.csi.aws.com`)
- **AKS**: Azure Disk CSI driver (`disk.csi.azure.com`)
- **GKE**: Google Persistent Disk CSI driver (`pd.csi.storage.gke.io`)

**Potential Conflicts**:
1. **Storage Class Names**: Applications hardcoding `mlstorage` will fail on cloud platforms
2. **Volume Modes**: Longhorn supports both Block and Filesystem; cloud providers may have restrictions
3. **Access Modes**: RWX (ReadWriteMany) behavior differs between Longhorn and cloud providers
4. **Performance Characteristics**: IOPS, throughput, and latency vary significantly
5. **Backup/Snapshot APIs**: Longhorn's backup system differs from cloud-native snapshot mechanisms

**Configuration Variables Needed**:
```yaml
# Storage configuration abstraction
STORAGE_CLASS_NAME: "mlstorage" # ClusterBloom default
# Cloud alternatives:
# - "gp3" or "gp2" (EKS with EBS)
# - "managed-csi" or "azuredisk-premium" (AKS)
# - "standard-rwo" or "premium-rwo" (GKE)

STORAGE_PROVISIONER: "driver.longhorn.io" # ClusterBloom
# Cloud alternatives:
# - "ebs.csi.aws.com" (EKS)
# - "disk.csi.azure.com" (AKS)
# - "pd.csi.storage.gke.io" (GKE)

VOLUME_BINDING_MODE: "Immediate" # or "WaitForFirstConsumer"
ALLOW_VOLUME_EXPANSION: true
RECLAIM_POLICY: "Delete" # or "Retain"

# Longhorn-specific features (not available in cloud)
LONGHORN_BACKUP_TARGET: "" # S3/NFS backup location
LONGHORN_REPLICA_COUNT: 3
LONGHORN_DATA_LOCALITY: "disabled" # or "best-effort"
```

#### 3. Load Balancer: MetalLB
**ClusterBloom Default**: MetalLB v0.14.9 with L2 Advertisement
- **IP Address Pool**: Single node IP (`<NODE_IP>/32`)
- **Mode**: Layer 2 (ARP-based)
- **Namespace**: `metallb-system`
- **CRDs**: `IPAddressPool`, `L2Advertisement`, `BGPPeer`, `BGPAdvertisement`

**Cloud Equivalents**:
- **EKS**: AWS Network Load Balancer (NLB) or Classic Load Balancer (CLB)
- **AKS**: Azure Load Balancer
- **GKE**: Google Cloud Load Balancer

**Potential Conflicts**:
1. **Service Type LoadBalancer**: MetalLB provides local IPs; clouds provide public IPs
2. **External DNS**: MetalLB IPs are not internet-routable; cloud LBs integrate with DNS
3. **Health Checks**: Different health check mechanisms and configurations
4. **SSL Termination**: Cloud load balancers offer native SSL termination
5. **Annotations**: MetalLB-specific vs cloud-specific service annotations

**Configuration Variables Needed**:
```yaml
# Load balancer abstraction
LOAD_BALANCER_TYPE: "metallb" # ClusterBloom
# Cloud alternatives: "aws-nlb", "aws-clb", "azure-lb", "gce-lb"

# MetalLB-specific
METALLB_IP_POOL: "cluster-bloom-ip-pool"
METALLB_ADVERTISEMENT_MODE: "l2" # or "bgp"

# Cloud-specific service annotations
# EKS:
service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
service.beta.kubernetes.io/aws-load-balancer-scheme: "internet-facing" # or "internal"
service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled: "true"

# AKS:
service.beta.kubernetes.io/azure-load-balancer-internal: "false" # or "true"
service.beta.kubernetes.io/azure-load-balancer-resource-group: "<resource-group>"

# GKE:
cloud.google.com/load-balancer-type: "External" # or "Internal"
```

#### 4. Ingress Controller
**ClusterBloom Default**: None (disabled `rke2-ingress-nginx`)
- Applications expected to provide their own ingress controller

**Cloud Equivalents**:
- **EKS**: AWS Load Balancer Controller (ALB Ingress)
- **AKS**: Azure Application Gateway Ingress Controller
- **GKE**: GKE Ingress (GCE)

**Potential Conflicts**:
1. **Ingress Class**: Different ingress controllers require different `ingressClassName`
2. **Annotations**: Platform-specific annotations for features
3. **TLS Termination**: Cloud ingress controllers integrate with certificate management
4. **Path-based Routing**: Behavior differences in path matching

**Configuration Variables Needed**:
```yaml
INGRESS_CLASS_NAME: "nginx" # Common default
# Cloud alternatives:
# - "alb" (EKS with AWS Load Balancer Controller)
# - "azure/application-gateway" (AKS)
# - "gce" (GKE)

INGRESS_CONTROLLER_TYPE: "none" # ClusterBloom (app provides own)
# - "nginx", "traefik", "aws-alb", "azure-appgw", "gce"

# Certificate management
CERT_MANAGER_ENABLED: true # ClusterBloom option
TLS_SECRET_NAME: "cluster-tls"
```

#### 5. Certificate Management
**ClusterBloom Default**: Optional cert-manager with Let's Encrypt or manual certificates
- **Cert-Manager Version**: v1.13.0 (if enabled)
- **Issuers**: Let's Encrypt production/staging
- **Email**: `CERT_MANAGER_EMAIL` (required for Let's Encrypt)

**Cloud Equivalents**:
- **EKS**: AWS Certificate Manager (ACM) integration
- **AKS**: Azure Key Vault integration
- **GKE**: Google-managed certificates

**Potential Conflicts**:
1. **Certificate Provisioning**: cert-manager vs cloud-native certificate services
2. **DNS-01 Challenges**: Different DNS provider integrations
3. **Certificate Storage**: Kubernetes secrets vs cloud key vaults
4. **Automatic Renewal**: Different renewal mechanisms

**Configuration Variables Needed**:
```yaml
CERTIFICATE_PROVIDER: "cert-manager" # ClusterBloom with cert-manager
# Cloud alternatives:
# - "aws-acm" (EKS)
# - "azure-keyvault" (AKS)
# - "google-managed-cert" (GKE)
# - "manual" (bring your own)

CERT_MANAGER_ISSUER: "letsencrypt-prod" # or "letsencrypt-staging"
CERT_MANAGER_EMAIL: "admin@example.com"
ACME_SERVER: "https://acme-v02.api.letsencrypt.org/directory"

# Cloud-specific
AWS_ACM_CERTIFICATE_ARN: "" # For EKS
AZURE_KEYVAULT_NAME: "" # For AKS
GCP_MANAGED_CERT_NAME: "" # For GKE
```

#### 6. Container Network Interface (CNI)
**ClusterBloom Default**: Cilium
- **Features**: Network policies, encryption, observability, service mesh
- **VXLAN Port**: 8472/udp
- **Health Check Port**: 4240/tcp
- **Mode**: VXLAN overlay network

**Cloud Equivalents**:
- **EKS**: AWS VPC CNI (native VPC networking)
- **AKS**: Azure CNI or Kubenet
- **GKE**: GKE CNI (VPC-native)

**Potential Conflicts**:
1. **Network Policies**: Different CNI plugins have varying NetworkPolicy support
2. **Pod CIDR**: Cloud CNIs may use VPC subnets directly
3. **Security Groups**: Cloud platforms use native security group integration
4. **Service Mesh**: Cilium provides service mesh; clouds may use separate solutions (Istio, Linkerd)
5. **Observability**: Cilium Hubble vs cloud-native network monitoring

**Configuration Variables Needed**:
```yaml
CNI_PLUGIN: "cilium" # ClusterBloom
# Cloud defaults:
# - "aws-vpc-cni" (EKS)
# - "azure-cni" or "kubenet" (AKS)
# - "gke-cni" (GKE)

NETWORK_POLICY_ENABLED: true
ENCRYPTION_ENABLED: false # Cilium encryption
SERVICE_MESH_ENABLED: false # Cilium service mesh mode

# Pod networking
POD_CIDR: "10.242.0.0/16" # ClusterBloom/RKE2
# Cloud platforms may assign from VPC subnets
```


#### 7. GPU Support: ROCm TODO update versions
**ClusterBloom Default**: AMD ROCm drivers and device plugins
- **Driver Version**: 6.3.2 (configurable via `ROCM_BASE_URL`)
- **Device Plugin**: Installed for GPU nodes
- **Node Labels**: `gpu=true`, `amd.com/gpu=true`
- **udev Rules**: Configured for `/dev/kfd` and `/dev/dri/renderD*`

**Cloud Equivalents TODO**:
- **EKS**: AMD GPU device plugins
- **AKS**: GPU-enabled node pools with pre-installed drivers
- **GKE**: GPU node pools with driver installation

**Potential Conflicts**:
1. **Driver Installation**: ClusterBloom installs on bare metal; clouds provide pre-configured images
2. **Device Plugin**: Different device plugin versions and configurations
3. **Node Selectors**: Label and taint differences for GPU scheduling
4. **Resource Limits**: GPU resource naming (`amd.com/gpu` vs vendor-specific)

**Configuration Variables Needed**:
```yaml
GPU_NODE: true # ClusterBloom
GPU_VENDOR: "amd" # or "nvidia"
GPU_DRIVER_VERSION: "6.3.2" # ROCm version

# Cloud GPU node configuration
GPU_NODE_POOL: "gpu-pool" # Cloud node pool name
GPU_INSTANCE_TYPE: "g4ad.xlarge" # EKS, varies by cloud
GPU_RESOURCE_NAME: "amd.com/gpu" # or "nvidia.com/gpu"

# Node labels and taints
GPU_NODE_LABEL: "gpu=true"
GPU_NODE_TAINT: "nvidia.com/gpu=NoSchedule" # Example
```

### Application Compatibility Strategy

To ensure applications work across ClusterBloom (bare-metal) and cloud platforms, implement the following:

#### 1. Storage Abstraction TODO check other storages
```yaml
# Use environment variables or ConfigMaps
apiVersion: v1
kind: ConfigMap
metadata:
  name: platform-config
data:
  storageClassName: ${STORAGE_CLASS_NAME:-mlstorage}
  storageProvisioner: ${STORAGE_PROVISIONER:-driver.longhorn.io}
  volumeBindingMode: ${VOLUME_BINDING_MODE:-Immediate}
```

#### 2. Service Abstraction
```yaml
# Conditional service annotations
apiVersion: v1
kind: Service
metadata:
  name: app-service
  annotations:
    # MetalLB (ClusterBloom)
    ${METALLB_ENABLED:+metallb.universe.tf/address-pool}: ${METALLB_IP_POOL}
    # AWS (EKS)
    ${AWS_LB_ENABLED:+service.beta.kubernetes.io/aws-load-balancer-type}: ${AWS_LB_TYPE}
    # Azure (AKS)
    ${AZURE_LB_ENABLED:+service.beta.kubernetes.io/azure-load-balancer-internal}: ${AZURE_LB_INTERNAL}
```

#### 3. Ingress Abstraction
```yaml
# Use platform-aware ingress class
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: app-ingress
spec:
  ingressClassName: ${INGRESS_CLASS_NAME:-nginx}
  # Platform-specific annotations via templates
```

#### 4. Node Selection Strategy
```yaml
# Platform-aware node affinity
apiVersion: v1
kind: Pod
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        # ClusterBloom GPU nodes
        - matchExpressions:
          - key: ${GPU_NODE_LABEL_KEY:-gpu}
            operator: In
            values: ["true"]
        # Cloud GPU nodes (use instance type labels)
        - matchExpressions:
          - key: ${CLOUD_INSTANCE_TYPE_LABEL:-node.kubernetes.io/instance-type}
            operator: In
            values: ${GPU_INSTANCE_TYPES}
```

### Recommended Configuration Pattern

Create a platform detection and configuration system:

```yaml
# Platform detection ConfigMap
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-platform-config
data:
  # Auto-detected or manually configured
  platform: "clusterbloom" # or "eks", "aks", "gke"
  
  # Storage configuration
  storage.class: "mlstorage" # platform-specific
  storage.provisioner: "driver.longhorn.io"
  storage.volumeBindingMode: "Immediate"
  
  # Networking configuration
  loadbalancer.type: "metallb"
  ingress.class: "nginx"
  cni.plugin: "cilium"
  
  # Certificate management
  cert.provider: "cert-manager"
  
  # GPU configuration (if applicable)
  gpu.enabled: "true"
  gpu.vendor: "amd"
  gpu.resource: "amd.com/gpu"
```

### Migration Checklist

When migrating applications between ClusterBloom and cloud platforms:

**Storage**:
- [ ] Replace hardcoded `mlstorage` with environment variable
- [ ] Update PVC volume binding mode if needed
- [ ] Migrate backup/snapshot mechanisms
- [ ] Test RWX volumes (behavior differs significantly)

**Networking**:
- [ ] Update Service LoadBalancer annotations
- [ ] Update Ingress controller class and annotations
- [ ] Verify DNS integration
- [ ] Test SSL/TLS termination

**Compute**:
- [ ] Update GPU resource requests/limits
- [ ] Modify node selectors and affinity rules
- [ ] Update pod security policies/pod security standards
- [ ] Verify resource quotas and limits

**Security**:
- [ ] Update certificate provisioning mechanism
- [ ] Migrate secrets to cloud key vaults (if applicable)
- [ ] Update network policies for cloud CNI
- [ ] Review RBAC for cloud-specific resources

**Monitoring & Observability**:
- [ ] Replace Cilium Hubble with cloud monitoring (if used)
- [ ] Update metric collection endpoints
- [ ] Modify logging aggregation
- [ ] Update tracing configuration

## Conclusion

ClusterBloom represents a specialized solution for organizations requiring reliable, automated Kubernetes cluster deployment with AMD GPU support. While the current implementation covers core deployment scenarios effectively, there are clear opportunities for enhancement in areas such as monitoring, backup, and operational automation. The modular architecture and configuration flexibility provide a solid foundation for future enhancements and broader adoption.