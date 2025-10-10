# Mock Harness Examples

## Side-by-Side Comparison: Real vs Mock Steps

### Example 1: Simple Validation Step

#### Real Step (from pkg/steps.go)
```go
var CheckUbuntuStep = Step{
    Id:          "CheckUbuntuStep",
    Name:        "Check Ubuntu Version",
    Description: "Verify running on supported Ubuntu version",
    Action: func() StepResult {
        if !IsRunningOnSupportedUbuntu() {
            return StepResult{
                Error: fmt.Errorf("this tool requires Ubuntu with one of these versions: %s",
                    strings.Join(SupportedUbuntuVersions, ", ")),
            }
        }
        return StepResult{Error: nil}
    },
}
```

#### Mock Step (from pkg/mocksteps.go)
```go
var MockCheckUbuntuStep = Step{
    Id:          "CheckUbuntuStep",
    Name:        "Check Ubuntu Version",
    Description: "Verify running on supported Ubuntu version (MOCK)",
    Action: func() StepResult {
        time.Sleep(100 * time.Millisecond)
        LogMessage(Info, "Mock: Ubuntu 22.04 LTS detected")
        return StepResult{Error: nil}
    },
}
```

**Key Differences:**
- Mock adds "(MOCK)" to description
- Mock always returns success
- Mock adds simulated delay
- Mock logs what would have been detected
- Mock doesn't check actual system

---

### Example 2: Step with Variables

#### Real Step (from pkg/steps.go)
```go
var SetupRKE2Step = Step{
    Id:          "SetupRKE2Step",
    Name:        "Setup RKE2",
    Description: "Setup RKE2 server and configure necessary modules",
    Action: func() StepResult {
        var err error
        if viper.GetBool("FIRST_NODE") {
            err = SetupFirstRKE2()
        } else if viper.GetBool("CONTROL_PLANE") {
            err = SetupRKE2ControlPlane()
        } else {
            err = SetupRKE2Additional()
        }
        if err != nil {
            return StepResult{Error: err}
        }
        return StepResult{Error: nil}
    },
}
```

#### Mock Step (from pkg/mocksteps.go)
```go
var MockSetupRKE2Step = Step{
    Id:          "SetupRKE2Step",
    Name:        "Setup RKE2",
    Description: "Setup RKE2 server and configure necessary modules (MOCK)",
    Action: func() StepResult {
        time.Sleep(1200 * time.Millisecond)
        if viper.GetBool("FIRST_NODE") {
            LogMessage(Info, "Mock: RKE2 v1.28.3+rke2r1 server setup complete")
            LogMessage(Info, "Mock: Generated cluster token: K10abc123xyz...")
            viper.Set("join_token", "K10abc123xyz456def789ghi012jkl345::server:mock-token-data")
        } else if viper.GetBool("CONTROL_PLANE") {
            LogMessage(Info, "Mock: RKE2 control plane joined to cluster")
        } else {
            LogMessage(Info, "Mock: RKE2 worker node joined to cluster")
        }
        return StepResult{Error: nil}
    },
}
```

**Key Differences:**
- Mock preserves conditional logic (FIRST_NODE check)
- Mock generates realistic variables (join_token)
- Mock logs what would have been installed
- Mock doesn't actually install RKE2
- Mock sets timing to simulate long operation (1.2s)

---

### Example 3: Step with Skip Logic

#### Real Step (from pkg/steps.go)
```go
var SetupAndCheckRocmStep = Step{
    Id:          "SetupAndCheckRocmStep",
    Name:        "Setup and Check ROCm",
    Description: "Verify, setup, and check ROCm devices",
    Skip: func() bool {
        if !viper.GetBool("GPU_NODE") {
            LogMessage(Info, "Skipping ROCm setup for non-GPU node")
            return true
        }
        return false
    },
    Action: func() StepResult {
        if !CheckAndInstallROCM() {
            return StepResult{
                Error: fmt.Errorf("setup of ROCm failed"),
            }
        }
        cmd := exec.Command("sh", "-c", `rocm-smi -i --json | jq -r '.[] | .["Device Name"]' | sort | uniq -c`)
        output, err := cmd.CombinedOutput()
        if err != nil {
            LogMessage(Error, "Failed to execute rocm-smi: "+err.Error())
            return StepResult{
                Error: fmt.Errorf("failed to execute rocm-smi: %w", err),
            }
        }
        // ... validation logic ...
        LogMessage(Info, "ROCm Devices:\n"+string(output))
        return StepResult{Error: nil}
    },
}
```

#### Mock Step (from pkg/mocksteps.go)
```go
var MockSetupAndCheckRocmStep = Step{
    Id:          "SetupAndCheckRocmStep",
    Name:        "Setup and Check ROCm",
    Description: "Verify, setup, and check ROCm devices (MOCK)",
    Skip: func() bool {
        if !viper.GetBool("GPU_NODE") {
            LogMessage(Info, "Skipping ROCm setup for non-GPU node")
            return true
        }
        return false
    },
    Action: func() StepResult {
        time.Sleep(800 * time.Millisecond)
        LogMessage(Info, "Mock: ROCm 6.0.2 installed successfully")
        LogMessage(Info, "Mock: Detected GPUs:\n      8   AMD Instinct MI250X")
        viper.Set("gpu_count", 8)
        viper.Set("gpu_model", "AMD Instinct MI250X")
        return StepResult{Error: nil}
    },
}
```

**Key Differences:**
- Mock preserves EXACT same skip logic
- Mock generates GPU detection output
- Mock sets variables for GPU count and model
- Mock doesn't run rocm-smi command
- Mock logs formatted GPU info

---

### Example 4: Step with User Interaction

#### Real Step (from pkg/steps.go)
```go
var SelectDrivesStep = Step{
    Id:          "SelectDrivesStep",
    Name:        "Select Unmounted Disks",
    Description: "Identify and select unmounted physical disks",
    Action: func() StepResult {
        if viper.IsSet("SELECTED_DISKS") && viper.GetString("SELECTED_DISKS") != "" {
            disks := strings.Split(viper.GetString("SELECTED_DISKS"), ",")
            LogMessage(Info, fmt.Sprintf("Selected disks: %v", disks))
            // ... unmount logic ...
            viper.Set("selected_disks", disks)
            return StepResult{Error: nil}
        }
        disks, err := GetUnmountedPhysicalDisks()
        if err != nil {
            return StepResult{
                Error: fmt.Errorf("failed to get unmounted disks: %v", err),
            }
        }
        // ... show selection screen ...
        result, err := ShowOptionsScreen(
            "Unmounted Disks",
            "Select disks to format and mount\n\n"+diskinfo+"\n\n...",
            options,
            options,
        )
        // ... handle selection ...
        viper.Set("selected_disks", result.Selected)
        return StepResult{Message: fmt.Sprintf("Selected disks: %v", result.Selected)}
    },
}
```

#### Mock Step (from pkg/mocksteps.go)
```go
var MockSelectDrivesStep = Step{
    Id:          "SelectDrivesStep",
    Name:        "Select Unmounted Disks",
    Description: "Identify and select unmounted physical disks (MOCK)",
    Action: func() StepResult {
        time.Sleep(400 * time.Millisecond)

        // If SELECTED_DISKS is already set, use it
        if viper.IsSet("SELECTED_DISKS") && viper.GetString("SELECTED_DISKS") != "" {
            LogMessage(Info, "Mock: Using pre-configured disk selection")
            viper.Set("selected_disks", []string{"/dev/nvme0n1", "/dev/nvme1n1"})
            return StepResult{Error: nil}
        }

        // Simulate disk selection
        mockDisks := []string{"/dev/nvme0n1", "/dev/nvme1n1", "/dev/nvme2n1", "/dev/nvme3n1"}
        LogMessage(Info, "Mock: Found unmounted disks: /dev/nvme0n1, /dev/nvme1n1, /dev/nvme2n1, /dev/nvme3n1")
        LogMessage(Info, "Mock: Auto-selected disks for testing: /dev/nvme0n1, /dev/nvme1n1")

        viper.Set("selected_disks", mockDisks[:2])
        return StepResult{Message: "Selected disks: /dev/nvme0n1, /dev/nvme1n1"}
    },
}
```

**Key Differences:**
- Mock doesn't show interactive selection screen
- Mock auto-selects reasonable defaults
- Mock preserves pre-configuration logic
- Mock logs what disks were "found"
- Mock sets same variables as real step

---

## Running Examples

### Example 1: First Node GPU Installation

**Configuration (mock-config.yaml):**
```yaml
FIRST_NODE: true
GPU_NODE: true
DOMAIN: "ai-cluster.local"
USE_CERT_MANAGER: false
CERT_OPTION: "generate"
METALLB_IP_RANGE: "192.168.1.240-192.168.1.250"
```

**Command:**
```bash
sudo ./bloom mock --config mock-config.yaml
```

**Expected Log Output:**
```
Mock: Configuration validated successfully
Mock: System requirements validated (32GB RAM, 8 CPUs, 500GB disk)
Mock: Ubuntu 22.04 LTS detected
Mock: Installed packages: jq, nfs-common, open-iscsi, chrony
...
Mock: ROCm 6.0.2 installed successfully
Mock: Detected GPUs:
      8   AMD Instinct MI250X
Mock: RKE2 v1.28.3+rke2r1 server setup complete
Mock: Generated cluster token: K10abc123xyz...
Mock: Created MetalLB IPAddressPool with range: 192.168.1.240-192.168.1.250
Mock: Generated self-signed TLS certificate for ai-cluster.local
Mock: Generated join token for additional nodes
```

**Variables Set:**
```
join_token: "K10abc123xyz456def789ghi012jkl345::server:mock-token-data"
server_ip: "10.0.100.50"
gpu_count: 8
gpu_model: "AMD Instinct MI250X"
selected_disks: ["/dev/nvme0n1", "/dev/nvme1n1"]
mounted_disks: ["/mnt/disk1", "/mnt/disk2"]
```

---

### Example 2: Additional CPU Worker

**Configuration:**
```yaml
FIRST_NODE: false
GPU_NODE: false
JOIN_TOKEN: "K10abc123xyz456def789ghi012jkl345::server:mock-token-data"
SERVER_IP: "10.0.100.50"
SKIP_DISK_CHECK: true
```

**Command:**
```bash
sudo ./bloom mock --config mock-config.yaml
```

**Expected Log Output:**
```
Mock: Configuration validated successfully
Mock: System requirements validated (32GB RAM, 8 CPUs, 500GB disk)
Skipping ROCm setup for non-GPU node
Skipping drive mounting as SKIP_DISK_CHECK is set.
Mock: RKE2 worker node joined to cluster
Mock: Longhorn drive setup instructions available in longhorn_drive_setup.txt
```

**Steps Skipped:**
- SetupAndCheckRocmStep (GPU_NODE: false)
- SelectDrivesStep (SKIP_DISK_CHECK: true)
- MountSelectedDrivesStep (SKIP_DISK_CHECK: true)
- SetupMetallbStep (FIRST_NODE: false)
- SetupLonghornStep (SKIP_DISK_CHECK: true)
- CreateDomainConfigStep (FIRST_NODE: false)
- SetupClusterForgeStep (FIRST_NODE: false)

---

### Example 3: Testing Specific Steps

**Configuration:**
```yaml
FIRST_NODE: true
GPU_NODE: true
ENABLED_STEPS: "ValidateArgsStep,CheckUbuntuStep,SetupAndCheckRocmStep,MockFinalOutput"
```

**Command:**
```bash
sudo ./bloom mock --config mock-config.yaml
```

**Expected Behavior:**
- Only runs 4 steps
- Perfect for testing specific functionality
- Skips all other steps

---

## Advanced Mock Patterns

### Pattern 1: Testing Error Conditions

You can modify mock steps to return errors:

```go
var MockFailingStep = Step{
    Id:          "FailingStep",
    Name:        "Failing Step",
    Description: "This step will fail (MOCK)",
    Action: func() StepResult {
        time.Sleep(200 * time.Millisecond)
        return StepResult{
            Error: fmt.Errorf("mock error: simulated failure"),
        }
    },
}
```

### Pattern 2: Testing Different Scenarios

Create multiple mock config files:

```bash
mock-config-gpu-first.yaml      # First GPU node
mock-config-cpu-first.yaml      # First CPU node
mock-config-gpu-worker.yaml     # Additional GPU worker
mock-config-cpu-worker.yaml     # Additional CPU worker
mock-config-control-plane.yaml  # Additional control plane
```

### Pattern 3: Variable Inspection

After mock run, inspect variables:

```go
// In FinalOutput step or after mock completes
fmt.Printf("Join Token: %s\n", viper.GetString("join_token"))
fmt.Printf("Server IP: %s\n", viper.GetString("server_ip"))
fmt.Printf("GPU Count: %d\n", viper.GetInt("gpu_count"))
fmt.Printf("Selected Disks: %v\n", viper.GetStringSlice("selected_disks"))
```

---

## Benefits Demonstrated

1. **Fast Iteration**: Mock runs in ~30 seconds vs 10-30 minutes
2. **No Hardware**: Test GPU features without actual GPUs
3. **Safe**: No system changes, no risk
4. **Realistic**: Variables and logs match real installation
5. **Complete**: All steps implemented, including edge cases
6. **Flexible**: Easy to test different scenarios
7. **Debuggable**: Add logging anywhere in mock steps

---

## When to Use Each

### Use Mock For:
- UI development and testing
- Final output format verification
- Configuration validation testing
- Workflow logic testing
- Integration testing without hardware
- CI/CD pipeline testing
- Quick demos

### Use Real For:
- Actual deployments
- Hardware compatibility testing
- Performance validation
- Integration with real hardware
- Production environments
- Final acceptance testing

---

## Tips

1. **Keep mock data realistic**: Use actual version numbers, realistic IPs
2. **Preserve skip logic**: Mock should respect same conditions as real
3. **Set same variables**: Mock should set all variables real steps would
4. **Match timing**: Longer operations get longer sleeps
5. **Log clearly**: Prefix logs with "Mock:" to distinguish
6. **Test both paths**: Test both first node and additional node scenarios
7. **Update together**: When real steps change, update mock steps too
