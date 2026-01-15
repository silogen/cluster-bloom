# Product Requirements Document: ClusterBloom V2

**Version:** 2.0
**Status:** In Development
**Issue:** #609 - Bloom V2
**Branch:** bloom-v2
**Last Updated:** 2025-12-10

## Executive Summary

ClusterBloom V2 is a complete reimagination of the Kubernetes cluster deployment tool, transitioning from imperative Go code to a declarative Ansible-based architecture while adding a modern web-based configuration generator. V2 maintains all V1 capabilities for AMD GPU environments while dramatically improving maintainability, extensibility, and user experience.

## Product Overview

### Purpose
ClusterBloom V2 automates Kubernetes cluster deployment with AMD GPU support through:
- **Web-based configuration generator** - No more manual YAML editing
- **Declarative Ansible playbooks** - More maintainable than shell scripts
- **Self-contained binary** - No external dependencies (Docker, Python, or pre-installed Ansible)
- **Separation of concerns** - Config generation decoupled from deployment

### Target Users
- DevOps Engineers managing AMD GPU workloads
- Platform Teams deploying Kubernetes infrastructure
- Organizations requiring automated cluster provisioning with AMD GPU support
- Teams needing reliable storage configuration with Longhorn
- **NEW:** Users preferring web interfaces over CLI/YAML editing

### What's New in V2

**Architecture Changes:**
- ✅ Web UI for configuration generation (no manual YAML editing required)
- ✅ Schema-driven validation (V1 pattern compatibility)
- ✅ Ansible playbooks instead of Go shell execution
- ✅ Self-contained binary with embedded Ansible runtime
- ✅ Minimal YAML output (only non-default values)

**User Experience Improvements:**
- ✅ Real-time form validation in browser
- ✅ Conditional field visibility (smart forms)
- ✅ Custom filename support for generated configs
- ✅ File saved to server's working directory
- ✅ Port auto-discovery (no conflicts)

**Developer Experience:**
- ✅ Single source of truth for configuration schema
- ✅ Easier to extend (add playbooks vs modify Go code)
- ✅ Better idempotency (Ansible modules)
- ✅ Clean separation: generate config → deploy with Ansible

## Core Features

### 1. Web-Based Configuration Generator ⭐ NEW

Browser-based configuration wizard that generates valid `bloom.yaml` files without manual editing.

**Features:**
- Schema-driven dynamic form generation
- Real-time HTML5 validation with custom error messages
- Conditional field visibility based on dependencies
- 6 organized sections: Basic, Node, Storage, SSL/TLS, Advanced, CLI Options
- Preview generated YAML before saving
- Save with custom filename to server's current directory
- Port management (auto-discovery from 62078 or explicit with `--port`)

**Technical Implementation:**
- Backend: Go with embedded static assets (`go:embed`)
- Frontend: Vanilla JavaScript (no external dependencies)
- Validation: HTML5 patterns matching V1 validators
- Schema: Single source of truth in `internal/config/schema.go`

**User Flow:**
```
1. Run: bloom webui
2. Open browser to http://localhost:62080
3. Fill configuration form with real-time validation
4. Click "Generate bloom.yaml"
5. Preview YAML output
6. Save with custom filename
7. Use saved bloom.yaml with deployment command
```

**Current Status:** ✅ COMPLETE

**[📄 Implementation Details](./BLOOM_V2_PLAN.md#phase-3-web-ui)**

### 2. Ansible-Based Deployment Engine 🔄 IN PROGRESS

Self-contained Go binary that runs embedded Ansible playbooks without requiring Docker, Python, or pre-installed Ansible.

**Features:**
- Embedded Ansible runtime using Linux namespaces
- Containerized Ansible image cached locally (~500MB one-time download)
- Host filesystem mounted at `/host` inside container
- Reads `bloom.yaml` and passes as Ansible variables (UPPERCASE, no conversion)
- Embedded playbooks from experiments/bloomv2
- Step filtering deferred to v2.1

**Technical Implementation:**
- Command: `bloom ansible <config-file>` subcommand
- Runtime: `pkg/ansible/runtime` package (extracted from bloomv2 experiment)
- Config: Reuses `internal/config` package for parsing/validation
- Container image: `willhallonline/ansible:latest`
- Image library: `go-containerregistry` for pulling/caching
- Isolation: Linux namespaces (UTS, PID, Mount)
- Cache location: `/var/lib/bloom/rootfs`
- Logs: `/var/log/bloom/run-*.log`

**Architecture:**
```
pkg/ansible/
├── runtime/
│   ├── container.go     # Image pull/cache
│   ├── executor.go      # Namespace creation & execution
│   └── playbook.go      # Playbook running logic
└── playbooks/           # Embedded via go:embed
    └── cluster-bloom.yaml (UPPERCASE vars)
```

**User Flow:**
```
1. Generate bloom.yaml via Web UI or manually
2. Run: sudo bloom ansible bloom.yaml
3. First run: Downloads Ansible image (~500MB)
4. Subsequent runs: Uses cached image
5. Executes cluster-bloom.yaml playbook
6. Cluster deployed and ready
```

**Current Status:** 🎯 DESIGN COMPLETE (2025-12-10) - Ready for implementation

**[📄 Reference Implementation](https://github.com/silogen/platform/tree/bloomV2/experiments/bloomv2)**

### 3. Automated RKE2 Kubernetes Deployment

Same as V1 - automated deployment of production-ready RKE2 clusters.

**V2 Changes:**
- Implemented via Ansible playbook instead of Go code
- Ansible tasks in `playbooks/cluster-bloom.yaml`
- Idempotent by default (Ansible module behavior)

**Status:** ✅ Playbook exists, needs integration

### 4. AMD GPU Support with ROCm

Same as V1 - automated AMD GPU driver installation and configuration.

**V2 Changes:**
- ROCm installation via Ansible apt module
- Device detection via Ansible facts
- Permission configuration via Ansible file module

**Status:** ✅ Playbook exists, needs integration

### 5. Storage Management with Longhorn

Same as V1 - distributed block storage with automatic disk detection.

**V2 Changes:**
- Disk preparation via Ansible mount/filesystem modules
- Longhorn deployment via Ansible kubernetes modules
- Better error handling with Ansible's built-in retries

**Status:** ✅ Playbook exists, needs integration

### 6. Network Configuration

Same as V1 - MetalLB load balancing, firewall configuration, multipath.

**V2 Changes:**
- Firewall rules via Ansible ufw/firewalld modules
- MetalLB config via Ansible template module
- Chrony setup via Ansible service module

**Status:** ✅ Playbook exists, needs integration

### 7. Configuration Management

**V2 Improvements:**
- ✅ Web UI for guided configuration (PRIMARY METHOD)
- ✅ Schema-driven validation (single source of truth)
- ✅ Real-time validation in browser
- ✅ Minimal YAML output (only non-default values)
- ✅ V1 pattern compatibility (all validators match)

**Configuration Sources (Priority Order):**
1. Web UI generated YAML (recommended)
2. Manually written YAML
3. Environment variables (via `.env` file)
4. CLI flags (for Ansible execution)

**Status:** ✅ COMPLETE

### 8. TLS Certificate Management

Same as V1 - three options (cert-manager, existing, self-signed).

**V2 Changes:**
- Certificate deployment via Ansible k8s module
- Cert-manager installation via Ansible helm module
- Certificate validation via Ansible openssl module

**Status:** ✅ Playbook exists, needs integration

### 9. Validation and Testing

**Pre-deployment Validation:**
- ✅ Web UI: Real-time form validation
- ✅ Backend: Schema validation before Ansible execution
- 📋 Ansible: System requirements check tasks

**Testing Framework:**
- ✅ Robot Framework tests for Web UI (10 tests, 100% passing)
- 📋 Robot Framework tests for Ansible deployment (pending)
- 📋 E2E tests for full workflow (pending)

**Status:** ✅ Web UI tested, deployment tests pending

## Technical Architecture

### V2 Architecture Shift

**From (V1):**
```
User → Manual YAML → bloom binary → Go code + shell scripts → Deployed cluster
```

**To (V2):**
```
User → Web UI → bloom.yaml → bloom ansible → Ansible playbooks → Deployed cluster
        ↓
    Validation
```

### Component Organization

```
cluster-bloom/
├── cmd/
│   ├── bloom/                    # Main binary
│   │   └── main.go              # Entry point with webui command
│   └── ansible/                  # Ansible command (NEW)
│       └── main.go              # Embedded Ansible runtime
├── pkg/
│   ├── config/
│   │   ├── schema.go            # ✅ Single source of truth
│   │   ├── validator.go         # ✅ Field validators (V1 compat)
│   │   ├── generator.go         # ✅ YAML generation
│   │   └── types.go             # ✅ Type definitions
│   ├── webui/
│   │   ├── server.go            # ✅ Web server
│   │   ├── handlers.go          # ✅ API endpoints
│   │   └── static/              # ✅ Embedded web assets
│   └── ansible/                  # 📋 Ansible runtime (TODO)
│       ├── container.go         # Container runtime
│       ├── runner.go            # Playbook execution
│       └── cache.go             # Image caching
├── cmd/bloom/web/
│   └── static/                   # ✅ Web UI assets
│       ├── index.html
│       ├── js/
│       │   ├── app.js
│       │   ├── form.js
│       │   ├── validator.js
│       │   └── schema.js
│       └── css/styles.css
├── playbooks/                    # 📋 Ansible playbooks (TODO)
│   └── cluster-bloom.yaml       # Main orchestration playbook
├── internal/config/              # ✅ Configuration handling
├── tests/robot/                  # ✅ Robot Framework tests
│   ├── api.robot                # API tests
│   ├── ui.robot                 # UI tests
│   └── validation.robot         # Validation tests
└── dist/
    └── bloom-v2                  # Compiled binary
```

### API Endpoints

**Web UI Backend:**
- `GET /` - Serve Web UI
- `GET /api/schema` - Return configuration schema
- `POST /api/validate` - Validate configuration
- `POST /api/generate` - Generate YAML preview
- `POST /api/save` - Save YAML to file

**Status:** ✅ All implemented

### Data Flow

**Configuration Generation:**
```
Browser → /api/schema → Schema JSON
Browser Form → /api/validate → Validation errors
Browser Form → /api/save → bloom.yaml file
```

**Deployment Execution:**
```
bloom.yaml → Ansible vars → Embedded playbook → Deployed cluster
```

## User Experience

### Installation Workflows

#### Web UI Configuration (Recommended)
```bash
# Start Web UI
bloom webui

# Or with custom port
bloom webui --port 9090

# Browser opens to http://localhost:62080
# Fill form, generate bloom.yaml
# Click "Save bloom.yaml"
```

#### Deploy with Generated Config
```bash
# After generating bloom.yaml via Web UI
sudo bloom ansible bloom.yaml

# First run downloads Ansible image (~500MB)
# Subsequent runs use cached image
```

#### First Node Setup
```bash
# 1. Generate config via Web UI (FIRST_NODE=true)
bloom webui

# 2. Deploy cluster
sudo bloom ansible bloom.yaml
```

#### Additional Node Setup
```bash
# 1. Generate config via Web UI (FIRST_NODE=false)
#    Provide SERVER_IP and JOIN_TOKEN from first node
bloom webui

# 2. Join cluster
sudo bloom ansible bloom.yaml
```

#### Manual Configuration (Advanced)
```bash
# Create bloom.yaml manually
cat > bloom.yaml <<EOF
FIRST_NODE: true
GPU_NODE: true
DOMAIN: cluster.example.com
CERT_OPTION: generate
EOF

# Deploy
sudo bloom ansible bloom.yaml
```

#### Custom Playbooks
```bash
# Use external playbook
sudo bloom ansible -playbook /path/to/custom.yml -var "domain=example.com"

# From URL
sudo bloom ansible -playbook https://example.com/playbook.yml
```

### System Requirements

Same as V1:
- **Disk Space**: 20GB+ root, 500GB+ /var/lib/rancher
- **System Resources**: 4GB+ RAM (8GB recommended), 2+ CPU cores
- **Ubuntu Version**: 20.04, 22.04, or 24.04
- **Kernel Modules**: overlay, br_netfilter (amdgpu for GPU nodes)

**Additional V2 Requirements:**
- Root access (for Linux namespaces)
- Internet access (first run to download Ansible image)

### Error Handling and Recovery

**Web UI:**
- Real-time validation prevents invalid configs
- Clear error messages for pattern mismatches
- Field-level validation with custom messages

**Ansible Execution:**
- Ansible's built-in idempotency (safe to retry)
- Structured error output
- Task-level error messages
- Logs saved to `/var/log/bloom/run-*.log`

## Current Status

### ✅ Completed (Phase 3)

**Web UI Configuration Generator:**
- Schema-driven dynamic forms
- Real-time HTML5 validation
- Conditional field visibility
- File save with custom filename
- Minimal YAML output
- Port management
- Robot Framework tests (100% passing)

**Commits:**
- `eb4d523` File save with custom filename and minimal YAML
- `8f5d384` Schema-driven validation with V1 pattern compatibility
- `3a7b079` HTML5 field validation with real-time feedback
- `9a3895a` Smart port management with auto-discovery

### 🔄 In Progress (Phase 1b)

**Ansible Deployment Engine:**
- ✅ Architecture design complete (2025-12-10)
- ✅ All design decisions documented
- 📋 Implementation pending

**Design Decisions:**
1. ✅ Command structure: `bloom ansible` subcommand
2. ✅ Config reading: Reuse internal/config package
3. ✅ Variable mapping: No conversion, UPPERCASE in playbook
4. ✅ Playbook embedding: Entire playbooks/ directory
5. ✅ Runtime architecture: pkg/ansible/runtime package
6. ✅ Step filtering: Deferred to v2.1

**Implementation Tasks:**
1. Add go-containerregistry dependency to go.mod
2. Create pkg/ansible/runtime package (container execution)
3. Copy and embed playbooks/ from experiments/bloomv2
4. Update cluster-bloom.yaml to use UPPERCASE variable names
5. Add ansible subcommand to cmd/bloom/main.go
6. Wire up bloom.yaml reading with internal/config
7. Test basic deployment workflow

**References:**
- Architecture: `tmp/ANSIBLE_ARCHITECTURE.md`
- Experiment: `/workspace/platform/experiments/bloomv2/`

### 📋 Not Started

**Phase 2: CLI Generator** - DEPRIORITIZED
- Web UI supersedes this
- Low priority

**Phase 4: Deployment Testing**
- Blocked on Ansible command
- Will use Robot Framework

**Phase 5: Documentation**
- README updates
- Migration guide
- API documentation

## Integration Capabilities

Same as V1:
- **1Password Connect**: Secure secrets management
- **ClusterForge**: Automated application deployment
- **OIDC Providers**: Authentication integration
- **Helm Charts**: Application deployment
- **Kubectl Access**: Automated kubeconfig setup

**V2 Additions:**
- External Ansible playbooks via `-playbook` flag
- Custom variables via `-var` flags
- Environment file support (`.env`)

## Testing and Quality Assurance

### Web UI Testing (Complete)
- ✅ 10 Robot Framework tests
- ✅ API endpoint testing (schema, validate, generate, save)
- ✅ UI functionality (form rendering, field visibility)
- ✅ Validation testing (pattern matching, error messages)
- ✅ 100% test success rate

### Deployment Testing (Pending)
- 📋 Single node deployment
- 📋 Multi-node cluster
- 📋 GPU node configuration
- 📋 Idempotency testing
- 📋 Error recovery

## Success Metrics

### Primary Metrics
- **Configuration Success Rate**: Target 100% valid YAML from Web UI ✅ ACHIEVED
- **Validation Accuracy**: Target 100% pattern match with V1 ✅ ACHIEVED
- **Web UI Usability**: Target <5 minutes to generate config ✅ ACHIEVED
- **Installation Success Rate**: Target >95% (pending ansible command)

### Secondary Metrics
- **Binary Size**: Target <50MB (TBD after Ansible embedding)
- **First Run Time**: Target <5 minutes (image download)
- **Subsequent Run Time**: Target <30 minutes (cluster deployment)
- **Test Coverage**: Web UI 100% ✅, Deployment 0% 📋

## Known Limitations

### V2 Specific
1. **Ansible Command Not Implemented**: Deployment engine pending
2. **No CLI Wizard**: Only Web UI for config generation (acceptable trade-off)
3. **Requires Root**: Ansible runtime needs root for namespaces
4. **First Run Download**: ~500MB Ansible image (one-time, cached)

### Inherited from V1
1. **No Backup/Recovery**: Same as V1
2. **No Built-in Monitoring**: Same as V1
3. **Ubuntu Only**: Same as V1
4. **No HA Automation**: Same as V1

## Future Roadmap

### Immediate (Next 2-3 Weeks)
1. **Implement Ansible Command**: Top priority
2. **Deployment Testing**: Robot Framework tests
3. **Documentation**: README, migration guide
4. **Release V2.0**: Production-ready

### Near-term (3-6 Months)
1. **CLI Config Generator**: If Web UI proves insufficient
2. **Enhanced Validation**: Pre-flight system checks via Ansible
3. **Monitoring Integration**: Optional Prometheus/Grafana playbook
4. **Backup Playbooks**: Automated backup via Ansible

### Medium-term (6-12 Months)
1. **Multi-OS Support**: Ansible playbooks for CentOS/RHEL
2. **Cloud Playbooks**: AWS/Azure/GCP specific tasks
3. **HA Playbooks**: Automated HA configuration
4. **Scaling Playbooks**: Automated cluster scaling

## Migration from V1

### Compatibility
- ✅ Config schema matches V1 (all arguments present)
- ✅ Validation patterns match V1 exactly
- ✅ Same deployment outcomes expected

### Migration Path
1. **Generate new config**: Use V2 Web UI instead of editing YAML
2. **Deploy with Ansible**: Use `bloom ansible` instead of `bloom`
3. **Same clusters**: V2 deploys identical clusters to V1

### Breaking Changes
- Command changed: `bloom` → `bloom ansible bloom.yaml`
- Requires root: Ansible runtime needs root access
- First run slower: One-time Ansible image download

## Conclusion

ClusterBloom V2 represents a significant architectural improvement while maintaining full compatibility with V1 cluster deployments. The Web UI dramatically improves user experience for configuration generation, while the Ansible-based deployment engine provides better maintainability and extensibility for future enhancements.

**Key Achievements:**
- ✅ Web UI eliminates manual YAML editing
- ✅ Schema-driven validation ensures correctness
- ✅ V1 pattern compatibility maintained
- ✅ Comprehensive testing framework

**Remaining Work:**
- 🔄 Ansible deployment engine (2-3 weeks)
- 📋 Deployment testing (~1 week)
- 📋 Documentation (~3 days)

**Timeline to Production:** ~2-3 weeks

---

**Status:** Phase 3 (Web UI) complete. Phase 1b (Ansible) in progress.
**Last Updated:** 2025-12-10
**Branch:** bloom-v2
**Issue:** #609 (Open, In Progress)
