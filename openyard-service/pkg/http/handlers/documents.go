package handlers

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/metadata"
)

// insecureClient skips TLS verification for internal data gateway calls.
var insecureClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

func withCS3Token(r *http.Request) *http.Request {
	sess := sessionFromCtx(r.Context())
	if sess != nil {
		ctx := metadata.AppendToOutgoingContext(r.Context(), "x-access-token", sess.CS3Token)
		return r.WithContext(ctx)
	}
	return r
}

// POST /api/advancedDocuments/GetDocument
// DocID is a query parameter (legacy DMS format), not in body.
func (h *Handlers) GetDocument(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	// DocID from query (legacy DMS) or ObjectId from body (legacy)
	docID := r.URL.Query().Get("DocID")
	if docID == "" {
		docID = r.URL.Query().Get("ObjectId")
	}
	if docID == "" {
		var body struct {
			ObjectId string `json:"ObjectId"`
			DocId    string `json:"DocId"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		docID = body.DocId
		if docID == "" {
			docID = body.ObjectId
		}
	}
	if docID == "" {
		writeError(w, 400, "INVALID_REQUEST", "DocID required")
		return
	}

	ref, err := refFromObjectID(docID)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", err.Error())
		return
	}

	res, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil {
		log.Error().Err(err).Msg("cs3 stat failed")
		writeError(w, 500, "INTERNAL_ERROR", "Storage error")
		return
	}
	if res.Status.Code == rpc.Code_CODE_NOT_FOUND {
		writeError(w, 404, "NOT_FOUND", "Document not found")
		return
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "INTERNAL_ERROR", res.Status.Message)
		return
	}

	writeJSON(w, 200, mapResourceInfo(res.Info))
}

// POST /api/advancedDocuments/SetDocument
func (h *Handlers) SetDocument(w http.ResponseWriter, r *http.Request) {
	// Spec: Impl-Kategorie reva — CS3 InitiateFileUpload + PUT
	h.SetFile(w, r)
}

// POST /api/advancedDocuments/GetFile
func (h *Handlers) GetFile(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Error().Interface("panic", rec).Msg("GetFile panic")
			writeError(w, 500, "INTERNAL_ERROR", "Panic in GetFile")
		}
	}()
	r = withCS3Token(r)

	var body struct {
		ObjectId string `json:"ObjectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ObjectId == "" {
		writeError(w, 400, "INVALID_REQUEST", "ObjectId required")
		return
	}

	ref, err := refFromObjectID(body.ObjectId)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", err.Error())
		return
	}

	// Use ResourceId + Path="." to point directly at the file node.
	// IsRelativeReference returns true (ResourceId set + Path starts with "."),
	// triggering the "spaces" protocol which embeds the ResourceId in the
	// data-server target URL. The datatx spaces handler resolves the node
	// directly via the OpaqueId — no path traversal needed.
	dlRef := &provider.Reference{ResourceId: ref.ResourceId, Path: "."}

	gw := h.gw.GetGateway()
	res, err := gw.InitiateFileDownload(r.Context(), &provider.InitiateFileDownloadRequest{Ref: dlRef})
	if err != nil && h.gw.CheckError(err) {
		log.Info().Msg("cs3: retrying InitiateFileDownload after reconnect")
		time.Sleep(time.Second)
		gw = h.gw.GetGateway()
		res, err = gw.InitiateFileDownload(r.Context(), &provider.InitiateFileDownloadRequest{Ref: dlRef})
	}
	if err != nil {
		log.Error().Err(err).Msg("cs3 download init failed")
		writeError(w, 500, "INTERNAL_ERROR", "Download error")
		return
	}
	log.Info().Int32("status", int32(res.Status.Code)).Int("protocols", len(res.Protocols)).Msg("cs3 InitiateFileDownload result")
	if res.Status.Code != rpc.Code_CODE_OK {
		log.Error().Str("code", res.Status.Code.String()).Str("msg", res.Status.Message).Msg("cs3 download init rejected")
		writeError(w, 500, "INTERNAL_ERROR", res.Status.Message)
		return
	}

	var downloadToken string
	for _, p := range res.Protocols {
		if p.Protocol == "spaces" || p.Protocol == "simple" {
			downloadToken = p.Token
			break
		}
		if downloadToken == "" {
			downloadToken = p.Token
		}
	}
	if downloadToken == "" {
		writeError(w, 500, "INTERNAL_ERROR", "No download protocol available")
		return
	}

	// Download via OC proxy /data endpoint (DataGatewayMiddleware handles JWT routing).
	// Same pattern as SetFile: use uploadURL (OC proxy) + /data.
	downloadTarget := h.uploadURL + "/data"
	log.Info().Str("target", downloadTarget).Msg("GetFile: download via OC proxy")

	sess := sessionFromCtx(r.Context())
	if sess == nil {
		writeError(w, 401, "AUTH_REQUIRED", "No session")
		return
	}

	downloadReq, err := http.NewRequestWithContext(r.Context(), "GET", downloadTarget, nil)
	if err != nil {
		log.Error().Err(err).Msg("GetFile: create request failed")
		writeError(w, 500, "INTERNAL_ERROR", "Create download request failed")
		return
	}
	downloadReq.Header.Set("X-Access-Token", sess.CS3Token)
	downloadReq.Header.Set("X-Reva-Transfer", downloadToken)

	resp, err := insecureClient.Do(downloadReq)
	if err != nil {
		log.Error().Err(err).Str("url", downloadTarget).Msg("GetFile: download failed")
		writeError(w, 500, "INTERNAL_ERROR", "Download failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		log.Error().Str("body", string(errBody)).Int("status", resp.StatusCode).Str("url", downloadTarget).Msg("GetFile: download error")
		writeError(w, resp.StatusCode, "DOWNLOAD_ERROR", string(errBody))
		return
	}

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	if etag := resp.Header.Get("Etag"); etag != "" {
		w.Header().Set("Etag", etag)
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// POST /api/advancedDocuments/SetFile
func (h *Handlers) SetFile(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	// Parse multipart or read ObjectId from query
	objectID := r.URL.Query().Get("ObjectId")
	if objectID == "" {
		// Try reading from multipart form
		if err := r.ParseMultipartForm(512 << 20); err != nil {
			writeError(w, 400, "INVALID_REQUEST", "Multipart form or ObjectId required")
			return
		}
		objectID = r.FormValue("ObjectId")
	}

	if objectID == "" {
		writeError(w, 400, "INVALID_REQUEST", "ObjectId required")
		return
	}

	ref, err := refFromObjectID(objectID)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", err.Error())
		return
	}

	gw := h.gw.GetGateway()
	res, err := gw.InitiateFileUpload(r.Context(), &provider.InitiateFileUploadRequest{Ref: ref})
	if err != nil && h.gw.CheckError(err) {
		log.Info().Msg("cs3: retrying InitiateFileUpload after reconnect")
		time.Sleep(time.Second)
		gw = h.gw.GetGateway()
		res, err = gw.InitiateFileUpload(r.Context(), &provider.InitiateFileUploadRequest{Ref: ref})
	}
	if err != nil {
		log.Error().Err(err).Msg("cs3 upload init failed")
		writeError(w, 500, "INTERNAL_ERROR", "Upload error")
		return
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "INTERNAL_ERROR", res.Status.Message)
		return
	}

	var uploadToken string
	for _, p := range res.Protocols {
		if p.Protocol == "simple" {
			uploadToken = p.Token
			break
		}
		if uploadToken == "" {
			uploadToken = p.Token
		}
	}
	if uploadToken == "" {
		writeError(w, 500, "INTERNAL_ERROR", "No upload protocol available")
		return
	}

	sess := sessionFromCtx(r.Context())
	if sess == nil {
		writeError(w, 401, "AUTH_REQUIRED", "No session")
		return
	}

	var fileReader io.Reader
	if r.MultipartForm != nil {
		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, 400, "INVALID_REQUEST", "File part required")
			return
		}
		defer file.Close()
		fileReader = file
	} else {
		fileReader = r.Body
	}

	dataURL := h.uploadURL + "/data"
	uploadReq, err := http.NewRequestWithContext(r.Context(), "PUT", dataURL, fileReader)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Create upload request failed")
		return
	}
	uploadReq.Header.Set("X-Access-Token", sess.CS3Token)
	uploadReq.Header.Set("X-Reva-Transfer", uploadToken)

	resp, err := insecureClient.Do(uploadReq)
	if err != nil {
		log.Error().Err(err).Str("url", dataURL).Msg("upload failed")
		writeError(w, 500, "INTERNAL_ERROR", "Upload failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		writeError(w, resp.StatusCode, "INTERNAL_ERROR", "Upload rejected by storage")
		return
	}

	writeJSON(w, 200, map[string]interface{}{"id": objectID, "status": "ok"})
}

// GET /api/advancedDocuments/IsDocument
func (h *Handlers) IsDocument(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	objectID := r.URL.Query().Get("ObjectId")
	if objectID == "" {
		writeError(w, 400, "INVALID_REQUEST", "ObjectId required")
		return
	}

	ref, err := refFromObjectID(objectID)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", err.Error())
		return
	}

	res, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		writeJSON(w, 200, map[string]bool{"isDocument": false})
		return
	}

	writeJSON(w, 200, map[string]bool{
		"isDocument": res.Info.Type == provider.ResourceType_RESOURCE_TYPE_FILE,
	})
}

// POST /api/advancedDocuments/BinDocuments
func (h *Handlers) BinDocuments(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	var body struct {
		ObjectIds []string `json:"ObjectIds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "INVALID_REQUEST", "ObjectIds required")
		return
	}

	for _, oid := range body.ObjectIds {
		ref, err := refFromObjectID(oid)
		if err != nil {
			continue
		}
		if _, err := h.gw.Gateway.Delete(r.Context(), &provider.DeleteRequest{Ref: ref}); err != nil {
			log.Error().Err(err).Str("objectId", oid).Msg("cs3 delete failed")
		}
	}

	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// POST /api/advancedDocuments/GetDeletedFiles
func (h *Handlers) GetDeletedFiles(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	var body struct {
		FolderId string `json:"FolderId"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	ref := &provider.Reference{Path: "/"}
	if body.FolderId != "" {
		if parsed, err := refFromObjectID(body.FolderId); err == nil {
			ref = parsed
		}
	}

	res, err := h.gw.Gateway.ListRecycle(r.Context(), &provider.ListRecycleRequest{Ref: ref})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		writeJSON(w, 200, map[string]interface{}{"items": []interface{}{}, "total": 0})
		return
	}

	items := make([]map[string]interface{}, 0, len(res.RecycleItems))
	for _, item := range res.RecycleItems {
		items = append(items, map[string]interface{}{
			"key":      item.Key,
			"name":     item.Ref.Path,
			"size":     item.Size,
			"deleted":  formatTimestamp(item.DeletionTime),
		})
	}

	writeJSON(w, 200, map[string]interface{}{"items": items, "total": len(items)})
}

// GET /api/advancedDocuments/GetDocumentVersions
func (h *Handlers) GetDocumentVersions(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	docID := r.URL.Query().Get("DocId")
	if docID == "" {
		docID = r.URL.Query().Get("ObjectId")
	}
	if docID == "" {
		writeError(w, 400, "INVALID_REQUEST", "DocId required")
		return
	}

	ref, err := refFromObjectID(docID)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", err.Error())
		return
	}

	res, err := h.gw.Gateway.ListFileVersions(r.Context(), &provider.ListFileVersionsRequest{Ref: ref})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		writeJSON(w, 200, map[string]interface{}{"versions": []interface{}{}})
		return
	}

	versions := make([]map[string]interface{}, 0, len(res.Versions))
	for _, v := range res.Versions {
		versions = append(versions, map[string]interface{}{
			"versionId": v.Key,
			"size":      v.Size,
			"timestamp": time.Unix(int64(v.Mtime), 0).UTC().Format("2006-01-02T15:04:05Z"),
			"etag":      v.Etag,
		})
	}

	writeJSON(w, 200, map[string]interface{}{"versions": versions})
}

// GET /api/advancedDocuments/GetPreviewFile
func (h *Handlers) GetPreviewFile(w http.ResponseWriter, r *http.Request) {
	// Spec: Impl-Kategorie opencloud-framework — Thumbnails-Service
	// Phase 1: Stub, liefert 404
	log.Warn().Msg("GetPreviewFile called — stub, thumbnails not yet integrated")
	writeError(w, 404, "NOT_FOUND", "Preview not available")
}

// GET /api/advancedDocuments/GetMetaData
func (h *Handlers) GetDocumentMetaData(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	objectID := r.URL.Query().Get("ObjectId")
	if objectID == "" {
		writeError(w, 400, "INVALID_REQUEST", "ObjectId required")
		return
	}

	ref, err := refFromObjectID(objectID)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", err.Error())
		return
	}

	res, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{
		Ref:                   ref,
		ArbitraryMetadataKeys: []string{"*"},
	})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "NOT_FOUND", "Resource not found")
		return
	}

	writeJSON(w, 200, mapResourceInfo(res.Info))
}

// extractDownloadTarget decodes a JWT transfer token and returns the internal target URL.
// The target is the internal data server URL (e.g. http://localhost:9158/data/simple/...).
func extractDownloadTarget(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		log.Warn().Msg("extractDownloadTarget: invalid JWT format")
		return ""
	}
	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			log.Warn().Err(err).Msg("extractDownloadTarget: base64 decode failed")
			return ""
		}
	}
	var claims struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		log.Warn().Err(err).Msg("extractDownloadTarget: JSON parse failed")
		return ""
	}
	return claims.Target
}

// DLTokenDebug is a temporary debug endpoint to inspect download tokens.
// POST /openyard/debug/dl-token with {"ObjectId": "..."}
func (h *Handlers) DLTokenDebug(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	var body struct {
		ObjectId string `json:"ObjectId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ObjectId == "" {
		writeError(w, 400, "INVALID_REQUEST", "ObjectId required")
		return
	}

	ref, err := refFromObjectID(body.ObjectId)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", err.Error())
		return
	}

	gw := h.gw.GetGateway()
	res, err := gw.InitiateFileDownload(r.Context(), &provider.InitiateFileDownloadRequest{Ref: ref})
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", err.Error())
		return
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "INTERNAL_ERROR", res.Status.Message)
		return
	}

	for _, p := range res.Protocols {
		target := extractDownloadTarget(p.Token)
		log.Info().Str("protocol", p.Protocol).Str("endpoint", p.DownloadEndpoint).Str("target", target).Str("token_len", fmt.Sprintf("%d", len(p.Token))).Msg("DLTokenDebug")
		writeJSON(w, 200, map[string]interface{}{
			"protocol": p.Protocol,
			"endpoint": p.DownloadEndpoint,
			"target":   target,
			"token":    p.Token,
		})
		return
	}
	writeJSON(w, 200, map[string]string{"error": "no protocols"})
}
