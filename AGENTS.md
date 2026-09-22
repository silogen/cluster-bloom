# AGENTS.md

This file tells agents how to work in this repository.

## What this is

`bloom` is a single static Go binary that installs RKE2 Kubernetes on Ubuntu
nodes, with AMD GPU driver management, disk and Longhorn storage setup, and
ClusterForge bootstrap. It works one node at a time. You run the binary once
per node, and every command except `--export` needs `sudo`.

## Build and test

```sh
just build              # -> dist/bloom, version "dev-build"
just build-commit       # version = short commit hash
just build v1.2.3       # explicit version (ldflags -X cmd.Version)

go build ./...
go test ./...                              # all Go tests, ~0.5s, no cluster needed
go test ./pkg/config -run TestValidate -v  # single test
```

Ansible unit tests run in a container, because the fixtures write to
`/etc/rancher/rke2` and refuse to run outside one:

```sh
bash tests/ansible/run_tests_docker.sh                        # all
bash tests/ansible/run_tests_docker.sh pytest unit/test_rke2_config.py -v
```

QEMU integration test, which CI runs on every pull request. It needs kvm,
`mkisofs` and `ovmf`:

```sh
sudo bash tests/qemu/manual-qemu-test.sh --timeout 120 qemu-vm \
  tests/qemu/profile_2_nvme.yaml dist/bloom tests/qemu/bloom.yaml
```

There is no linter config and no pre-commit hook. CI (`run-tests.yml`) runs
`go test ./...`, the Ansible container tests, and the QEMU test.

## Architecture

The Go layer is a thin wrapper. All installation logic lives in Ansible
playbooks under `pkg/ansible/runtime/playbooks/`, embedded into the binary with
`go:embed`. Go reads and validates the config, resolves the GPU version pins,
then passes everything to Ansible as `-e` vars. To change install behavior,
edit the YAML, not the Go.

Execution path for `bloom cli bloom.yaml`:

1. `cmd/main.go` loads `bloom.yaml`, applies the schema defaults and the
   environment overrides, validates the result, and injects the GPU stack vars.
2. `pkg/ansible/runtime` extracts the embedded playbooks and manifests to
   `./.bloom/playbooks/`, and the Ansible runtime image to `./.bloom/rootfs/`.
3. `executor_linux.go` re-execs `/proc/self/exe __child__` inside new UTS, PID
   and mount namespaces, bind-mounts the host at `/host`, chroots, and runs
   `ansible-playbook` against `localhost`.
4. `output.go` reformats the Ansible output into the terse emoji mode and
   writes `bloom.log`, which it rotates to `bloom-<timestamp>.log` on each run.

Bloom needs no Docker. `container.go` pulls the runtime image
(`willhallonline/ansible:latest`) with `crane`, untars it into a rootfs, and
runs it in namespaces bloom creates itself. Ansible connects over SSH to
localhost with an ephemeral ed25519 key. `pkg/ssh` adds that key to the
invoking user's `authorized_keys` and removes it when the run ends, including
on a signal (`signals.go`). If you change the executor, keep that cleanup path.
A leaked key or a damaged `authorized_keys` file is the worst failure this code
can cause.

`RunContainer` and `RunChild` run on Linux only (`executor_linux.go`, with an
`executor_other.go` stub), and so does the disk-safety code. Builds for other
systems compile but refuse to deploy.

## Conventions

Write all documentation, commit messages and plan files using
ASD-STE100 Simplified Technical English.

Use conventional commits: feat, docs, fix, chore. After the type, start the
description with a capitalized verb in present tense, for example
"feat: Remove something from somewhere" or "fix: Prevent X from doing Y".
Keep the title at 72 characters or less, and each body line at 80 or less.
A commit that changes only tests is a chore. For a breaking change, append
"!" to the type, such as "feat!" or "fix!", and start the body with the
paragraph "BREAKING CHANGE: <what breaks and why>".

Title each pull request "EAI-NNNN Verb ...".
EAI-NNNN is the Jira ticket number, and the word after it is a verb with a
capital first letter. Ask me for the ticket number if you do not have it.

Ensure when adding or removing features that the docs/PRD.md and the
docs/installation-guide.md are aligned and updated to match implementation.

### Config is schema-driven

`pkg/config/bloom.yaml.schema.yaml` is the single source of truth for every
`bloom.yaml` field. It holds the type, the default, the section, the
`required: when(...)` condition, and the regex patterns under the top-level
`types:` block. It feeds:

- defaults and environment overrides (`loader.go`)
- validation (`validator.go`, plus the mutual-exclusion rules in
  `constraints.go`)
- the `CONFIGURATION FIELDS` block in `bloom --help` (`buildConfigFieldsHelp`)

To add or change a field, edit the schema. No Go code is needed. Two places
duplicate the defaults on purpose, and you must update them with the schema.
The first is the `vars:` block of `playbooks/cluster-bloom.yaml`, which lets a
direct `ansible-playbook` run work without the Go layer. The second is the
config table in `README.md`. `CONTRIBUTING.md` also asks you to document the
field in `docs/configuration-reference.md`.

The validator rejects unknown keys in `bloom.yaml`, so the Go layer injects the
internal Ansible vars (`bloom_run_id`, `RKE2_PRESERVE_EXISTING`, `PAUSE_K3S`,
`bloom_config_file`) after validation.

### Playbook tags are the public interface

`playbooks/cluster-bloom.yaml` imports one task tree per phase, and a tag gates
each one: `pre_deployment`, `k3s_pause`, `validate_node`, `prepare_node`,
`deploy_cluster`, `deploy_k8s_apps`, `deploy_clusterforge`, `update_cert`.
Users call these tags themselves, for example `--tags deploy_clusterforge` or
`--tags gpu`. A tag name is therefore part of the public interface, and a
rename breaks the user commands that name it. Several Go tests parse the
playbook YAML and check its structure
(`pkg/ansible/runtime/playbooks_test.go`), so rename a tag or an include in
both places.

`--export` writes the same tree to `./bloom-playbook/` as a self-contained
directory that stock `ansible-playbook` can run. Keep the playbooks free of
anything that works only under bloom's own runtime.

### GPU driver policy

`pkg/config/gpu_stack_matrix.go` holds an exact allowlist of driver tuples. A
tuple names the driver release, the installer version, the DKMS build, the
paired ROCm release, and the AMD-SMI package. Do not turn this into a `>=`
comparison, because the members of a tuple are qualified together. Bloom
manages only the host DKMS driver and the standalone AMD-SMI package. It never
installs the host ROCm runtime, which stays in containers. An unsupported or
ambiguous driver stops the run before bloom changes a repository or a package.

A driver install that needs a reboot writes
`/var/lib/bloom/reboot-required.json`, tagged with the run ID. The `Attempted`
field is a loop guard. Bloom reboots at most once for the same unresolved
condition, then leaves the run to Ansible's failure message.

### Disk safety

`cleanup_preflight.go` and `diskguard.go` are fail-closed and need extra care.
They cross-check `bloom.yaml` against `/etc/fstab`, the live mounts, and a list
of protected mounts (`/`, `/boot`, `/usr`, `/home`, and others) before bloom
wipes anything. Preflight canonicalizes the device aliases (`UUID=`,
`/dev/disk/by-id`) to compare identities. The playbook keeps the operator's
original spelling, so a stable reference still works if the kernel renumbers
the devices. `bloom cleanup --preflight-only` validates and changes nothing.

## Stale documentation

`CODE_STYLE.md` and `docs/technical-architecture.md` describe the architecture
before the Ansible refactor, with viper, logrus, a Bubble Tea TUI, a
`pkg/steps.go` and a web wizard. Those files, packages and dependencies are
gone. Trust the code and `README.md` instead. Treat those two documents as
history unless you are updating them.

One rule from `CODE_STYLE.md` still holds. Every new Go file gets the Apache
2.0 AMD copyright header.
