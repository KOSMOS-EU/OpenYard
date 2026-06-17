# Task: Aktenplan-Extraktor und OpenCloud-Setup-Helper

## Kontext

Im KOSMOS-/OpenYard-Setup gibt es bei Brandis und anderen Referenzkunden
einen **handgepflegten Aktenplan**, der als unveränderliche obere
Ordnerstruktur in einem WinYard-Volume liegt. Unter dessen Blättern werden
neue Ordner mit Aktenkennung angelegt.

Der Aktenplan ist in WinYard kein API-Konzept, sondern eine
**Setup-Konvention**. Er muss extrahiert und in OpenCloud als
strukturierter Space mit definierten Berechtigungen wieder aufgebaut
werden.

### Strategische Leitlinien

1. Der Aktenplan ist **kommunales Verwaltungswissen**, kein
   Anbieterprodukt. Die extrahierte Form soll als wiederverwendbares
   Asset (z.B. YAML) gepflegt werden können.
2. Setup ist **einmaliger Vorgang pro Kommune** beim Onboarding,
   ergänzt um gelegentliche Aktenplan-Erweiterungen.
3. Berechtigungen am Aktenplan-Baum werden initial gesetzt und bleiben
   weitgehend stabil. Spätere Pflege ist Admin-Aufgabe in OpenCloud.

## Ziel dieses Tasks

Zwei zusammenhängende CLI-Werkzeuge:

1. **`aktenplan-extract`**: Liest einen vorhandenen WinYard-Aktenplan
   aus und exportiert ihn als maschinenlesbares YAML-Format.
2. **`aktenplan-apply`**: Nimmt eine YAML-Definition entgegen und legt
   sie als Space-Struktur mit Berechtigungen in OpenCloud/Reva an.

Beide Tools sind eigenständig nutzbar und müssen sich nicht gegenseitig
voraussetzen. Eine extrahierte YAML kann auch manuell editiert werden,
bevor sie eingespielt wird.

## Extraktion: Datenquellen

Der Aktenplan-Extraktor unterstützt **zwei Modi**, je nach verfügbarem
Zugriff beim Kunden:

### Modus A: SQL-Direktzugriff

Wenn DB-Zugriff auf die WinYard-Instanz (Microsoft SQL Server) verfügbar
ist, ist das der schnellste Weg.

**Annahmen über das Schema** (zu verifizieren beim ersten Kunden):
- Eine Tabelle, vermutlich mit Präfix `K`, `V` oder `T`, enthält die
  Ordner-Hierarchie. Wahrscheinliche Kandidaten: `V0000FolderTree`,
  `K0001Ordner`, oder ähnlich. Genauer Name muss per Schema-Inspektion
  ermittelt werden.
- Selbst-referenzierende Spalte `ParentId` oder ähnlich.
- Mandanten-Filter über Spalte `Kind` (siehe Brandis-Beispiel:
  `Kind = '96956270100917093657456'`).
- Volume- oder Space-Zuordnung über eigene Spalte (z.B. `VolumeId`).

**Vorgehen**:
1. Verbindung zur MSSQL-DB (über `denisenkom/go-mssqldb`).
2. Schema-Inspektion: Tabellen mit Selbstreferenz auf Parent-ID
   identifizieren, Folder-Hierarchien finden.
3. Konfigurierbares Mapping (YAML) der Spaltennamen: welche Spalte ist
   ID, welche ist Parent, welche ist Name, welche ist
   Aktenkennung etc.
4. Rekursive Traversierung ab Root des angegebenen Volumes/Spaces.
5. Export der Struktur als YAML.

### Modus B: OpenYard-API-Crawl

Wenn nur API-Zugriff verfügbar ist (kein DB-Login), wird der Aktenplan
über die OpenYard-API rekursiv durchsucht.

**Vorgehen**:
1. Login gegen die WinYard-API (Username/Passwort -> SessionID).
2. Start-Pfad konfigurierbar (z.B. `/Aktenplan/`).
3. Rekursive Traversierung mit `GetFolderByFolderpath` und
   `GetFolderSubElementsByMemberId`.
4. Für jeden Knoten: Name, Pfad, ggf. Beschreibung, ggf. Rechte
   (über `GetFolderRights`).
5. Export als YAML.

**Hinweis**: Modus B ist langsamer als Modus A (viele HTTP-Calls), aber
robuster gegen Schema-Variationen. Wenn das Schema beim Kunden
unbekannt ist, ist Modus B der sicherere Start.

## YAML-Format des Aktenplans

Vorschlag (anpassbar, soll diskutiert werden):

```yaml
aktenplan:
  metadata:
    kommune: Brandis
    quelle: winyard_extract
    extraktionsdatum: 2026-05-25
    aktenplan_version: "1.3"
    bemerkung: "Stand vor Migration nach OpenYard"

  space:
    name: "Aktenplan"
    typ: "managed"
    quota_gb: 500
    initial_rechte:
      - rolle: "AktenplanAdmin"
        wirkung: "manage"
      - rolle: "Sachbearbeiter"
        wirkung: "read"

  baum:
    - kennung: "00"
      name: "Allgemeine Verwaltung"
      immutable: true
      kinder:
        - kennung: "00.01"
          name: "Organisation und Verwaltungsorganisation"
          immutable: true
          rechte:
            - rolle: "Hauptverwaltung"
              wirkung: "write"
          kinder:
            - kennung: "00.01.01"
              name: "Hauptsatzung"
              immutable: true
        - kennung: "00.02"
          name: "Mitarbeiter"
          immutable: true
    - kennung: "10"
      name: "Sicherheit und Ordnung"
      immutable: true
    # ... weitere Top-Level-Knoten
```

**Wichtige Eigenschaften**:
- `immutable: true` markiert Knoten, die nicht editierbar sind (obere
  Aktenplan-Hierarchie). Diese werden mit entsprechenden Rechten
  geschützt: niemand außer dem AktenplanAdmin kann sie umbenennen,
  verschieben oder löschen.
- Blätter ohne `immutable: true` sind erweiterbar. Unter ihnen können
  Sachbearbeiter eigene Unter-Ordner mit Aktenkennungen anlegen.
- `rechte:` werden vererbt auf alle Unter-Knoten, sofern nicht
  überschrieben.

## Apply: Einspielen in OpenCloud

### Vorgehen

1. CLI nimmt YAML entgegen, validiert Schema.
2. Verbindung zum CS3-Gateway (über `cs3org/go-cs3apis`).
3. Erzeugt Space mit konfigurierten Eigenschaften (CreateStorageSpace).
4. Legt rekursiv Ordnerstruktur an (CreateContainer).
5. Setzt Berechtigungen pro Ordner (AddGrant).
6. Markiert immutable Ordner über Reva ArbitraryMetadata (z.B.
   `openyard.aktenplan.immutable=true`).
7. Logs aller angelegten Knoten plus Status.

### Idempotenz

Das Tool muss **idempotent** sein. Erneutes Apply auf eine bereits
angelegte Struktur:
- Bereits existierende Ordner werden nicht neu angelegt.
- Berechtigungen werden mit Soll-Zustand abgeglichen (additiv oder
  überschreibend, konfigurierbar).
- Fehlende Knoten werden ergänzt.
- Geänderte Namen werden gemeldet, aber NICHT automatisch umbenannt
  (das ist eine Aktenplan-Pflege-Entscheidung, kein Setup-Schritt).

## Lieferumfang

1. **`cmd/aktenplan-extract/main.go`** - CLI für Extraktion
2. **`cmd/aktenplan-apply/main.go`** - CLI für Einspielen
3. **`pkg/extract/sql/`** - SQL-Modus, MSSQL-Anbindung
4. **`pkg/extract/api/`** - API-Modus, OpenYard-Client
5. **`pkg/yaml/`** - YAML-Schema-Definition, Parser, Validator
6. **`pkg/apply/`** - CS3-basierte Anwendung der YAML auf Reva
7. **`pkg/permissions/`** - Berechtigungs-Mapping (Aktenplan-Rollen
   auf OpenCloud-Rollen)
8. **`config/schema-mapping-winyard.yaml`** - Beispiel-Mapping für
   bekannte WinYard-Schema-Variante
9. **`examples/brandis-aktenplan.yaml`** - anonymisierte Beispiel-YAML
   eines kommunalen Aktenplans (mit Brandis als Vorlage, bereinigt)
10. **`docs/`** - Setup, Format-Referenz, Migrationsleitfaden
11. **`tests/`** - Unit-Tests (Parser, Validator) und Integration-Tests
    (gegen Mock-MSSQL und Mock-Reva)

## Akzeptanzkriterien

1. **Extraktion SQL**: Gegen einen Mock-MSSQL-Container mit
   simuliertem WinYard-Schema liefert `aktenplan-extract --mode sql`
   eine YAML, die alle Knoten und ihre Hierarchie korrekt abbildet.
2. **Extraktion API**: Gegen einen Mock-WinYard-Server liefert
   `aktenplan-extract --mode api` eine semantisch gleichwertige YAML.
3. **Roundtrip**: Eine extrahierte YAML kann mit `aktenplan-apply` in
   eine leere Reva-Instanz angewendet werden. Die resultierende
   Ordner-Struktur entspricht der YAML.
4. **Idempotenz**: Zweites `aktenplan-apply` auf dieselbe YAML produziert
   keine Duplikate, keine Fehler, sondern lediglich einen Bericht
   "alle Knoten existieren, alle Rechte konsistent".
5. **Immutable-Schutz**: Nach `aktenplan-apply` ist ein
   `immutable`-Knoten für Standardnutzer nicht umbenennbar oder
   löschbar (verifiziert per CS3-Aufruf mit Standard-User-Token).
6. **Diff-Anzeige**: `aktenplan-apply --dry-run` zeigt an, was sich
   ändern würde, ohne Änderungen durchzuführen.

## Technologie-Vorgaben

- **Sprache**: Go (konsistent zu OpenYard-Service)
- **MSSQL-Treiber**: `github.com/denisenkom/go-mssqldb`
- **CS3-API**: gleicher Stack wie OpenYard-Service
- **YAML**: `gopkg.in/yaml.v3`
- **CLI**: `spf13/cobra`

## Hinweise zur Implementierung

- Das Schema-Mapping (Modus A) MUSS konfigurierbar sein. Das WinYard-
  Schema kann je nach Version oder Customizing variieren. Eine
  Konfigurationsdatei beschreibt, welche Tabelle und welche Spalten
  als Aktenplan-Quelle dienen.
- Beim Schema-Discovery (vor der Extraktion) hilft eine
  "Auto-Detect"-Funktion, die typische Patterns sucht (selbst-
  referenzierende Folder-Tabellen, Mandantenfilter, Volume-Markierung).
  Findet sie nichts Eindeutiges, wird der Anwender interaktiv durch
  die Konfiguration geführt.
- Beim Apply sollte das Tool defensiv vorgehen: kein Löschen ohne
  explizites Flag, kein Überschreiben von vorhandenen Dokumenten,
  kein automatisches Korrigieren von Konflikten.
- Personenbezogene Daten (Sachbearbeiter-Zuordnung) sollten in der
  YAML als Rollen, nicht als konkrete Personen abgebildet sein. Die
  Personen-Rollen-Zuordnung passiert in OpenCloud, nicht in
  der Aktenplan-YAML.

## Out-of-Scope

- Migration von **Inhalten** (Dokumenten) aus WinYard nach OpenCloud.
  Das ist ein separater ETL-Job.
- Aktive Synchronisation Aktenplan WinYard <-> OpenCloud. Der Aktenplan
  wird einmalig extrahiert, dann in OpenCloud weitergepflegt.
- Aktenplan-Pflege-UI. Aktenplan-Änderungen erfolgen entweder über
  YAML-Edit + erneutes Apply, oder direkt in OpenCloud (Web-UI).
