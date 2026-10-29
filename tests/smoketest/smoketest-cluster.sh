#!/usr/bin/env bash
#
# smoketest-cluster.sh — substrate health checks for EAI / AIRM clusters.
#
# The layer BELOW cluster-forge: nodes, control plane, CNI, DNS and storage —
# whatever produces a working Kubernetes for the platform to be deployed onto.
# Today that layer is produced by cluster-bloom on RKE2. The file is named for
# the layer rather than for bloom so an OpenShift or Talos substrate can be
# added as another check_substrate_* function rather than another file.
#
# Deliberately knows nothing about cluster-forge: no ArgoCD, no Gateway, no
# Keycloak, no HTTPS. It needs no domain and makes no outbound HTTP request,
# so it runs on a cluster where forge has never been deployed.
#
# Needs an admin kubeconfig. There is no alternative way in at this layer:
# the only other kubeconfig the platform issues comes from Resource Manager,
# which is AIRM, which cluster-forge deploys. See smoketest-platform.sh.
#
# DELIBERATELY does not use `set -e`. Every check runs to completion; nothing
# stops the run except a missing tool or no cluster access at all.
#
# Requires : bash 4+, kubectl, jq
# Exit     : 0 clean · 1 warnings · 2 something in scope could not run
#
# EAI-5860
#

VERSION="0.6.0"
SMOKE_LAYER="cluster"

# Sourced BEFORE this script's own defaults and before argument parsing. The
# library assigns the shared defaults unconditionally (INSECURE=0, USE_COLOR=1,
# VERBOSE=0), so sourcing it afterwards silently undid every flag that sets
# one — --verbose did nothing at all.
# shellcheck source=lib-smoketest.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib-smoketest.sh"


SKIP_MTU=0
MTU_SPREAD=0
INSECURE=0
USE_COLOR=1
VERBOSE=0
KUBE_TIMEOUT=15

usage() {
    cat <<'USAGE'
smoketest-cluster.sh — substrate (cluster-bloom / RKE2) health checks

Usage:
  ./smoketest-cluster.sh [options]

Options:
  --skip-mtu           Skip the Cilium overlay MTU check (the only check that
                       execs into a pod)
  --mtu-spread <n>     Allowed difference between the highest and lowest Cilium
                       overlay MTU before warning (default 0 — the overlay MTU
                       has to be uniform across nodes)
  --no-color           Disable coloured output
  -v, --verbose        Show extra detail
  -h, --help           Show this help

Cluster access: the current kubectl context, which must be an admin
kubeconfig. There is no Resource Manager at this layer to fetch an
application kubeconfig from.

Checks: nodes · control plane · CoreDNS · Cilium (incl. overlay MTU) ·
storage provisioner (Longhorn on large, local-path on small/medium) and the
StorageClasses cluster-bloom creates.

Not checked here, because cluster-forge owns them: ArgoCD, Envoy Gateway and
the Gateway resource, MetalLB (bloom writes the IPAddressPool CRs but forge
supplies the CRDs and controller, so on a bloom-only cluster they do not
exist), SeaweedFS, PVCs, Keycloak, OpenBao, TLS, AIRM/AIWB. Run
smoketest-platform.sh for those.
USAGE
}

while [ $# -gt 0 ]; do
    case "$1" in
        --skip-mtu)     SKIP_MTU=1; shift ;;
        --mtu-spread)   MTU_SPREAD="$2"; shift 2 ;;
        --mtu-spread=*) MTU_SPREAD="${1#*=}"; shift ;;
        --no-color)     USE_COLOR=0; shift ;;
        -v|--verbose)   VERBOSE=1; shift ;;
        -h|--help)      usage; exit 0 ;;
        *)              echo "Unknown option: $1" >&2; usage; exit 2 ;;
    esac
done

case "$MTU_SPREAD" in
    ''|*[!0-9]*) echo "Invalid --mtu-spread: '${MTU_SPREAD}' (expected a whole number)" >&2; exit 2 ;;
esac


smoke_init_output
smoke_install_traps
smoke_preflight
smoke_cluster_access

# ---------------------------------------------------------------------------
# Discovery — substrate identity
#
# No domain is resolved here. Nothing at this layer speaks HTTPS, and the
# domain only becomes meaningful once forge's Gateway exists.
# ---------------------------------------------------------------------------
section "Discovery"

KUBELET_VER=$(kc_json get nodes 2>/dev/null | jq -r '.items[0].status.nodeInfo.kubeletVersion // empty' 2>/dev/null)
case "$KUBELET_VER" in
    *rke2*) PLATFORM="rke2" ;;
    *)      if kc get clusterversion >/dev/null 2>&1; then PLATFORM="openshift"; else PLATFORM="generic"; fi ;;
esac
info "platform: ${PLATFORM}${KUBELET_VER:+ (kubelet ${KUBELET_VER})}"
[ "$PLATFORM" = "openshift" ] && info "OpenShift detected — RKE2-specific checks will SKIP"

# cluster-bloom records deployment metadata in ConfigMap bloom/default
# (pkg/ansible/runtime/playbooks/tasks/deploy_k8s_apps/bloom_config.yaml).
# cluster_size drives which storage layer is expected.
BLOOM_CM=$(kc_json get configmap bloom -n default 2>/dev/null)
if [ $? -eq 0 ]; then
    CLUSTER_SIZE=$(printf '%s' "$BLOOM_CM" | jq -r '.data.cluster_size // empty' | tr '[:upper:]' '[:lower:]')
    BLOOM_VERSION=$(printf '%s' "$BLOOM_CM" | jq -r '.data.version // empty')
    RKE2_VERSION=$(printf '%s' "$BLOOM_CM" | jq -r '.data.rke2_version // empty')
    GPU_NODE=$(printf '%s' "$BLOOM_CM" | jq -r '.data.gpu_node // empty')
    BLOOM_DOMAIN=$(printf '%s' "$BLOOM_CM" | jq -r '.data.DOMAIN // empty')
    if [ -n "$CLUSTER_SIZE" ]; then
        ok "cluster size: ${CLUSTER_SIZE} (from ConfigMap bloom/default)"
        info "bloom ${BLOOM_VERSION:-?} · RKE2 ${RKE2_VERSION:-?} · gpu_node=${GPU_NODE:-?}"
    else
        info "ConfigMap bloom/default has no cluster_size — falling back to detection"
    fi
    case "$BLOOM_VERSION" in
        main|master|HEAD|latest)
            info "bloom is pinned to a moving ref '${BLOOM_VERSION}' — not a fixed release" ;;
    esac
else
    info "ConfigMap bloom/default not readable — cluster size unknown, falling back to detection"
fi

# cluster-bloom also writes ConfigMap cluster-domain/default
# (deploy_k8s_apps/domain.yaml). use-cert-manager tells the platform layer
# whether to expect a real certificate or bloom's self-signed one.
DOMAIN_CM=$(kc_json get configmap cluster-domain -n default 2>/dev/null)
if [ $? -eq 0 ]; then
    CD_DOMAIN=$(printf '%s' "$DOMAIN_CM" | jq -r '.data.DOMAIN // empty')
    CD_CERTMGR=$(printf '%s' "$DOMAIN_CM" | jq -r '.data["use-cert-manager"] // empty')
    if [ -n "$CD_DOMAIN" ]; then
        ok "domain: ${CD_DOMAIN} (ConfigMap cluster-domain/default, use-cert-manager=${CD_CERTMGR:-?})"
        DOMAIN="$CD_DOMAIN"
        if [ -n "$BLOOM_DOMAIN" ] && [ "$BLOOM_DOMAIN" != "$CD_DOMAIN" ]; then
            warn "ConfigMap bloom/default says DOMAIN=${BLOOM_DOMAIN} but cluster-domain/default says ${CD_DOMAIN}"
        fi
    else
        warn "ConfigMap cluster-domain/default has no DOMAIN key"
    fi
else
    info "ConfigMap cluster-domain/default not readable — bloom writes it only when DOMAIN was set"
fi

# ---------------------------------------------------------------------------
# Interface parsing for the Cilium MTU check
#
# `ip -o link show` is read once per node and parsed here rather than grepped
# once per interface: the cilium-agent pod is hostNetwork, so a single exec
# yields the overlay device, the underlay NICs and any tailscale0 in one round
# trip. Interface names carry a trailing ':' and veth peers an '@ifN' suffix,
# so both are stripped before matching.
# ---------------------------------------------------------------------------
link_mtu() {
    printf '%s\n' "$1" | awk -v want="$2" '
        { n=$2; sub(/:$/,"",n); sub(/@.*/,"",n)
          if (n == want)
              for (i=1;i<=NF;i++) if ($i == "mtu") { print $(i+1); exit } }'
}

# MTU of every interface that looks like a physical underlay NIC — the largest
# is taken as the underlay the VXLAN header has to fit inside. Deliberately
# excludes rdma*, which GPU nodes commonly carry at 1500 and which is not the
# tunnel's path.
link_underlay_max_mtu() {
    printf '%s\n' "$1" | awk '
        { n=$2; sub(/:$/,"",n); sub(/@.*/,"",n)
          if (n ~ /^(eth|en[a-z]|em|bond|ib)[0-9]/)
              for (i=1;i<=NF;i++) if ($i == "mtu") { if ($(i+1)+0 > max) max=$(i+1)+0; break } }
        END { print max+0 }'
}

# ===========================================================================
# Nodes
# ===========================================================================
section_n "Nodes"

if [ "$CLUSTER_OK" != "1" ]; then
    blocked "node checks — no cluster access"
else
    NODES_JSON=$(kc_json get nodes)
    if [ $? -ne 0 ]; then
        kc_report_failure "node list"
    else
        TOTAL_NODES=$(printf '%s' "$NODES_JSON" | jq '[.items[]] | length')
        NOT_READY=$(printf '%s' "$NODES_JSON" | jq -r '
            .items[]
            | . as $n
            | ($n.status.conditions[] | select(.type=="Ready")) as $r
            | select($r.status != "True")
            | "\($n.metadata.name) Ready=\($r.status) \($r.reason // "")"')
        if [ -z "$NOT_READY" ]; then
            ok "all ${TOTAL_NODES} nodes Ready"
        else
            warn "$(printf '%s\n' "$NOT_READY" | grep -c .)/${TOTAL_NODES} nodes not Ready"
            printf '%s\n' "$NOT_READY" | while read -r l; do [ -n "$l" ] && info "  $l"; done
        fi

        PRESSURE=$(printf '%s' "$NODES_JSON" | jq -r '
            .items[]
            | . as $n
            | $n.status.conditions[]
            | select(.type=="DiskPressure" or .type=="MemoryPressure" or .type=="PIDPressure")
            | select(.status=="True")
            | "\($n.metadata.name) \(.type)=True"')
        if [ -z "$PRESSURE" ]; then
            ok "no node pressure conditions (Disk/Memory/PID)"
        else
            warn "node pressure conditions present"
            printf '%s\n' "$PRESSURE" | while read -r l; do [ -n "$l" ] && info "  $l"; done
        fi

        CORDONED=$(printf '%s' "$NODES_JSON" | jq -r '
            .items[] | select(.spec.unschedulable == true) | .metadata.name')
        if [ -z "$CORDONED" ]; then
            ok "no nodes cordoned"
        else
            warn "nodes cordoned (unschedulable): $(printf '%s' "$CORDONED" | tr '\n' ' ')"
        fi
    fi
fi

# ===========================================================================
# Kubernetes control plane
# ===========================================================================
section_n "Kubernetes control plane"

if [ "$CLUSTER_OK" != "1" ]; then
    blocked "control plane checks — no cluster access"
else
    # /readyz?verbose reports etcd, scheduler and controller-manager directly,
    # rather than inferring health from pod status.
    READYZ=$(kc get --raw='/readyz?verbose' 2>&1)
    if [ $? -eq 0 ]; then
        BAD_READYZ=$(printf '%s' "$READYZ" | grep -E '^\[-\]' | sed 's/^\[-\]//')
        if [ -z "$BAD_READYZ" ]; then
            ok "/readyz — all subsystems passing ($(printf '%s' "$READYZ" | grep -cE '^\[\+\]') checks)"
        else
            warn "/readyz reports failing subsystems"
            printf '%s\n' "$BAD_READYZ" | while read -r l; do [ -n "$l" ] && info "  $l"; done
        fi
    else
        if printf '%s' "$READYZ" | grep -qiE 'forbidden|Unauthorized'; then
            blocked "/readyz — permission denied (needs the system:public-info-viewer binding)"
        else
            warn "/readyz unreachable: $(printf '%s' "$READYZ" | head -1)"
        fi
    fi

    if [ "$PLATFORM" = "openshift" ]; then
        skip "control-plane static pods — OpenShift does not use RKE2 static pod naming"
    else
        CP_JSON=$(pods_for_ns "$NS_KUBESYSTEM")
        if [ $? -ne 0 ]; then
            kc_report_failure "control-plane pods"
        else
            for comp in kube-apiserver kube-controller-manager kube-scheduler; do
                MATCH=$(printf '%s' "$CP_JSON" | jq -r --arg c "$comp" '
                    .items[] | select(.metadata.name | startswith($c)) | .metadata.name')
                if [ -z "$MATCH" ]; then
                    warn "${comp} — no pod found in ns/${NS_KUBESYSTEM}"
                else
                    BADC=$(printf '%s' "$CP_JSON" | jq -r --arg c "$comp" '
                        .items[] | select(.metadata.name | startswith($c))
                        | select(.status.phase != "Running"
                              or ([.status.containerStatuses[]? | select(.ready != true)] | length > 0))
                        | .metadata.name')
                    if [ -z "$BADC" ]; then
                        ok "${comp} — $(printf '%s\n' "$MATCH" | grep -c .) pod(s) Running"
                    else
                        warn "${comp} — unhealthy: $(printf '%s' "$BADC" | tr '\n' ' ')"
                    fi
                fi
            done

            # kube-proxy is legitimately absent when Cilium replaces it.
            KP=$(printf '%s' "$CP_JSON" | jq -r '.items[] | select(.metadata.name | startswith("kube-proxy")) | .metadata.name')
            if [ -z "$KP" ]; then
                skip "kube-proxy — no pods found; expected if Cilium kube-proxy replacement is enabled"
            else
                BADKP=$(printf '%s' "$CP_JSON" | jq -r '
                    .items[] | select(.metadata.name | startswith("kube-proxy"))
                    | select(.status.phase != "Running"
                          or ([.status.containerStatuses[]? | select(.ready != true)] | length > 0))
                    | .metadata.name')
                if [ -z "$BADKP" ]; then
                    ok "kube-proxy — $(printf '%s\n' "$KP" | grep -c .) pod(s) Running"
                else
                    warn "kube-proxy — unhealthy: $(printf '%s' "$BADKP" | tr '\n' ' ')"
                fi
            fi
        fi
    fi
fi

# ===========================================================================
# Network
# ===========================================================================
section_n "Network"

if [ "$CLUSTER_OK" != "1" ]; then
    blocked "network checks — no cluster access"
else
    # CoreDNS — RKE2 names it rke2-coredns-*
    DNS_JSON=$(pods_for_ns "$NS_KUBESYSTEM")
    if [ $? -ne 0 ]; then
        kc_report_failure "CoreDNS pods"
    else
        DNS_PODS=$(printf '%s' "$DNS_JSON" | jq -r '
            .items[] | select(.metadata.name | test("coredns"))
            | select(.metadata.name | startswith("helm-install-") | not)
            | select(.status.phase != "Succeeded")
            | .metadata.name')
        if [ -z "$DNS_PODS" ]; then
            warn "CoreDNS — no pods matching 'coredns' in ns/${NS_KUBESYSTEM}"
        else
            BADDNS=$(printf '%s' "$DNS_JSON" | jq -r '
                .items[] | select(.metadata.name | test("coredns"))
                | select(.metadata.name | startswith("helm-install-") | not)
                | select(.status.phase != "Succeeded")
                | select(.status.phase != "Running"
                      or ([.status.containerStatuses[]? | select(.ready != true)] | length > 0))
                | .metadata.name')
            if [ -z "$BADDNS" ]; then
                ok "CoreDNS — $(printf '%s\n' "$DNS_PODS" | grep -c .) pod(s) healthy"
            else
                warn "CoreDNS — unhealthy: $(printf '%s' "$BADDNS" | tr '\n' ' ')"
            fi
        fi
    fi

    # Cilium DaemonSet — desired == ready == available
    CIL_JSON=$(kc_json_cached get daemonset -n "$NS_KUBESYSTEM")
    if [ $? -ne 0 ]; then
        kc_report_failure "Cilium DaemonSet"
    else
        CIL=$(printf '%s' "$CIL_JSON" | jq -r '
            .items[] | select(.metadata.name | test("cilium"))
            | "\(.metadata.name) \(.status.desiredNumberScheduled // 0) \(.status.numberReady // 0) \(.status.numberAvailable // 0)"')
        if [ -z "$CIL" ]; then
            blocked "Cilium — no DaemonSet matching 'cilium' in ns/${NS_KUBESYSTEM}"
        else
            while read -r name desired ready avail; do
                [ -z "$name" ] && continue
                if [ "$desired" = "$ready" ] && [ "$ready" = "$avail" ] && [ "$desired" != "0" ]; then
                    ok "Cilium ${name} — ${ready}/${desired} ready"
                else
                    warn "Cilium ${name} — ${ready}/${desired} ready, ${avail} available"
                fi
            done <<< "$CIL"
        fi
    fi

    # -----------------------------------------------------------------------
    # Cilium overlay MTU
    #
    # Cilium auto-detects its MTU at agent startup whenever cilium-config has no
    # `mtu` key, and it detects per node. A node carrying a low-MTU interface —
    # a VPN tunnel device at 1280, say — can pick that up while its peers detect
    # the real underlay. The resulting mismatch across the VXLAN overlay breaks
    # the Longhorn nfs-ganesha data path in one direction: pods schedule, get
    # SuccessfulAttachVolume, then fail the mount with DeadlineExceeded — and
    # only the pods that landed on the odd node, which is why it reads as a
    # flaky application bug rather than a network one. Nothing else in this
    # script would catch it: the DaemonSet above is fully ready throughout.
    #
    # This is the only check in the script that execs into a pod, which is why
    # it has its own --skip-mtu.
    # -----------------------------------------------------------------------
    if [ "$SKIP_MTU" = "1" ]; then
        skip "Cilium overlay MTU — --skip-mtu"
    else
        CILCM_MTU=""
        if CILCM_JSON=$(kc_json get configmap cilium-config -n "$NS_KUBESYSTEM"); then
            CILCM_MTU=$(printf '%s' "$CILCM_JSON" | jq -r '.data.mtu // empty')
        fi

        MTU_PODS=""
        if CILPODS_JSON=$(pods_for_ns "$NS_KUBESYSTEM"); then
            MTU_PODS=$(printf '%s' "$CILPODS_JSON" | jq -r '
                .items[]
                | select(.metadata.labels["k8s-app"] == "cilium")
                | select(.status.phase == "Running")
                | "\(.metadata.name) \(.spec.nodeName)"')
        else
            kc_report_failure "Cilium overlay MTU — pod list"
        fi

        if [ -z "$MTU_PODS" ] && [ -n "$CILPODS_JSON" ]; then
            blocked "Cilium overlay MTU — no running pods with label k8s-app=cilium in ns/${NS_KUBESYSTEM}"
        elif [ -n "$MTU_PODS" ]; then
            # node|mtu|device|underlay_max|tailscale0_mtu, one line per node.
            MTU_ROWS=""
            MTU_EXEC_ERR=""
            MTU_FAILED=""
            while read -r mpod mnode; do
                [ -z "$mpod" ] && continue
                MLINKS=$(kc exec "$mpod" -n "$NS_KUBESYSTEM" -c cilium-agent -- ip -o link show 2>&1)
                if [ $? -ne 0 ]; then
                    # One forbidden is enough — the rest will fail identically.
                    if printf '%s' "$MLINKS" | grep -qiE 'forbidden|is not allowed|Unauthorized'; then
                        MTU_EXEC_ERR="forbidden"
                        break
                    fi
                    MTU_EXEC_ERR=$(printf '%s' "$MLINKS" | head -1)
                    MTU_FAILED="${MTU_FAILED}${mnode}: ${MTU_EXEC_ERR}"$'\n'
                    continue
                fi
                # VXLAN is the normal tunnel device; fall back to geneve, then to
                # cilium_host so native-routing clusters still report something.
                MDEV="cilium_vxlan"; MVAL=$(link_mtu "$MLINKS" cilium_vxlan)
                if [ -z "$MVAL" ]; then MDEV="cilium_geneve"; MVAL=$(link_mtu "$MLINKS" cilium_geneve); fi
                if [ -z "$MVAL" ]; then MDEV="cilium_host";   MVAL=$(link_mtu "$MLINKS" cilium_host);   fi
                if [ -z "$MVAL" ]; then
                    MTU_ROWS="${MTU_ROWS}${mnode}|-|none|0|"$'\n'
                    continue
                fi
                MTU_ROWS="${MTU_ROWS}${mnode}|${MVAL}|${MDEV}|$(link_underlay_max_mtu "$MLINKS")|$(link_mtu "$MLINKS" tailscale0)"$'\n'
            done <<< "$MTU_PODS"

            MTU_VALS=$(printf '%s' "$MTU_ROWS" | awk -F'|' 'NF && $2 != "-" { print $2 }')

            if [ "$MTU_EXEC_ERR" = "forbidden" ]; then
                blocked "Cilium overlay MTU — cannot exec into cilium pods (permission denied on ${NS_KUBESYSTEM})"
            elif [ -z "$MTU_VALS" ]; then
                if [ -n "$MTU_EXEC_ERR" ]; then
                    warn "Cilium overlay MTU — could not read interfaces: ${MTU_EXEC_ERR}"
                else
                    skip "Cilium overlay MTU — no cilium_vxlan, cilium_geneve or cilium_host device found (native routing?)"
                fi
            else
                MTU_MIN=$(printf '%s\n' "$MTU_VALS" | sort -n | head -1)
                MTU_MAX=$(printf '%s\n' "$MTU_VALS" | sort -n | tail -1)
                MTU_NODES=$(printf '%s\n' "$MTU_VALS" | grep -c .)
                MTU_DIFF=$((MTU_MAX - MTU_MIN))
                MTU_DEV=$(printf '%s' "$MTU_ROWS" | awk -F'|' 'NF && $3 != "none" { print $3; exit }')

                # Nodes that dropped out of the comparison are named before the
                # verdict, so that "consistent across 5 node(s)" on a six-node
                # cluster cannot be read as a clean bill of health. A node whose
                # agent has no overlay device while its peers do is itself a
                # finding; a whole cluster without one is native routing and has
                # already been skipped above.
                if [ -n "$MTU_FAILED" ]; then
                    warn "Cilium overlay MTU — could not read interfaces on $(printf '%s' "$MTU_FAILED" | grep -c .) node(s), excluded from the comparison below"
                    while read -r l; do [ -n "$l" ] && info "  $l"; done <<< "$MTU_FAILED"
                fi
                MTU_NODEV=$(printf '%s' "$MTU_ROWS" | awk -F'|' 'NF && $2 == "-" { print $1 }')
                if [ -n "$MTU_NODEV" ]; then
                    warn "Cilium overlay device missing on $(printf '%s' "$MTU_NODEV" | grep -c .) of $(printf '%s' "$MTU_ROWS" | grep -c .) node(s): $(printf '%s' "$MTU_NODEV" | tr '\n' ' ')"
                fi

                if [ "$MTU_DIFF" -le "$MTU_SPREAD" ]; then
                    if [ "$MTU_DIFF" = "0" ]; then
                        ok "Cilium ${MTU_DEV} MTU consistent — ${MTU_MAX} on all ${MTU_NODES} node(s)"
                    else
                        ok "Cilium ${MTU_DEV} MTU — ${MTU_MIN}…${MTU_MAX} across ${MTU_NODES} node(s), spread ${MTU_DIFF} within tolerance ${MTU_SPREAD}"
                    fi
                else
                    warn "Cilium ${MTU_DEV} MTU inconsistent — ${MTU_MIN}…${MTU_MAX} across ${MTU_NODES} node(s), spread ${MTU_DIFF} (tolerance ${MTU_SPREAD}); a mismatched overlay MTU breaks Longhorn NFS mounts in one direction"
                    while IFS='|' read -r mn mv md mu mts; do
                        [ -z "$mn" ] && continue
                        [ "$mv" = "$MTU_MAX" ] && continue
                        if [ "$mv" = "-" ]; then
                            info "  ${mn}: no overlay device found"
                        else
                            info "  ${mn}: ${md} mtu ${mv}${mu:+, underlay ${mu}}${mts:+ — tailscale0 is mtu ${mts}, the likely auto-detect source}"
                        fi
                    done <<< "$MTU_ROWS"
                fi

                # A uniform overlay is not sufficient on its own: an overlay MTU
                # equal to the underlay leaves nothing for the 50-byte VXLAN
                # header, so a 9000 underlay wants 8950 here, not 9000. Nodes
                # can agree with each other and still all be wrong this way.
                MTU_NOHEAD=$(printf '%s' "$MTU_ROWS" | awk -F'|' '
                    NF && $2 != "-" && $4+0 > 0 && $2+0 > $4-50 { print $1" ("$2" on a "$4" underlay)" }')
                if [ -n "$MTU_NOHEAD" ]; then
                    warn "Cilium overlay MTU leaves no headroom for the 50-byte VXLAN header on $(printf '%s' "$MTU_NOHEAD" | grep -c .) node(s)"
                    while read -r l; do [ -n "$l" ] && info "  $l"; done <<< "$MTU_NOHEAD"
                fi

                # Reported last, and never as a finding on its own: a cluster
                # whose agents happen to agree is healthy right now. It is the
                # reason one that agrees today can drift tomorrow, so it is
                # worth stating without training people to ignore a standing
                # warning.
                if [ -n "$CILCM_MTU" ]; then
                    ok "cilium-config mtu pinned to ${CILCM_MTU}"
                else
                    info "cilium-config has no mtu key — every agent auto-detects at startup, so the values above can drift apart on a rebuild or a node replacement"
                fi
            fi
        fi
    fi
fi

# ===========================================================================
# Storage
# ===========================================================================
section_n "Storage"

if [ "$CLUSTER_OK" != "1" ]; then
    blocked "storage checks — no cluster access"
else
    # cluster-bloom deploys Longhorn ONLY for CLUSTER_SIZE=large; small and medium
    # get the local-path provisioner instead
    # (pkg/ansible/runtime/playbooks/tasks/deploy_k8s_apps/main.yaml).
    # Checking for Longhorn on a small cluster would be a guaranteed false warning.
    # What SHOULD be here, from the detected cluster size.
    EXPECTED_STORAGE=""
    case "$CLUSTER_SIZE" in
        large)        EXPECTED_STORAGE="longhorn" ;;
        small|medium) EXPECTED_STORAGE="local-path" ;;
    esac

    # What IS here.
    STORAGE_PROVISIONER="unknown"
    if ns_exists "$NS_LONGHORN"; then
        STORAGE_PROVISIONER="longhorn"
    elif kc_json_cached get storageclass 2>/dev/null | jq -e '.items[]? | select(.provisioner | test("local-path"))' >/dev/null 2>&1; then
        STORAGE_PROVISIONER="local-path"
    fi

    # Expected but absent is a real finding; absent and not expected is not.
    if [ -n "$EXPECTED_STORAGE" ] && [ "$STORAGE_PROVISIONER" = "unknown" ]; then
        warn "cluster size '${CLUSTER_SIZE}' expects ${EXPECTED_STORAGE}, but neither Longhorn nor a local-path StorageClass was found"
    elif [ -n "$EXPECTED_STORAGE" ] && [ "$EXPECTED_STORAGE" != "$STORAGE_PROVISIONER" ]; then
        info "cluster size '${CLUSTER_SIZE}' expects ${EXPECTED_STORAGE}, found ${STORAGE_PROVISIONER} — checking what is present"
    fi

    case "$STORAGE_PROVISIONER" in
        longhorn)
            info "storage provisioner: Longhorn${CLUSTER_SIZE:+ (cluster size ${CLUSTER_SIZE})}"
            check_ns_pods "$NS_LONGHORN" "Longhorn"

            # Pods Running with a Degraded volume is a false green — check volumes too.
            LHV_JSON=$(kc_json get volumes.longhorn.io -n "$NS_LONGHORN")
            if [ $? -ne 0 ]; then
                if [ "$(kc_status)" = "forbidden" ]; then
                    blocked "Longhorn volumes — permission denied; this layer needs an admin kubeconfig"
                elif [ "$(kc_status)" = "notfound" ]; then
                    skip "Longhorn volumes — CRD not present"
                else
                    kc_report_failure "Longhorn volumes"
                fi
            else
                VOL_TOTAL=$(printf '%s' "$LHV_JSON" | jq '[.items[]?] | length')
                if [ "${VOL_TOTAL:-0}" = "0" ]; then
                    info "no Longhorn volumes present"
                else
                    BAD_VOLS=$(printf '%s' "$LHV_JSON" | jq -r '
                        .items[]
                        | select((.status.robustness // "unknown") != "healthy"
                              and (.status.state // "") == "attached")
                        | "\(.metadata.name) state=\(.status.state // "?") robustness=\(.status.robustness // "?")"')
                    if [ -z "$BAD_VOLS" ]; then
                        ok "all ${VOL_TOTAL} Longhorn volumes healthy"
                    else
                        warn "$(printf '%s\n' "$BAD_VOLS" | grep -c .)/${VOL_TOTAL} Longhorn volumes degraded or faulted"
                        printf '%s\n' "$BAD_VOLS" | while read -r l; do [ -n "$l" ] && info "  $l"; done
                    fi
                fi
            fi
            ;;
        local-path)
            ok "storage provisioner: local-path${CLUSTER_SIZE:+ (cluster size ${CLUSTER_SIZE})} — Longhorn not expected here"
            # It lives in its own namespace (local-path-storage), so search cluster-wide.
            LP_JSON=$(kc_json_cached get pods -A)
            if [ $? -eq 0 ]; then
                LP_ALL=$(printf '%s' "$LP_JSON" | jq -r '
                    .items[] | select(.metadata.name | test("local-path"))
                    | select(.status.phase != "Succeeded")
                    | "\(.metadata.namespace)/\(.metadata.name)"')
                LP_BAD=$(printf '%s' "$LP_JSON" | jq -r '
                    .items[] | select(.metadata.name | test("local-path"))
                    | select(.status.phase != "Succeeded")
                    | select(.status.phase != "Running"
                          or ([.status.containerStatuses[]? | select(.ready != true)] | length > 0))
                    | "\(.metadata.namespace)/\(.metadata.name)"')
                if [ -z "$LP_ALL" ]; then
                    warn "local-path provisioner — no pods found in any namespace"
                elif [ -z "$LP_BAD" ]; then
                    ok "local-path provisioner — $(printf '%s\n' "$LP_ALL" | grep -c .) pod(s) healthy ($(printf '%s' "$LP_ALL" | head -1 | cut -d/ -f1))"
                else
                    warn "local-path provisioner unhealthy: $(printf '%s' "$LP_BAD" | tr '\n' ' ')"
                fi
            fi
            ;;
        *)
            warn "no recognised storage provisioner found (neither Longhorn namespace nor a local-path StorageClass)"
            ;;
    esac

    # Exactly one default StorageClass. cluster-bloom marks its own "default"
    # class as the cluster default (manifests/local-path/local-path-storageclass.yaml)
    # and Longhorn marks "longhorn". Two defaults is not an error Kubernetes
    # reports anywhere the operator will see — a PVC with no storageClassName
    # just gets whichever one the apiserver picks — and none at all makes every
    # such PVC stay Pending forever.
    SC_JSON=$(kc_json_cached get storageclass 2>/dev/null)
    if [ $? -ne 0 ]; then
        kc_report_failure "StorageClasses"
    else
        SC_TOTAL=$(printf '%s' "$SC_JSON" | jq '[.items[]?] | length')
        SC_DEFAULT=$(printf '%s' "$SC_JSON" | jq -r '
            .items[]? | select(
                (.metadata.annotations["storageclass.kubernetes.io/is-default-class"] // "false") == "true"
            ) | .metadata.name')
        SC_DEFAULT_N=$(printf '%s\n' "$SC_DEFAULT" | grep -c . 2>/dev/null || echo 0)
        case "$SC_DEFAULT_N" in
            0) warn "no default StorageClass among ${SC_TOTAL} — a PVC without storageClassName will never bind" ;;
            1) ok "default StorageClass: ${SC_DEFAULT} (${SC_TOTAL} total)" ;;
            *) warn "${SC_DEFAULT_N} StorageClasses are marked default: $(printf '%s' "$SC_DEFAULT" | tr '\n' ' ')— a PVC without storageClassName gets an arbitrary one" ;;
        esac
        vinfo "storageclasses: $(printf '%s' "$SC_JSON" | jq -r '[.items[]?.metadata.name] | join(" ")')"
    fi
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
smoke_summary "$SMOKE_LAYER"
exit $?
