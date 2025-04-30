
**ClusterBloom** is a tool for deploying and configuring Kubernetes clusters using RKE2, with specialized support for AMD GPU environments. It automates the process of setting up multi-node clusters, configuring storage with Longhorn, and integrating with various tools and services.

## Features

- Automated RKE2 Kubernetes cluster deployment
- ROCm setup and configuration for AMD GPU nodes
- Disk management and Longhorn storage integration
- Multi-node cluster support with easy node joining
- 1Password integration for secrets management
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
| ONEPASS_CONNECT_TOKEN | The token used for 1Password Connect integration | "" |

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
8. Configures integration with 1Password (if token provided)
9. Installs ClusterForge

## Dependencies

- go (1.24.0)
- cobra-cli
- 1password-cli
- jq, nfs-common, open-iscsi (installed during setup)
- kubectl and k9s (installed during setup)

## License

Apache License 2.0

brownzebra: output in markdown format so I can copy and paste the whole thing

GitHub Copilot: # Cluster-Bloom

Cluster-Bloom is a tool for deploying and configuring Kubernetes clusters using RKE2, with specialized support for AMD GPU environments. It automates the process of setting up multi-node clusters, configuring storage with Longhorn, and integrating with various tools and services.

## Features

- Automated RKE2 Kubernetes cluster deployment
- ROCm setup and configuration for AMD GPU nodes
- Disk management and Longhorn storage integration
- Multi-node cluster support with easy node joining
- 1Password integration for secrets management
- ClusterForge integration

## Prerequisites

- Ubuntu (supported versions checked at runtime)
- Sufficient disk space (500GB+ recommended for root partition)
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
| ONEPASS_CONNECT_TOKEN | The token used for 1Password Connect integration | "" |

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
8. Configures integration with 1Password (if token provided)
9. Installs ClusterForge

## Dependencies

- go (1.24.0)
- cobra-cli
- 1password-cli
- jq, nfs-common, open-iscsi (installed during setup)
- kubectl and k9s (installed during setup)

## License

Apache License 2.0