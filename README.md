# OpenYard

**DMS-Adapter für OpenCore (OpenCloud + OpenCosmos).**

OpenYard verbindet Legacy-DMS-Clients und CMIS-1.1-Anwendungen mit
dem OpenCore-Storage-Backend über die CS3-API.

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
          OpenCloud / Reva (OpenCore)
```

## Komponenten

| Verzeichnis | Zweck |
|---|---|
| `openyard-service/` | Haupt-Go-Service — HTTP-Server mit doppeltem API |
| `aktenplan-setup/` | CLI-Tools für Aktenplan-Provisionierung |
| `folderviews/` | Vue.js-Frontend-Plugin für typisierte Ordneransichten |
| `proxy-setup/` | OpenCloud-Proxy- und Docker-Compose-Overlays |

## API-Flächen

### Legacy DMS API (`/api/...`)

Drop-in-Ersatz für die Legacy-DMS-REST-API. Bestehende DMS-Clients
verbinden sich ohne Änderungen.

- **Auth:** Session-basiert (OIDC-Brücke oder direkte Zugangsdaten)
- **Dokumente:** CRUD, Upload/Download, Versionierung, Preview, Metadaten
- **Ordner:** Hierarchie, Spaces/Volumes, Rechteverwaltung, Templates
- **Suche:** Titelsuche, Indexsuche, Metadaten-Abfragen
- **Config:** App-Config, User-Settings, Enumerationen

### CMIS 1.1 Browser JSON Binding (`/cmis`)

Komplette [OASIS CMIS 1.1](https://docs.oasis-open.org/cmis/CMIS/v1.1/CMIS-v1.1.html)
Implementierung mit Browser (JSON) Binding.

- **Auth:** HTTP Basic, Bearer Token oder SessionID-Cookie
- **Repository Services:** Repository-Liste, Info, Typsystem
- **Navigation:** getChildren, getDescendants, getFolderTree, parents
- **Object Services:** CRUD für Dokumente und Ordner, Move
- **Content Streams:** Download, Upload, Content-Ersatz
- **Versionierung:** checkOut, checkIn, cancelCheckOut, History
- **Discovery:** CMIS-QL Queries (ORDER BY, CONTAINS, IN_FOLDER, IN_TREE, JOIN)
- **ACL:** Zugriffskontrolle (gemappt auf CS3-Shares)
- **Renditions:** Thumbnails und Icons basierend auf MIME-Type
- **Typsystem:** 6 CMIS-Basistypen + OpenYard-Sekundärtypen
  (`oy:documentMetadata`, `oy:folderMetadata`, `oy:indexData`)

## Quick Start

### Voraussetzungen

- OpenCore-Instanz mit CS3/Reva-Gateway
- Go 1.23+ (Build aus Source)
- Docker/Podman (Container-Deployment)

### Aus Source

```bash
cd openyard-service
go build -o openyard ./cmd/openyard/

export OPENYARD_REVA_GATEWAY=localhost:9142
export OPENYARD_SERVICE_USER=admin
export OPENYARD_SERVICE_PASS=admin

./openyard server
```

### Container

```bash
./build.sh openyard-service
# oder:
podman build -t openyard:latest -f openyard-service/Containerfile openyard-service/
```

## Konfiguration

| Variable | Default | Zweck |
|---|---|---|
| `OPENYARD_HTTP_ADDR` | `0.0.0.0:9201` | HTTP-Adresse |
| `OPENYARD_REVA_GATEWAY` | `127.0.0.1:9142` | CS3-Gateway gRPC |
| `OPENYARD_SERVICE_USER` | | Service-Account Username |
| `OPENYARD_SERVICE_PASS` | | Service-Account Passwort |
| `OPENYARD_UPLOAD_METHOD` | `reva` | Upload-Methode: `reva` oder `webdav` |
| `OPENYARD_BASE_URL` | `http://localhost:9201` | Externe URL (CMIS URIs) |
| `OPENYARD_SESSION_TTL` | `8h` | Session-Abgelauf |
| `OPENYARD_LOG_LEVEL` | `info` | Log-Level |

## License

[AGPL-3.0-or-later](LICENSE)
