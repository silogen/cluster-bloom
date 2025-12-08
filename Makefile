.PHONY: build clean test test-api test-webui test-docker help

BINARY_NAME=bloom-v2
BUILD_DIR=dist
CMD_DIR=cmd/bloom

help:
	@echo "Bloom V2 Build & Test"
	@echo ""
	@echo "Targets:"
	@echo "  build        Build the bloom-v2 binary"
	@echo "  clean        Remove build artifacts"
	@echo "  test         Run all Robot Framework tests (requires pip)"
	@echo "  test-api     Run API tests only (requires pip)"
	@echo "  test-webui   Run Web UI tests only (requires pip)"
	@echo "  test-docker  Run all tests in Docker (no pip needed)"
	@echo "  help         Show this help message"

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -rf tests/robot/results
	@echo "Clean complete"

test: build
	@echo "Running all Robot Framework tests..."
	@cd tests/robot && ./run_tests.sh

test-api: build
	@echo "Running API tests..."
	@cd tests/robot && ./run_tests.sh api/

test-webui: build
	@echo "Running Web UI tests..."
	@cd tests/robot && ./run_tests.sh webui/

test-docker: build
	@echo "Running all tests in Docker..."
	@cd tests/robot && ./run_tests_docker.sh
