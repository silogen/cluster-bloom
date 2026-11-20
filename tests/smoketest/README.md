# Cluster smoke test

A read-first health check for an EAI / AIRM cluster. It answers one question:

> **Does this cluster actually work — infrastructure, platform and applications?**

If every check passes, the run ends with `No warnings.` and exit code `0`, and
you can take that as: the substrate is healthy, cluster-forge is deployed and
serving, and a user can really create a workspace, upload a dataset, deploy a
model and fine-tune it.

Nothing is inferred. Each check either tests the thing or says it did not.
A run that could not test something never reports success.

**Version 0.6.0. Targets the current cluster-bloom and cluster-forge. It is
not backwards compatible with older versions of either** — a deliberate
decision, not an accident.

Intended use: **on release**, **after a cluster deployment** to validate the
cluster is healthy, and **as part of CI**.

---

## Target state

What the smoke test is required to cover, and where each item lives. Use this
to tell at a glance whether a change drops a requirement.

### Required test components

| Requirement | Where |
|---|---|
| Application kubectl access verification | platform §1 |
| Download kubeconfig from Resource Manager | platform §1 |
| Authenticate as `devuser@<domain>` | platform §1 (`SMOKETEST_USER`, default `devuser@<domain>`) |
| `kubectl get nodes` / `get apps -n argocd` proves the kubeconfig works | platform §1 — node count checked against both Resource Manager and the admin context; reading ArgoCD Applications must be **forbidden**, since success there would mean the application kubeconfig is over-privileged |
| AIRM + AIWB services up and running | platform §6 |
| Deploy workspace functionality (MLflow) | application — `--with-workspace`, MLflow is the default type |
| Single AIM deployment test | application — `--with-model`, verified by an actual chat completion |
| Model must not require an HF token | default `google/gemma-3-1b-it` is ungated; with no token the check picks an open model and only SKIPs if the catalog has none |
| Smallest model possible | `gemma-3-1b-it`; fine-tuning likewise defaults to the smallest base model the cluster can run |
| Workbench dataset upload capability | application — `--with-dataset` (upload, read back, download, delete) |
| Authentication checks for Gitea | platform §7 — real API login as `silogen-admin` |
| Authentication checks for Argo | platform §7 — real session token issued |
| Connectivity and health checks for all critical services | cluster §1–5 and platform §2–8 |
| Workbench fine-tuning | application — `--with-finetune`, waits for completion and checks the produced model is registered |

### Required behaviour

| Requirement | How it is met |
|---|---|
| Automated, clear pass/fail | Exit `0` / `1` / `2` plus a machine-readable `SMOKETEST_RESULT` line |
| Error handling and meaningful messages | No `set -e` — every check runs to completion and reports its own reason; only a missing tool or total loss of cluster access exits early |
| Completes in a reasonable timeframe (< 10 min) | The summary prints the runtime and says plainly when a run approaches or exceeds the 10-minute budget |
| Summary report of all tested components | The Summary section: OK / WARN / SKIP / BLOCKED counts and every finding |
| Rollback / cleanup of created test resources | Everything the application layer creates is deleted again; an interrupted run says what was left behind, and re-running adopts and cleans it up |
| No backwards compatibility required | Stated above — current bloom/forge only |

### Known limitation

Earlier versions drove the browser UI with Playwright, and broke every time a
UI changed. This version talks to the **application APIs** instead, which is
far more stable — but the APIs are still a moving target. When forge changes
an API, the test needs updating; treat a failure as "check the test too", not
only "the cluster is broken".

---

## The three layers

The test is split by layer, and the layers are independent.

| Layer | What it covers | Writes anything? | Typical time |
|---|---|---|---|
| **cluster** | The substrate: nodes, control plane, CoreDNS, Cilium (incl. overlay MTU), storage provisioner and StorageClasses. Produced by cluster-bloom on RKE2. | No (one pod exec for the MTU check) | seconds |
| **platform** | What cluster-forge deploys: Resource Manager kubeconfig, ArgoCD apps, Envoy Gateway, MetalLB, SeaweedFS, PVCs, Keycloak, OpenBao, AIRM/AIWB workloads, application logins, TLS certificates. | No — fully read-only | ~1 min |
| **application** | The real AIWB user paths: workspace, dataset upload, model deployment, fine-tuning. | **Yes** — creates and deletes real resources, uses accelerators | several minutes |

**A layer does not check the one underneath it.** If the cluster is sick, the
platform checks fail in ways that look like platform faults. When in doubt,
run `just cluster` first.

`just application` runs the platform layer's read-only sections first — it
needs their authentication, domain and project before it can reach the AIWB
API. That is about a minute of a ten-minute run, not a separate choice.

---

## Prerequisites

**Tools** — `bash` 4+, `kubectl`, `curl`, `jq`. Missing any of these exits
immediately with code 2. `openssl` is optional; without it, certificate
expiry checks are skipped. `just` is optional but is the intended entry point.

**Cluster access** — a working **admin** kubeconfig (`KUBECONFIG`, or the
current `kubectl` context). There is no other way in: the Resource Manager
application kubeconfig is downloaded and exercised as a check of its own, but
it is not a substitute for admin access.

**Credentials** — none need setting up. Environment variables always win;
anything left is read from the cluster itself when the current context works
(`SMOKETEST_USER`, `SMOKETEST_PASS`, `KEYCLOAK_CLIENT_ID` /
`KEYCLOAK_CLIENT_SECRET`, the domain, the project).

**For the application layer only:**
- A free accelerator. Without one, the model and fine-tune checks SKIP rather
  than fail.
- **A Hugging Face token (`HF_TOKEN_FILE=<path>`). Not required for
  correctness, but required in practice if you want the run to finish in
  reasonable time.**

  The test never *needs* a token — by design, it always falls back to a model
  it can pull without one, and says which it picked.

  **Give it a token and it selects smaller models.** That sounds backwards
  until you look at the catalogs: the small models are the gated ones, and
  what is left ungated is the large end. So a token is not about unlocking
  more capability here — it is what lets the test pick the cheapest model that
  proves the path works, instead of the smallest one it happens to be allowed
  to pull.

  | Check | With a token | Without |
  |---|---|---|
  | Model deployment | a ~1B model, ~1 min | a mid-size model, ~5 min |
  | Fine-tuning | a small gated model, a couple of min | a 30B-class model — download, load and shard tens of GB for a tiny training set, 10 min or more |

  That is the usual reason a run looks like it has hung: it has not, it is
  loading an enormous checkpoint for a trivial job. **Pass a token and the
  whole run fits inside the 10-minute budget; without one, expect to exceed
  it.**

  ```bash
  just HF_TOKEN_FILE=/path/to/hf_token application
  ```

  Use `HF_TOKEN_FILE` rather than `--hf-token` / `SMOKETEST_HF_TOKEN`: a token
  on the command line is visible in `ps` and in shell history.

---

## Usage

```bash
just                       # list recipes
just help                  # the full built-in reference

just cluster               # substrate only
just platform              # cluster-forge only — read-only, creates nothing
just application           # the AIWB flows — slow, creates real resources, uses GPUs
just all                   # all three layers, one merged verdict
just check                 # syntax-check the scripts (no cluster needed)
```

Variables come **before** the recipe name, though the shell-prefix form also
works:

```bash
just KUBECONFIG=/path/to/admin_kubeconfig all
KUBECONFIG=/path/to/admin_kubeconfig just all

just DOMAIN=cluster.example.com INSECURE=1 platform
just HF_TOKEN_FILE=/path/to/hf_token PROJECT=myproject application
```

| Variable | Meaning |
|---|---|
| `KUBECONFIG=<path>` | Admin kubeconfig. Unset uses `~/.kube/config` |
| `DOMAIN=<domain>` | Override the auto-detected cluster domain |
| `VERBOSE=1` | Per-section timings and extra detail |
| `INSECURE=1` | Skip TLS verification (self-signed cluster certs) |
| `PROJECT=<name>` | AIWB project for the application layer |
| `HF_TOKEN_FILE=<path>` | Hugging Face token, read from a file. Strongly recommended for the application layer — see [Prerequisites](#prerequisites) |
| `FLAGS="..."` | Extra flags passed straight through to the scripts |

### Running the scripts directly

`just` is a thin wrapper. The scripts stand alone and have their own help:

```bash
./smoketest-cluster.sh --help
./smoketest-platform.sh --help
./smoketest-platform.sh --with-workspace --with-dataset --with-model --with-finetune
```

`cluster-smoketest.sh` is a compatibility shim kept for older callers — it
runs both layers and forwards flags to whichever one understands them. For
new work use `just` or the two scripts directly.

---

## Reading the result

Every run ends with a summary: counts, then each finding, then the runtime.

```
  OK   58    WARN 0    SKIP 3    BLOCKED 0

  Runtime: 0m 47s

  No warnings. 3 check(s) skipped (not applicable or not requested).

SMOKETEST_RESULT platform ok=58 warn=0 skip=3 blocked=0
```

| Exit | Meaning |
|---|---|
| **0** | Clean. Everything in scope was tested and passed. |
| **1** | Warnings — something is off but working. Review the findings. |
| **2** | Blocked. A check was in scope and could not run, so **this run does not prove the layer is healthy**. |

`just all` exits with the worst of the layers it ran.

**SKIP vs BLOCKED** — this distinction is the point of the whole design.
*SKIP* means the check did not apply or was not asked for: a small cluster has
no Longhorn, no accelerator is free, the model checks were not opted into. A
skip never fails a run. *BLOCKED* means the check was in scope and could not
run: no credentials, no domain, forbidden. A blocked check always fails the
run, because a run that could not test something must not report that it
passed.

The last line, `SMOKETEST_RESULT <layer> ok=N warn=N skip=N blocked=N`, is
machine-readable and is what `just all` collects for its merged verdict.

The whole thing is budgeted at 10 minutes; the summary says so plainly when a
run goes over.

**If a run looks stuck**, it is almost always the model or fine-tuning check
loading a large checkpoint with no Hugging Face token set — see
[Prerequisites](#prerequisites). The check prints which model it picked and
why before it starts waiting, so scroll back to that line first.

---

## What the application layer creates

The four opt-in checks create real resources and delete them again:

- a workspace (MLflow by default — no GPU, small image)
- a small JSONL dataset, read back, downloaded, deleted
- one AIM model deployment (default `google/gemma-3-1b-it`), verified by
  serving an actual chat completion. If the model is already deployed and
  serving, nothing is deployed — the check just talks to it.
- a fine-tuning job, waited to completion, with the resulting fine-tuned model
  checked for registration in the project

If the run is interrupted in between, a resource can be left behind and will
keep holding whatever it reserved; the run says so on its way out. **Re-running
with the same flags adopts and cleans up what it recognises.** The `--keep-*`
flags deliberately leave resources in place for inspection — you then delete
them yourself.

---

## Files

| File | |
|---|---|
| `justfile` | Entry point. `just help` for the full reference |
| `smoketest-cluster.sh` | Substrate layer |
| `smoketest-platform.sh` | Platform + application layers |
| `lib-smoketest.sh` | Shared output, preflight, cluster access, summary |
| `cluster-smoketest.sh` | Compatibility shim — runs both layers |

None of the scripts use `set -e`. Every check runs to completion; only a
missing tool or total loss of cluster access stops a run early.
