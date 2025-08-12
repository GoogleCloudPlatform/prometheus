#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

echo "Installing dependencies (Go & npm) with docker..."
DOCKER_BUILDKIT=1 docker buildx build . -f ./Dockerfile.google --target vendor -o . -t gmp/sync
