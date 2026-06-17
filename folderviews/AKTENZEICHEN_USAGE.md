# Aktenzeichen-Nutzung in WinYard (Stadt Brandis)

**Analyse:** 2026-06-17, Quelle: WinYard DMS via Loewe-Account
**20 Volumes, ~12.240 Ordner, ~41.188 Dokumente**

## Übersicht

| Volume | Root-Aktz | Ordner | Aktz im Feld | Aktz im Namen | Muster |
|---|---|---|---|---|---|
| Innere Verwaltung | 11 | 242 | 231 | 0 | Feld-gepflegt |
| Sicherheit und Ordnung | 12 | 37 | 37 | 0 | Feld-gepflegt |
| Räumliche Planung und Entwicklung | 51 | 6 | 6 | 0 | Feld-gepflegt |
| Bau- und Grundstücksordnung | 52 | 7 | 7 | 4 | Feld + Name |
| Ver- und Entsorgung | 53 | 20 | 0 | 4 | Nur im Namen |
| Verkehrsflächen und -anlagen | 54 | 24 | 24 | 0 | Feld-gepflegt |
| Natur- und Landschaftspflege | 55 | 35 | 35 | 1 | Feld-gepflegt |
| Wirtschaft und Tourismus | 57 | 16 | 16 | 0 | Feld-gepflegt |
| Allgemeine Finanzwirtschaft | 61 | 4 | 4 | 0 | Feld-gepflegt |
| Katastrophenschutz, Hochwasser | 71 | 2 | 2 | 0 | Feld-gepflegt |
| Lohnverrechnung | 81 | 2 | 2 | 0 | Feld-gepflegt |
| 99 Archikart DMS | 99 | 4.781 | 0 | 0 | Extern (Archikart) |
| ___Klammer | — | 111 | 0 | 19 | Nur im Namen |
| Mitarbeiterordner | — | 712 | 1 | 5 | Kaum Aktz |
| Anordnungen | — | 21 | 0 | 0 | Kein Aktz |
| Briefvorlagen | — | 1 | 0 | 0 | Kein Aktz |
| Post Eingang | — | 4 | 0 | 0 | Kein Aktz |
| Rechnungsausgang | — | 3.771 | 0 | 0 | Kein Aktz |
| Test LCS/KISA | — | 425 | 0 | 0 | Kein Aktz |
| Vorlagen | — | 54 | 0 | 0 | Kein Aktz |

## Drei Gruppen

### 1. Feld-gepflegt (10 Volumes)
Aktenzeichen als `Aktz`-Feld im WinYard FolderInfo gespeichert.
Hierarchisch aufgebaut: Root `52` → `52.20` → `52.20.01`.
Bei Migration als `oy.fileReference` xattr übernommen.

**Volumes:** Innere Verwaltung, Sicherheit und Ordnung, Räumliche Planung,
Bau- und Grundstücksordnung, Verkehrsflächen, Natur- und Landschaftspflege,
Wirtschaft und Tourismus, Allgemeine Finanzwirtschaft, Katastrophenschutz, Lohnverrechnung

**Beispiel:**
```
Bau- und Grundstücksordnung (Aktz: 52)
  └── 52.20 (Aktz: 52.20)
      └── 52.20.01 (Aktz: 52.20.01)
```

### 2. Nur im Ordnernamen (2 Volumes)
Aktenzeichen steckt als Präfix im Ordnernamen (z.B. `53.10 Gebäudemanagement`),
das `Aktz`-Feld im FolderInfo ist leer.

**Volumes:** Ver- und Entsorgung, ___Klammer

**Beispiel:**
```
Ver- und Entsorgung (Aktz: 53)
  └── 53.10 Gebäudemanagement (Aktz: '')
      └── 53.10.01 Betriebskostenabrechnung (Aktz: '')
```

### 3. Extern geführt (1 Volume)
Aktenzeichen/Flurstücksreferenzen kommen von Archikart-Software.
Stecken ausschließlich in Ordner- und Dateinamen, kein Aktz-Feld.
Die DocIndex-Felder `Flurstücksdruckident` und `Aktenzeichen` an den
Dokumenten sind die einzige strukturierte Referenz.

**Volume:** 99 Archikart DMS (4.781 Ordner, 1.848 Docs)

**Beispiel:**
```
99 Archikart DMS (Aktz: 99)
  └── Liegenschaften (Aktz: '')
      └── Flurstücke (Aktz: '')
          └── 8604 - 287/23 (Aktz: '')     ← Flurstück nur im Namen
              └── 2.Verträge (Aktz: '')
                  └── Dok: Flurstücksdruckident=8604/00287/0023  ← im DocIndex
```

## Konsequenzen für die Migration

1. **Gruppe 1** (Feld-gepflegt): Aktz wird als `oy.fileReference` am Ordner gespeichert ✓
2. **Gruppe 2** (Nur im Namen): Aktz muss aus dem Ordnernamen extrahiert werden
   (Regex `^\d{2}\.\d{2}` als Präfix). Alternativ: Ordnername beibehalten reicht.
3. **Gruppe 3** (Extern): Flurstücksreferenzen aus DocIndex (`info.parcelIdent`,
   `info.fileReference`) + Pfad (`oy.fullPath`) sind die Gegenreferenz.
   Der Ordnername ist die einzige Verbindung zur Archikart-Welt.
