#!/bin/bash
set -e

echo "Testing Python 3..."
$1 -c "import sys; print(f'Python {sys.version}')"

echo "Testing Curl..."
$2 --version | head -n 1

echo "Testing ImageMagick..."
$3 --version | head -n 1
