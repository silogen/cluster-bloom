# InstallDependentPackagesStep Integration Tests

## Purpose
Install essential system packages required for Kubernetes cluster operation and Bloom functionality.

## Step Overview
- **Execution Order**: Step 5
- **Commands Executed**:
  - `apt-get install -y open-iscsi`
  - `apt-get install -y jq`
  - `apt-get install -y nfs-common`
  - `apt-get install -y chrony`
  - `sudo snap install kubectl --classic`
  - `sudo snap install k9s`
  - `sudo snap install helm --classic`
  - `sudo snap install yq`
- **Skip Conditions**: Never skipped

## What This Step Does
1. Installs required apt packages:
   - **open-iscsi**: iSCSI initiator for Longhorn storage
   - **jq**: JSON processor for scripts and configuration
   - **nfs-common**: NFS client for network storage
   - **chrony**: Time synchronization daemon
2. Installs Kubernetes tools via snap:
   - **kubectl**: Kubernetes command-line tool
   - **k9s**: Terminal UI for Kubernetes
   - **helm**: Kubernetes package manager
   - **yq**: YAML processor
3. Fails if any package installation fails

## Test Scenarios

### Success Scenarios

#### 1. Fresh Install - All Packages Installed
Tests successful installation of all packages on clean system.

#### 2. Packages Already Installed - apt
Tests that apt handles already-installed packages gracefully (open-iscsi, jq, nfs-common, chrony).

#### 3. Snap Tools Already Installed
Tests that snap handles already-installed tools gracefully.

#### 4. Partial Installation - Some Packages Exist
Tests installation when some packages already exist.

#### 5. All Tools Already Installed
Tests when all apt packages and snap tools are already present.

### Failure Scenarios

#### 6. open-iscsi Installation Fails
Tests error handling when open-iscsi apt install fails.

#### 7. jq Installation Fails
Tests error handling when jq apt install fails.

#### 8. nfs-common Installation Fails
Tests error handling when nfs-common apt install fails.

#### 9. chrony Installation Fails
Tests error handling when chrony apt install fails.

#### 10. kubectl Snap Install Fails
Tests error handling when kubectl snap install fails.

#### 11. k9s Snap Install Fails
Tests error handling when k9s snap install fails.

#### 12. helm Snap Install Fails
Tests error handling when helm snap install fails.

#### 13. yq Snap Install Fails
Tests error handling when yq snap install fails.

#### 14. Network Unavailable - apt Update Needed
Tests handling when network is down and packages can't be fetched.

## Configuration Requirements

- `ENABLED_STEPS: "InstallDependentPackagesStep"`
- No specific configuration required
- Requires internet connection for package downloads

## Mock Requirements

```yaml
mocks:
  # Scenario 1: Fresh install
  "InstallPackage.AptInstallopen-iscsi":
    output: |
      Reading package lists...
      Building dependency tree...
      The following NEW packages will be installed:
        open-iscsi
      Setting up open-iscsi (2.1.8-1ubuntu1) ...
    error: null

  "InstallPackage.AptInstalljq":
    output: |
      Reading package lists...
      The following NEW packages will be installed:
        jq
      Setting up jq (1.6-2.1ubuntu3) ...
    error: null

  "InstallPackage.AptInstallnfs-common":
    output: |
      Reading package lists...
      The following NEW packages will be installed:
        nfs-common
      Setting up nfs-common (1:2.6.1-1ubuntu1) ...
    error: null

  "InstallPackage.AptInstallchrony":
    output: |
      Reading package lists...
      The following NEW packages will be installed:
        chrony
      Setting up chrony (4.2-2ubuntu2) ...
    error: null

  "InstallK8sTools.SnapInstall":
    output: |
      kubectl 1.28.4 from Canonical✓ installed
    error: null

  "InstallK8sTools.SnapInstall":
    output: |
      k9s 0.27.4 from Fernand Galiana (derailed) installed
    error: null

  "InstallK8sTools.SnapInstall":
    output: |
      helm 3.13.2 from Snapcrafters✓ installed
    error: null

  "InstallK8sTools.SnapInstall":
    output: |
      yq 4.35.2 from Mike Farah (mikefarah) installed
    error: null

  # Scenario 2: Already installed (apt)
  "InstallPackage.AptInstallopen-iscsi":
    output: |
      Reading package lists...
      open-iscsi is already the newest version (2.1.8-1ubuntu1).
    error: null

  # Scenario 3: Already installed (snap)
  "InstallK8sTools.SnapInstall":
    output: |
      snap "kubectl" is already installed
    error: null

  # Scenario 6: open-iscsi fails
  "InstallPackage.AptInstallopen-iscsi":
    output: |
      E: Unable to locate package open-iscsi
    error: "exit status 100"

  # Scenario 10: kubectl fails
  "InstallK8sTools.SnapInstall":
    output: ""
    error: "error: snap not found"
```

## Running Tests

```bash
# Test 1: Fresh install
./cluster-bloom cli --config step_integration_tests/05_InstallDependentPackagesStep/01-fresh-install/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/05_InstallDependentPackagesStep/01-fresh-install/mocks.yaml

# Test 2: Already installed
./cluster-bloom cli --config step_integration_tests/05_InstallDependentPackagesStep/02-already-installed-apt/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/05_InstallDependentPackagesStep/02-already-installed-apt/mocks.yaml

# Test 6: open-iscsi fails
./cluster-bloom cli --config step_integration_tests/05_InstallDependentPackagesStep/06-open-iscsi-fails/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/05_InstallDependentPackagesStep/06-open-iscsi-fails/mocks.yaml
```

## Expected Outcomes

### Success Cases
- ✅ open-iscsi installed
- ✅ jq installed
- ✅ nfs-common installed
- ✅ chrony installed
- ✅ kubectl installed
- ✅ k9s installed
- ✅ helm installed
- ✅ yq installed
- ✅ All packages available in PATH
- ✅ Step completes successfully

### Failure Cases
- ❌ Any apt package installation failure stops execution
- ❌ Any snap tool installation failure stops execution
- ❌ Error message indicates which package failed
- ❌ Network errors stop execution

## Related Code
- Step implementation: `pkg/steps.go:119-131`
- Main function: `pkg/packages.go:InstallDependentPackages()`
- Apt installer: `pkg/packages.go:installpackage()`
- Snap installer: `pkg/packages.go:installK8sTools()`
- Package list: Hardcoded in InstallDependentPackages function

## Notes
- **open-iscsi**: iSCSI initiator daemon
  - Required for Longhorn distributed block storage
  - Provides iSCSI client functionality
  - Communicates with Longhorn iSCSI targets
- **jq**: Command-line JSON processor
  - Used for parsing JSON in Bloom scripts
  - Lightweight and fast
  - Common dependency for cloud-native tooling
- **nfs-common**: NFS client utilities
  - Required for NFS-based persistent volumes
  - Provides mount.nfs and related tools
  - Backup storage option to Longhorn
- **chrony**: NTP time synchronization
  - More accurate than legacy ntpd
  - Critical for distributed systems (etcd, certificates)
  - Time skew can cause cluster issues
- **kubectl**: Kubernetes CLI
  - Version 1.28+ recommended
  - Installed with --classic (full system access)
  - Primary tool for cluster management
- **k9s**: Kubernetes terminal UI
  - User-friendly alternative to kubectl
  - Real-time cluster monitoring
  - Interactive pod/log viewing
- **helm**: Kubernetes package manager
  - Installs applications via "charts"
  - Version 3.x (no Tiller)
  - --classic flag for full functionality
- **yq**: YAML processor (similar to jq for JSON)
  - Used for YAML manipulation in scripts
  - Complements jq for configuration files
- **Installation order**: apt packages first, then snap tools
- **Error handling**: Stops at first failure (sequential installation)
- **Idempotent**: apt and snap handle already-installed packages
- **Network requirement**: Requires internet for package downloads
- **snap advantages**: Auto-updates, dependency management, newer versions
- **apt -y flag**: Non-interactive mode (auto-confirm prompts)
- **sudo usage**: Snap commands run with sudo for system-wide installation
- **Package versions**: System uses default repo versions for apt, snap manages kubectl/helm/yq versions
- **Note**: This step installs kubectl/k9s/helm/yq which are also installed by InstallK8SToolsStep (step 14)
  - Step 5 runs early (pre-K8S setup)
  - Step 14 runs later (just before K8S installation)
  - Appears to be redundant - likely step 5 is older code
