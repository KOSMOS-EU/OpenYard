# Task: OpenYard-Microservice als OpenCloud-Plugin

## Kontext

Im Rahmen des KOSMOS-Projekts wird OpenYard als **Microservice innerhalb des
OpenCloud/oCIS-Ökosystems** entwickelt. Ziel: Drop-in-Kompatibilität für
WinYard-Clients durch Bereitstellung der OpenYard-API (Core 1.1, 39
Operationen) gegen ein OpenCloud/Reva-Backend.

Spec-Quelle: <https://github.com/KOSMOS-EU/OpenYard>
Architektur-Vorbild: <https://owncloud.dev/ocis/development/extensions/>
SDK/Pkg-Vorbild: <https://github.com/owncloud/ocis-pkg>

### Strategische Leitlinien

1. **Drop-in für Clients** ist oberste Priorität. Keine Änderung an
   bestehenden WinYard-Clients der Referenzkunden, abgesehen von
   IP-Adresse und ggf. Logins.
2. **OpenCloud absorbiert alles, was es kann.** Identität, Auth, Storage,
   Versionierung, Trash, einfache Suche, Sharing kommen aus OpenCloud/Reva.
3. **Adapter pflegt nur Restmengen.** Was OpenCloud/Reva nicht abdeckt,
   wird minimal-invasiv im Adapter selbst gehalten.
4. **Stub-Maxime**: Endpunkte, die in WinYard nur existieren, um
   interne Architekturkonflikte (Windows-Pseudo-Explorer vs.
   REST-API) zu überbrücken, sind in OpenYard Stubs ohne semantische
   Wirkung.

## Ziel dieses Tasks

Eine produktionsreife Microservice-Implementierung, die:

- sich als regulärer OpenCloud-Service registriert (go-micro Service Registry),
- vom OpenCloud-Proxy auf einem **separaten Listener** (z.B. Port 9201)
  adressiert wird, ausschließlich für die WinYard-API-Pfade,
- die OpenYard-API Core 1.1 implementiert, mit `unprotected: true` am
  Proxy und eigener Auth-Bridge im Service,
- intern das Reva-Gateway über CS3-API anspricht,
- als Podman-Image bereitgestellt wird, kompatibel zum OpenCloud-Compose-Setup.

## Architektur-Grundriss

```
[WinYard-Client]
      |  HTTP/HTTPS (WinYard-Wire-Protokoll, SessionID)
      v
[OpenCloud-Proxy : 9201]   (zweiter Listener, unprotected für /api/*)
      |  internes go-micro mesh
      v
[OpenYard-Service]
      |  CS3-gRPC mit Bearer-Token
      v
[Reva-Gateway]  ->  [Storage-Backend (S3, ZFS, ...)]
```

Der Standard-OpenCloud-Listener auf 9200 bleibt unberührt und bedient
weiterhin Web-UI, WebDAV, OCS etc. mit OpenCloud-eigener Auth.

### Komponenten innerhalb des OpenYard-Service

1. **HTTP-Layer**: Empfängt WinYard-API-Requests, parsing der Query-
   und Body-Parameter, Übersetzung in interne Service-Calls.
2. **Auth-Bridge**: SessionID-Verwaltung. Beim Login wird gegen
   OpenCloud authentifiziert (über deren interne Auth-API bzw.
   WebDAV-Basic-Auth-Probe). Bei Erfolg wird eine opake SessionID
   erzeugt und im In-Memory-Cache mit dem User-Kontext und einer
   Lebensdauer (TTL, default 8h) gespeichert. Bei nachfolgenden
   Requests wird die SessionID validiert und der User-Kontext für
   die CS3-Calls verwendet.
3. **CS3-Client-Wrapper**: Spricht das Reva-Gateway mit dem
   user-spezifischen Bearer-Token an. Standard-Operationen
   (List, Stat, InitiateFileUpload, Move, Copy, Delete, ListRevisions).
4. **Mapping-Layer**: Übersetzt CS3-ResourceInfo in WinYard-
   Antwort-Formate (ObjectId, Path, MimeType, Size, Modified, etc.).
   Pflegt das ObjectId-Schema (Base64-codiertes CS3-ResourceID-Tupel).
5. **Adapter-DB-Layer (Postgres)**: Pflegt die Restmengen:
   - WinYard-spezifische User-Profilfelder (Telefon-Format, Sachbearbeiter-
     Kennung), soweit nicht aus OpenCloud-User-Attributen ableitbar
   - Permalink-Verwaltung (falls WinYard-Permalink-Semantik nicht direkt
     auf OCS-Share-Tokens passt)
   - Optional: Pfad-zu-ObjectId-Cache fuer Performance
6. **Field-Default-Engine**: Konfigurierbare Defaults für WinYard-Felder,
   die nicht aus OpenCloud-Quellen kommen (`""`, `null`, `"N/A"` je nach
   Feld). YAML-konfiguriert, zur Laufzeit ladbar.

### Was NICHT in den Service gehört

- Keine eigene Datenpersistenz für Dokument-Inhalte. **S3 / Storage wird
  ausschließlich von Reva angesprochen.** Der Adapter sieht S3 nie direkt.
- Keine eigene Versionierung. Reva versioniert.
- Keine eigene Lock-Tabelle. OpenYard-API hat kein Locking-Konzept.
- Keine eigene Volltext-Suche oder Index-DB (kommt in Phase 2 als
  separater Dienst).
- Keine eigene Berechtigungslogik. CS3-Grants und OpenCloud-Rollen
  werden genutzt.

## Funktionsumfang Phase 1

Implementiert werden alle Core-1.1-Operationen (siehe separate
Funktionsübersicht-Datei `docs/api-coverage.md`, die im zweiten Lieferschritt
erstellt wird). Implementierungs-Strategie je Funktion ist dort
ausgewiesen (OpenCloud-Framework, Reva, eigen, Stub, Archäologie-Wrapper).

## Anforderungen

### Service-Integration in OpenCloud

- Go-Module-Struktur, kompatibel zu `ocis-pkg` (go-micro Registry,
  Service Discovery, einheitliches Logging und Konfiguration).
- Service-Name: `eu.kosmos.openyard.dms` (oder konfigurierbar).
- Health-, Metrics-, Debug-Endpoints auf separatem Admin-Port nach
  oCIS-Konvention (z.B. 9255 als Debug-Port).
- Konfiguration über YAML-Datei plus Environment-Variablen, oCIS-Stil.

### Proxy-Integration

- Beispiel-Konfiguration für `proxy.yaml` mit zweitem Listener auf 9201:
  ```yaml
  policies:
    - name: openyard
      routes:
        - endpoint: /api/advancedDocuments/
          service: eu.kosmos.openyard.dms
          unprotected: true
        - endpoint: /api/advancedFolders/
          service: eu.kosmos.openyard.dms
          unprotected: true
        - endpoint: /api/advancedCommon/
          service: eu.kosmos.openyard.dms
          unprotected: true
        - endpoint: /api/advancedUsers/
          service: eu.kosmos.openyard.dms
          unprotected: true
        - endpoint: /api/advancedGeneral/
          service: eu.kosmos.openyard.dms
          unprotected: true
        - endpoint: /api/basicCommon/
          service: eu.kosmos.openyard.dms
          unprotected: true
        - endpoint: /api/basicConfig/
          service: eu.kosmos.openyard.dms
          unprotected: true
  ```
- Setup-Doku für den separaten Listener (HTTP/HTTPS-Konfiguration,
  TLS-Zertifikate, Netzwerk-Bindung).

### Auth-Bridge

- Login-Endpoint nimmt User+Passwort entgegen, authentifiziert gegen
  OpenCloud (über deren interne Auth-API oder als Basic-Auth-Probe
  gegen WebDAV-Endpoint), speichert SessionID->User-Kontext im
  In-Memory-Cache mit TTL (Default 8h, konfigurierbar).
- SessionID-Format: 128 bit Zufallswert, Base64-kodiert (vergleichbar
  zu WinYard-Captures, sobald analysiert).
- Validierung pro Request: SessionID -> Cache-Lookup -> User-Kontext.
- Logout-Endpoint entfernt Mapping.
- Bei ungültiger oder abgelaufener Session: 401-Antwort mit
  WinYard-kompatiblem Fehlerformat.
- Cache-Implementierung: in-process Map mit Mutex und periodischem
  Expiration-Sweep, oder Bibliothek wie `patrickmn/go-cache`. Keine
  externe Dependency (kein Redis, kein eigener Store-Prozess).

### CS3-Anbindung

- Verwendung der `cs3org/go-cs3apis`-Bibliothek (oder die
  opencloud-eu-Variante, je nach Ziel-Stack).
- Bearer-Token-Propagierung in jedem gRPC-Call.
- Connection-Pooling und Retry-Logik nach oCIS-Konvention.

### Field-Default-Engine

- YAML-Konfiguration `config/field-defaults.yaml`:
  ```yaml
  user_fields:
    LoginName: source: opencloud.user.username
    Klarname: source: opencloud.user.displayName
    Email: source: opencloud.user.email
    Funktion: default: ""
    Telefon: default: ""
    SachbearbeiterKennung: source: adapter_db, default: ""
    Adresse: default: "N/A"
    Geburtsdatum: default: null
    SuperNutzer: source: opencloud.user.role.admin (boolean)
    Gesperrt: source: opencloud.user.enabled (inverted)
  document_fields:
    ObjectId: source: cs3.resource_id (base64)
    Name: source: cs3.path.basename
    Path: source: cs3.path
    Size: source: cs3.size
    MimeType: source: cs3.mime_type
    Modified: source: cs3.mtime (ISO-8601 WinYard-Format)
    ETag: source: cs3.etag
  ```
- Zentrale Engine, die jeden API-Response durch die Field-Defaults
  filtert und mit den entsprechenden Quellen anreichert.

## Lieferumfang

1. **`cmd/openyard/main.go`** - Service-Entry-Point, oCIS-Stil
2. **`pkg/http/`** - HTTP-Layer (Handler pro API-Endpoint-Gruppe)
3. **`pkg/auth/`** - SessionID-Bridge, In-Memory-Cache
4. **`pkg/cs3/`** - CS3-Client-Wrapper
5. **`pkg/mapping/`** - WinYard <-> CS3 Datenstruktur-Mapping
6. **`pkg/db/`** - Adapter-DB-Zugriff (Postgres, sqlc oder sqlboiler)
7. **`pkg/fielddefaults/`** - Field-Default-Engine
8. **`pkg/operations/`** - Eine Datei pro WinYard-Operation, jeweils
   mit Implementierungs-Kategorie als Code-Kommentar (siehe Maxime).
9. **`config/`** - Beispiel-Konfiguration, Field-Defaults-YAML.
10. **`deployments/`** - Podman-Image, podman-compose-Beispiel zur
    Integration mit OpenCloud, proxy.yaml-Snippet.
11. **`tests/integration/`** - Integration-Tests gegen einen
    laufenden OpenCloud-Container.
12. **`docs/`** - Setup-Anleitung, Konfigurations-Referenz, Diagramme.

## Akzeptanzkriterien

1. **Service-Start**: `openyard --config config.yaml` startet
   erfolgreich, registriert sich am go-micro Mesh, beantwortet
   Health-Check auf Debug-Port.
2. **Proxy-Integration**: Mit Beispiel-`proxy.yaml` ist
   `https://localhost:9201/api/advancedGeneral/IsListening` aufrufbar,
   antwortet mit `200 OK`.
3. **Auth-Flow E2E**: Login-Aufruf mit gültigen OpenCloud-Credentials
   liefert SessionID. Folge-Aufruf mit SessionID liefert authentifizierte
   Antwort. Logout entfernt Session, Folge-Aufruf danach liefert 401.
4. **Document-E2E**: Upload, Download, List, Move, Copy, Delete und
   Versions-Abfrage funktionieren gegen ein konfiguriertes Reva-Backend
   und liefern WinYard-kompatible JSON-Strukturen.
5. **Folder-E2E**: Ordner anlegen, listen, umbenennen, löschen, per
   Pfad finden funktioniert.
6. **Field-Defaults**: Eine Benutzer-Abfrage liefert für nicht
   gepflegte Felder die konfigurierten Defaults, nicht `null` oder
   Fehler.
7. **Replay-tauglich**: Captures aus dem Capture-Pod (siehe
   separater Task `openyard-diff-capture-task.md`) lassen sich
   gegen den Service abspielen, mit dokumentierbarer Diff-Klasse.
8. **Reproduzierbares Build**: `podman build .` produziert ein
   funktionsfähiges Container-Image, das in eine bestehende
   OpenCloud-Podman-Compose-Umgebung integriert werden kann.

## Technologie-Vorgaben

- **Sprache**: Go 1.22 oder höher (oCIS-kompatibel).
- **CS3-API**: `github.com/cs3org/go-cs3apis` oder
  `github.com/opencloud-eu/reva/v2`.
- **ocis-pkg**: für Service-Boilerplate, Logging, Konfiguration.
- **Persistenz**: Postgres (über pgx oder sqlx).
- **Session-Cache**: in-process (Go-Map mit Mutex, oder `patrickmn/go-cache`).
- **HTTP-Framework**: chi oder gorilla/mux, konsistent mit oCIS-Praxis.
- **Container**: Podman-kompatibles Containerfile.

## Hinweise zur Implementierung

- Jede Operation in `pkg/operations/` enthält im Kopf-Kommentar:
  - Spec-Stand (Core 1.1 / Extended)
  - Implementierungs-Kategorie (siehe api-coverage.md)
  - Begründung der Wahl (1-2 Sätze)
  - Hinweis auf bekannte Edge Cases aus Captures (wird im Laufe
    der Capture-Phase ergänzt)
- Operationen, die als Stub klassifiziert sind, liefern eine minimal
  plausible Antwort (z.B. leere Liste, `200 OK` mit minimalem JSON).
  Sie loggen prominent, dass sie aufgerufen wurden, damit im
  Operations-Monitoring sichtbar wird, wenn ein Client sie nutzt.
- ObjectId-Schema: `base64(cs3_resource_id.storage_id + ":" + opaque_id)`.
  Damit ist die ObjectId stateless ableitbar, ohne Mapping-Tabelle.

## Out-of-Scope

- Die Capture/Diff-Pipeline (separater Task).
- Der Aktenplan-Helper (separater Task).
- Ein dedizierter Index-/Suchdienst für komplexe Metadaten-Queries
  (Phase 2).
- Eine moderne Native-API über den Standard-OpenCloud-Listener
  (Phase 3).
