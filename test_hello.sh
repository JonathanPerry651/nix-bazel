#!/bin/bash
set -e

# Find the git binary in the runfiles
echo "Current directory: $(pwd)"
# Find the git binary in the runfiles
echo "Current directory: $(pwd)"

if [ -z "$1" ]; then
  echo "Error: Path to git binary must be provided as the first argument"
  exit 1
fi

GIT_BIN="$1"

if [ -z "$GIT_BIN" ]; then
  echo "Error: Could not find 'git' binary"
  exit 1
fi

echo "Found binary at: $GIT_BIN"
echo "Checking file type..."

# Check file type (should be Mach-O executable)
file "$GIT_BIN"

echo "Success: Binary found."
echo "Executing binary..."
OUTPUT=$($GIT_BIN --version)
echo "Output: $OUTPUT"

if [[ "$OUTPUT" == *"git version"* ]]; then
  echo "Success: Git executed successfully"
else
  echo "Error: Git execution failed or unexpected output"
  exit 1
fi
