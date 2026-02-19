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

## Getting Started

### Download and Setup

1. Download the latest bloom binary:
```sh
wget https://github.com/silogen/cluster-bloom/releases/download/<version>/bloom
```

2. Make the binary executable:
```sh
chmod +x bloom
```

## Usage

### Configuration Generation

Launch the web UI to generate your bloom.yaml configuration:

```sh
./bloom
```

Access the configuration wizard at http://127.0.0.1:62078

### Additional Node Setup

After setting up the first node, it will generate a command in `additional_node_command.txt` that you can run on other nodes to join them to the cluster:

```sh
# Example (actual command will be different)
echo -e 'FIRST_NODE: false\nJOIN_TOKEN: your-token-here\nSERVER_IP: your-server-ip' > bloom.yaml && sudo ./bloom cli bloom.yaml
```

### Version Information

```sh
./bloom version
```

### Command Help

Show all available commands and complete configuration reference:

```sh
./bloom help
```

*Note: Help includes auto-generated documentation for all configuration fields.*

Get help for specific commands:

```sh
./bloom cleanup --help  # Remove existing cluster installation
./bloom cli --help      # Deploy cluster using configuration file
```

## Configuration

Cluster-Bloom can be configured through environment variables, command-line flags, or a configuration file.

### Configuration Variables

| Variable | Description | Default |
|----------|-------------|---------|
| ADDITIONAL_OIDC_PROVIDERS | List of additional OIDC providers for authentication (see examples below) | [] |
| ADDITIONAL_TLS_SAN_URLS | Additional TLS Subject Alternative Name URLs for Kubernetes API server certificate | [] |
| CERT_OPTION | Certificate option when USE_CERT_MANAGER is false. Choose 'existing' or 'generate' | "" |
| CF_VALUES | Path to ClusterForge values file (optional). Example: "values_cf.yaml" | "" |
| CLUSTER_DISKS | Comma-separated list of disk devices. Example "/dev/sdb,/dev/sdc". Also skips NVME drive checks. | "" |
| CLUSTER_SIZE | Size category for cluster deployment planning. Options: small, medium, large | medium |
| CLUSTER_PREMOUNTED_DISKS | Comma-separated list of absolute disk paths to use for Longhorn | "" |
| CLUSTERFORGE_RELEASE | The version of Cluster-Forge to install. Pass the URL for a specific release, or 'none' to not install ClusterForge. | "https://github.com/silogen/cluster-forge/releases/download/v1.8.0/release-enterprise-ai-v1.8.0.tar.gz" |
| CONTROL_PLANE | Set to true if this node should be a control plane node | false, only applies when FIRST_NODE is false |
| DISABLED_STEPS | Comma-separated list of steps to skip. Example "SetupLonghornStep,SetupMetallbStep" | "" |
| DOMAIN | The domain name for the cluster (e.g., "cluster.example.com") (required). | "" |
| ENABLED_STEPS | Comma-separated list of steps to perform. If empty, perform all. Example "SetupLonghornStep,SetupMetallbStep" | "" |
| FIRST_NODE | Set to true if this is the first node in the cluster | true |
| GPU_NODE | Set to true if this node has GPUs | true |
| JOIN_TOKEN | The token used to join additional nodes to the cluster | |
| NO_DISKS_FOR_CLUSTER | Set to true to skip disk-related operations | false |
| RKE2_VERSION | Specific RKE2 version to install (e.g., "v1.34.1+rke2r1") | "" |
| SERVER_IP | The IP address of the RKE2 server (required for additional nodes) | |
| SKIP_RANCHER_PARTITION_CHECK | Set to true to skip /var/lib/rancher partition size check | false |
| TLS_CERT | Path to TLS certificate file for ingress (required if CERT_OPTION is 'existing') | "" |
| TLS_KEY | Path to TLS private key file for ingress (required if CERT_OPTION is 'existing') | "" |
| USE_CERT_MANAGER | Use cert-manager with Let's Encrypt for automatic TLS certificates | false |
| ARGOCD_VERSION | ArgoCD version to install | v2.14.11 |
| CLUSTERFORGE_BRANCH | ClusterForge git branch for ArgoCD-based deployment | "" |
| CLUSTERFORGE_REPO | ClusterForge git repository URL for ArgoCD-based deployment | https://github.com/silogen/cluster-forge.git |
| INSTALL_ARGOCD | Install ArgoCD core for GitOps (small clusters only) | true |
| PRELOAD_IMAGES | Comma-separated list of container images to preload | docker.io/rocm/pytorch:rocm6.4_ubuntu24.04_py3.12_pytorch_release_2.6.0,docker.io/rocm/vllm:rocm6.4.1_vllm_0.9.0.1_20250605 |
| RKE2_EXTRA_CONFIG | Additional RKE2 configuration in YAML format | "" |
| RKE2_INSTALLATION_URL | RKE2 installation script URL | https://get.rke2.io |
| ROCM_BASE_URL | ROCm base repository URL | https://repo.radeon.com/amdgpu-install/7.0.2/ubuntu/ |
| ROCM_DEB_PACKAGE | ROCm DEB package name | amdgpu-install_7.0.2.70002-1_all.deb |

### OIDC Configuration Examples

**Basic OIDC Provider:**
```yaml
ADDITIONAL_OIDC_PROVIDERS:
  - url: "https://keycloak.example.com/realms/main"
    audiences: ["k8s"]
```

**Notes:**
- ClaimMappings use `username` and `groups` with prefix `"oidc:"`
- `url`: HTTPS URL of your OIDC provider (Keycloak, Auth0, etc.)
- `audiences`: List of client IDs from your OIDC provider
- **Default behavior**: If `ADDITIONAL_OIDC_PROVIDERS` is skipped, a default OIDC provider will be configured pointing to the internal Keycloak `airm` realm at `https://kc.{DOMAIN}/realms/airm`

For advanced configuration, multiple providers, and troubleshooting, see [docs/oidc-authentication.md](docs/oidc-authentication.md).

### TLS-SAN Configuration

TLS Subject Alternative Names (SANs) allow your Kubernetes API server to be accessed via multiple domain names. Cluster-Bloom automatically configures TLS-SANs for secure remote access to your cluster.

**Note:** Wildcard domains (*.example.com) are not supported by RKE2.

**Basic Configuration:**
```yaml
DOMAIN: "example.com"
ADDITIONAL_TLS_SAN_URLS:
  - "api.example.com"
  - "kubernetes.example.com"
```

**Key Points:**
- Cluster-Bloom automatically generates `k8s.{DOMAIN}` as a default TLS-SAN
- Do not duplicate the auto-generated SAN in `ADDITIONAL_TLS_SAN_URLS`
- Valid domain names only (no wildcards)
- The configuration wizard provides real-time validation

For detailed examples, testing instructions, and common use cases, see [docs/tls-san-configuration.md](docs/tls-san-configuration.md).

### Using a Configuration File

Create a YAML configuration file (e.g., `bloom.yaml`):

```yaml
DOMAIN: "your-domain.example.com"  # Required: Your cluster domain
FIRST_NODE: true
GPU_NODE: true                     # Set to false if no GPUs
CLUSTER_DISKS: "/dev/nvme1n1"     # Disk device path for storage
CERT_OPTION: "generate"           # Options: "generate" or "existing"
CLUSTERFORGE_RELEASE: "https://github.com/silogen/cluster-forge/releases/download/v1.8.0/release-enterprise-ai-v1.8.0.tar.gz"  # ClusterForge release URL or "none" to skip
PRELOAD_IMAGES: ""                # Optional: comma-separated container images
```

Then run with:

```sh
sudo ./bloom cli bloom.yaml
```

## Installation Process

Cluster-Bloom performs the following steps during installation:

1. Checks for supported Ubuntu version
2. Installs required packages (jq, nfs-common, open-iscsi)
3. Configures firewall and networking
4. Sets up ROCm for GPU nodes
5. Prepares and installs RKE2
6. Configures storage (local-path for small/medium clusters, Longhorn for large clusters)
7. Sets up Kubernetes tools and configuration
8. Installs ClusterForge

## Dependencies

- go (1.24.0)
- cobra-cli
- jq, nfs-common, open-iscsi (installed during setup)
- kubectl and k9s (installed during setup)

## License

Apache License 2.0
