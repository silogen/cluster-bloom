# Go Project Style Guide - Cluster-Bloom Template

This file contains coding conventions, project structure, and patterns based on the cluster-bloom project for use in future Go applications.

## Project Structure

```
project-root/
├── main.go                 # Simple main entry point
├── go.mod                  # Go module definition with semantic versioning
├── go.sum                  # Dependency lock file
├── devbox.json            # Development environment setup (optional)
├── cmd/                   # CLI command definitions
│   ├── root.go           # Root command with cobra setup
│   ├── demo.go           # Subcommand example
│   └── *.go              # Additional commands
├── pkg/                   # Core application logic
│   ├── steps.go          # Main business logic steps
│   ├── view.go           # UI/TUI components
│   ├── *.go              # Feature-specific modules
│   ├── manifests/        # Embedded YAML/config files
│   └── templates/        # Template files
└── dist/                  # Build artifacts directory
```

## Code Style Conventions

### File Headers
Every Go file should start with this copyright header:
```go
/**
 * Copyright 2025 [Company Name]. All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
**/
```

### Import Organization
Group imports in this order:
1. Standard library imports
2. Third-party imports
3. Local project imports

```go
import (
    "fmt"
    "os"
    "path/filepath"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    log "github.com/sirupsen/logrus"

    "github.com/yourorg/yourproject/pkg"
)
```

### Main Entry Point Pattern
Keep main.go minimal - delegate to cmd package:
```go
package main

import (
    "github.com/yourorg/yourproject/cmd"
)

func main() {
    cmd.Execute()
}
```

## CLI Architecture (Cobra)

### Root Command Structure
- Use `cmd/root.go` for the main command setup
- Include comprehensive help text with configuration variables
- Set up persistent flags and subcommands in `init()`
- Use Viper for configuration management

### Command Pattern
```go
var myCmd = &cobra.Command{
    Use:   "command-name",
    Short: "Brief description",
    Long:  `Detailed description with usage examples`,
    Run: func(cmd *cobra.Command, args []string) {
        // Command logic here
    },
}

func init() {
    rootCmd.AddCommand(myCmd)
}
```

## Configuration Management

### Viper Configuration Pattern
- Set sensible defaults with `viper.SetDefault()`
- Support environment variables with `viper.AutomaticEnv()`
- Allow config file override with `--config` flag
- Validate required configurations at startup

```go
func initConfig() {
    // Config file handling
    if cfgFile != "" {
        viper.SetConfigFile(cfgFile)
    } else {
        home, _ := os.UserHomeDir()
        viper.AddConfigPath(home)
        viper.SetConfigType("yaml")
        viper.SetConfigName(".appname")
    }

    // Set defaults
    viper.SetDefault("KEY_NAME", defaultValue)
    
    // Enable environment variables
    viper.AutomaticEnv()
    
    // Read config
    viper.ReadInConfig()
    
    // Validate required configs
    validateRequiredConfigs()
}
```

## Logging Standards

### Use Structured Logging
- Use `github.com/sirupsen/logrus` for structured logging
- Create logging functions in pkg for consistency
- Support both file and UI logging

```go
func LogMessage(level LogLevel, message string) {
    switch level {
    case Debug:
        log.Debug(message)
    case Info:
        log.Info(message)
    case Warn:
        log.Warn(message)
    case Error:
        log.Error(message)
    }
}
```

## Business Logic Architecture

### Step-based Execution Pattern
Define operations as discrete steps with consistent interface:

```go
type Step struct {
    Id          string
    Name        string
    Description string
    Action      func() StepResult
}

type StepResult struct {
    Error   error
    Message string
}
```

### Step Definition Pattern
```go
var MyOperationStep = Step{
    Id:          "MyOperationStep",
    Name:        "Operation Name",
    Description: "What this step does",
    Action: func() StepResult {
        err := performOperation()
        if err != nil {
            return StepResult{
                Error: fmt.Errorf("operation failed: %w", err),
            }
        }
        return StepResult{Error: nil}
    },
}
```

### Step Execution with UI Integration
Execute steps with progress tracking and UI integration:
```go
func rootSteps() {
    preK8Ssteps := []pkg.Step{
        pkg.CheckUbuntuStep,
        pkg.InstallDependentPackagesStep,
        // ... more steps
    }
    
    k8Ssteps := []pkg.Step{
        pkg.SetupRKE2Step,
    }
    
    postK8Ssteps := []pkg.Step{
        pkg.SetupLonghornStep,
        pkg.SetupMetallbStep,
        // ... more steps
    }
    
    // Combine and execute all steps with UI
    pkg.RunStepsWithUI(append(append(preK8Ssteps, k8Ssteps...), postK8Ssteps...))
}
```

### Conditional Step Execution
Include conditional logic within steps for different node types:
```go
var SetupLonghornStep = Step{
    Id:          "SetupLonghornStep",
    Name:        "Setup Longhorn manifests",
    Description: "Copy Longhorn YAML files to the RKE2 manifests directory",
    Action: func() StepResult {
        if viper.GetBool("FIRST_NODE") {
            err := setupManifests("longhorn")
            if err != nil {
                return StepResult{Error: err}
            }
        } else {
            return StepResult{Error: nil}
        }
        return StepResult{Error: nil}
    },
}
```

## Error Handling

### Error Wrapping
Always wrap errors with context:
```go
if err != nil {
    return fmt.Errorf("failed to perform operation: %w", err)
}
```

### Error Logging
Log errors before returning them:
```go
if err != nil {
    LogMessage(Error, fmt.Sprintf("Operation failed: %v", err))
    return StepResult{Error: fmt.Errorf("operation failed: %w", err)}
}
```

## Embedded Resources

### Embed Static Files
Use Go 1.16+ embed for static resources:
```go
//go:embed manifests/*.yaml
var manifestFiles embed.FS

//go:embed templates/*.yaml
var templateFiles embed.FS
```

## Kubernetes Integration Patterns

### Manifest Embedding
Use Go 1.16+ embed for Kubernetes manifests with nested directory structure:
```go
//go:embed manifests/*/*.yaml
var manifestFiles embed.FS
```

### Step-based Kubernetes Operations
Define K8s operations as steps with consistent error handling and ID tracking:
```go
var SetupLonghornStep = Step{
    Id:          "SetupLonghornStep",
    Name:        "Setup Longhorn manifests", 
    Description: "Copy Longhorn YAML files to RKE2 manifests directory",
    Action: func() StepResult {
        if viper.GetBool("FIRST_NODE") {
            err := setupManifests("longhorn")
            if err != nil {
                return StepResult{Error: err}
            }
        }
        return StepResult{Error: nil}
    },
}
```

### Manifest Directory Management
Use consistent manifest directory patterns:
```go
var rke2ManifestDirectory = "/var/lib/rancher/rke2/server/manifests"

func setupManifests(manifestType string) error {
    // Implementation for copying embedded manifests to K8s
}
```

## Configuration Validation Patterns

### Required Configuration Checks
Validate required configurations at startup with descriptive errors:
```go
func validateRequiredConfigs() {
    requiredConfigs := []string{"FIRST_NODE", "GPU_NODE"}
    for _, config := range requiredConfigs {
        if !viper.IsSet(config) {
            log.Fatalf("Required configuration item '%s' is not set", config)
        }
    }
}
```

### Conditional Configuration Validation
Implement context-aware configuration validation:
```go
// Additional validation for non-first nodes
if !viper.GetBool("FIRST_NODE") {
    requiredConfigs := []string{"SERVER_IP", "JOIN_TOKEN"}
    for _, config := range requiredConfigs {
        if !viper.IsSet(config) {
            log.Fatalf("Required configuration item '%s' is not set", config)
        }
    }
}
```

### Configuration Length Validation
Validate configuration constraints (e.g., Kubernetes resource limits):
```go
if viper.IsSet("LONGHORN_DISKS") && viper.GetString("LONGHORN_DISKS") != "" {
    longhornDiskString := pkg.ParseLonghornDiskConfig()
    if len(longhornDiskString) > 63 {
        log.Fatalf("Too many disks, %s is longer than 63", longhornDiskString)
    }
}
```

## Secure Logging Practices

### Sensitive Data Redaction
Always redact sensitive information in logs:
```go
func logConfigValues() {
    log.Info("Configuration values:")
    for _, key := range viper.AllKeys() {
        value := viper.Get(key)
        if key == "join_token" || strings.Contains(strings.ToLower(key), "token") {
            value = "---redacted---"
        }
        log.Infof("%s: %v", key, value)
    }
}
```

### File and Console Logging Setup
Configure both file and console logging with proper permissions:
```go
func setupLogging() {
    log.SetFormatter(&log.TextFormatter{
        FullTimestamp: true,
    })

    currentDir, err := os.Getwd()
    if err != nil {
        log.Warnf("Could not determine current directory: %v", err)
        return
    }

    logPath := filepath.Join(currentDir, "appname.log")
    logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        log.Warnf("Could not open log file: %v", err)
        return
    }
    log.SetOutput(logFile)
}
```

### Structured Error Logging
Use consistent error logging with context:
```go
func LogMessage(level LogLevel, message string) {
    switch level {
    case Debug:
        log.Debug(message)
    case Info:
        log.Info(message)
    case Warn:
        log.Warn(message)
    case Error:
        log.Error(message)
    }
}
```

## Dependencies

### Standard Dependencies for CLI Apps
```go
require (
    github.com/spf13/cobra v1.9.1      // CLI framework
    github.com/spf13/viper v1.19.0     // Configuration management
    github.com/sirupsen/logrus v1.9.3  // Structured logging
    github.com/rivo/tview v0.0.0-...   // Terminal UI (if needed)
    github.com/gdamore/tcell/v2 v2.8.1 // Terminal handling (if using TUI)
)
```

## Build Configuration

### Devbox Setup (devbox.json)
```json
{
  "$schema": "https://raw.githubusercontent.com/jetify-com/devbox/0.14.0/.schema/devbox.schema.json",
  "packages": [
    "go@1.24.0",
    "cobra-cli@latest"
  ],
  "shell": {
    "scripts": {
      "build": [
        "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags=\"-s -w\" -o dist/appname"
      ]
    }
  }
}
```

## Function Naming Conventions

- Use PascalCase for exported functions
- Use camelCase for internal functions
- Use descriptive names that indicate the action being performed
- Functions that check conditions should start with "Check", "Has", "Is"
- Functions that perform setup should start with "Setup", "Install", "Configure"

## Variable Naming

- Use descriptive variable names
- Use ALL_CAPS for constants and environment variable names
- Use camelCase for local variables
- Use PascalCase for exported variables

## Comments and Documentation

- Add package-level comments explaining the purpose
- Document exported functions with standard Go doc comments
- Use inline comments sparingly, only for complex logic
- Include usage examples in long descriptions

## TUI (Terminal UI) Patterns

If building interactive terminal applications:
- Use `github.com/rivo/tview` for TUI components
- Implement global UI state management
- Create modal dialogs for user interactions
- Support keyboard navigation and shortcuts

## Security Considerations

- Never log sensitive information (tokens, passwords)
- Redact sensitive config values in logs
- Use proper file permissions for config files (0644)
- Validate and sanitize all user inputs

## Testing Patterns

### Test File Organization
- Place unit tests in `*_test.go` files alongside source code
- Use `integration_test.go` with build tags for integration tests
- Create separate `test/` directory for VM and system tests
- Organize mocks in `mocks_test.go` files

### Unit Testing Strategy
```go
func TestConfigValidation(t *testing.T) {
    tests := []struct {
        name        string
        setup       func()
        expectError bool
    }{
        {
            name: "valid first node config",
            setup: func() {
                viper.Reset()
                viper.Set("FIRST_NODE", true)
                viper.Set("GPU_NODE", true)
            },
            expectError: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tt.setup()
            // Test logic here
        })
    }
}
```

### Mock-based Testing
Use dependency injection and interfaces for mocking system dependencies:
```go
type CommandExecutor interface {
    Execute(name string, args ...string) ([]byte, error)
}

type MockCommandExecutor struct {
    commands []MockCommand
    callLog  []string
}

func (m *MockCommandExecutor) Execute(name string, args ...string) ([]byte, error) {
    // Mock implementation
}
```

### Step Testing Pattern
Test individual steps with mocked dependencies:
```go
func TestStepExecution(t *testing.T) {
    step := Step{
        Id:          "TestStep",
        Name:        "Test Step",
        Description: "A test step",
        Action: func() StepResult {
            return StepResult{Error: nil, Message: "Success"}
        },
    }
    
    result := step.Action()
    assert.NoError(t, result.Error)
    assert.Equal(t, "Success", result.Message)
}
```

### Integration Testing
Use build tags to separate integration tests:
```go
//go:build integration
// +build integration

func TestIntegrationStepExecution(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    
    if os.Getenv("CLUSTER_BLOOM_TEST_ENV") != "true" {
        t.Skip("Skipping integration test")
    }
    
    // Integration test logic
}
```

### VM-based System Testing
For full system testing that requires actual Ubuntu environments:
- Use Vagrant with Ubuntu VMs
- Test complete workflows including disk operations
- Validate multi-node cluster setup
- Test with real hardware constraints

### Test Execution Commands
```bash
# Unit tests only
go test -short ./pkg/...

# All unit tests
go test ./pkg/...

# Integration tests
export CLUSTER_BLOOM_TEST_ENV=true
go test -tags=integration ./pkg/...

# VM-based tests
cd test/vm && ./run-vm-tests.sh
```

### Test Coverage and CI
- Aim for >80% unit test coverage
- Use table-driven tests for multiple scenarios
- Mock external dependencies (exec.Command, file I/O, network)
- Test error conditions explicitly
- Include tests in CI/CD pipeline with multiple Ubuntu versions

## Code Organization Principles

1. **Single Responsibility**: Each file/package should have one clear purpose
2. **Dependency Injection**: Pass dependencies as parameters rather than globals
3. **Interface Segregation**: Define small, focused interfaces
4. **Consistent Error Handling**: Use the same error patterns throughout
5. **Configuration Over Code**: Use config files/environment variables for behavior

## Common Patterns to Follow

- Initialize logging early in application startup
- Validate configuration before starting main logic
- Use channels for async communication
- Implement graceful shutdown handling
- Support both interactive and non-interactive modes
- Provide comprehensive help text and examples