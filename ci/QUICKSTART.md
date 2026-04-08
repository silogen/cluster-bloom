# Bloom CI - Quick Start

## Setup (One Time)

```bash
cd cluster-bloom/ci
dagger develop
```

**Note**: If `dagger develop` times out on network issues, try:
```bash
GOPROXY=https://goproxy.io,direct dagger develop
```

## Common Commands

### Build Only
```bash
dagger call build export --path=../dist/bloom
```

### Test Only  
```bash
dagger call test
```

### Full CI (Build + Test)
```bash
dagger call all
```

### Full CI + QEMU Validation
```bash
dagger call all --skip-qemu=false
```

### QEMU Test with Custom Config
```bash
dagger call validate-in-qemu \
  --profile=../tests/qemu/profile_2_nvme.yaml \
  --config=test-bloom.yaml
```

## What Gets Tested

1. **Unit Tests**: All tests in `pkg/` directory
2. **Build**: Compiles bloom binary with version `ci-build`
3. **QEMU Validation** (optional):
   - Spins up Ubuntu 24.04 VM with QEMU
   - Runs the existing `tests/qemu/manual-qemu-test.sh` script
   - Tests bloom CLI installation end-to-end
   - Uses 2 NVMe drives by default (configurable)

## CI/CD Integration

Copy this into `.github/workflows/ci.yml`:

```yaml
name: Bloom CI

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: dagger/dagger-for-github@v6
      - name: Run Tests
        working-directory: ci
        run: dagger call all
```

## Troubleshooting

**Problem**: `dagger develop` fails  
**Solution**: Check network connectivity, try alternative proxy

**Problem**: QEMU tests timeout  
**Solution**: Use `--skip-qemu=true` or run on hardware with KVM support

**Problem**: Build fails  
**Solution**: Check that parent directory has valid `go.mod` and source files

## Next Steps

See [README.md](README.md) for full documentation.
