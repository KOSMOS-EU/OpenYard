# Task: OpenYard Akten-Tool für Windows (minimalistisch)

## Kontext

Im OpenYard-Setup arbeiten Sachbearbeiter über OpenCloud-Desktop mit
einem als Netzlaufwerk gemounteten Aktenplan-Space. Beim Anlegen einer
neuen Akte unterhalb eines Aktenplan-Knotens muss der Verzeichnisname
einer festen Konvention entsprechen (sächsischer Aktenplan).

Der normale Explorer-Workflow "Neuer Ordner" mit manueller Namenseingabe
ist fehleranfällig. Es wird ein kleines Hilfswerkzeug benötigt, das im
Kontextmenü des Windows-Explorers verfügbar ist, den nächsten freien
Aktennamen ermittelt und das Verzeichnis korrekt anlegt.

## Ziel

Ein minimalistisches Tool aus zwei Dateien:

- **`openyard-akte.exe`** - Standalone-Binary, keine Installation nötig
- **`register.ini`** oder `register.reg` - Eintrag ins Windows-Kontextmenü

Keine MSIX-App. Keine Server-Anbindung. Kein Token. Kein Auth.
Reine Filesystem-Operation im Kontext des bereits authentifizierten
OpenCloud-Desktop-Mounts.

## Funktionsumfang

Das Tool wird aus dem Kontextmenü heraus aufgerufen, mit dem aktuellen
Verzeichnispfad als Argument:

```
openyard-akte.exe "K:\Aktenplan\63-Bauen\Baugenehmigungen\2024"
```

Ablauf:

1. Tool liest den übergebenen Pfad.
2. Listet alle Unterverzeichnisse.
3. Findet das höchste vorhandene Aktenzeichen nach Konvention.
4. Schlägt das nächste freie vor.
5. Zeigt einen einfachen Dialog: "Neue Akte anlegen: BA-2024-0816 [OK / Abbrechen]".
6. Bei OK: `mkdir` des neuen Verzeichnisses.
7. Bei Konflikt (Race Condition, anderer Anwender war schneller):
   Retry mit nächster Nummer, max. 3 Versuche.
8. Bei Erfolg: Dialog schließt, Explorer zeigt nach Refresh den neuen Ordner.

## Konvention

Identisch im gesamten Aktenplan, daher hartcodiert oder über einfache
Variable konfigurierbar:

- **Format**: `{prefix}-{year}-{seq:04d}`
- **Prefix**: zweistelliger Buchstabencode, abgeleitet aus dem
  Aktenplan-Knoten (z.B. `BA` für Baugenehmigungen). Ableitung:
  letzter Pfadteil-Präfix in Großbuchstaben oder fester Mapping-
  Mechanismus über eine kleine eingebaute Tabelle.
  Falls keine eindeutige Ableitung möglich: Anwender wird im Dialog
  nach Präfix gefragt.
- **Year**: aktuelles Jahr (`time.Now().Year()`).
- **Seq**: höchste vorhandene Sequenznummer in den Geschwistern plus 1.
  Falls keine vorhanden: 1.

Beispiel: In `\Baugenehmigungen\2024\` existieren `BA-2024-0001` bis
`BA-2024-0815`. Tool schlägt `BA-2024-0816` vor.

## Lieferumfang

1. **`openyard-akte.exe`** - Single-File Binary, Windows x64.
2. **`register.reg`** - Registry-Datei zum Eintragen ins
   Kontextmenü (Doppelklick durch Anwender oder per Group Policy
   verteilt).
3. **`README.md`** - Installation in zwei Schritten:
   - EXE in einen festen Pfad legen (z.B. `C:\Program Files\OpenYard\`)
   - register.reg ausführen
4. **Optional: `uninstall.reg`** - Entfernt den Kontextmenü-Eintrag.

## Registry-Eintrag (Beispiel)

```reg
Windows Registry Editor Version 5.00

[HKEY_CLASSES_ROOT\Directory\Background\shell\OpenYardAkte]
@="Neue Akte anlegen"
"Icon"="C:\\Program Files\\OpenYard\\openyard-akte.exe,0"

[HKEY_CLASSES_ROOT\Directory\Background\shell\OpenYardAkte\command]
@="\"C:\\Program Files\\OpenYard\\openyard-akte.exe\" \"%V\""
```

Eintrag erscheint im Kontextmenü, wenn man im Explorer in einem
Verzeichnis-Hintergrund rechtsklickt (nicht auf einer Datei).

## Akzeptanzkriterien

1. **Build**: `openyard-akte.exe` lässt sich aus dem Source-Repository
   reproduzierbar bauen, ohne externe Abhängigkeiten zur Laufzeit.
2. **Funktionstest manuell**:
   - In leerem Test-Verzeichnis aufrufen → Vorschlag `XX-2026-0001`.
   - Mit `XX-2026-0001`, `XX-2026-0002` vorhanden → Vorschlag `XX-2026-0003`.
   - Mit Lücken (`XX-2026-0001`, `XX-2026-0005`) → Vorschlag `XX-2026-0006`
     (höchste vorhandene + 1, NICHT erste Lücke).
3. **Race-Condition-Test**: Zwei parallele Aufrufe im selben Verzeichnis
   produzieren zwei unterschiedliche Verzeichnisse, kein Fehler.
4. **Registry-Integration**: Nach Anwenden von `register.reg` erscheint
   im Explorer-Kontextmenü der Eintrag "Neue Akte anlegen".
5. **Pfad-Übergabe**: Klick auf den Kontextmenü-Eintrag in einem
   beliebigen Verzeichnis übergibt diesen Pfad korrekt an die EXE.

## Technologie-Vorgaben

- **Sprache**: Go (statisch gelinkt, single binary) oder Rust
  (gleicher Vorteil). C# möglich, aber dann .NET-Runtime-Abhängigkeit.
- **GUI**: Minimal. Eine einzige MessageBox oder ein einfacher Dialog
  reicht. In Go: `walk` oder native Win32-Calls über `golang.org/x/sys/windows`.
  In Rust: `native-windows-gui` oder `winsafe`.
- **Distribution**: Single EXE plus Registry-Datei. Kein Installer
  notwendig, kein MSIX, kein Code-Signing-Zwang (nice-to-have, aber
  nicht blockierend für initiale Verteilung in Test-Umgebung).

## Hinweise zur Implementierung

- Das Tool muss **defensiv** mit unklaren Pfaden umgehen:
  - Wenn der Pfad nicht existiert: Fehlerdialog und beenden.
  - Wenn der Pfad nicht beschreibbar ist: Fehlerdialog und beenden.
  - Wenn keine eindeutige Präfix-Ableitung möglich: Dialog mit
    Eingabefeld für Präfix.
- Die Präfix-Ableitung kann zu Beginn fest verdrahtet sein (Liste der
  bei Brandis verwendeten Präfixe) und später konfigurierbar gemacht
  werden, falls weitere Kommunen andere Konventionen haben.
- Das Listing der Geschwister muss alle Verzeichnisse berücksichtigen,
  auch solche, die nicht der Konvention entsprechen (sonst stört eine
  Fremd-Anlage die Nummerierung). Beim Pattern-Match wird nur die
  Sequenznummer aus passenden Namen extrahiert; andere werden ignoriert.

## Out-of-Scope

- Bearbeiten von bereits angelegten Akten.
- Synchronisation mit Server-Logik.
- Authentifizierung.
- Setzen von Metadaten oder Berechtigungen (passiert über die normalen
  OpenCloud-Mechanismen).
- Validierung, ob der Anwender überhaupt Schreibrechte hat - das
  übernimmt das Betriebssystem beim mkdir-Versuch.
- Linux/macOS-Variante. Falls später benötigt: separater Task.
