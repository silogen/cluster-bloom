# Bloom V2 Implementation Plan

**Issue:** #609 - Bloom V2
**Priority:** P0 (Critical)
**Repository:** cluster-bloom
**Branch:** bloom-v2
**Date:** 2025-12-08

## Context

Issue #609 requires implementing Bloom V2 with focus on:
- **HIGH PRIORITY:** Web UI for generating bloom.yaml
- **HIGH PRIORITY:** CLI tool for generating bloom.yaml
- **HIGH PRIORITY:** Blooming only with bloom.yaml (simplified workflow)
- **LOW PRIORITY:** Monitoring deployment
- **TESTING:** Robot Framework tests

## Reference Code (For Ideas Only)

### PoC in platform/experiments/bloomv2/ (bloomV2 branch)
Ideas to consider:
- Self-contained Go binary approach
- Linux namespace container runtime concept
- Image caching strategy
- Playbook execution patterns

### Existing Bloom v1 in cluster-bloom
Ideas to consider:
- Deployment step flow
- Configuration validation patterns
- Disk management logic
- Test structure

**IMPORTANT:** Both are reference only - NOT to be ported. This is a clean reimplementation.

## Bloom V2 Goals

### Architecture Shift

**From:** Imperative Go code with embedded shell scripts
**To:** Declarative Ansible playbooks with config generation tools

**Benefits:**
- More maintainable (Ansible best practices)
- Easier to extend (add playbooks vs modify Go code)
- Better idempotency (Ansible modules)
- Separation of concerns (config generation vs execution)

### User Experience Flow

#### Current (v1):
```
User manually writes bloom.yaml → Run bloom binary → Deploy
```

#### Target (v2):
```
Option A: User uses Web UI → Generate bloom.yaml → Run bloom → Deploy
Option B: User uses CLI wizard → Generate bloom.yaml → Run bloom → Deploy
Option C: User manually writes bloom.yaml → Run bloom → Deploy
```

## Implementation Phases

### Phase 1: Core Architecture (Week 1-2)

**Goal:** Design and implement clean V2 architecture from scratch

**Design Decisions:**
1. **Execution model:** How to run deployment tasks?
   - Option A: Embedded Ansible (like PoC)
   - Option B: Pure Go implementation
   - Option C: Hybrid (Go + external tools)

2. **Configuration:** How to handle bloom.yaml?
   - Schema definition
   - Validation rules
   - Variable substitution

3. **Deployment flow:** Step orchestration
   - Sequential vs parallel execution
   - Error handling and rollback
   - Progress reporting

**Tasks:**
1. Define V2 architecture
   - Clean module structure
   - Clear separation of concerns
   - Extensibility for future features

2. Implement config parser
   - YAML schema for bloom.yaml
   - Validation library
   - Type-safe config structures

3. Create minimal deployment engine
   - Basic task execution framework
   - Logging infrastructure
   - Error handling patterns

4. Implement one end-to-end deployment
   - Pick simplest use case (e.g., RKE2 install)
   - Prove architecture works
   - Establish patterns for other components

**Deliverable:** Working minimal bloom v2 that can deploy one component with proper config handling

### Phase 2: CLI Generator (Week 2-3)

**Goal:** CLI tool for interactive bloom.yaml generation

**Tasks:**
1. Create `cmd/bloom-config/` subcommand
   - Interactive prompts for common configs
   - Validation of user inputs
   - Smart defaults based on system detection

2. Question flow:
   - Node role (first node vs additional node)
   - Domain name
   - Certificate options (Let's Encrypt, self-signed, existing)
   - GPU configuration (detect AMD GPUs, ROCm version)
   - Disk selection (list available disks, let user choose)
   - Network settings (if needed)

3. Output generation:
   - Write bloom.yaml to current directory or specified path
   - Show preview before writing
   - Validate generated config

**Example Usage:**
```bash
bloom config init                    # Interactive wizard
bloom config init --quick            # Quick mode with defaults
bloom config validate bloom.yaml    # Validate existing config
bloom config show-defaults           # Show all default values
```

**Deliverable:** CLI wizard that generates valid bloom.yaml files

### Phase 3: Web UI (Week 3-5) - HIGH PRIORITY

**Goal:** Web-based bloom.yaml generator

**Architecture:**
- Embedded web server in bloom binary
- Single-page application (SPA) for config generation
- Static files embedded in Go binary

**Tech Stack:**
- Backend: Go net/http (embedded in bloom)
- Frontend: Vanilla JS or lightweight framework (Preact/Alpine.js)
- No external dependencies at runtime

**Features:**
1. Form-based configuration
   - Step-by-step wizard interface
   - Auto-detection of system capabilities
   - Real-time validation
   - Preview generated YAML

2. Templates
   - Pre-configured scenarios (single node, multi-node, GPU cluster)
   - Load/save/export configurations
   - Import existing bloom.yaml for editing

3. Documentation
   - Inline help for each field
   - Link to full docs
   - Example values

**UI Flow:**
```
1. Node Type → 2. Network → 3. Storage → 4. GPU → 5. Review → 6. Download/Deploy
```

**Example Usage:**
```bash
bloom webui                          # Start web UI on http://localhost:8080
bloom webui --port 9090              # Custom port
bloom webui --no-browser             # Don't auto-open browser
```

**Deliverable:** Web UI accessible at localhost that generates bloom.yaml files

### Phase 4: Testing (Week 4-5) ✅ COMPLETED

**Goal:** Robot Framework test suite

**Test Coverage:**
1. ✅ Schema validation tests
   - Schema-driven validation (all pattern types)
   - Tests ALL valid/invalid examples from schema
   - Automatic field visibility handling

2. ✅ Web UI tests
   - UI loads correctly
   - Form generation from schema
   - Pattern validation through UI
   - Required fields validation

3. ✅ API tests
   - Schema endpoint returns valid JSON
   - Config validation endpoint

4. ✅ Config generation tests
   - Generate valid first node config
   - Generate valid additional node config
   - Generate config with TLS certificates
   - Generate config with advanced options
   - API generate endpoint
   - Invalid config rejection
   - Default values in generated config
   - Field visibility affects generated config

**Implemented Structure:**
```
tests/robot/
├── api.robot                    # API endpoint tests
├── ui.robot                     # UI loading tests
├── validation.robot             # Form validation tests
├── schema_validation.robot      # Schema-driven validation (comprehensive)
├── config_generation.robot      # Config generation tests (NEW)
├── yaml_loader.py              # Helper to extract examples from schema
└── run_tests_docker.sh         # Docker-based test runner
```

**Key Achievement:** Schema-driven testing approach
- Tests automatically stay in sync with schema changes
- All 9 pattern types validated with complete example coverage
- Eliminated test duplication (removed 245 lines of redundant tests)

**Deliverable:** ✅ Automated Robot Framework test suite (COMPLETE)

### Phase 4.5: Schema Refactoring ✅ COMPLETED

**Goal:** Consolidate schema definition to single source of truth

**Implementation:**
1. ✅ Created schema_loader.go
   - Loads schema from schema/bloom.yaml.schema.yaml at runtime
   - Converts YAML schema to Argument structs for API
   - Maps types, dependencies, patterns, and validation messages

2. ✅ Removed hardcoded schema
   - Eliminated 270 lines of hardcoded Go schema definitions
   - Schema.go now only contains Argument struct definition
   - All field definitions driven by YAML schema

3. ✅ Enhanced YAML schema
   - Added comprehensive examples for all 9 pattern types
   - Added error messages for better UX
   - Added section groupings for UI organization
   - Removed empty strings from valid examples (browser validation issue)

4. ✅ Updated frontend
   - Fixed bug where fields with "URL" in name got wrong input type
   - Frontend now uses pattern attribute for validation when available
   - Respects schema-driven field visibility

5. ✅ Schema-driven testing
   - Created yaml_loader.py to extract examples from schema
   - Tests automatically use all schema examples
   - Added visibility step handling (GPU_NODE, FIRST_NODE, CERT_OPTION)
   - Fixed double-click issue for checkbox validation

**Benefits:**
- Schema is the single source of truth (no duplication)
- Adding new fields only requires updating YAML schema
- Tests automatically stay in sync with schema
- Frontend validation driven by schema patterns
- Consistent validation across backend and frontend

**Files:**
- `schema/bloom.yaml.schema.yaml` (enhanced with examples)
- `internal/config/schema_loader.go` (NEW - 192 lines)
- `internal/config/schema_loader_test.go` (NEW - 170 lines)
- `internal/config/schema.go` (reduced from 285 to 16 lines)
- `tests/robot/schema_validation.robot` (NEW - 112 lines)
- `tests/robot/yaml_loader.py` (NEW - 123 lines)
- `tests/robot/validation.robot` (reduced from 266 to 21 lines)

**Deliverable:** ✅ Schema-driven architecture (COMPLETE)

### Phase 5: Refactoring & UI Cleanup (In Progress)

**Goal:** Improve code organization, schema handling, and UI polish

**Schema Handling Tasks:**
1. ⬜ Consolidate validation logic
   - Move all pattern validation to use schema types
   - Eliminate any remaining hardcoded validation
   - Ensure frontend and backend use same patterns

2. ⬜ Schema versioning
   - Add schema version field
   - Plan for future schema migrations
   - Document schema extension process

3. ⬜ Schema documentation
   - Auto-generate field reference from schema
   - Add inline examples to all fields
   - Document pattern syntax and requirements

4. ⬜ Error message improvements
   - Ensure all patterns have clear error messages
   - Add contextual help for common validation failures
   - Improve error display in UI

**UI Cleanup Tasks:**
1. ⬜ Form organization
   - Review section groupings
   - Improve conditional field visibility logic
   - Add field dependency visualization

2. ⬜ Validation feedback
   - Real-time validation indicators
   - Clear error/success states
   - Help text for complex fields

3. ⬜ YAML preview improvements
   - Syntax highlighting
   - Copy to clipboard button
   - Download with timestamp

4. ⬜ Accessibility
   - Keyboard navigation
   - Screen reader support
   - Focus management

5. ⬜ Responsive design
   - Mobile-friendly layout
   - Touch-friendly controls
   - Adaptive form sections

**Code Organization Tasks:**
1. ⬜ Extract reusable components
   - Form field generators
   - Validation utilities
   - Schema parsers

2. ⬜ Consistent error handling
   - Standardize error types
   - Improve error propagation
   - Add error context

3. ⬜ Code documentation
   - Add godoc comments
   - Document public APIs
   - Add usage examples

4. ⬜ Test coverage improvements
   - Add unit tests for schema loader
   - Test edge cases in validation
   - Add integration tests

**Deliverable:** Polished, maintainable codebase with excellent UX

### Phase 6: Documentation & Polish (Week 5-6)

**Tasks:**
1. Update README with v2 usage
2. Migration guide from v1 to v2
3. API documentation for playbook variables
4. Video/GIF demos of Web UI
5. Performance optimization
6. Error message improvements

**Deliverable:** Production-ready Bloom V2

## File Structure (Target)

```
cluster-bloom/
├── cmd/
│   ├── bloom/
│   │   └── main.go           # Main bloom binary
│   └── bloom-config/
│       └── main.go           # Config generator CLI
├── pkg/
│   ├── ansible/
│   │   ├── container.go      # Ansible container runtime
│   │   ├── runner.go         # Playbook execution
│   │   └── cache.go          # Image caching
│   ├── config/
│   │   ├── parser.go         # YAML parsing
│   │   ├── validator.go      # Config validation
│   │   └── generator.go      # Config generation logic
│   └── webui/
│       ├── server.go         # Web server
│       ├── handlers.go       # API handlers
│       └── static/           # Embedded web assets
│           ├── index.html
│           ├── app.js
│           └── styles.css
├── playbooks/
│   ├── main.yml              # Orchestration playbook
│   ├── rocm.yml
│   ├── disks.yml
│   ├── rke2.yml
│   ├── longhorn.yml
│   ├── metallb.yml
│   └── cluster-forge.yml
├── tests/
│   ├── robot/                # Robot Framework tests
│   └── e2e/                  # Existing e2e tests
├── tmp/                      # Planning docs (gitignored)
└── dist/
    └── bloom                 # Compiled binary
```

## Key Decisions

### 1. Backward Compatibility

**Question:** Should v2 support v1 deployment logic?

**Decision:** NO - clean break
- V2 is a complete reimplementation
- V1 remains available on main/release branches for existing users
- V2 starts fresh with new design
- Migration guide will help users transition

### 2. Config Format

**Question:** Should we extend/change bloom.yaml format?

**Decision:** Design optimal format for v2
- Learn from v1 config structure
- Design clean, intuitive schema
- Use schema versioning for future changes
- Provide conversion tool for v1 → v2 configs (optional, low priority)

### 3. Web UI Distribution

**Question:** Separate binary or embedded?

**Decision:** Embedded in main bloom binary
- Single binary distribution
- `bloom webui` command launches server
- No separate deployment needed

### 4. Ansible Image

**Question:** Which Ansible container image to use?

**Decision:** Use `willhallonline/ansible:latest` (PoC proven)
- ~500MB download (one-time)
- Cached at `/var/lib/bloom/rootfs`
- Contains full Ansible with common modules

### 5. State Management

**Question:** Track deployment state?

**Decision:** Phase 1 - No state tracking (rely on Ansible idempotency)
- Future: Consider state file for resume capability
- Let Ansible modules handle "already done" checks

## Success Metrics

1. **Functionality:**
   - CLI wizard generates valid configs (100% success rate)
   - Web UI generates valid configs (100% success rate)
   - Bloom deploys clusters successfully (same success rate as v1)

2. **User Experience:**
   - Config generation time: < 5 minutes (CLI/Web UI)
   - First-time deployment: similar to v1 (~10-15 min)
   - Subsequent runs: < 5 minutes (idempotent)

3. **Code Quality:**
   - Robot Framework tests: > 80% coverage
   - All tests passing in CI/CD
   - Documentation complete

4. **Adoption:**
   - Internal team uses v2 for new deployments
   - Migration guide helps v1 → v2 transition

## Risks & Mitigations

### Risk 1: Ansible Learning Curve
**Impact:** Medium
**Mitigation:** Reference PoC, use well-documented modules, start simple

### Risk 2: Binary Size Increase
**Impact:** Low
**Mitigation:** Web UI assets are small, overall binary still < 30MB

### Risk 3: Breaking Changes from V1
**Impact:** Medium (V2 is separate implementation)
**Mitigation:**
- V1 remains on main branch for existing users
- Clear documentation that V2 is new implementation
- Migration guide for transitioning users
- V2 developed on bloom-v2 branch until ready

### Risk 4: Web UI Complexity
**Impact:** Medium
**Mitigation:** Keep UI simple, use lightweight framework, progressive enhancement

## Timeline

| Phase | Duration | Deliverable |
|-------|----------|-------------|
| Phase 1: Foundation | 2 weeks | Working bloom binary with Ansible |
| Phase 2: CLI Generator | 1 week | Interactive config wizard |
| Phase 3: Web UI | 2 weeks | Web-based config generator |
| Phase 4: Testing | 1 week | Robot Framework test suite |
| Phase 5: Documentation | 1 week | Complete docs and polish |
| **Total** | **7 weeks** | **Production-ready Bloom V2** |

## Design Questions to Answer (Phase 1)

Before starting implementation, need to decide:

1. **Execution Engine:**
   - Pure Go vs embedded Ansible vs hybrid?
   - Tradeoffs: complexity, maintainability, capabilities

2. **Config Schema:**
   - Minimal required fields vs comprehensive?
   - How to handle optional components?
   - Validation strategy?

3. **Task Orchestration:**
   - Linear steps vs DAG?
   - Parallel execution?
   - Retry/rollback mechanisms?

4. **Code Organization:**
   - Monorepo vs separate CLI/server?
   - Package structure?
   - Plugin architecture?

## Implementation Status

### ✅ Completed

**Phase 3: Web UI (HIGH PRIORITY)** - COMPLETE
- ✅ Web-based bloom.yaml generator (`bloom webui` command)
- ✅ Schema-driven dynamic form generation from Go backend
- ✅ HTML5 real-time validation with V1 pattern compatibility
- ✅ Conditional field visibility based on dependencies
- ✅ File save to server's cwd with custom filename
- ✅ Minimal YAML output (only non-default values)
- ✅ FIRST_NODE and GPU_NODE always included
- ✅ Port management (auto-discovery from 62078, explicit with --port)
- ✅ Robot Framework tests (10 essential tests, 100% passing)
- ✅ V1/V2 schema parity - all V1 arguments present

**Commits:**
- `eb4d523` feat(webui): add file save with custom filename and minimal YAML output
- `8f5d384` feat(webui): implement schema-driven validation with V1 pattern compatibility
- `3a7b079` feat(webui): add HTML5 field validation with real-time feedback
- `9a3895a` feat(webui): implement smart port management with auto-discovery

**Phase 1: Core Architecture** - COMPLETE ✅
- ✅ Config parser (pkg/config/schema.go - single source of truth)
- ✅ Config validator (pkg/config/validator.go - schema-driven validation)
- ✅ Config generator (Web UI + YAML generation)
- ✅ **Deployment engine** - IMPLEMENTED

### ✅ Completed

**Phase 1b: Ansible Deployment Engine** - COMPLETE ✅

**Design Complete** - All architectural decisions finalized (2025-12-10)

**Architecture:**
- Command: `bloom ansible <config-file>` subcommand
- Runtime: pkg/ansible/runtime package (extracted from bloomv2 experiment)
- Playbooks: Embedded in pkg/ansible/runtime/playbooks/
- Config: Reuses pkg/config package (no conversion needed)
- Filtering: DISABLED_STEPS/ENABLED_STEPS deferred to v2.1

**Implementation Tasks:**
1. ✅ Add go-containerregistry dependency to go.mod
2. ✅ Create pkg/ansible/runtime package (container execution)
   - container.go: Image pulling and caching
   - executor_linux.go: Linux namespace container execution
   - executor_other.go: Stub for non-Linux platforms
   - playbook.go: Playbook execution orchestration
3. ✅ Copy and embed playbooks/ from experiments/bloomv2
   - cluster-bloom.yaml (main deployment playbook)
   - hello.yml (test playbook)
4. ✅ Playbooks already use UPPERCASE variable names
5. ✅ Add ansible subcommand to cmd/main.go
6. ✅ Wire up bloom.yaml reading with pkg/config
   - Config loading with LoadConfig()
   - Validation with Validate()
   - Pass config as Ansible extra vars
7. ⬜ Test basic deployment workflow (NEXT STEP)

### 📋 Not Started

**Phase 2: CLI Generator** - LOW PRIORITY
- Interactive CLI wizard for bloom.yaml generation
- Deprioritized since Web UI is complete and working

**Phase 4: Testing (Deployment)** - BLOCKED
- Waiting for ansible command implementation
- Web UI tests already complete

**Phase 5: Documentation & Polish** - FUTURE
- Update README with v2 usage
- Migration guide from v1 to v2
- Performance optimization

## Next Actions

1. ✅ Create bloom-v2 branch in cluster-bloom
2. ✅ Write planning document
3. ✅ Design and implement Web UI
4. ✅ Implement schema-driven validation
5. ✅ Add Robot Framework tests for Web UI
6. ✅ Implement `bloom ansible` command
   - ✅ Copy pattern from /workspace/platform/experiments/bloomv2
   - ✅ Adapt to use bloom.yaml as input
   - ✅ Embed cluster-bloom.yaml playbook
   - ✅ Add to cluster-bloom repository
7. **→ Test ansible command with generated bloom.yaml files (CURRENT)**
   - Test hello.yml playbook execution
   - Verify config loading and validation
   - Test cluster-bloom.yaml with sample config
8. Complete deployment tests
9. Documentation and polish

## Outstanding Design Questions

### 1. ✅ Execution Engine - ANSWERED
**Decision:** Embedded Ansible (from platform/experiments/bloomv2)
- Proven pattern in PoC
- Self-contained binary
- No external dependencies
- Uses willhallonline/ansible:latest image

### 2. ✅ Config Schema - ANSWERED
**Decision:** Comprehensive schema matching V1
- 26 arguments across 6 sections
- Pattern validation for all fields
- Conditional field visibility
- Type-safe with proper defaults

### 3. ✅ Web UI Distribution - ANSWERED
**Decision:** Embedded in main bloom binary
- `bloom webui` command
- Static assets embedded via go:embed
- No separate deployment

### 4. ✅ Task Orchestration - ANSWERED
**Decision:** Use existing cluster-bloom.yaml playbook
- Embed entire playbooks/ directory from experiments/bloomv2
- Update playbook to use UPPERCASE variable names (no conversion needed)
- Step filtering (DISABLED_STEPS/ENABLED_STEPS) deferred to v2.1

### 5. ✅ Command Structure - ANSWERED
**Decision:** Subcommand architecture
- `bloom ansible <config-file>` as subcommand in cmd/bloom/main.go
- Consistent with `bloom webui` pattern
- Single binary distribution

### 6. ✅ Config Reading - ANSWERED
**Decision:** Reuse existing internal/config package
- Parse bloom.yaml using internal/config
- Validate with existing validators
- Pass Config map directly as Ansible extra vars (-e KEY=value)
- No conversion logic needed

### 7. ✅ Runtime Architecture - ANSWERED
**Decision:** Extract into pkg/ansible/runtime package
- Clean separation from command logic
- Reusable container runtime: image pulling, layer extraction, namespace creation
- Better testing isolation
- More maintainable structure

## Current Blockers

**None** - Path forward is clear:
1. Implement `bloom ansible` command using bloomv2 experiment pattern
2. Existing playbooks in platform/experiments/bloomv2/playbooks/cluster-bloom.yaml
3. Integration with bloom.yaml schema already defined

## Updated Timeline

| Phase | Status | Actual Duration | Notes |
|-------|--------|-----------------|-------|
| Phase 1: Foundation | 🔄 Partial | 2 weeks | Config done, need deployment |
| Phase 2: CLI Generator | ⏸️ Skipped | - | Web UI supersedes this |
| Phase 3: Web UI | ✅ Complete | 3 weeks | Done with full validation |
| **Phase 1b: Ansible Command** | 📋 Next | ~1 week | Copy pattern from bloomv2 |
| Phase 4: Testing | 📋 Pending | ~1 week | After ansible command |
| Phase 5: Documentation | 📋 Pending | ~3 days | Final polish |
| **Remaining** | - | **~2-3 weeks** | **To production** |

---

**Status:** Phase 3 (Web UI) complete. Next: Implement ansible deployment engine.
**Last Updated:** 2025-12-10
**Branch:** bloom-v2
**Issue:** #609 (Open, In Progress)
