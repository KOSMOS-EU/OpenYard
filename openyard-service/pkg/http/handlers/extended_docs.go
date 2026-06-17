package handlers

import (
	"encoding/json"
	"net/http"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/rs/zerolog/log"
)

// POST /api/basicDocuments/ImportDocumentByParentFolderID
func (h *Handlers) ImportDocumentByParentFolderID(w http.ResponseWriter, r *http.Request) {
	// Spec: reva — wie SetFile mit ObjectId-Auflösung
	h.SetFile(w, r)
}

// POST /api/basicDocuments/ImportDocumentByParentFolderPath
func (h *Handlers) ImportDocumentByParentFolderPath(w http.ResponseWriter, r *http.Request) {
	// Spec: reva — wie SetFile mit Pfad-Anlage
	h.SetFile(w, r)
}

// POST /api/basicDocuments/RenameDocument
func (h *Handlers) RenameDocument(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	var body struct {
		ObjectId string `json:"ObjectId"`
		NewName  string `json:"NewName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ObjectId == "" || body.NewName == "" {
		writeError(w, 400, "INVALID_REQUEST", "ObjectId and NewName required")
		return
	}

	srcRef, err := refFromObjectID(body.ObjectId)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", err.Error())
		return
	}

	statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: srcRef})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "NOT_FOUND", "Document not found")
		return
	}

	p := statRes.Info.Path
	parent := p[:len(p)-len(path_base(p))]
	dstRef := refFromPath(parent + body.NewName)

	res, err := h.gw.Gateway.Move(r.Context(), &provider.MoveRequest{Source: srcRef, Destination: dstRef})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "INTERNAL_ERROR", "Rename failed")
		return
	}

	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// POST /api/basicDocuments/UploadFile
func (h *Handlers) UploadFile(w http.ResponseWriter, r *http.Request) {
	// Spec: reva — wie SetFile mit zusätzlichen Flags
	h.SetFile(w, r)
}

// POST /api/basicDocuments/CheckInAndIgnoreChanges
func (h *Handlers) CheckInAndIgnoreChanges(w http.ResponseWriter, r *http.Request) {
	// Spec: archaeologie-wrapper — OpenYard hat kein Locking. Stub gibt 200 OK.
	log.Warn().Msg("CheckInAndIgnoreChanges called — archaeologie-wrapper stub")
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// POST /api/basicDocuments/CreateNewDocVersion
func (h *Handlers) CreateNewDocVersion(w http.ResponseWriter, r *http.Request) {
	// Spec: archaeologie-wrapper — funktional identisch zu SetFile
	log.Warn().Msg("CreateNewDocVersion called — pass-through to SetFile")
	h.SetFile(w, r)
}
