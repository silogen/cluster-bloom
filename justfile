# ClusterBloom build recipes

# Default recipe - show available commands
default:
    @just --list

# Build the bloom binary with optional version parameter
build version="dev-build":
    @echo "Building bloom (version: {{version}})..."
    @mkdir -p dist
    CGO_ENABLED=0 go build -ldflags="-X 'github.com/silogen/cluster-bloom/cmd.Version={{version}}'" -o dist/bloom
    @echo "Built: dist/bloom"