# Configuration Reference

## Overview

This document provides a comprehensive reference for all ClusterBloom configuration variables. Configuration can be provided via command-line flags, YAML files, or environment variables.

## Configuration Priority

Configuration sources in priority order (highest to lowest):

1. Command-line flags
2. Configuration file (bloom.yaml)
3. Environment variables
4. Default values

## Core Configuration Variables

### Node Type Configuration

#### FIRST_NODE
- **Type**: Boolean
- **Default**: `true`
- **Description**: Designates whether this is the first node in the cluster
- **Values**: `true` | `false`
- **Example**: `FIRST_NODE: false`

#### CONTROL_PLANE
- **Type**: Boolean
- **Default**: `false`
- **Description**: Indicates if additional node should be a control plane (only applies when `FIRST_NODE: false`)
- **Values**: `true` | `false`
- **Example**: `CONTROL_PLANE: true`

#### GPU_NODE
- **Type**: Boolean
- **Default**: `false`
- **Description**: Enables GPU-specific configurations and ROCm installation
- **Values**: `true` | `false`
- **Example**: `GPU_NODE: true`

#### CLUSTER_SIZE
- **Type**: Enum
- **Default**: `small`
- **Description**: Size category for cluster deployment planning
- **Values**: `small` | `medium` | `large`
- **Example**: `CLUSTER_SIZE: medium`

### Cluster Joining Configuration

#### SERVER_IP
- **Type**: String (IP Address)
- **Default**: None
- **Description**: IP address of the first node (required for additional nodes)
- **Required When**: `FIRST_NODE: false`
- **Example**: `SERVER_IP: "192.168.1.100"`

#### JOIN_TOKEN
- **Type**: String
- **Default**: None
- **Description**: Token for joining additional nodes to the cluster
- **Required When**: `FIRST_NODE: false`
- **Example**: `JOIN_TOKEN: "K10abcdef..."`
- **Note**: Retrieved from first node at `/var/lib/rancher/rke2/server/node-token`

### Storage Configuration

#### NO_DISKS_FOR_CLUSTER
- **Type**: Boolean
- **Default**: `false`
- **Description**: Bypasses all disk-related operations
- **Values**: `true` | `false`
- **Example**: `NO_DISKS_FOR_CLUSTER: true`
- **Use Case**: CPU-only nodes or when using external storage

#### CLUSTER_PREMOUNTED_DISKS
- **Type**: String (comma-separated paths)
- **Default**: None
- **Description**: Manual disk specification for pre-mounted disks
- **Example**: `CLUSTER_PREMOUNTED_DISKS: "/mnt/disk0,/mnt/disk1"`

#### CLUSTER_DISKS
- **Type**: String (comma-separated device names)
- **Default**: None
- **Description**: Pre-selected disk devices to use
- **Example**: `CLUSTER_DISKS: "/dev/nvme0n1,/dev/nvme1n1"`
- **Note**: Also skips NVMe drive availability checks

### Step Control Configuration

#### DISABLED_STEPS
- **Type**: String (comma-separated step IDs)
- **Default**: None
- **Description**: Comma-separated list of step IDs to skip during installation
- **Example**: `DISABLED_STEPS: "install-longhorn,install-metallb"`
- **Mutually Exclusive With**: `ENABLED_STEPS`

#### ENABLED_STEPS
- **Type**: String (comma-separated step IDs)
- **Default**: None
- **Description**: Comma-separated list of step IDs to execute (all others skipped)
- **Example**: `ENABLED_STEPS: "install-rke2,configure-kubeconfig"`
- **Mutually Exclusive With**: `DISABLED_STEPS`
- **Use Case**: Targeted operations or troubleshooting

### Domain and Certificate Configuration

#### DOMAIN
- **Type**: String (domain name)
- **Default**: None
- **Description**: Domain name for cluster ingress configuration
- **Example**: `DOMAIN: "cluster.example.com"`

#### USE_CERT_MANAGER
- **Type**: Boolean
- **Default**: `false`
- **Description**: Enable cert-manager with Let's Encrypt for TLS certificates
- **Values**: `true` | `false`
- **Example**: `USE_CERT_MANAGER: true`

#### CERT_MANAGER_EMAIL
- **Type**: String (email address)
- **Default**: None
- **Description**: Email address for Let's Encrypt certificate notifications
- **Required When**: `USE_CERT_MANAGER: true`
- **Example**: `CERT_MANAGER_EMAIL: "admin@example.com"`

#### CERT_OPTION
- **Type**: String
- **Default**: None
- **Description**: Certificate handling when cert-manager is disabled
- **Values**: `existing` | `generate`
- **Example**: `CERT_OPTION: "existing"`
- **Applies When**: `USE_CERT_MANAGER: false`

#### TLS_CERT
- **Type**: String (file path)
- **Default**: None
- **Description**: Path to TLS certificate file for ingress
- **Example**: `TLS_CERT: "/path/to/tls.crt"`
- **Required When**: `CERT_OPTION: "existing"`

#### TLS_KEY
- **Type**: String (file path)
- **Default**: None
- **Description**: Path to TLS key file for ingress
- **Example**: `TLS_KEY: "/path/to/tls.key"`
- **Required When**: `CERT_OPTION: "existing"`

### ArgoCD Configuration (Small Clusters)

#### INSTALL_ARGOCD
- **Type**: Boolean
- **Default**: `true`
- **Description**: Install ArgoCD core (headless/CLI-only mode) for GitOps-based app deployment. Only applies to `CLUSTER_SIZE: small`.
- **Values**: `true` | `false`
- **Example**: `INSTALL_ARGOCD: false`

#### ARGOCD_VERSION
- **Type**: String (version tag)
- **Default**: `v2.14.11`
- **Description**: ArgoCD version to install
- **Example**: `ARGOCD_VERSION: "v2.14.11"`

#### CLUSTERFORGE_REPO
- **Type**: String (git URL)
- **Default**: `https://github.com/silogen/cluster-forge.git`
- **Description**: Git repository URL for the ClusterForge Helm chart used in ArgoCD-based deployment
- **Example**: `CLUSTERFORGE_REPO: "https://github.com/myorg/cluster-forge.git"`

#### CLUSTERFORGE_BRANCH
- **Type**: String (branch name)
- **Default**: `creating_small_configuration`
- **Description**: Git branch of the ClusterForge repository to use
- **Example**: `CLUSTERFORGE_BRANCH: "main"`

### Integration Configuration

#### CLUSTERFORGE_RELEASE
- **Type**: String (version or URL)
- **Default**: None
- **Description**: ClusterForge release tarball URL for non-ArgoCD deployment (medium/large clusters). Set to `none` to skip.
- **Example**: `CLUSTERFORGE_RELEASE: "https://github.com/silogen/cluster-forge/releases/download/v1.8.0/release.tar.gz"`

#### CF_VALUES
- **Type**: String (file path)
- **Default**: None
- **Description**: ClusterForge values file path (optional)
- **Example**: `CF_VALUES: "/path/to/values.yaml"`

#### OIDC_URL
- **Type**: String (URL)  
- **Default**: None
- **Description**: **DEPRECATED** - Legacy OIDC provider configuration (removed in this branch)
- **Replacement**: Use `ADDITIONAL_OIDC_PROVIDERS` for multiple provider support
- **Breaking Change**: This variable no longer works - migrate to `ADDITIONAL_OIDC_PROVIDERS`

#### ADDITIONAL_OIDC_PROVIDERS
- **Type**: Array of OIDC Provider objects
- **Default**: `[]` (empty, uses default provider)
- **Description**: List of additional OIDC providers for multi-provider authentication
- **Required When**: Multiple authentication providers needed
- **Example**: 
  ```yaml
  ADDITIONAL_OIDC_PROVIDERS:
    - url: "https://kc.plat-dev-3.silogen.ai/realms/airm"
      audiences: ["k8s"]
    - url: "https://kc.plat-dev-4.silogen.ai/realms/k8s"
      audiences: ["kubernetes", "api"]
  ```
- **Default Behavior**: If empty, auto-configures `https://kc.{DOMAIN}/realms/airm` with audience `k8s`
- **Provider Object Fields**:
  - `url`: HTTPS URL of the OIDC provider (required)
  - `audiences`: Array of client IDs/audiences (required)

#### RKE2_VERSION
- **Type**: String (version)
- **Default**: `""` (latest stable)
- **Description**: Specific RKE2 version to install
- **Example**: `RKE2_VERSION: "v1.34.1+rke2r1"`
- **Format**: Must include RKE2 suffix (e.g., "+rke2r1")

#### ADDITIONAL_TLS_SAN_URLS
- **Type**: Array of strings (domain names)
- **Default**: `[]`
- **Description**: Additional TLS Subject Alternative Name URLs for Kubernetes API server certificate
- **Example**: `ADDITIONAL_TLS_SAN_URLS: ["api.example.com", "kubernetes.example.com"]`
- **Auto-generated**: Always includes `k8s.{DOMAIN}` - do not duplicate
- **Validation**: Each entry must be a valid domain name format

#### ONEPASSWORD_CONNECT_TOKEN
- **Type**: String
- **Default**: None
- **Description**: Token for 1Password Connect integration (optional)
- **Example**: `ONEPASSWORD_CONNECT_TOKEN: "eyJhbGc..."`

#### ONEPASSWORD_CONNECT_HOST
- **Type**: String (URL)
- **Default**: None
- **Description**: Host URL for 1Password Connect service (optional)
- **Example**: `ONEPASSWORD_CONNECT_HOST: "http://onepassword-connect:8080"`

### Advanced Configuration

#### RKE2_EXTRA_CONFIG
- **Type**: String (YAML format)
- **Default**: None
- **Description**: Additional RKE2 configuration in YAML format to append to `/etc/rancher/rke2/config.yaml`
- **Example**:
  ```yaml
  RKE2_EXTRA_CONFIG: |
    node-taint:
      - "CriticalAddonsOnly=true:NoExecute"
    node-label:
      - "workload-type=ml"
  ```

#### PRELOAD_IMAGES
- **Type**: String (comma-separated image references)
- **Default**: None
- **Description**: Comma-separated list of container images to preload into the cluster
- **Example**: `PRELOAD_IMAGES: "docker.io/nvidia/cuda:11.8.0-base,ghcr.io/myapp:latest"`

#### SKIP_RANCHER_PARTITION_CHECK
- **Type**: Boolean
- **Default**: `false`
- **Description**: Skip validation of `/var/lib/rancher` partition size (useful for CPU-only nodes)
- **Values**: `true` | `false`
- **Example**: `SKIP_RANCHER_PARTITION_CHECK: true`

## Configuration File Format

### YAML Configuration File (bloom.yaml)

```yaml
# Node configuration
FIRST_NODE: true
GPU_NODE: false
CONTROL_PLANE: false

# Storage configuration
NO_DISKS_FOR_CLUSTER: false
CLUSTER_DISKS: "/dev/nvme0n1,/dev/nvme1n1"

# Domain and certificates
DOMAIN: "cluster.example.com"
USE_CERT_MANAGER: true
CERT_MANAGER_EMAIL: "admin@example.com"

# Integration
CLUSTERFORGE_RELEASE: "v1.2.3"
ADDITIONAL_OIDC_PROVIDERS:
  - url: "https://kc.example.com/realms/airm"
    audiences: ["k8s"]
  - url: "https://auth.example.com/realms/main"
    audiences: ["kubernetes", "api"]

# Advanced options
RKE2_EXTRA_CONFIG: |
  node-label:
    - "environment=production"
```

### Additional Node Configuration (bloom.yaml)

```yaml
FIRST_NODE: false
CONTROL_PLANE: false
GPU_NODE: true
SERVER_IP: "192.168.1.100"
JOIN_TOKEN: "K10abcdef1234567890::server:abcdef1234567890"
```

## Command-Line Usage

### Configuration File
```bash
sudo ./bloom --config /path/to/bloom.yaml
```

### Environment Variables
```bash
export FIRST_NODE=false
export SERVER_IP="192.168.1.100"
export JOIN_TOKEN="K10..."
sudo -E ./bloom
```

### Mixed Configuration
```bash
# Use config file but override specific values
sudo ./bloom --config bloom.yaml --domain custom.example.com
```

## Validation Rules

### Required Fields

**For First Node**:
- `FIRST_NODE: true` (or omitted, default is true)

**For Additional Nodes**:
- `FIRST_NODE: false`
- `SERVER_IP` (required)
- `JOIN_TOKEN` (required)

### Mutually Exclusive Fields

- `DISABLED_STEPS` and `ENABLED_STEPS` cannot both be set
- `USE_CERT_MANAGER: true` and `CERT_OPTION: "existing"` cannot both be set

### Conditional Requirements

- `CONTROL_PLANE: true` requires `FIRST_NODE: false`
- `CERT_MANAGER_EMAIL` required when `USE_CERT_MANAGER: true`
- `TLS_CERT` and `TLS_KEY` required when `CERT_OPTION: "existing"`

## Common Configuration Scenarios

### First Node (GPU-enabled)
```yaml
FIRST_NODE: true
GPU_NODE: true
DOMAIN: "ml-cluster.example.com"
USE_CERT_MANAGER: true
CERT_MANAGER_EMAIL: "admin@example.com"
CLUSTERFORGE_RELEASE: "v1.2.3"
RKE2_VERSION: "v1.34.1+rke2r1"
ADDITIONAL_OIDC_PROVIDERS:
  - url: "https://kc.ml-cluster.example.com/realms/airm"
    audiences: ["k8s"]
ADDITIONAL_TLS_SAN_URLS:
  - "api.ml-cluster.example.com"
```

### Additional Worker Node (GPU-enabled)
```yaml
FIRST_NODE: false
GPU_NODE: true
SERVER_IP: "192.168.1.100"
JOIN_TOKEN: "K10..."
```

### Additional Control Plane Node
```yaml
FIRST_NODE: false
CONTROL_PLANE: true
SERVER_IP: "192.168.1.100"
JOIN_TOKEN: "K10..."
```

### CPU-Only Node (No Storage)
```yaml
FIRST_NODE: false
GPU_NODE: false
NO_DISKS_FOR_CLUSTER: true
SKIP_RANCHER_PARTITION_CHECK: true
SERVER_IP: "192.168.1.100"
JOIN_TOKEN: "K10..."
```

### Small Cluster with ArgoCD (GitOps)
```yaml
FIRST_NODE: true
GPU_NODE: true
DOMAIN: "165.245.128.225.nip.io"
CERT_OPTION: generate
CLUSTER_SIZE: small
CLUSTER_DISKS: /dev/vdc1
CLUSTERFORGE_RELEASE: none
```

### Small Cluster without ArgoCD
```yaml
FIRST_NODE: true
GPU_NODE: true
DOMAIN: "165.245.128.225.nip.io"
CERT_OPTION: generate
CLUSTER_SIZE: small
CLUSTER_DISKS: /dev/vdc1
INSTALL_ARGOCD: false
CLUSTERFORGE_RELEASE: none
```

### Testing/Development Configuration
```yaml
FIRST_NODE: true
GPU_NODE: false
NO_DISKS_FOR_CLUSTER: true
DISABLED_STEPS: "install-longhorn,install-metallb,install-clusterforge"
```

## Environment Variable Mapping

All YAML configuration variables can be set as environment variables:

```bash
export FIRST_NODE=true
export GPU_NODE=false
export DOMAIN="cluster.example.com"
export USE_CERT_MANAGER=true
export CERT_MANAGER_EMAIL="admin@example.com"
```

## See Also

- [PRD.md](../../PRD.md) - Product overview
- [06-technical-architecture.md](07-technical-architecture.md) - Technical architecture
- [07-installation-guide.md](08-installation-guide.md) - Installation procedures
