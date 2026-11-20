#!/usr/bin/env bash
#
# cluster-smoketest.sh — compatibility shim.
#
# The smoke test was split by layer in v0.6.0: smoketest-cluster.sh checks the
# substrate (cluster-bloom / RKE2), smoketest-platform.sh checks what
# cluster-forge deploys. This runs both, so anything that already invokes
# cluster-smoketest.sh keeps working.
#
# Flags are forwarded to whichever layer understands them; --skip-mtu and
# --mtu-spread go to the cluster layer, everything else to the platform layer.
# For new work prefer `just cluster`, `just platform` or `just all`.
#
# Exit: the worse of the two layers. 0 clean · 1 warnings · 2 blocked.
#
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

cluster_args=()
platform_args=()
while [ $# -gt 0 ]; do
    case "$1" in
        --skip-mtu)     cluster_args+=("$1"); shift ;;
        --mtu-spread)   cluster_args+=("$1" "$2"); shift 2 ;;
        --mtu-spread=*) cluster_args+=("$1"); shift ;;
        # Understood by both.
        -v|--verbose|--no-color)
                        cluster_args+=("$1"); platform_args+=("$1"); shift ;;
        -h|--help)
            printf 'cluster-smoketest.sh is now a shim that runs both layers.\n\n'
            printf '  smoketest-cluster.sh --help   substrate options\n'
            printf '  smoketest-platform.sh --help  platform options\n'
            printf '  just help                     the menu\n\n'
            exit 0 ;;
        *)              platform_args+=("$1"); shift ;;
    esac
done

"${here}/smoketest-cluster.sh" "${cluster_args[@]}"
cluster_rc=$?
"${here}/smoketest-platform.sh" "${platform_args[@]}"
platform_rc=$?

[ "$cluster_rc" -gt "$platform_rc" ] && exit "$cluster_rc"
exit "$platform_rc"
