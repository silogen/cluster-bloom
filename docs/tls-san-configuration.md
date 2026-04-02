# TLS SAN Configuration

## Overview

The `ADDITIONAL_TLS_SAN_URLS` field allows you to specify additional domain names for the Kubernetes API server certificate. This enables secure access to your cluster using multiple domain names beyond the automatically generated `k8s.<DOMAIN>`.

## Quick Start

```yaml
# Basic configuration
DOMAIN: cluster.example.com
ADDITIONAL_TLS_SAN_URLS:
  - "api.cluster.example.com"
  - "kubectl.cluster.example.com"
```

## Key Features

- **Array Format**: Clean YAML array syntax (no more comma-separated strings)
- **Real-time Validation**: UI prevents wildcard domains with immediate feedback
- **Automatic Base Domain**: System automatically includes `k8s.<DOMAIN>`
- **RKE2 Integration**: Domains are added to RKE2's certificate configuration
- **Wildcard Protection**: Blocks `*.domain.com` patterns that break RKE2

## Configuration Examples

### Single Additional Domain
```yaml
DOMAIN: prod.company.com
ADDITIONAL_TLS_SAN_URLS:
  - "api.prod.company.com"
```

### Multiple API Endpoints
```yaml
DOMAIN: k8s.example.org
ADDITIONAL_TLS_SAN_URLS:
  - "api.k8s.example.org"
  - "kubectl.k8s.example.org"
  - "dashboard.k8s.example.org"
```

### Production Setup
```yaml
DOMAIN: cluster.enterprise.com
ADDITIONAL_TLS_SAN_URLS:
  - "api.cluster.enterprise.com"
  - "management.cluster.enterprise.com"
  - "monitoring.cluster.enterprise.com"
```

## Validation Rules

### ✅ Valid Domains
- `api.example.com`
- `management.cluster.example.org`
- `kubectl-prod.company.net`
- `api-v2.test.co`

### ❌ Invalid Domains
- `*.example.com` (wildcards not supported)
- `api.*.example.com` (wildcards anywhere)
- `invalid` (no top-level domain)
- `example.c` (TLD too short)

## UI Validation

The Bloom UI provides real-time validation:

- **Green Border**: Valid domain entered
- **Red Border**: Invalid domain (with error message)
- **Error Message**: Clear explanation of validation issue
- **Form Blocking**: Cannot generate config until all domains are valid

## Generated Certificate

Your configuration will result in a certificate containing:
```yaml
# For DOMAIN: cluster.example.com with additional SANs
Certificate SANs:
- k8s.cluster.example.com          # Auto-generated
- api.cluster.example.com          # Your additional domains
- kubectl.cluster.example.com      # Your additional domains
```

## Migration from String Format

**Old format (still supported):**
```yaml
ADDITIONAL_TLS_SAN_URLS: "api.example.com,kubectl.example.com"
```

**New format (recommended):**
```yaml
ADDITIONAL_TLS_SAN_URLS:
  - "api.example.com"
  - "kubectl.example.com"
```