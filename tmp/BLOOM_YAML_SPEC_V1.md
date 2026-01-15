# bloom.yaml Specification (V1 - Must Remain Identical in V2)

**Decision:** Bloom V2 reimplements the code but keeps the exact same bloom.yaml format.

## All Configuration Variables

Based on v1 implementation in `pkg/args/args_test.go`:

```yaml
# Node Configuration
FIRST_NODE: true                    # bool - Is this the first node in cluster?
GPU_NODE: true                      # bool - Does this node have GPUs?
CONTROL_PLANE: false                # bool - Control plane node? (only when FIRST_NODE=false)

# Cluster Join (for additional nodes)
SERVER_IP: ""                       # string - RKE2 server IP (required when FIRST_NODE=false)
JOIN_TOKEN: ""                      # string - Join token (required when FIRST_NODE=false)

# Domain & Networking
DOMAIN: ""                          # string - Domain name (required when FIRST_NODE=true)

# Certificates
USE_CERT_MANAGER: false             # bool - Use cert-manager + Let's Encrypt?
CERT_OPTION: ""                     # enum: "existing" or "generate" (when USE_CERT_MANAGER=false)
TLS_CERT: ""                        # file path - TLS cert (when CERT_OPTION=existing)
TLS_KEY: ""                         # file path - TLS key (when CERT_OPTION=existing)

# Authentication
ADDITIONAL_OIDC_PROVIDERS: []       # array - Additional OIDC providers

# GPU/ROCm
ROCM_BASE_URL: "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/"  # url (when GPU_NODE=true)

# Storage
CLUSTER_DISKS: ""                   # string - Comma-separated disk paths
CLUSTER_PREMOUNTED_DISKS: ""        # string - Premounted disk paths
NO_DISKS_FOR_CLUSTER: false         # bool - Skip disk operations?
SKIP_RANCHER_PARTITION_CHECK: false # bool - Skip partition size check?

# ClusterForge
CLUSTERFORGE_RELEASE: "https://..."  # url or "none"
CF_VALUES: ""                        # string - Path to CF values file

# Step Control
DISABLED_STEPS: ""                  # string - Comma-separated step names to skip
ENABLED_STEPS: ""                   # string - Comma-separated steps to run (if empty, run all)

# Misc
PRELOAD_IMAGES: ""                  # string - Images to preload
```

## Example: First Node

```yaml
FIRST_NODE: true
GPU_NODE: false
DOMAIN: cluster.example.com
USE_CERT_MANAGER: false
CERT_OPTION: generate
CLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1
NO_DISKS_FOR_CLUSTER: false
CLUSTERFORGE_RELEASE: none
PRELOAD_IMAGES: ""
```

## Example: Additional Node

```yaml
FIRST_NODE: false
GPU_NODE: false
SERVER_IP: 10.100.100.11
JOIN_TOKEN: K10abc123...xyz::server:abc123
CLUSTER_DISKS: /dev/nvme0n1,/dev/nvme1n1
NO_DISKS_FOR_CLUSTER: false
CLUSTERFORGE_RELEASE: none
```

## Dependencies

Some fields only apply when others are set:

- `CONTROL_PLANE`: Only when `FIRST_NODE=false`
- `SERVER_IP`, `JOIN_TOKEN`: Required when `FIRST_NODE=false`
- `DOMAIN`, `USE_CERT_MANAGER`, `CERT_OPTION`: Only when `FIRST_NODE=true`
- `TLS_CERT`, `TLS_KEY`: Required when `CERT_OPTION=existing`
- `ROCM_BASE_URL`: Only when `GPU_NODE=true`

## Validation Rules

From v1 validators:
- `JOIN_TOKEN`: Must be valid RKE2/K3s token format
- Step names in `DISABLED_STEPS`/`ENABLED_STEPS`: Must match valid step IDs
- `DISABLED_STEPS`/`ENABLED_STEPS`: Cannot both be set
- URLs: Must be valid URLs
- File paths: Must exist
- IP addresses: Must be valid IPs

## What V2 Must Do

1. **Parse exactly this format** - no changes to field names or structure
2. **Same validation rules** - maintain compatibility
3. **Same defaults** - users expect same behavior
4. **Same dependencies** - conditional fields work the same way

## What Web UI/CLI Must Generate

Web UI and CLI wizard must generate bloom.yaml files in exactly this format with these exact field names.

---

**Status:** V1 spec documented for V2 implementation
**Date:** 2025-12-08
