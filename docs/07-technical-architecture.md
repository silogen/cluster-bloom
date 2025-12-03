# Technical Architecture

## Overview

This document provides detailed technical architecture information for ClusterBloom, including component organization, package structure, and system integration patterns.

## Core Components

### Command Structure

#### Root Command (`cmd/root.go`)
- Main application entry point
- Configuration management and loading
- Coordinates installation pipeline execution
- Web server initialization and routing

#### Demo Command (`cmd/demo.go`)
- UI demonstration functionality
- Testing mode for components
- Non-destructive operation testing

#### Version Command (`cmd/version.go`)
- Version information display
- Build metadata reporting
- Git commit and tag information

#### Wizard Command (`cmd/wizard.go`)
- Interactive configuration generation
- Step-by-step wizard interface
- bloom.yaml file creation
- Input validation and defaults

#### Proof Command (`cmd/proof.go`)
- Pre-deployment validation
- Node readiness verification
- Prerequisite checking
- Connectivity testing

#### Test Command (`cmd/test.go`)
- Integration testing framework
- Mock-based validation
- Multi-configuration testing
- YAML result output

### Package Organization

#### Installation Steps (`pkg/steps.go`)
- Modular step definitions
- Dependency ordering
- Pre/post Kubernetes separation
- Step state management
- DISABLED_STEPS/ENABLED_STEPS filtering

#### Disk Management (`pkg/disks.go`)
- Storage device detection
- Disk formatting and mounting
- UUID-based fstab management
- Interactive disk selection UI
- Auto-detection with virtual disk filtering

#### RKE2 Integration (`pkg/rke2.go`)
- Kubernetes cluster initialization
- Node joining procedures
- Token generation and management
- Cilium CNI configuration
- Audit logging setup

#### ROCm Support (`pkg/rocm.go`)
- AMD GPU driver installation
- Device detection and validation
- udev rule configuration
- Kernel module management
- ROCm version management

#### UI Framework (`pkg/view.go`)
- Terminal user interface
- Bubble Tea integration
- Progress tracking components
- Log streaming views
- Interactive dialogs

#### Web Handlers (`pkg/webhandlers.go`)
- HTTP request handling
- Configuration wizard API
- Monitoring dashboard API
- Real-time progress endpoints
- Form validation and submission

#### Configuration Maps (`pkg/configmaps.go`)
- Kubernetes ConfigMap creation
- bloom.yaml to ConfigMap conversion
- Cluster configuration persistence

#### Package Management (`pkg/packages.go`)
- System package installation
- Dependency resolution
- APT package management
- Version verification

#### OS Setup (`pkg/os-setup.go`)
- Operating system validation
- System configuration
- Kernel module loading
- Firewall configuration
- Time synchronization

#### Demo Steps (`pkg/demosteps.go`)
- Demonstration step definitions
- UI showcase functionality
- Safe testing operations

#### Validation (`cmd/validation.go`)
- Configuration input validation
- URL format verification
- IP address validation
- Domain name checking
- File path validation
- Conflict detection

## Installation Pipeline

### Pipeline Execution Model

The installation system uses a sequential pipeline approach with three distinct phases:

1. **Pre-Kubernetes Phase**: System preparation and configuration
2. **Kubernetes Setup Phase**: RKE2 cluster deployment
3. **Post-Kubernetes Phase**: Add-on installation and configuration

### Step Categories

#### Pre-Kubernetes Steps
Execute before Kubernetes cluster deployment:

- **System Validation**: Ubuntu version, disk space, resources
- **Dependency Installation**: Required system packages
- **Storage Preparation**: Longhorn cleanup, disk detection, formatting, mounting
- **Network Configuration**: Firewall rules, multipath, kernel modules
- **GPU Setup**: ROCm installation and validation (GPU nodes only)
- **Time Synchronization**: Chrony NTP configuration

#### Kubernetes Setup
Core cluster deployment:

- **RKE2 Installation**: Download and install RKE2 binaries
- **Cluster Initialization**: First node cluster bootstrap
- **Node Joining**: Additional node agent/server setup
- **CNI Deployment**: Cilium network plugin

#### Post-Kubernetes Steps
Add-ons and integrations after cluster is running:

- **Longhorn Deployment**: Distributed storage system
- **MetalLB Installation**: Load balancer configuration
- **Kubeconfig Setup**: kubectl access configuration
- **ConfigMap Creation**: bloom configuration persistence
- **Domain Configuration**: Ingress domain setup
- **Certificate Management**: TLS certificate provisioning
- **ClusterForge Integration**: Application platform deployment
- **1Password Integration**: Secrets management

### Step Control Mechanisms

#### Step Filtering
Two mutually exclusive mechanisms for controlling step execution:

**DISABLED_STEPS**:
```yaml
DISABLED_STEPS: "install-longhorn,install-metallb"
```
- Skips specified steps
- All other steps execute normally
- Comma-separated step IDs

**ENABLED_STEPS**:
```yaml
ENABLED_STEPS: "install-rke2,configure-kubeconfig"
```
- Executes ONLY specified steps
- All other steps are skipped
- Useful for targeted operations
- Mutually exclusive with DISABLED_STEPS

#### Conditional Execution

Steps may execute conditionally based on:

- **Node Type**: FIRST_NODE vs additional nodes
- **GPU Configuration**: GPU_NODE flag
- **Disk Configuration**: NO_DISKS_FOR_CLUSTER flag
- **Feature Flags**: USE_CERT_MANAGER, CLUSTERFORGE_RELEASE, etc.

## Web UI Architecture

### Application Modes

#### Configuration Mode
Active when no installation is running:

- Displays configuration wizard form
- Validates user input
- Generates bloom.yaml
- Triggers installation

#### Monitoring Mode
Active during installation execution:

- Real-time progress display
- Step status tracking
- Log streaming
- Error reporting
- Reconfiguration option

### WebHandlerService Structure

```go
type WebHandlerService struct {
    configFile        string                     // bloom.yaml path
    prefilledConfig   map[string]interface{}    // Loaded from bloom.log
    steps             []Step                     // Installation step definitions
    startInstallation func() error               // Installation trigger callback
}
```

### HTTP Endpoints

#### Dashboard Routes
- `/`: Main entry point (mode-based redirect)
- `/config`: Configuration wizard interface
- `/monitor`: Installation monitoring dashboard
- `/reconfigure`: Switch to configuration mode

#### API Routes
- `/api/config`: Configuration submission endpoint
- `/api/prefilled-config`: Pre-filled configuration data
- `/api/steps`: Real-time step status
- `/api/variables`: Current configuration variables

### Form Validation System

#### Client-Side Validation
- HTML5 pattern attributes
- JavaScript validation
- Required field enforcement
- Type checking (URL, IP, domain)

#### Server-Side Validation
- Configuration structure validation
- Value format verification
- Conflict detection
- Resource requirement checks

### Configuration Flow

1. **Startup Detection**: Check for existing bloom.log
2. **Mode Selection**: Configuration vs Monitoring
3. **Form Rendering**: Pre-fill from bloom.log if available
4. **Submission**: Validate and save configuration
5. **Installation Trigger**: Execute installation pipeline
6. **Monitoring**: Real-time progress tracking
7. **Error Recovery**: Reconfigure option on failure

## Integration Architecture

### External System Integration

#### 1Password Connect
- Token-based authentication
- Secret synchronization
- Kubernetes Secret creation
- Namespace isolation

#### ClusterForge
- Release-based deployment
- Custom values file support
- Helm chart installation
- Application platform integration

#### OIDC Providers
- Authentication configuration
- RKE2 OIDC integration
- API server configuration

### Kubernetes Ecosystem Integration

#### Helm Charts
- Automated chart deployment
- Values file customization
- Release management

#### kubectl Access
- Kubeconfig generation
- RBAC configuration
- User access setup

#### k9s Integration
- Terminal-based management
- Automatic installation
- Cluster navigation

## CI/CD Pipeline Architecture

### GitHub Actions Workflow

#### Build and Release
- Devbox-based build environment
- Multi-architecture support
- Automated binary creation
- Release asset upload

#### Version Management
- Git tag-based versioning
- Build-time version injection
- Semantic versioning support

#### Testing Infrastructure
- Chromedp-based UI testing
- Mock system integration
- Automated test execution
- YAML-based test definitions

## Configuration System Architecture

### Configuration Sources (Priority Order)

1. **Command-line flags**: Highest priority
2. **Configuration file**: bloom.yaml
3. **Environment variables**: System environment
4. **Default values**: Built-in defaults

### Configuration Loading

```go
// Pseudo-code representation
func LoadConfiguration() Config {
    config := LoadDefaults()
    config.MergeFrom(EnvironmentVariables())
    config.MergeFrom(ConfigFile())
    config.MergeFrom(CommandLineFlags())
    return config
}
```

### Configuration Validation

Pre-flight validation checks:

- **URL Validation**: OIDC, ClusterForge, ROCm, RKE2 URLs
- **Network Validation**: IP addresses, token formats
- **Step Validation**: Step names in DISABLED_STEPS/ENABLED_STEPS
- **Conflict Detection**: Mutually exclusive options
- **Resource Validation**: Disk space, memory, CPU
- **OS Compatibility**: Ubuntu version, kernel modules

## State Management

### Installation State

State persistence mechanisms:

- **bloom.log**: Installation progress and errors
- **bloom.yaml**: Configuration state
- **Kubernetes Resources**: ConfigMaps for cluster state
- **File System**: Mount points, installed components

### State Recovery

Recovery mechanisms:

- **Bloom.log Parsing**: Extract configuration from logs
- **Pre-filled Forms**: Auto-populate from previous attempts
- **Idempotent Operations**: Safe to re-run steps
- **Cleanup Operations**: Automated partial installation cleanup

## Error Handling Architecture

### Error Categories

1. **Validation Errors**: Pre-flight configuration issues
2. **System Errors**: OS-level failures
3. **Network Errors**: Connectivity and download issues
4. **Kubernetes Errors**: Cluster and workload failures
5. **Integration Errors**: External system integration failures

### Error Recovery Strategies

- **Automatic Retry**: Transient failures
- **Manual Intervention**: Configuration errors
- **Graceful Degradation**: Optional component failures
- **Rollback Support**: Partial installation cleanup

### Error Reporting

- **UI Display**: Clear error messages with context
- **Log Files**: Detailed error information
- **Suggestions**: Actionable recovery steps
- **Documentation Links**: Context-specific help

## Performance Considerations

### Optimization Strategies

- **Parallel Operations**: Independent step parallelization
- **Caching**: Package and image caching
- **Incremental Updates**: Partial configuration changes
- **Resource Limits**: Configurable resource constraints

### Monitoring Points

- **Step Duration**: Installation timing metrics
- **Resource Usage**: CPU, memory, disk I/O
- **Network Bandwidth**: Download speeds
- **Error Rates**: Failure frequency tracking

## Security Architecture

### Security Layers

1. **System Access**: sudo requirement for privileged operations
2. **Network Security**: Firewall configuration
3. **Kubernetes RBAC**: Role-based access control
4. **Secrets Management**: 1Password integration
5. **TLS Certificates**: Encrypted communication

### Security Best Practices

- **Principle of Least Privilege**: Minimal required permissions
- **Audit Logging**: Kubernetes API audit trail
- **Encryption**: TLS for in-transit data
- **Secret Rotation**: External secrets management
- **Network Policies**: Pod-to-pod communication control

## Testing Architecture

### Test Types

#### UI Tests
- Browser-based automation (chromedp)
- Form validation testing
- Configuration workflow testing
- Mock system integration

#### Integration Tests
- Mock-based command execution
- Multi-configuration scenarios
- Step execution validation
- YAML result verification

#### Unit Tests
- Individual component testing
- Function-level validation
- Edge case coverage

### Test Infrastructure

- **Docker Containers**: Isolated test environments
- **Mock System**: Command execution mocking
- **YAML Test Definitions**: Declarative test cases
- **CI/CD Integration**: Automated test execution

## Extensibility Points

### Custom Step Development

Add new installation steps:

1. Define step structure in `pkg/steps.go`
2. Implement step logic
3. Add to appropriate pipeline phase
4. Update configuration schema
5. Add validation rules

### Plugin Architecture

Future extensibility mechanisms:

- **Custom Storage Providers**: Beyond Longhorn
- **Alternative CNI Plugins**: Beyond Cilium
- **Additional GPU Vendors**: Beyond AMD/ROCm
- **Custom Integration Hooks**: External system integration

## Deployment Patterns

### Single Node Deployment
- All components on one node
- Development/testing environments
- Minimal resource requirements

### Multi-Node Cluster
- Dedicated control plane nodes
- Worker node pool
- GPU node specialization
- Storage node optimization

### High Availability
- Multiple control plane nodes
- etcd cluster distribution
- Load balancer redundancy
- Storage replication

## Maintenance Operations

### Upgrade Procedures
- RKE2 version upgrades
- Component updates
- Configuration changes
- Node additions/removals

### Backup Operations
- Configuration backups
- State persistence
- Disaster recovery preparation

### Monitoring Integration
- Metrics collection points
- Log aggregation hooks
- Alert integration
- Dashboard connectivity

## Technical Debt Areas

### Current Limitations

1. **Limited Test Coverage**: Backend unit tests needed
2. **Minimal Documentation**: Operational procedures missing
3. **Basic Validation**: Comprehensive checks needed
4. **Simple Logging**: Centralized log aggregation missing
5. **No Performance Tuning**: Optimization configurations needed

### Improvement Roadmap

- Enhanced testing framework
- Comprehensive validation system
- Advanced logging infrastructure
- Performance optimization
- Extended documentation

See [PRD.md](../../PRD.md) for product overview and feature descriptions.
