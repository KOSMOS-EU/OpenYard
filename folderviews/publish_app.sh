#!/bin/bash
set -euo pipefail

APP=folderviews
HOST=cloud.brandis.eu
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
echo "[sync] -> $HOST:$REMOTE_BASE/views/$APP/"
ssh "root@$HOST" "mkdir -p $REMOTE_BASE/views/$APP"
rsync -avz --delete "$DEPLOY_DIR/" "root@$HOST:$REMOTE_BASE/views/$APP/"

echo ""
echo "=== $APP published ==="
echo "Mount in extensions.yml:"
echo "      - ./views/$APP:$VIEWS_PATH/$APP:ro"
