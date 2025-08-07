# ClusterBloom
**ClusterBloom** is a tool for deploying and configuring Kubernetes clusters using RKE2, with specialized support for AMD GPU environments. It automates the process of setting up multi-node clusters, configuring storage with Longhorn, and integrating with various tools and services.

## Features

- Automated RKE2 Kubernetes cluster deployment
- ROCm setup and configuration for AMD GPU nodes
- Disk management and Longhorn storage integration
- Multi-node cluster support with easy node joining
- ClusterForge integration

## Prerequisites

- Ubuntu (supported versions checked at runtime)
- Sufficient disk space (500GB+ recommended for root partition, 2TB+ for workloads)
- NVMe drives for optimal storage configuration
- ROCm-compatible AMD GPUs (for GPU nodes)
- Root/sudo access

## Installation

### Using Devbox

1. Install Devbox if you haven't already:
   ```sh
   curl -fsSL https://get.jetpack.io/devbox | bash
   ```

2. Clone the repository:
   ```sh
   git clone https://github.com/silogen/cluster-bloom.git
   cd cluster-bloom
   ```

3. Build with Devbox:
   ```sh
   devbox run build
   ```

The binary will be created in the dist directory as `bloom`.

## Usage

### First Node Setup

To set up the first node in your cluster:

```sh
sudo ./bloom
```

### Additional Node Setup

After setting up the first node, it will generate a command in `additional_node_command.txt` that you can run on other nodes to join them to the cluster:

```sh
# Example (actual command will be different)
echo -e 'FIRST_NODE: false\nJOIN_TOKEN: your-token-here\nSERVER_IP: your-server-ip' > bloom.yaml && sudo ./bloom --config bloom.yaml
```

### Demo UI

```sh
sudo ./bloom demo-ui
```

### Version Information

```sh
./bloom version
```

### Help

```sh
./bloom help
```

## Configuration

Cluster-Bloom can be configured through environment variables, command-line flags, or a configuration file.

### Configuration Variables

| Variable | Description | Default |
|----------|-------------|---------|
| FIRST_NODE | Set to true if this is the first node in the cluster | true |
| GPU_NODE | Set to true if this node has GPUs | true |
| OIDC_URL | The URL of the OIDC provider | "" |
| SERVER_IP | The IP address of the RKE2 server (required for additional nodes) | |
| JOIN_TOKEN | The token used to join additional nodes to the cluster | |
| SKIP_DISK_CHECK | Set to true to skip disk-related operations | false |
| LONGHORN_DISKS | Comma-separated list of disk paths to use for Longhorn | "" |
| CLUSTERFORGE_RELEASE | The version of Cluster-Forge to install. Pass the URL for a specific release, or 'none' to not install ClusterForge. | "https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz" |
| DISABLED_STEPS | Comma-separated list of steps to skip. Example "SetupLonghornStep,SetupMetallbStep" | "" |
| ENABLED_STEPS | Comma-separated list of steps to perform. If empty, perform all. Example "SetupLonghornStep,SetupMetallbStep" | "" |
| SELECTED_DISKS | Comma-separated list of disk devices. Example "/dev/sdb,/dev/sdc". Also skips NVME drive checks. | "" |

### Using a Configuration File

Create a YAML configuration file (e.g., `bloom.yaml`):

```yaml
FIRST_NODE: true
GPU_NODE: true
SKIP_DISK_CHECK: true
```

Then run with:

```sh
sudo ./bloom --config bloom.yaml
```

## Installation Process

Cluster-Bloom performs the following steps during installation:

1. Checks for supported Ubuntu version
2. Installs required packages (jq, nfs-common, open-iscsi)
3. Configures firewall and networking
4. Sets up ROCm for GPU nodes
5. Prepares and installs RKE2
6. Configures storage with Longhorn
7. Sets up Kubernetes tools and configuration
8. Installs ClusterForge

## Roadmap

### Phase 1: Foundation & Quality (High Priority)

#### 🔧 Testing & Validation
- **Task 1**: Implement Comprehensive Unit Tests for All Exported Functions in pkg/
  - Create thorough unit tests for steps.go, disks.go, rke2.go, rocm.go, and view.go
  - Ensure code quality and reliability foundation

#### 🛡️ Configuration & Security  
- **Task 4**: Enhance Configuration Validation with Comprehensive Checks
  - Robust validation logic for all configuration parameters
  - Check interdependencies and best practices adherence

### Phase 2: Enterprise Features (Medium Priority)

#### 💾 Data Protection
- **Task 2**: Implement Cluster State Backup and Recovery Functionality
  - etcd data backup capabilities
  - Configuration file backup and restore
  - *Depends on: Task 1*

#### 📊 Observability
- **Task 3**: Add Built-in Monitoring Stack Deployment with Prometheus and Grafana
  - Automated monitoring stack deployment
  - Kubernetes manifests and installation scripts
  - *Depends on: Task 1*

#### 🔐 Certificate Management
- **Task 5**: Implement Automated Certificate Lifecycle Management for Cluster
  - Seamless certificate issuance, renewal, and rotation
  - Minimize manual intervention and outage risks
  - *Depends on: Task 4*

#### 🌐 Advanced Networking
- **Task 6**: Implement Network Policy Management and Advanced Networking Features
  - Network policies, security policies, resource quotas
  - Enhanced cluster security and traffic control
  - *Depends on: Tasks 4, 5*

#### ⚡ High Availability
- **Task 7**: Add High Availability Configuration Support for etcd and Control Plane Components
  - HA configuration for etcd and control plane
  - Ensure cluster resilience and fault tolerance
  - *Depends on: Tasks 4, 5*

### Phase 3: Automation & Scalability (Low Priority)

#### 📈 Auto-scaling
- **Task 8**: Implement Automated Cluster Scaling and Workload-Based Autoscaling
  - Node addition/removal automation
  - Workload-based scaling decisions
  - *Depends on: Tasks 4, 7*

#### 🐧 Multi-OS Support
- **Task 9**: Add CentOS/RHEL Compatibility for Multi-OS Cluster Support
  - Extend beyond Ubuntu-only compatibility
  - Enable multi-OS Kubernetes cluster deployments
  - *Depends on: Task 4*

#### ☁️ Cloud Integration
- **Task 10**: Implement Cloud Provider Integration for AWS, Azure, and GCP
  - Integration modules for major cloud providers
  - Cloud-native Kubernetes cluster deployment and management
  - *Depends on: Task 4*

### Development Status

**Current Focus**: Foundation phase with emphasis on testing and configuration validation.

**Progress**: 0/10 tasks completed (0% complete)

**Next Milestone**: Complete comprehensive unit testing to establish a solid foundation for advanced features.

## Dependencies

- go (1.24.0)
- cobra-cli
- jq, nfs-common, open-iscsi (installed during setup)
- kubectl and k9s (installed during setup)

## License

Apache License 2.0
