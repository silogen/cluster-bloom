#!/bin/bash

# Browser-based integration test runner for PrepareLonghornDisksStep skip mode
# This script:
# 1. Checks prerequisites (remote Chrome, cluster-bloom binary)
# 2. Cleans up from previous test runs
# 3. Starts cluster-bloom in configuration mode
# 4. Runs the browser automation test
# 5. Displays results

set -e

echo "🌸 Cluster-Bloom Browser Test Runner"
echo "===================================="
echo ""

# Check for remote Chrome on port 9222
if ! curl -s http://localhost:9222/json/version > /dev/null 2>&1; then
    echo "❌ Remote Chrome not available on port 9222"
    echo ""
    echo "Start Chrome with:"
    echo "  docker run -d --name chromium \\"
    echo "    --network container:\$(docker ps --filter 'label=com.docker.compose.service=claude-code' -q | head -1) \\"
    echo "    -e PORT=9222 \\"
    echo "    browserless/chrome:latest"
    echo ""
    exit 1
fi

echo "✅ Remote Chrome available on port 9222"

# Check for cluster-bloom binary
if [ ! -f /workspace/cluster-bloom ]; then
    echo "❌ cluster-bloom binary not found at /workspace/cluster-bloom"
    echo ""
    echo "Build it with:"
    echo "  cd /workspace && go build ."
    echo ""
    exit 1
fi

echo "✅ cluster-bloom binary found"

# Get test directory
TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$TEST_DIR"

echo "📁 Test directory: $TEST_DIR"

# Clean up previous test runs
echo ""
echo "🧹 Cleaning up previous test runs..."
killall -9 cluster-bloom 2>/dev/null || true
sleep 1
rm -f bloom.log bloom.yaml bloom-*.log server.log

echo "✅ Cleanup complete"

# Start cluster-bloom in configuration mode
echo ""
echo "🚀 Starting cluster-bloom in configuration mode..."
/workspace/cluster-bloom --reconfigure > server.log 2>&1 &
CLUSTER_BLOOM_PID=$!

# Wait for cluster-bloom to start
sleep 3

# Check if cluster-bloom is running
if ! ps -p $CLUSTER_BLOOM_PID > /dev/null 2>&1; then
    echo "❌ cluster-bloom failed to start"
    echo ""
    echo "Server log:"
    cat server.log
    exit 1
fi

# Find which port cluster-bloom is using
BLOOM_PORT=""
for port in 62078 62079 62080; do
    if curl -s http://127.0.0.1:$port > /dev/null 2>&1; then
        BLOOM_PORT=$port
        break
    fi
done

if [ -z "$BLOOM_PORT" ]; then
    echo "❌ cluster-bloom not responding on ports 62078-62080"
    echo ""
    echo "Server log:"
    tail -20 server.log
    kill $CLUSTER_BLOOM_PID 2>/dev/null || true
    exit 1
fi

echo "✅ cluster-bloom running on port $BLOOM_PORT (PID: $CLUSTER_BLOOM_PID)"

# Run the browser test
echo ""
echo "🌐 Running browser automation test..."
echo "===================================="
echo ""

if go test -v -run TestSkipModeViaBrowser; then
    TEST_RESULT=0
    echo ""
    echo "✅ Test PASSED!"
else
    TEST_RESULT=1
    echo ""
    echo "❌ Test FAILED!"
fi

# Show server log
echo ""
echo "📋 Cluster-Bloom Server Log:"
echo "============================"
cat server.log

# Clean up
echo ""
echo "🧹 Cleaning up..."
kill $CLUSTER_BLOOM_PID 2>/dev/null || true
echo "✅ cluster-bloom stopped"

# Exit with test result
exit $TEST_RESULT
