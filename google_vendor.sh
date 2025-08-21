#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

echo "Installing dependencies (Go & npm) with docker..."
DOCKER_BUILDKIT=1 docker buildx build . -f ./Dockerfile.google --target vendor -o . -t gmp/sync

echo "Early validation for vendored files..."
dirs_to_check=(
  "./vendor"
  "./web/ui/node_modules"
  "./web/ui/module/codemirror-promql/node_modules"
)
for dir in "${dirs_to_check[@]}"; do
  if [ ! -d "$dir" ]; then
    echo "FAIL: $dir directory is missing after vendoring...."
    exit 1
  fi
done
