# Technical Architecture

## Overview

ClusterBloom is a single Go binary that deploys RKE2-based Kubernetes clusters by driving Ansible playbooks. The Go layer handles the CLI, configuration loading/validation, and a self-contained Ansible runtime; the playbooks perform the actual node provisioning, cluster bootstrap, and add-on deployment. All playbooks and Kubernetes manifests are embedded into the binary, and the Ansible engine itself ships as a pinned container image (pulled and cached on first run, or embedded for offline builds).

## Command Structure

Commands are defined in `cmd/main.go` (Cobra).

- **`bloom`** (root, no subcommand) / **`bloom webui`** — start the web UI configuration generator (default action). Flag: `--port` (default 62078).
- **`bloom cli <config-file>`** — deploy a cluster from a `bloom.yaml`. Flags: `--tags`, `--dry-run`, `--export`, `--destroy-data`, `--cluster-listen-ip`, `--playbook` (default `cluster-bloom.yaml`). Requires root unless `--export`.
- **`bloom run <playbook>`** — run an external Ansible playbook through bloom's bundled runtime. Flags: `--tags`, `--dry-run`, `--extra-vars`, `--config`, `--verbose`. Requires root.
- **`bloom cleanup [config-file]`** — tear down a previous install (RKE2 uninstall, Longhorn/mount cleanup, managed-disk wipe). Flag: `--force`. Requires root.
- **`bloom version`** / **`bloom --version`** — print the build version (injected via ldflags).
- **`bloom __child__ …`** — internal re-exec entry point for the namespaced Ansible runtime (see below); not user-invoked.

## Package Organization

- **`cmd/`** — CLI entry point (`main.go`), the host-side post-run ClusterForge/next-step summary (`clusterforge_summary.go`), and embedded web assets (`embed.go`, `web/`).
- **`pkg/config/`** — schema-driven configuration. The schema (`bloom.yaml.schema.yaml`) is the source of truth, loaded by `schema_loader.go`; `loader.go` loads a `bloom.yaml` and applies defaults; `validator.go` / `constraints.go` validate it; `deprecations.go` strips removed keys with a migration warning; `gpu_stack_matrix.go` resolves the GPU driver/ROCm/GPU-operator stack; `supported_os.go` is the single source of truth for supported host OSes; `generator.go` backs the web wizard.
- **`pkg/ansible/runtime/`** — the containerized Ansible runtime (see below): image handling (`container.go`), playbook orchestration (`playbook.go`), the Linux namespace/pivot-root executor (`executor_linux.go`, with a non-Linux stub in `executor_other.go`), embedded-image build hooks (`embedded_image*.go`), output processing (`output.go`, `parser.go`, `stats.go`), signal handling (`signals.go`), and cleanup (`cleanup.go`). Playbooks and manifests are embedded from `playbooks/` and `manifests/`.
- **`pkg/ssh/`** — ephemeral SSH key lifecycle used by the runtime to reach the local node.
- **`pkg/webui/`** — HTTP server, handlers, and embedded filesystem for the configuration wizard/monitoring UI.

## Containerized Ansible Runtime

Bloom does not require Ansible (or Python) to be installed on the host. Instead it runs Ansible from a bundled runtime image inside a lightweight, self-managed container.

### Runtime image

- **Pinned by digest.** `ImageRef` in `pkg/ansible/runtime/container.go` pins `willhallonline/ansible` by digest (corresponding to a specific `…-alpine` tag) for reproducible, supply-chain-safe builds — no floating `:latest`.
- **Default builds** pull and extract the pinned image into a cached rootfs (`.bloom/rootfs`) on first run.
- **Offline builds** (`just build-offline`, build tag `embed_ansible_image`) embed a flattened rootfs tarball into the binary; at run time the rootfs is extracted from the embed with no network pull. See `README.md` → *Building from Source*.

### Execution model

`RunPlaybook` extracts the embedded playbooks and manifests to a working directory, then `RunPlaybookDirect` ensures the rootfs is present and calls `RunContainer` (`executor_linux.go`):

1. **SSH pre-flight** — verify an SSH server is reachable on `127.0.0.1:22`; fail early with guidance otherwise.
2. **Ephemeral SSH key** — `pkg/ssh` generates a throwaway keypair in a temp dir and authorizes it, with cleanup (restoring the original `authorized_keys`) deferred and signal-handled.
3. **Namespaced child** — bloom re-execs itself as `__child__` in new mount/PID/UTS namespaces (`CLONE_NEWNS | CLONE_NEWPID | CLONE_NEWUTS`).
4. **`pivot_root`** — the child pivots into the runtime rootfs, mounts a fresh `/proc`, `/sys`, `/dev`, `/tmp`, bind-mounts the host root at `/host` (for reading resolv.conf and writing `bloom.log`) and the ephemeral SSH dir, then detaches the old root.
5. **Run Ansible** — inside the rootfs it runs `ansible-playbook --connection=ssh --inventory=127.0.0.1, --user=<user> --become …`. Because tasks execute over SSH back to the host, they run in the host's real context (systemd as PID 1, real devices, package manager) even though the Ansible engine is isolated in the container. `BLOOM_DIR` and `BLOOM_VERSION` are injected as extra vars.

This runtime is Linux-only; `executor_other.go` returns an error on other platforms.

### Playbooks and manifests

- **`playbooks/cluster-bloom.yaml`** — the root playbook; it wires shared vars (including the GPU driver support matrix) and includes task groups from `playbooks/tasks/`: `validate_node`, `prepare_node`, `deploy_cluster`, `deploy_clusterforge`, `deploy_k8s_apps`, and `update_certificate`, plus shared checks (`data_safety_check.yaml`, `gpu_rocm_detect.yaml`). Task groups are selectable with `--tags`.
- **`manifests/`** — Kubernetes manifests and scripts extracted alongside the playbooks: `longhorn/`, `local-path/`, and `scripts/`.

### Export mode

`bloom cli … --export` writes a self-contained `./bloom-playbook/` directory (root playbook rewritten to target `localhost`, a `bloom-vars.yaml` derived from the config, and the `tasks/`+`manifests/` trees) instead of executing. It can then be run with a host-native `ansible-playbook bloom-playbook/cluster-bloom.yaml` — the SSH-free path when no `sshd` is available.

## Configuration System

### Loading and precedence

`config.LoadConfig` reads `bloom.yaml` and applies schema defaults. CLI flags (e.g. `--cluster-listen-ip`) override file values. Deprecated keys are stripped with a warning (`ApplyDeprecations`) before validation so a stale config keeps working.

### Validation

- **`Validate`** — full validation (required cluster fields, formats, mutually-exclusive/one-of constraints, GPU-stack compatibility) for normal runs.
- **`ValidateOptional`** — relaxed mode for node-local diagnostic tags (e.g. `--tags validate_node`): still flags unknown keys and malformed values but does not require full cluster fields, so a node can be checked against a minimal or empty `bloom.yaml`.
- **`update_cert`** runs skip schema validation entirely (they use a separate cert-update config).

After validation, bloom injects derived vars: `ApplyGPUStackVars` resolves the GPU driver/ROCm/GPU-operator defaults, and `supported_ubuntu_versions` is populated from `config.SupportedOSes` so the Go side and the playbook's Ubuntu check never drift.

### Host OS pre-flight

Before running, bloom checks the host OS against `SupportedOSes` and fails by name on an unsupported OS (overridable with `BLOOM_ALLOW_UNSUPPORTED_OS=true`).

## Run Output and Post-Run Guidance

Output is processed by `pkg/ansible/runtime/output.go`:

- **Clean mode** (default) — one emoji-tagged line per task, a `Playbook complete: …` counts summary, and a single **overall status verdict** (`SUCCESS` / `COMPLETED WITH WARNINGS` / `FAILED`). Full output always goes to `bloom.log`.
- **Verbose mode** (`bloom run --verbose`) — raw Ansible output.

After the playbook, the host process prints next-step guidance based on the exit code and real cluster state (see `configuration-reference.md` → *Deployment output and post-run guidance*): remediation on failure (including `RANCHER_DISK`/`SKIP_RANCHER_PARTITION_CHECK` for an undersized `/var/lib/rancher`), a full-run hint for a validated but unprovisioned node, a `deploy_clusterforge` hint when the cluster is up without ClusterForge, or the endpoint/credential reference block once ClusterForge is deployed. This runs in the host process (`cmd/clusterforge_summary.go`) because the namespaced child cannot query the cluster via `kubectl`.

## Web UI

`pkg/webui` serves the configuration wizard and monitoring UI (`server.go`, `handlers.go`, `fs.go`) with embedded static assets. It generates a `bloom.yaml` from schema-driven form fields (`config.Schema`) and can trigger a deployment.

## Integration Architecture

### ClusterForge

ClusterForge is deployed as the post-Kubernetes application platform, selected by `CLUSTERFORGE_RELEASE` (version tag, release URL, `latest`, or `none`) via the `deploy_clusterforge` tasks. After deployment, bloom surfaces endpoint URLs and credential-retrieval commands.

### OIDC Authentication Architecture

**Multi-Provider Support**:
ClusterBloom supports both default and additional OIDC providers for flexible authentication:

```yaml
# Default provider (auto-configured)
# Generated from DOMAIN: "example.com"
# Results in: https://kc.example.com/realms/airm

# Additional providers (optional)
ADDITIONAL_OIDC_PROVIDERS:
  - url: "https://auth.company.com/realms/main"
    audiences: ["kubernetes", "api"]
  - url: "https://external-provider.com/auth"
    audiences: ["k8s"]
```

**Configuration Generation Pipeline** (applies to all control-plane nodes):
1. **Default Provider Generation**: Auto-create `https://kc.{DOMAIN}/realms/airm` with audience `k8s`
2. **Provider Validation**: Validate HTTPS URLs and audience format for additional providers
3. **Certificate Fetching**: Retrieve SSL certificates for each OIDC provider
4. **Authentication Configuration**: Generate `/etc/rancher/rke2/auth/auth-config.yaml` with all providers
5. **RKE2 Integration**: Configure kube-apiserver to use authentication configuration file
6. **Service Restart**: Trigger RKE2 server restart with new authentication configuration

Each control-plane node runs its own kube-apiserver and needs OIDC configuration.

**RKE2 Integration**:
Generated configuration for `/etc/rancher/rke2/config.yaml`:
```yaml
kube-apiserver-arg:
  - "--authentication-config=/etc/rancher/rke2/auth/auth-config.yaml"
```

**Authentication Configuration File**:
Generated `/etc/rancher/rke2/auth/auth-config.yaml`:
```yaml
apiVersion: apiserver.config.k8s.io/v1
kind: AuthenticationConfiguration
jwt:
- issuer:
    url: https://kc.example.com/realms/airm
    certificateAuthority: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
    audiences:
    - k8s
  claimMappings:
    username:
      claim: preferred_username
      prefix: "oidc:"
    groups:
      claim: groups
      prefix: "oidc:"
```

**Authentication Flow**:
1. User authenticates with the configured OIDC provider (Keycloak, etc.)
2. Provider issues a JWT with user claims and group memberships
3. kubectl sends the token via the `Authorization: Bearer <jwt>` header
4. kube-apiserver validates the token against the configured providers
5. Kubernetes RBAC determines permissions based on token claims

## Security Architecture

- **Privilege**: `cli`, `run`, and `cleanup` require root; the runtime uses `--become` for host tasks.
- **Ephemeral SSH**: the loopback SSH access uses a throwaway keypair created per run and removed afterward, restoring the original `authorized_keys`.
- **Runtime isolation**: the Ansible engine runs in its own namespaces/rootfs; host mutation flows only through the explicit SSH channel and the `/host` bind mount.
- **OS pre-flight**: unsupported host OSes are rejected early by name.

## Platform Support

The deployment runtime is **Linux-only** (namespaces + `pivot_root`). The web UI and configuration tooling run anywhere Go builds, but `bloom cli`/`bloom run` return an error on non-Linux hosts.

See [PRD.md](./PRD.md) for the product overview and feature descriptions, and [configuration-reference.md](./configuration-reference.md) for configuration keys and run behavior.
