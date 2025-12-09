#!/bin/bash
set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
REPO_ROOT="$( cd "$SCRIPT_DIR/../.." && pwd )"

echo "Starting Bloom V2 Web UI..."
# Build if binary doesn't exist
if [ ! -f "$REPO_ROOT/dist/bloom-v2" ]; then
    echo "Building bloom-v2..."
    cd "$REPO_ROOT"
    CGO_ENABLED=0 go build -o dist/bloom-v2 ./cmd/bloom
fi

# Start bloom webui in background
"$REPO_ROOT/dist/bloom-v2" webui --port 8080 &
BLOOM_PID=$!
echo "Bloom Web UI started (PID: $BLOOM_PID)"

# Wait for server to be ready
echo "Waiting for server to be ready..."
sleep 3

# Cleanup function
cleanup() {
    echo "Stopping Bloom Web UI (PID: $BLOOM_PID)..."
    kill $BLOOM_PID 2>/dev/null || true
    pkill -f bloom-v2 2>/dev/null || true
}
trap cleanup EXIT

# Get host IP for Docker to access (use host.docker.internal on Mac/Windows, or host IP on Linux)
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    HOST_IP=$(ip route get 1.1.1.1 | awk '{print $7; exit}')
else
    HOST_IP="host.docker.internal"
fi

echo "Running Robot Framework tests via Docker..."
echo "Target URL: http://$HOST_IP:8080"

# Run tests in Docker with network access to host
docker run --rm \
    --network host \
    -v "$REPO_ROOT/tests/robot:/robot/tests" \
    -v "$REPO_ROOT/results:/robot/results" \
    -e BASE_URL="http://localhost:8080" \
    marketsquare/robotframework-browser:latest \
    bash -c "source /home/pwuser/.venv/bin/activate && \
        pip install --quiet robotframework-requests && \
        robot --outputdir /robot/results /robot/tests/*.robot"

echo "Tests completed! Results in $REPO_ROOT/results/"
