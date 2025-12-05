#!/bin/bash
set -e

echo "Packaging workspace..."
tar --exclude='bazel-*' --exclude='.git' --exclude='workspace.tar.gz' --exclude='nix-bazel-gen/nix-bazel-resolve' --exclude='nix-bazel-gen/nix-bazel-fetch' -czf workspace.tar.gz .

echo "Copying workspace to Pod..."
kubectl cp workspace.tar.gz nix-bazel-builder:/workspace.tar.gz

echo "Unpacking workspace in Pod..."
kubectl exec nix-bazel-builder -- bash -c "mkdir -p /workspace && tar -xzf /workspace.tar.gz -C /workspace"

echo "Workspace ready in /workspace"
