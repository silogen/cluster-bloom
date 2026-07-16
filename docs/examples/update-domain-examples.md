# Domain Update Examples

This document provides practical examples for updating the domain or the TLS certificates in cluster-forge installations.

## Understanding Domain Updates

Domain updates in cluster-forge involve two layers:

### Application Layer
- Keycloak client redirect URIs
- AIRM cluster records
- Gitea repository configuration
- TLS certificates for ingress (Envoy Gateway)
- ConfigMaps and secrets

### RKE2 Infrastructure Layer (Default)
- API server certificate (regenerated with new domain SAN)
- OIDC authentication configuration
- RKE2 service restart

**Important:** RKE2 layer updates require restarting the rke2-server service, which causes API server downtime (typically 1-2 minutes per control plane node).

Use `--skip-rke2` to update only the application layer if you want to avoid API server downtime.

## Example 1: Development Environment with Self-Signed Certificates

**Scenario:** You're running a local development cluster and want to change from `dev.local` to `test.local`.

```bash
# Update domain with auto-generated certificate
./bloom update \
  --new-domain test.local \
  --cert-option generate

# The tool will:
# 1. Validate cluster connectivity
# 2. Generate new self-signed certificate with SANs
# 3. Update all cluster configurations
# 4. Display DNS requirements
```

**After completion:**
1. Update your local DNS or `/etc/hosts`:
   ```
   192.168.1.100  argocd.test.local
   192.168.1.100  gitea.test.local
   192.168.1.100  k8s.test.local
   192.168.1.100  kc.test.local
   ```
2. Update kubeconfig to use `k8s.test.local`
3. Wait for ArgoCD to sync (~5 minutes)

---

## Example 2: Production Environment with Commercial Certificate

**Scenario:** Moving production from `staging.company.com` to `prod.company.com` with a certificate from your CA.

```bash
# First, verify you have the certificate files
ls -l /secure/certs/
# prod.company.com.crt
# prod.company.com.key

# Verify certificate includes required SANs
openssl x509 -in /secure/certs/prod.company.com.crt -noout -text | grep DNS

# Preview the changes
./bloom update \
  --new-domain prod.company.com \
  --cert-option provide \
  --cert-path /secure/certs/prod.company.com.crt \
  --key-path /secure/certs/prod.company.com.key \
  --dry-run

# Apply the update
./bloom update \
  --new-domain prod.company.com \
  --cert-option provide \
  --cert-path /secure/certs/prod.company.com.crt \
  --key-path /secure/certs/prod.company.com.key
```

**After completion:**
1. Update DNS records at your DNS provider
2. Monitor ArgoCD sync progress:
   ```bash
   kubectl get applications -n argocd -w
   ```
3. Test all critical endpoints
4. Update monitoring dashboards and alerts
5. Notify team members of the new URLs

---

## Example 3: Using cert-manager with Let's Encrypt

**Scenario:** Automated certificate management with cert-manager already configured.

```bash
# Check cert-manager is configured
kubectl get certificate cluster-tls -n envoy-gateway-system

# Update domain (cert-manager will issue new certificate automatically)
./bloom update \
  --new-domain app.example.com \
  --cert-option cert-manager

# Wait for certificate to be issued
kubectl wait --for=condition=Ready \
  certificate/cluster-tls \
  -n envoy-gateway-system \
  --timeout=300s

# Verify certificate
kubectl get certificate cluster-tls -n envoy-gateway-system -o yaml
```

**After completion:**
1. Update DNS records (required for Let's Encrypt DNS-01 challenge)
2. cert-manager will automatically renew the certificate before expiration
3. No manual certificate management needed

---

## Example 4: Update TLS Certificates Only

**Scenario:** Your certificate is expiring soon but the domain remains the same.

```bash
# Option 1: Generate new self-signed certificate
./bloom update --cert-option generate

# Option 2: Provide new certificate from your CA
./bloom update \
  --cert-option provide \
  --cert-path /secure/certs/renewed-cert.crt \
  --key-path /secure/certs/renewed-key.key

# Option 3: Use cert-manager to issue new certificate
./bloom update --cert-option cert-manager

# The tool will:
# 1. Validate cluster connectivity
# 2. Update TLS certificates (cluster-tls secret)
# 3. Restart gateway pods to pick up new certificate
# 4. Skip all domain configuration updates
```

**After completion:**
1. Verify new certificate is active:
   ```bash
   # Check certificate expiration
   kubectl get secret cluster-tls -n envoy-gateway-system -o jsonpath='{.data.tls\.crt}' | \
     base64 -d | openssl x509 -noout -enddate
   
   # Test HTTPS endpoint
   curl -vI https://argocd.your-domain.com 2>&1 | grep "expire date"
   ```
2. No DNS changes required
3. No ArgoCD sync needed (domain configuration unchanged)
4. Services remain accessible at existing URLs

---

## Example 5: Pre-Flight DNS Check

**Scenario:** Verify DNS is configured correctly before updating the domain.

```bash
# First, update your DNS records manually
# Then verify they're propagating correctly

./bloom update --check-dns new.example.com

# Output will show:
# ✅ Checking argocd.new.example.com... ✅ (203.0.113.50)
# ✅ Checking gitea.new.example.com... ✅ (203.0.113.50)
# ⚠️  Checking k8s.new.example.com... ❌ (not found)
# ...

# Wait for DNS to propagate, then proceed with domain update
```

---

## Example 6: Rolling Back to Previous Domain

**Scenario:** Something went wrong, need to revert to the old domain.

```bash
# Run the update-domain command again with the original domain
./bloom update \
  --new-domain original.example.com \
  --cert-option generate

# Restore DNS records to point to original domain
# Wait for ArgoCD to sync
# Verify services are accessible
```

---

## Example 7: Large Cluster with Minimal Downtime

**Scenario:** Production cluster requiring careful domain migration.

```bash
# Step 1: Prepare new certificates ahead of time
# (certificate matches new domain)

# Step 2: Schedule maintenance window
# Notify users of upcoming change

# Step 3: Run dry-run to verify
./bloom update \
  --new-domain prod.company.com \
  --cert-option provide \
  --cert-path /certs/prod.crt \
  --key-path /certs/prod.key \
  --dry-run

# Step 4: Execute during maintenance window
./bloom update \
  --new-domain prod.company.com \
  --cert-option provide \
  --cert-path /certs/prod.crt \
  --key-path /certs/prod.key

# Step 5: Monitor ArgoCD sync progress
watch kubectl get applications -n argocd

# Step 6: Verify each service manually
curl -k https://argocd.prod.company.com
curl -k https://gitea.prod.company.com
# etc.

# Step 7: Update kubeconfig for all users
# Send updated configuration to team
```

---

## Example 8: Application-Layer-Only Update (Skip RKE2)

**Scenario:** Update application URLs without API server downtime.

```bash
# Update only application layer, skip RKE2 infrastructure updates
./bloom update \
  --new-domain new.company.com \
  --cert-option generate \
  --skip-rke2

# The tool will:
# 1. Update application configurations (Keycloak, AIRM, Gitea)
# 2. Update ingress TLS certificates
# 3. Skip API server certificate regeneration
# 4. Skip rke2-server restart (no downtime)
```

**After completion:**
1. Applications are accessible at new domain
2. API server still responds only on old domain:
   ```bash
   kubectl --server=https://k8s.old.company.com:6443 get nodes  # ✅ Works
   kubectl --server=https://k8s.new.company.com:6443 get nodes  # ❌ TLS error
   ```
3. To complete the migration, run without `--skip-rke2` during maintenance window:
   ```bash
   ./bloom update \
     --new-domain new.company.com \
     --cert-option generate
   ```

**Use case:** Deploy application changes first, then schedule RKE2 updates during off-hours.

---

## Example 9: Complete Domain Update with RKE2 Layer

**Scenario:** Full domain migration including API server certificate.

```bash
# Complete update (default behavior - includes RKE2 layer)
./bloom update \
  --new-domain new.company.com \
  --cert-option generate

# The tool will:
# 1. Update application layer
# 2. Update RKE2 config and auth files
# 3. Regenerate API server certificate
# 4. Restart rke2-server (causes ~1-2 min downtime)
# 5. Wait for cluster readiness
```

**After completion:**
1. Verify API server responds on both domains (dual SAN):
   ```bash
   kubectl --server=https://k8s.new.company.com:6443 get nodes  # ✅ Works
   kubectl --server=https://k8s.old.company.com:6443 get nodes  # ✅ Also works
   ```
2. Update kubeconfig to use new domain:
   ```bash
   kubectl config set-cluster default --server=https://k8s.new.company.com:6443
   ```
3. Verify certificate contains both domains:
   ```bash
   openssl s_client -connect k8s.new.company.com:6443 </dev/null 2>&1 | \
     openssl x509 -noout -text | grep -A1 "Subject Alternative Name"
   # Should show: DNS:k8s.old.company.com, DNS:k8s.new.company.com
   ```

---

## Troubleshooting Examples

### Issue: ArgoCD Applications Stuck Syncing

```bash
# Check which apps are having issues
kubectl get applications -n argocd | grep -v Synced

# Force refresh a stuck application
kubectl patch application <app-name> -n argocd \
  --type merge \
  -p '{"metadata":{"annotations":{"argocd.argoproj.io/refresh":"hard"}}}'

# Or delete and let ArgoCD recreate
kubectl delete application <app-name> -n argocd
# ArgoCD will recreate it from cluster-forge
```

### Issue: Gateway Not Picking Up New Certificate

```bash
# Restart envoy gateway pods
kubectl rollout restart deployment -n envoy-gateway-system \
  -l gateway.envoyproxy.io/owning-gateway-name=https

# Verify secret was updated
kubectl get secret cluster-tls -n envoy-gateway-system -o yaml

# Check gateway status
kubectl describe gateway https -n envoy-gateway-system
```

### Issue: DNS Not Resolving

```bash
# Test DNS resolution
dig argocd.new-domain.com
nslookup k8s.new-domain.com

# Check DNS propagation
./bloom update --check-dns new-domain.com

# Update local DNS for testing
sudo echo "192.168.1.100 argocd.new-domain.com" >> /etc/hosts
```

---

## Best Practices

1. **Always use --dry-run first** to preview changes
2. **Test DNS configuration** before updating domain
3. **Schedule maintenance windows** for production changes
4. **Backup certificates** before making changes
5. **Monitor ArgoCD sync progress** after update
6. **Test all critical paths** after domain change
7. **Document the change** in your runbooks
8. **Keep old domain active** during DNS TTL period
9. **Update monitoring/alerting** to use new domain
10. **Communicate with users** about the change
