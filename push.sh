#!/bin/bash
set -euo pipefail

# openyard image push — primary registry + optional mirror (PUSH_MIRRORS).
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
[ -f "$SCRIPT_DIR/DIST" ] && . "$SCRIPT_DIR/DIST"

TAG="${TAG:?TAG required}"
REGISTRY="${PUSH_REGISTRY:-docker.io}"
NS="${PUSH_NS:-flash7777pods}"
APP="${APP:-openyard}"
IMAGE="${REGISTRY}/${NS}/${APP}"
PUSH_TOKEN="${PUSH_TOKEN:?PUSH_TOKEN required}"
PUSH_USER="${PUSH_USER:-token}"

CMD="podman"
command -v podman &>/dev/null || CMD="buildah"

# --- primary registry ---
echo "[push] registry=${REGISTRY} user=${PUSH_USER}"
printf '%s' "${PUSH_TOKEN}" | $CMD login -u "${PUSH_USER}" --password-stdin "${REGISTRY}"
$CMD tag "${IMAGE}:${TAG}" "${IMAGE}:latest"
echo "=== Push ${IMAGE}:${TAG} ==="
$CMD push "${IMAGE}:${TAG}"
$CMD push "${IMAGE}:latest"

# --- mirror registry (PUSH_MIRRORS = user:token@registry/ns/app) ---
if [ -n "${PUSH_MIRRORS:-}" ]; then
    # split user:token@rest
    M_USER="${PUSH_MIRRORS%%:*}"
    REST="${PUSH_MIRRORS#*:}"
    M_TOKEN="${REST%%@*}"
    M_TARGET="${REST#*@}"
    M_REGISTRY="${M_TARGET%%/*}"
    echo "[push] mirror registry=${M_REGISTRY} user=${M_USER}"
    printf '%s' "${M_TOKEN}" | $CMD login -u "${M_USER}" --password-stdin "${M_REGISTRY}"
    $CMD tag "${IMAGE}:${TAG}" "${M_TARGET}:${TAG}"
    echo "=== Push ${M_TARGET}:${TAG} ==="
    $CMD push "${M_TARGET}:${TAG}"
fi

echo "=== Pushed ${IMAGE}:${TAG} ==="
