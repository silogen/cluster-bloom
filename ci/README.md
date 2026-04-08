# Bloom CI - Dagger Pipeline

This directory contains a Dagger-based CI/CD pipeline for building and testing the Bloom cluster deployment tool.

## Overview

The Bloom CI pipeline provides:
- **Build**: Compile the bloom binary from source
- **Test**: Run unit tests
- **QEMU Validation**: Test bloom installation in isolated QEMU VMs
- **Export**: Build and export the binary to the dist/ directory

## Prerequisites

1. **Dagger** (v0.20.3 or later)
   ```bash
   # Install dagger
   curl -fsSL https://dl.dagger.io/dagger/install.sh | sh
   ```

2. **Go** (1.23 or later) - for local development
   ```bash
   go version
   ```

3. **QEMU** (optional, for full validation)
   - Only required if running QEMU validation tests
   - Tests can be skipped with `--skip-qemu=true` (default)

## Setup

Initialize the Dagger module (first time only):

```bash
cd cluster-bloom/ci
dagger develop
```

This will:
- Generate the Dagger SDK code in `internal/dagger/`
- Download Go dependencies
- Create `go.sum` file

## Usage

### List Available Functions

```bash
dagger functions
```

### Build the Bloom Binary

```bash
dagger call build export --path=../dist/bloom
```

### Run Unit Tests

```bash
dagger call test
```

### Run Full CI Pipeline

Run build + tests (skips QEMU by default):

```bash
dagger call all
```

Run build + tests + QEMU validation:

```bash
dagger call all --skip-qemu=false
```

### QEMU Validation Only

Test bloom installation in a QEMU VM:

```bash
dagger call validate-in-qemu \
  --profile=../tests/qemu/profile_2_nvme.yaml \
  --config=../tests/qemu/bloom.yaml
```

Or use the CI test configuration:

```bash
dagger call validate-in-qemu \
  --profile=../tests/qemu/profile_2_nvme.yaml \
  --config=ci/test-bloom.yaml
```

### Export Binary

Build and export the bloom binary:

```bash
dagger call export-binary --output-path=../dist
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: CI

on: [push, pull_request]

jobs:
  build-and-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Install Dagger
        run: |
          curl -fsSL https://dl.dagger.io/dagger/install.sh | sh
          sudo mv ./bin/dagger /usr/local/bin/
      
      - name: Run CI Pipeline
        working-directory: ci
        run: dagger call all
      
      - name: Export Binary
        working-directory: ci
        run: dagger call export-binary --output-path=../dist
      
      - name: Upload Artifact
        uses: actions/upload-artifact@v4
        with:
          name: bloom-binary
          path: dist/bloom
```

### With QEMU Validation

For environments with KVM/nested virtualization support:

```yaml
- name: Run Full Validation
  working-directory: ci
  run: dagger call all --skip-qemu=false
```

## Architecture

```
ci/
├── dagger.json          # Dagger module configuration
├── go.mod              # Go module definition
├── main.go             # Dagger pipeline functions
├── test-bloom.yaml     # CI test configuration
└── README.md           # This file

Generated (after dagger develop):
├── go.sum              # Go dependencies checksum
├── dagger.gen.go       # Generated Dagger boilerplate
└── internal/
    └── dagger/         # Generated Dagger SDK
```

## Pipeline Functions

### `Build()`
Compiles the bloom binary from source using Go 1.24.

**Returns**: Dagger File containing the bloom binary

### `Test()`
Runs unit tests in `pkg/` directory.

**Returns**: Test output as string

### `ValidateInQemu()`
Runs the full QEMU test suite using existing test scripts.

**Parameters**:
- `profile` (optional): VM hardware profile (default: `tests/qemu/profile_2_nvme.yaml`)
- `config` (optional): Bloom configuration (default: `tests/qemu/bloom.yaml`)

**Returns**: QEMU test output

**Note**: Requires Ubuntu 24.04 with QEMU, typically runs in Dagger container.

### `All()`
Runs the complete CI pipeline: test + build + optional QEMU validation.

**Parameters**:
- `skip-qemu` (optional): Skip QEMU tests (default: `true`)

**Returns**: Combined pipeline output

### `ExportBinary()`
Builds and exports the bloom binary to the host filesystem.

**Parameters**:
- `output-path` (optional): Export directory (default: `../dist`)

**Returns**: Success message with path

## Troubleshooting

### Dagger Connection Issues

If you see connection errors:

```bash
# Check Dagger engine status
dagger version

# Restart Dagger engine
dagger engine stop
dagger version  # Auto-starts engine
```

### Go Module Download Timeouts

If `dagger develop` fails with timeout errors:

```bash
# Try with direct proxy
GOPROXY=direct dagger develop

# Or use a different mirror
GOPROXY=https://goproxy.io,direct dagger develop
```

### QEMU Tests Failing

QEMU validation requires:
- KVM/nested virtualization support
- Ubuntu 24.04 base image
- ~2GB RAM per test VM

To skip QEMU tests:
```bash
dagger call all --skip-qemu=true
```

## Local Development

To modify the pipeline:

1. Edit `main.go` with your changes
2. Run `dagger develop` to regenerate SDK code
3. Test your changes:
   ```bash
   dagger call <your-function>
   ```

## Additional Resources

- [Dagger Documentation](https://docs.dagger.io)
- [Bloom Repository](https://github.com/silogen/cluster-bloom)
- [QEMU Test Documentation](../tests/qemu/README.md)
