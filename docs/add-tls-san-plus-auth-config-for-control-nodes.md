# Control Node Manual Upgrade Guide

## Overview

Existing control plane nodes deployed before the `v2.2.2` fix lack DOMAIN configuration, resulting in missing TLS SAN and OIDC authentication settings. This causes kubectl TLS verification and OIDC login failures on those nodes.

**Solution:** Copy two configuration blocks from the first node to each additional control plane node.

---

## Prerequisites

- Identify your first control node (already has the correct configuration)
- List all additional control plane nodes that need updating
- Root/sudo access to all control nodes

---

## Manual Upgrade Steps

Perform these steps **on each additional control plane node**:

### Step 1: Backup Current Configuration

```bash
sudo cp /etc/rancher/rke2/config.yaml /etc/rancher/rke2/config.yaml.backup
```

### Step 2: Add TLS SAN Configuration

Copy the `tls-san` block from the first node's `/etc/rancher/rke2/config.yaml` and append it to the additional control node:

```bash
sudo tee -a /etc/rancher/rke2/config.yaml > /dev/null <<EOF

# BEGIN ANSIBLE MANAGED BLOCK - tls-san
tls-san:
  - k8s.your-domain.com
# END ANSIBLE MANAGED BLOCK - tls-san
EOF
```

Replace `your-domain.com` with your actual domain. Include any additional TLS SANs configured on the first node.

### Step 3: Add OIDC Authentication Configuration

**3a.** Create the auth directory:
```bash
sudo mkdir -p /etc/rancher/rke2/auth
```

**3b.** Copy `/etc/rancher/rke2/auth/auth-config.yaml` from the first node to this control node (use scp, or copy/paste the content).

**3c.** Add the authentication config reference to `/etc/rancher/rke2/config.yaml`:
```bash
sudo tee -a /etc/rancher/rke2/config.yaml > /dev/null <<EOF

# BEGIN ANSIBLE MANAGED BLOCK - authentication-config
kube-apiserver-arg:
  - "--authentication-config=/etc/rancher/rke2/auth/auth-config.yaml"
# END ANSIBLE MANAGED BLOCK - authentication-config
EOF
```

### Step 4: Restart RKE2 Service

```bash
sudo systemctl restart rke2-server.service
sudo systemctl status rke2-server.service
```

### Step 5: Verify

```bash
# Check kube-apiserver is running
sudo crictl ps | grep kube-apiserver

# Check logs for errors
sudo journalctl -u rke2-server.service -n 50 --no-pager
```

---

## Rollback

If issues occur:

```bash
sudo cp /etc/rancher/rke2/config.yaml.backup /etc/rancher/rke2/config.yaml
sudo rm -rf /etc/rancher/rke2/auth/
sudo systemctl restart rke2-server.service
```

---

## Summary

**Two configuration changes required:**
1. **TLS SAN block** - Enables proper TLS certificate verification
2. **OIDC auth config** - Enables OIDC authentication on kube-apiserver

**Note:** First node already has these configurations. Worker nodes don't need this update (they don't run kube-apiserver). New control nodes deployed with the updated code get this automatically.

