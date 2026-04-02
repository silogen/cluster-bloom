# OIDC Authentication Configuration

## Overview

ClusterBloom provides comprehensive OIDC (OpenID Connect) authentication support for Kubernetes clusters, enabling integration with enterprise identity providers like Keycloak, Auth0, Azure AD, and others. The system supports both automatic default provider configuration and multiple additional providers for complex authentication scenarios.

## Default Provider Behavior

### Automatic Configuration
When you configure a `DOMAIN` setting, ClusterBloom automatically sets up a default OIDC provider:

- **Provider URL**: `https://kc.{DOMAIN}/realms/airm`
- **Client ID**: `k8s`
- **Username Claim**: `preferred_username`
- **Groups Claim**: `groups`

### Example Default Configuration
```yaml
DOMAIN: "cluster.example.com"
# Automatically generates:
# OIDC Provider: https://kc.cluster.example.com/realms/airm
# Audience: k8s
```

**Generated RKE2 Configuration**:
```yaml
kube-apiserver-arg:
  - "--authentication-config=/etc/rancher/rke2/auth/auth-config.yaml"
```

## Multiple Provider Configuration

### Configuration Format
For complex environments requiring multiple authentication providers:

```yaml
DOMAIN: "cluster.example.com"
ADDITIONAL_OIDC_PROVIDERS:
  - url: "https://corp-auth.company.com/realms/employees"
    audiences: ["k8s", "api"]
  - url: "https://external-auth.partner.com/auth"
    audiences: ["kubernetes"]
  - url: "https://azure-tenant.b2clogin.com/tenant-id/v2.0"
    audiences: ["client-id-1", "client-id-2"]
```

### Provider Object Fields
Each provider in `ADDITIONAL_OIDC_PROVIDERS` requires:

- **url** (required): HTTPS URL of the OIDC provider's issuer endpoint
- **audiences** (required): Array of client IDs that this provider should accept

### Validation Rules
- URLs must use HTTPS protocol
- URLs must be valid and properly formatted
- Audiences array cannot be empty
- Duplicate URLs are automatically deduplicated

## Provider Integration Examples

### Keycloak Integration
```yaml
DOMAIN: "cluster.example.com"
ADDITIONAL_OIDC_PROVIDERS:
  - url: "https://keycloak.example.com/realms/kubernetes"
    audiences: ["k8s-cluster", "kubectl"]
```

**Keycloak Client Configuration**:
- Create client with ID matching audience (e.g., "k8s-cluster")
- Enable "Standard Flow" and "Direct Access Grants"
- Set valid redirect URIs for kubectl
- Configure user attributes mapping (username, groups)


## Authentication Flow

### User Authentication Process
1. **Token Acquisition**: User authenticates with OIDC provider via web browser
2. **Token Exchange**: Provider issues JWT token with user claims and groups
3. **kubectl Configuration**: User configures kubectl with token
4. **API Requests**: kubectl sends token in Authorization header
5. **Token Validation**: kube-apiserver validates token against configured providers
6. **Authorization**: Kubernetes RBAC determines user permissions

### kubectl Configuration

ClusterBloom automatically generates `kubeconfig-oidc-template.yaml` after installation. Administrators can distribute this template to developers for easy OIDC authentication setup with `kubectl oidc-login` plugin.

**For Developers**: Use the provided kubeconfig template file - no manual configuration needed.

## RBAC Integration

### User Permissions
After OIDC authentication, Kubernetes RBAC controls user permissions:

```yaml
# ClusterRole for OIDC users
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: oidc-user-role
rules:
- apiGroups: [""]
  resources: ["pods", "services"]
  verbs: ["get", "list", "watch"]

---
# ClusterRoleBinding for OIDC group
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: oidc-group-binding
subjects:
- kind: Group
  name: "k8s-users"  # Group claim from OIDC token
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: oidc-user-role
  apiGroup: rbac.authorization.k8s.io
```

### Group-based Access Control
ClusterBloom automatically configures these standard OIDC groups:

```yaml
# ClusterBloom standard OIDC groups
subjects:
- kind: Group
  name: "oidc:admin"      # Full cluster admin
  apiGroup: rbac.authorization.k8s.io
- kind: Group  
  name: "oidc:developer"  # Edit access
  apiGroup: rbac.authorization.k8s.io
- kind: Group
  name: "oidc:viewer"     # Read-only access
  apiGroup: rbac.authorization.k8s.io
```

**Note**: Configure these group mappings in your OIDC provider to match your users' roles.

## Troubleshooting

### Common Issues

#### Token Validation Failures
```
error: unable to authenticate the request due to an error on the client
```

**Solutions**:
- Verify OIDC provider URL is accessible from cluster nodes
- Check client ID matches provider configuration
- Ensure token hasn't expired
- Validate provider's SSL certificate

#### Provider URL Connectivity
```
failed to verify the token: oidc: failed to query provider
```

**Solutions**:
- Test provider URL accessibility: `curl https://provider-url/.well-known/openid-configuration`
- Check firewall rules allow HTTPS traffic from cluster to provider
- Verify DNS resolution for provider domain

#### Group Claims Not Working
```
User authenticated but has no permissions
```

**Solutions**:
- Verify groups claim is included in JWT token
- Check RBAC binding references correct group names
- Ensure group claim name matches kube-apiserver configuration

### Debug Commands

**Check Current Authentication**:
```bash
kubectl auth whoami
kubectl auth can-i --list
```

**Verify OIDC Configuration**:
```bash
# Check RKE2 configuration
sudo cat /etc/rancher/rke2/config.yaml | grep authentication-config

# Check authentication configuration
sudo cat /etc/rancher/rke2/auth/auth-config.yaml

# Check kube-apiserver logs
sudo journalctl -u rke2-server | grep authentication
```

**Test Token Locally**:
```bash
# Decode JWT token to inspect claims
echo "your-jwt-token" | cut -d. -f2 | base64 -d | jq .
```

## Security Considerations

### Best Practices
- **Use HTTPS Only**: All OIDC providers must use HTTPS
- **Token Expiration**: Configure appropriate token lifetimes
- **Group Mapping**: Use principle of least privilege for group RBAC
- **Audit Logging**: Enable Kubernetes audit logging for authentication events
- **Regular Rotation**: Rotate client secrets regularly

### Provider Security
- **Client Secret Management**: Store client secrets securely
- **Redirect URI Validation**: Configure strict redirect URI validation
- **Scope Limitation**: Request minimal required scopes
- **Certificate Validation**: Validate OIDC provider SSL certificates

## Advanced Configuration

### Multiple Audience Support
Configure multiple client IDs for single provider:

```yaml
ADDITIONAL_OIDC_PROVIDERS:
  - url: "https://provider.example.com"
    audiences: 
      - "k8s-prod"
      - "k8s-staging"  
      - "kubectl-cli"
      - "dashboard"
```

## See Also

- [01-rke2-deployment.md](./01-rke2-deployment.md) - RKE2 deployment details
- [07-technical-architecture.md](./07-technical-architecture.md) - Technical architecture
- [10-configuration-reference.md](./10-configuration-reference.md) - Configuration reference
- [PRD.md](./PRD.md) - Product overview