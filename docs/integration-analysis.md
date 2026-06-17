# OpenYard Microservice — Integrations-Analyse

Ergebnis der Quellcode-Analyse von `opencloud`, `reva`, `go-micro-plugins`,
`OpenYard` (API-Spec) und `opencloud_stub` (bestehender Code).

---

## 1. Service-Registrierung (go-micro + OpenCloud)

### Registry: NATS-JS-KV

OpenCloud verwendet **NATS-JS-KV** als Default-Registry (nicht Consul, nicht etcd).

```go
// opencloud/pkg/registry/registry.go
type Config struct {
    Type      string   // "nats-js" (default), "memory"
    Addresses []string // Default: 127.0.0.1:9233
}
reg := registry.GetRegistry()
```

### Service-Registrierung

```go
// opencloud/pkg/service/http/service.go
wopts := []micro.Option{
    micro.Server(mServer),
    micro.Address(sopts.Address),
    micro.Name(strings.Join([]string{sopts.Namespace, sopts.Name}, ".")),
    micro.Version(sopts.Version),
    micro.Registry(registry.GetRegistry()),
    micro.RegisterTTL(registry.GetRegisterTTL()),
    micro.RegisterInterval(registry.GetRegisterInterval()),
}
service := micro.NewService(wopts...)
```

**Naming-Konvention**: `{namespace}.{servicename}` → z.B. `eu.kosmos.openyard.dms`

### Für OpenYard

```go
service := micro.NewService(
    micro.Name("eu.kosmos.openyard.dms"),
    micro.Version("1.0.0"),
    micro.Server(httpServer.NewServer()),
    micro.Registry(registry.GetRegistry()),
)
```

**Env-Variablen** (identisch zu OpenCloud):
```bash
MICRO_REGISTRY=nats-js-kv
MICRO_REGISTRY_ADDRESS=127.0.0.1:9233
```

---

## 2. Proxy-Konfiguration (zweiter Listener, Routing)

### Route-Definition

```go
// opencloud/services/proxy/pkg/config/config.go
type Route struct {
    Type              RouteType // "prefix", "query", "regex"
    Endpoint          string    // Pfad-Pattern
    Service           string    // Service-Name (go-micro Discovery)
    Backend           string    // Oder statische URL
    Unprotected       bool      // Auth überspringen
    RemoteUserHeader  string    // Header für User-Identity
    SkipXAccessToken  bool
    AdditionalHeaders map[string]string
}

type Policy struct {
    Name   string
    Routes []Route
}
```

### Router-Middleware

```go
// opencloud/services/proxy/pkg/router/router.go
type RoutingInfo struct {
    rewrite          func(*httputil.ProxyRequest)
    endpoint         string
    unprotected      bool      // ← KEY: wenn true, keine Auth-Prüfung
    remoteUserHeader string
    skipXAccessToken bool
}
```

### Proxy-Config für OpenYard

```yaml
policies:
  - name: openyard
    routes:
      - endpoint: /api/advancedDocuments/(.*)
        type: regex
        service: eu.kosmos.openyard.dms
        unprotected: true          # eigene Auth-Bridge
      - endpoint: /api/advancedFolders/(.*)
        type: regex
        service: eu.kosmos.openyard.dms
        unprotected: true
      - endpoint: /api/advancedCommon/(.*)
        type: regex
        service: eu.kosmos.openyard.dms
        unprotected: true
      - endpoint: /api/advancedUsers/(.*)
        type: regex
        service: eu.kosmos.openyard.dms
        unprotected: true
      - endpoint: /api/advancedGeneral/(.*)
        type: regex
        service: eu.kosmos.openyard.dms
        unprotected: true
      - endpoint: /api/basicCommon/(.*)
        type: regex
        service: eu.kosmos.openyard.dms
        unprotected: true
      - endpoint: /api/basicConfig/(.*)
        type: regex
        service: eu.kosmos.openyard.dms
        unprotected: true
      - endpoint: /api/basicDocuments/(.*)
        type: regex
        service: eu.kosmos.openyard.dms
        unprotected: true
      - endpoint: /api/basicFolders/(.*)
        type: regex
        service: eu.kosmos.openyard.dms
        unprotected: true
```

### Zweiter Listener (Port 9201)

OpenCloud nutzt **Runner Groups** für Multi-Listener:

```go
// opencloud/services/proxy/pkg/command/server.go
gr := runner.NewGroup()

// Standard-Listener (9200)
httpServer, _ := http.Server(...)
gr.Add(runner.NewGoMicroHttpServerRunner("proxy.http", httpServer))

// Zweiter Listener für OpenYard (9201)
openyardServer := &http.Server{
    Addr:    "0.0.0.0:9201",
    Handler: openyardHandler,
}
gr.Add(runner.NewGolangHttpServerRunner("proxy.openyard", openyardServer))

// Debug-Port
debugServer, _ := debug.Server(...)
gr.Add(runner.NewGolangHttpServerRunner("proxy.debug", debugServer))

gr.Run(ctx)
```

---

## 3. Auth-Bridge (CS3-Authentifizierung)

### OpenCloud Auth-Chain

```go
// opencloud/services/proxy/pkg/middleware/authentication.go
type Authenticator interface {
    Authenticate(*http.Request) (*http.Request, bool)
}

// Kette: BasicAuth → AppAuth → OIDC → PublicShare → SignedURL
```

### BasicAuth gegen CS3 Gateway

```go
// opencloud/services/proxy/pkg/middleware/basic_auth.go
func (m BasicAuthenticator) Authenticate(r *http.Request) (*http.Request, bool) {
    login, password, ok := r.BasicAuth()
    user, _, err := m.UserProvider.Authenticate(r.Context(), login, password)
    // ...
}

// opencloud/services/proxy/pkg/user/backend/cs3.go
func (c *cs3backend) Authenticate(ctx context.Context, username, password string) (*cs3.User, string, error) {
    gatewayClient, _ := c.gatewaySelector.Next()
    res, _ := gatewayClient.Authenticate(ctx, &gateway.AuthenticateRequest{
        Type:         "basic",
        ClientId:     username,
        ClientSecret: password,
    })
    return res.User, res.Token, nil  // ← Token für alle CS3-Calls
}
```

### WinYard Auth-Flow (aus Capture-Analyse, 2026-05-27)

**WICHTIG**: Der reale Auth-Flow weicht von der ursprünglichen Annahme ab.
Die Capture-Sonde hat den tatsächlichen Flow aufgezeichnet:

```
WinYard-Client (Delphi/Indy Library)
    |
    |  1. OIDC Authorization Request
    |     GET /connect/authorize → IDP :52329
    |     Response: 302 Redirect (mit Auth-Code)
    |
    |  2. Token Exchange (intern, Client → IDP)
    |     POST /connect/token → IDP :52329
    |     Response: Access-Token / ID-Token
    |
    |  3. DMS API Calls mit Token
    |     POST /api/advancedUsers/Login → DMS :33387
    |     Header: Authorization: Basic <base64>
    |     Body: JSON (NICHT Username/Password, sondern Such-Query!)
    |     Response: SessionID (GUID)
    |
    |  4. Folge-Requests mit SessionID
    |     GET /api/.../Endpoint?SessionID={guid}
    v
```

**Erkenntnisse aus den Captures:**

1. **`Authorization: Basic Og==`** = Base64(`:`), also leerer User:leeres Passwort.
   Der Basic-Auth-Header ist ein Dummy — die eigentliche Auth läuft über OIDC.

2. **`/api/advancedUsers/Login` Body** ist KEIN Username/Password-JSON, sondern
   eine Such-Query mit `SearchValue1=UUID`, `OrgName=checked_out_user`.
   Der Endpoint dient der Session-Initialisierung nach erfolgter OIDC-Auth.

3. **`connect.authorize` (302)** auf dem IDP bestätigt OAuth2/OIDC-Flow.

4. **Identity Server** (`Winyard.Identity.WebApi` auf :52329) ist der Auth-Provider,
   nicht die DMS-API selbst.

### Konsequenz für OpenYard Auth-Bridge

Der OpenYard-Adapter muss **zwei Auth-Modi** unterstützen:

**Modus A: OIDC-kompatibel (Drop-in für WinYard-Clients)**

```
Client → OpenYard IDP-Proxy → connect/authorize → OpenCloud OIDC
Client → OpenYard DMS → /api/advancedUsers/Login (Session-Init)
Client → OpenYard DMS → /api/...?SessionID=...
```

**Modus B: Direkt-Login (für Tests, API-Nutzer)**

```
Client → OpenYard DMS → /api/advancedUsers/Login (Username/Password)
       → CS3 Authenticate → SessionID
Client → OpenYard DMS → /api/...?SessionID=...
```

```go
// OpenYard Login-Handler (beide Modi)
func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
    // Modus A: OIDC — Authorization-Header enthält Bearer-Token vom IDP
    if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
        token := strings.TrimPrefix(authHeader, "Bearer ")
        // Token gegen OpenCloud OIDC validieren
        // SessionID erzeugen, User-Context aus Token extrahieren
        ...
        return
    }

    // Modus B: Basic/Direct — Username/Password im Body
    var req loginRequest
    json.NewDecoder(r.Body).Decode(&req)
    if req.Username != "" && req.Password != "" {
        // CS3 Authenticate
        res, _ := s.gatewayClient.Authenticate(ctx, &gateway.AuthenticateRequest{
            Type:         "basic",
            ClientId:     req.Username,
            ClientSecret: req.Password,
        })
        ...
        return
    }

    // Modus A fallback: Basic Og== (Dummy) mit OIDC-Session
    // Body enthält Such-Query, nicht Credentials
    // Session wurde bereits via IDP etabliert
    ...
}
```

---

## 4. CS3/Reva Gateway-Anbindung

### Client-Verbindung

```go
// reva/pkg/rgrpc/todo/pool/connection.go
conn, _ := grpc.NewClient(
    "127.0.0.1:9142",  // Reva Gateway Default
    grpc.WithTransportCredentials(cred),
    grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10240000)),
)
gatewayClient := gateway.NewGatewayAPIClient(conn)

// Oder via Pool:
// reva/pkg/rgrpc/todo/pool/client.go
client, _ := pool.GetGatewayServiceClient("127.0.0.1:9142")
```

### Token-Propagierung

```go
// Token-Header-Konstante
const TokenHeader = "x-access-token"  // reva/pkg/ctx/tokenctx.go

// Token in Context setzen (für ausgehende gRPC-Calls)
ctx = metadata.AppendToOutgoingContext(ctx, "x-access-token", cs3Token)

// ODER via reva-Helpers:
ctx = ctxpkg.ContextSetToken(ctx, cs3Token)
```

### ResourceID-Format

```
<storage_id>$<space_id>!<opaque_id>
```

```go
// reva/pkg/storagespace/storagespace.go
sid, spid, oid, _ := storagespace.SplitID("storage123$space456!node789")
rid, _ := storagespace.ParseID("storage123$space456!node789")
formatted := storagespace.FormatResourceID(&rid)
```

**OpenYard ObjectId-Schema**: `base64(storage_id + ":" + opaque_id)`

### Referenzen

```go
// By ID
ref := &provider.Reference{
    ResourceId: &provider.ResourceId{
        StorageId: "...",
        SpaceId:   "...",
        OpaqueId:  "...",
    },
}

// By Path
ref := &provider.Reference{Path: "/Aktenplan/11/11.13"}
```

---

## 5. File-Operationen (CS3 → WinYard-Mapping)

### Stat (→ GetDocument, IsDocument, IsFolder)

```go
res, _ := gatewayClient.Stat(ctx, &provider.StatRequest{
    Ref:                   ref,
    ArbitraryMetadataKeys: []string{"winyard.*"},
})
info := res.Info  // *provider.ResourceInfo
// info.Type == provider.ResourceType_RESOURCE_TYPE_FILE
// info.Type == provider.ResourceType_RESOURCE_TYPE_CONTAINER
// info.Size, info.Mtime, info.Etag, info.MimeType, info.Path
```

### ListContainer (→ GetFolder)

```go
res, _ := gatewayClient.ListContainer(ctx, &provider.ListContainerRequest{
    Ref:                   ref,
    ArbitraryMetadataKeys: []string{"winyard.*"},
})
for _, info := range res.Infos {
    // Jedes info ist *provider.ResourceInfo
}
```

### Upload (→ SetDocument, SetFile)

```go
// 1. Initiate
res, _ := gatewayClient.InitiateFileUpload(ctx, &provider.InitiateFileUploadRequest{
    Ref: ref,
    Opaque: &types.Opaque{...},  // Size-Hint etc.
})
// 2. HTTP PUT an res.Protocols[0].UploadEndpoint mit Transfer-Token
```

### Download (→ GetFile)

```go
res, _ := gatewayClient.InitiateFileDownload(ctx, &provider.InitiateFileDownloadRequest{
    Ref: ref,
})
// HTTP GET an res.Protocols[0].DownloadEndpoint mit Transfer-Token
```

### CreateContainer (→ SetFolder)

```go
res, _ := gatewayClient.CreateContainer(ctx, &provider.CreateContainerRequest{
    Ref: &provider.Reference{Path: "/Aktenplan/new-folder"},
})
```

### Move (→ MoveObjects, RenameDocument, RenameFolder)

```go
res, _ := gatewayClient.Move(ctx, &provider.MoveRequest{
    Source:      srcRef,
    Destination: dstRef,
})
// ACHTUNG: Cross-Storage-Moves nicht unterstützt!
```

### Delete (→ BinDocuments, DeleteFolders)

```go
res, _ := gatewayClient.Delete(ctx, &provider.DeleteRequest{Ref: ref})
```

### Trash (→ GetDeletedFiles, RecoverObjects)

```go
// List
res, _ := gatewayClient.ListRecycle(ctx, &provider.ListRecycleRequest{Ref: ref})
// Restore
res, _ := gatewayClient.RestoreRecycleItem(ctx, &provider.RestoreRecycleItemRequest{
    Ref: ref,
    Key: item.Key,
})
```

### Versionen (→ GetDocumentVersions)

```go
res, _ := gatewayClient.ListFileVersions(ctx, &provider.ListFileVersionsRequest{Ref: ref})
for _, v := range res.Versions {
    // v.Key, v.Size, v.Mtime, v.Etag
}
// Restore:
gatewayClient.RestoreFileVersion(ctx, &provider.RestoreFileVersionRequest{
    Ref: ref,
    Key: version.Key,
})
```

---

## 6. Arbitrary Metadata (Aktenzeichen, Custom Fields)

```go
// Setzen
gatewayClient.SetArbitraryMetadata(ctx, &provider.SetArbitraryMetadataRequest{
    Ref: ref,
    ArbitraryMetadata: &provider.ArbitraryMetadata{
        Metadata: map[string]string{
            "winyard.aktenzeichen":   "11.12.02.01-15",
            "winyard.object_type":    "Personalakte",
            "winyard.care_period_id": "42",
        },
    },
})

// Lesen (via Stat mit Keys)
res, _ := gatewayClient.Stat(ctx, &provider.StatRequest{
    Ref:                   ref,
    ArbitraryMetadataKeys: []string{"winyard.*"},
})
aktz := res.Info.ArbitraryMetadata.Metadata["winyard.aktenzeichen"]

// Löschen
gatewayClient.UnsetArbitraryMetadata(ctx, &provider.UnsetArbitraryMetadataRequest{
    Ref:                      ref,
    ArbitraryMetadataKeys: []string{"winyard.aktenzeichen"},
})
```

### Opaque-Encoding-Helpers

```go
// reva/pkg/utils/utils.go
utils.AppendPlainToOpaque(opaque, "key", "value")
value := utils.ReadPlainFromOpaque(opaque, "key")

utils.AppendJSONToOpaque(opaque, "key", structValue)
utils.ReadJSONFromOpaque(opaque, "key", &target)
```

---

## 7. Spaces (Aktenplan-Space)

```go
// Space anlegen
res, _ := gatewayClient.CreateStorageSpace(ctx, &provider.CreateStorageSpaceRequest{
    Type:  "project",
    Name:  "Aktenplan",
    Owner: user,
    Quota: &provider.Quota{QuotaMaxBytes: 500 * 1024 * 1024 * 1024},
})

// Spaces auflisten
res, _ := gatewayClient.ListStorageSpaces(ctx, &provider.ListStorageSpacesRequest{
    Filters: []*provider.ListStorageSpacesRequest_Filter{
        {Type: provider.ListStorageSpacesRequest_Filter_TYPE_SPACE_TYPE,
         Term: &provider.ListStorageSpacesRequest_Filter_SpaceType{SpaceType: "project"}},
    },
})
```

---

## 8. Grants & Berechtigungen (→ FolderRights)

```go
// Grant hinzufügen
gatewayClient.AddGrant(ctx, &provider.AddGrantRequest{
    Ref: ref,
    Grant: &provider.Grant{
        Grantee: &provider.Grantee{
            Type: provider.GranteeType_GRANTEE_TYPE_GROUP,
            Id:   &provider.Grantee_GroupId{GroupId: &grouppb.GroupId{OpaqueId: "sachbearbeiter"}},
        },
        Permissions: &provider.ResourcePermissions{
            GetPath:              true,
            ListContainer:       true,
            Stat:                true,
            InitiateFileDownload: true,
            // ... weitere Permissions
        },
    },
})

// Grants auflisten
res, _ := gatewayClient.ListGrants(ctx, &provider.ListGrantsRequest{
    Ref: ref,
})

// Space-Root-Grants (speziell):
opaque := &types.Opaque{
    Map: map[string]*types.OpaqueEntry{
        "spacegrant": {},  // Marker für Space-Level Grants
    },
}
```

---

## 9. User-Daten (Graph API / CS3)

### Aus Request-Context (nach Proxy-Auth)

```go
u, ok := revactx.ContextGetUser(r.Context())
// u.Id.OpaqueId, u.Username, u.Mail, u.Groups, u.DisplayName
```

### Via CS3 Gateway

```go
res, _ := gatewayClient.GetUserByClaim(ctx, &userpb.GetUserByClaimRequest{
    Claim: "username",
    Value: "sachbearbeiter1",
})
user := res.User
```

### WinYard Field-Mapping

| WinYard-Feld | Quelle | Zugriff |
|---|---|---|
| LoginName | `user.Username` | CS3 User |
| Klarname | `user.DisplayName` | CS3 User |
| Email | `user.Mail` | CS3 User |
| SuperNutzer | `user.Opaque` oder Rollen-Check | Graph API |
| Gesperrt | `!user.Enabled` | Graph API |
| Funktion | `""` (Default) | Field-Default-Engine |
| Telefon | `""` (Default) | Field-Default-Engine |
| SachbearbeiterKennung | Adapter-DB | Postgres |

---

## 10. Thumbnails (→ GetPreviewFile)

```go
// Via gRPC
thumbnailClient := thumbnailssvc.NewThumbnailService(
    "eu.opencloud.api.thumbnails",
    grpcClient,
)
resp, _ := thumbnailClient.GetThumbnail(ctx, &thumbnailssvc.GetThumbnailRequest{
    Filepath:       "/path/to/document.pdf",
    ThumbnailType:  thumbnailssvc.ThumbnailType_PNG,
    Width:          256,
    Height:         256,
    Source: &thumbnailssvc.GetThumbnailRequest_Cs3Source{
        Cs3Source: &thumbnailssvc.CS3Source{...},
    },
})
// resp.DataEndpoint → HTTP GET mit resp.TransferToken
```

---

## 11. Service-Boilerplate (Minimal-Template)

### Verzeichnisstruktur

```
cmd/openyard/
    main.go                 # Cobra root command
pkg/
    command/
        server.go           # Server-Subcommand
    config/
        config.go           # Service-Konfiguration
    http/
        service.go          # chi.Mux HTTP-Handler
    auth/
        bridge.go           # SessionID-Cache
    cs3/
        client.go           # Gateway-Wrapper
    mapping/
        resource.go         # CS3 ↔ WinYard
```

### Entry Point

```go
// cmd/openyard/main.go
func main() {
    cfg := config.DefaultConfig()
    rootCmd := &cobra.Command{Use: "openyard"}
    rootCmd.AddCommand(command.Server(cfg))
    rootCmd.Execute()
}
```

### Server Command

```go
// pkg/command/server.go
func Server(cfg *config.Config) *cobra.Command {
    return &cobra.Command{
        Use:  "server",
        RunE: func(cmd *cobra.Command, args []string) error {
            logger := log.Configure(cfg.Service.Name, ...)

            // CS3 Gateway Client
            gwConn, _ := grpc.NewClient(cfg.Reva.GatewayAddr, ...)
            gwClient := gateway.NewGatewayAPIClient(gwConn)

            // HTTP Service (chi-basiert)
            svc := httpservice.NewService(
                httpservice.Logger(logger),
                httpservice.GatewayClient(gwClient),
                httpservice.Config(cfg),
            )

            // go-micro Registration
            gr := runner.NewGroup()
            microSvc := micro.NewService(
                micro.Name(cfg.Service.Name),
                micro.Server(httpServer.NewServer(
                    server.Address(cfg.HTTP.Addr),
                )),
                micro.Registry(registry.GetRegistry()),
            )
            handler := microSvc.Server().NewHandler(svc)
            microSvc.Server().Handle(handler)

            gr.Add(runner.NewGoMicroHttpServerRunner(
                cfg.Service.Name+".http", microSvc.Server(),
            ))

            return gr.Run(cmd.Context())
        },
    }
}
```

### HTTP-Handler (chi)

```go
// pkg/http/service.go
func NewService(opts ...Option) *Service {
    o := applyOptions(opts...)
    m := chi.NewMux()

    m.Route("/api", func(r chi.Router) {
        // Auth-Endpoints (kein SessionID-Check)
        r.Post("/advancedUsers/Login", o.handleLogin)
        r.Get("/advancedUsers/Logout", o.handleLogout)

        // Geschützte Endpoints (SessionID-Middleware)
        r.Group(func(r chi.Router) {
            r.Use(o.sessionMiddleware)

            r.Get("/advancedGeneral/IsListening", o.handleIsListening)
            r.Post("/advancedDocuments/GetDocument", o.handleGetDocument)
            r.Post("/advancedFolders/GetFolder", o.handleGetFolder)
            // ... alle weiteren Endpoints
        })
    })

    return &Service{mux: m}
}
```

---

## 12. Schlüssel-Dateien (Referenz)

### OpenCloud

| Bereich | Datei |
|---|---|
| Registry | `opencloud/pkg/registry/registry.go` |
| HTTP Service | `opencloud/pkg/service/http/service.go` |
| gRPC Client | `opencloud/pkg/service/grpc/client.go` |
| Proxy Router | `opencloud/services/proxy/pkg/router/router.go` |
| Proxy Config | `opencloud/services/proxy/pkg/config/config.go` |
| Auth Middleware | `opencloud/services/proxy/pkg/middleware/authentication.go` |
| Basic Auth | `opencloud/services/proxy/pkg/middleware/basic_auth.go` |
| CS3 User Backend | `opencloud/services/proxy/pkg/user/backend/cs3.go` |
| Graph Users | `opencloud/services/graph/pkg/service/v0/users.go` |
| Runner Group | `opencloud/pkg/runner/runner.go` |
| OCS Service | `opencloud/services/ocs/pkg/service/v0/service.go` |
| Thumbnails gRPC | `opencloud/services/thumbnails/pkg/service/grpc/v0/service.go` |

### Reva

| Bereich | Datei |
|---|---|
| Gateway Service | `reva/internal/grpc/services/gateway/gateway.go` |
| Storage Ops | `reva/internal/grpc/services/gateway/storageprovider.go` |
| Auth Provider | `reva/internal/grpc/services/gateway/authprovider.go` |
| User Shares | `reva/internal/grpc/services/gateway/usershareprovider.go` |
| Pool/Clients | `reva/pkg/rgrpc/todo/pool/client.go` |
| Connection | `reva/pkg/rgrpc/todo/pool/connection.go` |
| Token Interceptor | `reva/internal/grpc/interceptors/token/token.go` |
| StorageSpace Utils | `reva/pkg/storagespace/storagespace.go` |
| Opaque Utils | `reva/pkg/utils/utils.go` |

### go-micro-plugins

| Bereich | Datei |
|---|---|
| NATS Registry | `go-micro-plugins/v2/registry/nats/nats.go` |
| HTTP Server | `go-micro-plugins/v2/server/http/http.go` |
| HTTP Client | `go-micro-plugins/v2/client/http/http.go` |
| Prometheus Wrapper | `go-micro-plugins/v2/wrapper/monitoring/prometheus/prometheus.go` |
| CORS Middleware | `go-micro-plugins/v2/micro/cors/cors.go` |

---

## 13. Gotchas & Wichtige Hinweise

1. **Cross-Storage-Moves**: Nicht unterstützt in Reva. Move/Copy nur innerhalb desselben Providers.
2. **Token-Propagierung**: IMMER `metadata.AppendToOutgoingContext(ctx, "x-access-token", token)` für jeden CS3-Call.
3. **ResourceID-Format**: Triple `storage_id$space_id!opaque_id` — nicht verwechseln mit dem OpenYard ObjectId-Schema (`base64(storage_id:opaque_id)`).
4. **Status-Code prüfen**: CS3-Responses haben eigene `Status.Code` (nicht nur gRPC-Fehler). Immer `res.Status.Code == rpc.OK` prüfen.
5. **Space-Grants**: Für Berechtigungen am Space-Root das `spacegrant`-Opaque-Flag setzen.
6. **NATS-JS-KV Registry**: OpenCloud nutzt eine spezielle NATS-Variante, nicht das Standard-NATS-Plugin aus go-micro-plugins.
7. **chi statt gorilla/mux**: OpenCloud-Services nutzen durchgehend `chi` als HTTP-Router.
8. **go-micro v4**: OpenCloud nutzt v4 (nicht v2!), mit `butonic/go-micro` Fork-Replace.
9. **SessionID via Query-Parameter**: WinYard-Clients senden SessionID als `?SessionID=...`, NICHT als Header.

---

## 14. OpenYard API-Spec (aus OpenYard-Repo)

Quelle: `/data/source/gitapps/OpenYard/docs/openyard-cloud-api-1.1-core.json` (OpenAPI 3.0)

### Auth-Flow (exakt)

**SessionID wird als Query-Parameter übergeben**, nicht als Header:
```
POST /api/advancedUsers/Login
→ Response: { "SessionID": "abc123-...", "Token": "eyJ...", "ExpiresIn": 3600, "TokenType": "Bearer" }

GET /api/advancedDocuments/IsDocument?SessionID=abc123-...&ObjectId=doc_12345
```

Zusätzlich wird Bearer-Token unterstützt: `Authorization: Bearer eyJ...`

### Error-Format (universal)

```json
{
  "error": "Error message",
  "code": "ERROR_CODE"
}
```

Codes: `INVALID_REQUEST` (400), `AUTH_REQUIRED` (401), `ACCESS_DENIED` (403),
`NOT_FOUND` (404), `CONFLICT` (409), `FILE_TOO_LARGE` (413), `INTERNAL_ERROR` (500)

### Kern-Datentypen

**Document**:
```json
{
  "id": "doc_12345",
  "name": "contract.pdf",
  "path": "/Documents/Contracts/contract.pdf",
  "type": "file",
  "size": 102400,
  "mimeType": "application/pdf",
  "modified": "2024-01-15T10:30:00Z",
  "created": "2024-01-10T08:00:00Z",
  "etag": "abc123def456",
  "creator": "user@kommune.de",
  "lastModifiedBy": "user@kommune.de",
  "versions": [{"versionId": "v1", "timestamp": "...", "createdBy": "..."}]
}
```

**Folder**:
```json
{
  "id": "folder_123",
  "name": "Contracts",
  "path": "/Documents/Contracts",
  "type": "folder",
  "modified": "2024-01-15T10:30:00Z",
  "etag": "xyz789",
  "childCount": 42,
  "creator": "admin@kommune.de",
  "created": "2023-06-01T00:00:00Z",
  "permissions": {"canRead": true, "canWrite": true, "canDelete": true},
  "contents": [...]
}
```

**User**:
```json
{
  "id": "user_123",
  "username": "user@kommune.de",
  "email": "user@kommune.de",
  "displayName": "Max Mustermann",
  "status": "active",
  "roles": ["user", "editor"],
  "permissions": {"canCreateDocuments": true, "canDeleteDocuments": true, "canShareDocuments": true}
}
```

### Pagination

```
?offset=0&limit=100
→ { "items": [...], "total": 500, "offset": 0, "limit": 100, "hasMore": true }
```

### File-Upload (Multipart)

```
POST /api/advancedDocuments/SetFile
Content-Type: multipart/form-data; boundary=----Boundary
→ Binary file im "file"-Part
```

Auch `application/octet-stream` wird unterstützt.

### Alle 39 Core-Endpoints

| # | Methode | Endpoint |
|---|---|---|
| 1 | POST | `/api/advancedUsers/Login` |
| 2 | GET | `/api/advancedUsers/Logout` |
| 3 | GET | `/api/advancedUsers/Authenticate` |
| 4 | GET | `/api/advancedUsers/GetSessionBySessionID` |
| 5 | POST | `/api/advancedUsers/GetUserInfo` |
| 6 | POST | `/api/advancedDocuments/GetDocument` |
| 7 | POST | `/api/advancedDocuments/SetDocument` |
| 8 | POST | `/api/advancedDocuments/GetFile` |
| 9 | POST | `/api/advancedDocuments/SetFile` |
| 10 | GET | `/api/advancedDocuments/IsDocument` |
| 11 | POST | `/api/advancedDocuments/BinDocuments` |
| 12 | POST | `/api/advancedDocuments/GetDeletedFiles` |
| 13 | GET | `/api/advancedDocuments/GetDocumentVersions` |
| 14 | GET | `/api/advancedDocuments/GetPreviewFile` |
| 15 | GET | `/api/advancedDocuments/GetMetaData` |
| 16 | POST | `/api/advancedFolders/GetFolder` |
| 17 | POST | `/api/advancedFolders/GetFolderByFolderpath` |
| 18 | GET | `/api/advancedFolders/IsFolder` |
| 19 | POST | `/api/advancedFolders/SetFolder` |
| 20 | POST | `/api/advancedFolders/DeleteFolders` |
| 21 | GET | `/api/advancedFolders/GetFolderRights` |
| 22 | POST | `/api/advancedFolders/SetFolderRights` |
| 23 | POST | `/api/advancedCommon/CopyObjects` |
| 24 | POST | `/api/advancedCommon/MoveObjects` |
| 25 | POST | `/api/advancedCommon/RecoverObjects` |
| 26 | GET | `/api/advancedCommon/ExistObjectId` |
| 27 | GET | `/api/advancedCommon/ExistObjectPath` |
| 28 | POST | `/api/advancedCommon/Search` |
| 29 | POST | `/api/basicCommon/SearchForTitle` |
| 30 | GET | `/api/basicCommon/GetMetaData` |
| 31 | GET | `/api/basicCommon/GetIndexData` |
| 32 | GET | `/api/basicCommon/GetExistingPermalinksByObjId` |
| 33 | GET | `/api/basicCommon/DeletePermalink` |
| 34 | GET | `/api/basicCommon/GetObjectIdByPermalink` |
| 35 | GET | `/api/basicCommon/GetPreviewFileByPermalink` |
| 36 | GET | `/api/advancedGeneral/IsListening` |
| 37 | GET | `/api/advancedGeneral/GetServerSettings` |
| 38 | GET | `/api/basicCommon/GetFolderSubElementsByMemberId` |
| 39 | GET | `/api/basicConfig/GetOpenyardDMSUserSettings` |

---

## 15. Bestehender Stub-Code (opencloud_stub)

### Was existiert

`opencloud_stub` ist ein **vollständiges OpenCloud-Deployment** mit 41 Microservices.
Es ist KEIN WinYard-Stub — es hat keine OpenYard-Endpoints. Aber es liefert:

### Wiederverwendbare Patterns

| Pattern | Quelle | Nutzen |
|---|---|---|
| Auth-Middleware | `opencloud_stub/packages/opencloud/pkg/middleware/account.go` | JWT-Extraktion, User-Context |
| OCS Response-Format | `opencloud_stub/.../services/ocs/pkg/service/v0/response/` | Response-Envelope, Rendering |
| Config-Pattern | `opencloud_stub/.../services/ocs/pkg/config/` | YAML + Env-Var Parsing |
| HTTP Server Setup | `opencloud_stub/.../services/ocs/pkg/server/http/server.go` | go-micro + chi Integration |
| Service-Handler | `opencloud_stub/.../services/ocs/pkg/service/v0/service.go` | Route-Registration |
| Test-Infrastruktur | `opencloud_stub/.../internal/testenv/test.go` | Integration-Tests |

### Port-Belegung (bestehend)

| Port | Dienst |
|---|---|
| 9200 | OpenCloud Proxy (HTTPS) |
| 9142 | CS3 Gateway (gRPC) |
| 9233 | NATS Registry |
| **9201** | **frei — für OpenYard vorgesehen** |

### Wichtig

- Go Version: 1.24.6
- Kein WinYard-spezifischer Code vorhanden
- OCS-Service hat nur 1 Endpoint (GetSigningKey)
- Deployment via Docker Compose, Build via `/bin/build.sh`

---

## 16. Dependency-Versionen (go.mod)

### Empfohlene go.mod für OpenYard

```go
module github.com/kosmos-eu/openyard

go 1.24.6

require (
    github.com/cs3org/go-cs3apis         v0.0.0-20250908152307-4ca807afe54e
    github.com/opencloud-eu/reva/v2      v2.42.6-0.20260311175421-d77bc89ffe35
    go-micro.dev/v4                      v4.11.0
    github.com/go-chi/chi/v5             v5.2.5
    github.com/go-chi/render             v1.0.3
    google.golang.org/grpc               v1.79.2
    google.golang.org/protobuf           v1.36.11
    github.com/go-micro/plugins/v4/server/http              v1.2.2
    github.com/go-micro/plugins/v4/logger/zerolog           v1.2.0
    github.com/go-micro/plugins/v4/wrapper/monitoring/prometheus v1.2.0
    github.com/rs/zerolog                v1.34.0
    github.com/nats-io/nats.go           v1.49.0
    github.com/patrickmn/go-cache        v2.1.0+incompatible
    github.com/lib/pq                    v1.10.9
    go.opentelemetry.io/otel             v1.42.0
    go.opentelemetry.io/otel/sdk         v1.42.0
)

// KRITISCH: OpenCloud nutzt diesen Fork
replace go-micro.dev/v4 => github.com/butonic/go-micro/v4 v4.11.1-0.20241115112658-b5d4de5ed9b3

// NATS-JS-KV Store (OpenCloud Fork)
replace github.com/go-micro/plugins/v4/store/nats-js-kv => github.com/opencloud-eu/go-micro-plugins/v4/store/nats-js-kv v0.0.0-20250512152754-23325793059a
```

### Versions-Kompatibilität

| Paket | opencloud | reva | opencloud_stub | OpenYard (empfohlen) |
|---|---|---|---|---|
| Go | 1.25.0 | 1.24.1 | 1.24.6 | **1.24.6** |
| go-cs3apis | 20260310 | 20250908 | 20250908 | **20250908** |
| reva/v2 | 2.42.6 | — | 2.39.2 | **2.42.6** |
| go-micro | v4.11.0 | v4.11.0 | v4.11.0 | **v4.11.0** |
| chi/v5 | v5.2.5 | v5.2.3 | v5.2.3 | **v5.2.5** |
| grpc | v1.79.2 | v1.75.1 | v1.76.0 | **v1.79.2** |
