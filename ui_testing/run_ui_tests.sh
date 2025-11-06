#!/bin/bash
# Script to run UI tests with a temporary bloom server instance
# This script starts bloom, runs the tests, and cleans up

set -e  # Exit on error

# Get script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Create temporary directory for test run
TEMP_DIR=$(mktemp -d -t bloom-ui-test.XXXXXX)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BLOOM_PORT=62078
BLOOM_PID=""
BLOOM_LOG="$TEMP_DIR/bloom.log"
BLOOM_BINARY="$PROJECT_ROOT/dist/bloom"
UI_TEST_DIR="$SCRIPT_DIR"
WORK_DIR="$TEMP_DIR/work"
BLOOM_YAML="$WORK_DIR/bloom.yaml"  # bloom.yaml is created in the working directory

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}🧹 Cleaning up...${NC}"

    if [ -n "$BLOOM_PID" ]; then
        echo -e "${YELLOW}   Stopping bloom process (PID: $BLOOM_PID)${NC}"
        kill $BLOOM_PID 2>/dev/null || true
        wait $BLOOM_PID 2>/dev/null || true
    fi

    # Also kill any bloom processes listening on the port
    if command -v lsof >/dev/null 2>&1; then
        if lsof -ti:$BLOOM_PORT >/dev/null 2>&1; then
            echo -e "${YELLOW}   Killing process on port $BLOOM_PORT${NC}"
            lsof -ti:$BLOOM_PORT | xargs kill -9 2>/dev/null || true
        fi
    fi

    # Clean up temporary directory
    if [ -d "$TEMP_DIR" ]; then
        echo -e "${YELLOW}   Removing temp directory: $TEMP_DIR${NC}"
        rm -rf "$TEMP_DIR"
    fi

    echo -e "${GREEN}✅ Cleanup complete${NC}"
}

# Set trap to cleanup on exit
trap cleanup EXIT INT TERM

echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}🚀 Cluster-Bloom UI Test Runner${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}\n"

echo -e "${BLUE}📁 Temporary directory: $TEMP_DIR${NC}"
echo -e "${BLUE}📁 Project root: $PROJECT_ROOT${NC}\n"

# Create working directory
mkdir -p "$WORK_DIR"

# Check if bloom binary exists
if [ ! -f "$BLOOM_BINARY" ]; then
    echo -e "${RED}❌ Error: bloom binary not found at $BLOOM_BINARY${NC}"
    echo -e "${YELLOW}   Please build it first: cd $PROJECT_ROOT && make build${NC}"
    echo -e "${YELLOW}   Or: cd $PROJECT_ROOT && go build -o dist/bloom${NC}"
    exit 1
fi

# Check if chromium debugger is available
if ! netstat -tln 2>/dev/null | grep -q ":9222" && ! ss -tln 2>/dev/null | grep -q ":9222"; then
    echo -e "${RED}❌ Error: Chromium debugger not found on port 9222${NC}"
    echo -e "${YELLOW}   Please ensure chromium is running with remote debugging enabled${NC}"
    exit 1
fi

# Kill any existing bloom process on the port
if ss -tln 2>/dev/null | grep -q ":$BLOOM_PORT " || netstat -tln 2>/dev/null | grep -q ":$BLOOM_PORT "; then
    echo -e "${YELLOW}⚠️  Port $BLOOM_PORT is already in use${NC}"
    if command -v lsof >/dev/null 2>&1; then
        echo -e "${YELLOW}   Killing existing process...${NC}"
        lsof -ti:$BLOOM_PORT 2>/dev/null | xargs kill -9 2>/dev/null || true
        sleep 2
    else
        echo -e "${RED}❌ Error: Cannot kill process (lsof not available)${NC}"
        echo -e "${YELLOW}   Please manually stop the process on port $BLOOM_PORT${NC}"
        exit 1
    fi
fi

# Start bloom in background (web UI mode)
echo -e "${BLUE}🌸 Starting bloom web server...${NC}"
echo -e "${BLUE}   Binary: $BLOOM_BINARY${NC}"
echo -e "${BLUE}   Working directory: $WORK_DIR${NC}"
cd "$WORK_DIR"
# Run bloom without arguments to start web UI (requires no bloom.log)
"$BLOOM_BINARY" > "$BLOOM_LOG" 2>&1 &
BLOOM_PID=$!

echo -e "${BLUE}   Process ID: $BLOOM_PID${NC}"
echo -e "${BLUE}   Log file: $BLOOM_LOG${NC}"

# Wait for bloom to start
echo -e "${YELLOW}⏳ Waiting for bloom to start on port $BLOOM_PORT...${NC}"
MAX_WAIT=30
ELAPSED=0
while ! (ss -tln 2>/dev/null | grep -q ":$BLOOM_PORT " || netstat -tln 2>/dev/null | grep -q ":$BLOOM_PORT "); do
    if [ $ELAPSED -ge $MAX_WAIT ]; then
        echo -e "${RED}❌ Timeout: bloom did not start within ${MAX_WAIT}s${NC}"
        echo -e "${YELLOW}📄 Last 20 lines of bloom log:${NC}"
        tail -20 "$BLOOM_LOG"
        exit 1
    fi

    # Check if process is still running
    if ! kill -0 $BLOOM_PID 2>/dev/null; then
        echo -e "${RED}❌ Error: bloom process died${NC}"
        echo -e "${YELLOW}📄 Bloom log:${NC}"
        cat "$BLOOM_LOG"
        exit 1
    fi

    sleep 1
    ELAPSED=$((ELAPSED + 1))
    echo -n "."
done

echo -e "\n${GREEN}✅ Bloom server started successfully${NC}\n"

# Run the UI tests
echo -e "${BLUE}🧪 Running UI tests...${NC}"
echo -e "${BLUE}   Test directory: $UI_TEST_DIR${NC}"
echo -e "${BLUE}   Bloom YAML will be created at: $BLOOM_YAML${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}\n"

cd "$UI_TEST_DIR"

# Run tests with environment variable pointing to temp bloom.yaml
# Run tests and capture exit code
set +e
BLOOM_YAML_PATH="$BLOOM_YAML" go test -v -run TestWebFormE2E 2>&1 | grep -v "ERROR: could not unmarshal"
TEST_EXIT_CODE=${PIPESTATUS[0]}
set -e

echo -e "\n${BLUE}═══════════════════════════════════════════════════════${NC}"

# Check test results
if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}✅ All UI tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ UI tests failed (exit code: $TEST_EXIT_CODE)${NC}"
    echo -e "${YELLOW}📄 Last 50 lines of bloom log:${NC}"
    tail -50 "$BLOOM_LOG"
    exit $TEST_EXIT_CODE
fi
