#!/usr/bin/env bash
#
# lib-smoketest.sh — shared plumbing for the EAI / AIRM smoke tests.
#
# Sourced by smoketest-cluster.sh (substrate layer) and smoketest-platform.sh
# (cluster-forge layer). Not executable on its own.
#
# DELIBERATELY does not use `set -e`, and neither should its callers. Every
# check runs to completion even when an earlier one fails; problems are
# collected and printed as a summary at the end. The ONLY early exit is when
# something makes all (or nearly all) checks impossible: a missing required
# tool, or no usable cluster access.
#
# Four outcomes, and the distinction between the last two matters:
#   ok       the check ran and passed
#   warn     the check ran and found a fault
#   skip     the check was not applicable or was not asked for — a small
#            cluster has no Longhorn, --skip-certs was passed, --with-model
#            was not. NOT a pass, but not a failure either.
#   blocked  the check was in scope and could not run — no credentials, no
#            domain, forbidden, or an opt-in check that WAS asked for and
#            could not run anyway. Counts against a clean result.
#
# Requires : bash 4+, kubectl, curl, jq      (openssl optional — cert expiry)
#
# EAI-5860
#

SMOKE_LIB_VERSION="0.6.0"

# ---------------------------------------------------------------------------
# Shared defaults — a caller may override any of these before smoke_preflight
# ---------------------------------------------------------------------------
DOMAIN="${DOMAIN:-}"
INSECURE=0
USE_COLOR=1
VERBOSE=0
HTTP_TIMEOUT=10
KUBE_TIMEOUT=15

TMPDIR_SMOKE=""
KUBECTL_GLOBAL=()
PLATFORM="unknown"
CLUSTER_OK=0
CLUSTER_SIZE=""

# Namespaces — verified against cluster-forge / cluster-bloom sources.
# Note longhorn is "longhorn", NOT "longhorn-system".
NS_ARGOCD="argocd"
NS_KEYCLOAK="keycloak"
NS_LONGHORN="longhorn"
NS_SEAWEEDFS="seaweedfs-instance"
NS_ENVOY="envoy-gateway-system"
NS_METALLB="metallb-system"
NS_GITEA="cf-gitea"
NS_OPENBAO="cf-openbao"
NS_KUBESYSTEM="kube-system"
NS_AIRM="airm"
NS_AIWB="aiwb"

# ---------------------------------------------------------------------------
# Output helpers
#
# Colours depend on --no-color, which is parsed by the caller, so they are set
# by smoke_init_output() rather than at source time.
# ---------------------------------------------------------------------------
C_RED=""; C_GRN=""; C_YEL=""; C_BLU=""; C_DIM=""; C_RST=""

smoke_init_output() {
    if [ "$USE_COLOR" = "1" ] && [ -t 1 ]; then
        C_RED=$'\033[0;31m'; C_GRN=$'\033[0;32m'; C_YEL=$'\033[1;33m'
        C_BLU=$'\033[0;34m'; C_DIM=$'\033[2m';    C_RST=$'\033[0m'
    fi
}

OK_COUNT=0
WARN_COUNT=0
SKIP_COUNT=0
BLOCKED_COUNT=0
FINDINGS=()

# ---------------------------------------------------------------------------
# Timing — total runtime plus a per-section breakdown, so a slow run can be
# attributed rather than guessed at. The ticket budgets 10 minutes.
# ---------------------------------------------------------------------------
now_ms() { date +%s%3N; }
RUN_START=$(now_ms)
SECTION_NAME=""
SECTION_START=0
SECTION_TIMES=()

fmt_ms() {
    local ms="$1"
    if [ "$ms" -lt 1000 ]; then printf '%dms' "$ms"
    else printf '%d.%01ds' $((ms/1000)) $(( (ms%1000)/100 )); fi
}

section_close() {
    [ -z "$SECTION_NAME" ] && return 0
    SECTION_TIMES+=("$(( $(now_ms) - SECTION_START ))|${SECTION_NAME}")
    SECTION_NAME=""
}

section() {
    section_close
    SECTION_NAME="$1"; SECTION_START=$(now_ms)
    printf '\n%s── %s %s%s\n' "$C_BLU" "$1" "$(printf '─%.0s' $(seq 1 $((60 - ${#1}))))" "$C_RST"
}

# Numbered sections. The counter keeps the numbering correct however many
# sections a run actually reaches.
SEC_N=0
section_n() {
    SEC_N=$((SEC_N + 1))
    section "${SEC_N} · $1"
}
ok()   { OK_COUNT=$((OK_COUNT+1));     printf '  %sOK%s    %s\n' "$C_GRN" "$C_RST" "$1"; }
warn() { WARN_COUNT=$((WARN_COUNT+1)); printf '  %sWARN%s  %s\n' "$C_YEL" "$C_RST" "$1"; FINDINGS+=("WARN  $1"); }
skip() { SKIP_COUNT=$((SKIP_COUNT+1)); printf '  %sSKIP%s  %s\n' "$C_DIM" "$C_RST" "$1"; FINDINGS+=("SKIP  $1"); }
info() { printf '  %s·%s     %s\n' "$C_DIM" "$C_RST" "$1"; }
vinfo(){ [ "$VERBOSE" = "1" ] && printf '        %s%s%s\n' "$C_DIM" "$1" "$C_RST"; return 0; }

# A check that was in scope and could not run. Distinct from skip(): a skip is
# "not applicable or not asked for", which is a legitimate outcome; blocked is
# "should have run, could not", which is not. Kept out of the pass column so a
# run that tested nothing cannot report clean.
blocked() {
    BLOCKED_COUNT=$((BLOCKED_COUNT+1))
    printf '  %sBLOCKED%s  %s\n' "$C_RED" "$C_RST" "$1"
    FINDINGS+=("BLOCKED  $1")
}
# ---------------------------------------------------------------------------
# kubectl wrapper
#
# Classifies failures so a permission problem is reported as what it is — a
# check that was in scope and could not run — rather than as a finding.
# ---------------------------------------------------------------------------
kc() { kubectl "${KUBECTL_GLOBAL[@]}" --request-timeout="${KUBE_TIMEOUT}s" "$@"; }

# Failure status is recorded in a file, not a variable: kc_json is almost always
# called inside $( ), and a variable set in that subshell never reaches the
# caller. kc_status/kc_err read it back after the substitution has finished.
kc_status() { cat "${TMPDIR_SMOKE}/kc_last_status" 2>/dev/null; }
kc_err()    { cat "${TMPDIR_SMOKE}/kc_last_err"    2>/dev/null; }

kc_json() {
    local out rc status
    out=$(kc "$@" -o json 2>&1); rc=$?
    if [ $rc -ne 0 ]; then
        if printf '%s' "$out" | grep -qiE 'forbidden|is not allowed|Unauthorized'; then
            status="forbidden"
        elif printf '%s' "$out" | grep -qiE 'not found|doesn.t have a resource type|could not find|no matches for kind'; then
            status="notfound"
        else
            status="error"
        fi
        printf '%s' "$status" > "${TMPDIR_SMOKE}/kc_last_status"
        printf '%s' "$out"    > "${TMPDIR_SMOKE}/kc_last_err"
        return 1
    fi
    printf 'ok' > "${TMPDIR_SMOKE}/kc_last_status"
    : > "${TMPDIR_SMOKE}/kc_last_err"
    printf '%s' "$out"
    return 0
}

# Memoised kc_json. Several collections are needed by more than one section
# (kube-system pods, storageclasses, all pods). Fetching once also means every
# section reasons about the SAME snapshot, so two sections can't disagree.
kc_json_cached() {
    local key file status
    key=$(printf '%s_' "$@" | tr -c 'A-Za-z0-9' '_')
    file="${TMPDIR_SMOKE}/cache_${key}"

    if [ -f "${file}.status" ]; then
        # Replay the cached outcome, including how it failed.
        status=$(cat "${file}.status")
        printf '%s' "$status" > "${TMPDIR_SMOKE}/kc_last_status"
        cp "${file}.err" "${TMPDIR_SMOKE}/kc_last_err" 2>/dev/null || : > "${TMPDIR_SMOKE}/kc_last_err"
        echo 1 >> "${TMPDIR_SMOKE}/cache_hits"
        [ "$status" != "ok" ] && return 1
        cat "$file"
        return 0
    fi

    local out rc
    out=$(kc_json "$@"); rc=$?
    cp "${TMPDIR_SMOKE}/kc_last_status" "${file}.status" 2>/dev/null
    cp "${TMPDIR_SMOKE}/kc_last_err"    "${file}.err"    2>/dev/null
    [ $rc -ne 0 ] && return 1
    printf '%s' "$out" > "$file"
    printf '%s' "$out"
    return 0
}

# The server's own message for a failed API call, on ONE line.
#
# A finding is a single line by construction, and the summary repeats it, so a
# multi-line message breaks both. FastAPI puts an *array* of validation errors
# in .detail — jq -r prints that across a dozen lines, and truncating by bytes
# trims the length without removing the newlines. Each entry is rendered as
# "msg (body.field)", which is the part a reader can act on; anything else is
# stringified. Whitespace is then collapsed and the result capped.
api_err_msg() {
    local file="$1" fallback="${2:-no message in response}" msg
    msg=$(jq -r '
        (.detail // .message // .error // empty)
        | if   type == "string" then .
          elif type == "array"  then
              [ .[]
                | if type == "object"
                  then ((.msg // (. | tostring))
                        + (if (.loc? | type) == "array"
                           then " (" + ([.loc[] | tostring] | join(".")) + ")"
                           else "" end))
                  else tostring end ]
              | join("; ")
          else tostring end' "$file" 2>/dev/null \
        | tr '\n\t' '  ' | tr -s ' ' | sed 's/^ *//; s/ *$//' | cut -c1-200)
    printf '%s' "${msg:-$fallback}"
}

# Report a kc_json failure in the right category.
# Callers that KNOW a resource is not readable on this cluster say so
# themselves with skip() before reaching here, so a
# forbidden that gets this far is unexpected: the check was in scope and could
# not run, which is blocked, not skipped.
kc_report_failure() {
    local what="$1" hint="${2:-}"
    case "$(kc_status)" in
        forbidden) blocked "$what — permission denied${hint:+ ($hint)}" ;;
        notfound)  warn "$what — resource or namespace not found" ;;
        *)         warn "$what — kubectl error: $(kc_err | head -1)" ;;
    esac
}

# True when the namespace exists. Used to tell "not deployed on this cluster"
# apart from "deployed but broken" — a missing component is a SKIP, not a finding.
# The namespace list is fetched once instead of once per lookup.
NS_LIST=""
NS_LIST_FETCHED=0
ns_exists() {
    if [ "$NS_LIST_FETCHED" = "0" ]; then
        NS_LIST=$(kc get namespaces -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)
        NS_LIST_FETCHED=1
    fi
    # Fall back to a direct lookup if the list could not be read: a restricted
    # kubeconfig may allow getting a named namespace but not listing them.
    if [ -z "$NS_LIST" ]; then
        kc get namespace "$1" >/dev/null 2>&1
        return $?
    fi
    printf '%s\n' $NS_LIST | grep -qx -- "$1"
}

# Read one key of one secret through the current kubectl context. Used to fill
# in Resource Manager credentials before any mode switch has happened. Quiet:
# a missing secret is an expected outcome, not an error.
# jq rather than jsonpath for the key lookup. A key containing a dot —
# ca.crt, tls.key — reads as two path segments in {.data.ca.crt}, so jsonpath
# looks for a nested object, finds nothing, and returns an empty string: the
# same answer it gives for a secret that has no such key at all, which makes an
# unreadable value indistinguishable from a missing one. (Hyphens are fine;
# it is only the dot that splits.) Indexing by the literal key avoids it.
admin_secret_value() {
    kubectl --request-timeout="${KUBE_TIMEOUT}s" get secret "$2" -n "$1" \
        -o json 2>/dev/null | jq -r --arg k "$3" '.data[$k] // empty' 2>/dev/null \
        | base64 -d 2>/dev/null
}

# ---------------------------------------------------------------------------
# HTTP helpers
# ---------------------------------------------------------------------------
CURL_CODE=""
CURL_RC=0
curl_probe() {
    local url="$1"; shift
    local tls=()
    [ "$INSECURE" = "1" ] && tls=(-k)
    CURL_CODE=$(curl -s -o /dev/null -w '%{http_code}' --max-time "$HTTP_TIMEOUT" \
                     "${tls[@]}" "$@" "$url" 2>/dev/null)
    CURL_RC=$?
    return $CURL_RC
}

curl_body() {
    local url="$1"; shift
    local tls=()
    [ "$INSECURE" = "1" ] && tls=(-k)
    curl -s --max-time "$HTTP_TIMEOUT" "${tls[@]}" "$@" "$url" 2>/dev/null
}

# Body to a file, HTTP status in CURL_CODE. Needed where both matter — an
# upload's response and a download's bytes — since curl_body drops the status
# and curl_probe drops the body.
curl_to_file() {
    local url="$1" out="$2"; shift 2
    local tls=()
    [ "$INSECURE" = "1" ] && tls=(-k)
    CURL_CODE=$(curl -s -o "$out" -w '%{http_code}' --max-time "$HTTP_TIMEOUT" \
                     "${tls[@]}" "$@" "$url" 2>/dev/null)
    CURL_RC=$?
    return $CURL_RC
}

curl_rc_reason() {
    case "$1" in
        6)  echo "DNS resolution failed" ;;
        7)  echo "connection refused" ;;
        28) echo "timed out after ${HTTP_TIMEOUT}s" ;;
        35) echo "TLS handshake failed" ;;
        51) echo "TLS certificate host mismatch" ;;
        60) echo "TLS certificate not trusted (Zscaler CA missing? try --insecure)" ;;
        *)  echo "curl exit $1" ;;
    esac
}

host_for() {
    # host_for <prefix> — returns "<prefix>.<domain>" if a domain is known
    [ -z "$DOMAIN" ] && return 1
    printf '%s.%s' "$1" "$DOMAIN"
}

# ---------------------------------------------------------------------------
# Shared pod-health helper
# ---------------------------------------------------------------------------
# Prints unhealthy pods, one per line: "<name> <phase> <reason>"
unhealthy_pods_json() {
    jq -r '
      .items[]?
      | select(.status.phase != "Succeeded")
      # Job/CronJob pods are transient by design — a heartbeat pod caught mid
      # ContainerCreating is not a fault, and its failures belong to the Job.
      | select([.metadata.ownerReferences[]? | select(.kind == "Job")] | length == 0)
      # Helm test-hook pods are install-time artifacts with restartPolicy Never
      # and no controller: one that lost a startup race stays Failed for the life
      # of the release and would otherwise be reported as a fault on every run
      # forever. A rerun of `helm test` is what re-evaluates them, not this.
      | select((.metadata.annotations["helm.sh/hook"] // "") | test("test") | not)
      | select(
          .status.phase != "Running"
          or ([.status.containerStatuses[]? | select(.ready != true)] | length > 0)
        )
      # The reason, not the state key: "ImagePullBackOff" is actionable where
      # "waiting" is not. Init containers are included because a pod blocked in
      # init reports only "PodInitializing" on its app container, which hides
      # the very thing that stopped it — an init container that cannot pull.
      # PodInitializing is therefore used only when nothing better is offered.
      | ([ ((.status.initContainerStatuses // [])[]
            | select((.state.waiting != null) or ((.state.terminated.exitCode // 0) != 0))
            | (.state.waiting.reason // .state.terminated.reason)),
           ((.status.containerStatuses // [])[]
            | select(.ready != true)
            | (.state.waiting.reason // .state.terminated.reason // (.state | keys[0]))) ]
          | map(select(. != null))) as $r
      | (($r | map(select(. != "PodInitializing")) | first) // ($r | first) // "notready") as $why
      | "\(.metadata.name) \(.status.phase) \($why)"
    ' 2>/dev/null
}

# Pods for one namespace, taken from a single cluster-wide fetch that every
# section shares. Six per-namespace round trips collapse into one.
# A restricted kubeconfig may allow reading pods in a namespace but not
# listing them cluster-wide, so a forbidden -A fetch falls back to
# per-namespace queries permanently (the outcome will not change mid-run).
pods_for_ns() {
    local ns="$1" all
    if [ "$(cat "${TMPDIR_SMOKE}/pods_all_usable" 2>/dev/null)" != "no" ]; then
        if all=$(kc_json_cached get pods -A 2>/dev/null); then
            printf 'ok' > "${TMPDIR_SMOKE}/kc_last_status"
            printf '%s' "$all" | jq --arg ns "$ns" \
                '{items: [.items[]? | select(.metadata.namespace == $ns)]}'
            return 0
        fi
        echo no > "${TMPDIR_SMOKE}/pods_all_usable"
    fi
    kc_json get pods -n "$ns"
}

check_ns_pods() {
    local ns="$1" label="$2" selector="${3:-}"
    local json
    if ! ns_exists "$ns"; then
        skip "$label — namespace ${ns} not present (component not deployed on this cluster)"
        return 2
    fi
    if [ -n "$selector" ]; then
        json=$(kc_json get pods -n "$ns" -l "$selector" 2>/dev/null)
    else
        json=$(pods_for_ns "$ns" 2>/dev/null)
    fi
    if [ $? -ne 0 ]; then
        kc_report_failure "$label pods in ns/${ns}"
        return 1
    fi
    local total bad
    total=$(printf '%s' "$json" | jq '[.items[]?] | length' 2>/dev/null)
    if [ "${total:-0}" = "0" ]; then
        warn "$label — no pods found in ns/${ns}"
        return 1
    fi
    bad=$(printf '%s' "$json" | unhealthy_pods_json)
    if [ -z "$bad" ]; then
        ok "$label — ${total}/${total} pods healthy in ns/${ns}"
        return 0
    fi
    local n; n=$(printf '%s\n' "$bad" | grep -c .)
    warn "$label — ${n}/${total} pods unhealthy in ns/${ns}"
    printf '%s\n' "$bad" | while read -r line; do [ -n "$line" ] && info "  $line"; done
    return 1
}

# Compare the desired state of a namespace's controllers against what is actually
# running. A pod-level check alone cannot see this: a Deployment scaled to 3 with
# 1 pod running looks perfectly healthy if you only list pods.
workload_shortfall() {
    # $1 = kubectl resource, $2 = kind label, $3 = namespace,
    # $4 = jq path to desired, $5 = jq path to ready
    local json
    json=$(kc_json get "$1" -n "$3" 2>/dev/null) || return 1
    printf '%s' "$json" | jq -r --arg k "$2" --arg d "$4" --arg r "$5" '
        [.items[]?] as $items
        | ($items | length) as $n
        | [ $items[]
            | (getpath($d | split(".")) // 0) as $want
            | (getpath($r | split(".")) // 0) as $have
            | select($want > 0 and $have < $want)
            | "\($k)/\(.metadata.name) ready \($have)/\($want)" ] as $bad
        | "\($n)\t\($bad | length)\t\($bad | join("|"))"' 2>/dev/null
}

check_ns_workloads() {
    local ns="$1" label="$2"
    if ! ns_exists "$ns"; then
        skip "$label — namespace ${ns} not present (component not deployed on this cluster)"
        return 2
    fi
    local kinds="deployment:Deployment:spec.replicas:status.readyReplicas
statefulset:StatefulSet:spec.replicas:status.readyReplicas
daemonset:DaemonSet:status.desiredNumberScheduled:status.numberReady"
    local total=0 shortfall=0 details="" line res kind want have out
    while IFS=: read -r res kind want have; do
        [ -z "$res" ] && continue
        out=$(workload_shortfall "$res" "$kind" "$ns" "$want" "$have")
        if [ $? -ne 0 ] || [ -z "$out" ]; then
            kc_report_failure "$label ${kind}s in ns/${ns}"
            return 1
        fi
        total=$((total + $(printf '%s' "$out" | cut -f1)))
        shortfall=$((shortfall + $(printf '%s' "$out" | cut -f2)))
        line=$(printf '%s' "$out" | cut -f3)
        [ -n "$line" ] && details="${details}${line}|"
    done <<EOF
$kinds
EOF
    if [ "$total" = "0" ]; then
        warn "$label — no Deployments, StatefulSets or DaemonSets in ns/${ns}"
        return 1
    fi
    if [ "$shortfall" = "0" ]; then
        ok "$label — all ${total} workload(s) at desired replicas in ns/${ns}"
        return 0
    fi
    warn "$label — ${shortfall}/${total} workload(s) below desired replicas in ns/${ns}"
    printf '%s\n' "${details%|}" | tr '|' '\n' | while read -r l; do [ -n "$l" ] && info "  $l"; done
    return 1
}

# ---------------------------------------------------------------------------
# Dangling in-cluster service references
#
# A workload's environment names the services it calls. When a component is
# removed from the platform and its callers are not updated, the pointer stays
# behind and nothing notices until something tries to use it — at which point
# the caller reports a generic 5xx and names nothing. Reading the pointers and
# checking the Service exists turns that into a finding at the layer that owns
# it, seconds into a read-only run.
#
# Only literal env values can be read; a valueFrom reference is skipped rather
# than guessed at. A URL whose <PREFIX>_ENABLED is explicitly "false" is
# skipped too — the feature is off, so a missing Service is not a finding.
# Hosts that are not in-cluster (anything with a dot that is not *.svc) are
# left alone: this check is about the platform's own components.
# ---------------------------------------------------------------------------
check_ns_service_refs() {
    local ns="$1" label="$2" json refs missing="" total=0 svc_all=""

    json=$(kc_json get deployments,statefulsets -n "$ns") || return 0

    refs=$(printf '%s' "$json" | jq -r --arg ns "$ns" '
        .items[]?
        | .metadata.name as $w
        | .spec.template.spec.containers[]?
        | ([.env[]? | select(.value != null) | {key: .name, value: .value}] | from_entries) as $env
        | $env
        | to_entries[]
        | select(.value | test("^https?://"))
        | . as $kv
        | ($kv.key | sub("_(URL|URI|ENDPOINT|ADDRESS|ADDR|HOST)$"; "")) as $prefix
        | select((($env[$prefix + "_ENABLED"]) // "true") | ascii_downcase != "false")
        | ($kv.value | capture("^https?://(?<h>[^/:]+)").h) as $host
        | select($host | test("^(localhost|127\\.0\\.0\\.1|::1)") | not)
        | if   ($host | test("\\.svc(\\.cluster\\.local)?$")) then ($host | split(".")) | "\($w)\t\(.[0])\t\(.[1])\tsvc"
          elif ($host | test("^[^.]+$"))                      then "\($w)\t\($host)\t\($ns)\tsvc"
          elif ($host | test("^[^.]+\\.[^.]+$"))              then ($host | split(".")) | "\($w)\t\(.[0])\t\(.[1])\tmaybe"
          else empty end
        ' 2>"${TMPDIR_SMOKE}/svcref_err" | sort -u)

    if [ -s "${TMPDIR_SMOKE}/svcref_err" ]; then
        warn "$label — could not read service references in ns/${ns}: $(head -1 "${TMPDIR_SMOKE}/svcref_err")"
        return 1
    fi
    [ -z "$refs" ] && return 0

    # One cluster-wide fetch beats one lookup per reference. A kubeconfig that
    # cannot list services cluster-wide falls back to a lookup per reference.
    svc_all=$(kc get services -A -o jsonpath='{range .items[*]}{.metadata.namespace}/{.metadata.name}{"\n"}{end}' 2>/dev/null)

    local w svc sns kind
    while IFS=$'\t' read -r w svc sns kind; do
        [ -z "$svc" ] && continue
        # "name.namespace" is a real in-cluster address, but it is also the
        # shape of an ordinary two-label domain. Treat it as in-cluster only
        # when that second label is actually a namespace here; otherwise it is
        # someone's external host and none of this check's business.
        [ "$kind" = "maybe" ] && ! ns_exists "$sns" && continue
        total=$((total + 1))
        if [ -n "$svc_all" ]; then
            printf '%s\n' "$svc_all" | grep -qx -- "${sns}/${svc}" && continue
        else
            kc get service "$svc" -n "$sns" >/dev/null 2>&1 && continue
        fi
        missing="${missing}${w} calls ${svc}.${sns}.svc, which does not exist|"
    done <<< "$refs"

    if [ -z "$missing" ]; then
        ok "$label — all ${total} in-cluster service reference(s) resolve in ns/${ns}"
        return 0
    fi

    local n
    n=$(printf '%s' "${missing%|}" | tr '|' '\n' | grep -c .)
    warn "$label — ${n} of ${total} in-cluster service reference(s) point at a Service that is not deployed"
    printf '%s\n' "${missing%|}" | tr '|' '\n' | while read -r l; do [ -n "$l" ] && info "  $l"; done
    return 1
}

# ---------------------------------------------------------------------------
# Cleanup
#
# The substrate checks create nothing, so the base cleanup only removes the
# temp dir. smoketest-platform.sh sets SMOKE_CLEANUP_HOOK to the function that
# deletes whatever its --with-* checks created.
# ---------------------------------------------------------------------------
SMOKE_CLEANUP_HOOK=""
SMOKE_INTERRUPT_HOOK=""

smoke_cleanup() {
    [ -n "$SMOKE_CLEANUP_HOOK" ] && "$SMOKE_CLEANUP_HOOK"
    [ -n "$TMPDIR_SMOKE" ] && [ -d "$TMPDIR_SMOKE" ] && rm -rf "$TMPDIR_SMOKE"
    return 0
}

smoke_interrupted() {
    printf '\n  %sInterrupted.%s\n' "$C_YEL" "$C_RST" >&2
    smoke_cleanup
    [ -n "$SMOKE_INTERRUPT_HOOK" ] && "$SMOKE_INTERRUPT_HOOK"
    return 0
}

smoke_install_traps() {
    # A handler that does not exit lets bash resume where it was interrupted,
    # so INT and TERM get their own that leave for real.
    trap smoke_cleanup EXIT
    trap 'smoke_interrupted; exit 130' INT
    trap 'smoke_interrupted; exit 143' TERM
}

# ---------------------------------------------------------------------------
# Preflight — required tools
#
# The one place an early exit is right: without kubectl, curl and jq nothing
# downstream can run at all. Exits 2 (blocked), not 0.
# ---------------------------------------------------------------------------
HAVE_OPENSSL=0
smoke_preflight() {
    section "Preflight"

    local missing=0 t
    for t in kubectl curl jq; do
        if command -v "$t" >/dev/null 2>&1; then
            vinfo "$t: $(command -v "$t")"
        else
            blocked "required tool missing: $t"
            missing=1
        fi
    done
    [ "$missing" = "0" ] && ok "required tools present (kubectl, curl, jq)"

    if command -v openssl >/dev/null 2>&1; then
        HAVE_OPENSSL=1
    else
        info "openssl not found — certificate expiry checks will be skipped"
    fi

    if [ "$missing" = "1" ]; then
        printf '\n%sCannot continue without kubectl, curl and jq.%s\n' "$C_RED" "$C_RST"
        exit 2
    fi

    TMPDIR_SMOKE=$(mktemp -d 2>/dev/null || mktemp -d -t smoketest)
    [ "$INSECURE" = "1" ] && info "TLS verification disabled (--insecure)"
    return 0
}

# ---------------------------------------------------------------------------
# Cluster access — an admin context is the only way in.
#
# No working context blocks every remaining check, so this is the second and
# last early exit.
# ---------------------------------------------------------------------------
smoke_cluster_access() {
    section "Cluster access"

    if ! kubectl --request-timeout=10s get --raw='/version' >/dev/null 2>&1; then
        blocked "no usable kubectl context — set KUBECONFIG or select a context"
        printf '\n%sCannot continue without cluster access.%s\n' "$C_RED" "$C_RST"
        smoke_summary "${SMOKE_LAYER:-cluster}"
        exit 2
    fi
    ok "using existing kubectl context: $(kubectl config current-context 2>/dev/null || echo 'n/a')"

    local version_raw srv_ver
    version_raw=$(kc get --raw='/version' 2>/dev/null)
    if [ -n "$version_raw" ]; then
        CLUSTER_OK=1
        srv_ver=$(printf '%s' "$version_raw" | jq -r '.gitVersion // empty')
        [ -n "$srv_ver" ] && info "API server: ${srv_ver}"
    else
        blocked "cluster reachable check failed"
    fi
    return 0
}

# ---------------------------------------------------------------------------
# Summary
#
# Prints the findings, the runtime, and one machine-readable result line the
# justfile wrapper collects when both layers run:
#
#   SMOKETEST_RESULT <layer> ok=N warn=N skip=N blocked=N
#
# Exit code (returned, not exited — the caller decides):
#   0  clean
#   1  warnings
#   2  something in scope could not run
# ---------------------------------------------------------------------------
smoke_summary() {
    local layer="$1"
    section "Summary"
    section_close

    printf '  %sOK%s   %d    %sWARN%s %d    %sSKIP%s %d    %sBLOCKED%s %d\n' \
        "$C_GRN" "$C_RST" "$OK_COUNT" \
        "$C_YEL" "$C_RST" "$WARN_COUNT" \
        "$C_DIM" "$C_RST" "$SKIP_COUNT" \
        "$C_RED" "$C_RST" "$BLOCKED_COUNT"

    if [ "$WARN_COUNT" -gt 0 ] || [ "$SKIP_COUNT" -gt 0 ] || [ "$BLOCKED_COUNT" -gt 0 ]; then
        printf '\n'
        local f
        for f in "${FINDINGS[@]}"; do
            case "$f" in
                WARN*)    printf '  %s%s%s\n' "$C_YEL" "$f" "$C_RST" ;;
                BLOCKED*) printf '  %s%s%s\n' "$C_RED" "$f" "$C_RST" ;;
                *)        printf '  %s%s%s\n' "$C_DIM" "$f" "$C_RST" ;;
            esac
        done
    fi

    local run_ms run_s
    run_ms=$(( $(now_ms) - RUN_START ))
    run_s=$(( run_ms / 1000 ))

    if [ "$VERBOSE" = "1" ] && [ ${#SECTION_TIMES[@]} -gt 0 ]; then
        printf '\n  %sTime by section (slowest first)%s\n' "$C_DIM" "$C_RST"
        printf '%s\n' "${SECTION_TIMES[@]}" | sort -t'|' -k1 -rn | head -8 | \
          while IFS='|' read -r ms name; do
            printf '    %s%8s  %s%s\n' "$C_DIM" "$(fmt_ms "$ms")" "$name" "$C_RST"
          done
        printf '    %s%8s  %s%s\n' "$C_DIM" \
          "$(grep -c . "${TMPDIR_SMOKE}/cache_hits" 2>/dev/null || echo 0)" \
          "kubectl queries served from cache" "$C_RST"
    fi

    printf '\n  Runtime: %dm %02ds' $((run_s / 60)) $((run_s % 60))
    # The ticket budgets 10 minutes for the whole thing; say so plainly when
    # the budget is blown.
    if [ "$run_s" -gt 600 ]; then
        printf ' %s— over the 10 minute budget%s' "$C_YEL" "$C_RST"
    elif [ "$run_s" -gt 480 ]; then
        printf ' %s— approaching the 10 minute budget%s' "$C_YEL" "$C_RST"
    fi
    printf '\n\n'

    if [ "$BLOCKED_COUNT" -gt 0 ]; then
        printf '  %s%d check(s) could not run — this run does not prove the layer is healthy.%s\n' \
            "$C_RED" "$BLOCKED_COUNT" "$C_RST"
    elif [ "$WARN_COUNT" -gt 0 ]; then
        printf '  %s%d warning(s) — review above.%s\n' "$C_YEL" "$WARN_COUNT" "$C_RST"
    else
        printf '  %sNo warnings.%s' "$C_GRN" "$C_RST"
        [ "$SKIP_COUNT" -gt 0 ] && printf ' %d check(s) skipped (not applicable or not requested).' "$SKIP_COUNT"
        printf '\n'
    fi

    # Machine-readable, for `just all`. Always last, always one line.
    printf '\nSMOKETEST_RESULT %s ok=%d warn=%d skip=%d blocked=%d\n' \
        "$layer" "$OK_COUNT" "$WARN_COUNT" "$SKIP_COUNT" "$BLOCKED_COUNT"

    if [ "$BLOCKED_COUNT" -gt 0 ]; then return 2; fi
    if [ "$WARN_COUNT" -gt 0 ];    then return 1; fi
    return 0
}
