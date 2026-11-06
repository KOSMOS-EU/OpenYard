package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/kosmos-eu/openyard/pkg/metadata"
)

// metaConfig returns the handler's metadata config or the built-in default.
func (h *Handlers) metaConfig() *metadata.Config {
	if h.metaCfg != nil {
		return h.metaCfg
	}
	return defaultMetaCfg
}

// isGUID reports whether s has the GUID layout xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.
func isGUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}

// PUT /api/basicDocuments/SetDocIndex?SessionID=...&DocId=...&DocType=...
//
// Body: JSON object {"<topic>": "<value>", ...} (WinYard DocIndex).
// Stores the topics as metadata (namespace mapping, shared keys pass
// through) plus source / doc.type / original doc type from DocType.
// Response: true (JSON bool).
func (h *Handlers) SetDocIndex(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	docID := r.URL.Query().Get("DocId")
	if docID == "" {
		writeError(w, 400, "INVALID_REQUEST", "DocId required")
		return
	}
	var docIndex map[string]string
	if err := json.NewDecoder(r.Body).Decode(&docIndex); err != nil {
		writeError(w, 400, "INVALID_REQUEST", "DocIndex JSON object required")
		return
	}
	resourceID, err := h.resourceIDFromID(r.Context(), docID)
	if err != nil {
		documentNotFound(w, docID)
		return
	}

	login := ""
	if sess := sessionFromCtx(r.Context()); sess != nil {
		login = sess.Login
	}
	if err := h.storeDocIndex(r, login, resourceID, r.URL.Query().Get("DocType"), docIndex); err != nil {
		log.Error().Err(err).Str("docId", docID).Msg("SetDocIndex: set metadata failed")
		writeError(w, 500, "INTERNAL_ERROR", "Set metadata failed")
		return
	}
	writeJSON(w, 200, true)
}
