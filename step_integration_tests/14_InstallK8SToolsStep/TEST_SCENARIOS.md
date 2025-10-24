# InstallK8SToolsStep Integration Tests

## Purpose
Install essential Kubernetes management tools (kubectl, k9s, helm, yq) via snap package manager.

## Step Overview
- **Execution Order**: Step 14
- **Commands Executed**:
  - `sudo snap install kubectl --classic`
  - `sudo snap install k9s`
  - `sudo snap install helm --classic`
  - `sudo snap install yq`
- **Skip Conditions**: Never skipped

## What This Step Does
1. Installs kubectl (Kubernetes command-line tool) with --classic confinement
2. Installs k9s (terminal UI for Kubernetes clusters)
3. Installs helm (Kubernetes package manager) with --classic confinement
4. Installs yq (YAML processor)

## Test Scenarios

### Success Scenarios

#### 1. Fresh Install - All Tools Installed
Tests successful installation of all four tools on clean system.

#### 2. Tools Already Installed
Tests that snap handles already-installed packages gracefully (no error).

#### 3. Partial Installation - Some Tools Exist
Tests installation when some tools already exist (snap updates or skips).

### Failure Scenarios

#### 4. kubectl Installation Fails
Tests error handling when kubectl snap install fails.

#### 5. k9s Installation Fails
Tests error handling when k9s snap install fails.

#### 6. helm Installation Fails
Tests error handling when helm snap install fails.

#### 7. yq Installation Fails
Tests error handling when yq snap install fails.

#### 8. Snap Service Not Available
Tests error handling when snapd service is not running.

## Configuration Requirements

- `ENABLED_STEPS: "InstallK8SToolsStep"`
- No specific configuration required
- Requires snapd to be installed and running

## Mock Requirements

```yaml
mocks:
  # Successful installations
  "InstallK8sTools.SnapInstall.kubectl":
    output: |
      kubectl 1.28.4 from Canonical✓ installed
    error: null

  "InstallK8sTools.SnapInstall.k9s":
    output: |
      k9s 0.27.4 from Fernand Galiana (derailed) installed
    error: null

  "InstallK8sTools.SnapInstall.helm":
    output: |
      helm 3.13.2 from Snapcrafters✓ installed
    error: null

  "InstallK8sTools.SnapInstall.yq":
    output: |
      yq 4.35.2 from Mike Farah (mikefarah) installed
    error: null

  # Already installed scenario
  "InstallK8sTools.SnapInstall.kubectl":
    output: |
      snap "kubectl" is already installed
    error: null

  # Failure scenario
  "InstallK8sTools.SnapInstall.kubectl":
    output: ""
    error: "error: snap not found"
```

## Running Tests

```bash
# Test 1: Fresh install
./cluster-bloom cli --config step_integration_tests/14_InstallK8SToolsStep/01-fresh-install/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/14_InstallK8SToolsStep/01-fresh-install/mocks.yaml

# Test 2: Already installed
./cluster-bloom cli --config step_integration_tests/14_InstallK8SToolsStep/02-already-installed/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/14_InstallK8SToolsStep/02-already-installed/mocks.yaml

# Test 4: kubectl fails
./cluster-bloom cli --config step_integration_tests/14_InstallK8SToolsStep/04-kubectl-fails/config.yaml \
                    --dry-run \
                    --dry-run-mocks step_integration_tests/14_InstallK8SToolsStep/04-kubectl-fails/mocks.yaml
```

## Expected Outcomes

### Success Cases
- ✅ kubectl installed with --classic confinement
- ✅ k9s installed
- ✅ helm installed with --classic confinement
- ✅ yq installed
- ✅ Tools available in PATH
- ✅ Step completes successfully

### Failure Cases
- ❌ Any tool installation failure stops execution
- ❌ Error message indicates which tool failed
- ❌ Snap service unavailable stops execution

## Related Code
- Step implementation: `pkg/steps.go:148-161`
- Sequential snap install commands

## Notes
- **kubectl**: Kubernetes CLI for cluster management and resource manipulation
- **k9s**: Terminal-based UI for managing Kubernetes clusters (user-friendly alternative to kubectl)
- **helm**: Package manager for Kubernetes (installs charts/applications)
- **yq**: Command-line YAML processor (similar to jq for JSON)
- **--classic flag**: Required for kubectl and helm to access system resources outside snap confinement
- **snap advantages**: Automatic updates, dependency management, easy installation
- **Installation location**: Tools installed to `/snap/bin/` (automatically in PATH)
- **Version management**: Snap handles version tracking and channels
- **Idempotent**: Snap handles already-installed packages without error
- **Alternative methods**: Could use apt, but snap provides newer versions and better isolation
