#!/bin/bash
set -euo pipefail

# openyard container build — Containerfile lives in openyard-service/.
# Expects: PUSH_REGISTRY, PUSH_NS, APP, TAG (from DIST / worker env).
# Build only; push is done by push.sh.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ -f "$SCRIPT_DIR/DIST" ] && . "$SCRIPT_DIR/DIST"

REGISTRY="${PUSH_REGISTRY:-docker.io}"
NS="${PUSH_NS:-flash7777pods}"
APP="${APP:-openyard}"
TAG="${TAG:?TAG required}"
IMAGE="${REGISTRY}/${NS}/${APP}"

BUILD="buildah bud"
command -v buildah &>/dev/null || BUILD="podman build"

echo "=== Build ${IMAGE}:${TAG} ==="
TMPDIR="${TMPDIR:-/tmp}" $BUILD --no-cache --network=host --security-opt label=disable \
    -f openyard-service/Containerfile \
    -t "${IMAGE}:${TAG}" \
    openyard-service/
echo "=== Built: ${IMAGE}:${TAG} ==="
