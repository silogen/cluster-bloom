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
- **Cilium**: `small` and `medium` set `operator.replicas: 1` via RKE2 `HelmChartConfig` on the first node at bootstrap; `large` uses the RKE2 default (2). To scale the operator after adding nodes, see [RKE2 deployment — Scaling cilium-operator](rke2-deployment.md#scaling-cilium-operator-after-install-multi-node--ha)

#### AIM_HARDWARE_FAMILY
- **Type**: String (comma-separated list)
- **Default**: `""` (empty)
- **Description**: Selects which AIM model sources cluster-forge installs, by hardware family. When left empty, bloom auto-detects the hardware physically present on this node — AMD GPU family/families via a local PCI scan, plus `epyc` if the CPU is an AMD EPYC part (via `/proc/cpuinfo`) — and uses that as the list; if nothing is detected (or detection itself fails), it falls back to the legacy branch: only the fixed generic `amd-aim-release-*` + `amd-aim-instinct-*` catalog installs, unchanged from before per-family support existed. `cpu` is never auto-detected — it stays opt-in only. When set explicitly, only the listed families' sources are installed (the legacy generic sources are not installed), and detection never overrides your value — but if detection finds hardware not in your explicit list (e.g. `AIM_HARDWARE_FAMILY: "epyc"` on a box that also has a Radeon GPU), bloom prints an informational notice naming the gap rather than silently using your config as-is with no feedback.
- **Values**: any comma-separated combination of `cpu`, `epyc`, `instinct`, `radeon` (lowercase, no spaces)
- **Example**: `AIM_HARDWARE_FAMILY: "epyc,instinct"`
- **Notes**: `instinct` and `radeon` are GPU families; `cpu` and `epyc` are CPU inference targets. All families' images are hosted on `docker.io`; `cpu` and `radeon` use the `silogenai` org and the chart references an optional `dockerhub-regcred` pull secret for those, in case the images are private. In a `bloom.yaml` file the value is a normal comma-separated string. cluster-bloom splits it into a list before passing it to cluster-forge, so no comma-escaping is needed at the bloom layer. Unlike `GPU_STACK_FAMILY`, a node with hardware from multiple families is never ambiguous here — it's multi-select, so auto-detection just lists every family found (GPU families plus `epyc`). See [GPU family auto-detection and ambiguous hardware](rocm-support.md#gpu-family-auto-detection-and-ambiguous-hardware) for the detection mechanism.

#### GPU_STACK_FAMILY
- **Type**: String (single value)
- **Default**: `""` (empty, auto-detected from this node's AMD GPU hardware; resolves to `instinct` if nothing is detected)
- **Description**: Selects the ROCm + GPU Operator install defaults by GPU family. This is independent of `AIM_HARDWARE_FAMILY` (which selects the AIM model catalog). When left empty, bloom auto-detects the AMD GPU family from a local PCI scan; if none is detected, it keeps the current qualified defaults (host ROCm `7.2.3`, GPU Operator `v1.4.1`, DeviceConfig ROCm driver `7.0`), so existing installs are unchanged. `radeon` selects the ROCm 7.13 tech-preview stack. Setting this explicitly always wins over detection.
- **Values**: `radeon` | `instinct` (lowercase, single value)
- **Example**: `GPU_STACK_FAMILY: "radeon"`
- **Notes**:
  - Single-select by design: host ROCm is one version per node, so a heterogeneous Radeon + Instinct GPU stack cannot be expressed here. The AIM catalog (`AIM_HARDWARE_FAMILY`) can still be heterogeneous.
  - **Ambiguous hardware**: if this node has AMD GPUs from *both* families and `GPU_STACK_FAMILY` is left unset, bloom cannot guess which stack you want — it prints the detected models for each family and interactively asks you to choose `instinct` or `radeon`, with an explanation of why. Running non-interactively (`--yes`/`--auto-confirm-prompts`, or no readable stdin) hard-fails instead of guessing; set `GPU_STACK_FAMILY` explicitly to proceed. See [GPU family auto-detection and ambiguous hardware](rocm-support.md#gpu-family-auto-detection-and-ambiguous-hardware) for details — this prompt is safe from the "no TTY" limitation noted below because it runs before bloom re-execs into the ansible/SSH container.
  - Selecting `radeon` defaults host ROCm and the GPU Operator to the ROCm 7.13 tech-preview train. These components are tech preview, not production qualified, and bloom prints a notice at install time.
  - Unsupported combinations (for example a Radeon stack resolving to ROCm 7.2) fail validation before install with an error naming the incompatible component.
  - **Overriding the version guard**: when a GPU node already has ROCm installed that does not match the selected family's train (e.g. `radeon` on a host with ROCm 7.2.3), bloom aborts early during node validation with an "Unsupported ROCm version" message. This guard is a hard fail (no interactive prompt, because bloom pipes ansible output over SSH with no TTY). To proceed anyway with the currently installed ROCm, set [`ROCM_ALLOW_VERSION_MISMATCH`](#rocm_allow_version_mismatch) in `bloom.yaml`.
  - The exact ROCm 7.13 tech-preview version strings and the vendored GPU Operator chart are tracked in EAI-5906; until that lands the `radeon` row carries placeholder pins.

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
- **Post-run summary**: after a run on the first node, bloom prints a ClusterForge section based on **actual deployment evidence**, not config alone. It checks for application pods in the cluster-forge app namespaces (`aiwb`, `airm`, `aim-system`, `blueprints`) via `kubectl`. If pods are found — or cluster-forge was deployed in this same run (a full run or `--tags deploy_clusterforge`) — it prints the endpoint/credential block. If no deployment is detected, it instead prints how to deploy it: set `CLUSTERFORGE_RELEASE` and run `sudo <bloom> cli <your-config> --tags deploy_clusterforge`. This means a `--tags prepare_node`/`validate_node` run no longer shows a misleading deployment banner for a cluster-forge that was never installed.

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

#### ROCM_ALLOW_VERSION_MISMATCH
- **Type**: Boolean
- **Default**: `false`
- **Description**: Force continuation past the ROCm version guard. On a GPU node, if the already-installed ROCm does not match the train required by `GPU_STACK_FAMILY` (e.g. `radeon` needs ROCm 7.13 but the host has 7.2.3), bloom flags it in two places: the interactive top-level *hardware / configuration mismatch check* (a `[y/N]` prompt before the playbook runs) and, as the authoritative backstop, a hard fail during ansible node validation (no prompt there — bloom pipes ansible output over SSH, so there is no TTY). Setting this to true suppresses the ROCm-version dimension of both, proceeding with the installed ROCm. Note: this flag governs only the ROCm-version check; other mismatches (GPU_NODE, GPU_STACK_FAMILY family, AIM_HARDWARE_FAMILY) are still surfaced by the interactive prompt and bypassed non-interactively with `--yes`/`--auto-confirm-prompts`.
- **Values**: `true` | `false` (also accepts `TRUE` / `1`)
- **Applicable**: `GPU_NODE: true`
- **Example**: `ROCM_ALLOW_VERSION_MISMATCH: true`
- **Notes**: Works with `bloom cli bloom.yaml`. With `bloom run` it can also be passed as an extra-var: `-e ROCM_ALLOW_VERSION_MISMATCH=true`.

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
- `--export`: Export generated playbook to stdout instead of executing it
- `--dry-run`: Run in check mode without making changes
- `--destroy-data`: ⚠️ DANGER: Wipes the cluster before redeploying (RKE2 uninstall, Longhorn cleanup, bloom-managed disk wipe). Shows a disk wipe preview before confirmation. Premounted disks (CLUSTER_PREMOUNTED_DISKS) have their bloom artifacts cleaned but their filesystem and fstab entries preserved
- `--skip-data-safety`: Downgrade the pre-deployment data-safety failure (running RKE2 / non-empty `/var/lib/rancher/rke2` and `/etc/rancher/rke2`) to a warning so bloom can re-run on an already-provisioned node — for example to add or update ClusterForge — without `--destroy-data`. Existing cluster and disk data are preserved. Does NOT bypass the premounted-disk mount check. Also settable in `bloom.yaml` as `SKIP_DATA_SAFETY: true`
- `--playbook string`: Playbook to run (default: "cluster-bloom.yaml")
- `--tags string`: Run only tasks with specific tags (e.g., cleanup, validate, storage)

**Examples:**
```bash
# Standard deployment
sudo ./bloom cli bloom.yaml

# Export playbook for inspection
./bloom cli bloom.yaml --export

# Export and save to file
./bloom cli bloom.yaml --export > deployment.yaml

# Export with cleanup tasks for existing installations
./bloom cli bloom.yaml --export --destroy-data > cleanupDeployment.yaml

# Dry run deployment
sudo ./bloom cli bloom.yaml --dry-run

# Run specific tags only
sudo ./bloom cli bloom.yaml --tags "validate_node,prep_node"

# Two-part deployment — deploy infrastructure first, then ClusterForge
# Part 1: set CLUSTERFORGE_RELEASE: none in bloom.yaml and run the full deployment
sudo ./bloom cli bloom.yaml
# Part 2: once all nodes have joined, run the ClusterForge bootstrap separately
sudo ./bloom cli bloom.yaml --tags deploy_clusterforge

# Add/update ClusterForge on an already-provisioned node via a FULL re-run.
# A plain re-run fails the pre-deployment data-safety check (RKE2 is already
# running and the rke2 dirs are non-empty). To keep the existing cluster and
# only layer ClusterForge on top, either run just the ClusterForge tag (above),
# or re-run the whole playbook with --skip-data-safety:
sudo ./bloom cli bloom.yaml --skip-data-safety
```

#### Node validation (`--tags validate_node`)

`validate_node` is a **read-only diagnostic**: it never mutates node state and is designed to be run standalone against a minimal or even empty `bloom.yaml`.

- **Relaxed config validation**: a `--tags validate_node` run validates the config in "optional" mode — it still flags unknown keys and malformed values, but does **not** require full cluster fields (`DOMAIN`, `CERT_OPTION`, ...). This lets you check a node before you have a complete config. An empty `bloom.yaml` is accepted.
- **Runs all checks, reports once**: instead of aborting at the first failure, `validate_node` runs every check (Ubuntu version, CPU/memory/disk, kernel modules, `/var/lib/rancher` partition size, iptables, and — on GPU nodes — installed ROCm compatibility) and then fails once with a consolidated summary listing every issue found. If all checks pass it prints `✅ All node validation checks passed.`

The `/var/lib/rancher` partition check is two-tier and configurable:

- `RANCHER_PARTITION_RECOMMENDED_GB` (default `500`) — below this the check **warns** but continues.
- `RANCHER_PARTITION_MIN_GB` (default `100`) — below this the check **records a failure**.

`RANCHER_PARTITION_MIN_GB` must be `<=` `RANCHER_PARTITION_RECOMMENDED_GB` (validated at load time), and both must be whole numbers of GB. Set `SKIP_RANCHER_PARTITION_CHECK: true` to skip the partition check entirely.

Note: when the same ROCm compatibility guard runs as part of `prepare_node` (or a full deploy), it still fails fast before any package/kernel work, since that is a mutating path.

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
sudo ./bloom run myPlaybook.yaml

# Run with additional configuration
sudo ./bloom run myPlaybook.yaml --config extra-vars.yaml

# Run with inline variables
sudo ./bloom run myPlaybook.yaml -e "CUSTOM_VAR=value" -e "ANOTHER_VAR=test"

# Run with verbose output
sudo ./bloom run myPlaybook.yaml --verbose
```

### Export Workflow

The `--export` flag enables a powerful workflow for playbook inspection and manual execution:

1. **Generate and Inspect**: Export the playbook to review what actions will be performed
2. **Modify if Needed**: Optionally customize the exported playbook
3. **Execute Manually**: Run the playbook using the `run` command

```bash
# Step 1: Export playbook
./bloom cli bloom.yaml --export > deployment.yaml

# Step 1b: Export with cleanup for existing installations
./bloom cli bloom.yaml --export --destroy-data > cleanupDeployment.yaml

# Step 2: Review the playbook
less deployment.yaml

# Step 3: Execute the playbook
sudo ./bloom run deployment.yaml
```

**Use Cases for Export:**
- **Debugging**: Understand exactly what the deployment will do
- **Compliance**: Review playbooks before execution in regulated environments
- **Customization**: Modify generated playbooks for specific requirements
- **Restricted Environments**: Generate playbooks on one system, execute on another
- **Learning**: Study the generated Ansible code to understand cluster setup
- **Existing Installations**: Use `--export --destroy-data` to handle existing cluster installations safely

**Technical Details:**
- **Self-Contained Playbooks**: Exported playbooks automatically inline all `include_tasks` directives, creating completely self-contained files
- **Configuration Integration**: All user configuration values are properly applied to playbook variables
- **Task Preservation**: Tags, when conditions, and other metadata from include directives are preserved on inlined tasks
- **Full Compatibility**: Exported playbooks are fully compatible with the `bloom run` command and standard Ansible tools
- **Cleanup Task Injection**: When `--destroy-data` is used with `--export`, cleanup tasks are automatically prepended. These tasks are equivalent to running `bloom cleanup <config-file>` before the deployment
- **Disk Wipe Preview**: Both `bloom cleanup` and `bloom cli --destroy-data` show a preview of bloom-managed mounts and the future mount range before requiring confirmation
- **Premounted Disk Safety**: `CLUSTER_PREMOUNTED_DISKS` entries have bloom artifacts (pvc-*, replicas, longhorn-disk.cfg) removed but their filesystem, fstab entry, and user files are preserved
- **Smart Index Allocation**: Mount indexes are chosen as the lowest contiguous range not conflicting with premounted disk indexes (from fstab and config), so `CLUSTER_DISKS` and `CLUSTER_PREMOUNTED_DISKS` can coexist

## See Also

- [PRD.md](../../PRD.md) - Product overview
- [06-technical-architecture.md](07-technical-architecture.md) - Technical architecture
- [07-installation-guide.md](08-installation-guide.md) - Installation procedures
