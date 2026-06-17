package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/kosmos-eu/openyard/pkg/migration"
)

// GET /api/advancedGeneral/IsListening
func (h *Handlers) IsListening(w http.ResponseWriter, r *http.Request) {
	// WinYard returns plain "true" (not JSON object)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("true"))
}

// GET /api/advancedGeneral/GetServerSettings
// WinYard-compatible response with Settings object and TaskLog.
func (h *Handlers) GetServerSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"Settings": map[string]interface{}{
			"ServerVersion":       "1.1.0",
			"ApiVersion":          "Core 1.1",
			"ProductName":         "OpenYard DMS",
			"MaxUploadSizeMB":     512,
			"SessionTimeoutMin":   480,
			"DefaultLanguage":     "de",
			"FullTextSearchEnabled": false,
			"WebDAVEnabled":       false,
			"WorkflowEnabled":    false,
			"PortalEnabled":      false,
			"DsgvoEnabled":       false,
			"ArchiveEnabled":     true,
			"IndexEnabled":       false,
		},
		"TaskLog": map[string]interface{}{
			"Canceled":  false,
			"hasError":  false,
			"ErrorType": 0,
		},
	})
}

// GET /api/ApiExplorer/GetApiDescriptions
func (h *Handlers) GetApiDescriptions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]string{
			"title":   "OpenYard Cloud API",
			"version": "1.1.0",
		},
	})
}

// GET /api/basicGeneral/GetEnumerationAsDictionary
func (h *Handlers) GetEnumerationAsDictionary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"enumerations": map[string]interface{}{},
	})
}

// GET /api/basicConfig/GetOpenyardDMSUserSettings
func (h *Handlers) GetDMSUserSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"settings": map[string]interface{}{},
	})
}

// GetAppConfigs is implemented in appconfig.go

// POST /api/advancedDocuments/GetDocTypes
// Returns available document types.
func (h *Handlers) GetDocTypes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, []interface{}{})
}

// GET /api/advancedCommon/GetSearchTopics
// Returns available search topic categories.
func (h *Handlers) GetSearchTopics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, []interface{}{})
}

// GET /api/basicDocuments/HasPreviewFile
func (h *Handlers) HasPreviewFile(w http.ResponseWriter, r *http.Request) {
	// Check if preview exists for given document
	writeJSON(w, 200, map[string]interface{}{
		"HasPreview": false,
		"TaskLog": map[string]interface{}{
			"Canceled":  false,
			"hasError":  false,
			"ErrorType": 0,
		},
	})
}

// GET /api/basicDocuments/DownloadPreviewFile
func (h *Handlers) DownloadPreviewFile(w http.ResponseWriter, r *http.Request) {
	writeError(w, 404, "NOT_FOUND", "Preview not available")
}

// GET /api/advancedGeneral/GetMigrationStatus
// Returns migration DB status (total entries, mapped entries, dirty flag).
func (h *Handlers) GetMigrationStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"total":  migration.Count(),
		"mapped": migration.MappedCount(),
		"dirty":  migration.IsDirty(),
	})
}

// POST /api/advancedGeneral/PersistMigration
// Writes the in-memory migration DB to disk.
func (h *Handlers) PersistMigration(w http.ResponseWriter, r *http.Request) {
	if err := migration.Persist(); err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Persist failed: "+err.Error())
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"status": "ok",
		"total":  migration.Count(),
		"mapped": migration.MappedCount(),
	})
}

// POST /api/management/migration/map
// Maps an old WinYard ID to a new OpenYard ID in memory.
// Body: {"OldId": "...", "NewId": "...", "Type": "folder|document", "Name": "..."}
func (h *Handlers) MapMigrationID(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OldId string `json:"OldId"`
		NewId string `json:"NewId"`
		Type  string `json:"Type"`
		Name  string `json:"Name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.OldId == "" || body.NewId == "" {
		writeError(w, 400, "INVALID_REQUEST", "OldId and NewId required")
		return
	}
	if body.Type == "" {
		body.Type = "unknown"
	}
	migration.MapID(body.OldId, body.NewId, body.Type, body.Name)
	writeJSON(w, 200, map[string]interface{}{
		"status": "ok",
		"oldId":  body.OldId,
		"newId":  body.NewId,
	})
}

// ImportDocumentDynamic is implemented in import_doc.go

// writeJSONRaw writes pre-encoded JSON (for responses that need exact format).
func writeJSONRaw(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(data)
}

// taskLogWrap wraps a response with a TaskLog envelope (common WinYard pattern).
func taskLogWrap(data interface{}) map[string]interface{} {
	b, _ := json.Marshal(data)
	var result map[string]interface{}
	json.Unmarshal(b, &result)
	if result == nil {
		result = map[string]interface{}{}
	}
	result["TaskLog"] = map[string]interface{}{
		"Canceled":  false,
		"hasError":  false,
		"ErrorType": 0,
	}
	return result
}
