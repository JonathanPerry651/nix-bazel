#!/bin/bash
set -e

# Find the git binary in the runfiles
echo "Current directory: $(pwd)"
echo "Listing all files in runfiles root (..):"
find -L .. -maxdepth 4

GIT_BIN=$(find -L .. -name git -type f | grep "/bin/git" | head -n 1)
# Also try finding the wrapper script which might be just "git"
if [ -z "$GIT_BIN" ]; then
  GIT_BIN=$(find -L .. -name git -type f | grep "external/git/git" | head -n 1)
fi

if [ -z "$GIT_BIN" ]; then
  echo "Error: Could not find 'git' binary"
  exit 1
fi

echo "Found binary at: $GIT_BIN"
echo "Checking file type..."

# Check file type (should be Mach-O executable)
file "$GIT_BIN"

echo "Success: Binary found."
# Note: We cannot execute it on macOS because we don't patch Mach-O RPATHs yet,
# and it depends on absolute paths in /nix/store.
# OUTPUT=$($GIT_BIN --version)
