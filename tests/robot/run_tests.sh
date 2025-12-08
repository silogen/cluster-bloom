#!/bin/bash
# Robot Framework test runner for Bloom V2 Web UI

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Check if bloom binary exists
if [ ! -f "../../dist/bloom-v2" ]; then
    echo "Error: bloom-v2 binary not found. Run 'make build' first."
    exit 1
fi

# Check Robot Framework is available
if ! python3 -c "import robot" 2>/dev/null; then
    echo "Error: Robot Framework not installed"
    echo ""
    echo "Install dependencies with:"
    echo "  pip3 install robotframework robotframework-requests robotframework-browser"
    echo "  rfbrowser init"
    echo ""
    exit 1
fi

# Create results directory
mkdir -p results

# Run tests
echo "Running Robot Framework tests..."
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
