# Configuration Validation System

This document describes the comprehensive validation system implemented in Cluster-Bloom to ensure configuration correctness and system compatibility before any modifications are made.

## Validation Order

The validation system runs in `initConfig()` in the following order:

1. **URL Validation** (`validateAllURLs()`)
2. **IP Address Validation** (`validateAllIPs()`)
3. **Token Validation** (`validateAllTokens()`)
4. **Step Name Validation** (`validateAllStepNames()`)
5. **Configuration Conflicts** (`validateConfigurationConflicts()`)
6. **System Resource Requirements** (`validateResourceRequirements()`)

## Validation Categories

### 1. URL Validation

**Function**: `validateURL()`, `validateAllURLs()`  
**Parameters**: `OIDC_URL`, `CLUSTERFORGE_RELEASE`, `ROCM_BASE_URL`, `RKE2_INSTALLATION_URL`

**Rules**:
- Must use `http://` or `https://` schemes
- Must have a valid host component
- Empty URLs are allowed for optional parameters
- Special case: `CLUSTERFORGE_RELEASE` accepts "none" value

**Error Messages**:
- `"invalid URL format for {param}: {error}"`
- `"invalid URL scheme for {param}: must be http or https, got {scheme}"`
- `"invalid URL for {param}: missing host"`

### 2. IP Address Validation

**Function**: `validateIPAddress()`, `validateAllIPs()`  
**Parameters**: `SERVER_IP` (when `FIRST_NODE=false`)

**Rules**:
- Must be valid IPv4 or IPv6 address
- Rejects loopback addresses (127.0.0.1, ::1)
- Rejects unspecified addresses (0.0.0.0, ::)
- Allows private/internal IPs for cluster networking
- Only validated when `FIRST_NODE=false`

**Error Messages**:
- `"invalid IP address for {param}: {address}"`
- `"loopback IP address not allowed for {param}: {address}"`
- `"unspecified IP address (0.0.0.0 or ::) not allowed for {param}: {address}"`

### 3. Token Validation

**Function**: `validateToken()`, `validateJoinToken()`, `validateOnePasswordToken()`, `validateAllTokens()`  
**Parameters**: `JOIN_TOKEN` (when `FIRST_NODE=false`)

**JOIN_TOKEN Rules**:
- Length: 32-512 characters
- Allowed characters: alphanumeric, +, /, =, _, ., :, -
- Supports RKE2/K3s token formats

**Error Messages**:
- `"JOIN_TOKEN is too short (minimum 32 characters), got {length} characters"`
- `"JOIN_TOKEN contains invalid characters (only alphanumeric, +, /, =, _, ., :, - allowed)"`

### 4. Step Name Validation

**Function**: `validateStepNames()`, `validateAllStepNames()`  
**Parameters**: `DISABLED_STEPS`, `ENABLED_STEPS`

**Rules**:
- Step names must match valid step IDs from `validStepIDs` array
- Comma-separated lists are supported
- Empty values and whitespace are handled gracefully

**Valid Step IDs**:
- CheckUbuntuStep, InstallDependentPackagesStep, OpenPortsStep
- InstallK8SToolsStep, SetupAndCheckRocmStep, SetupRKE2Step
- CleanDisksStep, SetupMultipathStep, MountSelectedDrivesStep
- SetupLonghornStep, SetupMetallbStep, SetupClusterForgeStep
- And 15+ additional steps (see `validStepIDs` in code)

**Error Messages**:
- `"invalid step name '{step}' in {param}. Valid step names are: {validSteps}"`

### 5. Configuration Conflicts

**Function**: `validateConfigurationConflicts()`

**Conflict Checks**:

#### FIRST_NODE Dependencies
- When `FIRST_NODE=false`, both `SERVER_IP` and `JOIN_TOKEN` must be provided
- **Error**: `"when FIRST_NODE=false, SERVER_IP must be provided"`
- **Error**: `"when FIRST_NODE=false, JOIN_TOKEN must be provided"`

#### GPU/ROCm Consistency
- When `GPU_NODE=true`, warns if `ROCM_BASE_URL` is empty
- When `GPU_NODE=true`, warns if `SetupAndCheckRocmStep` is disabled
- **Warning**: `"GPU_NODE=true but ROCM_BASE_URL is empty - ROCm installation may fail"`

#### Disk Parameter Consistency
- When `SKIP_DISK_CHECK=true` but disk parameters are set
- When `SKIP_DISK_CHECK=false` but no disk parameters specified
- **Warning**: `"SKIP_DISK_CHECK=true but disk parameters are set - disk operations will be skipped"`
- **Note**: `SELECTED_DISKS` also skips NVME drive availability checks

#### Step Conflicts
- Same step cannot be both enabled and disabled
- **Error**: `"step '{step}' is both enabled and disabled - this is conflicting"`

#### Essential Step Warnings
- Warns when critical steps are disabled
- **Warning**: `"CheckUbuntuStep is disabled - system compatibility may not be verified"`
- **Warning**: `"SetupRKE2Step is disabled - Kubernetes cluster will not be set up"`

### 6. System Resource Requirements

**Function**: `validateResourceRequirements()`, `validateDiskSpace()`, `validateSystemResources()`, `validateUbuntuVersion()`, `validateKernelModules()`

#### Disk Space Requirements
- **Root partition**: Minimum 10GB required, 20GB recommended
- **Available space**: Minimum 10GB required
- **/var partition**: 5GB recommended for container images
- **Error**: `"insufficient disk space: {available}GB available, minimum 10GB required"`

#### System Resources
- **Memory**: Minimum 4GB required, 8GB recommended for Kubernetes
- **CPU**: Minimum 2 cores required, 4 cores recommended
- **Error**: `"insufficient memory: {available}GB available, minimum 4GB required for Kubernetes"`
- **Error**: `"insufficient CPU cores: {count} available, minimum 2 cores required for Kubernetes"`

#### Ubuntu Version Compatibility
- **Supported versions**: 20.04, 22.04, 24.04
- **Warning**: `"Not running on Ubuntu (detected: {os}) - some features may not work as expected"`
- **Warning**: `"Ubuntu version {version} may not be fully supported. Supported versions: {supported}"`

#### Kernel Module Checks
- **Required modules**: overlay, br_netfilter
- **GPU modules**: amdgpu (when GPU_NODE=true)
- **Warning**: `"Kernel module '{module}' is not loaded and may not be available - this could cause issues"`

## Error Handling

### Fatal Errors
Fatal errors cause immediate program termination with `log.Fatalf()`:
- Invalid URLs, IPs, or tokens
- Invalid step names
- Configuration conflicts (missing required parameters, conflicting steps)
- Insufficient system resources (disk, memory, CPU)

### Warnings
Warnings are logged but do not stop execution:
- Suboptimal configurations
- Missing optional components
- System compatibility issues
- Kernel module availability

## Testing

### Unit Tests
Each validation function has dedicated unit tests:
- `TestValidateURL()` - URL format validation
- `TestValidateIPAddress()` - IP address validation  
- `TestValidateJoinToken()` - JOIN_TOKEN format validation
- `TestValidateStepNames()` - Step name validation
- `TestValidateConfigurationConflicts()` - Conflict detection
- `TestValidateResourceRequirements()` - System resource checks

### Integration Tests
`TestValidationIntegration()` tests complete configuration scenarios:
- Valid first node configuration
- Valid additional node configuration
- Invalid URL configuration
- Missing required parameters
- Invalid step names
- Conflicting step configuration
- Invalid token format

## Usage Examples

### Valid First Node Configuration
```yaml
FIRST_NODE: true
GPU_NODE: true
OIDC_URL: "https://auth.example.com"
CLUSTERFORGE_RELEASE: "https://github.com/example/repo/releases/download/v1.0/release.tar.gz"
ROCM_BASE_URL: "https://repo.radeon.com/amdgpu-install/6.3.2/ubuntu/"
RKE2_INSTALLATION_URL: "https://get.rke2.io"
DISABLED_STEPS: ""
ENABLED_STEPS: ""
```

### Valid Additional Node Configuration
```yaml
FIRST_NODE: false
GPU_NODE: false
SERVER_IP: "192.168.1.100"
JOIN_TOKEN: "K10831EXAMPLE::server:aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789"
CLUSTERFORGE_RELEASE: "none"
DISABLED_STEPS: "SetupAndCheckRocmStep,SetupClusterForgeStep"
```

## Extending Validation

To add new validation rules:

1. Create validation function following the pattern `validate{Category}()`
2. Add function call to `initConfig()` in appropriate order
3. Add comprehensive unit tests
4. Update this documentation
5. Consider error vs. warning classification based on impact