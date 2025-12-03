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
| CERT_OPTION | Certificate option when USE_CERT_MANAGER is false. Choose 'existing' or 'generate' | "" |
| CF_VALUES | Path to ClusterForge values file (optional). Example: "values_cf.yaml" | "" |
| CLUSTERFORGE_RELEASE | The version of Cluster-Forge to install. Pass the URL for a specific release, or 'none' to not install ClusterForge. | "https://github.com/silogen/cluster-forge/releases/download/deploy/deploy-release.tar.gz" |
| CLUSTER_DISKS | Comma-separated list of disk devices. Example "/dev/sdb,/dev/sdc". Also skips NVME drive checks. | "" |
| CLUSTER_PREMOUNTED_DISKS | Comma-separated list of absolute disk paths to use for Longhorn | "" |
| CONTROL_PLANE |  Set to true if this node should be a control plane node |false, only applies when FIRST_NODE is false |
| DISABLED_STEPS | Comma-separated list of steps to skip. Example "SetupLonghornStep,SetupMetallbStep" | "" |
| DOMAIN | The domain name for the cluster (e.g., "cluster.example.com") (required). | "" |
| ENABLED_STEPS | Comma-separated list of steps to perform. If empty, perform all. Example "SetupLonghornStep,SetupMetallbStep" | "" |
| FIRST_NODE | Set to true if this is the first node in the cluster | true |
| GPU_NODE | Set to true if this node has GPUs | true |
| JOIN_TOKEN | The token used to join additional nodes to the cluster | |
| NO_DISKS_FOR_CLUSTER | Set to true to skip disk-related operations | false |
| OIDC_URL | The URL of the OIDC provider | "" |
| SERVER_IP | The IP address of the RKE2 server (required for additional nodes) | |
| SKIP_RANCHER_PARTITION_CHECK | Set to true to skip /var/lib/rancher partition size check | false |
| TLS_CERT | Path to TLS certificate file for ingress (required if CERT_OPTION is 'existing') | "" |
| TLS_KEY | Path to TLS private key file for ingress (required if CERT_OPTION is 'existing') | "" |
| USE_CERT_MANAGER | Use cert-manager with Let's Encrypt for automatic TLS certificates | false |


### Using a Configuration File

Create a YAML configuration file (e.g., `bloom.yaml`):

```yaml
FIRST_NODE: true
GPU_NODE: true
NO_DISKS_FOR_CLUSTER: true
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

## Dependencies

- go (1.24.0)
- cobra-cli
- jq, nfs-common, open-iscsi (installed during setup)
- kubectl and k9s (installed during setup)

## License

Apache License 2.0
