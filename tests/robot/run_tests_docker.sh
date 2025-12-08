#!/bin/bash
# Run Robot Framework tests using Docker container

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Check if bloom binary exists
if [ ! -f "$PROJECT_ROOT/dist/bloom-v2" ]; then
    echo "Error: bloom-v2 binary not found. Run 'make build' first."
    exit 1
fi

# Create results directory
mkdir -p "$SCRIPT_DIR/results"

echo "Running Robot Framework tests in Docker..."
docker run --rm \
    -v "$PROJECT_ROOT:/workspace" \
    -w /workspace/tests/robot \
    --network host \
    --user "$(id -u):$(id -g)" \
    marketsquare/robotframework-browser:latest \
    robot \
    --outputdir results \
    --loglevel DEBUG \
    --variable BASE_URL:http://localhost:8080 \
    "$@" \
    .

echo ""
echo "Tests complete. Results in: $SCRIPT_DIR/results/"
echo "  - report.html: Test execution report"
echo "  - log.html: Detailed test log"
