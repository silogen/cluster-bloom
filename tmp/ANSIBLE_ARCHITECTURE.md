# Bloom Ansible Command - Technical Architecture

**Date:** 2025-12-10
**Status:** Design Complete - Ready for Implementation
**Issue:** #609 - Bloom V2

## Overview

The `bloom ansible` command executes Kubernetes cluster deployment using embedded Ansible playbooks. It reads a `bloom.yaml` configuration file, validates it, and runs the deployment in a containerized Ansible environment.

## Design Decisions

### 1. Command Structure
**Decision:** Subcommand in cmd/bloom/main.go
**Rationale:** Consistent with `bloom webui`, maintains single binary distribution

```bash
bloom ansible bloom.yaml
```

### 2. Configuration Reading
**Decision:** Reuse existing internal/config package
**Rationale:** Single source of truth, DRY principle, already validated

- Parse bloom.yaml → internal/config.Config (map[string]any)
- Validate using internal/config.Validate()
- Pass directly to Ansible as extra vars

### 3. Variable Mapping
**Decision:** No conversion - use UPPERCASE in playbook
**Rationale:** Simplest solution, no mapping code needed

- bloom.yaml: `FIRST_NODE: true`
- Playbook: `{{ FIRST_NODE }}`
- Ansible command: `-e FIRST_NODE=true`

### 4. Playbook Embedding
**Decision:** Embed entire playbooks/ directory
**Rationale:** Future-proof, supports modular playbooks, minimal overhead

```go
//go:embed playbooks/*
var embeddedPlaybooks embed.FS
```

### 5. Runtime Architecture
**Decision:** Extract into pkg/ansible/runtime package
**Rationale:** Clean separation, reusable, testable

```
pkg/ansible/
├── runtime/
│   ├── container.go     # Image pull/cache with go-containerregistry
│   ├── executor.go      # Linux namespace creation & execution
│   └── playbook.go      # Playbook running logic
└── playbooks/           # Embedded via go:embed
    └── cluster-bloom.yaml
```

### 6. Step Filtering
**Decision:** Defer to v2.1
**Rationale:** Get core working first, add features incrementally

DISABLED_STEPS and ENABLED_STEPS will be implemented later using Ansible `--skip-tags` and `--tags`.

## Component Architecture

### File Structure

```
cluster-bloom/
├── cmd/bloom/
│   └── main.go                      # Add ansibleCmd cobra.Command
│
├── pkg/ansible/
│   ├── runtime/
│   │   ├── container.go             # pullImage(), extractLayers(), cacheImage()
│   │   ├── executor.go              # createNamespaces(), mountHost(), runAnsible()
│   │   └── playbook.go              # RunPlaybook(config, playbook)
│   └── playbooks/                   # go:embed directory
│       ├── cluster-bloom.yaml       # Main deployment playbook (UPPERCASE vars)
│       └── hello.yml                # Test playbook
│
├── internal/config/                 # Existing - reuse as-is
│   ├── schema.go                    # Schema with all arguments
│   ├── validator.go                 # Validation logic
│   ├── generator.go                 # YAML generation
│   └── types.go                     # Config map type
│
└── go.mod                           # Add go-containerregistry dependency
```

### Data Flow

```
User runs: bloom ansible bloom.yaml
    ↓
cmd/bloom/main.go (ansibleCmd)
    ↓
internal/config.LoadConfig(bloom.yaml) → Config map
    ↓
internal/config.Validate(config) → []string errors
    ↓
pkg/ansible/runtime.RunPlaybook(config, "cluster-bloom.yaml")
    ↓
    ├─ Pull/cache Ansible image (willhallonline/ansible:latest)
    ├─ Extract playbook from embed.FS
    ├─ Convert Config map → Ansible extra vars (-e KEY=value)
    ├─ Create Linux namespaces (UTS, PID, Mount)
    ├─ Mount host filesystem at /host
    └─ Execute: ansible-playbook -e ... /playbooks/cluster-bloom.yaml
```

## Implementation Pattern (from bloomv2 experiment)

### Container Runtime

**Image Pull & Cache:**
```go
// Uses go-containerregistry/pkg/crane
img, err := crane.Pull("willhallonline/ansible:latest")
layers, err := img.Layers()
for layer := range layers {
    extractLayer(layer, "/var/lib/bloom/rootfs")
}
```

**Caching:**
- Location: `/var/lib/bloom/rootfs`
- First run: ~500MB download
- Subsequent runs: Reuse cached rootfs
- Check: `stat /var/lib/bloom/rootfs/usr`

### Namespace Creation

**Linux Namespaces:**
```go
cmd := exec.Command("/proc/self/exe", "__child__", ...)
cmd.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
}
```

**Host Mount:**
```go
// Inside namespace
syscall.Mount("/", "/mnt/host", "", syscall.MS_BIND|syscall.MS_REC, "")
```

### Ansible Execution

**Command Structure:**
```bash
ansible-playbook \
    -i localhost, \
    -c local \
    -e FIRST_NODE=true \
    -e GPU_NODE=false \
    -e DOMAIN=example.com \
    /playbooks/cluster-bloom.yaml
```

**Variable Passing:**
```go
args := []string{"ansible-playbook", "-i", "localhost,", "-c", "local"}
for key, value := range config {
    args = append(args, "-e", fmt.Sprintf("%s=%v", key, value))
}
args = append(args, "/playbooks/cluster-bloom.yaml")
```

## Playbook Updates Required

### Variable Name Changes

Update `/workspace/platform/experiments/bloomv2/playbooks/cluster-bloom.yaml`:

```yaml
# FROM (lowercase):
vars:
  first_node: true
  gpu_node: true
  domain: ""

# TO (UPPERCASE):
vars:
  FIRST_NODE: true
  GPU_NODE: true
  DOMAIN: ""
```

All variable references in tasks must also change:
```yaml
# FROM:
when: first_node

# TO:
when: FIRST_NODE
```

### Host Filesystem Access

Playbook already uses `host_root: /host` pattern:
```yaml
vars:
  host_root: /host

tasks:
  - name: Example task
    copy:
      src: /local/file
      dest: "{{ host_root }}/etc/config"
```

This works because the runtime mounts host at `/host`.

## User Experience

### Installation Workflow

```bash
# Step 1: Generate configuration
bloom webui
# Fill form, save bloom.yaml to /workspace/cluster

# Step 2: Deploy cluster
cd /workspace/cluster
sudo bloom ansible bloom.yaml

# Output:
# Checking for Ansible image...
# Downloading Ansible image (500MB)... [first run only]
# Image ready.
# Running playbook: cluster-bloom.yaml
# [Ansible output follows...]
# Cluster deployment complete!
```

### Subsequent Runs

```bash
sudo bloom ansible bloom.yaml

# Output:
# Using cached Ansible image.
# Running playbook: cluster-bloom.yaml
# [Ansible output follows...]
```

### Error Handling

**Invalid Config:**
```bash
bloom ansible invalid.yaml

# Output:
# Error validating configuration:
#   - DOMAIN: must match pattern ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]...
#   - CLUSTER_DISKS: must be valid block device paths
```

**Missing Config:**
```bash
bloom ansible missing.yaml

# Output:
# Error: configuration file not found: missing.yaml
```

**Ansible Failure:**
```bash
sudo bloom ansible bloom.yaml

# Output:
# [Ansible task output...]
# TASK [Install RKE2 server] *****
# fatal: [localhost]: FAILED! => {"msg": "Unable to download..."}
#
# Deployment failed. Check logs at: /var/log/bloom/run-20251210-143022.log
```

## Dependencies

### New Dependency
```
github.com/google/go-containerregistry
```

### Existing Dependencies (reuse)
```
github.com/spf13/cobra          # CLI framework
gopkg.in/yaml.v3                # YAML parsing
```

## Testing Strategy

### Unit Tests
- `pkg/ansible/runtime`: Mock container operations
- Config to Ansible vars conversion
- Playbook extraction from embed.FS

### Integration Tests
- Pull real Ansible image
- Run hello.yml playbook
- Verify marker file created

### Robot Framework Tests (Phase 4)
- Single node deployment
- Multi-node cluster
- GPU node configuration
- Idempotency (run twice)

## Security Considerations

### Root Requirement
The command requires root because:
- Linux namespace creation needs CAP_SYS_ADMIN
- Cluster deployment modifies system configuration
- Alternative: Use sudo in docs, check in code

### Image Trust
- Uses official willhallonline/ansible:latest
- Cached at /var/lib/bloom/rootfs
- Future: Add image signature verification

### Host Filesystem Access
- Container has full host access via /host mount
- Necessary for system configuration
- Same security model as V1

## Performance Considerations

### First Run
- Image download: ~500MB, 2-5 minutes depending on bandwidth
- Layer extraction: ~1 minute
- Total first run overhead: 3-6 minutes

### Subsequent Runs
- Image check: <1 second
- No download needed
- Deployment time: Same as V1 (~10-15 minutes)

### Disk Usage
- Ansible image: ~500MB at /var/lib/bloom/rootfs
- Logs: Rotated at /var/log/bloom/

## Future Enhancements (Post-v2.0)

### v2.1 - Step Filtering
```bash
bloom ansible bloom.yaml --skip-tags gpu,longhorn
bloom ansible bloom.yaml --tags rke2
```

Map DISABLED_STEPS/ENABLED_STEPS from schema to Ansible flags.

### v2.2 - Custom Playbooks
```bash
bloom ansible bloom.yaml --playbook custom.yml
bloom ansible bloom.yaml --playbook https://example.com/playbook.yml
```

Already supported by bloomv2 experiment pattern.

### v2.3 - Dry Run
```bash
bloom ansible bloom.yaml --check
```

Pass `--check` to Ansible for dry-run mode.

## Success Criteria

- [ ] `bloom ansible bloom.yaml` runs without errors
- [ ] Validates config before execution
- [ ] Downloads and caches Ansible image
- [ ] Executes cluster-bloom.yaml playbook
- [ ] Logs saved to /var/log/bloom/
- [ ] Deploys identical cluster to V1
- [ ] Binary size < 50MB
- [ ] No external dependencies (Docker, Python, Ansible)

## References

- **Experiment Code:** `/workspace/platform/experiments/bloomv2/`
- **Existing Playbook:** `experiments/bloomv2/playbooks/cluster-bloom.yaml`
- **V1 Steps:** `pkg/steps.go` (26 steps → Ansible tasks)
- **Config Schema:** `internal/config/schema.go`

---

**Status:** Design approved. Ready for implementation.
**Next:** Begin implementation with go.mod dependency addition.
**ETA:** 3-5 days for core implementation, 1-2 days for testing.
