# OpenYard

**DMS-Adapter for OpenCloud** -- bridges legacy DMS clients and CMIS 1.1 applications to an [OpenCloud](https://opencloud.eu) storage backend via the CS3 API.

```
 Legacy DMS Client          CMIS 1.1 Client
  (Legacy DMS UI)       (LibreOffice, custom apps)
       |                         |
       v                         v
  /api/advanced*            /cmis/{repoId}/root
  /api/basic*               Browser JSON Binding
       |                         |
       +------------+------------+
                    |
             OpenYard Service
              (Go, chi, gRPC)
                    |
                CS3 Gateway
                    |
               OpenCloud / Reva
```

## Components

| Directory | Description |
|---|---|
| `openyard-service/` | Main Go service -- HTTP server with dual API surface |
| `aktenplan-setup/` | CLI tools for file plan (Aktenplan) provisioning |
| `folderviews/` | Vue.js frontend plugin for typed folder views |
| `proxy-setup/` | OpenCloud proxy & Docker Compose overlay configs |

## API Surfaces

### Legacy DMS API (`/api/...`)

Drop-in replacement for the legacy DMS REST API. Existing DMS clients connect without modification.

- **Auth:** Session-based login (OIDC bridge or direct credentials)
- **Documents:** CRUD, file up/download, versioning, preview, metadata
- **Folders:** Hierarchy, spaces/volumes, rights management, templates
- **Search:** Title search, index-based search, metadata queries
- **Config:** App config storage, user settings, enumerations

### CMIS 1.1 Browser JSON Binding (`/cmis`)

Full [OASIS CMIS 1.1](https://docs.oasis-open.org/cmis/CMIS/v1.1/CMIS-v1.1.html) implementation using the Browser (JSON) Binding.

- **Auth:** HTTP Basic, Bearer token, or SessionID cookie
- **Repository Services:** list repositories, repository info, type system
- **Navigation:** getChildren, getDescendants, getFolderTree, parents
- **Object Services:** create/read/update/delete documents and folders, move
- **Content Streams:** download, upload, replace content
- **Versioning:** checkOut, checkIn, cancelCheckOut, version history
- **Discovery:** CMIS-QL queries with `ORDER BY`, `CONTAINS`, `IN_FOLDER`, `IN_TREE`, `JOIN`
- **ACL:** read and manage access control (mapped to CS3 shares)
- **Renditions:** thumbnails and icons based on MIME type
- **Type System:** all 6 CMIS base types + OpenYard secondary types (`oy:documentMetadata`, `oy:folderMetadata`, `oy:indexData`)

## Quick Start

### Prerequisites

- [OpenCloud](https://opencloud.eu) instance with CS3/Reva gateway
- Go 1.23+ (for building from source)
- Docker/Podman (for container deployment)

### Run from source

```bash
cd openyard-service
go build -o openyard ./cmd/openyard/

export OPENYARD_REVA_GATEWAY=localhost:9142
export OPENYARD_SERVICE_USER=admin
export OPENYARD_SERVICE_PASS=admin

./openyard server
```

The service starts on `http://localhost:9201`.

### Container build

```bash
# Build image
./build.sh openyard-service

# Or directly:
podman build -t openyard:latest -f openyard-service/Containerfile openyard-service/
```

### Docker Compose with OpenCloud

```bash
docker compose -f docker-compose.yml -f opencloud.yml \
  -f proxy-setup/docker-compose.openyard.yml up -d
```

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|---|---|---|
| `OPENYARD_HTTP_ADDR` | `0.0.0.0:9201` | HTTP listen address |
| `OPENYARD_HTTPS_ADDR` | *(disabled)* | HTTPS listen address |
| `OPENYARD_TLS_CERT` | | Path to TLS certificate |
| `OPENYARD_TLS_KEY` | | Path to TLS private key |
| `OPENYARD_BASE_URL` | `http://localhost:9201` | External URL (for CMIS service URIs) |
| `OPENYARD_REVA_GATEWAY` | `127.0.0.1:9142` | CS3 gateway gRPC address |
| `OPENYARD_SESSION_TTL` | `8h` | Session expiration time |
| `OPENYARD_SERVICE_USER` | | Service account username |
| `OPENYARD_SERVICE_PASS` | | Service account password |
| `OPENYARD_UPLOAD_METHOD` | `reva` | Upload method: `reva` or `webdav` |
| `OPENYARD_UPLOAD_URL` | `http://opencloud:9200` | Upload endpoint base URL |
| `OPENYARD_DB_DSN` | | Migration database DSN (optional) |
| `OPENYARD_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |

## Usage Examples

### Legacy DMS API

```bash
# Login
curl -X POST http://localhost:9201/api/advancedUsers/Login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","Password":"admin"}'

# List root folders (spaces)
curl -X POST 'http://localhost:9201/api/advancedFolders/GetFolder?SessionID=<sid>&FolderID=00000000-0000-0000-0000-000000000000'

# Health check
curl http://localhost:9201/api/advancedGeneral/IsListening
```

### CMIS 1.1

```bash
# List repositories
curl -u admin:admin http://localhost:9201/cmis

# Browse root folder
curl -u admin:admin 'http://localhost:9201/cmis/<repoId>/root?cmisselector=children'

# Download a file
curl -u admin:admin 'http://localhost:9201/cmis/<repoId>/root/path/to/file.pdf?cmisselector=content'

# Upload a document
curl -u admin:admin \
  -F 'cmisaction=createDocument' \
  -F 'propertyValue[cmis:name]=report.pdf' \
  -F 'content=@report.pdf' \
  http://localhost:9201/cmis/<repoId>/root/Documents

# Query with CMIS-QL
curl -u admin:admin \
  -F 'cmisaction=query' \
  -F "statement=SELECT * FROM cmis:document WHERE cmis:name LIKE '%invoice%' ORDER BY cmis:lastModificationDate DESC" \
  http://localhost:9201/cmis/<repoId>

# Folder tree
curl -u admin:admin 'http://localhost:9201/cmis/<repoId>/root?cmisselector=foldertree&depth=3'
```

## Architecture

```
openyard-service/
  cmd/openyard/           Entry point
  pkg/
    auth/                 Session cache (GUID-based SessionIDs)
    config/               Environment-based configuration
    cs3client/            CS3 gRPC gateway client with retries
    http/
      service.go          Chi router -- mounts both API surfaces
      handlers/           Legacy DMS endpoint handlers
      cmis/               CMIS 1.1 Browser Binding
        service.go          Router, dispatcher, auth middleware
        types.go            CMIS JSON type definitions
        mapping.go          CS3 ResourceInfo <-> CMIS Object
        repository.go       Repository & type services
        navigation.go       Children, descendants, folder tree
        object.go           CRUD, move, property updates
        content.go          Content stream up/download
        versioning.go       CheckOut/CheckIn via CS3 Lock
        query.go            CMIS-QL parser & executor
        acl.go              ACL via CS3 shares
        renditions.go       Thumbnail/icon renditions
        typedefs.go         Secondary type definitions
    migration/            Legacy DMS ID mapping
    upload/               Upload strategies (Reva / WebDAV)
```

## Deployment

```bash
# Full deploy (build, push, pull, restart)
./deploy.sh

# Individual steps
./deploy.sh push      # Build and push image
./deploy.sh pull      # Pull image on remote
./deploy.sh restart   # Restart container
./deploy.sh setup     # Copy config to remote
./deploy.sh logs      # Tail logs
./deploy.sh status    # Show container status
```

Create a `DIST` file for deployment configuration:

```bash
HOST=your-server.example.com
DOCKER_REGISTRY=registry.example.com
DOCKER_NS=openyard
IMAGE=openyard
BUILD_TOOL=podman
```

## License

AGPL-3.0-or-later

See [LICENSE](LICENSE) for the full license text.
