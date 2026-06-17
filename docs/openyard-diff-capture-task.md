# Task: OpenYard Differential Capture & Replay Pod

## Kontext

Im Rahmen des KOSMOS/OpenYard-Projekts wird ein DMS-Adapter entwickelt, der
nach außen die WinYard-HTTP-API spricht und intern gegen OpenCloud/Reva
(CS3) arbeitet. **Oberste Priorität: Drop-in-Replacebarkeit für bestehende
Clients.** Keine Änderung an Client-Code, identisches Wire-Verhalten in dem
Umfang, der durch Capture-Samples nachgewiesen ist.

Spec-Quelle: <https://github.com/KOSMOS-EU/OpenYard> (Core 1.1: 39 Operationen,
Extended: 76).

### Coverage-Klassifikation (projektweit)

Jede API-Operation wird in eine von drei Stufen eingeordnet:

1. **Durch Samples nachgewiesen** – echter Capture-Trace vorhanden, Adapter
   differenziell validiert. Drop-in-Garantie vertretbar.
2. **Nicht nachgewiesen, hinreichend dokumentiert** – Spec/Hersteller-Doku
   reicht für eine plausible Implementierung. Best-effort.
3. **Nicht hinreichend spezifiziert** – kein Sample, keine Doku.
   Stub-Antwort oder 501. Keine Garantie.

Dieser Task baut die Infrastruktur, die Operationen von Stufe 3/2 auf
Stufe 1 hebt: Capture-Pod beim Referenzkunden + Offline-Replay/Diff gegen
den Adapter.

## Ziel

Eine reproduzierbar deploybare Podman-Pod-Definition auf Debian 13, die:

- als transparenter Reverse-Proxy zwischen DMS-Clients (LAN) und der
  WinYard-Instanz (LAN) steht;
- jeden HTTP-Request/Response in einem HAR-ähnlichen Format persistiert;
- später Captures gegen den OpenYard-Adapter (im Rechenzentrum) abspielen
  und die Antworten Byte-genau bzw. mit konfigurierbaren Toleranzen
  vergleichen kann;
- Diff-Ergebnisse in einer SQLite-DB strukturiert ablegt, abfragbar nach
  Operation, Zeitpunkt, Client, Diff-Klasse.

## Topologie (Soll)

```
[DMS-Clients (LAN)] --HTTP--> [Capture-Pod (LAN-IP)] --HTTP--> [WinYard-Server (LAN)]
                                       |
                                       v
                              [/var/lib/openyard-capture]
                              (HAR + SQLite, LUKS-verschlüsselt)

Später, offline:
[HAR-Archiv] --replay--> [OpenYard-Adapter (RZ)] --> [Diff-DB]
```

- Capture-Host: Debian 13, Mini-PC-Klasse (N100, 16 GB RAM, 500 GB SSD
  reicht für mehrere Monate Capture bei typischem Kommunen-Workload).
- Eine NIC reicht funktional; zwei NICs (Client-LAN / WinYard-LAN) sind
  optional für saubere Netzwerk-Trennung.
- Clients bekommen alternative IP konfiguriert (DNS-Override oder
  hosts-Eintrag), die auf den Capture-Host zeigt.

## Komponenten im Pod

1. **mitmproxy** im Reverse-Mode (`mitmdump --mode reverse:http://winyard:port`).
2. **capture-addon** (Python): persistiert Flows als HAR-Files, ruft
   Diff-Service nur im Replay-Mode auf.
3. **diff-service** (Python): nimmt Replay-Befehle entgegen, spielt HAR
   gegen Ziel-URL ab, vergleicht Responses, schreibt Ergebnisse in SQLite.
4. **SQLite-Volume** für strukturierte Diff-Daten.
5. **HAR-Volume** für Raw-Captures.

Keine Datenbank-Container nötig (SQLite reicht). Kein Web-UI im ersten
Wurf – das kann nachgelagert auf der DB aufsetzen.

## Anforderungen

### Capture-Phase

- Reverse-Proxy hört auf konfigurierbarem Port (Default 80/443).
- TLS-Termination optional – wenn Clients HTTPS sprechen, muss
  mitmproxy-CA in deren Trust Store. Configuration-Hint im README.
- Jeder Flow wird als HAR-Entry in eine täglich rotierte Datei
  geschrieben: `/var/lib/openyard-capture/har/YYYY-MM-DD.har`.
- Bodies werden **vollständig** persistiert (PDFs, docx, Bilder – im
  Verwaltungskontext meist <10 MB, in Summe handhabbar). Round-Trip-
  Validierung verlangt echte Bytes.
- HAR-Entries enthalten zusätzlich Custom-Felder:
  - `_openyard.operation`: aus Pfad abgeleiteter Operationsname
    (z.B. `advancedDocuments.GetDocument`).
  - `_openyard.client_fingerprint`: aus User-Agent + Source-IP
    (anonymisiert via HMAC mit lokalem Secret).
  - `_openyard.session_id`: extrahierte SessionID, vor Persistierung
    durch stabiles Token ersetzt (gleiche SessionID → gleiches Token).
- Logrotation: tägliche HAR-Files, gzip nach 24h, Löschung nach
  konfigurierbarer Frist (Default 90 Tage, DSGVO-relevant).
- Health-Endpoint `/_capture/health` (auf separatem Admin-Port, nicht
  über mitmproxy geroutet).

### Replay/Diff-Phase

- CLI-Tool `openyard-replay`:
  - Input: HAR-File oder HAR-Verzeichnis.
  - Target: URL des OpenYard-Adapters.
  - Output: Diff-Records in SQLite.
- Pro Request:
  1. Original-Request rekonstruieren (Methode, Pfad, Headers, Body).
  2. SessionID-Token durch frische SessionID ersetzen (Adapter-Login
     vorab, Token im Replay-Kontext halten).
  3. Request an Adapter senden.
  4. Response mit aufgezeichneter Response vergleichen.
- Vergleichsregeln (konfigurierbar pro Operation):
  - **Status-Code**: muss identisch sein.
  - **Header**: Subset-Match gegen Whitelist (Content-Type, ETag,
    Content-Length); andere ignoriert.
  - **Body**:
    - JSON: strukturell vergleichen, mit Toleranz-Regeln für
      Timestamp-Felder, ID-Felder, Token-Felder (regex-basiert,
      konfigurierbar pro Operation).
    - Binary (Files): SHA-256 vergleichen.
    - XML/HTML: optional, vorerst als Bytes vergleichen.
- Diff-Klassifikation:
  - `match` – identisch oder innerhalb Toleranz.
  - `tolerated` – Abweichung, aber durch konfigurierte Regel akzeptiert.
  - `mismatch` – Abweichung, nicht toleriert.
  - `error` – Adapter unerreichbar / Replay-Fehler.

### SQLite-Schema (Vorschlag)

```sql
CREATE TABLE captures (
    id INTEGER PRIMARY KEY,
    har_file TEXT NOT NULL,
    har_entry_index INTEGER NOT NULL,
    timestamp_captured TEXT NOT NULL,  -- ISO-8601
    operation TEXT,                     -- z.B. advancedDocuments.GetDocument
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    client_fingerprint TEXT,
    session_token TEXT,
    request_size INTEGER,
    response_size INTEGER,
    response_status INTEGER
);

CREATE TABLE replays (
    id INTEGER PRIMARY KEY,
    capture_id INTEGER NOT NULL REFERENCES captures(id),
    timestamp_replayed TEXT NOT NULL,
    adapter_url TEXT NOT NULL,
    adapter_version TEXT,              -- Git-Hash o.ä.
    response_status INTEGER,
    diff_class TEXT NOT NULL,          -- match | tolerated | mismatch | error
    diff_details TEXT                  -- JSON: welche Felder weichen ab
);

CREATE INDEX idx_captures_operation ON captures(operation);
CREATE INDEX idx_replays_diff_class ON replays(diff_class);
CREATE INDEX idx_replays_capture ON replays(capture_id);
```

### Sicherheit / DSGVO

- Storage-Volume auf LUKS-verschlüsselter Partition (Doku im README).
- Kein Body-Inhalt in Logs (nur Größen, Hashes).
- Anonymisierungs-Hook für Capture-Phase: konfigurierbare Liste von
  JSON-Pfaden in Request/Response, die durch `<REDACTED>` ersetzt werden
  (z.B. `$.user.email`, falls bekannt). Im ersten Wurf optional, aber
  als Konfigurations-Hook vorgesehen.
- HAR-Files enthalten potenziell personenbezogene Daten in PDF-Bodies.
  Zugriff strikt beschränkt; Aufbewahrungsfrist konfigurierbar.

## Was *nicht* Teil dieses Tasks ist

- Der OpenYard-Adapter selbst.
- Live-Differential-Testing (paralleler Real-Time-Vergleich). Hier nur
  Capture und Offline-Replay.
- Web-UI für Diff-Inspektion (kann auf SQLite nachgelagert gebaut werden).
- Integration in CI-Pipeline (eigener Task).

## Lieferumfang

1. **`pod-spec.yaml`**: Podman-Kube-YAML oder Compose-File für den Pod.
2. **`Containerfile`** (oder mehrere): für capture-addon und diff-service.
3. **`src/capture_addon.py`**: mitmproxy-Addon, HAR-Schreiber,
   Session-Token-Mapping, Operation-Erkennung aus Pfad.
4. **`src/replay.py`**: CLI-Tool für Offline-Replay + Diff.
5. **`src/diff_engine.py`**: Vergleichs-Logik mit konfigurierbaren
   Toleranz-Regeln.
6. **`config/operations.yaml`**: pro Operation Toleranz-Regeln,
   z.B.:

   ```yaml
   advancedDocuments.GetDocument:
     ignore_response_fields:
       - "$.Modified"      # Timestamp, irrelevant für Drop-in
       - "$.ETag"          # ID, irrelevant
     tolerate_response_fields:
       - path: "$.SessionID"
         match: regex
         pattern: "^[a-f0-9-]{36}$"
   ```

7. **`schema.sql`**: SQLite-Initialisierungs-Schema (siehe oben).
8. **`README.md`**: Setup-Anleitung für Debian-13-Host, inkl.
   LUKS-Setup, NIC-Konfiguration, TLS-CA-Verteilung, Health-Check.
9. **`docs/coverage.md`**: Template für die Operations-Coverage-Tabelle
   (Stufe 1/2/3, Validation-Status). Wird im Adapter-Repo gepflegt.

## Akzeptanzkriterien

1. **Capture-E2E**: Pod startet auf einer frischen Debian-13-VM mit
   einem Befehl (`podman play kube pod-spec.yaml`). Ein Test-Client
   gegen einen Mock-WinYard-Container produziert HAR-Einträge im
   Volume. Mindestens 10 verschiedene WinYard-Operationen werden im
   E2E-Test abgedeckt.
2. **Replay-E2E**: `openyard-replay` nimmt die generierten HARs, spielt
   sie gegen denselben Mock-WinYard ab (als "Adapter"-Surrogat) und
   liefert `diff_class=match` für ≥95% der Requests.
3. **Session-Mapping**: Wenn ein Capture-Flow `SessionID=ABC123` enthält
   und der Replay-Adapter beim Login `SessionID=XYZ789` zurückgibt,
   ersetzt der Replayer alle folgenden `ABC123` durch `XYZ789` in
   nachfolgenden Requests des gleichen Flows.
4. **Toleranz-Regeln**: Ein bewusst manipulierter Adapter, der bei
   `GetDocument` ein abweichendes `Modified`-Feld liefert, erzeugt
   trotzdem `diff_class=tolerated` (wenn `$.Modified` in der Ignore-
   Liste steht).
5. **DSGVO-Defaults**: HAR-Files werden auf einem Volume abgelegt, das
   im README-Setup als LUKS-Mount beschrieben ist. Aufbewahrungsfrist
   wird per Cron/systemd-Timer durchgesetzt (default 90 Tage).
6. **Reproduzierbarkeit**: `podman play kube pod-spec.yaml` und
   anschließende E2E-Tests laufen auf einem zweiten Debian-13-Host
   ohne Anpassungen durch.

## Technologie-Vorgaben

- **Sprache**: Python 3.12 (passt zu mitmproxy-Addons, ausreichend
  performant für Capture-Last <500 req/s).
- **mitmproxy**: aktuelle stabile Version, Reverse-Mode.
- **DB**: SQLite (kein Postgres im Pod).
- **Container-Runtime**: Podman (rootless wenn möglich, sonst rootful
  mit dokumentierter Begründung).
- **Build**: Containerfile, nicht Dockerfile (für Klarheit), aber
  kompatibel zu beidem.

## Hinweise zur Implementierung

- HAR-Format folgt der Spezifikation des HTTP Archive (W3C-Entwurf).
  Custom-Felder unter `_openyard.*` sind ergänzend und kollidieren
  nicht mit Standard-Parsern.
- Die Operation-Erkennung aus dem Pfad ist deterministisch: der letzte
  Pfad-Segment ist der Operationsname, das vorletzte ist die Kategorie
  (z.B. `/api/advancedDocuments/GetDocument` →
  `advancedDocuments.GetDocument`).
- Bei `multipart/form-data` (SetFile, SetDocument) muss der Body
  vollständig persistiert werden; der Diff vergleicht SHA-256 der
  einzelnen Parts.
- Replay sollte Idempotenz nicht voraussetzen. Wenn ein Capture eine
  Upload-Sequenz enthält, wird der Adapter bei Replay zusätzliche
  Dokumente erzeugen. Das ist OK – der Test prüft Wire-Verhalten,
  nicht Idempotenz. Test-Adapter sollten zwischen Test-Runs gereinigt
  werden.
- Konsistente Session-Behandlung: Capture-Phase ersetzt echte
  SessionIDs durch stabile Tokens vor Persistierung. Replay-Phase
  ersetzt diese Tokens durch frische SessionIDs aus Adapter-Login.

## Erste Schritte für den Code-Agent

1. Repo-Struktur aufsetzen: `src/`, `config/`, `tests/`, `docs/`,
   `pod-spec.yaml`, `Containerfile.capture`, `Containerfile.replay`.
2. `capture_addon.py` als minimales mitmproxy-Addon mit HAR-Writer.
3. `replay.py` als CLI-Skelett mit argparse.
4. `diff_engine.py` mit JSON-Diff-Logik (deepdiff o.ä. als Basis).
5. Mock-WinYard für Tests: einfacher FastAPI/Flask-Service, der ein
   paar Operations stubst.
6. End-to-End-Test in `tests/e2e/`: Mock starten, Pod starten,
   Test-Client laufen lassen, HAR prüfen, Replay laufen lassen,
   SQLite prüfen.

## Out-of-Scope-Erinnerung

Dieser Task baut **nur die Capture- und Replay-Infrastruktur**. Was
mit den gesammelten Daten passiert (Operations-Coverage-Pflege,
Adapter-Implementierung, Stage-Kategorisierung der Endpoints), ist
Sache nachgelagerter Tasks.
