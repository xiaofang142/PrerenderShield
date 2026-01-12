#!/bin/bash
set -euo pipefail

echo "Self-check: Verifying build artifacts..."

errors=0
if [ ! -f "bin/api" ]; then
  echo "[ERROR] bin/api not found. Run ./build.sh first."
  errors=$((errors+1))
fi
if [ ! -d "bin/web" ] || [ ! -f "bin/web/index.html" ]; then
  echo "[ERROR] frontend assets not found in bin/web. Run ./build.sh."
  errors=$((errors+1))
fi
if [ -n "${errors}" ] && [ "$errors" -gt 0 ]; then
  exit 1
fi

echo "Self-check passed: artifacts exist."
exit 0
