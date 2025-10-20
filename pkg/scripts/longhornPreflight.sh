#!/bin/bash

# Configuration variables
LonghornVersion=v1.8.0
OS=linux
ARCH=amd64
CHECK_FREQUENCY=${CHECK_FREQUENCY:-5}  # Default: check every 5 seconds
TIMEOUT=${TIMEOUT:-600}  # Default: 10 minutes (600 seconds)

# Calculate max attempts based on timeout and frequency
MAX_ATTEMPTS=$((TIMEOUT / CHECK_FREQUENCY))

echo "Longhorn Preflight Check Script"
echo "================================"
echo "Version: ${LonghornVersion}"
echo "Check Frequency: ${CHECK_FREQUENCY} seconds"
echo "Timeout: ${TIMEOUT} seconds (${MAX_ATTEMPTS} attempts)"
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
    
    # Run the preflight check (don't exit on failure due to set -e)
    set +e
    longhornctl check preflight --namespace=longhorn --kubeconfig=$HOME/.kube/config
    PREFLIGHT_EXIT_CODE=$?
    set -e
    
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
        
        if [ $ATTEMPT -lt $MAX_ATTEMPTS ]; then
            echo "  Retrying in ${CHECK_FREQUENCY} seconds..."
            sleep ${CHECK_FREQUENCY}
        fi
    fi
    
    ATTEMPT=$((ATTEMPT + 1))
done

# Timeout reached
ELAPSED=$(($(date +%s) - START_TIME))
echo ""
echo "================================"
echo "✗ TIMEOUT: Preflight check failed after ${ELAPSED} seconds"
echo "✗ Maximum attempts (${MAX_ATTEMPTS}) reached"
echo "✗ Please check system requirements and logs"
echo "================================"
exit 1
