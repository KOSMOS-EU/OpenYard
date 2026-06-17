#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"

# Load DIST
if [[ -f "$PROJECT_ROOT/DIST" ]]; then
    set -a
    . "$PROJECT_ROOT/DIST"
    set +a
fi

# Build version
if [[ ! -f "$PROJECT_ROOT/BUILD_VERSION" ]]; then
    echo "1" > "$PROJECT_ROOT/BUILD_VERSION"
fi
BUILD_NUM=$(cat "$PROJECT_ROOT/BUILD_VERSION")
NEXT_BUILD=$((BUILD_NUM + 1))
echo "$NEXT_BUILD" > "$PROJECT_ROOT/BUILD_VERSION"

VERSION="0.1.${BUILD_NUM}"

if [[ -z "$BUILD_TOOL" ]]; then
    echo "ERROR: BUILD_TOOL not set. Use: export BUILD_TOOL=docker|podman or create DIST file"
    exit 1
fi

COMPONENT="${1:-openyard-service}"

case "$COMPONENT" in
    openyard-service)
        echo "Building openyard-service v${VERSION}..."
        $BUILD_TOOL build \
            -t "${REGISTRY:-local}/openyard:${VERSION}" \
            -t "${REGISTRY:-local}/openyard:latest" \
            -f "$PROJECT_ROOT/openyard-service/Containerfile" \
            "$PROJECT_ROOT/openyard-service"
        echo "Built: ${REGISTRY:-local}/openyard:${VERSION}"
        ;;
    aktenplan-setup)
        echo "Building aktenplan-setup v${VERSION}..."
        $BUILD_TOOL build \
            -t "${REGISTRY:-local}/aktenplan-setup:${VERSION}" \
            -t "${REGISTRY:-local}/aktenplan-setup:latest" \
            -f "$PROJECT_ROOT/aktenplan-setup/Containerfile" \
            "$PROJECT_ROOT/aktenplan-setup"
        echo "Built: ${REGISTRY:-local}/aktenplan-setup:${VERSION}"
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

if [[ "${2:-}" == "--push" && -n "$REGISTRY" && "$REGISTRY" != "local" ]]; then
    echo "Pushing to $REGISTRY..."
    $BUILD_TOOL push "${REGISTRY}/openyard:${VERSION}"
    $BUILD_TOOL push "${REGISTRY}/openyard:latest"
fi
