#!/bin/bash
set -e

# Build the Docker image
echo "Building Docker image..."
docker build --platform linux/amd64 -f Dockerfile.debian -t nix-bazel-debian .

# Run the command inside the container
echo "Running in Docker..."
docker run --rm \
    -v "$(pwd):/workspace" \
    -v "bazel_cache_vol:/root/.cache/bazel" \
    nix-bazel-debian \
    bazel "$@"
