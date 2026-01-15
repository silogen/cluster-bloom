.PHONY: build clean help

BINARY_NAME=bloom-v2
BUILD_DIR=dist
CMD_DIR=cmd/bloom

help:
	@echo "Bloom V2 Build"
	@echo ""
	@echo "Targets:"
	@echo "  build   Build the bloom-v2 binary"
	@echo "  clean   Remove build artifacts"
	@echo "  help    Show this help message"

build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"
