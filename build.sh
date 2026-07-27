#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load DIST
if [[ -f "$SCRIPT_DIR/DIST" ]]; then
    set -a
    . "$SCRIPT_DIR/DIST"
    set +a
fi

IMAGE="${IMAGE:-docker.io/flash7777pods/openyard}"
TAG="${TAG:-$(date +%Y%m%d-%H%M)}"

# Push credentials: PUSH_USER:PUSH_TOKEN for Docker Hub, token:PUSH_TOKEN for Codeberg
if [[ -n "${PUSH_USER:-}" ]]; then
    PUSH_CREDS="${PUSH_USER}:${PUSH_TOKEN}"
else
    PUSH_CREDS="token:${PUSH_TOKEN}"
fi

COMPONENT="${1:-openyard-service}"

case "$COMPONENT" in
    openyard-service)
        echo "=== Build openyard: ${IMAGE}:${TAG} ==="
        if command -v buildah &>/dev/null; then
            TMPDIR="${TMPDIR:-/tmp}" buildah bud --network=host --security-opt label=disable \
                -t "${IMAGE}:${TAG}" \
                -f "$SCRIPT_DIR/openyard-service/Containerfile" \
                "$SCRIPT_DIR/openyard-service"
        else
            TMPDIR="${TMPDIR:-/tmp}" podman build --network=host --security-opt label=disable \
                -t "${IMAGE}:${TAG}" \
                -f "$SCRIPT_DIR/openyard-service/Containerfile" \
                "$SCRIPT_DIR/openyard-service"
        fi

        echo ""
        echo "=== Built: ${IMAGE}:${TAG} ==="

        if [[ -n "${PUSH_TOKEN:-}" ]]; then
            echo "Pushing..."
            buildah push --creds="${PUSH_CREDS}" "${IMAGE}:${TAG}"
            buildah tag "${IMAGE}:${TAG}" "${IMAGE}:latest"
            buildah push --creds="${PUSH_CREDS}" "${IMAGE}:latest"
            echo "Pushed: ${IMAGE}:${TAG} + latest"
        fi
        ;;
    aktenplan-setup)
        echo "=== Build aktenplan-setup: ${IMAGE}-aktenplan:${TAG} ==="
        if command -v buildah &>/dev/null; then
            TMPDIR="${TMPDIR:-/tmp}" buildah bud --network=host --security-opt label=disable \
                -t "${IMAGE}-aktenplan:${TAG}" \
                -f "$SCRIPT_DIR/aktenplan-setup/Containerfile" \
                "$SCRIPT_DIR/aktenplan-setup"
        else
            TMPDIR="${TMPDIR:-/tmp}" podman build --network=host --security-opt label=disable \
                -t "${IMAGE}-aktenplan:${TAG}" \
                -f "$SCRIPT_DIR/aktenplan-setup/Containerfile" \
                "$SCRIPT_DIR/aktenplan-setup"
        fi

        echo ""
        echo "=== Built: ${IMAGE}-aktenplan:${TAG} ==="

        if [[ -n "${PUSH_TOKEN:-}" ]]; then
            echo "Pushing..."
            buildah push --creds="${PUSH_CREDS}" "${IMAGE}-aktenplan:${TAG}"
            buildah tag "${IMAGE}-aktenplan:${TAG}" "${IMAGE}-aktenplan:latest"
            buildah push --creds="${PUSH_CREDS}" "${IMAGE}-aktenplan:latest"
            echo "Pushed: ${IMAGE}-aktenplan:${TAG} + latest"
        fi
        ;;
    all)
        "$0" openyard-service
        "$0" aktenplan-setup
        ;;
    *)
        echo "Usage: $0 {openyard-service|aktenplan-setup|all}"
        exit 1
        ;;
esac
