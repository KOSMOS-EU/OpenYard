#!/bin/bash
# OpenYard Deploy - analog zu opencloud_httpd
# Usage:
#   ./deploy.sh              # Build, push, pull, restart
#   ./deploy.sh push         # Only build and push image
#   ./deploy.sh pull         # Only pull on remote
#   ./deploy.sh restart      # Only restart container
#   ./deploy.sh setup        # Copy openyard.yml to remote
#   ./deploy.sh logs         # Show logs
#   ./deploy.sh status       # Show status
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Load DIST
if [[ -f "$SCRIPT_DIR/DIST" ]]; then
    while IFS='=' read -r key value; do
        [[ "$key" =~ ^#.*$ ]] && continue
        [[ -z "$key" ]] && continue
        value=$(echo "$value" | sed 's/^["'\'']//' | sed 's/["'\'']$//')
        case "$key" in
            HOST) HOST="${value}" ;;
            DOCKER_REGISTRY) DOCKER_REGISTRY="${value}" ;;
            DOCKER_NS) DOCKER_NS="${value}" ;;
            IMAGE) IMAGE="${value}" ;;
            DATAPATH) DATAPATH="${value}" ;;
        esac
    done < "$SCRIPT_DIR/DIST"
fi

REMOTE="root@${HOST:?HOST not set}"
REMOTE_DIR="${DATAPATH:-/data/opencloud_podman}"
CONTAINER="opencloud-openyard"
FULL_IMAGE="${DOCKER_REGISTRY}/${DOCKER_NS}/${IMAGE}"

cmd_push() {
    echo "=== Building and pushing ${FULL_IMAGE} ==="
    cd "$SCRIPT_DIR/openyard-service"
    podman build -t "${FULL_IMAGE}:latest" -f Containerfile .
    podman push "${FULL_IMAGE}:latest"
    echo "Pushed ${FULL_IMAGE}:latest"
}

cmd_pull() {
    echo "=== Pulling image on ${HOST} ==="
    ssh "${REMOTE}" "docker pull ${FULL_IMAGE}:latest"
}

cmd_restart() {
    echo "=== Restarting openyard ==="
    ssh "${REMOTE}" "cd ${REMOTE_DIR} && docker compose up -d openyard"
    sleep 2
    ssh "${REMOTE}" "docker logs ${CONTAINER} --tail 10"
}

cmd_setup() {
    echo "=== Copying openyard.yml to ${HOST}:${REMOTE_DIR} ==="
    scp "$SCRIPT_DIR/openyard-service/openyard.yml" "${REMOTE}:${REMOTE_DIR}/openyard.yml"
    echo "Done. Add to .env: OPENYARD=:openyard.yml"
}

cmd_logs() {
    ssh "${REMOTE}" "docker logs ${CONTAINER} --tail 50 -f"
}

cmd_status() {
    echo "=== OpenYard Status ==="
    ssh "${REMOTE}" "docker ps --filter name=${CONTAINER} --format 'table {{.Names}}\t{{.Status}}\t{{.Image}}'"
    echo ""
    echo "Health:"
    curl -sf "https://${HOST}/api/advancedGeneral/IsListening" 2>/dev/null && echo " OK" || echo " not reachable"
}

cmd_deploy() {
    echo "=== Full Deploy to ${HOST} ==="
    cmd_push
    cmd_pull
    cmd_restart
    echo ""
    echo "=== Deploy complete ==="
    echo "URL: https://${HOST}/api/advancedGeneral/IsListening"
}

case "${1:-deploy}" in
    push)    cmd_push ;;
    pull)    cmd_pull ;;
    restart) cmd_restart ;;
    setup)   cmd_setup ;;
    logs)    cmd_logs ;;
    status)  cmd_status ;;
    deploy)  cmd_deploy ;;
    *)
        echo "Usage: $0 {deploy|push|pull|restart|setup|logs|status}"
        exit 1
        ;;
esac
