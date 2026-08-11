# CLUSTER_LISTEN_IP Configuration Manual

## Overview
The `CLUSTER_LISTEN_IP` feature allows you to specify which network interface your Kubernetes cluster should use for communication. This is particularly useful for systems with multiple network interfaces.

## When to Use
Use `CLUSTER_LISTEN_IP` when:
- Your system has multiple network interfaces
- You need to force cluster communication over a specific network
- You want to avoid automatic interface selection

## Configuration Methods

### 1. YAML Configuration
Add to your `bloom.yaml` file:
```yaml
CLUSTER_LISTEN_IP: "192.168.1.100"  # Explicit IP
# OR
CLUSTER_LISTEN_IP: "192.168.1.0/24" # CIDR subnet
```

### 2. Environment Variable
```bash
export CLUSTER_LISTEN_IP="192.168.1.100"
```

### 3. CLI Flag
```bash
sudo ./bloom cli bloom.yaml --cluster-listen-ip "192.168.1.100"
```

## Supported Formats

### Explicit IP Address
```yaml
CLUSTER_LISTEN_IP: "192.168.1.100"
```
Uses the exact IP address specified. The IP must exist on your system.

### CIDR Subnet
```yaml
CLUSTER_LISTEN_IP: "192.168.1.0/24"
```
Automatically selects the first IP address found in the specified subnet.

### Auto-detection (Default)
If not specified, the system automatically uses the default route interface.

## Priority Order
Configuration values are applied in this order (highest to lowest priority):
1. CLI flag (`--cluster-listen-ip`)
2. Environment variable (`CLUSTER_LISTEN_IP`)
3. YAML file configuration
4. Auto-detection (default behavior)

## Validation
The system validates that:
- IP addresses follow correct format
- Specified IP addresses exist on the target system
- CIDR subnets contain at least one matching interface
- Network interfaces are accessible

## Common Use Cases

### Multi-homed Server
```yaml
# Force cluster traffic over management network
CLUSTER_LISTEN_IP: "10.0.1.100"
```

### Development Environment
```bash
# Quick testing with specific interface
export CLUSTER_LISTEN_IP="192.168.56.10"
```

### Production Deployment
```yaml
# Use dedicated cluster network
CLUSTER_LISTEN_IP: "172.16.0.0/24"
```

That's it! The configuration is automatically applied during cluster deployment.

