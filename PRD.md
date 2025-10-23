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
- **Additional Node Joining**: Automated process for adding worker/control plane nodes
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
- `DISABLED_STEPS`/`ENABLED_STEPS`: Step execution control
- `CLUSTER_DISKS`: Pre-selected disk devices (also skips NVME drive checks)
- `DOMAIN`: Domain name for cluster ingress configuration
- `USE_CERT_MANAGER`: Enable cert-manager with Let's Encrypt for TLS
- `CERT_OPTION`: Certificate handling when cert-manager disabled ('existing' or 'generate')
- `TLS_CERT`/`TLS_KEY`: Paths to TLS certificate files for ingress
- `OIDC_URL`: OIDC provider URL for authentication

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
sudo ./bloom test
```
- Tests disk selection and mounting operations
- Validates UI components and workflows

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
1. **Testing Coverage**: Limited unit and integration test coverage
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
1. **Enhanced Testing**: Comprehensive test suite development
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

## Conclusion

ClusterBloom represents a specialized solution for organizations requiring reliable, automated Kubernetes cluster deployment with AMD GPU support. While the current implementation covers core deployment scenarios effectively, there are clear opportunities for enhancement in areas such as monitoring, backup, and operational automation. The modular architecture and configuration flexibility provide a solid foundation for future enhancements and broader adoption.