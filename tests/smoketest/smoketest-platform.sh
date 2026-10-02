#!/usr/bin/env bash
#
# smoketest-platform.sh — cluster-forge platform health checks.
#
# The layer ABOVE the substrate: everything cluster-forge deploys through
# ArgoCD — the Gateway, MetalLB, SeaweedFS, Keycloak, OpenBao, Gitea, AIRM,
# AIWB — plus the application paths the ticket asks for (Resource Manager
# kubeconfig, workspace, dataset, model, fine-tuning).
#
# Substrate-agnostic on purpose: it asserts nothing about RKE2, Cilium, bloom
# or Longhorn, so it keeps working if the cluster underneath becomes OpenShift
# or Talos. Run smoketest-cluster.sh for that layer.
#
# It does NOT check whether the cluster underneath is healthy. If it is not,
# these checks will fail in ways that look like platform faults. Run the
# cluster layer first when in doubt.
#
# DELIBERATELY does not use `set -e`. Every check runs to completion; nothing
# stops the run except a missing tool or no cluster access at all.
#
# Requires : bash 4+, kubectl, curl, jq      (openssl optional — cert expiry)
# Exit     : 0 clean · 1 warnings · 2 something in scope could not run
#
# EAI-5860
#

VERSION="0.6.0"
SMOKE_LAYER="platform"

# Sourced BEFORE this script's own defaults and before argument parsing. The
# library assigns the shared defaults unconditionally (INSECURE=0, USE_COLOR=1,
# VERBOSE=0), so sourcing it afterwards silently undid every flag that sets
# one — --verbose did nothing at all.
# shellcheck source=lib-smoketest.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib-smoketest.sh"


DOMAIN=""
INSECURE=0
SKIP_LOGIN=0
SKIP_CERTS=0
USE_COLOR=1
VERBOSE=0
HTTP_TIMEOUT=10
KUBE_TIMEOUT=15
CERT_WARN_DAYS=14
SKIP_RM=0
WITH_WORKSPACE=0
WORKSPACE_TYPE="mlflow"
WITH_DATASET=0
KEEP_DATASET=0
KEEP_WORKSPACE=0
WORKSPACE_TIMEOUT=300
WITH_MODEL=0
KEEP_MODEL=0
MODEL_AIM_ID="google/gemma-3-1b-it"
MODEL_EXPLICIT=0
MODEL_TIMEOUT=900
# Most environments have no Hugging Face token, and the platform does not
# inject one for inference, so a gated model simply cannot be deployed without
# it. HF_TOKEN is honoured as a convenience because it is the name the
# huggingface tooling already uses.
SMOKETEST_HF_TOKEN="${SMOKETEST_HF_TOKEN:-${HF_TOKEN:-}}"
HF_TOKEN_FILE=""
WITH_FINETUNE=0
KEEP_FINETUNE=0
FINETUNE_MODEL=""
FINETUNE_TIMEOUT=1800

# ---------------------------------------------------------------------------
usage() {
    cat <<'USAGE'
smoketest-platform.sh — cluster health smoke test

Usage:
  ./smoketest-platform.sh [options]

Options:
  --domain <domain>    Override auto-detected cluster domain
  --insecure           Skip TLS verification on HTTPS checks
  --skip-login         Skip platform application login checks (section 7)
  --skip-certs         Skip TLS certificate checks
  --skip-rm            Skip the Resource Manager kubeconfig checks
  --with-workspace     Deploy a workspace through the AIWB API and wait for it
                       to run. The only check that creates anything — off by
                       default, since every other check is read-only.
  --workspace-type <t> mlflow (default), vscode, jupyterlab or comfyui. MLflow
                       needs no GPU and pulls a small image, so it is the
                       cheapest proof that the workspace path works.
  --keep-workspace     Do not delete the workspace afterwards (leaves it for
                       inspection; delete it manually)
  --with-dataset       Upload a small JSONL dataset through the AIWB API, read
                       it back, download it and delete it. Also creates
                       something, so it is off by default too.
  --keep-dataset       Do not delete the uploaded dataset afterwards
  --workspace-timeout <s>  How long to wait for it to reach Running (default 300)
  --with-model         Deploy one AIM model, wait for it to serve a chat
                       completion, then delete it. Needs a free accelerator,
                       so it is off by default and SKIPs when there is none.
  --model <aimId>      Which model to deploy (default google/gemma-3-1b-it).
                       This is the aimId, not the cluster resource name.
  --keep-model         Do not delete the deployment afterwards. It keeps
                       holding its accelerator until you delete it yourself.
  --model-timeout <s>  How long to wait for it to reach Running (default 900).
                       Longer than a workspace: a cold AIM image pull is large.
  --with-finetune      Submit a fine-tuning job through the AIWB API, wait for
                       it to complete, check the fine-tuned model it produced
                       is registered in the project, then delete it. Trains on
                       the dataset the dataset check uploads, so it turns
                       --with-dataset on when it is not already on. Needs a
                       free accelerator, so it is off by default and SKIPs
                       when there is none.
  --finetune-model <m> Which base model to fine-tune, by canonical name (e.g.
                       meta-llama/Llama-3.2-1B-Instruct). Default: the
                       smallest one this cluster can actually run — see the
                       Hugging Face note below, which decides that for you.
  --keep-finetune      Do not delete the fine-tuned model (or the dataset it
                       trained on) afterwards
  --finetune-timeout <s>  How long to wait for the job to finish (default
                       1800). A first run downloads the base model weights
                       from Hugging Face, which dominates the time.

  The four --with-* flags above create real resources and delete them again. If the
  script is interrupted in between, one can be left behind and will keep
  holding whatever it reserved; the run says so on its way out. Re-running with
  the same flags adopts and cleans up what it recognises. --with-model deploys
  nothing at all when the model is already deployed and serving — it just
  talks to it.
  --cert-warn-days <n> Warn when a certificate expires within n days (default 14)
  --no-color           Disable coloured output
  -v, --verbose        Show extra detail
  -h, --help           Show this help

Cluster access:
  KUBECONFIG / the current context must already work (admin kubeconfig).

The Resource Manager application kubeconfig is still downloaded and tested as
a check of its own — EAI-5860 asks for that path to be exercised — but it is
no longer a way in.

Credentials — environment always wins; whatever is left is read from the
cluster when the current context works, so no setup is needed:
  SMOKETEST_DOMAIN   cluster domain                 (or --domain)
  SMOKETEST_USER     default devuser@<domain>
  SMOKETEST_PASS     devuser password               (secret airm-user-credentials)
  KEYCLOAK_CLIENT_ID / KEYCLOAK_CLIENT_SECRET       (deploy/airm-ui KEYCLOAK_ID,
                     secret airm-keycloak-ui-creds)
  SMOKETEST_PROJECT  AIWB project for --with-workspace / --with-dataset /
                     --with-model / --with-finetune
  SMOKETEST_HF_TOKEN Hugging Face token, for --with-model only (HF_TOKEN is
                     also honoured). Optional: without one the check deploys
                     an open model instead of a gated one, and only skips if
                     the catalog has no open model at all. The platform does
                     not supply a token of its own for inference.
                     (default: the first project visible to the user)

Hugging Face and --with-finetune. The fine-tuning catalog is small, and on
every environment seen so far its only ungated base models are the large ones
— so a token is the difference between a job that finishes in a couple of
minutes and one that takes ten or more. The check works either way and says
which it picked. Unlike the inference API, the fine-tuning API takes the name
of a Secret in the project namespace rather than the token itself, so:
  --hf-token <t>       Use this token: creates a temporary Secret with the
                       display name smoketest-hf-token-* in the project,
                       through the Workbench secrets API, and deletes it at
                       the end. Also settable as SMOKETEST_HF_TOKEN /
                       HF_TOKEN, which --with-model already uses.
  --hf-token-file <p>  The same, read from a file. A token given on the
                       command line is visible in `ps` and in shell history;
                       this is the way that is not.
  With no token at all, a Secret the project already classifies as a Hugging
  Face one is used if there is one, and failing that the check falls back to
  an ungated base model and says what that costs.

  Through the application kubeconfig, devuser can read nodes, pods, secrets,
  namespaces, storageclasses and httproutes; ArgoCD Applications and Longhorn
  volumes are forbidden. Section 1 asserts exactly that restriction.
USAGE
}

# ---------------------------------------------------------------------------
while [ $# -gt 0 ]; do
    case "$1" in
        --domain)      DOMAIN="$2"; shift 2 ;;
        --domain=*)    DOMAIN="${1#*=}"; shift ;;
        --insecure)    INSECURE=1; shift ;;
        --skip-login)  SKIP_LOGIN=1; shift ;;
        --skip-certs)  SKIP_CERTS=1; shift ;;
        --skip-rm)     SKIP_RM=1; shift ;;
        --with-workspace)   WITH_WORKSPACE=1; shift ;;
        --with-dataset)     WITH_DATASET=1; shift ;;
        --keep-dataset)     KEEP_DATASET=1; shift ;;
        --workspace-type)   WORKSPACE_TYPE="$2"; shift 2 ;;
        --workspace-type=*) WORKSPACE_TYPE="${1#*=}"; shift ;;
        --keep-workspace)   KEEP_WORKSPACE=1; shift ;;
        --workspace-timeout)   WORKSPACE_TIMEOUT="$2"; shift 2 ;;
        --workspace-timeout=*) WORKSPACE_TIMEOUT="${1#*=}"; shift ;;
        --with-model)      WITH_MODEL=1; shift ;;
        --keep-model)      KEEP_MODEL=1; shift ;;
        --model)           MODEL_AIM_ID="$2"; MODEL_EXPLICIT=1; shift 2 ;;
        --model=*)         MODEL_AIM_ID="${1#*=}"; MODEL_EXPLICIT=1; shift ;;
        --model-timeout)   MODEL_TIMEOUT="$2"; shift 2 ;;
        --model-timeout=*) MODEL_TIMEOUT="${1#*=}"; shift ;;
        --with-finetune)   WITH_FINETUNE=1; shift ;;
        --keep-finetune)   KEEP_FINETUNE=1; shift ;;
        --finetune-model)     FINETUNE_MODEL="$2"; shift 2 ;;
        --finetune-model=*)   FINETUNE_MODEL="${1#*=}"; shift ;;
        --finetune-timeout)   FINETUNE_TIMEOUT="$2"; shift 2 ;;
        --finetune-timeout=*) FINETUNE_TIMEOUT="${1#*=}"; shift ;;
        --hf-token)        SMOKETEST_HF_TOKEN="$2"; shift 2 ;;
        --hf-token=*)      SMOKETEST_HF_TOKEN="${1#*=}"; shift ;;
        --hf-token-file)   HF_TOKEN_FILE="$2"; shift 2 ;;
        --hf-token-file=*) HF_TOKEN_FILE="${1#*=}"; shift ;;
        --cert-warn-days)   CERT_WARN_DAYS="$2"; shift 2 ;;
        --cert-warn-days=*) CERT_WARN_DAYS="${1#*=}"; shift ;;
        --no-color)    USE_COLOR=0; shift ;;
        -v|--verbose)  VERBOSE=1; shift ;;
        -h|--help)     usage; exit 0 ;;
        *)             echo "Unknown option: $1" >&2; usage; exit 2 ;;
    esac
done

WORKSPACE_TYPE=$(printf '%s' "$WORKSPACE_TYPE" | tr 'A-Z' 'a-z')
case "$WORKSPACE_TYPE" in
    mlflow|vscode|jupyterlab|comfyui) ;;
    *) echo "Unknown --workspace-type: $WORKSPACE_TYPE (expected mlflow, vscode, jupyterlab or comfyui)" >&2; exit 2 ;;
esac

# Timeouts feed [ x -lt y ] inside the poll loops; a non-numeric value makes
# every comparison error out and the loop spin with no sleep at all.
for _t in WORKSPACE_TIMEOUT MODEL_TIMEOUT FINETUNE_TIMEOUT CERT_WARN_DAYS HTTP_TIMEOUT KUBE_TIMEOUT; do
    eval "_v=\$$_t"
    case "$_v" in
        ''|*[!0-9]*) echo "Invalid ${_t}: '${_v}' (expected a whole number of seconds)" >&2; exit 2 ;;
    esac
done
unset _t _v

# A token in a file beats one in the environment or on the command line, and
# is the only form that does not end up in `ps` or in a shell history.
if [ -n "$HF_TOKEN_FILE" ]; then
    if [ ! -r "$HF_TOKEN_FILE" ]; then
        echo "Cannot read --hf-token-file: $HF_TOKEN_FILE" >&2; exit 2
    fi
    # Trailing newlines are what a file written by `echo` or an editor has;
    # sending one to Hugging Face makes the header invalid for no good reason.
    SMOKETEST_HF_TOKEN=$(tr -d ' \t\r\n' < "$HF_TOKEN_FILE")
    [ -z "$SMOKETEST_HF_TOKEN" ] && { echo "--hf-token-file ${HF_TOKEN_FILE} is empty" >&2; exit 2; }
fi

# Fine-tuning trains on a dataset in the project, and the dataset check is what
# puts one there. Turning it on quietly would be worse than saying so, but
# refusing to run because a second flag was not typed would be worse still.
FINETUNE_FORCED_DATASET=0
if [ "$WITH_FINETUNE" = "1" ] && [ "$WITH_DATASET" != "1" ]; then
    WITH_DATASET=1
    FINETUNE_FORCED_DATASET=1
fi



# ---------------------------------------------------------------------------
# Cleanup of what the --with-* checks create
#
# The substrate layer creates nothing, so this lives here rather than in the
# library and is wired in through SMOKE_CLEANUP_HOOK.
# ---------------------------------------------------------------------------

# Anything this script creates and has not yet deleted is recorded in a global
# so an interrupt can still take it away. Raw curl, not the curl_* helpers:
# those set globals and frame output, and a trap handler runs at an arbitrary
# point in some other check.
AIM_ID=""
AIM_KEY_ID=""
# Fine-tuning leaves more behind than the other checks: a running job holding
# an accelerator, the model it registered on the way out, the dataset it
# trained on (whose delete the dataset check defers so it outlives the job)
# and the Secret the token was written into. They come off in that order —
# cancelling the job first stops it producing anything new to clean up.
FT_JOB_ID=""
FT_MODEL_ID=""
FT_DATASET_ID=""
FT_SECRET=""
DS_ID=""
platform_cleanup() {
    if [ -n "$AIM_ID" ] && [ -n "$AIWB_API" ] && [ -n "$RM_TOKEN" ] && [ -n "$AIWB_PROJECT" ]; then
        printf '\n  cleaning up inference deployment %s ...\n' "$AIM_ID" >&2
        # The token may well have aged out during the wait that was just
        # interrupted; a 401 here would leave the accelerator held.
        rm_token_refresh
        curl -s -o /dev/null --max-time 30 $([ "$INSECURE" = "1" ] && printf '%s' -k) \
             -X DELETE -H "Authorization: Bearer ${RM_TOKEN}" \
             "${AIWB_API}/v1/projects/${AIWB_PROJECT}/inference/${AIM_ID}" 2>/dev/null
        AIM_ID=""
    fi
    # A key outlives the run it was made for unless it is revoked; the ttl is
    # the backstop, not the plan.
    if [ -n "$AIM_KEY_ID" ] && [ -n "$AIWB_API" ] && [ -n "$RM_TOKEN" ] && [ -n "$AIWB_PROJECT" ]; then
        curl -s -o /dev/null --max-time 30 $([ "$INSECURE" = "1" ] && printf '%s' -k) \
             -X DELETE -H "Authorization: Bearer ${RM_TOKEN}" \
             "${AIWB_API}/v1/projects/${AIWB_PROJECT}/api-keys/${AIM_KEY_ID}" 2>/dev/null
        AIM_KEY_ID=""
    fi
    if [ -n "$AIWB_API" ] && [ -n "$RM_TOKEN" ] && [ -n "$AIWB_PROJECT" ]; then
        if [ -n "$FT_JOB_ID" ] || [ -n "$FT_MODEL_ID" ] || [ -n "$DS_ID" ]; then
            printf '\n  cleaning up fine-tuning resources ...\n' >&2
            rm_token_refresh
        fi
        if [ -n "$FT_JOB_ID" ]; then
            curl -s -o /dev/null --max-time 30 $([ "$INSECURE" = "1" ] && printf '%s' -k) \
                 -X DELETE -H "Authorization: Bearer ${RM_TOKEN}" \
                 "${AIWB_API}/v1/projects/${AIWB_PROJECT}/fine-tuning/jobs/${FT_JOB_ID}" 2>/dev/null
            FT_JOB_ID=""
        fi
        if [ -n "$FT_MODEL_ID" ]; then
            # force: a model with an active deployment refuses a plain delete,
            # and a trap has no way to unwind that first.
            curl -s -o /dev/null --max-time 30 $([ "$INSECURE" = "1" ] && printf '%s' -k) \
                 -X DELETE -H "Authorization: Bearer ${RM_TOKEN}" \
                 "${AIWB_API}/v1/projects/${AIWB_PROJECT}/fine-tuning/models/${FT_MODEL_ID}?force=true" 2>/dev/null
            FT_MODEL_ID=""
        fi
        if [ -n "$DS_ID" ]; then
            curl -s -o /dev/null --max-time 30 $([ "$INSECURE" = "1" ] && printf '%s' -k) \
                 -X DELETE -H "Authorization: Bearer ${RM_TOKEN}" \
                 "${AIWB_API}/v1/projects/${AIWB_PROJECT}/datasets/${DS_ID}" 2>/dev/null
            DS_ID=""
        fi
        # It holds a Hugging Face token, so leaving it behind is not neutral.
        if [ -n "$FT_SECRET" ]; then
            curl -s -o /dev/null --max-time 30 $([ "$INSECURE" = "1" ] && printf '%s' -k) \
                 -X DELETE -H "Authorization: Bearer ${RM_TOKEN}" \
                 "${AIWB_API}/v1/projects/${AIWB_PROJECT}/secrets/${FT_SECRET}" 2>/dev/null
            FT_SECRET=""
        fi
    fi
    return 0
}

# Cleanup only knows about what it recorded, and it can itself be cut short by
# a second signal, so an interrupted run says plainly that it may not have
# tidied everything rather than leaving that to be discovered later.
platform_interrupt_notice() {
    if [ "$WITH_MODEL" = "1" ] || [ "$WITH_WORKSPACE" = "1" ] || [ "$WITH_DATASET" = "1" ] || [ "$WITH_FINETUNE" = "1" ]; then
        printf '  This run creates resources. Stopping between a create and its delete can\n' >&2
        printf '  leave one behind, and a model deployment goes on holding its accelerator\n' >&2
        printf '  until it is removed. Look for anything named cluster-smoketest* in\n' >&2
        printf '  project %s — in the AIWB workloads list, and for a model:\n' "${AIWB_PROJECT:-<project>}" >&2
        printf '    kubectl get aimservices -n %s\n' "${AIWB_PROJECT:-<project>}" >&2
        if [ "$WITH_FINETUNE" = "1" ]; then
            printf '  A fine-tuning job runs as a Job and holds its accelerator to the end:\n' >&2
            printf '    kubectl get jobs -n %s\n' "${AIWB_PROJECT:-<project>}" >&2
        fi
        printf '  A later run with the same flags adopts and deletes what it recognises.\n' >&2
    fi
    return 0
}

SMOKE_CLEANUP_HOOK="platform_cleanup"
SMOKE_INTERRUPT_HOOK="platform_interrupt_notice"

smoke_init_output
smoke_install_traps
smoke_preflight

# ---------------------------------------------------------------------------
# Cluster access
# ---------------------------------------------------------------------------
section "Cluster access"

# ---------------------------------------------------------------------------
# Resource Manager application kubeconfig
#
# EAI-5860 asks for the *application* kubeconfig path to be exercised: download
# it from Resource Manager, authenticate as devuser, and prove it works with
# kubectl. That is a test in its own right — see run_section_rm below. It is
# not a way into the cluster: this script needs a working admin context.
#
# These functions must be called as plain statements, never inside $( ): a
# global assigned in a command substitution is lost with the subshell.
# ---------------------------------------------------------------------------
RM_DOMAIN=""; RM_USER=""; RM_PASS=""; RM_CID=""; RM_CSEC=""
RM_CRED_ERR=""; RM_CRED_SRC=""
RM_TOKEN=""; RM_TOKEN_EXP=0; RM_CLUSTER_ID=""; RM_CLUSTER_NAME=""; RM_CLUSTER_STATUS=""; RM_CLUSTER_NODES=""
RM_KUBECONFIG=""; RM_SERVER=""; RM_ID_TOKEN=""
RM_STEPS=()

# Record one line of the RM story. Printed later by rm_report, so the steps
# appear inside their own section even when the work happened during access.
rm_step() { RM_STEPS+=("$1|$2"); }

rm_report() {
    local entry
    for entry in "${RM_STEPS[@]}"; do
        case "${entry%%|*}" in
            ok)   ok   "${entry#*|}" ;;
            warn) warn "${entry#*|}" ;;
            skip)    skip    "${entry#*|}" ;;
            blocked) blocked "${entry#*|}" ;;
            *)       info    "${entry#*|}" ;;
        esac
    done
}

# Fill in user/password/client. Environment always wins; anything missing is
# read from the cluster when a working context exists, so no setup is needed.
rm_creds_resolve() {
    RM_CRED_ERR=""
    local d="${DOMAIN:-${SMOKETEST_DOMAIN:-}}"
    # Discovery has normally run by now, but the domain is still unset when it
    # was neither supplied nor discoverable. Read it the way Discovery would.
    if [ -z "$d" ] && [ "$CTX_OK" = "1" ]; then
        d=$(kubectl --request-timeout="${KUBE_TIMEOUT}s" get gateway https -n "$NS_ENVOY" \
              -o jsonpath='{.spec.listeners[0].hostname}' 2>/dev/null)
        d="${d#\*.}"
        # cluster-bloom writes ConfigMap cluster-domain in namespace default
        # (deploy_k8s_apps/domain.yaml) and repeats DOMAIN in ConfigMap bloom
        # in the same namespace (bloom_config.yaml). Neither is "default in
        # namespace bloom", which is what this used to ask for — so the
        # fallback never fired.
        [ -z "$d" ] && d=$(kubectl --request-timeout="${KUBE_TIMEOUT}s" get configmap cluster-domain -n default \
              -o jsonpath='{.data.DOMAIN}' 2>/dev/null)
        [ -z "$d" ] && d=$(kubectl --request-timeout="${KUBE_TIMEOUT}s" get configmap bloom -n default \
              -o jsonpath='{.data.DOMAIN}' 2>/dev/null)
    fi
    if [ -z "$d" ]; then
        RM_CRED_ERR="no domain known — set SMOKETEST_DOMAIN or pass --domain"
        return 1
    fi
    RM_DOMAIN="$d"
    # Publish it: even if the Resource Manager path fails, the HTTPS-only checks
    # (Keycloak realm, platform logins) can still run against this domain.
    [ -z "$DOMAIN" ] && DOMAIN="$d"
    RM_USER="${SMOKETEST_USER:-devuser@${d}}"
    RM_PASS="${SMOKETEST_PASS:-}"
    RM_CID="${KEYCLOAK_CLIENT_ID:-}"
    RM_CSEC="${KEYCLOAK_CLIENT_SECRET:-}"
    RM_CRED_SRC="environment"

    if [ "$CTX_OK" = "1" ]; then
        local from_cluster=0
        if [ -z "$RM_CID" ]; then
            RM_CID=$(kubectl --request-timeout="${KUBE_TIMEOUT}s" get deploy airm-ui -n airm \
                       -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="KEYCLOAK_ID")].value}' 2>/dev/null)
            [ -z "$RM_CID" ] && RM_CID=$(kubectl --request-timeout="${KUBE_TIMEOUT}s" get deploy airm -n airm \
                       -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="OPENID_CLIENT_ID")].value}' 2>/dev/null)
            [ -n "$RM_CID" ] && from_cluster=1
        fi
        if [ -z "$RM_CSEC" ]; then
            RM_CSEC=$(admin_secret_value airm airm-keycloak-ui-creds KEYCLOAK_SECRET)
            [ -z "$RM_CSEC" ] && RM_CSEC=$(admin_secret_value keycloak airm-realm-credentials FRONTEND_CLIENT_SECRET)
            [ -n "$RM_CSEC" ] && from_cluster=1
        fi
        if [ -z "$RM_PASS" ]; then
            RM_PASS=$(admin_secret_value airm airm-user-credentials USER_PASSWORD)
            [ -z "$RM_PASS" ] && RM_PASS=$(admin_secret_value keycloak airm-realm-credentials KEYCLOAK_INITIAL_DEVUSER_PASSWORD)
            [ -n "$RM_PASS" ] && from_cluster=1
        fi
        [ "$from_cluster" = "1" ] && RM_CRED_SRC="cluster"
    fi

    local missing=""
    [ -z "$RM_PASS" ] && missing="${missing} SMOKETEST_PASS"
    [ -z "$RM_CID" ]  && missing="${missing} KEYCLOAK_CLIENT_ID"
    [ -z "$RM_CSEC" ] && missing="${missing} KEYCLOAK_CLIENT_SECRET"
    if [ -n "$missing" ]; then
        RM_CRED_ERR="missing credentials:${missing}"
        return 1
    fi
    return 0
}

# Keycloak access tokens are short-lived — 300s on a default realm — and the
# model poll alone can outlast that several times over. Once the token ages
# out every call 401s: the poll reads no status and the cleanup DELETE cannot
# remove what it created, so an accelerator stays held. Re-mint before that
# happens, taking the expiry from the token itself rather than assuming a
# lifespan, and falling back to a conservative age if it will not decode.
#
# Assigns globals, so it must be called as a plain statement — inside $( ) the
# refreshed token dies with the subshell and the caller keeps the stale one.
rm_token_refresh() {
    [ -n "$RM_TOKEN" ] || return 1
    local now; now=$(date +%s)
    [ "$now" -lt "$(( RM_TOKEN_EXP - 60 ))" ] && return 0
    local t
    t=$(curl_body "https://kc.${RM_DOMAIN}/realms/${KEYCLOAK_REALM:-airm}/protocol/openid-connect/token" \
          -d grant_type=password -d scope=openid \
          -d "client_id=${RM_CID}" -d "client_secret=${RM_CSEC}" \
          -d "username=${RM_USER}" -d "password=${RM_PASS}" | jq -r '.access_token // empty')
    [ -z "$t" ] && return 1
    RM_TOKEN="$t"
    rm_token_expiry_set
    return 0
}

# exp out of the JWT payload. Base64url, and the padding has to be put back by
# hand or base64 refuses the string.
rm_token_expiry_set() {
    local pay exp
    pay=$(printf '%s' "$RM_TOKEN" | cut -d. -f2 | tr '_-' '/+')
    case $(( ${#pay} % 4 )) in 2) pay="${pay}==" ;; 3) pay="${pay}=" ;; esac
    exp=$(printf '%s' "$pay" | base64 -d 2>/dev/null | jq -r '.exp // empty' 2>/dev/null)
    case "$exp" in
        ''|*[!0-9]*) RM_TOKEN_EXP=$(( $(date +%s) + 240 )) ;;
        *)           RM_TOKEN_EXP="$exp" ;;
    esac
}

# Keycloak → Resource Manager → kubeconfig → id_token. Records each step.
rm_fetch_kubeconfig() {
    local realm="${KEYCLOAK_REALM:-airm}"
    local kc_url="https://kc.${RM_DOMAIN}"
    local api_url="https://airmapi.${RM_DOMAIN}"

    # Fetched to a file rather than through $( ) so CURL_RC and CURL_CODE
    # survive: without them a refused connection and a rejected password are
    # the same empty string, and the check would name a cause it never tested.
    local tok_body="${TMPDIR_SMOKE}/rm_token.json"
    curl_to_file "${kc_url}/realms/${realm}/protocol/openid-connect/token" "$tok_body" \
        -d grant_type=password -d scope=openid \
        -d "client_id=${RM_CID}" -d "client_secret=${RM_CSEC}" \
        -d "username=${RM_USER}" -d "password=${RM_PASS}"
    RM_TOKEN=$(jq -r '.access_token // empty' "$tok_body" 2>/dev/null)
    if [ -z "$RM_TOKEN" ]; then
        if [ "$CURL_RC" -ne 0 ]; then
            # Nothing answered. Say only that — the credentials were never
            # put to Keycloak, so nothing here is evidence about them.
            rm_step warn "Keycloak unreachable at ${kc_url} — $(curl_rc_reason "$CURL_RC")"
        else
            # Keycloak answered and refused. Its own error is worth more than
            # a guess, so lead with it and keep the guess as context.
            local kc_err
            kc_err=$(jq -r '(.error_description // .error // empty)' "$tok_body" 2>/dev/null | head -c 200)
            if [ -n "$kc_err" ]; then
                rm_step warn "Keycloak rejected the password grant for ${RM_USER} (HTTP ${CURL_CODE}) — ${kc_err}"
            else
                rm_step warn "Keycloak password grant failed for ${RM_USER} (HTTP ${CURL_CODE}) — wrong password, or Direct Access Grants disabled on client ${RM_CID}"
            fi
        fi
        return 1
    fi
    rm_token_expiry_set
    rm_step ok "authenticated ${RM_USER} against ${kc_url}/realms/${realm}"

    local clusters
    clusters=$(curl_body "${api_url}/v1/clusters" -H "Authorization: Bearer ${RM_TOKEN}")
    RM_CLUSTER_ID=$(printf '%s' "$clusters" | jq -r '[(.data // [])[] | select(.status=="healthy")][0].id // empty' 2>/dev/null)
    [ -z "$RM_CLUSTER_ID" ] && RM_CLUSTER_ID=$(printf '%s' "$clusters" | jq -r '(.data // [])[0].id // empty' 2>/dev/null)
    if [ -z "$RM_CLUSTER_ID" ]; then
        rm_step warn "Resource Manager returned no cluster from ${api_url}/v1/clusters: $(printf '%s' "$clusters" | jq -r '.detail // .message // "empty response"' 2>/dev/null | head -1)"
        return 1
    fi
    RM_CLUSTER_NAME=$(printf '%s'   "$clusters" | jq -r --arg i "$RM_CLUSTER_ID" '(.data // [])[] | select(.id==$i) | .name // empty')
    RM_CLUSTER_STATUS=$(printf '%s' "$clusters" | jq -r --arg i "$RM_CLUSTER_ID" '(.data // [])[] | select(.id==$i) | .status // empty')
    RM_CLUSTER_NODES=$(printf '%s'  "$clusters" | jq -r --arg i "$RM_CLUSTER_ID" '(.data // [])[] | select(.id==$i) | .totalNodeCount // empty')
    if [ "$RM_CLUSTER_STATUS" = "healthy" ]; then
        rm_step ok "Resource Manager knows cluster '${RM_CLUSTER_NAME}' — status ${RM_CLUSTER_STATUS}, ${RM_CLUSTER_NODES} node(s)"
    else
        rm_step warn "Resource Manager reports cluster '${RM_CLUSTER_NAME}' as ${RM_CLUSTER_STATUS:-unknown}"
    fi

    RM_KUBECONFIG="${TMPDIR_SMOKE}/rm-kubeconfig.yaml"
    curl_body "${api_url}/v1/clusters/${RM_CLUSTER_ID}/kube-config" \
        -H "Authorization: Bearer ${RM_TOKEN}" | jq -r '.kubeConfig // empty' > "$RM_KUBECONFIG"
    chmod 600 "$RM_KUBECONFIG" 2>/dev/null
    if [ ! -s "$RM_KUBECONFIG" ]; then
        rm_step warn "kubeconfig download from ${api_url}/v1/clusters/${RM_CLUSTER_ID}/kube-config returned nothing"
        return 1
    fi
    rm_step ok "application kubeconfig downloaded ($(wc -c < "$RM_KUBECONFIG" | tr -d ' ') bytes)"

    RM_SERVER=$(kubectl --kubeconfig="$RM_KUBECONFIG" config view --raw -o json 2>/dev/null \
                  | jq -r '.clusters[0].cluster.server // empty')
    if [ -z "$RM_SERVER" ]; then
        rm_step warn "downloaded kubeconfig has no API server address — not a usable kubeconfig"
        return 1
    fi
    if grep -q 'oidc-login' "$RM_KUBECONFIG" && grep -q -- '--oidc-client-id=k8s' "$RM_KUBECONFIG"; then
        rm_step ok "kubeconfig points at ${RM_SERVER} and uses the k8s OIDC client"
    else
        rm_step warn "kubeconfig points at ${RM_SERVER} but has no kubectl oidc-login exec plugin for client k8s"
    fi
    # Resource Manager issues this kubeconfig with TLS verification switched off
    # and no CA bundle. Worth stating; it is RM's choice, not the script's.
    if ! grep -q 'certificate-authority-data' "$RM_KUBECONFIG" && grep -q 'insecure-skip-tls-verify: *true' "$RM_KUBECONFIG"; then
        rm_step info "Resource Manager issues it with insecure-skip-tls-verify: true and no CA bundle"
    fi

    # The exec plugin wants a browser. Mint the id_token directly with the
    # client secret embedded in the kubeconfig — same token, no kubelogin.
    local k8s_secret
    k8s_secret=$(grep -o -- '--oidc-client-secret=[^[:space:]"]*' "$RM_KUBECONFIG" | head -1 | cut -d= -f2-)
    if [ -z "$k8s_secret" ]; then
        rm_step warn "no --oidc-client-secret in the kubeconfig — cannot authenticate without a browser"
        return 1
    fi
    RM_ID_TOKEN=$(curl_body "${kc_url}/realms/${realm}/protocol/openid-connect/token" \
                    -d grant_type=password -d scope=openid \
                    -d "client_id=k8s" -d "client_secret=${k8s_secret}" \
                    -d "username=${RM_USER}" -d "password=${RM_PASS}" | jq -r '.id_token // empty')
    if [ -z "$RM_ID_TOKEN" ]; then
        rm_step warn "could not obtain an id_token for client_id=k8s"
        return 1
    fi
    rm_step ok "id_token issued for client k8s"
    return 0
}

# kubectl through the downloaded kubeconfig. --token bypasses the exec plugin;
# everything else (server, TLS settings) comes from the file as RM wrote it.
rm_kubectl() {
    kubectl --kubeconfig="$RM_KUBECONFIG" --token="$RM_ID_TOKEN" \
            --request-timeout="${KUBE_TIMEOUT}s" "$@"
}

# Run everything the RM path needs, in order. Returns non-zero if the chain
# broke; RM_STEPS then holds the reason.
rm_resolve() {
    if ! rm_creds_resolve; then
        rm_step blocked "Resource Manager kubeconfig — ${RM_CRED_ERR}"
        return 1
    fi
    rm_fetch_kubeconfig
}

# ---------------------------------------------------------------------------
# Cluster access
#
# An admin context is the only way in. Without one every cluster check is
# skipped, but the run still continues: the host-level probes (Keycloak,
# OpenBao, the login checks, TLS) need only a domain and reach the cluster
# over HTTPS, so they remain meaningful.
# ---------------------------------------------------------------------------
CTX_OK=0
# One call is enough to prove the context works — `kubectl version` would
# contact the same endpoint again for no extra information.
if kubectl --request-timeout=10s get --raw='/version' >/dev/null 2>&1; then
    CTX_OK=1
    ok "using existing kubectl context: $(kubectl config current-context 2>/dev/null || echo 'n/a')"
else
    warn "no usable kubectl context — all cluster checks will be skipped"
fi

# ---------------------------------------------------------------------------
# The Resource Manager section itself: run the whole chain as a test, then
# prove the kubeconfig it produced actually works.
# ---------------------------------------------------------------------------
run_section_rm() {
    section_n "Resource Manager kubeconfig"

    if [ "$SKIP_RM" = "1" ]; then
        skip "Resource Manager kubeconfig checks (--skip-rm)"
        return 0
    fi

    RM_STEPS=()
    rm_resolve

    if [ ${#RM_STEPS[@]} -eq 0 ]; then
        blocked "Resource Manager kubeconfig — no credentials and no cluster to read them from"
        return 0
    fi
    rm_report
    [ -z "$RM_ID_TOKEN" ] && return 0

    if [ "$RM_CRED_SRC" = "cluster" ]; then
        vinfo "credentials read from the cluster (deploy/airm-ui, secret airm-keycloak-ui-creds, secret airm-user-credentials)"
    fi

    # The acceptance step from the ticket: does the downloaded kubeconfig work?
    local nodes_json rm_nodes
    nodes_json=$(rm_kubectl get nodes -o json 2>"${TMPDIR_SMOKE}/rm_nodes_err")
    if [ $? -ne 0 ] || [ -z "$nodes_json" ]; then
        warn "kubectl get nodes with the application kubeconfig failed: $(head -1 "${TMPDIR_SMOKE}/rm_nodes_err")"
        return 0
    fi
    rm_nodes=$(printf '%s' "$nodes_json" | jq '[.items[]?] | length')
    ok "kubectl get nodes works with the application kubeconfig — ${rm_nodes} node(s)"

    if [ -n "$RM_CLUSTER_NODES" ] && [ "$rm_nodes" != "$RM_CLUSTER_NODES" ]; then
        warn "node count disagrees — Resource Manager reports ${RM_CLUSTER_NODES}, the cluster returns ${rm_nodes}"
    fi
    if [ -n "${TOTAL_NODES:-}" ] && [ "$rm_nodes" != "$TOTAL_NODES" ]; then
        warn "the application kubeconfig sees ${rm_nodes} node(s) but the admin context sees ${TOTAL_NODES} — different clusters?"
    fi

    # Who does the API server think we are?
    local whoami
    whoami=$(rm_kubectl auth whoami -o json 2>/dev/null | jq -r '.status.userInfo.username // empty')
    if [ -z "$whoami" ]; then
        vinfo "kubectl auth whoami unavailable — skipping the identity check"
    elif printf '%s' "$whoami" | grep -qF -- "$RM_USER"; then
        ok "authenticated to the API server as ${whoami}"
        vinfo "groups: $(rm_kubectl auth whoami -o json 2>/dev/null | jq -r '[.status.userInfo.groups[]?] | join(" ")')"
    else
        warn "expected the API server to see ${RM_USER}, it sees ${whoami}"
    fi

    # An application kubeconfig must NOT be able to read ArgoCD Applications.
    # Forbidden is the pass here; success would mean it is over-privileged.
    local probe
    probe=$(rm_kubectl get applications.argoproj.io -n "$NS_ARGOCD" 2>&1)
    if [ $? -ne 0 ]; then
        if printf '%s' "$probe" | grep -qi 'forbidden'; then
            ok "application kubeconfig is correctly restricted (ArgoCD Applications forbidden)"
        else
            vinfo "scope probe inconclusive: $(printf '%s' "$probe" | head -1)"
        fi
    else
        warn "application kubeconfig can read ArgoCD Applications — wider permissions than airm-platform-admin should grant"
    fi
}

CLUSTER_OK=0
if [ "$CTX_OK" = "1" ]; then
    VERSION_RAW=$(kc get --raw='/version' 2>/dev/null)
    if [ -n "$VERSION_RAW" ]; then
        CLUSTER_OK=1
        SRV_VER=$(printf '%s' "$VERSION_RAW" | jq -r '.gitVersion // empty')
        [ -n "$SRV_VER" ] && info "API server: ${SRV_VER}"
    else
        warn "cluster reachable check failed — cluster checks will be skipped"
    fi
fi

# ---------------------------------------------------------------------------
# Discovery — cluster-forge identity, domain and hostnames
#
# The substrate script reads the bloom ConfigMaps; this one reads what forge
# publishes. cluster_size is still needed, to tell "Longhorn UI missing" from
# "small cluster, no Longhorn by design".
# ---------------------------------------------------------------------------
section "Discovery"

if [ "$CLUSTER_OK" = "1" ]; then
    # Node count, for the Resource Manager cross-check below: the application
    # kubeconfig should see exactly the same cluster the admin context does.
    TOTAL_NODES=$(kc_json_cached get nodes 2>/dev/null | jq '[.items[]?] | length' 2>/dev/null)

    BLOOM_CM=$(kc_json get configmap bloom -n default 2>/dev/null)
    CLUSTER_SIZE=$(printf '%s' "$BLOOM_CM" | jq -r '.data.cluster_size // empty' 2>/dev/null | tr '[:upper:]' '[:lower:]')
    BLOOM_DOMAIN=$(printf '%s' "$BLOOM_CM" | jq -r '.data.DOMAIN // empty' 2>/dev/null)
    [ -n "$CLUSTER_SIZE" ] && info "cluster size: ${CLUSTER_SIZE} (from ConfigMap bloom/default)"

    # use-cert-manager decides whether a real certificate is expected at all or
    # whether the Gateway is serving cluster-bloom's self-signed one, which the
    # TLS section would otherwise report as untrusted every single run.
    USE_CERT_MANAGER=$(kc_json get configmap cluster-domain -n default 2>/dev/null \
                       | jq -r '.data["use-cert-manager"] // empty' 2>/dev/null)

    # cluster-forge version — the root Application's targetRevision is what was
    # actually deployed. Note it is a MULTI-SOURCE Application (spec.sources), so
    # spec.source is null and must not be read on its own.
    FORGE_JSON=$(kc_json get application cluster-forge -n "$NS_ARGOCD" 2>/dev/null)
    if [ $? -eq 0 ]; then
        FORGE_VERSION=$(printf '%s' "$FORGE_JSON" | jq -r '
            [ (.spec.sources // [.spec.source // empty])[]?
              | select((.repoURL // "") | test("cluster-forge")) | .targetRevision ][0] // empty' 2>/dev/null)
        CLUSTER_VALUES_REV=$(printf '%s' "$FORGE_JSON" | jq -r '
            [ (.spec.sources // [])[]?
              | select((.repoURL // "") | test("cluster-values")) | .targetRevision ][0] // empty' 2>/dev/null)
        FORGE_COMMIT=$(printf '%s' "$FORGE_JSON" | jq -r '
            (.status.sync.revisions // [.status.sync.revision // empty])[0] // empty' 2>/dev/null | cut -c1-7)
    fi

    # Fallback: the most common targetRevision among Applications rendered from
    # the cluster-forge repo.
    if [ -z "$FORGE_VERSION" ]; then
        FORGE_VERSION=$(kc_json_cached get applications.argoproj.io -n "$NS_ARGOCD" 2>/dev/null | jq -r '
            [ .items[]? | (.spec.sources // [.spec.source // empty])[]?
              | select((.repoURL // "") | test("cluster-forge")) | .targetRevision ]
            | group_by(.) | max_by(length) | .[0] // empty' 2>/dev/null)
        [ -n "$FORGE_VERSION" ] && FORGE_COMMIT=""
    fi

    if [ -n "$FORGE_VERSION" ]; then
        ok "cluster-forge: ${FORGE_VERSION}${FORGE_COMMIT:+ (commit ${FORGE_COMMIT})}${CLUSTER_VALUES_REV:+ · cluster-values ${CLUSTER_VALUES_REV}}"
    else
        info "cluster-forge version unknown — Application 'cluster-forge' not readable in ns/${NS_ARGOCD}"
    fi

    # A moving ref is worth calling out: re-running this test later tests
    # different code, which matters when validating a release.
    # bloom's own ref is reported by smoketest-cluster.sh, which owns that
    # ConfigMap; only forge's is this layer's business.
    case "$FORGE_VERSION" in
        main|master|HEAD|latest)
            info "cluster-forge is pinned to a moving ref '${FORGE_VERSION}' — not a fixed release" ;;
    esac

    # Apps can also be switched off per-deployment via disabledApps. Handle both
    # single- and multi-source Applications.
    DISABLED_APPS=$(printf '%s' "$FORGE_JSON" | jq -r '
        [ (.spec.sources // [.spec.source // empty])[]?
          | .helm.parameters[]? | select(.name=="disabledApps") | .value ][0] // empty' 2>/dev/null)
    [ -n "$DISABLED_APPS" ] && info "explicitly disabled apps: ${DISABLED_APPS}"

    # Domain from the Gateway listener: "*.<domain>"
    if [ -z "$DOMAIN" ]; then
        GW_HOST=$(kc get gateway https -n "$NS_ENVOY" -o jsonpath='{.spec.listeners[0].hostname}' 2>/dev/null)
        if [ -n "$GW_HOST" ]; then
            DOMAIN="${GW_HOST#\*.}"
            ok "domain discovered from Gateway listener: ${DOMAIN}"
        else
            if [ -n "${BLOOM_DOMAIN:-}" ]; then
                DOMAIN="$BLOOM_DOMAIN"
                ok "domain from ConfigMap bloom/default: ${DOMAIN}"
            else
                warn "could not read domain from Gateway 'https' in ${NS_ENVOY} nor ConfigMap bloom/default — pass --domain to enable HTTP checks"
            fi
        fi
    else
        ok "domain: ${DOMAIN} (supplied)"
    fi

    # Hostnames from HTTPRoute Host header regexes, e.g. "argocd\..*"
    HOST_PREFIXES=$(kc_json get httproute -A 2>/dev/null | jq -r '
        .items[]? | .spec.rules[]?.matches[]?.headers[]?
        | select(.name=="Host") | .value' 2>/dev/null \
        | sed -E 's/\\\.\.\*$//; s/\\\..*$//; s/\..*$//' | sort -u | grep -v '^$')
    if [ -n "$HOST_PREFIXES" ]; then
        ok "HTTPRoute host prefixes discovered: $(printf '%s' "$HOST_PREFIXES" | tr '\n' ' ')"
    else
        info "no HTTPRoute host prefixes discovered — falling back to conventional names"
    fi
else
    blocked "discovery — no cluster access"
fi

# The Resource Manager section runs here: the domain is known by now and the
# credentials can be read from the cluster.
run_section_rm

# ===========================================================================
# ArgoCD applications
# ===========================================================================
section_n "ArgoCD applications"

if [ "$CLUSTER_OK" != "1" ]; then
    blocked "ArgoCD checks — no cluster access"
else
    APPS_JSON=$(kc_json_cached get applications.argoproj.io -n "$NS_ARGOCD")
    if [ $? -ne 0 ]; then
        kc_report_failure "ArgoCD Applications"
    else
        APP_TOTAL=$(printf '%s' "$APPS_JSON" | jq '[.items[]?] | length')
        if [ "${APP_TOTAL:-0}" = "0" ]; then
            warn "no ArgoCD Applications found in ns/${NS_ARGOCD}"
        else
            BAD_APPS=$(printf '%s' "$APPS_JSON" | jq -r '
                .items[]
                | select((.status.health.status // "Unknown") != "Healthy"
                      or (.status.sync.status // "Unknown") != "Synced")
                | "\(.metadata.name) health=\(.status.health.status // "Unknown") sync=\(.status.sync.status // "Unknown")"')
            if [ -z "$BAD_APPS" ]; then
                ok "all ${APP_TOTAL} applications Healthy + Synced"
            else
                warn "$(printf '%s\n' "$BAD_APPS" | grep -c .)/${APP_TOTAL} applications not Healthy/Synced"
                printf '%s\n' "$BAD_APPS" | while read -r l; do [ -n "$l" ] && info "  $l"; done
            fi
        fi
    fi
fi


# ===========================================================================
# Network
#
# CoreDNS and Cilium belong to the substrate and are checked by
# smoketest-cluster.sh. What is left here is what cluster-forge deploys: the
# Envoy Gateway controller and the Gateways themselves.
# ===========================================================================
section_n "Network"

if [ "$CLUSTER_OK" != "1" ]; then
    blocked "network checks — no cluster access"
else
    # Envoy Gateway controller
    check_ns_pods "$NS_ENVOY" "Envoy Gateway"

    # Gateways programmed?
    #
    # Every Gateway is checked, not just 'https'. A single admission webhook can
    # take down one Gateway's data plane while leaving another's intact: a stale
    # envoy-ai-gateway webhook caBundle makes pod CREATE fail in this namespace,
    # so the affected Envoy Deployments sit at 0 replicas and their Gateway goes
    # Programmed=False/NoResources. Checking one hardcoded name reported this
    # section fully green on a cluster with 'ai-gateway' down (chalupa-491a,
    # EAI-8292), which is exactly the state a smoke test exists to catch.
    GW_JSON=$(kc_json get gateway -A); GW_RC=$?
    if [ $GW_RC -ne 0 ] && [ "$(kc_status)" = "forbidden" ]; then
        # A restricted kubeconfig may refuse a cluster-wide list yet still allow
        # the namespace the platform's own Gateways live in.
        GW_JSON=$(kc_json get gateway -n "$NS_ENVOY"); GW_RC=$?
    fi
    if [ $GW_RC -ne 0 ]; then
        kc_report_failure "Gateways"
    elif [ "$(printf '%s' "$GW_JSON" | jq -r '.items | length' 2>/dev/null)" = "0" ]; then
        warn "no Gateways found — the platform routes all ingress through one, so this is not an empty-by-design case"
    else
        # 'first' rather than a bare select so a Gateway that has no Programmed
        # condition at all still produces a row (reported as unknown) instead of
        # silently vanishing from the output.
        while IFS=$'\t' read -r GW_NS GW_NAME GW_PROG GW_REASON GW_MSG GW_ADDR; do
            [ -z "$GW_NAME" ] && continue
            # Namespace is shown only when it is not the expected one, so the
            # common single-namespace output stays as it was.
            GW_LABEL="$GW_NAME"
            [ "$GW_NS" != "$NS_ENVOY" ] && GW_LABEL="${GW_NS}/${GW_NAME}"
            if [ "$GW_PROG" = "True" ]; then
                ok "Gateway '${GW_LABEL}' Programmed=True${GW_ADDR:+ (address ${GW_ADDR})}"
            else
                # reason/message carry the actual diagnosis (NoResources,
                # "Envoy replicas unavailable"), which is what points at the
                # data plane rather than at the Gateway resource itself.
                warn "Gateway '${GW_LABEL}' Programmed=${GW_PROG:-unknown}${GW_REASON:+ reason=${GW_REASON}}${GW_ADDR:+ address ${GW_ADDR}}"
                [ -n "$GW_MSG" ] && info "  ${GW_MSG}"
            fi
        done < <(printf '%s' "$GW_JSON" | jq -r '
            .items
            | sort_by(.metadata.namespace, .metadata.name)[]
            | . as $g
            | ([ $g.status.conditions[]? | select(.type=="Programmed") ] | first) as $c
            | [ $g.metadata.namespace,
                $g.metadata.name,
                ($c.status  // ""),
                ($c.reason  // ""),
                ($c.message // ""),
                ($g.status.addresses[0].value // "") ]
            | @tsv' 2>/dev/null)
    fi
fi


# ===========================================================================
# MetalLB
#
# Split ownership, which is why it is checked here and not in the substrate
# script: cluster-forge deploys the controller and the metallb.io CRDs
# (root/values.yaml), while cluster-bloom writes the IPAddressPool and
# L2Advertisement CRs into RKE2's auto-deploy directory
# (deploy_k8s_apps/metallb.yaml). Without forge the CRDs do not exist, so
# bloom's manifest never applies and there is nothing to check.
#
# The last check is the one that earns its place: an address handed out from
# a pool nobody meant to use still looks Programmed=True on the Gateway.
# ===========================================================================
section_n "MetalLB"

# True when $1 (a.b.c.d) falls inside $2 (a.b.c.d/len). Integer division
# rather than bitwise and(), which POSIX awk does not have.
ip_in_cidr() {
    awk -v ip="$1" -v cidr="$2" 'BEGIN {
        n = index(cidr, "/")
        net  = (n ? substr(cidr, 1, n-1) : cidr)
        bits = (n ? substr(cidr, n+1)    : 32) + 0
        if (bits < 0 || bits > 32) exit 1
        split(ip, a, "."); split(net, b, ".")
        ipn  = a[1]*16777216 + a[2]*65536 + a[3]*256 + a[4]
        netn = b[1]*16777216 + b[2]*65536 + b[3]*256 + b[4]
        d = 2 ^ (32 - bits)
        exit (int(ipn/d) == int(netn/d)) ? 0 : 1
    }'
}

if [ "$CLUSTER_OK" != "1" ]; then
    blocked "MetalLB checks — no cluster access"
elif ! ns_exists "$NS_METALLB"; then
    skip "MetalLB — namespace ${NS_METALLB} not present (component not deployed on this cluster)"
else
    check_ns_pods "$NS_METALLB" "MetalLB"

    POOL_ROWS=""
    POOL_JSON=$(kc_json get ipaddresspools.metallb.io -A 2>/dev/null)
    if [ $? -ne 0 ]; then
        if [ "$(kc_status)" = "notfound" ]; then
            blocked "MetalLB IPAddressPools — metallb.io CRD not present but the namespace is"
        else
            kc_report_failure "MetalLB IPAddressPools"
        fi
    else
        POOL_ROWS=$(printf '%s' "$POOL_JSON" | jq -r '
            .items[]? | .metadata.name as $n | .spec.addresses[]? | "\($n)\t\(.)"' 2>/dev/null)
        if [ -z "$POOL_ROWS" ]; then
            warn "MetalLB is deployed but no IPAddressPool has any address — LoadBalancer Services will stay Pending"
        else
            ok "MetalLB — $(printf '%s\n' "$POOL_ROWS" | grep -c .) address range(s) across $(printf '%s' "$POOL_JSON" | jq '[.items[]?] | length') pool(s)"
            printf '%s\n' "$POOL_ROWS" | while IFS=$'\t' read -r pn pa; do
                [ -n "$pn" ] && vinfo "pool ${pn}: ${pa}"
            done
        fi

        # cluster-bloom's own pool. Absent is worth saying out loud rather than
        # inferring from the count: it means bloom's manifest never applied.
        if printf '%s' "$POOL_JSON" | jq -e '.items[]? | select(.metadata.name=="cluster-bloom-ip-pool")' >/dev/null 2>&1; then
            ok "cluster-bloom-ip-pool present"
        else
            info "no pool named cluster-bloom-ip-pool — addresses come from somewhere else (cluster-values?)"
        fi
    fi

    L2_JSON=$(kc_json get l2advertisements.metallb.io -A 2>/dev/null)
    if [ $? -eq 0 ]; then
        L2_TOTAL=$(printf '%s' "$L2_JSON" | jq '[.items[]?] | length')
        if [ "${L2_TOTAL:-0}" = "0" ]; then
            warn "no L2Advertisement — the pools exist but nothing advertises them on the LAN"
        else
            ok "L2Advertisement present (${L2_TOTAL})"
        fi
    fi

    # Every LoadBalancer Service that has an address should have been given one
    # from a pool — that is precisely MetalLB's job, and it is the check worth
    # having. Gateways are deliberately NOT used for this: a Gateway backed by
    # a ClusterIP Service (ai-gateway, on clusters that have it) carries a
    # service-CIDR address that never came from MetalLB and is not a fault.
    LB_JSON=$(kc_json_cached get svc -A 2>/dev/null); LB_RC=$?
    if [ -n "$POOL_ROWS" ] && [ "$LB_RC" -eq 0 ]; then
        LB_ROWS=$(printf '%s' "$LB_JSON" | jq -r '
            .items[]? | select(.spec.type == "LoadBalancer")
            | "\(.metadata.namespace)/\(.metadata.name)\t\(.status.loadBalancer.ingress[0].ip // "")"' 2>/dev/null)
        if [ -z "$LB_ROWS" ]; then
            info "no LoadBalancer Services — nothing for MetalLB to assign yet"
        else
            LB_PENDING=""; LB_ORPHAN=""; LB_OK=0
            while IFS=$'\t' read -r lname laddr; do
                [ -z "$lname" ] && continue
                if [ -z "$laddr" ]; then
                    LB_PENDING="${LB_PENDING}${lname}"$'\n'
                    continue
                fi
                lfound=""
                while IFS=$'\t' read -r pn pa; do
                    [ -z "$pa" ] && continue
                    if ip_in_cidr "$laddr" "$pa"; then lfound="$pn"; break; fi
                done <<< "$POOL_ROWS"
                if [ -n "$lfound" ]; then
                    LB_OK=$((LB_OK + 1))
                    vinfo "${lname} ${laddr} from pool ${lfound}"
                else
                    LB_ORPHAN="${LB_ORPHAN}${lname} (${laddr})"$'\n'
                fi
            done <<< "$LB_ROWS"

            # No address at all is the classic MetalLB failure: the Service sits
            # Pending forever and whatever fronts it is simply unreachable.
            if [ -n "$LB_PENDING" ]; then
                warn "$(printf '%s' "$LB_PENDING" | grep -c .) LoadBalancer Service(s) have no address — MetalLB has not assigned one"
                printf '%s' "$LB_PENDING" | while read -r l; do [ -n "$l" ] && info "  $l"; done
            fi
            if [ -n "$LB_ORPHAN" ]; then
                warn "$(printf '%s' "$LB_ORPHAN" | grep -c .) LoadBalancer address(es) not in any MetalLB pool"
                printf '%s' "$LB_ORPHAN" | while read -r l; do [ -n "$l" ] && info "  $l"; done
            elif [ "$LB_OK" -gt 0 ]; then
                ok "all ${LB_OK} LoadBalancer address(es) come from a MetalLB pool"
            fi
        fi
    fi
fi

# ===========================================================================
# Storage
#
# The provisioner itself (Longhorn on large, local-path on small/medium) is
# cluster-bloom's and is checked by smoketest-cluster.sh. SeaweedFS is forge's,
# and PVCs only become meaningful once forge's workloads are claiming them.
# ===========================================================================
section_n "Storage"

if [ "$CLUSTER_OK" != "1" ]; then
    blocked "storage checks — no cluster access"
else

    check_ns_pods "$NS_SEAWEEDFS" "SeaweedFS"

    PVC_JSON=$(kc_json get pvc -A)
    if [ $? -ne 0 ]; then
        kc_report_failure "PersistentVolumeClaims"
    else
        # A Pending PVC is only a fault if it SHOULD have bound. With
        # volumeBindingMode: WaitForFirstConsumer (what local-path uses) a claim
        # with no consuming pod stays Pending forever by design — not a finding.
        WFFC_SC=$(kc_json_cached get storageclass 2>/dev/null | jq -r '
            .items[]? | select(.volumeBindingMode=="WaitForFirstConsumer") | .metadata.name')
        CONSUMED=$(kc_json_cached get pods -A 2>/dev/null | jq -r '
            .items[]? | .metadata.namespace as $ns
            | .spec.volumes[]?.persistentVolumeClaim?.claimName // empty
            | "\($ns)/\(.)"' | sort -u)

        PVC_TOTAL=$(printf '%s' "$PVC_JSON" | jq '[.items[]?] | length')
        REAL_BAD=""
        EXPECTED_PENDING=""
        while read -r ns name phase sc; do
            [ -z "$ns" ] && continue
            if [ "$phase" = "Pending" ] \
               && printf '%s\n' "$WFFC_SC" | grep -qx -- "$sc" \
               && ! printf '%s\n' "$CONSUMED" | grep -qx -- "${ns}/${name}"; then
                EXPECTED_PENDING="${EXPECTED_PENDING}${ns}/${name} (sc=${sc}, WaitForFirstConsumer, no consumer)"$'\n'
            else
                REAL_BAD="${REAL_BAD}${ns}/${name} ${phase} (sc=${sc})"$'\n'
            fi
        done <<< "$(printf '%s' "$PVC_JSON" | jq -r '
            .items[]? | select(.status.phase != "Bound")
            | "\(.metadata.namespace) \(.metadata.name) \(.status.phase) \(.spec.storageClassName // "-")"')"

        if [ -z "$REAL_BAD" ]; then
            ok "all PVCs Bound or awaiting first consumer (${PVC_TOTAL} total)"
        else
            warn "$(printf '%s' "$REAL_BAD" | grep -c .) PVC(s) not Bound unexpectedly"
            printf '%s' "$REAL_BAD" | while read -r l; do [ -n "$l" ] && info "  $l"; done
        fi
        if [ -n "$EXPECTED_PENDING" ]; then
            printf '%s' "$EXPECTED_PENDING" | while read -r l; do [ -n "$l" ] && vinfo "pending by design: $l"; done
        fi
    fi
fi

# ===========================================================================
# Keycloak / OpenBao
# ===========================================================================
section_n "Keycloak / OpenBao"

if [ "$CLUSTER_OK" = "1" ]; then
    check_ns_pods "$NS_KEYCLOAK" "Keycloak"
else
    blocked "Keycloak pods — no cluster access"
fi

KC_REALM_OK=0
if [ -z "$DOMAIN" ]; then
    blocked "Keycloak realm endpoint — no domain known"
else
    KC_HOST=$(host_for kc)
    REALM="${KEYCLOAK_REALM:-airm}"
    if curl_probe "https://${KC_HOST}/realms/${REALM}"; then
        if [ "$CURL_CODE" = "200" ]; then
            ok "Keycloak realm '${REALM}' responds 200 at https://${KC_HOST}"
            KC_REALM_OK=1
        else
            # Some environments use a central Keycloak with a per-env realm name.
            if curl_probe "https://${KC_HOST}/realms/${DOMAIN%%.*}" && [ "$CURL_CODE" = "200" ]; then
                ok "Keycloak realm '${DOMAIN%%.*}' responds 200 at https://${KC_HOST}"
                KC_REALM_OK=1
            else
                warn "Keycloak realm endpoint returned HTTP ${CURL_CODE} (tried '${REALM}' and '${DOMAIN%%.*}')"
            fi
        fi
    else
        warn "Keycloak unreachable at https://${KC_HOST} — $(curl_rc_reason $CURL_RC)"
    fi
fi

# ---------------------------------------------------------------------------
# OpenBao
#
# Pod readiness is necessary but not sufficient: a sealed instance passes its
# readiness probe on some chart configurations while serving nothing, and the
# platform components that read their secrets from it fail later and elsewhere.
# The unauthenticated /v1/sys/health endpoint settles it directly.
# ---------------------------------------------------------------------------
if [ "$CLUSTER_OK" = "1" ]; then
    check_ns_pods "$NS_OPENBAO" "OpenBao"
else
    blocked "OpenBao pods — no cluster access"
fi

if [ -z "$DOMAIN" ]; then
    blocked "OpenBao health endpoint — no domain known"
else
    OB_HOST=$(host_for openbao)
    OB_BODY="${TMPDIR_SMOKE}/openbao_health.json"
    if curl_to_file "https://${OB_HOST}/v1/sys/health" "$OB_BODY"; then
        # The status code alone is not the verdict. /v1/sys/health answers 200
        # only for the active node and 429 for an unsealed standby — a wholly
        # normal state in an HA cluster, and the one a routed request is most
        # likely to land on. The body is what distinguishes healthy from not.
        OB_INIT=$(jq -r '.initialized // empty' "$OB_BODY" 2>/dev/null)
        OB_SEALED=$(jq -r '.sealed // empty'      "$OB_BODY" 2>/dev/null)
        OB_STANDBY=$(jq -r '.standby // empty'    "$OB_BODY" 2>/dev/null)
        OB_VER=$(jq -r '.version // empty'        "$OB_BODY" 2>/dev/null)

        if [ -z "$OB_INIT" ]; then
            warn "OpenBao health endpoint returned HTTP ${CURL_CODE} with no recognisable JSON body at https://${OB_HOST}"
        elif [ "$OB_INIT" != "true" ]; then
            warn "OpenBao is not initialized (HTTP ${CURL_CODE}) — no secrets can be read or written"
        elif [ "$OB_SEALED" = "true" ]; then
            warn "OpenBao is SEALED (HTTP ${CURL_CODE}) — it will not serve secrets until unsealed; components that read from it will fail"
        elif [ "$OB_STANDBY" = "true" ]; then
            ok "OpenBao unsealed and serving at https://${OB_HOST} (standby node${OB_VER:+, v${OB_VER}})"
        else
            ok "OpenBao unsealed and active at https://${OB_HOST}${OB_VER:+ (v${OB_VER})}"
        fi
    else
        warn "OpenBao unreachable at https://${OB_HOST} — $(curl_rc_reason $CURL_RC)"
    fi
    rm -f "$OB_BODY" 2>/dev/null
fi

# ===========================================================================
# AIRM / AIWB workloads
# ===========================================================================
section_n "AIRM / AIWB workloads"

if [ "$CLUSTER_OK" != "1" ]; then
    blocked "AIRM / AIWB workload checks — no cluster access"
else
    check_ns_workloads    "$NS_AIRM" "AIRM"
    check_ns_pods         "$NS_AIRM" "AIRM"
    check_ns_service_refs "$NS_AIRM" "AIRM"
    check_ns_workloads    "$NS_AIWB" "AIWB"
    check_ns_pods         "$NS_AIWB" "AIWB"
    check_ns_service_refs "$NS_AIWB" "AIWB"
fi

# ===========================================================================
# Platform application login
# ===========================================================================
section_n "Platform application login"

reachability_check() {
    local label="$1" host="$2"
    if curl_probe "https://${host}/"; then
        case "$CURL_CODE" in
            200|301|302|303|307|308|401|403)
                ok "${label} reachable at https://${host} (HTTP ${CURL_CODE})" ;;
            502|503|504)
                warn "${label} gateway error at https://${host} (HTTP ${CURL_CODE}) — backend likely down" ;;
            000)
                warn "${label} no HTTP response from https://${host}" ;;
            *)
                warn "${label} unexpected HTTP ${CURL_CODE} at https://${host}" ;;
        esac
    else
        warn "${label} unreachable at https://${host} — $(curl_rc_reason $CURL_RC)"
        return 1
    fi
    # Certificate validity is checked for every host in section 8, not here.
    return 0
}

# Read a secret value, quietly. Echoes value or nothing.
secret_value() {
    local ns="$1" name="$2" key="$3"
    kc get secret "$name" -n "$ns" -o jsonpath="{.data.${key}}" 2>/dev/null | base64 -d 2>/dev/null
}

if [ "$SKIP_LOGIN" = "1" ]; then
    skip "platform login checks (--skip-login)"
elif [ -z "$DOMAIN" ]; then
    blocked "platform login checks — no domain known"
else
    # ---- ArgoCD ----
    ARGO_HOST=$(host_for argocd)
    if reachability_check "ArgoCD" "$ARGO_HOST"; then
        ARGO_PW=$(secret_value "$NS_ARGOCD" "argocd-initial-admin-secret" "password")
        if [ -z "$ARGO_PW" ]; then
            blocked "ArgoCD login — argocd-initial-admin-secret not readable (may have been rotated or deleted)"
        else
            ARGO_RESP=$(curl_body "https://${ARGO_HOST}/api/v1/session" \
                          -X POST -H 'Content-Type: application/json' \
                          -d "$(jq -nc --arg p "$ARGO_PW" '{username:"admin",password:$p}')")
            if printf '%s' "$ARGO_RESP" | jq -e '.token' >/dev/null 2>&1; then
                ok "ArgoCD login succeeded (admin session token issued)"
            else
                warn "ArgoCD login failed: $(printf '%s' "$ARGO_RESP" | jq -r '.error // .message // "no token returned"' 2>/dev/null | head -1)"
            fi
        fi
    fi

    # ---- Gitea ----
    GITEA_HOST=$(host_for gitea)
    if reachability_check "Gitea" "$GITEA_HOST"; then
        GITEA_USER="${GITEA_ADMIN_USER:-silogen-admin}"
        GITEA_PW=$(secret_value "$NS_GITEA" "gitea-admin-credentials" "password")
        if [ -z "$GITEA_PW" ]; then
            VER=$(curl_body "https://${GITEA_HOST}/api/v1/version" | jq -r '.version // empty' 2>/dev/null)
            if [ -n "$VER" ]; then
                blocked "Gitea login — gitea-admin-credentials not readable in ns/${NS_GITEA}; API up (version ${VER}) but unauthenticated"
            else
                blocked "Gitea login — gitea-admin-credentials not readable in ns/${NS_GITEA}"
            fi
        else
            # /api/v1/user returns the authenticated user — a real login test,
            # unlike /api/v1/version which answers unauthenticated.
            GITEA_ME=$(curl_body "https://${GITEA_HOST}/api/v1/user" \
                         -u "${GITEA_USER}:${GITEA_PW}" | jq -r '.login // empty' 2>/dev/null)
            if [ -n "$GITEA_ME" ]; then
                VER=$(curl_body "https://${GITEA_HOST}/api/v1/version" \
                        -u "${GITEA_USER}:${GITEA_PW}" | jq -r '.version // empty' 2>/dev/null)
                ok "Gitea login succeeded as ${GITEA_ME}${VER:+ (version ${VER})}"
            else
                warn "Gitea login failed for ${GITEA_USER} — credentials rejected or API unreachable"
            fi
        fi
    fi

    # ---- SeaweedFS admin ----
    SW_HOST=$(host_for seaweed-admin)
    if reachability_check "SeaweedFS admin" "$SW_HOST"; then
        SW_PW=$(secret_value "$NS_SEAWEEDFS" "seaweedfs-admin-secret" "admin-password")
        if [ -z "$SW_PW" ]; then
            blocked "SeaweedFS login — secret seaweedfs-admin-secret/admin-password not readable in ns/${NS_SEAWEEDFS}"
        else
            SW_USER="${SEAWEED_ADMIN_USER:-admin}"
            SW_JAR="${TMPDIR_SMOKE}/seaweed-cookies.txt"
            rm -f "$SW_JAR"
            # The form is CSRF-protected: fetch a token + session cookie first.
            SW_CSRF=$(curl_body "https://${SW_HOST}/login" -c "$SW_JAR" \
                       | grep -oE 'name="csrf_token"[^>]*value="[^"]*"' \
                       | head -1 | sed -E 's/.*value="([^"]*)".*/\1/')
            if [ -z "$SW_CSRF" ]; then
                blocked "SeaweedFS login — could not read csrf_token from the login form"
            else
                curl_probe "https://${SW_HOST}/login" -X POST -b "$SW_JAR" -c "$SW_JAR" \
                    --data-urlencode "csrf_token=${SW_CSRF}" \
                    --data-urlencode "username=${SW_USER}" \
                    --data-urlencode "password=${SW_PW}"
                case "$CURL_CODE" in
                    302|303)
                        ok "SeaweedFS admin login succeeded as ${SW_USER} (HTTP ${CURL_CODE})" ;;
                    200)
                        # A rejected login re-renders the form rather than redirecting.
                        warn "SeaweedFS admin login rejected for ${SW_USER} (login form re-rendered)" ;;
                    *)
                        warn "SeaweedFS admin login returned HTTP ${CURL_CODE}" ;;
                esac
            fi
        fi
    fi

    # ---- Longhorn ----
    if [ "$CLUSTER_OK" = "1" ] && ! ns_exists "$NS_LONGHORN"; then
        skip "Longhorn UI — Longhorn not deployed${CLUSTER_SIZE:+ (cluster size ${CLUSTER_SIZE} uses local-path)}"
    else
        LH_HOST=$(host_for longhorn)
        reachability_check "Longhorn UI" "$LH_HOST"
        info "Longhorn has no independent auth — reachability only"
    fi
fi

# ===========================================================================
# TLS certificates
#
# All applications share one wildcard certificate, so checking three
# representative hosts is enough: Keycloak plus the two platform UIs. Each gets
# the same three checks — is the certificate trusted, does it match the
# hostname, and how long until it expires. The fingerprints are compared so a
# host that has drifted onto a different certificate still gets caught.
# Override with CERT_HOSTS="a b c" to check a different set.
# ===========================================================================
section_n "TLS certificates"

# Fetch a certificate once and populate CERT_*. Returns 1 if no certificate
# could be retrieved (host down, TLS handshake refused).
CERT_DAYS=""; CERT_END=""; CERT_ISSUER=""; CERT_FP=""
cert_info() {
    local host="$1" pem end_ts now_ts
    CERT_DAYS=""; CERT_END=""; CERT_ISSUER=""; CERT_FP=""
    pem=$(echo | openssl s_client -servername "$host" -connect "${host}:443" 2>/dev/null \
          | openssl x509 2>/dev/null)
    [ -z "$pem" ] && return 1

    CERT_END=$(printf '%s' "$pem"    | openssl x509 -noout -enddate 2>/dev/null | cut -d= -f2)
    CERT_ISSUER=$(printf '%s' "$pem" | openssl x509 -noout -issuer 2>/dev/null \
                  | sed -E 's/^issuer=? ?//' \
                  | awk -F' *, *' '{cn="";o="";
                      for(i=1;i<=NF;i++){split($i,kv,/ *= */); if(kv[1]=="CN")cn=kv[2]; if(kv[1]=="O")o=kv[2]}
                      if(o!=""&&cn!="") print o" "cn; else print (cn!=""?cn:o)}')
    CERT_FP=$(printf '%s' "$pem"     | openssl x509 -noout -fingerprint -sha256 2>/dev/null | cut -d= -f2)

    [ -z "$CERT_END" ] && return 0
    end_ts=$(date -d "$CERT_END" +%s 2>/dev/null) || return 0
    now_ts=$(date +%s)
    CERT_DAYS=$(( (end_ts - now_ts) / 86400 ))
    return 0
}

# Trust + hostname validation, deliberately ignoring --insecure: a certificate
# check that skips verification would not be checking anything.
cert_trust_reason() {
    local host="$1" rc
    curl -s -o /dev/null --max-time "$HTTP_TIMEOUT" "https://${host}/" 2>/dev/null
    rc=$?
    case "$rc" in
        0)  echo "" ;;
        51) echo "certificate does not match hostname" ;;
        60) echo "certificate not trusted by this machine's CA store (self-signed, or Zscaler CA not installed locally)" ;;
        35) echo "TLS handshake failed" ;;
        *)  echo "" ;;   # 6/7/28 are connectivity, reported separately
    esac
}

if [ "$SKIP_CERTS" = "1" ]; then
    skip "TLS certificate checks (--skip-certs)"
elif [ "$HAVE_OPENSSL" != "1" ]; then
    blocked "TLS certificate checks — openssl not installed"
elif [ -z "$DOMAIN" ]; then
    blocked "TLS certificate checks — no domain known"
else
    CERT_PREFIXES=$(printf '%s\n' ${CERT_HOSTS:-kc airmui aiwbui})

    CERT_FPS=""
    CERT_CHECKED=0
    while read -r prefix; do
        [ -z "$prefix" ] && continue
        CHOST="${prefix}.${DOMAIN}"

        if ! cert_info "$CHOST"; then
            # Distinguish "no such host" from "host up, TLS broken".
            curl_probe "https://${CHOST}/"
            case "$CURL_RC" in
                6)  skip "${CHOST} — DNS does not resolve (route published but no record?)" ;;
                7|28) warn "${CHOST} — no TLS response ($(curl_rc_reason $CURL_RC))" ;;
                *)  warn "${CHOST} — could not retrieve certificate" ;;
            esac
            continue
        fi
        CERT_CHECKED=$((CERT_CHECKED+1))
        CERT_FPS="${CERT_FPS}${CERT_FP} ${CHOST}"$'\n'

        TRUST_ERR=$(cert_trust_reason "$CHOST")
        DETAIL="issuer ${CERT_ISSUER:-unknown}, expires ${CERT_END:-unknown}"

        if [ -n "$TRUST_ERR" ]; then
            warn "${CHOST} — ${TRUST_ERR}"
            vinfo "${DETAIL}"
        elif [ -z "$CERT_DAYS" ]; then
            warn "${CHOST} — could not parse certificate expiry date"
        elif [ "$CERT_DAYS" -lt 0 ]; then
            warn "${CHOST} — certificate EXPIRED $(( 0 - CERT_DAYS )) day(s) ago (${DETAIL})"
        elif [ "$CERT_DAYS" -lt "$CERT_WARN_DAYS" ]; then
            warn "${CHOST} — certificate expires in ${CERT_DAYS} day(s) (${DETAIL})"
        else
            ok "${CHOST} — valid, ${CERT_DAYS} day(s) left"
            vinfo "${DETAIL}"
        fi
    done <<< "$CERT_PREFIXES"

    # One wildcard certificate is the norm; outliers expire on their own clock.
    if [ "$CERT_CHECKED" -gt 1 ]; then
        UNIQUE_FPS=$(printf '%s' "$CERT_FPS" | awk '{print $1}' | sort -u | grep -c .)
        if [ "$UNIQUE_FPS" = "1" ]; then
            ok "all ${CERT_CHECKED} hosts present the same certificate"
        else
            warn "${UNIQUE_FPS} different certificates across ${CERT_CHECKED} hosts — renewal is not uniform"
            printf '%s' "$CERT_FPS" | awk '{print $1}' | sort | uniq -c | sort -rn \
              | while read -r count fp; do
                    hosts=$(printf '%s' "$CERT_FPS" | awk -v f="$fp" '$1==f {printf "%s ", $2}')
                    info "  ${count} host(s): ${hosts}"
                done
        fi
    fi
fi

# ---------------------------------------------------------------------------
# AI Workbench API — shared preamble for the two sections below
# ---------------------------------------------------------------------------
# Token, API health and project selection are the same for workspaces and
# datasets, so they are resolved once and cached. Whichever section runs first
# pays for them and prints the OK lines; the second just reuses the result.
AIWB_STATE=""
AIWB_ERR=""
AIWB_SKIPPABLE=0
AIWB_API=""
AIWB_PROJECT=""

aiwb_api_get() {
    curl_body "${AIWB_API}$1" -H "Authorization: Bearer ${RM_TOKEN}"
}

# Give the resource-creating sections somewhere to work when the cluster has no
# project the run's user can see.
#
# A freshly built cluster can come up with none at all, and every --with-*
# section then declines for want of a container to put its workload in — the
# sections most worth running are exactly the ones that cannot. Creating one is
# an AIRM call, not an AIWB one: POST /v1/projects lives on the Resource
# Manager, and its description says it "requires platform administrator role".
# The run's own token is used for it rather than a separate admin credential,
# because on a platform-bootstrapped cluster the seeded user already holds that
# role — devuser carries realm role "Platform Administrator" — and the one
# admin credential the cluster does store, airm/airm-keycloak-admin-client,
# turns out not to: its client-credentials grant is refused by /v1/users with
# "Missing required role: Platform Administrator". Where the run user is a
# plain user the create is refused, and that is reported as what it is.
#
# The project is deliberately never deleted. Everything else this script makes
# is removed in cleanup, but a project is the container those things live in,
# and a cluster that needed one still needs it once the run is over. That also
# means an existing project — 'demo' on a hand-built cluster — is only ever
# read here, never re-created and never removed.
aiwb_project_create() {
    local want="$1"
    local api_url="https://airmapi.${RM_DOMAIN}"
    local existing pid body

    rm_token_refresh

    # An administrator can see projects an ordinary member cannot. If the name
    # is already taken the create would 409, and the real problem is a missing
    # membership rather than a missing project — so look before creating and
    # fall through to the same user-add either way.
    existing=$(curl_body "${api_url}/v1/projects" -H "Authorization: Bearer ${RM_TOKEN}")
    pid=$(printf '%s' "$existing" | jq -r --arg n "$want" \
            '[(.data // [])[] | select((.name // "") == $n)][0].id // empty' 2>/dev/null)

    if [ -n "$pid" ]; then
        info "project '${want}' exists but is not visible to ${RM_USER} — adding the user to it"
    else
        if [ -z "$RM_CLUSTER_ID" ]; then
            AIWB_ERR="no project visible to ${RM_USER} and no cluster id known, which ProjectCreate requires — set SMOKETEST_PROJECT"
            return 1
        fi
        # A quota is required by the schema and is a ceiling, not a
        # reservation, so it is set to what the cluster actually has: a
        # smaller number would cap workloads the later sections then fail to
        # schedule, for a project meant to be usable after the run.
        # allocatable.cpu comes in either of two units and the suffix is the only
        # thing that says which: "512" is cores, "63800m" is already millicores.
        # Stripping the "m" and then multiplying by 1000 regardless — which is
        # what this did — asks for a thousand times the cluster on every node
        # that reports the milli form. It went unnoticed because chalupa reports
        # a plain 512. The conversion is decided per node, then summed.
        local q_cpu q_mem q_gpu
        q_cpu=$(kc_json get nodes | jq '[.items[].status.allocatable.cpu // "0"
                  | tostring
                  | if endswith("m") then (.[:-1] | tonumber)
                    else (tonumber * 1000) end] | add // 8000' 2>/dev/null)
        q_gpu=$(kc_json get nodes | jq '[.items[].status.allocatable["amd.com/gpu"] // "0" | tonumber] | add // 0' 2>/dev/null)
        # Zero is caught alongside the non-numeric cases: this jq returns 0 from
        # add on an empty array rather than null, so the fallback inside the
        # filter never fires and a node listing that came back empty would
        # otherwise ask for a quota of no CPU at all.
        case "$q_cpu" in ''|*[!0-9]*|0) q_cpu=8000 ;; esac
        case "$q_gpu" in ''|*[!0-9]*) q_gpu=0 ;; esac
        q_mem=$(( 64 * 1024 * 1024 * 1024 ))
        body=$(jq -nc --arg n "$want" --arg c "$RM_CLUSTER_ID" \
                 --argjson cpu "$q_cpu" --argjson mem "$q_mem" --argjson gpu "$q_gpu" \
                 '{name:$n, description:"Created by cluster-smoketest.sh — kept after the run",
                   clusterId:$c,
                   quota:{cpuMilliCores:$cpu, memoryBytes:$mem, ephemeralStorageBytes:107374182400, gpuCount:$gpu}}')
        # To a file rather than a subshell: curl_body drops the status, and the
        # reason a create failed is mostly in the status. A 500 behind an HTML
        # error page has no .detail to parse, so the body-only version could say
        # no more than "no id in response" for anything that was not a clean
        # validation error.
        local created="${TMPDIR_SMOKE}/airm_project_create.json"
        curl_to_file "${api_url}/v1/projects" "$created" -X POST \
            -H "Authorization: Bearer ${RM_TOKEN}" -H 'Content-Type: application/json' \
            --data "$body"
        pid=$(jq -r '.id // empty' "$created" 2>/dev/null)
        if [ -z "$pid" ]; then
            local why
            why=$(jq -r '(.detail|if type=="array" then .[0].msg else . end) // .message // empty' "$created" 2>/dev/null | head -c 200)
            [ -z "$why" ] && why="no id in response"
            AIWB_ERR="could not create project '${want}' (HTTP ${CURL_CODE:-?}): ${why}"
            return 1
        fi
        ok "created project '${want}' (${pid}) — left in place for later runs"
    fi

    # Creating a project does not put anyone in it: without this the run's user
    # still sees nothing and the sections would decline for the same reason.
    # The listing is read to a file so a refusal can be told apart from a user
    # who genuinely is not there — both leave the id empty, and only one of
    # them is worth telling the reader to go and fix the account for.
    local users_body="${TMPDIR_SMOKE}/airm_users.json" uid
    curl_to_file "${api_url}/v1/users" "$users_body" -H "Authorization: Bearer ${RM_TOKEN}"
    if [ "$CURL_CODE" = "403" ] || [ "$CURL_CODE" = "401" ]; then
        AIWB_ERR="project '${want}' is ready but ${RM_USER} may not list AIRM users (HTTP ${CURL_CODE}: $(jq -r '.detail // "forbidden"' "$users_body" 2>/dev/null | head -c 120)), so it cannot be added to the project — add it in the AIRM UI, or set SMOKETEST_PROJECT to one it already belongs to"
        return 1
    fi
    uid=$(jq -r --arg u "$RM_USER" \
            '[(.data // [])[] | select((.email // "") == $u or (.username // "") == $u)][0].id // empty' \
            "$users_body" 2>/dev/null)
    if [ -z "$uid" ]; then
        AIWB_ERR="project '${want}' is ready but ${RM_USER} was not found in AIRM, so it cannot be added to it"
        return 1
    fi
    # The status is checked rather than discarded. A refusal here is otherwise
    # invisible until the visibility loop below runs out, and that reports a
    # project still provisioning — which sends the reader after the namespace
    # when the membership is what was declined.
    local add_body="${TMPDIR_SMOKE}/airm_project_add_user.json"
    curl_to_file "${api_url}/v1/projects/${pid}/users" "$add_body" -X POST \
        -H "Authorization: Bearer ${RM_TOKEN}" -H 'Content-Type: application/json' \
        --data "$(jq -nc --arg i "$uid" '{userIds:[$i]}')"
    case "$CURL_CODE" in
        2*) : ;;
        # Already a member is the state this is trying to reach, so it is not
        # a failure — the earlier listing just could not see the membership.
        409) : ;;
        *)  AIWB_ERR="could not add ${RM_USER} to project '${want}' (HTTP ${CURL_CODE:-?}): $(jq -r '(.detail|if type=="array" then .[0].msg else . end) // .message // "no detail"' "$add_body" 2>/dev/null | head -c 200)"
            return 1 ;;
    esac

    # Membership reaches the Workbench as a group claim inside the token, so
    # the one in hand cannot see a project the user joined a moment ago — it
    # was minted before the group existed. A plain refresh is not enough
    # either: it returns early while the token is still young, which is
    # exactly the case here. Expire it by hand to force a new one.
    RM_TOKEN_EXP=0
    rm_token_refresh
    if ! printf '%s' "$RM_TOKEN" | cut -d. -f2 | tr '_-' '/+' \
         | { read -r p; case $(( ${#p} % 4 )) in 2) p="${p}==";; 3) p="${p}=";; esac; printf '%s' "$p"; } \
         | base64 -d 2>/dev/null | jq -e --arg n "$want" '[(.groups // [])[]] | index($n)' >/dev/null 2>&1; then
        vinfo "group claim for '${want}' not in the new token yet"
    fi

    # The project is Pending until its namespace is provisioned, and a
    # workspace created against it before then is rejected. Wait for the
    # Workbench to list it rather than assuming the create was instant.
    local waited=0
    while [ "$waited" -lt 60 ]; do
        if aiwb_api_get "/v1/projects" | jq -e --arg n "$want" \
             '[(.data // [])[] | if type == "string" then . else (.id // .name // empty) end] | index($n)' \
             >/dev/null 2>&1; then
            AIWB_PROJECT="$want"
            return 0
        fi
        sleep 5; waited=$(( waited + 5 ))
    done
    AIWB_ERR="project '${want}' was created but has not become visible to ${RM_USER} within 60s — it may still be provisioning"
    return 1
}

aiwb_bootstrap() {
    if [ -n "$AIWB_STATE" ]; then [ "$AIWB_STATE" = "ok" ]; return; fi
    AIWB_STATE="bad"
    # Not having a domain or credentials is a SKIP (nothing to test against);
    # an API that is there but unhealthy is a WARN.
    AIWB_SKIPPABLE=1

    [ -z "$DOMAIN" ] && { AIWB_ERR="no domain known"; return 1; }
    AIWB_API="https://aiwbapi.${DOMAIN}"

    # The Workbench API takes the same Keycloak access token as the Resource
    # Manager. It is already in hand unless --skip-rm was used.
    if [ -z "$RM_TOKEN" ]; then
        RM_STEPS=()
        rm_creds_resolve && rm_fetch_kubeconfig >/dev/null 2>&1
    fi
    [ -z "$RM_TOKEN" ] && { AIWB_ERR="no access token (${RM_CRED_ERR:-authentication failed})"; return 1; }

    AIWB_SKIPPABLE=0
    curl_probe "${AIWB_API}/v1/health"
    if [ "$CURL_RC" -ne 0 ]; then
        AIWB_ERR="AI Workbench API unreachable at ${AIWB_API} — $(curl_rc_reason $CURL_RC)"
        return 1
    elif [ "$CURL_CODE" != "200" ]; then
        AIWB_ERR="AI Workbench API ${AIWB_API}/v1/health returned HTTP ${CURL_CODE}"
        return 1
    fi
    ok "AI Workbench API healthy at ${AIWB_API}"

    # Pick a project: the requested one, or the first the user can see.
    #
    # GET /v1/projects is a ListResponse_str_ — .data is an array of plain
    # project-name strings, not of objects. Reading .id off a string makes jq
    # error out, and with stderr discarded that is indistinguishable from an
    # empty list: on chalupa-491a the script reported "no project visible" for
    # a user who could see 'demo', and every resource-creating section below
    # declined to run. The names are read as strings, but an object with .id is
    # still accepted so a future envelope change degrades rather than breaks.
    local projects project_names
    AIWB_PROJECT="${SMOKETEST_PROJECT:-}"
    projects=$(aiwb_api_get "/v1/projects")
    project_names=$(printf '%s' "$projects" | jq -r '
        (.data // [])[]
        | if type == "string" then . else (.id // .name // empty) end' 2>/dev/null)
    if [ -n "$AIWB_PROJECT" ]; then
        if ! printf '%s\n' "$project_names" | grep -qxF "$AIWB_PROJECT"; then
            AIWB_ERR="project '${AIWB_PROJECT}' (SMOKETEST_PROJECT) not visible to ${RM_USER}"
            AIWB_PROJECT=""
            return 1
        fi
    else
        AIWB_PROJECT=$(printf '%s\n' "$project_names" | head -n1)
    fi

    # Only a run that is going to create something needs a project, so a
    # read-only run keeps reporting the absence rather than provisioning on
    # the way past.
    if [ -z "$AIWB_PROJECT" ]; then
        if [ "$WITH_WORKSPACE" = "1" ] || [ "$WITH_DATASET" = "1" ] || [ "$WITH_MODEL" = "1" ] || [ "$WITH_FINETUNE" = "1" ]; then
            info "no project visible to ${RM_USER} — creating '${SMOKETEST_PROJECT:-demo}' via the AIRM admin client"
            aiwb_project_create "${SMOKETEST_PROJECT:-demo}" || return 1
        else
            AIWB_ERR="no project visible to ${RM_USER} — set SMOKETEST_PROJECT"
            return 1
        fi
    fi
    [ -z "$AIWB_PROJECT" ] && { AIWB_ERR="no project visible to ${RM_USER} — set SMOKETEST_PROJECT"; return 1; }
    ok "using project '${AIWB_PROJECT}'"

    AIWB_STATE="ok"
    return 0
}

# Report a section that cannot run because the preamble failed.
aiwb_unavailable() {
    if [ "$AIWB_SKIPPABLE" = "1" ]; then skip "$1 — ${AIWB_ERR}"; else warn "$1 — ${AIWB_ERR}"; fi
}

# ===========================================================================
# Workspace deployment (AIWB)
# ===========================================================================
# The only section that writes anything. Everything else in this script is
# read-only, so this one is opt-in via --with-workspace and deletes the
# workspace it created before it returns — including on a failed or timed-out
# start, so a bad run does not leave a pod behind for the next one to trip on.
case "$WORKSPACE_TYPE" in
    mlflow)      WS_LABEL="MLflow" ;;
    vscode)      WS_LABEL="VS Code" ;;
    jupyterlab)  WS_LABEL="JupyterLab" ;;
    comfyui)     WS_LABEL="ComfyUI" ;;
esac
# MLflow is limited to one active instance per namespace; the other types are
# one per user per namespace. That changes who can block a create, so the
# pre-check below has to say which it is.
if [ "$WORKSPACE_TYPE" = "mlflow" ]; then
    WS_SCOPE="the API allows one per project"
else
    WS_SCOPE="the API allows one per user per project"
fi

section_n "${WS_LABEL} workspace (AIWB)"

ws_api_get() {
    curl_body "${WS_API}$1" -H "Authorization: Bearer ${RM_TOKEN}"
}

# Workspaces have no GET of their own; they are listed through /workloads.
ws_workload_status() {
    ws_api_get "/v1/projects/${WS_PROJECT}/workloads?pageSize=100&workloadType=WORKSPACE" \
      | jq -r --arg id "$1" '(.data // [])[] | select(.id==$id) | .status // "Unknown"' 2>/dev/null | head -1
}

ws_delete() {
    curl_probe "${WS_API}/v1/projects/${WS_PROJECT}/workspaces/$1" \
      -X DELETE -H "Authorization: Bearer ${RM_TOKEN}"
    printf '%s' "$CURL_CODE"
}

if [ "$WITH_WORKSPACE" != "1" ]; then
    skip "${WS_LABEL} workspace deployment — opt-in with --with-workspace (this check creates and deletes a real workspace)"
elif ! aiwb_bootstrap; then
    aiwb_unavailable "${WS_LABEL} workspace deployment"
else
    WS_API="$AIWB_API"
    WS_PROJECT="$AIWB_PROJECT"
    WS_ID=""

    # A leftover of the same type from an earlier run would come
    # back as a 409, so say so up front rather than reporting the
    # conflict as a failure.
    WS_EXISTING=$(ws_api_get "/v1/projects/${WS_PROJECT}/workloads?pageSize=100&workloadType=WORKSPACE" \
      | jq -r --arg t "$WORKSPACE_TYPE" '[(.data // [])[]
                | select(.status != "Deleted" and .status != "Complete" and .status != "Failed")
                | select((.name // "") | test($t))] | length' 2>/dev/null)
    if [ "${WS_EXISTING:-0}" != "0" ]; then
        skip "${WS_LABEL} workspace deployment — ${WS_EXISTING} active ${WS_LABEL} workspace(s) already in '${WS_PROJECT}' (${WS_SCOPE})"
    else
        # gpus=0: the point is that the workspace comes up, not that
        # it gets an accelerator. Asking for a GPU makes the check
        # fail on any cluster whose GPUs are busy.
        WS_RESP=$(curl_body "${WS_API}/v1/projects/${WS_PROJECT}/workspaces?displayName=cluster-smoketest" \
                    -X POST -H "Authorization: Bearer ${RM_TOKEN}" \
                    -H 'Content-Type: application/json' \
                    -d "{\"workspaceType\":\"${WORKSPACE_TYPE}\",\"gpus\":0}")
        WS_ID=$(printf '%s' "$WS_RESP" | jq -r '.id // empty' 2>/dev/null)
        if [ -z "$WS_ID" ]; then
            warn "workspace create failed: $(printf '%s' "$WS_RESP" | jq -r '.detail // .message // "no id in response"' 2>/dev/null | head -c 200)"
        else
            ok "${WS_LABEL} workspace created ($(printf '%s' "$WS_RESP" | jq -r '.name // .id'))"

            WS_WAITED=0
            WS_STATUS=$(printf '%s' "$WS_RESP" | jq -r '.status // "Unknown"')
            while [ "$WS_WAITED" -lt "$WORKSPACE_TIMEOUT" ]; do
                case "$WS_STATUS" in
                    Running|Failed) break ;;
                esac
                sleep 5; WS_WAITED=$((WS_WAITED + 5))
                rm_token_refresh
                WS_STATUS=$(ws_workload_status "$WS_ID")
                [ -z "$WS_STATUS" ] && WS_STATUS="Unknown"
                vinfo "  ${WS_WAITED}s — ${WS_STATUS}"
            done

            if [ "$WS_STATUS" = "Running" ]; then
                ok "workspace reached Running after ${WS_WAITED}s"
                WS_URL=$(ws_api_get "/v1/projects/${WS_PROJECT}/workloads?pageSize=100&workloadType=WORKSPACE" \
                  | jq -r --arg id "$WS_ID" '(.data // [])[] | select(.id==$id) | (.endpoints // {}) | to_entries[0].value // empty' 2>/dev/null | head -1)
                # The API reports the in-cluster Service URL
                # (*.svc.cluster.local), which nothing outside the
                # cluster can resolve. The externally routable
                # address lives on the workspace's HTTPRoute —
                # read it rather than reconstructing it, so the
                # check does not depend on how the platform builds
                # the path.
                WS_EXT=""
                if [ "$CLUSTER_OK" = "1" ]; then
                    WS_EXT=$(kc_json get httproute -n "$WS_PROJECT" 2>/dev/null \
                      | jq -r --arg id "$WS_ID" '
                          .items[]?
                          | . as $r
                          | ($r.spec.rules[]?.matches[]?.path.value // "") as $path
                          | select($path | contains($id))
                          | "https://\($r.spec.hostnames[0] // "")\($path)"' 2>/dev/null | head -1)
                    [ "$WS_EXT" = "https://" ] && WS_EXT=""
                fi

                if [ -n "$WS_EXT" ]; then
                    curl_probe "$WS_EXT"
                    if [ "$CURL_RC" -ne 0 ]; then
                        warn "workspace not reachable at ${WS_EXT} — $(curl_rc_reason $CURL_RC)"
                    elif [ "$CURL_CODE" -ge 200 ] && [ "$CURL_CODE" -lt 400 ]; then
                        ok "workspace reachable at ${WS_EXT} (HTTP ${CURL_CODE})"
                    else
                        warn "workspace endpoint ${WS_EXT} returned HTTP ${CURL_CODE}"
                    fi
                elif [ -n "$WS_URL" ]; then
                    info "workspace endpoint is cluster-internal (${WS_URL}) — no HTTPRoute found to probe from here"
                else
                    info "workspace exposes no endpoint URL"
                fi
            elif [ "$WS_STATUS" = "Failed" ]; then
                warn "workspace failed to start after ${WS_WAITED}s"
            else
                # A first pull on a node can outlast the timeout —
                # badly so for the ROCm PyTorch images behind vscode
                # and jupyterlab, which run to several GB.
                warn "workspace still ${WS_STATUS} after ${WS_WAITED}s — giving up (first-time image pull is the usual cause; raise --workspace-timeout)"
            fi

            # Clean up whatever the outcome above, unless the run
            # was asked to leave the workspace for inspection.
            if [ "$KEEP_WORKSPACE" = "1" ]; then
                info "workspace ${WS_ID} left running in '${WS_PROJECT}' (--keep-workspace) — delete it manually"
                WS_DEL=""
            else
                rm_token_refresh
                WS_DEL=$(ws_delete "$WS_ID")
            fi
            case "$WS_DEL" in
                "")          : ;;
                200|202|204) ok "workspace deleted (HTTP ${WS_DEL})"; WS_ID="" ;;
                404)         ok "workspace already gone (HTTP 404)"; WS_ID="" ;;
                *)           warn "workspace delete returned HTTP ${WS_DEL} — ${WS_ID} may still be running in '${WS_PROJECT}'" ;;
            esac
        fi
    fi
fi

# ===========================================================================
# Dataset upload (AIWB)
# ===========================================================================
# The second writing check, and opt-in for the same reason as the workspace
# one. It exercises the whole dataset lifecycle rather than just the POST:
# upload, read back, find by name, download and compare the bytes, delete,
# and confirm it is gone. The download comparison is the part that actually
# proves the S3 backend works — an upload that returns 200 but stores nothing
# would pass a create-only check.
section_n "Dataset upload (AIWB)"

ds_api() {   # $1 = method, $2 = path suffix under the project
    curl_probe "${AIWB_API}/v1/projects/${AIWB_PROJECT}$2" \
      -X "$1" -H "Authorization: Bearer ${RM_TOKEN}"
    printf '%s' "$CURL_CODE"
}

if [ "$WITH_DATASET" != "1" ]; then
    skip "dataset upload — opt-in with --with-dataset (this check uploads and deletes a real dataset)"
elif ! aiwb_bootstrap; then
    aiwb_unavailable "dataset upload"
else
    DS_NAME="cluster-smoketest-$(date +%s)"
    DS_SRC="${TMPDIR_SMOKE}/dataset.jsonl"
    DS_GOT="${TMPDIR_SMOKE}/dataset-download.jsonl"
    DS_RESP="${TMPDIR_SMOKE}/dataset-resp.json"
    DS_ID=""

    [ "$FINETUNE_FORCED_DATASET" = "1" ] && \
        info "--with-finetune needs a dataset to train on, so --with-dataset was enabled with it"

    # Fine-tuning is the only type the API accepts today, so the payload is
    # shaped like one: a few chat turns, small enough to be free but real
    # enough that a schema check would have something to look at.
    cat > "$DS_SRC" <<'JSONL'
{"messages":[{"role":"user","content":"ping"},{"role":"assistant","content":"pong"}]}
{"messages":[{"role":"user","content":"What is 2+2?"},{"role":"assistant","content":"4"}]}
{"messages":[{"role":"user","content":"Name a GPU vendor."},{"role":"assistant","content":"AMD"}]}
JSONL

    # A multipart upload has to reach S3 before it answers, and on a busy
    # backend that is comfortably longer than the default probe timeout —
    # measured at ~10s for three lines of JSONL. curl honours the last
    # --max-time it is given, so this one wins over curl_to_file's own.
    curl_to_file "${AIWB_API}/v1/projects/${AIWB_PROJECT}/datasets" "$DS_RESP" \
      --max-time 120 \
      -X POST -H "Authorization: Bearer ${RM_TOKEN}" \
      -F "name=${DS_NAME}" \
      -F "type=Fine-tuning" \
      -F "description=temporary dataset created by cluster-smoketest" \
      -F "jsonl=@${DS_SRC};type=application/octet-stream"

    if [ "$CURL_RC" -ne 0 ]; then
        warn "dataset upload failed — $(curl_rc_reason $CURL_RC)"
    elif [ "$CURL_CODE" != "200" ]; then
        warn "dataset upload returned HTTP ${CURL_CODE}: $(jq -r '.detail // .message // empty' "$DS_RESP" 2>/dev/null | head -c 200)"
    else
        DS_ID=$(jq -r '.id // empty' "$DS_RESP" 2>/dev/null)
        # DS_ID doubles as the "still needs deleting" flag for the exit trap,
        # so the fine-tuning check keeps its own reference to the same dataset.
        FT_DATASET_ID="$DS_ID"
        DS_PATH=$(jq -r '.path // empty' "$DS_RESP" 2>/dev/null)
        if [ -z "$DS_ID" ]; then
            warn "dataset upload returned HTTP 200 with no id"
        else
            ok "dataset '${DS_NAME}' uploaded to '${AIWB_PROJECT}' (${DS_PATH})"

            # Read it back by id: proves the DB record exists and that the
            # metadata came back the way it went in.
            DS_ONE=$(aiwb_api_get "/v1/projects/${AIWB_PROJECT}/datasets/${DS_ID}")
            DS_BACK=$(printf '%s' "$DS_ONE" | jq -r --arg n "$DS_NAME" \
                        'if .name == $n and .type == "Fine-tuning" then "ok" else "\(.name // "?")/\(.type // "?")" end' 2>/dev/null)
            if [ "$DS_BACK" = "ok" ]; then
                ok "dataset readable by id, name and type match"
            else
                warn "dataset read back as '${DS_BACK}' — expected '${DS_NAME}'/Fine-tuning"
            fi

            # And through the list, with the name filter the UI uses.
            DS_HITS=$(aiwb_api_get "/v1/projects/${AIWB_PROJECT}/datasets?pageSize=100&name=${DS_NAME}" \
                        | jq -r '[(.data // [])[] | select(.id=="'"$DS_ID"'")] | length' 2>/dev/null)
            if [ "${DS_HITS:-0}" = "1" ]; then
                ok "dataset appears in the project listing"
            else
                warn "dataset ${DS_ID} not returned by ?name=${DS_NAME} — listing or filter is out of step with the record"
            fi

            # Download and compare. This is the only step that can tell an
            # S3 write that happened from one that was merely acknowledged.
            # Same round trip to S3 as the upload, so the same allowance.
            curl_to_file "${AIWB_API}/v1/projects/${AIWB_PROJECT}/datasets/${DS_ID}/download" "$DS_GOT" \
              --max-time 120 \
              -H "Authorization: Bearer ${RM_TOKEN}"
            if [ "$CURL_RC" -ne 0 ]; then
                warn "dataset download failed — $(curl_rc_reason $CURL_RC)"
            elif [ "$CURL_CODE" != "200" ]; then
                warn "dataset download returned HTTP ${CURL_CODE}"
            elif cmp -s "$DS_SRC" "$DS_GOT"; then
                ok "dataset downloaded and matches the uploaded bytes ($(wc -c < "$DS_GOT") bytes)"
            else
                warn "dataset downloaded but differs from what was uploaded ($(wc -c < "$DS_SRC") bytes out, $(wc -c < "$DS_GOT") bytes back)"
            fi

            # Clean up whatever happened above, unless asked not to. The
            # fine-tuning check trains on this dataset, so when it is going to
            # run the delete is left to the exit trap instead — the job needs
            # the dataset to still be there long after this section is done.
            if [ "$KEEP_DATASET" = "1" ]; then
                info "dataset ${DS_ID} left in '${AIWB_PROJECT}' (--keep-dataset) — delete it manually"
                DS_ID=""
            elif [ "$WITH_FINETUNE" = "1" ]; then
                info "dataset ${DS_ID} kept for the fine-tuning check — deleted once the job is done"
            else
                DS_DEL=$(ds_api DELETE "/datasets/${DS_ID}")
                case "$DS_DEL" in
                    200|202|204)
                        # Delete is documented as idempotent, so a 404 now is
                        # the proof it is gone — not an error.
                        DS_AFTER=$(ds_api GET "/datasets/${DS_ID}")
                        if [ "$DS_AFTER" = "404" ]; then
                            ok "dataset deleted (HTTP ${DS_DEL}) and no longer readable"
                            DS_ID=""
                        else
                            warn "dataset deleted (HTTP ${DS_DEL}) but still readable afterwards (HTTP ${DS_AFTER})"
                        fi
                        ;;
                    404) ok "dataset already gone (HTTP 404)"; DS_ID="" ;;
                    *)   warn "dataset delete returned HTTP ${DS_DEL} — ${DS_ID} may still be in '${AIWB_PROJECT}'" ;;
                esac
            fi
        fi
    fi
fi

# ===========================================================================
# AIM model deployment (AIWB)
# ===========================================================================
# The third writing check, and the most expensive one: it puts a real model
# onto a real accelerator. Opt-in via --with-model for that reason, and it
# deletes what it created on every path out — including a failed or timed-out
# start, because a leftover deployment holds an accelerator until someone
# notices it.
#
# Deploying is the means, not the end. What is being proven is that this
# cluster can serve a completion, so if the model is already deployed and
# serving, this talks to it and deploys nothing — cheaper, and it avoids a
# second copy competing for the same accelerators.
#
# Reaching Running is likewise necessary but not sufficient. A service can
# report Running while the engine behind it never finished loading weights,
# so the verdict is a real chat completion, not a status field.
aim_api_get() {
    curl_body "${AIWB_API}$1" -H "Authorization: Bearer ${RM_TOKEN}"
}

# Status of one deployment. curl_* set globals that do not survive $( ), so
# these print what the caller needs from inside the subshell instead.
# Prints the status, or "!<code>" when the API would not answer at all. The
# caller has to be able to tell "no status yet" from "we can no longer ask":
# while both collapsed to Unknown, an expired token looked exactly like a
# model that was slow to start, and the loop sat there until its deadline.
aim_status() {
    local f
    f="${TMPDIR_SMOKE}/aim_status.json"
    curl_to_file "${AIWB_API}/v1/projects/${AIWB_PROJECT}/inference/$1" "$f" \
        -H "Authorization: Bearer ${RM_TOKEN}"
    if [ "$CURL_RC" -ne 0 ]; then printf '!rc%s' "$CURL_RC"; return 0; fi
    if [ "$CURL_CODE" != "200" ]; then printf '!%s' "$CURL_CODE"; return 0; fi
    jq -r '.statusValue // empty' "$f" 2>/dev/null | head -1
}

aim_delete() {
    curl_probe "${AIWB_API}/v1/projects/${AIWB_PROJECT}/inference/$1" \
        -X DELETE -H "Authorization: Bearer ${RM_TOKEN}"
    printf '%s' "$CURL_CODE"
}

# Whole catalog, one TSV row per model:
#   aimId <TAB> resourceName <TAB> status <TAB> gated|open
# Gating comes from the image metadata rather than the top-level metadata
# field, which is null on every model. A model that declares nothing either
# way is counted as gated: that is how the platform itself treats one whose
# gating cannot be determined, and guessing "open" turns a clean SKIP into a
# failed deploy.
# pageSize caps at 100, so a large catalog needs more than one request; the
# page count comes from pagination.total rather than an assumption, since a
# silently truncated catalog degrades the fallback into "smallest of page 1".
aim_catalog() {
    local page=1 total=0 got=0 body n
    while [ "$page" -le 10 ]; do
        body=$(aim_api_get "/v1/inference/models?page=${page}&pageSize=100${1:+&acceleratorType=$1}")
        [ -z "$body" ] && return 1
        printf '%s' "$body" | jq -r '
            (.data // [])[]
            | select((.status.aimId // "") != "")
            | [ .status.aimId,
                .metadata.name,
                (.status.status // ""),
                ( if   (.status.imageMetadata.model.hfTokenRequired == true) then "gated"
                  elif ((.status.imageMetadata.originalLabels["com.amd.aim.hfToken.required"] // "") == "True")  then "gated"
                  elif ((.status.imageMetadata.originalLabels["com.amd.aim.hfToken.required"] // "") == "False") then "open"
                  else "gated" end ) ]
            | @tsv' 2>/dev/null
        total=$(printf '%s' "$body" | jq -r '.pagination.total // 0' 2>/dev/null)
        n=$(printf '%s' "$body" | jq -r '(.data // []) | length' 2>/dev/null)
        [ -z "$n" ] && n=0
        got=$(( got + n ))
        [ "$n" -eq 0 ] && break
        [ "$got" -ge "${total:-0}" ] && break
        page=$(( page + 1 ))
    done
    return 0
}

# Where to send a completion. The API's own endpoints field is the in-cluster
# Service URL, which nothing outside the cluster can resolve, so the external
# address is read off the HTTPRoute instead of being rebuilt from a guess at
# how the platform composes the path.
aim_external_url() {
    [ "$CLUSTER_OK" = "1" ] || return 0
    local u
    u=$(kc_json get httproute -n "$AIWB_PROJECT" 2>/dev/null \
      | jq -r --arg id "$1" '
          .items[]?
          | . as $r
          | ($r.spec.rules[]?.matches[]?
             | select((.path.type // "") == "PathPrefix")
             | .path.value // "") as $path
          | select($path | contains($id))
          | "https://\($r.spec.hostnames[0] // "")\($path)"' 2>/dev/null | head -1)
    [ "$u" = "https://" ] && u=""
    printf '%s' "$u"
}

section_n "AIM model deployment (AIWB)"

if [ "$WITH_MODEL" != "1" ]; then
    skip "AIM model deployment — opt-in with --with-model (this check deploys a real model onto an accelerator)"
elif ! aiwb_bootstrap; then
    aiwb_unavailable "AIM model deployment"
else
    AIM_CATALOG="${TMPDIR_SMOKE}/aim-catalog.tsv"
    aim_catalog > "$AIM_CATALOG" 2>/dev/null
    AIM_TOTAL=$(grep -c . "$AIM_CATALOG" 2>/dev/null || echo 0)

    if [ "$AIM_TOTAL" -eq 0 ]; then
        # The API answered for /v1/health and /v1/projects, so an empty model
        # catalog is either "AIM is not installed here" (nothing to test) or a
        # real fault. Distinguish on whether the endpoint answers at all.
        if [ -z "$(aim_api_get '/v1/inference/models?pageSize=1')" ]; then
            warn "AIM model catalog unreadable — GET /v1/inference/models returned nothing"
        else
            skip "AIM model deployment — no models in the catalog (AIM not installed on this cluster)"
        fi
    else
        # ---- which model -------------------------------------------------
        # Resolve by status.aimId. The AIMClusterModel resource name carries a
        # version and hash suffix and differs between clusters, so it is never
        # hardcoded — but it is what the deploy API wants, so keep both.
        # Exact match, never a substring: over a long catalog a substring
        # match would catch '<id>-instruct' when asked for '<id>'.
        # With a token every model is fair game; without one only the open
        # ones are, and on a catalog like this that can rule out every small
        # model at once — the tiny instruct models are exactly the gated ones.
        AIM_TOK=0
        [ -n "$SMOKETEST_HF_TOKEN" ] && AIM_TOK=1

        AIM_ROW=$(awk -F'\t' -v id="$MODEL_AIM_ID" -v tok="$AIM_TOK" \
            '$1==id && ($3=="Ready"||$3=="") && (tok=="1" || $4=="open") {print; exit}' "$AIM_CATALOG")

        if [ -z "$AIM_ROW" ]; then
            # "Not stocked here" and "stocked but gated" call for completely
            # different action from whoever reads the output, so they are not
            # allowed to collapse into one message.
            AIM_GATED_HIT=$(awk -F'\t' -v id="$MODEL_AIM_ID" '$1==id && $4=="gated" {print; exit}' "$AIM_CATALOG")
            # Fall back to the smallest model the catalog does have, by the
            # parameter count in its id. A cluster is allowed to stock a
            # different catalog; that is only a warning when the caller named
            # a specific model and did not get it.
            AIM_ROW=$(awk -F'\t' -v tok="$AIM_TOK" '($3=="Ready"||$3=="") && (tok=="1" || $4=="open") {
                    n=$1; sz=99999
                    # A mixture name states experts x size, so the weights that
                    # have to be pulled and held are the product, not the
                    # second number: 8x7B outweighs a dense 14B.
                    if (match(n, /[0-9]+[xX][0-9]+(\.[0-9]+)?[bB]([^0-9]|$)/)) {
                        s=substr(n, RSTART, RLENGTH); gsub(/[^0-9.xX]/, "", s)
                        i=index(s, "x"); if (i==0) i=index(s, "X")
                        sz=(substr(s,1,i-1)+0) * (substr(s,i+1)+0)
                    } else if (match(n, /[0-9]+(\.[0-9]+)?[bB]([^0-9]|$)/)) {
                        s=substr(n, RSTART, RLENGTH); gsub(/[^0-9.]/, "", s); sz=s+0
                    }
                    printf "%09.3f\t%s\n", sz, $0
                }' "$AIM_CATALOG" | sort -k1,1n -k2,2 | head -1 | cut -f2-)
            if [ -n "$AIM_ROW" ] && [ -n "$AIM_GATED_HIT" ]; then
                # Not a cluster fault and not worth a WARN on a default run:
                # no token is the normal state of most environments.
                info "'${MODEL_AIM_ID}' is gated on Hugging Face and no token is set — deploying the smallest open model instead: $(printf '%s' "$AIM_ROW" | cut -f1)"
                info "export SMOKETEST_HF_TOKEN to use the gated model; open models here are far larger, so expect a longer first pull"
            elif [ -n "$AIM_ROW" ]; then
                if [ "$MODEL_EXPLICIT" = "1" ]; then
                    warn "requested model '${MODEL_AIM_ID}' is not in this cluster's catalog — falling back to $(printf '%s' "$AIM_ROW" | cut -f1)"
                else
                    info "'${MODEL_AIM_ID}' not in this cluster's catalog — using the smallest available instead: $(printf '%s' "$AIM_ROW" | cut -f1)"
                fi
            fi
        fi

        AIM_AIMID=$(printf '%s' "$AIM_ROW" | cut -f1)
        AIM_RESOURCE=$(printf '%s' "$AIM_ROW" | cut -f2)

        if [ -z "$AIM_RESOURCE" ]; then
            if [ "$AIM_TOK" = "0" ] && ! awk -F'\t' '$4=="open"{f=1} END{exit !f}' "$AIM_CATALOG"; then
                skip "AIM model deployment — every one of the ${AIM_TOTAL} models here is gated on Hugging Face and no token is set (export SMOKETEST_HF_TOKEN)"
            else
                skip "AIM model deployment — no Ready model in a catalog of ${AIM_TOTAL}"
            fi
        else
            # ---- is it already serving? ----------------------------------
            # Asked before anything about capacity, because a hit here needs
            # no accelerator at all. capability=chat is the server's own
            # filter for "supports chat completions AND the serving stack is
            # fully ready", which is a stronger claim than statusValue alone.
            AIM_TARGET=""
            AIM_OWNED=0
            AIM_TARGET=$(aim_api_get "/v1/projects/${AIWB_PROJECT}/inference?pageSize=100&capability=chat&statusFilter=Running" \
              | jq -r --arg m "$AIM_RESOURCE" '
                  [ (.data // [])[]
                    | select((.spec.model.name // "") == $m)
                    | .id ][0] // empty' 2>/dev/null)

            if [ -n "$AIM_TARGET" ]; then
                ok "${AIM_AIMID} is already deployed and serving (id ${AIM_TARGET}) — talking to it instead of deploying a second copy"
                info "this run did not create it, so it will be left alone"
            fi
        fi

        if [ -n "$AIM_RESOURCE" ] && [ -z "$AIM_TARGET" ]; then
            # ---- can this cluster run it? --------------------------------
            # The platform supports CPU-only AIMs. Ask the API which models
            # publish a CPU footprint rather than inferring it: the filter is
            # authoritative and excludes models with no published hardware.
            AIM_CPU_OK=0
            if aim_catalog cpu 2>/dev/null | cut -f1 | grep -qxF "$AIM_AIMID"; then
                AIM_CPU_OK=1
            fi

            AIM_GATE="ok"
            if [ "$AIM_CPU_OK" = "1" ]; then
                info "${AIM_AIMID} publishes a CPU footprint — no accelerator required"
            elif [ "$CLUSTER_OK" = "1" ]; then
                # Free accelerators, not merely present ones. Requests are
                # counted across every non-terminal pod, Pending included: a
                # pod that is scheduled but not yet running still owns its
                # GPU, and counting Running alone would let two concurrent
                # runs both believe there was capacity.
                AIM_GPU_ALLOC=$(kc_json get nodes 2>/dev/null | jq -r '
                    [ .items[]
                      | select((.spec.unschedulable // false) | not)
                      | select([.status.conditions[]? | select(.type=="Ready" and .status=="True")] | length > 0)
                      | (.status.allocatable["amd.com/gpu"] // "0") | tonumber ] | add // 0' 2>/dev/null)
                AIM_GPU_USED=$(kc_json get pods -A 2>/dev/null | jq -r '
                    [ .items[]
                      | select(.status.phase != "Succeeded" and .status.phase != "Failed")
                      | [ (.spec.containers[]?.resources.requests["amd.com/gpu"] // "0") | tonumber ] | add // 0
                    ] | add // 0' 2>/dev/null)
                [ -z "$AIM_GPU_ALLOC" ] && AIM_GPU_ALLOC=0
                [ -z "$AIM_GPU_USED" ]  && AIM_GPU_USED=0
                AIM_GPU_FREE=$(( AIM_GPU_ALLOC - AIM_GPU_USED ))
                [ "$AIM_GPU_FREE" -lt 0 ] && AIM_GPU_FREE=0

                if [ "$AIM_GPU_ALLOC" -eq 0 ]; then
                    skip "AIM model deployment — this cluster has no GPU nodes and ${AIM_AIMID} has no CPU footprint"
                    AIM_GATE="no"
                elif [ "$AIM_GPU_FREE" -eq 0 ]; then
                    skip "AIM model deployment — no free GPUs (${AIM_GPU_USED} of ${AIM_GPU_ALLOC} allocated)"
                    AIM_GATE="no"
                else
                    ok "${AIM_GPU_FREE} of ${AIM_GPU_ALLOC} GPU(s) free"
                fi
            else
                # No cluster access: the accelerators endpoint answers the
                # coarse question, but reports allocatable only — with no way
                # to tell free from used, refusing to run would be a guess.
                AIM_ACC=$(aim_api_get "/v1/cluster/accelerators" \
                  | jq -r '[(.data // [])[] | select(.acceleratorType=="gpu" or .acceleratorType=="apu") | .allocatableCount] | add // 0' 2>/dev/null)
                [ -z "$AIM_ACC" ] && AIM_ACC=0
                if [ "$AIM_ACC" -eq 0 ]; then
                    skip "AIM model deployment — no GPU or APU accelerator on any ready node, and ${AIM_AIMID} has no CPU footprint"
                    AIM_GATE="no"
                else
                    info "${AIM_ACC} accelerator(s) on the cluster — free capacity unknown without cluster access"
                fi
            fi

            if [ "$AIM_GATE" = "ok" ]; then
                # ---- quota: context, never a gate ------------------------
                # Admission is a queue decision, and a project on nominalQuota
                # 0 borrows from the cohort rather than being refused. Print
                # it before the poll so a Pending-forever result reads in
                # causal order.
                AIM_Q=$(aim_api_get "/v1/projects/${AIWB_PROJECT}/quota" \
                  | jq -r '(.data // .) | select(.isQuotaManaged == true) | [(.resources // [])[] | select(.name=="amd.com/gpu") | .nominalQuota] | add // empty' 2>/dev/null | head -1)
                if [ "$AIM_Q" = "0" ]; then
                    info "project '${AIWB_PROJECT}' has nominalQuota 0 for amd.com/gpu — it borrows from the cohort and can be preempted; the deployment may sit Pending"
                fi

                # ---- adopt anything an earlier run left behind -----------
                # A run killed between create and delete leaves a deployment
                # holding an accelerator. It was ours, so take it back rather
                # than adding a second one beside it.
                AIM_ID=$(aim_api_get "/v1/projects/${AIWB_PROJECT}/inference?pageSize=100" \
                  | jq -r --arg m "$AIM_RESOURCE" '
                      [ (.data // [])[]
                        | select((.statusValue // "") | test("Deleted|Deleting") | not)
                        | select(((.metadata.displayName // "") + " " + (.metadata.name // "")) | test("cluster-smoketest"))
                        | select((.spec.model.name // "") == $m)
                        | .id ][0] // empty' 2>/dev/null)

                if [ -n "$AIM_ID" ]; then
                    warn "leftover smoketest deployment ${AIM_ID} from an earlier interrupted run — adopting it; it will be deleted at the end of this run"
                else
                    # ---- create ------------------------------------------
                    AIM_DISPLAY="cluster-smoketest-$(date +%s)"
                    AIM_CREATE="${TMPDIR_SMOKE}/aim-create.json"
                    curl_to_file "${AIWB_API}/v1/projects/${AIWB_PROJECT}/inference" "$AIM_CREATE" \
                        -X POST -H "Authorization: Bearer ${RM_TOKEN}" \
                        -H 'Content-Type: application/json' \
                        --max-time 60 \
                        -d "$(jq -nc --arg m "$AIM_RESOURCE" --arg d "$AIM_DISPLAY" --arg t "$SMOKETEST_HF_TOKEN" \
                                '{model:$m, displayName:$d, replicas:1}
                                 + (if $t == "" then {} else {hfToken:$t} end)')"
                    AIM_CREATE_RC=$CURL_RC
                    AIM_CREATE_CODE=$CURL_CODE
                    AIM_ID=$(jq -r '.id // empty' "$AIM_CREATE" 2>/dev/null)

                    if [ -z "$AIM_ID" ]; then
                        # A create that timed out or answered badly may still
                        # have made the deployment, and that orphan pins an
                        # accelerator. The unique displayName exists so it can
                        # be found and adopted rather than abandoned.
                        sleep 5
                        AIM_ID=$(aim_api_get "/v1/projects/${AIWB_PROJECT}/inference?pageSize=100" \
                          | jq -r --arg n "$AIM_DISPLAY" '[(.data // [])[] | select((.metadata.displayName // .metadata.name // "") == $n) | .id][0] // empty' 2>/dev/null)
                        if [ -n "$AIM_ID" ]; then
                            if [ "$AIM_CREATE_RC" -ne 0 ]; then
                                warn "inference create did not return an id ($(curl_rc_reason $AIM_CREATE_RC)) but the deployment exists — adopted ${AIM_ID}"
                            else
                                warn "inference create returned HTTP ${AIM_CREATE_CODE} with no id but the deployment exists — adopted ${AIM_ID}"
                            fi
                        fi
                    fi

                    if [ -n "$AIM_ID" ]; then
                        ok "inference deployment created (${AIM_AIMID}, id ${AIM_ID})"
                    elif [ "$AIM_CREATE_RC" -ne 0 ]; then
                        warn "inference create failed — $(curl_rc_reason $AIM_CREATE_RC)"
                    else
                        AIM_CREATE_MSG=$(api_err_msg "$AIM_CREATE" "no id in response")
                        # Belt and braces behind the catalog filter above: a
                        # token the API wants and does not have is a missing
                        # prerequisite, not a broken cluster, so it skips.
                        if printf '%s' "$AIM_CREATE_MSG" | grep -qi 'hugging face token'; then
                            skip "AIM model deployment — ${AIM_AIMID} needs a Hugging Face token and none is set (export SMOKETEST_HF_TOKEN)"
                        else
                            warn "inference create returned HTTP ${AIM_CREATE_CODE} — ${AIM_CREATE_MSG}"
                        fi
                    fi
                fi

                if [ -n "$AIM_ID" ]; then
                    AIM_OWNED=1

                    # ---- poll --------------------------------------------
                    # Wall-clock deadline, not an accumulator: each poll costs
                    # a round trip, and over a long wait a counter that only
                    # adds the sleep understates elapsed time by enough to
                    # make the reported figure wrong.
                    AIM_START=$(date +%s)
                    AIM_DEADLINE=$(( AIM_START + MODEL_TIMEOUT ))
                    AIM_SAW_STARTING=0
                    AIM_PENDING_FOR=0
                    AIM_NAGGED=0
                    AIM_BLIND=0
                    AIM_BLIND_CODE=""
                    AIM_STATUS=$(aim_status "$AIM_ID")
                    [ -z "$AIM_STATUS" ] && AIM_STATUS="Unknown"
                    AIM_ELAPSED=0

                    while [ "$(date +%s)" -lt "$AIM_DEADLINE" ]; do
                        case "$AIM_STATUS" in
                            Running|Failed|Degraded|Deleted) break ;;
                        esac
                        sleep 5
                        rm_token_refresh
                        AIM_STATUS=$(aim_status "$AIM_ID")
                        AIM_ELAPSED=$(( $(date +%s) - AIM_START ))

                        # An unreadable status is not a status. Retry a couple
                        # of times — a single blip should not end a run that
                        # holds an accelerator — then give up and say so
                        # rather than polling a question nothing will answer.
                        case "$AIM_STATUS" in
                            '!'*)
                                AIM_BLIND_CODE="${AIM_STATUS#!}"
                                AIM_BLIND=$(( AIM_BLIND + 1 ))
                                if [ "$AIM_BLIND" -ge 3 ]; then
                                    AIM_STATUS="Unreadable"
                                    break
                                fi
                                # Force a re-mint on the next pass in case the
                                # token, not the API, is what went away.
                                RM_TOKEN_EXP=0
                                vinfo "  ${AIM_ELAPSED}s — status unreadable (${AIM_BLIND_CODE}), retrying"
                                continue
                                ;;
                            *) AIM_BLIND=0 ;;
                        esac
                        [ -z "$AIM_STATUS" ] && AIM_STATUS="Unknown"

                        # Pending does double duty — "just created" and "the
                        # queue will not admit me". The move to Starting is
                        # the only thing that separates them, so record
                        # whether it ever happened.
                        if [ "$AIM_SAW_STARTING" = "0" ]; then
                            case "$AIM_STATUS" in
                                Starting|Running)
                                    AIM_SAW_STARTING=1
                                    AIM_PENDING_FOR=$AIM_ELAPSED
                                    info "admitted after ${AIM_ELAPSED}s — now ${AIM_STATUS} (pulling image / loading weights)"
                                    ;;
                            esac
                        fi
                        if [ "$AIM_STATUS" = "Pending" ] && [ "$AIM_ELAPSED" -ge 120 ] && [ "$AIM_NAGGED" = "0" ]; then
                            AIM_NAGGED=1
                            info "still Pending after ${AIM_ELAPSED}s — not yet admitted, so no accelerator has been assigned; this is not an image pull"
                        fi
                        vinfo "  ${AIM_ELAPSED}s — ${AIM_STATUS}"
                    done
                    AIM_ELAPSED=$(( $(date +%s) - AIM_START ))

                    case "$AIM_STATUS" in
                    Running)
                        if [ "$AIM_SAW_STARTING" = "1" ] && [ "$AIM_PENDING_FOR" -gt 0 ]; then
                            ok "model reached Running after ${AIM_ELAPSED}s (pending ${AIM_PENDING_FOR}s, starting $(( AIM_ELAPSED - AIM_PENDING_FOR ))s)"
                        else
                            ok "model reached Running after ${AIM_ELAPSED}s"
                        fi
                        AIM_TARGET="$AIM_ID"
                        ;;
                    Failed)
                        AIM_WHY=$(aim_api_get "/v1/projects/${AIWB_PROJECT}/inference/${AIM_ID}" \
                          | jq -r '[(.status.conditions // [])[] | select(.status=="False") | .message] | join("; ")' 2>/dev/null | head -c 200)
                        warn "model deployment failed after ${AIM_ELAPSED}s${AIM_WHY:+ — ${AIM_WHY}}"
                        ;;
                    Degraded)
                        # Came up and then regressed — a different fault from
                        # never starting, and worth saying so.
                        warn "model is Degraded after ${AIM_ELAPSED}s — it started and then lost replicas"
                        ;;
                    Unreadable)
                        warn "lost the ability to read deployment status after ${AIM_ELAPSED}s (HTTP ${AIM_BLIND_CODE}) — the model may still be starting; it is deleted below either way"
                        ;;
                    Deleted)
                        warn "model deployment disappeared while starting — something else deleted it"
                        AIM_ID=""
                        AIM_OWNED=0
                        ;;
                    *)
                        if [ "$AIM_SAW_STARTING" = "1" ]; then
                            warn "model still ${AIM_STATUS} after ${AIM_ELAPSED}s — it was admitted but the image pull or weight load did not finish (a first AIM pull runs to many GB; raise --model-timeout)"
                        else
                            warn "model never left Pending after ${AIM_ELAPSED}s — it was never admitted, so no accelerator was assigned; check queue admission and project quota rather than the model"
                        fi
                        ;;
                    esac
                fi
            fi
        fi

        # ---- the actual verdict, however we got here ---------------------
        # A completion, not a status field: the only check that proves the
        # engine loaded its weights and can answer. curl_to_file's own
        # --max-time comes first and curl honours the last occurrence, so
        # this later one wins for this one slow call.
        if [ -n "$AIM_TARGET" ]; then
            AIM_EXT=$(aim_external_url "$AIM_TARGET")
            if [ -z "$AIM_EXT" ]; then
                AIM_INT=$(aim_api_get "/v1/projects/${AIWB_PROJECT}/inference/${AIM_TARGET}" \
                  | jq -r '(.endpoints // {}) | to_entries[0].value // empty' 2>/dev/null | head -1)
                if [ -n "$AIM_INT" ]; then
                    info "model endpoint is cluster-internal (${AIM_INT}) — no HTTPRoute found to probe from here"
                else
                    warn "model is Running but publishes no endpoint"
                fi
            else
                # The model endpoint does not accept the Keycloak token. Its
                # route is guarded by an api-key policy that checks the
                # Authorization header against a per-model secret, so a key
                # has to be minted for this deployment and presented instead.
                # Bind it to this deployment only and give it an hour, so a
                # run that dies before the revoke below leaves nothing
                # long-lived behind.
                AIM_CHAT="${TMPDIR_SMOKE}/aim-chat.json"
                AIM_KEY=""
                rm_token_refresh
                curl_to_file "${AIWB_API}/v1/projects/${AIWB_PROJECT}/api-keys" "$AIM_CHAT" \
                    -X POST -H "Authorization: Bearer ${RM_TOKEN}" \
                    -H 'Content-Type: application/json' \
                    -d "$(jq -nc --arg n "cluster-smoketest-$(date +%s)" --arg a "$AIM_TARGET" \
                            '{displayName:$n, ttl:"1h", aimIds:[$a]}')"
                if [ "$CURL_RC" -eq 0 ] && [ "$CURL_CODE" = "200" ]; then
                    AIM_KEY=$(jq -r '.fullKey // empty' "$AIM_CHAT" 2>/dev/null)
                    AIM_KEY_ID=$(jq -r '.id // empty' "$AIM_CHAT" 2>/dev/null)
                fi
                if [ -z "$AIM_KEY" ]; then
                    warn "could not mint an API key for the model endpoint (HTTP ${CURL_CODE}) — cannot ask the model to serve a completion"
                fi
            fi
            if [ -n "$AIM_EXT" ] && [ -n "$AIM_KEY" ]; then
                curl_to_file "${AIM_EXT}/v1/chat/completions" "$AIM_CHAT" \
                    -X POST -H "Authorization: Bearer ${AIM_KEY}" \
                    -H 'Content-Type: application/json' \
                    --max-time 120 \
                    -d "{\"model\":\"${AIM_AIMID}\",\"messages\":[{\"role\":\"user\",\"content\":\"Reply with the single word: ok\"}],\"max_tokens\":16,\"temperature\":0}"
                # Whether the gateway compares the whole header or only what
                # follows the scheme has moved between versions of the policy,
                # and neither the API nor the policy says which this is. One
                # retry with the bare key costs a second and removes the guess.
                if [ "$CURL_RC" -eq 0 ] && [ "$CURL_CODE" = "401" ]; then
                    curl_to_file "${AIM_EXT}/v1/chat/completions" "$AIM_CHAT" \
                        -X POST -H "Authorization: ${AIM_KEY}" \
                        -H 'Content-Type: application/json' \
                        --max-time 120 \
                        -d "{\"model\":\"${AIM_AIMID}\",\"messages\":[{\"role\":\"user\",\"content\":\"Reply with the single word: ok\"}],\"max_tokens\":16,\"temperature\":0}"
                fi
                if [ "$CURL_RC" -ne 0 ]; then
                    warn "chat completion failed at ${AIM_EXT} — $(curl_rc_reason $CURL_RC)"
                elif [ "$CURL_CODE" != "200" ]; then
                    warn "chat completion returned HTTP ${CURL_CODE} at ${AIM_EXT} — $(jq -r '.detail // .message // .error.message // empty' "$AIM_CHAT" 2>/dev/null | head -c 200)"
                else
                    AIM_REPLY=$(jq -r '.choices[0].message.content // empty' "$AIM_CHAT" 2>/dev/null | tr '\n' ' ' | sed 's/[[:space:]]*$//' | head -c 60)
                    if [ -n "$AIM_REPLY" ]; then
                        ok "model served a chat completion at ${AIM_EXT} — replied '${AIM_REPLY}'"
                    else
                        warn "chat completion returned HTTP 200 but no message content — the engine answered without generating"
                    fi
                fi
                rm -f "$AIM_CHAT" 2>/dev/null
                # Revoke now rather than leaning on the ttl.
                if [ -n "$AIM_KEY_ID" ]; then
                    rm_token_refresh
                    curl_probe "${AIWB_API}/v1/projects/${AIWB_PROJECT}/api-keys/${AIM_KEY_ID}" \
                        -X DELETE -H "Authorization: Bearer ${RM_TOKEN}"
                    case "$CURL_CODE" in
                        200|202|204|404) AIM_KEY_ID="" ;;
                        *) warn "could not revoke the smoke test API key ${AIM_KEY_ID} (HTTP ${CURL_CODE}) — it expires on its own within the hour" ;;
                    esac
                fi
                AIM_KEY=""
            fi
        fi

        # ---- clean up, but only what this run created --------------------
        # AIM_OWNED is the whole point: a deployment that was already serving
        # when this started belongs to someone else and is not ours to remove.
        if [ "$AIM_OWNED" = "1" ] && [ -n "$AIM_ID" ]; then
            if [ "$KEEP_MODEL" = "1" ]; then
                info "inference deployment ${AIM_ID} left running in '${AIWB_PROJECT}' (--keep-model) — it holds an accelerator until you delete it"
                AIM_ID=""
            else
                rm_token_refresh
                AIM_DEL=$(aim_delete "$AIM_ID")
                case "$AIM_DEL" in
                    200|202|204) ok "inference deployment deleted (HTTP ${AIM_DEL})" ;;
                    404)         ok "inference deployment already gone (HTTP 404)" ;;
                    *)           warn "inference delete returned HTTP ${AIM_DEL} — ${AIM_ID} may still be holding an accelerator in '${AIWB_PROJECT}'" ;;
                esac
                # DELETE only marks it; the accelerator is not free until the
                # pod goes, and a still-terminating workload is what the next
                # run's adoption check would trip on.
                AIM_GONE=0
                AIM_DWAIT=0
                while [ "$AIM_DWAIT" -lt 60 ]; do
                    AIM_DS=$(aim_status "$AIM_ID")
                    if [ -z "$AIM_DS" ] || [ "$AIM_DS" = "Deleted" ]; then AIM_GONE=1; break; fi
                    sleep 5
                    AIM_DWAIT=$(( AIM_DWAIT + 5 ))
                done
                if [ "$AIM_GONE" = "1" ]; then
                    ok "accelerator released after ${AIM_DWAIT}s"
                else
                    info "deployment still terminating after ${AIM_DWAIT}s — the accelerator frees shortly"
                fi
                AIM_ID=""
            fi
        fi
    fi
fi

# ===========================================================================
# Workbench fine-tuning (AIWB)
# ===========================================================================
# The last of the writing checks and the slowest: it trains a real model on a
# real accelerator. Opt-in via --with-finetune, and it needs a dataset, so it
# turns --with-dataset on for itself and holds that dataset open until the
# job is done.
#
# What is being proven is narrow on purpose: that a job submitted through the
# API is admitted, trains to Complete, and registers a model the project can
# read back. The produced model is not deployed and never asked to generate
# anything — that is the AIM section's job, and doing it twice would double
# the most expensive part of the run for no new information.
section_n "Workbench fine-tuning (AIWB)"

ft_api_get() {
    curl_body "${AIWB_API}$1" -H "Authorization: Bearer ${RM_TOKEN}"
}

# Status of the training job. Fine-tuning jobs are tracked as workloads —
# there is no GET on /fine-tuning/jobs — and like aim_status this prints
# "!<code>" when the API will not answer, because a token that expired
# mid-training must not look like a job that is merely slow.
ft_status() {
    local f
    f="${TMPDIR_SMOKE}/ft_status.json"
    curl_to_file "${AIWB_API}/v1/projects/${AIWB_PROJECT}/workloads/$1" "$f" \
        -H "Authorization: Bearer ${RM_TOKEN}"
    if [ "$CURL_RC" -ne 0 ]; then printf '!rc%s' "$CURL_RC"; return 0; fi
    if [ "$CURL_CODE" != "200" ]; then printf '!%s' "$CURL_CODE"; return 0; fi
    jq -r 'if (.status | type) == "string" then .status else (.status.value // .statusValue // empty) end' "$f" 2>/dev/null | head -1
}

if [ "$WITH_FINETUNE" != "1" ]; then
    skip "Workbench fine-tuning — opt-in with --with-finetune (this check trains a real model on an accelerator)"
elif ! aiwb_bootstrap; then
    aiwb_unavailable "Workbench fine-tuning"
elif [ -z "$FT_DATASET_ID" ]; then
    # No dataset, nothing to train on. The dataset section has already said
    # why, so this only records the consequence.
    blocked "Workbench fine-tuning — the dataset check did not leave a dataset to train on"
else
    FT_GATE="ok"

    # ---- Hugging Face token ------------------------------------------
    # The job takes the *name of a secret* in the project namespace, not a
    # token — unlike the inference API, which takes one inline. So a token
    # supplied on the command line has to be written into a secret first,
    # and one that already exists in the project can be used as-is.
    #
    # Both halves go through the Workbench secrets API rather than kubectl,
    # which is what makes this work — and, more importantly, what
    # makes the secret the right shape. A Workbench secret has a generated
    # resource name, carries its human-readable name in an annotation and
    # its purpose in the label 'use-case', and holds the token under the
    # key 'token'. A hand-made secret with a guessable name and an
    # 'hf-token' key is not something the platform recognises.
    #
    # useCase is a fixed enum shared by AIRM and AIWB
    # (HuggingFace | ImagePullSecret | S3 | Database | Generic), read back
    # from the label, so it — not the key name — is how an existing token
    # is found. hfTokenSecretName is mounted in-cluster, so it wants the
    # resource name from metadata, never the display name.
    FT_SECRET_NAME=""
    if [ -n "$SMOKETEST_HF_TOKEN" ]; then
        FT_SEC_OUT="${TMPDIR_SMOKE}/ft-secret.json"
        FT_SEC_DISPLAY="smoketest-hf-token-$(date +%s)"
        # Values are handed to the K8s Secret unmodified, so they have to
        # arrive base64-encoded. -w0: a wrapped line would not decode.
        curl_to_file "${AIWB_API}/v1/projects/${AIWB_PROJECT}/secrets" "$FT_SEC_OUT" \
            -X POST -H "Authorization: Bearer ${RM_TOKEN}" \
            -H 'Content-Type: application/json' \
            -d "$(jq -nc --arg d "$FT_SEC_DISPLAY" \
                         --arg t "$(printf '%s' "$SMOKETEST_HF_TOKEN" | base64 -w0)" \
                    '{displayName:$d, useCase:"HuggingFace", data:{token:$t}}')"
        case "${CURL_RC}:${CURL_CODE}" in
            0:200|0:201)
                FT_SECRET_NAME=$(jq -r '.metadata.name // empty' "$FT_SEC_OUT" 2>/dev/null)
                if [ -n "$FT_SECRET_NAME" ]; then
                    FT_SECRET="$FT_SECRET_NAME"
                    ok "wrote the supplied Hugging Face token to secret '${FT_SEC_DISPLAY}' (${FT_SECRET_NAME}) in '${AIWB_PROJECT}'"
                else
                    info "the Hugging Face token secret was created but the API returned no resource name — looking for an existing one instead"
                fi
                ;;
            0:*)
                info "could not create a Hugging Face token secret in '${AIWB_PROJECT}' (HTTP ${CURL_CODE}) — looking for an existing one instead"
                ;;
            *)
                info "could not create a Hugging Face token secret in '${AIWB_PROJECT}' ($(curl_rc_reason "$CURL_RC")) — looking for an existing one instead"
                ;;
        esac
    fi

    if [ -z "$FT_SECRET_NAME" ]; then
        # Anything the project already classifies as a Hugging Face secret.
        # The display name is only for the message — several may exist, and
        # which one is picked is arbitrary, so say which one it was.
        FT_SEC_FOUND=$(ft_api_get "/v1/projects/${AIWB_PROJECT}/secrets" 2>/dev/null | jq -r '
            [ (.data // [])[]
              | select(.useCase == "HuggingFace")
              | select((.metadata.name // "") != "")
              | "\(.metadata.name)\t\(.displayName // .metadata.name)" ]
            | sort | .[0] // empty' 2>/dev/null)
        if [ -n "$FT_SEC_FOUND" ]; then
            FT_SECRET_NAME="${FT_SEC_FOUND%%$'\t'*}"
            info "using the existing Hugging Face token secret '${FT_SEC_FOUND##*$'\t'}' (${FT_SECRET_NAME}) in '${AIWB_PROJECT}'"
        fi
    fi

    # ---- pick a base model -------------------------------------------
    # Data-driven, with no model name written into this script: what is
    # finetunable is a property of the recipes installed on the cluster.
    FT_CAT="${TMPDIR_SMOKE}/ft-catalog.json"
    curl_to_file "${AIWB_API}/v1/fine-tuning/models?pageSize=100" "$FT_CAT" \
        -H "Authorization: Bearer ${RM_TOKEN}"
    if [ "$CURL_RC" -ne 0 ]; then
        warn "fine-tuning catalog unreadable — $(curl_rc_reason $CURL_RC)"
        FT_GATE="no"
    elif [ "$CURL_CODE" != "200" ]; then
        warn "GET /v1/fine-tuning/models returned HTTP ${CURL_CODE}"
        FT_GATE="no"
    fi

    FT_TOTAL=0
    [ "$FT_GATE" = "ok" ] && FT_TOTAL=$(jq -r '(.data // []) | length' "$FT_CAT" 2>/dev/null)
    [ -z "$FT_TOTAL" ] && FT_TOTAL=0
    if [ "$FT_GATE" = "ok" ] && [ "$FT_TOTAL" -eq 0 ]; then
        skip "Workbench fine-tuning — this cluster publishes no finetunable base models"
        FT_GATE="no"
    fi

    # This cluster's AMD device ids, for intersecting with what each recipe
    # declares it can train on. The node label reads '74a1' and the API
    # reports '0x74a1', so both are normalised to the bare hex.
    FT_DEVIDS=""
    if [ "$FT_GATE" = "ok" ] && [ "$CLUSTER_OK" = "1" ]; then
        FT_DEVIDS=$(kc_json get nodes 2>/dev/null | jq -r '
            [ .items[]
              | select((.spec.unschedulable // false) | not)
              | (.metadata.labels["amd.com/gpu.device-id"] // empty) ]
            | unique | .[]' 2>/dev/null | tr 'A-Z' 'a-z' | sed 's/^0x//')
    fi

    # One TSV row per usable recipe, smallest first:
    #   parameterCount <TAB> canonicalName <TAB> gpuCount <TAB> gated|open
    # Size is parsed out of the canonical name because nothing in the recipe
    # reports it, and '8x7B' means 56B, not 7B. Sorting on it is what keeps a
    # smoke test off a model that trains for an hour when a smaller one is
    # sitting right beside it in the same catalog.
    FT_ROWS="${TMPDIR_SMOKE}/ft-rows.tsv"
    : > "$FT_ROWS"
    if [ "$FT_GATE" = "ok" ]; then
        jq -r --arg ids "$FT_DEVIDS" '
            ($ids | split("\n") | map(select(length > 0))) as $have
            | (.data // [])[]
            | . as $m
            | ([ (.compatibleAccelerators // [])[] | ascii_downcase | sub("^0x"; "") ]) as $want
            | select(($have | length) == 0 or ($want | length) == 0
                     or (($want - ($want - $have)) | length) > 0)
            | [ (.canonicalName // empty),
                ((.gpuCount // 1) | tostring),
                (if (.hfTokenRequired == false) then "open" else "gated" end) ]
            | @tsv' "$FT_CAT" 2>/dev/null \
        | awk -F'\t' -v tok="$([ -n "$FT_SECRET_NAME" ] && echo 1 || echo 0)" '
            $1 == "" { next }
            # A gated recipe with no token in reach cannot download its
            # weights; drop it here rather than discovering it as a failed
            # job twenty minutes in.
            $3 == "gated" && tok == 0 { next }
            {
                n = tolower($1); sz = 0
                if (match(n, /[0-9]+[xX][0-9]+(\.[0-9]+)?[bB]([^0-9]|$)/)) {
                    s = substr(n, RSTART, RLENGTH); gsub(/[^0-9.xX]/, "", s)
                    i = index(s, "x"); if (i == 0) i = index(s, "X")
                    sz = (substr(s, 1, i-1) + 0) * (substr(s, i+1) + 0)
                } else if (match(n, /[0-9]+(\.[0-9]+)?[bB]([^0-9]|$)/)) {
                    s = substr(n, RSTART, RLENGTH); gsub(/[^0-9.]/, "", s); sz = s + 0
                }
                printf "%09.3f\t%s\t%s\t%s\n", sz, $1, $2, $3
            }' | sort -k1,1n -k2,2 > "$FT_ROWS"
    fi

    FT_MODEL=""
    FT_GPUS=1
    if [ "$FT_GATE" = "ok" ]; then
        if [ -n "$FINETUNE_MODEL" ]; then
            FT_ROW=$(awk -F'\t' -v m="$FINETUNE_MODEL" '$2 == m {print; exit}' "$FT_ROWS")
            if [ -z "$FT_ROW" ]; then
                # Say which of the two reasons it was: absent from the
                # catalog altogether, or present but out of reach here.
                if jq -e --arg m "$FINETUNE_MODEL" '[(.data // [])[] | select(.canonicalName == $m)] | length > 0' "$FT_CAT" >/dev/null 2>&1; then
                    skip "Workbench fine-tuning — '${FINETUNE_MODEL}' is in the catalog but not usable here (gated with no token, or no compatible accelerator)"
                else
                    skip "Workbench fine-tuning — '${FINETUNE_MODEL}' is not among the ${FT_TOTAL} finetunable base models on this cluster"
                fi
                FT_GATE="no"
            fi
        else
            FT_ROW=$(head -1 "$FT_ROWS")
            if [ -z "$FT_ROW" ]; then
                if [ -z "$FT_SECRET_NAME" ]; then
                    skip "Workbench fine-tuning — every one of the ${FT_TOTAL} finetunable base models is gated on Hugging Face and no token is available (pass --hf-token or --hf-token-file)"
                else
                    skip "Workbench fine-tuning — none of the ${FT_TOTAL} finetunable base models is compatible with this cluster's accelerators"
                fi
                FT_GATE="no"
            fi
        fi
    fi

    if [ "$FT_GATE" = "ok" ]; then
        FT_MODEL=$(printf '%s' "$FT_ROW" | cut -f2)
        FT_GPUS=$(printf '%s' "$FT_ROW" | cut -f3)
        [ -z "$FT_GPUS" ] && FT_GPUS=1
        FT_SIZE=$(printf '%s' "$FT_ROW" | cut -f1 | sed 's/^0*//; s/^\./0./; s/\.000$//')
        ok "base model ${FT_MODEL} selected (${FT_SIZE:-?}B parameters, ${FT_GPUS} GPU)"
        if [ -z "$FT_SECRET_NAME" ]; then
            # Not a fault: most environments have no token. But the open
            # models here are large, and the wait is the visible consequence.
            info "no Hugging Face token available, so only ungated base models were considered — these are much larger and train correspondingly longer"
            info "pass --hf-token or --hf-token-file to train a small gated model instead"
        fi
    fi

    # ---- is there an accelerator free? --------------------------------
    if [ "$FT_GATE" = "ok" ]; then
        if [ "$CLUSTER_OK" = "1" ]; then
            # Allocatable minus requests across every non-terminal pod, the
            # same accounting the AIM section uses: /v1/cluster/accelerators
            # reports allocatable only and cannot answer "free".
            FT_GPU_ALLOC=$(kc_json get nodes 2>/dev/null | jq -r '
                [ .items[]
                  | select((.spec.unschedulable // false) | not)
                  | select([.status.conditions[]? | select(.type=="Ready" and .status=="True")] | length > 0)
                  | (.status.allocatable["amd.com/gpu"] // "0") | tonumber ] | add // 0' 2>/dev/null)
            FT_GPU_USED=$(kc_json get pods -A 2>/dev/null | jq -r '
                [ .items[]
                  | select(.status.phase != "Succeeded" and .status.phase != "Failed")
                  | [ (.spec.containers[]?.resources.requests["amd.com/gpu"] // "0") | tonumber ] | add // 0
                ] | add // 0' 2>/dev/null)
            [ -z "$FT_GPU_ALLOC" ] && FT_GPU_ALLOC=0
            [ -z "$FT_GPU_USED" ]  && FT_GPU_USED=0
            FT_GPU_FREE=$(( FT_GPU_ALLOC - FT_GPU_USED ))
            [ "$FT_GPU_FREE" -lt 0 ] && FT_GPU_FREE=0

            if [ "$FT_GPU_ALLOC" -eq 0 ]; then
                skip "Workbench fine-tuning — this cluster has no GPU nodes"
                FT_GATE="no"
            elif [ "$FT_GPU_FREE" -lt "$FT_GPUS" ]; then
                skip "Workbench fine-tuning — ${FT_MODEL} needs ${FT_GPUS} GPU(s) and only ${FT_GPU_FREE} of ${FT_GPU_ALLOC} are free"
                FT_GATE="no"
            else
                ok "${FT_GPU_FREE} of ${FT_GPU_ALLOC} GPU(s) free, ${FT_GPUS} needed"
            fi
        else
            FT_ACC=$(ft_api_get "/v1/cluster/accelerators" \
              | jq -r '[(.data // [])[] | select(.acceleratorType=="gpu" or .acceleratorType=="apu") | .allocatableCount] | add // 0' 2>/dev/null)
            [ -z "$FT_ACC" ] && FT_ACC=0
            if [ "$FT_ACC" -lt "$FT_GPUS" ]; then
                skip "Workbench fine-tuning — ${FT_ACC} accelerator(s) on the cluster, ${FT_GPUS} needed"
                FT_GATE="no"
            else
                info "${FT_ACC} accelerator(s) on the cluster — free capacity unknown without cluster access"
            fi
        fi
    fi

    # ---- adopt anything an earlier run left behind --------------------
    # A run killed mid-training leaves a job holding its accelerators for as
    # long as the training would have taken. It was ours, so take it back and
    # wait on it rather than starting a second one beside it.
    FT_DISPLAY=""
    if [ "$FT_GATE" = "ok" ]; then
        FT_JOB_ID=$(ft_api_get "/v1/projects/${AIWB_PROJECT}/workloads?pageSize=100&workloadType=FINE_TUNING" \
          | jq -r '
              [ (.data // [])[]
                | select(((.status | if type == "string" then . else (.value // "") end)) as $s
                         | $s == "Pending" or $s == "Starting" or $s == "Running")
                | select(((.displayName // .name // "")) | startswith("cluster-smoketest-"))
                | .id ][0] // empty' 2>/dev/null)
        if [ -n "$FT_JOB_ID" ]; then
            warn "leftover smoketest fine-tuning job ${FT_JOB_ID} from an earlier interrupted run — adopting it; it will be cleaned up at the end of this run"
        fi
    fi

    # ---- submit --------------------------------------------------------
    if [ "$FT_GATE" = "ok" ] && [ -z "$FT_JOB_ID" ]; then
        FT_DISPLAY="cluster-smoketest-$(date +%s)"
        FT_CREATE="${TMPDIR_SMOKE}/ft-create.json"
        rm_token_refresh
        # epochs 1 and the API's default batch size: the point is that
        # training runs to completion, not that the result is any good.
        curl_to_file "${AIWB_API}/v1/projects/${AIWB_PROJECT}/fine-tuning/jobs" "$FT_CREATE" \
            -X POST -H "Authorization: Bearer ${RM_TOKEN}" \
            -H 'Content-Type: application/json' \
            --max-time 60 \
            -d "$(jq -nc --arg d "$FT_DISPLAY" --arg b "$FT_MODEL" --arg ds "$FT_DATASET_ID" --arg s "$FT_SECRET_NAME" \
                    '{displayName:$d, baseModel:$b, datasetIds:[$ds], epochs:1,
                      description:"temporary fine-tuning job created by cluster-smoketest"}
                     + (if $s == "" then {} else {hfTokenSecretName:$s} end)')"
        FT_CREATE_RC=$CURL_RC
        FT_CREATE_CODE=$CURL_CODE
        FT_JOB_ID=$(jq -r '.workloadId // empty' "$FT_CREATE" 2>/dev/null)

        if [ -z "$FT_JOB_ID" ]; then
            # A create that timed out may still have started a job, and that
            # job holds accelerators. The unique display name is there so it
            # can be found and adopted rather than abandoned.
            sleep 5
            FT_JOB_ID=$(ft_api_get "/v1/projects/${AIWB_PROJECT}/workloads?pageSize=100&workloadType=FINE_TUNING" \
              | jq -r --arg n "$FT_DISPLAY" '[(.data // [])[] | select((.displayName // .name // "") == $n) | .id][0] // empty' 2>/dev/null)
            if [ -n "$FT_JOB_ID" ]; then
                if [ "$FT_CREATE_RC" -ne 0 ]; then
                    warn "fine-tuning create did not return a workload id ($(curl_rc_reason $FT_CREATE_RC)) but the job exists — adopted ${FT_JOB_ID}"
                else
                    warn "fine-tuning create returned HTTP ${FT_CREATE_CODE} with no workload id but the job exists — adopted ${FT_JOB_ID}"
                fi
            fi
        fi

        if [ -n "$FT_JOB_ID" ]; then
            ok "fine-tuning job submitted (${FT_MODEL}, workload ${FT_JOB_ID})"
        elif [ "$FT_CREATE_RC" -ne 0 ]; then
            warn "fine-tuning create failed — $(curl_rc_reason $FT_CREATE_RC)"
            FT_GATE="no"
        else
            FT_CREATE_MSG=$(api_err_msg "$FT_CREATE" "no workloadId in response")
            warn "fine-tuning create returned HTTP ${FT_CREATE_CODE} — ${FT_CREATE_MSG}"
            FT_GATE="no"
        fi
    fi

    # ---- wait ----------------------------------------------------------
    if [ "$FT_GATE" = "ok" ] && [ -n "$FT_JOB_ID" ]; then
        # Wall-clock deadline rather than an accumulator: over a wait this
        # long, a counter that only adds the sleep understates elapsed time
        # by more than enough to make the reported figure wrong.
        FT_START=$(date +%s)
        FT_DEADLINE=$(( FT_START + FINETUNE_TIMEOUT ))
        FT_SAW_STARTING=0
        FT_PENDING_FOR=0
        FT_NAGGED=0
        FT_BLIND=0
        FT_BLIND_CODE=""
        FT_ELAPSED=0
        FT_STATUS=$(ft_status "$FT_JOB_ID")
        [ -z "$FT_STATUS" ] && FT_STATUS="Unknown"

        while [ "$(date +%s)" -lt "$FT_DEADLINE" ]; do
            case "$FT_STATUS" in
                Complete|Completed|Failed|Deleted) break ;;
            esac
            sleep 10
            # Training outlives the access token several times over. This is
            # the refresh that keeps a long poll from going blind halfway
            # through and leaving the job running with nobody watching.
            rm_token_refresh
            FT_STATUS=$(ft_status "$FT_JOB_ID")
            FT_ELAPSED=$(( $(date +%s) - FT_START ))

            case "$FT_STATUS" in
                '!'*)
                    FT_BLIND_CODE="${FT_STATUS#!}"
                    FT_BLIND=$(( FT_BLIND + 1 ))
                    if [ "$FT_BLIND" -ge 3 ]; then
                        FT_STATUS="Unreadable"
                        break
                    fi
                    RM_TOKEN_EXP=0
                    vinfo "  ${FT_ELAPSED}s — status unreadable (${FT_BLIND_CODE}), retrying"
                    continue
                    ;;
                *) FT_BLIND=0 ;;
            esac
            [ -z "$FT_STATUS" ] && FT_STATUS="Unknown"

            # Pending means both "just submitted" and "the queue will not
            # admit me"; only the move off it tells the two apart.
            if [ "$FT_SAW_STARTING" = "0" ]; then
                case "$FT_STATUS" in
                    Starting|Running)
                        FT_SAW_STARTING=1
                        FT_PENDING_FOR=$FT_ELAPSED
                        info "admitted after ${FT_ELAPSED}s — now ${FT_STATUS} (pulling the base model and training)"
                        ;;
                esac
            fi
            if [ "$FT_STATUS" = "Pending" ] && [ "$FT_ELAPSED" -ge 120 ] && [ "$FT_NAGGED" = "0" ]; then
                FT_NAGGED=1
                info "still Pending after ${FT_ELAPSED}s — not yet admitted, so no accelerator has been assigned; check queue admission and project quota"
            fi
            vinfo "  ${FT_ELAPSED}s — ${FT_STATUS}"
        done
        FT_ELAPSED=$(( $(date +%s) - FT_START ))

        case "$FT_STATUS" in
        Complete|Completed)
            if [ "$FT_SAW_STARTING" = "1" ] && [ "$FT_PENDING_FOR" -gt 0 ]; then
                ok "fine-tuning job completed in ${FT_ELAPSED}s (pending ${FT_PENDING_FOR}s, training $(( FT_ELAPSED - FT_PENDING_FOR ))s)"
            else
                ok "fine-tuning job completed in ${FT_ELAPSED}s"
            fi
            # The job is over, so there is nothing left to cancel — but there
            # is now a model to remove.
            FT_MODEL_ID="$FT_JOB_ID"
            FT_JOB_ID=""

            # ---- did it register a model? ---------------------------
            # A job that exits 0 without registering anything is a failure
            # this check exists to catch, so the model is looked for in the
            # project's own list and then read back by id.
            # An adopted job carries no display name of ours, so the id is
            # the only handle that always works; the name is added to the
            # pattern when there is one.
            FT_LISTED=$(ft_api_get "/v1/projects/${AIWB_PROJECT}/fine-tuning/models?pageSize=100" \
              | jq -r --arg n "$FT_DISPLAY" --arg id "$FT_MODEL_ID" '
                  ([$id] + (if $n == "" then [] else [$n] end) | join("|")) as $pat
                  | [ (.data // [])[]
                      | select(((.metadata // {}) | tostring) + " " + ((.spec // {}) | tostring)
                               | test($pat)) ]
                  | length' 2>/dev/null)
            [ -z "$FT_LISTED" ] && FT_LISTED=0
            if [ "$FT_LISTED" -gt 0 ]; then
                ok "the fine-tuned model is listed among the project's fine-tuned models"
            else
                warn "the job completed but no matching model appears in GET /v1/projects/${AIWB_PROJECT}/fine-tuning/models"
            fi

            # By id as well: the list is a label-filtered query, this reads
            # the AIMModel CR the job actually produced.
            FT_ONE="${TMPDIR_SMOKE}/ft-model.json"
            curl_to_file "${AIWB_API}/v1/projects/${AIWB_PROJECT}/fine-tuning/models/${FT_MODEL_ID}" "$FT_ONE" \
                -H "Authorization: Bearer ${RM_TOKEN}"
            if [ "$CURL_CODE" = "200" ]; then
                FT_CANON=$(jq -r '.metadata.name // .spec.model.canonicalName // empty' "$FT_ONE" 2>/dev/null | head -1)
                ok "fine-tuned model readable by id${FT_CANON:+ (${FT_CANON})}"
            else
                warn "fine-tuned model not readable at /fine-tuning/models/${FT_MODEL_ID} (HTTP ${CURL_CODE})"
                # Nothing to delete if nothing is there.
                [ "$CURL_CODE" = "404" ] && FT_MODEL_ID=""
            fi
            ;;
        Failed)
            # "It failed" is not a finding. A rejected token, an OOM and a
            # dataset the trainer could not read all end here and read
            # completely differently in the log, so the reason goes into the
            # warning.
            #
            # direction=backward is what makes this the tail: the default is
            # forward, which returns the first N entries — image pulls and
            # startup banners, never the reason. A wide window is asked for
            # and then narrowed here, because a training log is mostly
            # progress bars, ANSI redraws and traceback frames.
            #
            # The first error-shaped lines are preferred over the last ones:
            # a Python traceback ends in the generic wrapper (ChildFailedError,
            # exit code 1) and names the actual cause near its top. The final
            # log line is appended regardless — that is where the runtime
            # says what it gave up on.
            FT_LOG=$(ft_api_get "/v1/projects/${AIWB_PROJECT}/workloads/${FT_JOB_ID}/logs?start=$(date -u -d "@$(( FT_START - 60 ))" +%Y-%m-%dT%H:%M:%SZ)&end=$(date -u +%Y-%m-%dT%H:%M:%SZ)&limit=1000&direction=backward" \
              | jq -r '
                  [ (.data // [])
                    | sort_by(.timestamp // "")[]
                    | (.message // .log // empty)
                    | gsub("\u001b\\[[0-9;?]*[A-Za-z]"; "")
                    | gsub("[\r\t]"; " ")
                    | gsub("^ +| +$"; "")
                    | select(length > 0)
                    | select(test("[0-9]+%\\|") | not)
                    | select(test("^[=_ -]+$") | not)
                  ]
                  | map(select(test("File \"[^\"]*\", line [0-9]+") | not))
                  | map(sub("^\\[rank[0-9]+\\]: *"; ""))
                  | map(select(test("^raise ") | not))
                  | . as $all
                  | (map(select(test("(?i)error|exception|fatal|refused|denied|out of memory|no space|not found|invalid|unauthor"))) | map(select(test("(?i)^traceback|to enable traceback|the above exception") | not))) as $bad
                  | (if ($bad | length) > 0 then ($bad[:3] + $all[-1:]) else $all[-5:] end)
                  | map(if length > 160 then .[:160] + "..." else . end)
                  | join(" | ")' 2>/dev/null)
            warn "fine-tuning job failed after ${FT_ELAPSED}s${FT_LOG:+ — ${FT_LOG}}"
            ;;
        Unreadable)
            warn "lost the ability to read job status after ${FT_ELAPSED}s (HTTP ${FT_BLIND_CODE}) — the job may still be training; it is cancelled below either way"
            ;;
        Deleted)
            warn "fine-tuning job disappeared while running — something else deleted it"
            FT_JOB_ID=""
            ;;
        *)
            if [ "$FT_SAW_STARTING" = "1" ]; then
                warn "fine-tuning job still ${FT_STATUS} after ${FT_ELAPSED}s — it was admitted but training did not finish (raise --finetune-timeout)"
            else
                warn "fine-tuning job never left Pending after ${FT_ELAPSED}s — it was never admitted, so no accelerator was assigned; check queue admission and project quota"
            fi
            ;;
        esac
    fi

    # ---- clean up ------------------------------------------------------
    # Inline, so the run reports what happened; the exit trap covers only the
    # paths that never reach here.
    if [ "$KEEP_FINETUNE" = "1" ]; then
        [ -n "$FT_MODEL_ID" ] && info "fine-tuned model ${FT_MODEL_ID} left in '${AIWB_PROJECT}' (--keep-finetune) — delete it manually"
        [ -n "$DS_ID" ] && info "dataset ${DS_ID} left in '${AIWB_PROJECT}' with it — the model records it as its training data"
        FT_MODEL_ID=""
        DS_ID=""
    else
        if [ -n "$FT_JOB_ID" ]; then
            rm_token_refresh
            FT_CANCEL=$(curl_probe "${AIWB_API}/v1/projects/${AIWB_PROJECT}/fine-tuning/jobs/${FT_JOB_ID}" \
                          -X DELETE -H "Authorization: Bearer ${RM_TOKEN}"; printf '%s' "$CURL_CODE")
            case "$FT_CANCEL" in
                200|202|204|404) ok "fine-tuning job cancelled (HTTP ${FT_CANCEL}) — the accelerator is released"; FT_JOB_ID="" ;;
                *) warn "cancelling the fine-tuning job returned HTTP ${FT_CANCEL} — ${FT_JOB_ID} may still be holding an accelerator" ;;
            esac
        fi
        if [ -n "$FT_MODEL_ID" ]; then
            rm_token_refresh
            FT_DEL=$(curl_probe "${AIWB_API}/v1/projects/${AIWB_PROJECT}/fine-tuning/models/${FT_MODEL_ID}" \
                       -X DELETE -H "Authorization: Bearer ${RM_TOKEN}"; printf '%s' "$CURL_CODE")
            # 409 means something is deployed on it. Nothing here deploys the
            # model, so a conflict is stale state rather than a live user, and
            # force is the honest way past it.
            if [ "$FT_DEL" = "409" ]; then
                FT_DEL=$(curl_probe "${AIWB_API}/v1/projects/${AIWB_PROJECT}/fine-tuning/models/${FT_MODEL_ID}?force=true" \
                           -X DELETE -H "Authorization: Bearer ${RM_TOKEN}"; printf '%s' "$CURL_CODE")
            fi
            case "$FT_DEL" in
                200|202|204|404) ok "fine-tuned model deleted (HTTP ${FT_DEL})"; FT_MODEL_ID="" ;;
                *) warn "fine-tuned model delete returned HTTP ${FT_DEL} — ${FT_MODEL_ID} may still be in '${AIWB_PROJECT}'" ;;
            esac
        fi
        if [ -n "$DS_ID" ]; then
            rm_token_refresh
            FT_DSDEL=$(ds_api DELETE "/datasets/${DS_ID}")
            case "$FT_DSDEL" in
                200|202|204|404) ok "training dataset deleted (HTTP ${FT_DSDEL})"; DS_ID="" ;;
                *) warn "training dataset delete returned HTTP ${FT_DSDEL} — ${DS_ID} may still be in '${AIWB_PROJECT}'" ;;
            esac
        fi
    fi
    if [ -n "$FT_SECRET" ]; then
        # Written by this run and holding a Hugging Face token, so it goes
        # whether or not the model is being kept.
        rm_token_refresh
        curl_to_file "${AIWB_API}/v1/projects/${AIWB_PROJECT}/secrets/${FT_SECRET}" /dev/null \
            -X DELETE -H "Authorization: Bearer ${RM_TOKEN}"
        case "${CURL_RC}:${CURL_CODE}" in
            0:200|0:202|0:204|0:404)
                ok "Hugging Face token secret removed (HTTP ${CURL_CODE})"
                FT_SECRET=""
                ;;
            *)
                warn "could not remove the Hugging Face token secret '${FT_SECRET}' from '${AIWB_PROJECT}' — delete it manually, it holds a token"
                ;;
        esac
    fi
fi


# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
smoke_summary "$SMOKE_LAYER"
exit $?
