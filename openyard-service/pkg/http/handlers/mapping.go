package handlers

import (
	"encoding/base64"
	"fmt"
	"path"
	"strings"
	"time"

	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	typesv1 "github.com/cs3org/go-cs3apis/cs3/types/v1beta1"
	"github.com/kosmos-eu/openyard/pkg/metadata"
	"github.com/kosmos-eu/openyard/pkg/migration"
)

// encodeObjectID creates the legacy DMS-compatible ObjectId from a CS3 ResourceId.
// Format: base64(storage_id$space_id!opaque_id) — matches CS3 triple format.
func encodeObjectID(rid *provider.ResourceId) string {
	if rid == nil {
		return ""
	}
	raw := rid.StorageId + "$" + rid.SpaceId + "!" + rid.OpaqueId
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(raw))
}

// decodeObjectID parses an ObjectId to CS3 ResourceId.
// Tries: 1) base64 encoded (OpenYard native)
//        2) Migration DB lookup (legacy DMS GUID → OpenYard ID)
func decodeObjectID(objectID string) (*provider.ResourceId, error) {
	// Try migration lookup first for legacy DMS GUIDs (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
	if len(objectID) == 36 && objectID[8] == '-' && objectID[13] == '-' {
		oyID := migration.LookupOpenYardID(objectID)
		if oyID != "" {
			return decodeObjectID(oyID) // Recurse with OpenYard ID
		}
	}

	raw, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(objectID)
	if err != nil {
		return nil, fmt.Errorf("invalid ObjectId encoding: %w", err)
	}
	s := string(raw)

	// Parse: storage_id$space_id!opaque_id
	dollarIdx := -1
	bangIdx := -1
	for i := 0; i < len(s); i++ {
		if s[i] == '$' && dollarIdx < 0 {
			dollarIdx = i
		}
		if s[i] == '!' {
			bangIdx = i
		}
	}

	if bangIdx < 0 {
		return nil, fmt.Errorf("invalid ObjectId format")
	}

	rid := &provider.ResourceId{}
	if dollarIdx >= 0 && dollarIdx < bangIdx {
		// Full triple: storage$space!opaque
		rid.StorageId = s[:dollarIdx]
		rid.SpaceId = s[dollarIdx+1 : bangIdx]
		rid.OpaqueId = s[bangIdx+1:]
	} else {
		// Legacy: storage!opaque
		rid.StorageId = s[:bangIdx]
		rid.OpaqueId = s[bangIdx+1:]
	}
	return rid, nil
}

// mapResourceInfo converts CS3 ResourceInfo to legacy DMS-compatible JSON map.
// Uses the handler's metadata config for reverse-mapping namespace keys.
func (h *Handlers) mapResourceInfo(info *provider.ResourceInfo) map[string]interface{} {
	// Unescape folder names (U+2215 → "/") for display
	displayName := strings.ReplaceAll(path.Base(info.Path), "\u2215", "/")
	displayPath := strings.ReplaceAll(info.Path, "\u2215", "/")

	m := map[string]interface{}{
		"id":       encodeObjectID(info.Id),
		"name":     displayName,
		"path":     displayPath,
		"modified": formatTimestamp(info.Mtime),
		"etag":     info.Etag,
	}

	if info.Type == provider.ResourceType_RESOURCE_TYPE_FILE {
		m["type"] = "file"
		m["size"] = info.Size
		m["mimeType"] = info.MimeType
	} else {
		m["type"] = "folder"
	}

	if info.ParentId != nil {
		m["parentId"] = encodeObjectID(info.ParentId)
	}

	// Arbitrary metadata — reverse-map namespace keys for client display
	cfg := h.metaCfg
	if cfg == nil {
		cfg = defaultMetaCfg
	}
	if info.ArbitraryMetadata != nil {
		for k, v := range info.ArbitraryMetadata.Metadata {
			clientKey := cfg.ToClientKey(k)
			m[clientKey] = v
		}
	}

	return m
}

func formatTimestamp(ts *typesv1.Timestamp) string {
	if ts == nil {
		return ""
	}
	return time.Unix(int64(ts.Seconds), int64(ts.Nanos)).UTC().Format("2006-01-02T15:04:05Z")
}

// refFromObjectID creates a CS3 Reference from a legacy DMS ObjectId.
func refFromObjectID(objectID string) (*provider.Reference, error) {
	rid, err := decodeObjectID(objectID)
	if err != nil {
		return nil, err
	}
	return &provider.Reference{ResourceId: rid}, nil
}

// refFromPath creates a CS3 Reference from a path string.
func refFromPath(p string) *provider.Reference {
	return &provider.Reference{Path: p}
}

// mapFolderInfo converts CS3 ResourceInfo to legacy DMS FolderInfo.
func mapFolderInfo(info *provider.ResourceInfo) map[string]interface{} {
	return map[string]interface{}{
		"FolderID":       encodeObjectID(info.Id),
		"ParentFolderID": encodeObjectID(info.ParentId),
		"Foldername":     strings.ReplaceAll(path.Base(info.Path), "\u2215", "/"),
		"FolderLevel":    0,
		"Aktz":           "",
		"Status":         0,
		"HasSubElements": true,
		"Created":        formatTimestamp(info.Mtime),
		"LastUpdated":    formatTimestamp(info.Mtime),
		"Rights": map[string]interface{}{
			"RightsDigit":    4095,
			"FolderCreate":   true,
			"FolderDelete":   true,
			"FolderChange":   true,
			"DocumentsRead":  true,
			"DocumentsCreateAndChange": true,
			"FolderSetRights": true,
		},
	}
}

// taskLogOK returns a standard legacy DMS TaskLog with no errors.
//
// NOTE: metaKeyMap and infoKeyMap have been moved to pkg/metadata/defaults.go.
// They are loaded via metadata.DefaultConfig() and configurable via YAML.

// legacyMetaKeyMap is kept for reference only — not used at runtime.
var _ = map[string]string{
	// Document metadata
	"Betreff":              "oy.subject",
	"Beschreibung":         "oy.description",
	"Kategorie":            "oy.category",
	"Aktz":                 "oy.fileReference",     // Aktenzeichen
	"Status":               "oy.status",
	"Version":              "oy.version",
	"LatestVersion":        "oy.latestVersion",

	// Timestamps
	"Created":              "oy.created",
	"LastUpdated":          "oy.lastUpdated",
	"FileCreated":          "oy.fileCreated",
	"Date_Rom":             "oy.dateRom",

	// People
	"Creator":              "oy.creatorId",
	"CreatorFullName":      "oy.creatorName",
	"LastUpdated_User":     "oy.lastUpdatedById",
	"LastUpdated_User_Fullname": "oy.lastUpdatedByName",

	// File info
	"DocName":              "oy.docName",
	"FileExtension":        "oy.fileExtension",
	"FileSize":             "oy.fileSize",

	// Paths & references
	"FullpathString":       "oy.fullPath",
	"ParentFolderId":       "oy.parentFolderId",
	"DocId":                "oy.docId",

	// Document type
	"DocTypeName":          "oy.type.name",
	"DocTypeId":            "oy.type.id",

	// Migration
	"OldDocId":             "oy.oldDocId",
	"OldFolderId":          "oy.oldFolderId",

	// Checkout
	"Checked_Out":          "oy.checkedOut",
	"Checked_Out_User_Id":  "oy.checkedOutById",
	"Checked_Out_User_FullName": "oy.checkedOutByName",
	"Checked_Out_Date":     "oy.checkedOutDate",

	// Deletion
	"DelStatus":            "oy.deleteStatus",
	"DelUserId":            "oy.deletedById",
	"DelDateUser":          "oy.deletedDate",

	// Notes
	"HasNotes":             "oy.hasNotes",
	"Notice":               "note",

	// Misc
	"BelongsTo":            "oy.belongsTo",
	"PrimaryIndexValue":    "oy.primaryIndex",
}

// infoKeyMap maps known legacy DMS DocIndex topic names to English info.* keys.
// User-defined index fields live in the info.* namespace, separate from oy.* system fields.
// Unknown topics pass through as info.<sanitized-topic>.
var infoKeyMap = map[string]string{
	"Ident":                        "info.ident",
	"Aktenzeichen":                 "info.fileReference",
	"Betreff":                      "info.subject",
	"Flurstücksdruckident":         "info.parcelIdent",
	"Bildungsvorschrift":           "info.formationRule",
	"Objektart":                    "info.objectType",
	"Register":                     "info.register",
	"Suchbegriff 1":                "info.searchTerm1",
	"Suchbegriff 2":                "info.searchTerm2",
	"Vertragsbeginn":               "info.contractStart",
	"Vertragsende":                 "info.contractEnd",
	"Absender":                     "info.sender",
	"Absender-E-Mailadresse":       "info.senderEmail",
	"Eingangs-/Versanddatum":       "info.sentDate",
	"Empfänger":                    "info.recipient",
	"Signatur":                     "info.signature",
	"Friedhofsbezeichnung":         "info.cemeteryName",
	"Grabfeldbezeichnung":          "info.graveSectionName",
	"Grabfeldnummer":               "info.graveSectionNumber",
	"Grabreihenbezeichnung":        "info.graveRowName",
	"Grabreihennummer":             "info.graveRowNumber",
	"Objektbezeichnung":            "info.objectName",
	"Objektnummer":                 "info.objectNumber",
	"Kassenzeichen":                "info.cashReference",
	"Bezeichnung":                  "info.designation",
	"Betrag in EUR":                "info.amountEur",
	"Bemerkung":                    "info.remark",
	"Baumname":                     "info.treeName",
	"KZ_DKS":                       "info.dksCode",
}

// mapMetaKey converts a client metadata key to a storage key using the config.
// Delegates to metadata.Config.ToStorageKey — see pkg/metadata/config.go.
// Falls back to package-level defaultMetaCfg if handler config is not available.
var defaultMetaCfg = metadata.DefaultConfig()

func mapMetaKey(clientKey string) string {
	return defaultMetaCfg.ToStorageKey("", clientKey)
}

// mapMetaKeyForUser converts a client key using a specific config and login.
func mapMetaKeyForUser(cfg *metadata.Config, login, clientKey string) string {
	if cfg == nil {
		return mapMetaKey(clientKey)
	}
	return cfg.ToStorageKey(login, clientKey)
}

func taskLogOK() map[string]interface{} {
	return map[string]interface{}{
		"Canceled":  false,
		"hasError":  false,
		"ErrorType": 0,
	}
}
