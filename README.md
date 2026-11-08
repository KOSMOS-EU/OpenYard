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
| `OPENYARD_METADATA_CONFIG` | | Pfad zur Metadaten-Mapping-YAML (siehe unten) |

### Metadaten-Mapping und Ablageregeln

Die YAML aus `OPENYARD_METADATA_CONFIG` legt je Namespace (= Login-Name eines DMS-Clients) fest,
wie Metadaten gespeichert werden und wo importierte Dokumente landen. Wird beim Start gelesen.

```yaml
namespaces:
  saskia:
    prefix: ctrl              # Client-Schluessel -> ctrl.<key>
    storage_case: lower
    client_case: upper
    source: saskia            # "source" an jedem Beleg
    defaults:                 # Vorgaben fuer Suche/DocIndex, nicht gespeichert
      booking.status: U
    formats:                  # Client-Formate, gespeichert wird ISO / Punkt
      date: "02.01.2006"
      decimal: ","
    # Ablageregel je Dokumenttyp (ConfigName von ImportDocumentDynamic).
    # Greift nur, wenn der Client keinen FolderId/SpaceId mitgibt.
    folders:
      IfrAnordnung: Anordnungen/{HH_JAHR}/{KZ_DKS}/{AO_NR|block:1000}
      IfrAnordnungListe: Anordnungen/{HH_JAHR}/Listen
    folder_default: Ablage/{doc.type}/{now|date:2006}/{now|date:01}
    # Eindeutigkeitsschluessel je Dokumenttyp: derselbe Beleg noch einmal (Nachdruck)
    # ersetzt den Inhalt des vorhandenen Dokuments im Zielordner (neue Version),
    # statt eine zweite Datei anzulegen. Gilt fuer ImportDocumentDynamic und SetDocument.
    unique_keys:
      IfrAnordnung: [Mand, HH_JAHR, AO_NR]
users:
  saskia: saskia
```

Pfadvorlagen: `{FELD}` ist ein Schluessel des gesendeten DocIndex (Gross/Klein egal),
`{doc.type}` der Dokumenttyp, `{now}` der Importzeitpunkt. Modifikatoren nach `|`:
`block:N` (Nummernkreis-Block, 7151 → `7001-8000`), `pad:N`, `left:N`, `right:N`, `upper`,
`lower`, `date:LAYOUT` (Go-Layout, Eingabe im Client- oder ISO-Format), `default:X`.
Bleibt ein Platzhalter leer, gilt `folder_default`; fehlt auch das, landet das Dokument in
der Space-Wurzel. Fehlende Ordner werden beim Import angelegt. Ein `/` im Wert wird zu `∕`
(U+2215), ein Wert erzeugt also nie zusaetzliche Ebenen.

#### Aktenplan-Lookup (`lookups`)

Statt eines festen Pfads kann ein Dokumenttyp ueber ein Metadatum des Zielordners abgelegt werden,
z. B. die Produktnummer einer Anordnung, die als Kennung (`oy.fileReference`) an den Aktenplan-Ordnern
haengt. Der Lookup wird vor `folders` geprueft; traegt der Beleg keinen Code oder schlaegt der Lookup
fehl, greifen die Pfadvorlagen wie gehabt (Notablage).

```yaml
namespaces:
  saskia:
    lookups:
      IfrAnordnung:
        keys: [PROD, PROD_G]            # DocIndex-Felder, der erste belegte Wert ist der Code
        names: [PROD_BEZ, PROD_G_BEZ]   # Bezeichnung zum jeweiligen Feld (fuer create)
        match: oy.fileReference         # Metadatum des Ordners, das dem Code entsprechen muss
        spaces: [Innere Verwaltung, Sicherheit und Ordnung]  # durchsuchte Spaces; leer oder "*" = alle sichtbaren Project-Spaces
        create: "{lookup.code} {lookup.name}"   # Ordnername, wenn kein Ordner den Code traegt
        subpath: "{doc.type}/{HH_JAHR}"         # Ebenen unter dem gefundenen Ordner
        numbered_levels: 1              # so viele neue Subpath-Ebenen werden nach dem Aktenzeichen-Plan nummeriert
    # Aktenzeichen-Syntax fuer neue Ebenen (Vorgabe = dokumentierter Aktenplan, folderviews/AKTENPLAN.md):
    # Sachgruppen 11 / 11.12 / 11.12.01, Aktenschrank 11.12.01.03, Akte -01, Vorgang /1, Register #1
    reference_plan: ["{n:2}", ".{n:2}", ".{n:2}", ".{n:2}", "-{n:2}", "/{n}", "#{n}"]
```

Ablauf: Die Ordner der genannten Spaces werden einmal durchlaufen und nach `match` indiziert
(Cache 10 Minuten; ein Fehltreffer loest einen erneuten Durchlauf aus, fruehestens alle 20 Sekunden,
damit auch von Hand angelegte Ordner gefunden werden). Fehlt der Ordner zum Code und ist `create`
gesetzt, entsteht er unter dem naechsten vorhandenen Vorfahren nach Kennung (`11.16.13.01` →
`11.16.13` → `11.16` → `11`, sonst Space-Wurzel) und erhaelt den Code als `match`-Metadatum.
Darunter werden die `subpath`-Ebenen angelegt; die ersten `numbered_levels` neuen Ebenen setzen die
Nummerierung nach `reference_plan` fort: unter dem Aktenschrank `11.16.13.01` wird die Typ-Ebene zur
Akte `11.16.13.01-01`, darunter das Jahr zum Vorgang `11.16.13.01-01/1`, jeweils mit der niedrigsten
freien Nummer unter den Geschwistern; vorhandene Ordner bleiben unveraendert. Der Vorfahre eines
fehlenden Codes wird ebenfalls nach dem Plan bestimmt. Jede Ordneranlage wird als Info-Log mit Name, Kennung und
Eltern-ID protokolliert. Platzhalter wie in `folders`, zusaetzlich `{lookup.code}` und `{lookup.name}`.
Der Lookup laeuft mit den Rechten des Client-Logins, der Space muss fuer ihn sichtbar sein.

Feldnamen sind Client-Sache: `keys` und `names` sind die Schluessel des gesendeten DocIndex, so wie
der jeweilige DMS-Client sie liefert (bei SASKIA die DMS-Feldnamen aus KS_DMS_META, z. B. `PROD` fuer
das Produkt der Buchungsstelle und `PROD_G` fuer das der Gegen-Buchungsstelle), noch vor dem Mapping
auf Speicherschluessel (`ctrl.prod`). Ein anderer Client (anderer Namespace) bringt andere Felder mit
und bekommt deshalb eigene `lookups` mit seinen Feldnamen; OpenYard kennt keine festen Feldbedeutungen.
Aendert der Client seine Feldnamen, wird nur die Regel angepasst.

#### Trenner der Speicherschluessel (`key_separator`)

`key_separator: "."` ersetzt den Unterstrich der Client-Schluessel durch eine weitere Ebene:
`PROD_G_BEZ` wird zu `ctrl.prod.g.bez` statt `ctrl.prod_g_bez`; in Richtung Client wird der Trenner
wieder zum Unterstrich. Nachtraeglich auf einem Namespace mit vorhandenen Belegen gesetzt, muessen deren
Schluessel umgeschrieben werden, sonst finden `unique_keys` und DocIndex den Altbestand nicht mehr.

## License

[AGPL-3.0-or-later](LICENSE)
