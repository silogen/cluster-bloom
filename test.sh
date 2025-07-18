#!/bin/bash

# Build test container
echo "Building test container..."
docker build -f Dockerfile.test -t bloom-test .

# Run validation tests
echo "Running validation tests..."
docker run --rm -v $(pwd):/app -w /app bloom-test

echo "Tests completed!"