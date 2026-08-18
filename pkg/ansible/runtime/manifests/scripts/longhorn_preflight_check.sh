#!/bin/bash

set -o pipefail

# Configuration variables
LonghornVersion=v1.10.0
OS=linux
ARCH=amd64
CHECK_FREQUENCY=${CHECK_FREQUENCY:-5}  # Default: check every 5 seconds
TIMEOUT=${TIMEOUT:-600}  # Default: 10 minutes (600 seconds)
KUBECTL=/var/lib/rancher/rke2/bin/kubectl
KUBECONFIG_PATH=/etc/rancher/rke2/rke2.yaml
PREFLIGHT_NAMESPACE=default

# Calculate max attempts based on timeout and frequency
MAX_ATTEMPTS=$((TIMEOUT / CHECK_FREQUENCY))

echo "Longhorn Preflight Check Script"
echo "================================"
echo "Version: ${LonghornVersion}"
echo "Check Frequency: ${CHECK_FREQUENCY} seconds"
echo "Timeout: ${TIMEOUT} seconds (at most ${MAX_ATTEMPTS} attempts)"
echo ""

# Download longhornctl if not already present
if ! command -v longhornctl &> /dev/null; then
    echo "Downloading longhornctl..."
    curl -L https://github.com/longhorn/cli/releases/download/${LonghornVersion}/longhornctl-${OS}-${ARCH} -o longhornctl
    
    if [ $? -ne 0 ]; then
        echo "Error: Failed to download longhornctl"
        exit 1
    fi
    
    chmod +x longhornctl
    if [ $? -ne 0 ]; then
        echo "Error: Failed to make longhornctl executable"
        exit 1
    fi
    
    sudo mv ./longhornctl /usr/local/bin/longhornctl
    if [ $? -ne 0 ]; then
        echo "Error: Failed to move longhornctl to /usr/local/bin"
        exit 1
    fi
    
    echo "✓ longhornctl installed successfully"
else
    echo "✓ longhornctl already installed"
fi

echo ""
echo "Starting preflight checks..."
echo "----------------------------"

# Run preflight check with retry logic
ATTEMPT=1
START_TIME=$(date +%s)

while [ $ATTEMPT -le $MAX_ATTEMPTS ]; do
    ELAPSED=$(($(date +%s) - START_TIME))
    
    echo "[Attempt ${ATTEMPT}/${MAX_ATTEMPTS}] Running preflight check... (elapsed: ${ELAPSED}s)"
    
    longhornctl check preflight --namespace="${PREFLIGHT_NAMESPACE}" --kubeconfig="${KUBECONFIG_PATH}"
    PREFLIGHT_EXIT_CODE=$?

    if [ $PREFLIGHT_EXIT_CODE -eq 0 ]; then
        echo ""
        echo "================================"
        echo "✓ Preflight check PASSED!"
        echo "✓ Longhorn prerequisites met"
        echo "✓ Elapsed time: ${ELAPSED} seconds"
        echo "================================"
        exit 0
    else
        echo "✗ Preflight check failed (exit code: ${PREFLIGHT_EXIT_CODE})"

        # Unconditional, including on the final attempt: leaving the DaemonSet
        # behind means the *next bloom run* re-adopts this same backed-off pod
        # and observes the identical failure. Retrying in place against a wedged
        # pod is never productive — a pod whose sandbox is broken stays broken,
        # because restarts recreate the container and not the sandbox. Delete it
        # so the next attempt gets a fresh pod with a fresh network namespace.
        echo "  Removing the preflight DaemonSet from this attempt..."
        if ! "$KUBECTL" --kubeconfig "$KUBECONFIG_PATH" -n "$PREFLIGHT_NAMESPACE" \
            delete daemonset longhorn-preflight-checker \
            --ignore-not-found --wait=true --timeout=60s; then
            # Silently ignoring this would leave the loop retrying against the
            # wedged pod for the full budget with the cleanup quietly inert.
            echo "  Warning: could not delete the preflight DaemonSet;" \
                 "subsequent attempts may re-adopt the failed pod"
        fi

        if [ $ATTEMPT -lt $MAX_ATTEMPTS ]; then
            echo "  Retrying in ${CHECK_FREQUENCY} seconds..."
            sleep ${CHECK_FREQUENCY}
        fi
    fi

    # MAX_ATTEMPTS alone does not bound wall clock: one iteration is a
    # longhornctl run (30-60s) plus a DaemonSet delete plus the sleep, so the
    # attempt count would overrun TIMEOUT several times over.
    ELAPSED=$(($(date +%s) - START_TIME))
    if [ $ELAPSED -ge $TIMEOUT ]; then
        break
    fi

    ATTEMPT=$((ATTEMPT + 1))
done

# Timeout reached
ELAPSED=$(($(date +%s) - START_TIME))
echo ""
echo "================================"
echo "✗ TIMEOUT: Preflight check failed after ${ELAPSED} seconds"
echo "✗ Gave up after ${ATTEMPT} of at most ${MAX_ATTEMPTS} attempts"
echo "✗ Please check system requirements and logs"
echo "================================"
exit 1
