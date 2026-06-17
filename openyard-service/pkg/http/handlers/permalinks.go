package handlers

import (
	"net/http"

	"github.com/rs/zerolog/log"
)

// GET /api/basicCommon/GetExistingPermalinksByObjId
func (h *Handlers) GetExistingPermalinksByObjId(w http.ResponseWriter, r *http.Request) {
	// Spec: opencloud-framework + eigen — OCS Public Links
	// Phase 1: Stub
	log.Warn().Msg("GetExistingPermalinksByObjId called — stub")
	writeJSON(w, 200, map[string]interface{}{"permalinks": []interface{}{}})
}

// GET /api/basicCommon/DeletePermalink
func (h *Handlers) DeletePermalink(w http.ResponseWriter, r *http.Request) {
	log.Warn().Msg("DeletePermalink called — stub")
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// GET /api/basicCommon/GetObjectIdByPermalink
func (h *Handlers) GetObjectIdByPermalink(w http.ResponseWriter, r *http.Request) {
	log.Warn().Msg("GetObjectIdByPermalink called — stub")
	writeError(w, 404, "NOT_FOUND", "Permalink not found")
}

// GET /api/basicCommon/GetPreviewFileByPermalink
func (h *Handlers) GetPreviewFileByPermalink(w http.ResponseWriter, r *http.Request) {
	log.Warn().Msg("GetPreviewFileByPermalink called — stub")
	writeError(w, 404, "NOT_FOUND", "Preview not available")
}
