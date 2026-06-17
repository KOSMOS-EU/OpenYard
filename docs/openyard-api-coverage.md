# OpenYard API Coverage und Implementierungsstrategie

## Zweck dieses Dokuments

Diese Datei klassifiziert jede Operation der OpenYard-API nach
**Implementierungsstrategie** und dient als verbindlicher Leitfaden
für die Microservice-Implementierung (siehe
`openyard-microservice-task.md`).

## Implementierungs-Kategorien

| Kategorie | Bedeutung |
|---|---|
| **opencloud-framework** | OpenCloud (Proxy, User-Verwaltung, Graph-API) liefert das nativ. Adapter mappt nur. |
| **reva** | Reva/CS3 liefert das nativ. Adapter mappt Datenstrukturen. |
| **eigen** | Eigener Adapter-Code mit eigener Persistenz oder Logik. |
| **unsinnig-stub** | Funktion ist semantisch leer in unserer Architektur. Stub liefert plausibel-leere Antwort. |
| **archaeologie-wrapper** | Funktion existiert nur als WinYard-Altlast für Cross-Welt-Konflikte (Windows-Explorer vs. REST-API). Pass-Through ohne semantische Wirkung. |

## Coverage-Klassifikation (orthogonal)

Erinnerung an die Spezifikations-Stufen (separate Dimension):

- **Stufe 1**: Durch Capture-Samples nachgewiesen.
- **Stufe 2**: Nicht nachgewiesen, aber dokumentiert.
- **Stufe 3**: Nicht hinreichend spezifiziert.

Beim Start sind alle Operationen auf Stufe 2. Captures heben sie auf
Stufe 1. Diese Spalte wird in der Tabelle weggelassen, weil sie sich
laufend ändert und in einem separaten `coverage-log.md` gepflegt wird.

## Funktionsübersicht Core 1.1

### Authentication

| Endpoint | Strategie | Bemerkung |
|---|---|---|
| `POST /api/advancedUsers/Login` | **eigen** | Auth gegen OpenCloud, SessionID im In-Memory-Cache (TTL). |
| `GET /api/advancedUsers/Logout` | **eigen** | Session aus In-Memory-Cache löschen. |
| `GET /api/advancedUsers/Authenticate` | **eigen** | Prüft SessionID-Gültigkeit im In-Memory-Cache. |
| `GET /api/advancedUsers/GetSessionBySessionID` | **eigen** | Liefert Session-Metadaten (User, TTL). |
| `POST /api/advancedUsers/GetUserInfo` | **opencloud-framework** | User-Daten aus OpenCloud Graph-API, mit Field-Default-Engine angereichert. |

### Documents

| Endpoint | Strategie | Bemerkung |
|---|---|---|
| `POST /api/advancedDocuments/GetDocument` | **reva** | CS3 Stat + Mapping auf WinYard-Antwort. |
| `POST /api/advancedDocuments/SetDocument` | **reva** | CS3 InitiateFileUpload + PUT. |
| `POST /api/advancedDocuments/GetFile` | **reva** | CS3 Download. |
| `POST /api/advancedDocuments/SetFile` | **reva** | CS3 Upload. Versionierung passiert automatisch in Reva. |
| `GET /api/advancedDocuments/IsDocument` | **reva** | CS3 Stat + Type-Check. |
| `POST /api/advancedDocuments/BinDocuments` | **reva** | CS3 Move-to-Trash. |
| `POST /api/advancedDocuments/GetDeletedFiles` | **reva** | CS3 ListRecycle. |
| `GET /api/advancedDocuments/GetDocumentVersions` | **reva** | CS3 ListRevisions. |
| `GET /api/advancedDocuments/GetPreviewFile` | **opencloud-framework** | Thumbnails-Service von oCIS. |
| `GET /api/advancedDocuments/GetMetaData` | **reva** | CS3 Stat + ArbitraryMetadata. Field-Default-Engine fuellt auf. |

### Folders

| Endpoint | Strategie | Bemerkung |
|---|---|---|
| `POST /api/advancedFolders/GetFolder` | **reva** | CS3 Stat (Container) + ListContainer. |
| `POST /api/advancedFolders/GetFolderByFolderpath` | **reva** | CS3 LookupPath + Stat. |
| `GET /api/advancedFolders/IsFolder` | **reva** | CS3 Stat + Type-Check. |
| `POST /api/advancedFolders/SetFolder` | **reva** | CS3 CreateContainer. |
| `POST /api/advancedFolders/DeleteFolders` | **reva** | CS3 Move-to-Trash (rekursiv). |
| `GET /api/advancedFolders/GetFolderRights` | **opencloud-framework** | OCS Share API + CS3 Grants. |
| `POST /api/advancedFolders/SetFolderRights` | **opencloud-framework** | OCS Share API + CS3 AddGrant. |

### Common Operations

| Endpoint | Strategie | Bemerkung |
|---|---|---|
| `POST /api/advancedCommon/CopyObjects` | **reva** | CS3 Copy. |
| `POST /api/advancedCommon/MoveObjects` | **reva** | CS3 Move. |
| `POST /api/advancedCommon/RecoverObjects` | **reva** | CS3 RestoreRecycleItem. |
| `GET /api/advancedCommon/ExistObjectId` | **reva** | CS3 Stat. |
| `GET /api/advancedCommon/ExistObjectPath` | **reva** | CS3 LookupPath + Stat. |
| `POST /api/advancedCommon/Search` | **eigen** (Phase 2: separater Index-Dienst) | Phase 1: degradiert auf OpenCloud-Search (Title/Pfad) mit Hinweis im Header. |

### Search

| Endpoint | Strategie | Bemerkung |
|---|---|---|
| `POST /api/basicCommon/SearchForTitle` | **opencloud-framework** | OpenCloud-Search-Service. |
| `GET /api/basicCommon/GetMetaData` | **reva** | wie `advancedDocuments.GetMetaData`. |
| `GET /api/basicCommon/GetIndexData` | **eigen** | Pfad-basierte Ableitung (Aktenplan -> Klassifikation). Phase 1: einfache Heuristik aus Pfad. |

### Sharing / Permalinks

| Endpoint | Strategie | Bemerkung |
|---|---|---|
| `GET /api/basicCommon/GetExistingPermalinksByObjId` | **opencloud-framework** + **eigen** | OCS Public Links abfragen, ObjectId-Bezug aus Adapter-DB ergänzen. |
| `GET /api/basicCommon/DeletePermalink` | **opencloud-framework** | OCS Public Link löschen. |
| `GET /api/basicCommon/GetObjectIdByPermalink` | **opencloud-framework** | OCS Public Link auflösen. |
| `GET /api/basicCommon/GetPreviewFileByPermalink` | **opencloud-framework** | OCS Public Link + Thumbnails. |

### System

| Endpoint | Strategie | Bemerkung |
|---|---|---|
| `GET /api/advancedGeneral/IsListening` | **eigen** | Simpler Health-Endpoint. |
| `GET /api/advancedGeneral/GetServerSettings` | **eigen** | Statische Antwort mit OpenYard-Version, Capabilities. WinYard-kompatibles Schema. |
| `GET /api/basicCommon/GetFolderSubElementsByMemberId` | **reva** | CS3 ListContainer mit User-Filter. |
| `GET /api/basicConfig/GetOpenyardDMSUserSettings` | **eigen** (Phase 1: Default-Stub) | User-Settings aus Adapter-DB, default leer. |

## Funktionsübersicht Extended 1.1 (Auswahl)

Die Extended-API enthält 37 weitere Operationen. Eine vollständige
Klassifikation erfolgt in Phase 2, sobald Captures zeigen, welche
Extended-Operationen tatsächlich genutzt werden. Aus dem Spec-Material
hier bereits klassifizierbare Fälle:

### Document-Versionierung

| Endpoint | Strategie | Bemerkung |
|---|---|---|
| `POST /api/basicDocuments/CheckInAndIgnoreChanges` | **archaeologie-wrapper** | OpenYard hat kein Locking. Stub gibt `200 OK`. Loggt prominent. |
| `POST /api/basicDocuments/CreateNewDocVersion` | **archaeologie-wrapper** | Funktional identisch zu `SetFile`. Pass-Through. |

### Document-Import-Varianten

| Endpoint | Strategie | Bemerkung |
|---|---|---|
| `POST /api/basicDocuments/ImportDocumentByParentFolderID` | **reva** | Wie `SetFile` mit ObjectId-Auflösung. |
| `POST /api/basicDocuments/ImportDocumentByParentFolderPath` | **reva** | Wie `SetFile` mit Pfad-Anlage falls noetig. |
| `POST /api/basicDocuments/UploadFile` | **reva** | Wie `SetFile` mit zusätzlichen Flags (Preview, Overwrite). |
| `POST /api/basicDocuments/RenameDocument` | **reva** | CS3 Move innerhalb desselben Verzeichnisses. |

### Folder-Templates

| Endpoint | Strategie | Bemerkung |
|---|---|---|
| `POST /api/basicFolders/CreateFolderByParentFolderID` | **reva** | CS3 CreateContainer. |
| `POST /api/basicFolders/CreateFolderByParentFolderPath` | **reva** | CS3 CreateContainer mit Path-Auflösung. |
| `POST /api/basicFolders/RenameFolder` | **reva** | CS3 Move. |
| `GET /api/basicFolders/GetFolderTemplates` | **eigen** (Stub Phase 1) | Liefert leere Liste oder konfigurierbare Defaults. Aktenplan-Helper könnte später Templates aus Aktenplan-YAML ableiten. |

### Index- und Metadaten-Operationen

| Endpoint | Strategie | Bemerkung |
|---|---|---|
| `POST /api/basicCommon/SearchForIndex` | **eigen** (Phase 2) | Strukturierte Metadaten-Suche braucht separaten Index-Dienst. Phase 1: degradiert auf `SearchForTitle`. |
| `POST /api/basicCommon/SetIndex` | **reva** | Speichern als ArbitraryMetadata an CS3-Ressource. |

### System-/API-Discovery

| Endpoint | Strategie | Bemerkung |
|---|---|---|
| `GET /api/ApiExplorer/GetApiDescriptions` | **eigen** | Statische OpenAPI-Spec-Antwort. |
| `GET /api/basicGeneral/GetEnumerationAsDictionary` | **eigen** (Stub) | Liefert konfigurierbare Enums (Document-Kategorien etc.) aus YAML. |

## Aufgaben-Verteilung (Übersichts-Tabelle)

| Bereich | Hauptverantwortlich | Bemerkung |
|---|---|---|
| Login, Auth, Session | OpenCloud + In-Memory-Bridge | Adapter authentifiziert gegen OpenCloud, Session-Cache im Service. |
| Profildaten (Standard) | OpenCloud User-Verwaltung | Über Graph-API. |
| Profildaten (WinYard-spezifisch) | Adapter-DB | Soweit Felder genutzt werden. |
| Dokumente und Ordner | Reva/CS3 | Spaces, Files, Folders. |
| Datei-Metadaten Standard | Reva | Size, Mtime, ETag, MimeType. |
| Strukturierte Metadaten | Reva ArbitraryMetadata | Aktenzeichen, Klassifikation, etc. |
| Berechtigungen | OpenCloud-Rollen + CS3-Grants | OCS Share API als Frontend. |
| Versionierung | Reva | Out of the box. |
| Trash/Recovery | Reva | Out of the box. |
| Locking | **entfällt** | OpenYard-API kennt kein Locking. |
| Permalinks | OCS Public Links + Adapter-DB | Hybrid. |
| Suche (einfach) | OpenCloud-Search | Titel, Pfad. |
| Suche (komplex) | Separater Index-Dienst (Phase 2) | Boolean ueber Metadaten. |
| User-Settings (UI) | Adapter-DB | WinYard-spezifisch. |
| ObjectId | Base64-Kodierung CS3-ResourceID | Stateless. |
| Audit-Log | Reva + OpenCloud + Adapter-Loggin | Mehrschichtig. |

## Field-Default-Strategie (Felder-Ebene)

Für jeden Antwort-Datentyp gibt es eine konfigurierte Field-Default-
Policy. Drei Kategorien:

- **A funktional**: muss aus echter Quelle kommen (OpenCloud, CS3).
- **B kontextuell**: kommt aus Adapter-DB falls verfügbar, sonst Default.
- **C ballast**: konstant leer oder `"N/A"`. Bewusste Reduktion.

Beispiel User-Felder:

| Feld | Kategorie | Default |
|---|---|---|
| LoginName | A | (opencloud.user.username) |
| Klarname | A | (opencloud.user.displayName) |
| Email | A | (opencloud.user.email) |
| Funktion | C | `""` |
| Telefon | C | `""` |
| SachbearbeiterKennung | B | adapter-db, sonst `""` |
| Adresse | C | `"N/A"` |
| Geburtsdatum | C | `null` |
| SuperNutzer | A | (opencloud.user.role.admin) |
| Gesperrt | A | `!opencloud.user.enabled` |

## Out-of-Scope-Erinnerung

Diese Übersicht ist die **architektonische Soll-Klassifikation**. Sie
wird über Capture-Ergebnisse validiert und ggf. korrigiert. Wenn
Captures zeigen, dass ein als "archaeologie-wrapper" klassifizierter
Endpoint tatsächlich produktive Semantik hat, wird er hochgestuft.
Umgekehrt: wenn ein als "reva" klassifizierter Endpoint nie aufgerufen
wird, kann er als "unsinnig-stub" implementiert werden, bis ein
Kunde es braucht.

Die Datei wird mit jedem Capture-Cycle aktualisiert. Versionierung
über Git.

## WinYard-Felder: API-Bezug vs. SQL-Interna

**Wichtige Disziplin**: Die WinYard-Objekttabelle hat 35 SQL-Spalten,
aber **nicht jede Spalte ist ein API-Feld**. Vieles ist Server-internes
Buchhaltungs-Metadatum, das nie über die WinYard-API gesendet oder
empfangen wird. Der Adapter implementiert ausschließlich Felder, die
in der API-Spezifikation oder in beobachteten Captures auftauchen.

### Felder, die sicher API-relevant sind

Diese Felder werden in WinYard-API-Antworten von praktisch allen
Folder/Document-Operationen erwartet (basierend auf der OpenAPI-Spec
und allgemeinem DMS-Verhalten):

| Feld | Mapping | Quelle |
|---|---|---|
| ObjectId | CS3-ResourceID (Base64) | Reva |
| ParentId | CS3-Parent-ResourceID | Reva |
| Name / Description | Ordner-/Dateiname | Reva |
| Created | CS3 ctime | Reva |
| Modified | CS3 mtime | Reva |
| Creator (ID + Name) | User-Lookup | OpenCloud |
| LastModifiedBy (ID + Name) | User-Lookup | OpenCloud |
| MimeType (bei Dokumenten) | CS3 mime_type | Reva |
| Size (bei Dokumenten) | CS3 size | Reva |
| ETag | CS3 etag | Reva |

### Felder, die möglicherweise API-relevant sind

Diese Felder existieren in der SQL-Tabelle und könnten in API-Antworten
auftauchen. Der Adapter implementiert sie **nur, wenn Captures zeigen,
dass Clients sie tatsächlich lesen oder setzen**:

| Feld | Falls genutzt: |
|---|---|
| `aktz` (Aktenzeichen) | Aus Tree-Pfad generiert (Algorithmus unten) |
| `aktz_by_template` | Position des Knotens unter Parent (Counter in Adapter-DB) |
| `type` (ObjectType) | CS3-Metadata `winyard.object_type`, durchgeschleift |
| `folderlevel` | Aus AncestorPath berechnet |
| `hasnotes` / `hasstw` | Indikator-Flags, falls Notiz-/STW-System gebraucht wird |
| `care_period_id` und ROM-Felder | Aufbewahrungsfristen (rechtsrelevant!) |
| `index_*` | Indizierungs-Mechanik |

### Felder, die wahrscheinlich SQL-Interna sind

Diese Felder werden vermutlich nie über die API gehen und können
ignoriert werden, bis Captures das Gegenteil zeigen:

- `master`, `m_status` (Master-Konzept unklar, in Daten leer)
- `session_id` (Server-interne Anlage-Session)
- `status` (interner Status-Flag)
- `checked_out` (WinYard-internes Locking, keine API-Endpoints)
- `level_0_parent_id` (Root-Tree-Referenz, redundant zu Space-Kontext)
- `fullpath` (GUID-Kette, vom Adapter zur Laufzeit aus Tree generierbar)
- `autofreeze_*` (außer wenn rechtlich gefordert)
- `del_date_final`, `date_rom`, `action_after_rom` (interne RM-Buchhaltung)
- `subfile_counter` (Server-interner Zähler, nur für Aktenzeichen-Vergabe)

Diese werden nicht im Adapter-API-Layer abgebildet. Falls die
Aktenzeichen-Vergabe einen Counter braucht, lebt der intern im Adapter,
ohne über die API zu gehen.

### Aktenzeichen-Generator

Das Aktenzeichen ist offenbar aus dem Tree-Pfad berechnet und nicht
unabhängig gespeichert. Wenn Captures zeigen, dass Clients ein
Aktenzeichen in API-Antworten erwarten:

**Algorithmus**:

```
def generate_aktz(node):
    if node.parent is None or node.parent.aktz is None:
        return None
    
    parent_aktz = node.parent.aktz
    position = node.position_under_parent  # adapter-interner Counter
    separator = separator_for_depth(node.depth)
    
    return parent_aktz + separator + str(position)
```

**Trenner-Tabelle** (aus Brandis-Daten ermittelt, in Captures zu verifizieren):

| Tiefe | Trenner | Beispiel |
|---|---|---|
| 5 | `-` | `11.12.02.01-15` |
| 6 | `/` | `11.12.02.01-15/27` |
| 7 | `#` | `11.12.02.01-15/27#33` |
| 8+ | `-` | `11.12.02.01-15/27#33-9-4-2-2-1` |

### Aufbewahrungsfristen

Falls die WinYard-API tatsächlich Endpoints für Aufbewahrungsfristen
exponiert (`SetCareperiod`, `GetCareperiod` o.ä.), ist dies ein
**rechtsrelevantes Drop-in-Feature**, das in Phase 1 implementiert
werden muss. Sonst gehen DSGVO- und Archivgesetz-Daten bei Migration
verloren.

Die konkrete Implementierungs-Tiefe (passive Datenhaltung vs. aktive
Verarbeitung mit Autofreeze und Auto-Löschung) wird aus Captures
abgeleitet. Bei reiner Datenhaltung: Adapter-DB. Bei aktiver
Verarbeitung: zusätzlicher Cron-Worker im Adapter.

## Offene Fragen für die Capture-Phase

Diese Fragen werden in der Capture-Phase systematisch beantwortet und
das Coverage-Modell entsprechend verfeinert:

1. **Welche der 35 SQL-Felder erscheinen tatsächlich in API-Antworten?**
   (Die meisten vermutlich nicht.)
2. **Generiert der Server das Aktenzeichen automatisch, oder schickt
   der Client es mit?**
3. **Wann ändert sich `aktz`?** Bei Verschiebung neu berechnet oder
   statisch?
4. **Was sind die genauen Trenner-Regeln pro Tiefe?**
5. **Gibt es Aktenplan-Templates?** Werden bei Personalakten die
   `0./1./2./.../6.`-Bestandteile automatisch beim Anlegen einer neuen
   Personalakte mitkreiert?
6. **Werden Aufbewahrungsfristen über die API gepflegt?**
7. **Existieren Notizen- und Stichwort-Endpoints in der API?**
