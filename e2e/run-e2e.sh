#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

echo "Building E2E test image..."
docker build -f e2e/Dockerfile.e2e -t k3os-e2e .

echo "Running E2E tests..."
docker run --rm --privileged k3os-e2e
