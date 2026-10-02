# Certificate Management in Cluster-Bloom

This document describes how TLS certificates are managed and handled in the Cluster-Bloom project.

## Overview

Cluster-Bloom provides flexible certificate management options to secure HTTPS traffic for your Kubernetes cluster. The system supports both automated certificate management through cert-manager and manual certificate provisioning.

## TLS Subject Alternative Names (SAN)

Configure additional domain names for your Kubernetes API server certificate using `ADDITIONAL_TLS_SAN_URLS`. This allows secure access via multiple domains beyond the default `k8s.<DOMAIN>`.

TLS SAN configuration applies to all control-plane nodes since each runs its own kube-apiserver.

**Quick Example:**
```yaml
DOMAIN: cluster.example.com
ADDITIONAL_TLS_SAN_URLS:
  - "api.cluster.example.com"
  - "kubectl.cluster.example.com"
```

For complete TLS SAN configuration details, see [TLS SAN Configuration](tls-san-configuration.md).

## Certificate Management Options

### 1. Automatic Certificate Management (Recommended)

When `USE_CERT_MANAGER=true`, the cluster uses cert-manager with Let's Encrypt for automatic certificate provisioning and renewal.

**Benefits:**
- Automatic certificate issuance
- Automatic renewal before expiration
- No manual intervention required
- Free SSL certificates from Let's Encrypt

**Requirements:**
- Public domain name accessible from the internet
- Port 80/443 accessible for Let's Encrypt validation

### 2. Manual Certificate Management

When `USE_CERT_MANAGER=false`, you must provide your own certificates. This option supports two approaches:

#### a) Using Existing Certificates

Set `CERT_OPTION=existing` and provide:
- `TLS_CERT`: Path to your TLS certificate file (PEM format)
- `TLS_KEY`: Path to your TLS private key file (PEM format)

**Example:**
```bash
export USE_CERT_MANAGER=false
export CERT_OPTION=existing
export TLS_CERT=/path/to/your/certificate.crt
export TLS_KEY=/path/to/your/private.key
```

#### b) Generate Self-Signed Certificates

Set `CERT_OPTION=generate` to automatically create a self-signed certificate on the first node.

**Characteristics:**
- Valid for 365 days
- Includes domain and wildcard subdomain (*.domain.com)
- Uses RSA 2048-bit encryption
- Generated using OpenSSL
- First node only (used for ingress/gateway TLS)

**Example:**
```bash
export USE_CERT_MANAGER=false
export CERT_OPTION=generate
export DOMAIN=cluster.example.com
```

## Certificate Storage

All TLS certificates are stored as Kubernetes secrets:
- **Secret Name:** `cluster-tls`
- **Namespace:** `envoy-gateway-system`
- **Type:** `kubernetes.io/tls`

The secret contains:
- `tls.crt`: The TLS certificate
- `tls.key`: The TLS private key

## Configuration Workflow

### Using the Wizard

The cluster setup wizard (`bloom wizard`) guides you through certificate configuration:

1. Choose whether to use cert-manager
2. If not using cert-manager, select certificate option:
   - `existing`: Provide paths to certificate files
   - `generate`: Auto-generate self-signed certificate
3. Validate certificate files (if using existing)


**Note:** When using `CERT_OPTION=generate`, you do NOT need to provide `TLS_CERT` or `TLS_KEY` values. The system will automatically generate and configure the certificates during setup.


### Manual Configuration

Set the following environment variables before running bloom:

```bash
# For cert-manager (automatic certificates)
export USE_CERT_MANAGER=true
export DOMAIN=cluster.example.com

# For existing certificates
export USE_CERT_MANAGER=false
export CERT_OPTION=existing
export TLS_CERT=/path/to/cert.pem
export TLS_KEY=/path/to/key.pem
export DOMAIN=cluster.example.com

# For self-signed certificates
export USE_CERT_MANAGER=false
export CERT_OPTION=generate
export DOMAIN=cluster.example.com
```

## Certificate Validation

The system performs validation at multiple stages:

1. **Configuration Validation:**
   - Ensures required certificate options are set
   - Validates file paths exist (for existing certificates)

2. **File Validation:**
   - Checks certificate and key files are readable
   - Verifies files exist at specified paths

3. **Runtime Validation:**
   - Creates Kubernetes secret successfully
   - Applies configuration to the cluster

## Updating Certificates in Running Clusters

Replace the ingress TLS certificate on a running cluster with `bloom update`. The command updates the `cluster-tls` secret in the `envoy-gateway-system` namespace. It does not change the cluster domain, and it does not regenerate the RKE2 API server certificate.

### Prerequisites

- A running RKE2 cluster that cluster-bloom deployed
- The new TLS certificate file and private key file on the node
- Root access on that node

### Update procedure

Run this command on the node. `<cert>` is the path to the certificate file. `<key>` is the path to the private key file.

```bash
sudo bloom update --cert-option provide --cert-path <cert> --key-path <key>
```

If the certificate does not include the current cluster domain, the command shows a warning and asks you to continue.

Other `--cert-option` values:

- `generate` — create a new self-signed certificate
- `cert-manager` — update the cert-manager Certificate so it issues a new certificate

Run `bloom update --help` for domain changes and the full flag list.

### What the command does

1. Reads the current domain from the `cluster-domain` ConfigMap.
2. Checks that the certificate file and the key file exist.
3. Warns you when the certificate does not include the current domain.
4. Replaces the `cluster-tls` secret in `envoy-gateway-system`.

### Verification

After the command completes:

1. Check the secret:

   ```bash
   kubectl get secret cluster-tls -n envoy-gateway-system
   ```

2. Open these URLs in a browser and confirm the new certificate:
   - `https://gitea.<your-domain>`
   - `https://argocd.<your-domain>`

### Notes

- The command does not save a copy of the current secret. Save one first when you need a backup:

  ```bash
  kubectl get secret cluster-tls -n envoy-gateway-system -o yaml > cluster-tls-backup.yaml
  ```

- This update changes only the ingress `cluster-tls` secret.
- On a multi-node cluster, etcd copies the secret to the other nodes.

## Integration with Ingress

The TLS certificates are used by the cluster's ingress controller to enable HTTPS:

1. The `cluster-tls` secret is referenced in the bloom configuration
2. Ingress controllers use this secret for TLS termination
3. All HTTPS traffic to the configured domain uses these certificates

## Development and Testing

For local development on macOS, helper scripts are provided:

- `setup-certs-macos.sh`: Exports system certificates for Docker builds
- `test-certs.sh`: Tests certificate chain and connectivity
- `Dockerfile.macos`: Builds containers with proper certificate bundles

These scripts help resolve certificate validation issues during development, especially with enterprise proxies or custom certificate authorities.

## Best Practices

1. **Production Use:**
   - Use cert-manager with Let's Encrypt for automatic management
   - Ensure your domain has proper DNS records pointing to the cluster

2. **Development/Testing:**
   - Use self-signed certificates for isolated environments
   - Consider using existing certificates from your organization's CA

3. **Security:**
   - Store certificate files securely before deployment
   - Use appropriate file permissions (readable only by authorized users)
   - Rotate certificates regularly if not using cert-manager

## Troubleshooting

### Certificate Not Found
```
Error: TLS_CERT file does not exist: /path/to/cert
```
**Solution:** Verify the certificate file path is correct and the file exists.

### Invalid Certificate Option
```
Error: When USE_CERT_MANAGER is false, CERT_OPTION must be specified
```
**Solution:** Set `CERT_OPTION` to either `existing` or `generate`.

### Self-Signed Certificate Warnings
Browsers will show security warnings for self-signed certificates. This is expected behavior. For production use, use cert-manager or provide certificates from a trusted CA.