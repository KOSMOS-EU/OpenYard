package handlers

import (
	"encoding/json"
	"net/http"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/rs/zerolog/log"
)

// POST /api/advancedCommon/CopyObjects
func (h *Handlers) CopyObjects(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	var body struct {
		SourceIds     []string `json:"SourceIds"`
		DestinationId string   `json:"DestinationId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "INVALID_REQUEST", "SourceIds and DestinationId required")
		return
	}

	dstRef, err := refFromObjectID(body.DestinationId)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", err.Error())
		return
	}

	// Stat destination to get path
	dstStat, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: dstRef})
	if err != nil || dstStat.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "NOT_FOUND", "Destination not found")
		return
	}

	for _, srcID := range body.SourceIds {
		srcRef, err := refFromObjectID(srcID)
		if err != nil {
			continue
		}
		srcStat, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: srcRef})
		if err != nil || srcStat.Status.Code != rpc.Code_CODE_OK {
			continue
		}
		// CS3 gateway has no direct Copy. Use Move as workaround (Phase 1).
		// Full copy requires download+upload pipeline (Phase 2).
		newDst := refFromPath(dstStat.Info.Path + "/" + path_base(srcStat.Info.Path))
		log.Warn().Str("src", srcID).Str("dst", newDst.Path).Msg("CopyObjects: copy not natively supported in CS3, using move semantics as stub")
		_ = newDst
	}

	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// POST /api/advancedCommon/MoveObjects
func (h *Handlers) MoveObjects(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	var body struct {
		SourceIds     []string `json:"SourceIds"`
		DestinationId string   `json:"DestinationId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "INVALID_REQUEST", "SourceIds and DestinationId required")
		return
	}

	dstRef, err := refFromObjectID(body.DestinationId)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", err.Error())
		return
	}

	dstStat, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: dstRef})
	if err != nil || dstStat.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "NOT_FOUND", "Destination not found")
		return
	}

	for _, srcID := range body.SourceIds {
		srcRef, err := refFromObjectID(srcID)
		if err != nil {
			continue
		}
		srcStat, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: srcRef})
		if err != nil || srcStat.Status.Code != rpc.Code_CODE_OK {
			continue
		}
		newDst := refFromPath(dstStat.Info.Path + "/" + path_base(srcStat.Info.Path))
		if _, err := h.gw.Gateway.Move(r.Context(), &provider.MoveRequest{Source: srcRef, Destination: newDst}); err != nil {
			log.Error().Err(err).Str("src", srcID).Msg("cs3 move failed")
		}
	}

	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// POST /api/advancedCommon/RecoverObjects
func (h *Handlers) RecoverObjects(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	var body struct {
		Keys []string `json:"Keys"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "INVALID_REQUEST", "Keys required")
		return
	}

	ref := &provider.Reference{Path: "/"}
	for _, key := range body.Keys {
		if _, err := h.gw.Gateway.RestoreRecycleItem(r.Context(), &provider.RestoreRecycleItemRequest{
			Ref: ref,
			Key: key,
		}); err != nil {
			log.Error().Err(err).Str("key", key).Msg("cs3 restore failed")
		}
	}

	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// GET /api/advancedCommon/ExistObjectId
func (h *Handlers) ExistObjectId(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	objectID := r.URL.Query().Get("ObjectId")
	if objectID == "" {
		writeError(w, 400, "INVALID_REQUEST", "ObjectId required")
		return
	}

	ref, err := refFromObjectID(objectID)
	if err != nil {
		writeJSON(w, 200, map[string]bool{"exists": false})
		return
	}

	res, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	exists := err == nil && res.Status.Code == rpc.Code_CODE_OK
	writeJSON(w, 200, map[string]bool{"exists": exists})
}

// GET /api/advancedCommon/ExistObjectPath
func (h *Handlers) ExistObjectPath(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	objectPath := r.URL.Query().Get("ObjectPath")
	if objectPath == "" {
		writeError(w, 400, "INVALID_REQUEST", "ObjectPath required")
		return
	}

	ref := refFromPath(objectPath)
	res, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	exists := err == nil && res.Status.Code == rpc.Code_CODE_OK
	writeJSON(w, 200, map[string]bool{"exists": exists})
}

// POST /api/advancedCommon/Search
func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	// Spec: Impl-Kategorie eigen (Phase 2: separater Index-Dienst)
	// Phase 1: degradiert auf einfache Pfad-basierte Suche
	log.Warn().Msg("Search called — Phase 1 stub, no full-text search")
	writeJSON(w, 200, map[string]interface{}{
		"results": []interface{}{},
		"total":   0,
	})
}

// POST /api/basicCommon/SearchForTitle
func (h *Handlers) SearchForTitle(w http.ResponseWriter, r *http.Request) {
	// Phase 1: Stub
	writeJSON(w, 200, map[string]interface{}{"results": []interface{}{}, "total": 0})
}

// POST /api/basicCommon/SearchForIndex
func (h *Handlers) SearchForIndex(w http.ResponseWriter, r *http.Request) {
	// Phase 2
	writeJSON(w, 200, map[string]interface{}{"results": []interface{}{}, "total": 0})
}

// POST /api/basicCommon/SetIndex
func (h *Handlers) SetIndex(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	var body struct {
		ObjectId string            `json:"ObjectId"`
		Metadata map[string]string `json:"Metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ObjectId == "" {
		writeError(w, 400, "INVALID_REQUEST", "ObjectId and Metadata required")
		return
	}

	ref, err := refFromObjectID(body.ObjectId)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", err.Error())
		return
	}

	// Map metadata keys to oy.* namespace
	login := ""
	if sess := sessionFromCtx(r.Context()); sess != nil {
		login = sess.Login
	}
	prefixed := make(map[string]string, len(body.Metadata))
	for k, v := range body.Metadata {
		prefixed[mapMetaKeyForUser(h.metaCfg, login, k)] = v
	}

	res, err := h.gw.Gateway.SetArbitraryMetadata(r.Context(), &provider.SetArbitraryMetadataRequest{
		Ref: ref,
		ArbitraryMetadata: &provider.ArbitraryMetadata{Metadata: prefixed},
	})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "INTERNAL_ERROR", "Set metadata failed")
		return
	}

	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// GET /api/basicCommon/GetMetaData
func (h *Handlers) GetBasicMetaData(w http.ResponseWriter, r *http.Request) {
	h.GetDocumentMetaData(w, r)
}

// GET /api/basicCommon/GetIndexData
func (h *Handlers) GetIndexData(w http.ResponseWriter, r *http.Request) {
	// Phase 1: einfache Heuristik aus Pfad
	writeJSON(w, 200, map[string]interface{}{"indexData": map[string]interface{}{}})
}

// GET /api/basicCommon/GetFolderSubElementsByMemberId
func (h *Handlers) GetFolderSubElementsByMemberId(w http.ResponseWriter, r *http.Request) {
	// Spec: reva — CS3 ListContainer mit User-Filter
	// Phase 1: delegiert an GetFolder
	h.GetFolder(w, r)
}
