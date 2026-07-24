#!/bin/bash
# Run Ansible playbook tests in Docker container
# Similar to tests/robot/run_tests_docker.sh

set -e

cd "$(dirname "$0")"
PROJECT_ROOT="$(cd ../.. && pwd)"

echo "Building test container..."
docker build -t cluster-bloom-ansible-tests -f Dockerfile "$PROJECT_ROOT"

echo "Running Ansible playbook tests..."
docker run --rm \
  -v "$PROJECT_ROOT:/workspace" \
  cluster-bloom-ansible-tests \
  "$@"

echo "Tests complete!"
