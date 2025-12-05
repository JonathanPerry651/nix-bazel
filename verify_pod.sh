#!/bin/bash
set -e

echo "Checking bwrap version..."
bwrap --version

echo "Building git..."
bazel build @nix_deps//:git

echo "Building python3..."
bazel build @nix_deps//:python3

echo "Verification passed!"
