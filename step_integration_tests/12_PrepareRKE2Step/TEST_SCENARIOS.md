# PrepareRKE2Step Integration Tests

## Purpose
Prepare the system for RKE2 installation by loading required kernel modules, creating directories, and configuring RKE2 settings including audit policy and optional OIDC authentication.

## Step Overview
- **Execution Order**: Step 12
- **Commands Executed**:
  - `modprobe iscsi_tcp`
  - `modprobe dm_mod`
  - `mkdir -p /etc/rancher/rke2`
  - `chmod 0755 /etc/rancher/rke2`
  - Writes `/etc/rancher/rke2/audit-policy.yaml`
  - Writes `/etc/rancher/rke2/config.yaml`
  - Optional: `openssl s_client -showcerts -connect <OIDC_URL>:443` (if OIDC_URL set)
  - Optional: Writes `/etc/rancher/rke2/oidc-ca.crt` (if OIDC_URL set)
- **Skip Conditions**: Never skipped

## What This Step Does
1. Loads required kernel modules (iscsi_tcp for iSCSI, dm_mod for device mapper)
2. Creates RKE2 configuration directory with proper permissions
3. Writes audit policy YAML for Kubernetes API server logging
4. Generates RKE2 configuration file (config.yaml)
5. If OIDC_URL configured:
   - Fetches OIDC provider's TLS certificate
   - Saves certificate to oidc-ca.crt
   - Adds OIDC configuration to config.yaml

## Test Scenarios

### Success Scenarios

#### 1. Basic Setup Without OIDC
Tests successful preparation with standard configuration (no OIDC).

#### 2. Setup With OIDC URL - Certificate Fetched
Tests successful OIDC certificate retrieval and configuration.

#### 3. Modules Already Loaded
Tests that modprobe handles already-loaded modules gracefully.

#### 4. Directory Already Exists
Tests idempotent directory creation.

### Failure Scenarios

#### 5. iscsi_tcp Module Load Fails
Tests error handling when iscsi_tcp module cannot be loaded.

#### 6. dm_mod Module Load Fails
Tests error handling when dm_mod module cannot be loaded.

#### 7. mkdir Command Fails
Tests error handling when directory creation fails.

#### 8. chmod Command Fails
Tests error handling when permission setting fails.

#### 9. Failed to Write audit-policy.yaml
Tests error handling when audit policy file cannot be written.

#### 10. Failed to Write config.yaml
Tests error handling when RKE2 config file cannot be written.

#### 11. OIDC Certificate Fetch Fails
Tests error handling when openssl cannot retrieve OIDC certificate.

#### 12. Failed to Write oidc-ca.crt
Tests error handling when OIDC certificate file cannot be written.

## Configuration Requirements

- `ENABLED_STEPS: "PrepareRKE2Step"`
- Optional: `OIDC_URL` (e.g., "https://sso.example.com")

## Mock Requirements

```yaml
mocks:
  # Basic setup
  "SetupFirstRKE2.modprobe.iscsi_tcp":
    output: ""
    error: null

  "SetupFirstRKE2.modprobe.dm_mod":
    output: ""
    error: null

  "SetupFirstRKE2.mkdir":
    output: ""
    error: null

  "SetupFirstRKE2.chmod":
    output: ""
    error: null

  "WriteFile./etc/rancher/rke2/audit-policy.yaml":
    output: ""
    error: null

  "WriteFile./etc/rancher/rke2/config.yaml":
    output: ""
    error: null

  # OIDC setup (if OIDC_URL set)
  "FetchAndSaveOIDCCertificate.OpensslClient":
    output: |
      -----BEGIN CERTIFICATE-----
      MIIDXTCCAkWgAwIBAgIJAKZ...
      -----END CERTIFICATE-----
    error: null

  "WriteFile./etc/rancher/rke2/oidc-ca.crt":
    output: ""
    error: null

  # Failure scenarios
  "SetupFirstRKE2.modprobe.iscsi_tcp":
    output: ""
    error: "modprobe: FATAL: Module iscsi_tcp not found"
```

## Running Tests

```bash
# Test 1: Basic setup
./cluster-bloom cli --config step_integration_tests/12_PrepareRKE2Step/01-basic-setup/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/12_PrepareRKE2Step/01-basic-setup/mocks.yaml

# Test 2: With OIDC
./cluster-bloom cli --config step_integration_tests/12_PrepareRKE2Step/02-with-oidc/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/12_PrepareRKE2Step/02-with-oidc/mocks.yaml

# Test 5: iscsi_tcp fails
./cluster-bloom cli --config step_integration_tests/12_PrepareRKE2Step/05-iscsi-fails/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/12_PrepareRKE2Step/05-iscsi-fails/mocks.yaml
```

## Expected Outcomes

### Success Cases
- ✅ Kernel modules loaded (iscsi_tcp, dm_mod)
- ✅ Directory `/etc/rancher/rke2` created with 0755 permissions
- ✅ Audit policy file created
- ✅ RKE2 config.yaml created
- ✅ OIDC certificate fetched and saved (if configured)
- ✅ Step completes successfully

### Failure Cases
- ❌ Module load failure stops execution
- ❌ Directory creation failure stops execution
- ❌ Permission setting failure stops execution
- ❌ File write failures stop execution
- ❌ OIDC certificate fetch failure stops execution

## Related Code
- Step implementation: `pkg/steps.go:473-485`
- Audit policy template: Embedded in step code
- RKE2 config template: Generated based on configuration
- OIDC certificate fetch: Uses `openssl s_client -showcerts`

## Notes
- **iscsi_tcp module**: Required for Longhorn iSCSI storage backend
- **dm_mod module**: Required for device mapper (LVM, encryption)
- **Audit policy**: Configures Kubernetes API server audit logging
- **OIDC integration**: Allows SSO authentication via external provider
- **Certificate extraction**: openssl retrieves server cert chain
- **Config.yaml contents**: Sets RKE2 server options, CNI, audit policy path
- **Directory permissions**: 0755 allows read/execute for all, write for owner
- **Idempotent**: Safe to run multiple times (modprobe, mkdir handle existing)
