# Aktenplan — Typen, Schutz und FolderViews

## Hierarchie

```
Space (Typ: aktenplan)
└── Aktenplan-Ebenen (protected, beliebig tief verschachtelt)
    └── Aktenschrank (shielded = letztes Blatt des Aktenplans)
        └── Akte (der "Leitzordner")
            ├── Variante leer: Dokumente direkt in der Akte
            ├── Variante thematisiert: Vorgang → Dokumente
            └── Variante voll: Vorgang → Register → Dokumente
```

## Typen (4 Stück, je ein FolderView)

| Typ | `.type_` | FolderView | Kinder | Schutz |
|-----|----------|-----------|--------|--------|
| **Aktenplan** | `.type_aktenplan` | Sachgruppen-Listing | Bei protected: aktenplan. Bei shielded: akte | protected / shielded |
| **Akte** | `.type_akte` | Akten-Ansicht | vorgang, dokument | — |
| **Vorgang** | `.type_vorgang` | Vorgangs-Ansicht | register, dokument | — |
| **Register** | `.type_register` | Register-Ansicht | dokument | — |

### Aktenplan: Zwei Modi über immutableState

Gleicher Typ `.type_aktenplan`, unterschiedliches Verhalten je nach Schutzstatus:

| immutableState | Bedeutung | Erlaubte Kinder | Actions |
|---------------|-----------|----------------|---------|
| `protected` | Sachgruppe (Struktur fixiert) | aktenplan (nur Manager) | "Neue Sachgruppe" (Manager) |
| `shielded` | Aktenschrank (letztes Blatt) | akte | "Neue Akte" (Editor+) |
| keiner | Ungeschützter Aktenplan-Ordner | aktenplan, akte | "Neue Sachgruppe", "Neue Akte" |

## Aktenzeichen (`oy.fileReference`)

Aktenzeichen werden **nie im Dateinamen** geführt, sondern als Metadatum:

```
xattr: user.oc.md.oy.fileReference = "11.12.01.03-01/1#1"
```

Der **Ordnername** ist frei wählbar (z.B. "Entschädigungssatzung", "Fassung 2016").
Das Aktenzeichen wird im FolderView als Spalte angezeigt und über die Metadata API
gelesen/geschrieben (`GET/PUT /metadata` → `{ "oy.fileReference": "..." }`).

### Aktenzeichen-Syntax

```
Ebene           Aktenzeichen      Trennzeichen
─────────────────────────────────────────────────
Sachgruppe 1    11                (Startcode)
Sachgruppe 2    11.12             . (Punkt)
Sachgruppe n    11.12.01          . (Punkt)
Aktenschrank    11.12.01.03       . (Punkt)
Akte            11.12.01.03-01    - (Bindestrich)
Vorgang         11.12.01.03-01/1  / (Schrägstrich)
Register        11.12.01.03-01/1#1  # (Raute)
```

### Beispiel Ordnerstruktur (Dateinamen ≠ Aktenzeichen)

```
Innere Verwaltung/                  oy.fileReference = "11"
  Kommunalverwaltung/               oy.fileReference = "11.12"
    Organisationsangelegenheiten/    oy.fileReference = "11.12.01"
      Satzungen/                     oy.fileReference = "11.12.01.03"
        Entschädigungssatzung/       oy.fileReference = "11.12.01.03-01"
          Fassung 2016/              oy.fileReference = "11.12.01.03-01/1"
            Vorlagen/                oy.fileReference = "11.12.01.03-01/1#1"
```

## FolderViews und Schema-Dateien

Jeder Typ hat eine Schema-Datei unter `.space/views/` und einen zugehörigen FolderView.

### .space/views/aktenplan.json
```json
{
  "label": "Aktenplan",
  "icon": "archive",
  "children": {
    "protected": ["aktenplan"],
    "shielded": ["akte"],
    "default": ["aktenplan", "akte"]
  },
  "columns": ["name", "oy.fileReference", "typ", "anzahl"],
  "fileReferencePattern": "{parentRef}.{seq:2}",
  "metadata": {
    "oy.fileReference": { "label": "Aktenzeichen", "type": "string", "auto": true }
  }
}
```

**FolderView Aktenplan** zeigt:
- Spalten: Name, Aktenzeichen (aus `oy.fileReference`), Untertyp, Anzahl Kinder
- Action-Button: "Neue Sachgruppe" (protected, Manager) oder "Neue Akte" (shielded, Editor+)

### .space/views/akte.json
```json
{
  "label": "Akte",
  "icon": "folder-open",
  "children": ["vorgang"],
  "columns": ["name", "oy.fileReference", "status", "abgelegt-von", "abgelegt-am"],
  "fileReferencePattern": "{parentRef}-{seq:2}",
  "metadata": {
    "oy.fileReference": { "label": "Aktenzeichen", "type": "string", "auto": true },
    "oy.status": {
      "label": "Status", "type": "enum",
      "values": ["offen", "gespeichert", "geschlossen"],
      "default": "offen"
    }
  }
}
```

**FolderView Akte** zeigt:
- Spalten: Name, Aktenzeichen, Status, abgelegt von/am
- Action-Button: "Neuer Vorgang", "Dokument hinzufügen"

### .space/views/vorgang.json
```json
{
  "label": "Vorgang",
  "icon": "file-list",
  "children": ["register"],
  "columns": ["name", "oy.fileReference", "version", "abgelegt-von", "abgelegt-am"],
  "fileReferencePattern": "{parentRef}/{seq}",
  "metadata": {
    "oy.fileReference": { "label": "Aktenzeichen", "type": "string", "auto": true },
    "oy.version": { "label": "Version", "type": "string" }
  }
}
```

**FolderView Vorgang** zeigt:
- Spalten: Name, Aktenzeichen, Version, abgelegt von/am
- Action-Button: "Neues Register", "Dokument hinzufügen"

### .space/views/register.json
```json
{
  "label": "Register",
  "icon": "bookmark",
  "children": [],
  "columns": ["name", "oy.fileReference", "abgelegt-von", "abgelegt-am"],
  "fileReferencePattern": "{parentRef}#{seq}",
  "metadata": {
    "oy.fileReference": { "label": "Aktenzeichen", "type": "string", "auto": true }
  }
}
```

**FolderView Register** zeigt:
- Spalten: Name, Aktenzeichen, abgelegt von/am
- Action-Button: "Dokument hinzufügen" (kein weiterer Ordner-Typ)

## Action-Buttons

Jeder FolderView hat typ-spezifische Action-Buttons im AppBar:

```
┌──────────────────────────────────────────────────────────────┐
│  ← Satzungen                                                │
│                                                              │
│  [+ Neue Akte]                               Filter  Ansicht│
│                                                              │
│  Name                       Aktenzeichen   Status    Datum   │
│  ──────────────────────────────────────────────────────────── │
│  📁 Entschädigungssatzung   11.12.01.03-01  offen    ...    │
│  📁 Feuerwehrsatzung        11.12.01.03-02  gesp.    ...    │
└──────────────────────────────────────────────────────────────┘
```

### Action-Button Logik

1. FolderView liest `children` aus Schema (+ `immutableState` für Aktenplan)
2. Für jeden erlaubten Kind-Typ: lade dessen Schema (Label, Icon)
3. Zeige Action-Button: "Neue(r/s) {label}"
4. Click → Dialog: Name eingeben, Aktenzeichen wird automatisch generiert
5. Erstellt: Ordner + `.type_<kind>` + `oy.fileReference` via Metadata PUT

### Aktenzeichen-Generierung im Dialog

```
┌─────────────────────────────────────────┐
│  Neue Akte anlegen                      │
│                                         │
│  Aktenzeichen: 11.12.01.03-03           │
│  (automatisch, nächste freie Nummer)    │
│                                         │
│  Name: [Brandverhütungsschauordnung  ]  │
│                                         │
│            [Abbrechen]  [Anlegen]        │
└─────────────────────────────────────────┘
```

Ordnername = Name-Eingabe. Aktenzeichen wird als `oy.fileReference` gespeichert.

## Beispiel: Komplette Struktur

```
Archikart DMS/                             .type_aktenplan           oy.fileReference=""
├── .space/views/{aktenplan,akte,vorgang,register}.json
├── Innere Verwaltung/                     .type_aktenplan protected oy.fileReference="11"
│   ├── Kommunalverwaltung/                .type_aktenplan protected oy.fileReference="11.12"
│   │   ├── Organisationsangelegenheiten/  .type_aktenplan protected oy.fileReference="11.12.01"
│   │   │   ├── Satzungen/                 .type_aktenplan shielded  oy.fileReference="11.12.01.03"
│   │   │   │   ├── Entschädigungssatzung/ .type_akte                oy.fileReference="11.12.01.03-01"
│   │   │   │   │   ├── Fassung 2016/      .type_vorgang             oy.fileReference="11.12.01.03-01/1"
│   │   │   │   │   │   ├── Vorlagen/      .type_register            oy.fileReference="11.12.01.03-01/1#1"
│   │   │   │   │   │   │   ├── Vergleich_A.pdf
│   │   │   │   │   │   │   └── Arbeitshilfe.docx
│   │   │   │   │   │   └── Beschluss.pdf
│   │   │   │   │   └── Fassung 2026/      .type_vorgang             oy.fileReference="11.12.01.03-01/2"
│   │   │   │   └── Feuerwehrsatzung/      .type_akte                oy.fileReference="11.12.01.03-02"
│   │   │   └── Landratsamt/               .type_aktenplan shielded  oy.fileReference="11.12.01.05"
│   │   └── Personalangelegenheiten/       .type_aktenplan protected oy.fileReference="11.12.02"
│   └── Finanzverwaltung/                  .type_aktenplan protected oy.fileReference="11.13"
├── Sicherheit und Ordnung/                .type_aktenplan protected oy.fileReference="12"
└── Schulträgeraufgaben/                   .type_aktenplan protected oy.fileReference="21"
```

## Implementierung

### Pro Typ ein FolderView

Jeder der 4 Typen bekommt eine eigene Vue-Komponente:

```
packages/web-app-files/src/components/TypedViews/
  AktenplanView.vue      ← Sachgruppen/Aktenschrank-Listing
  AkteView.vue           ← Akten-Ansicht mit Status
  VorgangView.vue        ← Vorgangs-Listing mit Version
  RegisterView.vue       ← Register-Listing (Blatt-Ebene)
  TypedNewDialog.vue     ← "Neuer [Typ]" Dialog mit Aktencode-Generator
```

### Integration in GenericSpace.vue

```
PROPFIND → Kinder-Liste
  │
  ├── .type_* gefunden?
  │   ├── Nein → Standard FolderView
  │   └── Ja → Typ erkennen
  │           ├── immutableState prüfen (protected/shielded)
  │           ├── Schema laden (.space/views/<typ>.json)
  │           └── Typed FolderView rendern
  │               ├── Typ-spezifische Spalten
  │               ├── Typ-spezifische Action-Buttons
  │               └── Typ-spezifisches Icon/Styling
```
