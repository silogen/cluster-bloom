# Plan: skip Case 1 reboot when adding a GPU worker whose driver is already good

Status: **IMPLEMENTED** on `fix_reboot_task` (see "Implementation" below).
Branch: `fix_reboot_task`
Scope: `pkg/ansible/runtime/playbooks/tasks/reboot_required_check.yaml`
       (+ optional wiring/config + tests)

## Problem

`reboot_required_check.yaml` triggers a reboot when **either** of two signals is
true:

```yaml
bloom_reboot_required:
  {{ bloom_reboot_required_stat.stat.exists            # Case 1: OS flag
     or (gpu_driver_reboot_required | default(false)) }} # Case 2: driver mismatch
```

**Case 1** — `/var/run/reboot-required` — is routinely left by *unrelated* OS
packages (`libc6`, `dbus`, kernel security updates). When we **add a GPU worker**
whose amdgpu DKMS driver is **already active and matching for the running
kernel**, Case 1 forces an unnecessary reboot + re-run, lengthening every
join. The out-of-tree OCT2 patch (`/workspace/oct2-ansible-patches/`) already
proves the fix works in the field; this plan folds that behavior into bloom
itself, safely.

## Goal / non-goals

**Goal:** On an add-worker run (`FIRST_NODE == false`), when Case 2 is false
(driver genuinely active for the running kernel), do **not** let a bare Case 1
OS flag trigger a reboot. Proceed with the join.

**Non-goals / must NOT change:**
- Fresh-cluster / first-node installs must keep the normal reboot-and-rerun
  path (inbox module → DKMS takeover). Do not weaken that.
- Case 2 (`gpu_driver_reboot_required`) must ALWAYS still trigger a reboot —
  a real driver mismatch is exactly what we must not skip.
- The loop-guard (`attempted: true` → fail hard) must keep working.

## Key facts established

- `gpu_driver_reboot_required` is set in `prepare_node/gpu_driver_only.yaml`
  from `gpu_driver_active_module_matches`, a robust check: active vs selected
  `srcversion`, then `version`, then the `O` (out-of-tree) taint bit. This is a
  strictly stronger signal than the OCT2 patch's `lsmod`+`dkms status` grep, so
  reusing it means we do NOT need to re-shell `dkms`/`lsmod`.
- "Adding a worker" is expressed as `FIRST_NODE == false` (see
  `bloom.yaml.schema.yaml`).
- `gpu_driver_verify_devices` (also in `gpu_driver_only.yaml`) confirms
  `/dev/kfd` + `/dev/dri/renderD*` exist — a second, independent "GPU is
  actually usable now" signal.

## Design

Introduce a **guarded bypass of Case 1 only**, gated on all of:

1. `not FIRST_NODE` — add-worker context only.
2. `not gpu_driver_reboot_required` — Case 2 is clear (driver active & matching).
3. `gpu_driver_verify_devices.rc == 0` — GPU device nodes present (belt &
   braces; the driver is not merely loaded but usable).

When all three hold and the ONLY reason to reboot is the OS flag, we:
- rename (not delete) `/var/run/reboot-required` and
  `/var/run/reboot-required.pkgs` to `*.backup` (matches OCT2 patch: reversible,
  auditable), and
- treat Case 1 as satisfied for this run.

Make it **opt-outable** with a new config knob so operators can force strict
behavior:

- `GPU_WORKER_SKIP_OS_REBOOT` (bool, default `true`) — when false, keep the
  current always-reboot-on-Case-1 behavior.

> Note: the new key lives under the existing **`⚙️ Advanced Configuration`**
> section (there is no separate "🖥️ GPU Configuration" section), next to
> `GPU_INSTALL_HOST_TOOLS`, and is gated `applicable: when(GPU_NODE == true)`
> to match the neighboring GPU keys.

### Where the change goes

`reboot_required_check.yaml`, immediately BEFORE
`Resolve effective reboot requirement`:

```yaml
- name: Decide whether a bare OS reboot flag can be bypassed on an add-worker join
  set_fact:
    bloom_os_reboot_bypassable: >-
      {{ (not (FIRST_NODE | default(false) | bool))
         and (GPU_WORKER_SKIP_OS_REBOOT | default(true) | bool)
         and (not (gpu_driver_reboot_required | default(false) | bool))
         and ((gpu_driver_verify_devices.rc | default(1)) == 0) }}

- name: Back up (rename) OS reboot-required markers when bypassing on a good GPU worker
  command: mv {{ item }} {{ item }}.backup
  loop:
    - /var/run/reboot-required
    - /var/run/reboot-required.pkgs
  failed_when: false
  when:
    - bloom_reboot_required_stat.stat.exists
    - bloom_os_reboot_bypassable | bool

- name: Report the OS-reboot bypass
  debug:
    msg: >-
      {{ inventory_hostname }}: add-worker join, amdgpu active & matching for
      {{ ansible_kernel }}; renamed OS reboot-required marker(s) to *.backup,
      proceeding without a reboot.
  when:
    - bloom_reboot_required_stat.stat.exists
    - bloom_os_reboot_bypassable | bool
```

Then change the effective-requirement line so a bypassed OS flag does not count:

```yaml
- name: Resolve effective reboot requirement
  set_fact:
    bloom_reboot_required: >-
      {{ (bloom_reboot_required_stat.stat.exists
          and not (bloom_os_reboot_bypassable | default(false) | bool))
         or (gpu_driver_reboot_required | default(false) | bool) }}
```

Everything downstream (marker write, loop guard, `end_play`) is unchanged and
now simply never fires for a bypassed-Case-1-only worker, because
`bloom_reboot_required` is false when Case 2 is also false.

## Safety argument

- Fresh install (`FIRST_NODE == true`): bypass disabled → unchanged.
- Case 2 true (driver mismatch/inbox still active): `gpu_driver_reboot_required`
  keeps `bloom_reboot_required` true → reboot still happens. Correct.
- Add-worker + Case 1 only + driver active + devices present: bypass →
  no reboot. This is the intended win.
- Devices missing / driver not matching: bypass condition false → we fall
  through to the normal reboot path. Fail-safe, not fail-open.
- Rename (not delete) keeps the change reversible and visible for audit.

## Files to change

1. `pkg/ansible/runtime/playbooks/tasks/reboot_required_check.yaml`
   - add the three tasks above; adjust `Resolve effective reboot requirement`;
     extend the header comment documenting the Case-1 bypass and its guards.
2. `pkg/config/bloom.yaml.schema.yaml`
   - add `GPU_WORKER_SKIP_OS_REBOOT` (bool, default true, section
     "🖥️ GPU Configuration"), applicable `when(FIRST_NODE == false)`.
   - update the field-count assertion in `schema_loader_test.go` (40 → 41).
3. Tests
   - `pkg/config/gpu_stack_matrix_test.go` / schema test: assert new key parses,
     default true, and mutually-exclusive rules unaffected.
   - If a playbook-lint/gate test exists (`playbooks_test.go`), ensure the new
     `when:` conditions reference defined vars.

## Implementation (applied on `fix_reboot_task`)

What actually changed on disk:

1. `pkg/ansible/runtime/playbooks/tasks/reboot_required_check.yaml`
   - Extended the header comment with a "Case-1 bypass on an add-worker join"
     paragraph.
   - Added three tasks after `Check whether a reboot is required`:
     `Decide whether a bare OS reboot flag can be bypassed on an add-worker
     join` (sets `bloom_os_reboot_bypassable`), `Back up (rename) OS
     reboot-required markers ...`, and `Report the OS-reboot bypass`.
   - Changed `Resolve effective reboot requirement` so a bypassed OS flag no
     longer counts:
     `(stat.exists and not bloom_os_reboot_bypassable) or gpu_driver_reboot_required`.
2. `pkg/config/bloom.yaml.schema.yaml`
   - Added `GPU_WORKER_SKIP_OS_REBOOT` (bool, default true,
     `applicable: when(GPU_NODE == true)`) under `⚙️ Advanced Configuration`,
     directly after `GPU_INSTALL_HOST_TOOLS`.
3. `pkg/config/schema_loader_test.go`
   - Field-count assertion updated **41 → 42** and the comment listing the
     expected keys extended with `GPU_WORKER_SKIP_OS_REBOOT`.

### Corrections vs the original draft

- The schema already had **41** keys (not 40); the new key makes it **42**.
- There is **no `🖥️ GPU Configuration` section**; GPU keys live under
  `⚙️ Advanced Configuration`, so the new key was placed there.

### Not yet verified

- `go build` / `go test` could not run here (`go` is only available via devbox
  in this environment). Run `devbox run -- go test ./pkg/config/...
  ./pkg/ansible/...` on a dev box to confirm the field-count test and playbook
  gates pass.

## Verification

- `go test ./pkg/config/... ./pkg/ansible/...`
- `just` build; `bloom` schema round-trips with and without the new key.
- Dry-run matrix (document expected outcome, no live node needed):
  | FIRST_NODE | OS flag | gpu_driver_reboot_required | devices | knob | reboot? |
  |---|---|---|---|---|---|
  | true  | yes | no  | yes | true  | YES (fresh install untouched) |
  | false | yes | no  | yes | true  | no (bypass — the fix) |
  | false | yes | yes | yes | true  | YES (Case 2 wins) |
  | false | yes | no  | no  | true  | YES (devices not ready) |
  | false | yes | no  | yes | false | YES (opt-out) |
  | false | no  | no  | yes | true  | no (nothing to do) |

## Rollback

- Set `GPU_WORKER_SKIP_OS_REBOOT: false` in `bloom.yaml` for strict behavior
  without reverting code.
- On a node, restore markers: `mv /var/run/reboot-required.backup
  /var/run/reboot-required` (and `.pkgs`).

## Open questions

- Confirm the exact section label / naming convention for the new schema key
  against neighboring GPU keys before adding it.
- Should the bypass also apply to a control-plane join (`FIRST_NODE == false`
  but `CONTROL_PLANE == true`)? Current plan: yes — the gate is `not FIRST_NODE`,
  which covers both worker and joining control-plane. Decide if control-plane
  joins should be stricter.
