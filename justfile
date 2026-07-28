# ClusterBloom build recipes

# Default recipe - show available commands
default:
    @just --list

# Build the bloom binary with optional version parameter (pulls the pinned
# Ansible runtime image at first run)
build version="dev-build":
    @echo "Building bloom (version: {{version}})..."
    @mkdir -p dist
    CGO_ENABLED=0 go build -ldflags="-X 'github.com/silogen/cluster-bloom/cmd.Version={{version}}'" -o dist/bloom
    @echo "Built: dist/bloom"

# Fetch the pinned Ansible runtime image and flatten it into an embeddable,
# gzip-compressed rootfs tarball (used by build-offline). Uses the digest pinned
# in pkg/ansible/runtime/container.go via `docker inspect`.
fetch-image:
    #!/usr/bin/env bash
    set -euo pipefail
    IMAGE="willhallonline/ansible@sha256:9b819715663f18cfd0eb6a6fb1aedbc9d839781ffdd5f4faeff61b8c8a09ae26"
    OUT="pkg/ansible/runtime/embedded/ansible-rootfs.tar.gz"
    echo "Pulling ${IMAGE}..."
    docker pull "${IMAGE}"
    CID=$(docker create "${IMAGE}")
    trap 'docker rm -f "${CID}" >/dev/null 2>&1 || true' EXIT
    mkdir -p pkg/ansible/runtime/embedded
    echo "Exporting flattened rootfs to ${OUT}..."
    docker export "${CID}" | gzip > "${OUT}"
    echo "Wrote ${OUT} ($(du -h "${OUT}" | cut -f1))"

# Build a self-contained (offline / air-gapped) bloom binary with the Ansible
# runtime image embedded — no network pull needed at run time. Fetches the image
# first, then builds with the embed_ansible_image tag.
build-offline version="dev-build": fetch-image
    @echo "Building offline bloom (version: {{version}}, embedded runtime)..."
    @mkdir -p dist
    CGO_ENABLED=0 go build -tags embed_ansible_image -ldflags="-X 'github.com/silogen/cluster-bloom/cmd.Version={{version}}'" -o dist/bloom
    @echo "Built: dist/bloom (embedded Ansible runtime)"