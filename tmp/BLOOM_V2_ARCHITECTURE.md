# Bloom V2 Architecture

**Issue:** #609 - Bloom V2
**Branch:** bloom-v2
**Date:** 2025-12-08
**Status:** Design Complete

## Design Decisions

### 1. Execution Engine: Embedded Ansible

**Decision:** Use embedded Ansible container runtime (like PoC)

**Implementation:**
- Embed Ansible container image extraction logic
- Pull `willhallonline/ansible:latest` (~500MB, one-time)
- Cache at `/var/lib/bloom/rootfs`
- Run playbooks in Linux namespaces
- Mount host filesystem at `/host`

**Rationale:**
- Leverage mature Ansible ecosystem
- Idempotency built-in
- Less code to write than pure Go
- Battle-tested modules

### 2. Config Schema: YAML Schema as Single Source of Truth

**Decision:** Use schema/bloom.yaml.schema.yaml as the single source of truth

**Implementation:**
- YAML schema defines all field definitions, patterns, and examples
- Schema loaded at runtime by Go backend (schema_loader.go)
- Frontend tests extract examples directly from schema
- Parse same field names (FIRST_NODE, DOMAIN, etc.)
- Validation rules defined in schema types
- Defaults specified in schema
- Dependencies mapped from schema conditions
- Pass as Ansible extra vars

**Schema Location:** `schema/bloom.yaml.schema.yaml`

**Schema Structure:**
- Type definitions with patterns and examples (domain, ipv4, url, etc.)
- Field mappings with type, default, description, section
- Conditional visibility via `applicable` and `required` fields

**Rationale:**
- Single source of truth eliminates duplication
- Schema drives both validation and testing
- Easy to add new fields or patterns
- Tests automatically stay in sync with schema

**Reference:** See `schema/bloom.yaml.schema.yaml`

### 3. Task Orchestration: Linear, Fail-Fast

**Decision:** Sequential playbook execution, stop on first error

**Implementation:**
- Run playbooks in order:
  1. Validation (ROCm check if GPU_NODE=true)
  2. System prep (packages, firewall)
  3. Disk setup
  4. RKE2 installation
  5. Longhorn deployment
  6. MetalLB setup
  7. ClusterForge (if enabled)
- Exit code != 0 stops execution
- No state tracking
- User re-runs from beginning (Ansible handles idempotency)

**Rationale:**
- Simpler implementation
- Clear failure points
- Ansible makes re-runs safe

### 4. Code Organization: Single Binary, Subcommands

**Decision:** One binary with multiple subcommands

**Binary:** `bloom`

**Subcommands:**
```bash
bloom deploy [config.yaml]    # Deploy cluster (default)
bloom webui [--port 8080]     # Start web UI server
bloom config                  # CLI wizard
bloom validate config.yaml    # Validate config
bloom version                 # Show version
```

**Project Structure:**
```
cluster-bloom/  (bloom-v2 branch)
├── cmd/
│   └── bloom/
│       └── main.go           # Entry point, subcommand routing
├── internal/
│   ├── ansible/
│   │   ├── runtime.go        # Container runtime (namespaces)
│   │   ├── image.go          # Image pull & cache
│   │   └── executor.go       # Playbook execution
│   ├── config/
│   │   ├── loader.go         # YAML config parsing
│   │   ├── validator.go      # Validation rules
│   │   ├── schema.go         # Argument struct definition
│   │   ├── schema_loader.go  # Load schema from YAML
│   │   ├── schema_loader_test.go  # Schema loader tests
│   │   ├── generator.go      # Generate bloom.yaml
│   │   ├── patterns_test.go  # Pattern validation tests
│   │   └── types.go          # Config structs
│   ├── deploy/
│   │   ├── orchestrator.go   # Playbook sequencing
│   │   └── logger.go         # Deployment logging
│   ├── webui/
│   │   ├── server.go         # HTTP server
│   │   ├── handlers.go       # API endpoints
│   │   └── embed.go          # Embedded web assets
│   └── wizard/
│       ├── cli.go            # Interactive prompts
│       └── generator.go      # bloom.yaml generation
├── playbooks/
│   ├── validate.yml          # Pre-flight checks
│   ├── system.yml            # System preparation
│   ├── disks.yml             # Disk configuration
│   ├── rke2.yml              # RKE2 installation
│   ├── longhorn.yml          # Longhorn deployment
│   ├── metallb.yml           # MetalLB setup
│   └── clusterforge.yml      # ClusterForge integration
├── schema/
│   └── bloom.yaml.schema.yaml  # Schema definition (single source of truth)
├── tests/
│   └── robot/
│       ├── api.robot         # API endpoint tests
│       ├── ui.robot          # UI loading tests
│       ├── validation.robot  # Form validation tests
│       ├── schema_validation.robot  # Schema-driven validation tests
│       ├── yaml_loader.py    # Schema example extraction
│       └── run_tests_docker.sh  # Test runner
├── cmd/bloom/web/
│   └── static/
│       ├── index.html        # Main page
│       ├── js/
│       │   ├── app.js        # Application logic
│       │   ├── form.js       # Form generation from schema
│       │   ├── schema.js     # Schema utilities
│       │   └── validator.js  # Frontend validation
│       └── css/
│           └── styles.css    # Styling
├── tmp/                      # Planning docs (gitignored)
└── Makefile                  # Build automation
```

**Rationale:**
- Single binary simplifies distribution
- Subcommands provide clear UX
- Internal packages prevent API exposure

### 5. Web UI Scope: Generator Only

**Decision:** Web UI generates bloom.yaml, does NOT deploy

**Features:**
- Form-based bloom.yaml editor
- Field validation
- Conditional field display (dependencies)
- YAML preview
- Download button
- No deployment capability
- No live progress

**User Flow:**
1. Run `bloom webui` → Opens browser to localhost:8080
2. Fill out form (domain, disks, certificates, etc.)
3. Validate config
4. Preview YAML
5. Click "Download bloom.yaml"
6. Exit web UI
7. Run `bloom deploy bloom.yaml` separately

**API Endpoints:**
```
GET  /                  # Serve web UI
POST /api/validate      # Validate config JSON
POST /api/generate      # Generate YAML from JSON
GET  /api/schema        # Get field definitions & dependencies
```

**Rationale:**
- Simpler implementation (no websockets, no streaming)
- Clear separation: config generation vs deployment
- Web UI is just a nice config editor
- Deployment stays in CLI (where logging/errors work well)

## Architecture Diagram

```
┌─────────────────────────────────────────────────┐
│              bloom (binary)                     │
├─────────────────────────────────────────────────┤
│                                                 │
│  Subcommands:                                   │
│  ┌──────────────┐  ┌──────────────┐            │
│  │ bloom deploy │  │ bloom webui  │            │
│  └──────┬───────┘  └──────┬───────┘            │
│         │                 │                     │
│         v                 v                     │
│  ┌──────────────┐  ┌──────────────┐            │
│  │ Deploy       │  │ Web Server   │            │
│  │ Orchestrator │  │ (Generator)  │            │
│  └──────┬───────┘  └──────────────┘            │
│         │                                       │
│         v                                       │
│  ┌──────────────┐                               │
│  │ Ansible      │                               │
│  │ Runtime      │                               │
│  └──────┬───────┘                               │
│         │                                       │
│         v                                       │
│  ┌──────────────┐                               │
│  │ Embedded     │                               │
│  │ Playbooks    │                               │
│  └──────────────┘                               │
│                                                 │
└─────────────────────────────────────────────────┘
         │
         v
┌─────────────────────────────────────────────────┐
│         Host System (Ubuntu)                    │
│  - /var/lib/bloom/rootfs (Ansible cache)       │
│  - /var/log/bloom/*.log                         │
│  - RKE2, Longhorn, etc. installed               │
└─────────────────────────────────────────────────┘
```

## Data Flow

### Deploy Command
```
bloom.yaml
    ↓
[Config Parser] → Validate
    ↓
[Deploy Orchestrator]
    ↓
[Ansible Runtime] → Pull/Cache Image (if needed)
    ↓
[Run Playbooks] → validate.yml
    ↓              system.yml
    ↓              disks.yml
    ↓              rke2.yml
    ↓              longhorn.yml
    ↓              metallb.yml
    ↓              clusterforge.yml
    ↓
Success/Failure Exit Code
```

### Web UI Flow
```
User Browser
    ↓
[Web UI Form] → Fill fields
    ↓
[POST /api/validate] → Check validity
    ↓
[Preview YAML]
    ↓
[POST /api/generate] → Get bloom.yaml
    ↓
Download bloom.yaml
    ↓
User runs: bloom deploy bloom.yaml
```

## Technology Stack

### Backend
- **Language:** Go 1.21+
- **Config:** `gopkg.in/yaml.v3` for YAML parsing
- **Validation:** Custom validators + `github.com/go-playground/validator`
- **HTTP:** Standard `net/http`
- **CLI:** `github.com/spf13/cobra` for subcommands
- **Ansible Image:** `go-containerregistry` for pulling
- **Namespaces:** `golang.org/x/sys/unix` for Linux syscalls

### Frontend (Web UI)
- **Framework:** Vanilla JavaScript (or Alpine.js if needed)
- **Build:** None required (simple HTML/CSS/JS)
- **Embedding:** `go:embed` for static files

### Ansible
- **Image:** `willhallonline/ansible:latest`
- **Playbook Format:** Standard Ansible YAML
- **Variables:** Passed via `-e key=value`

## Build Process

```bash
# Build web UI (if using build step)
cd web && npm run build

# Build Go binary
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags="-s -w" \
  -o dist/bloom \
  ./cmd/bloom

# Result: Single static binary at dist/bloom
```

## Deployment Example

```bash
# Generate config using Web UI
bloom webui
# Browser opens, fill form, download bloom.yaml

# OR generate using CLI wizard
bloom config > bloom.yaml

# Validate (optional)
bloom validate bloom.yaml

# Deploy
sudo bloom deploy bloom.yaml

# Logs written to:
# /var/log/bloom/deploy-20251208-120000.log
```

## Success Criteria

1. **Functionality:**
   - Web UI generates valid bloom.yaml ✓
   - CLI wizard generates valid bloom.yaml ✓
   - bloom deploy successfully deploys cluster ✓
   - Same success rate as V1 ✓

2. **Compatibility:**
   - V1 bloom.yaml files work in V2 ✓
   - All V1 config options supported ✓

3. **User Experience:**
   - Single binary distribution ✓
   - Clear subcommands ✓
   - Web UI is intuitive ✓
   - Good error messages ✓

4. **Code Quality:**
   - Clean architecture ✓
   - Testable components ✓
   - Robot Framework tests ✓
   - Schema-driven validation ✓
   - Comprehensive test coverage (all patterns) ✓

## Out of Scope (V2.0)

- Resume capability
- Parallel playbook execution
- Web-based deployment monitoring
- State tracking
- Rollback functionality
- Multi-cluster orchestration
- Config versioning/migration

## Future Considerations (V2.x)

- Resume from failure
- Progress bars in CLI
- Web UI with deployment capability
- Diff between configs
- Dry-run mode
- Plugin system

---

**Status:** Architecture finalized, ready for implementation
**Next:** Begin Phase 1 implementation
**Date:** 2025-12-08
