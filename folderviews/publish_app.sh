#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Load DIST
if [[ -f "$PROJECT_ROOT/DIST" ]]; then
    set -a
    . "$PROJECT_ROOT/DIST"
    set +a
fi

APP=folderviews
HOST="${HOST:?HOST not set — create DIST file}"
DEPLOY_DIR=deploy/folderviews
REMOTE_BASE=/data/opencloud_podman
VIEWS_PATH=/var/lib/opencloud/web/assets/views

echo "=== Publish: $APP ==="

# Build
echo "[build] vite build --mode opencloud"
npx vite build --mode opencloud

# Verify
if [ ! -f "$DEPLOY_DIR/remoteEntry.mjs" ]; then
    echo "ERROR: $DEPLOY_DIR/remoteEntry.mjs not found"
    exit 1
fi

# Deploy
# nuhost6 deploy
if [ -n "$NUHOST_TARGET" ]; then
    echo "[nuhost] nu packages pull $NUHOST_TARGET $APP"
    ssh "root@$HOST" "nu packages pull $NUHOST_TARGET $APP && nu restart $NUHOST_TARGET"
    echo ""
    echo "=== $APP published (nuhost6) ==="
    exit 0
fi

echo "[sync] -> $HOST:$REMOTE_BASE/views/$APP/"
ssh "root@$HOST" "mkdir -p $REMOTE_BASE/views/$APP"
rsync -avz --delete "$DEPLOY_DIR/" "root@$HOST:$REMOTE_BASE/views/$APP/"

echo ""
echo "=== $APP published ==="
echo "Mount in extensions.yml:"
echo "      - ./views/$APP:$VIEWS_PATH/$APP:ro"
