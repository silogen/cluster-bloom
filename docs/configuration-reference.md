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
- **Default**: `true`
- **Description**: Enables AMD GPU driver validation or installation, device configuration, and Kubernetes GPU integration. Bloom does not install host ROCm workload libraries.
- **Values**: `true` | `false`
- **Example**: `GPU_NODE: true`

#### CLUSTER_SIZE
- **Type**: Enum
- **Default**: `small`
- **Description**: Size category for cluster deployment planning
- **Values**: `small` | `medium` | `large`
- **Example**: `CLUSTER_SIZE: medium`
- **Cilium**: `small` and `medium` set `operator.replicas: 1` via RKE2 `HelmChartConfig` on the first node at bootstrap; `large` uses the RKE2 default (2). To scale the operator after adding nodes, see [RKE2 deployment — Scaling cilium-operator](rke2-deployment.md#scaling-cilium-operator-after-install-multi-node--ha)

#### AIM_HARDWARE_FAMILY
- **Type**: String (comma-separated list)
- **Default**: `""` (empty)
- **Description**: Selects which AIM model sources cluster-forge installs, by hardware family. Empty installs the full legacy model catalog (no change from previous behavior). When set, only the listed families are installed.
- **Values**: any comma-separated combination of `cpu`, `epyc`, `instinct`, `radeon` (lowercase, no spaces)
- **Example**: `AIM_HARDWARE_FAMILY: "epyc,instinct"`
- **Notes**: `instinct` and `radeon` are GPU families; `cpu` and `epyc` are CPU inference targets. `cpu` and `radeon` are currently placeholders pointing at `ghcr.io` images that require a pull secret this cluster does not provision, so they will fail to pull until a `docker.io` release is published. In a `bloom.yaml` file the value is a normal comma-separated string. cluster-bloom splits it into a list before passing it to cluster-forge, so no comma-escaping is needed at the bloom layer.

#### GPU_STACK_FAMILY
- **Type**: String (single value)
- **Default**: `""` (empty, resolves to `instinct`)
- **Description**: Selects the ClusterForge AMD GPU Operator and DeviceConfig profile. This is independent of both the host driver allowlist and `AIM_HARDWARE_FAMILY`. Empty or `instinct` selects GPU Operator `v1.4.1` with DeviceConfig driver train `7.0`; `radeon` selects GPU Operator `v1.5.1-beta.0` with DeviceConfig driver train `7.13`.
- **Values**: `radeon` | `instinct` (lowercase, single value)
- **Example**: `GPU_STACK_FAMILY: "radeon"`
- **Notes**:
  - This setting does not install host ROCm. The production-default host driver remains `31.40.0` for both families.
  - The `radeon` GPU Operator and DeviceConfig profile is tech preview; Bloom prints a notice at install time.
  - Unsupported combinations (for example a Radeon stack resolving to ROCm 7.2) fail validation before install with an error naming the incompatible component.

#### GPU_DRIVER_SKIP_INSTALL
- **Type**: Boolean
- **Default**: `false`
- **Description**: Leaves the host GPU stack untouched by skipping driver compatibility validation, driver installation, and standalone AMD-SMI installation.
- **Values**: `true` | `false`
- **Example**: `GPU_DRIVER_SKIP_INSTALL: true`

#### GPU_INSTALL_HOST_TOOLS
- **Type**: Boolean
- **Default**: `true`
- **Description**: Installs and verifies the standalone AMD-SMI package matched to the effective driver. This does not install a host ROCm runtime.
- **Values**: `true` | `false`
- **Example**: `GPU_INSTALL_HOST_TOOLS: false`

#### GPU_DRIVER_VERSION and GPU_DRIVER_BUILD
- **Type**: String pair
- **Defaults**: `""` and `""` (resolve to installer version `31.40`, build `314000-1`, and AMD driver `31.40.0`)
- **Description**: Advanced override selecting one exact validated `amdgpu-install` package. Both values must be set together and match a supported tuple. Bloom validates the pair while reading `bloom.yaml`, before Ansible starts.
- **Supported pairs**: See the
  [full GPU driver compatibility table](gpu-driver-support.md#supported-version-matrix).

See [GPU Driver Support](gpu-driver-support.md) for detection and
verification behavior.

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

> **⚠️ Pending Implementation**: `DISABLED_STEPS` and `ENABLED_STEPS` are not yet active.
> These fields are reserved for a future release and have no effect in the current version.

#### DISABLED_STEPS *(pending implementation)*
- **Type**: String (comma-separated step IDs)
- **Default**: None
- **Description**: Comma-separated list of step IDs to skip during installation
- **Example**: `DISABLED_STEPS: "install-longhorn,install-metallb"`
- **Mutually Exclusive With**: `ENABLED_STEPS`

#### ENABLED_STEPS *(pending implementation)*
- **Type**: String (comma-separated step IDs)
- **Default**: None
- **Description**: Comma-separated list of step IDs to execute (all others skipped)
- **Example**: `ENABLED_STEPS: "install-rke2,configure-kubeconfig"`
- **Mutually Exclusive With**: `DISABLED_STEPS`
- **Use Case**: Targeted operations or troubleshooting

### Container Registry Configuration

#### DOCKERHUB_USER
- **Type**: String
- **Default**: `""` (empty — unauthenticated pulls)
- **Description**: DockerHub username for authenticated image pulls. Authenticating removes the anonymous pull rate limit (typically 100 pulls/6h per IP). Must be set together with `DOCKERHUB_TOKEN`.
- **Required With**: `DOCKERHUB_TOKEN`
- **Example**: `DOCKERHUB_USER: "myusername"`

#### DOCKERHUB_TOKEN
- **Type**: String
- **Default**: `""` (empty — unauthenticated pulls)
- **Description**: DockerHub personal access token for authenticated image pulls. Written to `/etc/rancher/rke2/registries.yaml` (mode `0600`, root-owned) before RKE2 starts, so no restart is needed. Must be set together with `DOCKERHUB_USER`.
- **Required With**: `DOCKERHUB_USER`
- **Example**: `DOCKERHUB_TOKEN: "dckr_pat_xxxxxxxxxxxx"`
- **Note**: Use a token with Read-only scope from [hub.docker.com/settings/personal-access-tokens](https://hub.docker.com/settings/personal-access-tokens)

### Domain and Certificate Configuration

#### DOMAIN
- **Type**: String (domain name)
- **Default**: None
- **Description**: Domain name for cluster ingress configuration. Required for first node. Also needed when joining as a control-plane node (for TLS SAN and OIDC configuration).
- **Example**: `DOMAIN: "cluster.example.com"`

### Network and DNS Configuration

#### FIX_DNS
- **Type**: Boolean
- **Default**: `false`
- **Description**: **Opt-in** flag to allow automatic DNS resolution fixes during installation. When enabled, the playbook will test current DNS configuration and only modify `/etc/resolv.conf` if DNS is broken AND external DNS servers are reachable. Creates timestamped backups before modification and automatically rolls back on failure.
- **Values**: `true` | `false`
- **Example**: `FIX_DNS: true`
- **Safety Features**:
  - Only modifies DNS if current DNS test fails AND external DNS (1.1.1.1) succeeds
  - Creates backup at `/etc/resolv.conf.backup-<timestamp>` before changes
  - Verifies DNS works after modification
  - Automatic rollback to backup if verification fails
  - Never removes immutable attribute until after successful verification
- **When Disabled** (`false`, default): Existing DNS configuration is never touched, even if broken
- **Use Cases**: 
  - Corporate networks with internal DNS servers: Leave `false` (default)
  - Servers with working systemd-resolved: Leave `false` (default)
  - Known DNS issues preventing apt updates: Set to `true`
- **⚠️ Warning**: When enabled, will overwrite `/etc/resolv.conf` with Google/Cloudflare DNS if local DNS is detected as broken

#### DNS_SERVERS
- **Type**: Sequence (List)
- **Default**: `[]` (empty list)
- **Description**: Custom DNS servers for RKE2 cluster. When provided, these nameservers will be written directly to `/etc/rancher/rke2/resolv.conf` instead of copying host DNS configuration. This allows explicit control over cluster DNS resolution.
- **Format**: YAML list of IP addresses
- **Example**: `DNS_SERVERS: ["8.8.8.8", "1.1.1.1", "208.67.222.222"]`
- **Behavior**:
  - **When Empty** (`[]`, default): Copies host DNS configuration to RKE2, with systemd-resolved detection and fallback logic
  - **When Set**: Writes only the specified nameservers to `/etc/rancher/rke2/resolv.conf`, bypassing host DNS entirely
- **Use Cases**:
  - Air-gapped environments requiring specific DNS servers: Set custom servers
  - Corporate networks with mandatory DNS servers: Set required servers
  - Performance optimization with preferred DNS providers: Set fastest servers
  - Standard deployments: Leave empty (default) to use host DNS
- **⚠️ Note**: When set, completely ignores host DNS configuration for the cluster

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
- **Description**: Certificate option when cert-manager is disabled (first node only)
- **Values**: `existing` | `generate`
- **Example**: `CERT_OPTION: "existing"`
- **Applies When**: `USE_CERT_MANAGER: false` and `FIRST_NODE: true`
- **Note**: Only used during cluster deployment. Not required when running `--tags deploy_clusterforge` to bootstrap ClusterForge on an already-deployed bloom cluster.

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

### ClusterForge Configuration

#### CLUSTERFORGE_REPO
- **Type**: String (git URL)
- **Default**: `https://github.com/silogen/cluster-forge.git`
- **Description**: Git repository URL for the ClusterForge Helm chart used in ArgoCD-based deployment
- **Example**: `CLUSTERFORGE_REPO: "https://github.com/myorg/cluster-forge.git"`

### Integration Configuration

#### CLUSTERFORGE_RELEASE
- **Type**: String (version, URL, or special value)
- **Default**: `latest`
- **Description**: ClusterForge version to deploy. Supports multiple formats:
  - **Version tag**: e.g., `v2.0.0-rc6` - Specifies exact version/branch to checkout
  - **Full release URL**: e.g., `https://github.com/silogen/cluster-forge/releases/download/v2.0.0-rc6/release-enterprise-ai-v2.0.0-rc6.tar.gz` - Downloads tarball and auto-extracts version for ArgoCD target
  - **Special values**: 
    - `latest` (or unset) - Fetches the latest published GitHub release tag via the GitHub API
    - `none` or `""` (empty string) - Deploys nothing from ClusterForge, not even ArgoCD (no ArgoCD, Gitea or OpenBao). Brings up the bare cluster only.
- **Version Parsing**: When a full URL is provided, the version is automatically extracted (e.g., `v2.0.0-rc6` from the URL) and used as the `--target-revision` for ArgoCD/Gitea
- **Examples**: 
  - `CLUSTERFORGE_RELEASE: "latest"`
  - `CLUSTERFORGE_RELEASE: "v2.0.2"`
  - `CLUSTERFORGE_RELEASE: "https://github.com/silogen/cluster-forge/releases/download/v2.0.2/release.tar.gz"`
  - `CLUSTERFORGE_RELEASE: "none"`

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
- **Example**: `ADDITIONAL_TLS_SAN_URLS: ["api.example.com", "management.example.com"]`
- **Auto-generated**: Always includes `k8s.{DOMAIN}` - do not duplicate
- **Validation**: 
  - Each entry must be a valid domain name format
  - Wildcard domains (*.example.com) are blocked by UI and server validation
  - Real-time validation provides immediate feedback
- **Migration**: Legacy comma-separated string format still supported
- **Documentation**: See [TLS SAN Configuration](tls-san-configuration.md) for detailed guide

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

#### RANCHER_DISK
- **Type**: String (device path)
- **Default**: None  
- **Description**: Device path for dedicated `/var/lib/rancher` storage. Primarily for GPU worker nodes with intensive workloads. Bloom will format and mount this device automatically.
- **Example**: `RANCHER_DISK: "/dev/nvme2n1"`
- **Requirements**: 
  - Must be a raw device path starting with `/dev/`
  - Device must exist and not be already mounted
  - Recommended 500GB+ available space
  - Mutually exclusive with `NO_DISKS_FOR_CLUSTER`
- **Primary Use Case**: **GPU worker nodes** with intensive workloads that benefit from dedicated fast storage for kubelet and container runtime data
- **Node Type Usage**: 
  - **GPU Worker Nodes** (Primary): Recommended for nodes with heavy GPU workloads, large container images, and extensive logging
  - **Control Plane Nodes** (Optional): Can be used for dedicated RKE2 control plane storage if desired
  - **CPU Worker Nodes** (Optional): May benefit nodes with high container churn or large log volumes

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

# Network and DNS (opt-in for safety)
FIX_DNS: false        # Set to true only if DNS is known to be broken

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

- `DISABLED_STEPS` and `ENABLED_STEPS` cannot both be set *(pending implementation)*
- `USE_CERT_MANAGER: true` and `CERT_OPTION: "existing"` cannot both be set

### Conditional Requirements

- `CONTROL_PLANE: true` requires `FIRST_NODE: false`
- `CERT_MANAGER_EMAIL` required when `USE_CERT_MANAGER: true`
- `TLS_CERT` and `TLS_KEY` required when `CERT_OPTION: "existing"`
- `DOCKERHUB_TOKEN` required when `DOCKERHUB_USER` is set (and vice versa)

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

### First Node with DNS Issues (Opt-in DNS Fix)
```yaml
FIRST_NODE: true
GPU_NODE: false
DOMAIN: "cluster.example.com"

# Enable DNS fixes (only if DNS is known to be broken)
FIX_DNS: true         # Allows automatic DNS repair if broken

USE_CERT_MANAGER: true
CERT_MANAGER_EMAIL: "admin@example.com"
```

### First Node with Custom DNS Servers
```yaml
FIRST_NODE: true
GPU_NODE: false
DOMAIN: "cluster.example.com"

# Use specific DNS servers instead of copying host DNS
DNS_SERVERS:
  - "8.8.8.8"        # Google Public DNS
  - "1.1.1.1"        # Cloudflare DNS  
  - "208.67.222.222" # OpenDNS

USE_CERT_MANAGER: true
CERT_MANAGER_EMAIL: "admin@example.com"
```

### Small Cluster with ClusterForge (GitOps, includes ArgoCD)
```yaml
FIRST_NODE: true
GPU_NODE: true
DOMAIN: "165.245.128.225.nip.io"
CERT_OPTION: generate
CLUSTER_SIZE: small
CLUSTER_DISKS: /dev/vdc1
CLUSTERFORGE_RELEASE: latest
```

### Bare Cluster (no ClusterForge, no ArgoCD)
```yaml
FIRST_NODE: true
GPU_NODE: true
DOMAIN: "165.245.128.225.nip.io"
CERT_OPTION: generate
CLUSTER_SIZE: small
CLUSTER_DISKS: /dev/vdc1
CLUSTERFORGE_RELEASE: none
```

### High-Performance GPU Worker Node (Primary Use Case)
```yaml
FIRST_NODE: false
CONTROL_PLANE: false
GPU_NODE: true
CLUSTER_DISKS: "/dev/nvme0n1,/dev/nvme1n1"
RANCHER_DISK: "/dev/nvme2n1"
SERVER_IP: "192.168.1.100"
JOIN_TOKEN: "K10..."
```

### First Node with Optional Dedicated Storage
```yaml
FIRST_NODE: true
GPU_NODE: true
DOMAIN: "cluster.example.com"
CERT_OPTION: "generate"
CLUSTER_DISKS: "/dev/nvme0n1,/dev/nvme1n1"
RANCHER_DISK: "/dev/nvme2n1"  # Optional for control plane
```

### Testing/Development Configuration
```yaml
FIRST_NODE: true
GPU_NODE: false
NO_DISKS_FOR_CLUSTER: true
DISABLED_STEPS: "install-longhorn,install-metallb,install-clusterforge"
```

### First Node with DockerHub Credentials (Avoiding Rate Limits)
```yaml
FIRST_NODE: true
GPU_NODE: false
DOMAIN: "cluster.example.com"
CERT_OPTION: generate
CLUSTER_DISKS: "/dev/nvme0n1"
CLUSTERFORGE_RELEASE: none

# DockerHub authenticated pulls — avoids anonymous rate limits
DOCKERHUB_USER: "myusername"
DOCKERHUB_TOKEN: "dckr_pat_xxxxxxxxxxxx"
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

## CLI Commands Reference

### CLI Command

Deploy cluster using configuration file:

```bash
bloom cli <config-file> [flags]
```

**Available Flags:**
- `--export`: Export the playbook to `./bloom-playbook/` (overwrites if exists) instead of executing it
- `--dry-run`: Run in check mode without making changes
- `--destroy-data`: ⚠️ DANGER: Wipes the cluster before redeploying (RKE2 uninstall, Longhorn cleanup, bloom-managed disk wipe). Shows a disk wipe preview before confirmation. Premounted disks (CLUSTER_PREMOUNTED_DISKS) have their bloom artifacts cleaned but their filesystem and fstab entries preserved
- `--preserve-existing-rke2`: Resume or reconcile an existing RKE2 installation without treating its running services and state directories as safety conflicts. Other safety checks, including disk checks, remain enforced
- `--pause-k3s`: Legacy alias — k3s conflicts are paused automatically; this flag still forces the pause step
- `--playbook string`: Playbook to run (default: "cluster-bloom.yaml")
- `--tags string`: Run only tasks with specific tags (e.g., cleanup, validate, storage)

**Examples:**
```bash
# Standard deployment
sudo ./bloom cli bloom.yaml

# Export playbook for inspection (writes ./bloom-playbook/)
./bloom cli bloom.yaml --export

# Resume an exported deployment after a partial run while preserving RKE2 state
./bloom cli bloom.yaml --export --preserve-existing-rke2

# Dry run deployment
sudo ./bloom cli bloom.yaml --dry-run

# Run specific tags only
sudo ./bloom cli bloom.yaml --tags "validate_node,prep_node"

# Two-part deployment — deploy infrastructure first, then ClusterForge
# Part 1: set CLUSTERFORGE_RELEASE: none in bloom.yaml and run the full deployment
sudo ./bloom cli bloom.yaml
# Part 2: once all nodes have joined, run the ClusterForge bootstrap separately
# (certificate params such as CERT_OPTION/TLS_CERT/TLS_KEY are not required here)
sudo ./bloom cli bloom.yaml --tags deploy_clusterforge
```

### Cleanup Command

Clean an existing Bloom installation:

```bash
sudo ./bloom cleanup [config-file]
```

Cleanup validates configured storage against strict Bloom-managed fstab entries, live block-device identities, mounts, and protected operating-system devices before making changes. A mismatch aborts before teardown or disk writes.

Use `--preflight-only` to run the same checks without confirmation or mutation:

```bash
sudo ./bloom cleanup bloom.yaml --preflight-only
```

Other cleanup flags:

- `--force`: Skip the destructive confirmation prompt after preflight succeeds
- `--yes`: Alias for automatic confirmation

### Run Command

Execute external Ansible playbook using Bloom's containerized runtime:

```bash
bloom run <playbook> [flags]
```

**Available Flags:**
- `--config string`: YAML config file whose keys become ansible extra vars
- `--dry-run`: Run in check mode without making changes
- `--extra-vars stringArray`: Extra variables passed to ansible-playbook (repeatable)
- `--tags string`: Run only tasks with specific tags
- `--verbose`: Show full Ansible output instead of clean summary

**Examples:**
```bash
# Run exported playbook
sudo ./bloom run bloom-playbook/cluster-bloom.yaml

# Run with additional configuration
sudo ./bloom run bloom-playbook/cluster-bloom.yaml --config extra-vars.yaml

# Run with inline variables
sudo ./bloom run bloom-playbook/cluster-bloom.yaml -e "CUSTOM_VAR=value" -e "ANOTHER_VAR=test"

# Run with verbose output
sudo ./bloom run bloom-playbook/cluster-bloom.yaml --verbose
```

### Export Workflow

The `--export` flag enables a workflow for playbook inspection and manual execution:

1. **Generate and Inspect**: Export the playbook directory to review what actions will be performed
2. **Modify if Needed**: Optionally customize files under `./bloom-playbook/`
3. **Execute Manually**: Run the playbook using the `run` command or `ansible-playbook`

```bash
# Step 1: Export playbook directory
./bloom cli bloom.yaml --export

# Step 2: Review the exported files
less bloom-playbook/cluster-bloom.yaml
less bloom-playbook/bloom-vars.yaml

# Step 3: Execute the playbook
sudo ./bloom run bloom-playbook/cluster-bloom.yaml
```

**Use Cases for Export:**
- **Debugging**: Understand exactly what the deployment will do
- **Compliance**: Review playbooks before execution in regulated environments
- **Customization**: Modify generated playbooks for specific requirements
- **Restricted Environments**: Generate playbooks on one system, execute on another
- **Learning**: Study the generated Ansible code to understand cluster setup
- **Existing Installations**: Run `bloom cleanup <config-file>` before export or deployment on existing clusters (`--destroy-data` cannot be combined with `--export`)
- **Loaned Nodes with k3s**: Bloom automatically pauses conflicting k3s before RKE2 deploy (non-destructive). After testing, remove RKE2 with `--destroy-data`, then `systemctl start k3s-server` (or `k3s`) to resume

**Technical Details:**
- **Directory Layout**: Export writes `./bloom-playbook/` with the root playbook, `bloom-vars.yaml` (config values), and the embedded `tasks/` and `manifests/` trees
- **Configuration Integration**: All user configuration values are written to `bloom-vars.yaml` and loaded by the exported root playbook
- **Standalone Execution**: The exported root playbook targets `localhost` and sets `BLOOM_DIR` so it can run outside Bloom's containerized runtime
- **Full Compatibility**: Exported playbooks work with the `bloom run` command and standard Ansible tools when run from the `./bloom-playbook/` directory
- **Disk Wipe Preview**: Both `bloom cleanup` and `bloom cli --destroy-data` show a preview of bloom-managed mounts and the future mount range before requiring confirmation
- **Cleanup Preflight**: Both cleanup paths validate `bloom.yaml`, strictly tagged fstab entries, live mounts, block-device identity, and the full system-device dependency chain before teardown
- **Premounted Disk Safety**: `CLUSTER_PREMOUNTED_DISKS` entries have bloom artifacts (pvc-*, replicas, longhorn-disk.cfg) removed but their filesystem, fstab entry, and user files are preserved
- **Smart Index Allocation**: Mount indexes are chosen as the lowest contiguous range not conflicting with premounted disk indexes (from fstab and config), so `CLUSTER_DISKS` and `CLUSTER_PREMOUNTED_DISKS` can coexist

## See Also

- [PRD](PRD.md) - Product overview and requirements
- [Technical Architecture](technical-architecture.md) - Technical architecture
- [Installation Guide](installation-guide.md) - Installation procedures
