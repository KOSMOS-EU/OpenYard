# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

This is a multi-module Go repo. Each module must be built/tested from its own directory.

```bash
# Build openyard-service
cd openyard-service && go build ./cmd/openyard/

# Build aktenplan-setup tools
cd aktenplan-setup && go build ./cmd/aktenplan-apply/ && go build ./cmd/aktenplan-extract/

# Run all tests in a module
cd openyard-service && go test ./...
cd aktenplan-setup && go test ./...

# Run a single test
cd openyard-service && go test ./pkg/http/cmis/ -run TestQueryParser

# Container build
podman build -t openyard:latest -f openyard-service/Containerfile openyard-service/
```

No Makefile exists. No linter is configured in the repo.

## Architecture

OpenYard is a Go service that bridges legacy WinYard DMS clients and CMIS 1.1 applications to an OpenCloud storage backend via the CS3 gRPC API.

### Dual API Surface

Both APIs share the same CS3 client, session cache, and upload infrastructure but have independent handler trees:

- **Legacy DMS API** (`/api/...`) — WinYard-compatible JSON endpoints. Session-based auth with GUID SessionIDs. Handlers in `openyard-service/pkg/http/handlers/`.
- **CMIS 1.1 Browser JSON Binding** (`/cmis`) — OASIS CMIS standard. Supports HTTP Basic, Bearer, and SessionID cookie auth. Handlers in `openyard-service/pkg/http/cmis/`.

The router is in `openyard-service/pkg/http/service.go` — it creates both handler trees and mounts them on a single chi mux.

### Key Patterns

- **ObjectID encoding**: `base64url(storageId$spaceId!opaqueId)` — shared between legacy and CMIS APIs. Encoding/decoding in `handlers/mapping.go` and `cmis/mapping.go`.
- **CS3 token injection**: All CS3 gRPC calls require `metadata.AppendToOutgoingContext(ctx, "x-access-token", token)`.
- **Upload protocols**: CS3 gateway returns upload/download protocol enums from `gateway/v1beta1` (not `provider/v1beta1`). Two strategies: `upload.Reva` (gRPC-initiated, default) and `upload.WebDAV`.
- **Configuration**: All env-based via `config.Load()`, no config files. Env vars prefixed `OPENYARD_`.
- **Session cache**: In-memory `go-cache` with configurable TTL. No external session store.

### Module Structure

Two independent Go modules (separate `go.mod`):
- `openyard-service/` — the main HTTP service (module `github.com/kosmos-eu/openyard`)
- `aktenplan-setup/` — CLI tools for file plan provisioning (module `github.com/kosmos-eu/openyard/aktenplan-setup`)

### Other Components

- `folderviews/` — Vue.js frontend plugin (separate build)
- `proxy-setup/` — Docker Compose overlays for OpenCloud proxy configuration

## Deployment

- `build.sh` builds container images
- `deploy.sh` handles full deploy cycle (build, push, pull, restart on remote)
- Deployment config goes in a `DIST` file (not committed)
