package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"sort"
	"strings"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/rs/zerolog/log"

	"github.com/kosmos-eu/openyard/pkg/migration"
	"github.com/kosmos-eu/openyard/pkg/upload"
)

// ImportDocDataDynamic is the WCF DataContract XML body.
type ImportDocDataDynamic struct {
	XMLName     xml.Name     `xml:"ImportDocDataDynamic"`
	DocIndex    DocIndexList `xml:"DocIndex"`
	FileContent string       `xml:"FileContent"`
}

type DocIndexList struct {
	Entries []DocIndexEntry `xml:"KeyValueOfstringstring"`
}

type DocIndexEntry struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// importDocDataJSON is the same body as the NSwag-generated WinYard client
// (Saskia SIS.Global.Dms, Newtonsoft.Json) sends it: application/json with
// FileContent as base64 string and DocIndex as a flat string dictionary.
type importDocDataJSON struct {
	DocIndex    map[string]string `json:"DocIndex"`
	FileContent string            `json:"FileContent"`
}

// parseImportDocData accepts the WCF DataContract XML body and the JSON body of
// the .NET WinYard client. The content type decides; without one, a body
// starting with "{" is JSON.
func parseImportDocData(contentType string, body []byte) (ImportDocDataDynamic, error) {
	trimmed := bytes.TrimSpace(body)
	isJSON := strings.Contains(strings.ToLower(contentType), "json") ||
		(!strings.Contains(strings.ToLower(contentType), "xml") && bytes.HasPrefix(trimmed, []byte("{")))
	if !isJSON {
		var importData ImportDocDataDynamic
		err := xml.Unmarshal(body, &importData)
		return importData, err
	}
	var jsonData importDocDataJSON
	if err := json.Unmarshal(trimmed, &jsonData); err != nil {
		return ImportDocDataDynamic{}, err
	}
	keys := make([]string, 0, len(jsonData.DocIndex))
	for key := range jsonData.DocIndex {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	importData := ImportDocDataDynamic{FileContent: jsonData.FileContent}
	for _, key := range keys {
		importData.DocIndex.Entries = append(importData.DocIndex.Entries, DocIndexEntry{Key: key, Value: jsonData.DocIndex[key]})
	}
	return importData, nil
}

// POST /api/basicDocuments/ImportDocumentDynamic
//
// Query: ?SessionID=...&ConfigName=...&FileName=...&OldId=...&SpaceId=...
// Body: application/xml with ImportDocDataDynamic (WCF DataContract)
// Response: "guid-of-new-document" (JSON string)
//
// OldId: optional legacy DMS document ID. If provided, the migration DB is
//
//	updated to map OldId → new OpenYard ID, enabling future lookups.
//
// SpaceId: optional target space (base64-encoded ObjectID of the space root).
//
//	If omitted, the uploader picks personal or first available space.
func (h *Handlers) ImportDocumentDynamic(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)
	sess := sessionFromCtx(r.Context())

	fileName := r.URL.Query().Get("FileName")
	oldID := r.URL.Query().Get("OldId")
	spaceID := r.URL.Query().Get("SpaceId")
	folderID := r.URL.Query().Get("FolderId")

	// Parse body: WCF XML (migration tools) or JSON (.NET WinYard client, Saskia)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", "Cannot read body")
		return
	}

	importData, err := parseImportDocData(r.Header.Get("Content-Type"), bodyBytes)
	if err != nil {
		log.Warn().Err(err).Str("contentType", r.Header.Get("Content-Type")).Int("bodyLen", len(bodyBytes)).
			Msg("ImportDocumentDynamic: body not parseable")
		writeError(w, 400, "INVALID_REQUEST", "Invalid body: expected ImportDocDataDynamic as XML or JSON")
		return
	}

	// Decode file content (base64)
	fileData, err := base64.StdEncoding.DecodeString(importData.FileContent)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", "Invalid base64 in FileContent")
		return
	}

	if fileName == "" {
		for _, e := range importData.DocIndex.Entries {
			if e.Key == "Betreff" && e.Value != "" {
				fileName = e.Value
				break
			}
		}
	}
	if fileName == "" {
		fileName = "imported_document"
	}

	// Upload via pluggable uploader (webdav-weg or reva-weg)
	uploadReq := &upload.Request{
		FileName: escapeFolderName(fileName),
		Data:     fileData,
		SpaceID:  spaceID,
		FolderID: folderID,
	}
	if sess != nil {
		uploadReq.Username = sess.Login
		uploadReq.Password = sess.Password
		uploadReq.CS3Token = sess.CS3Token
	}

	result, err := h.uploader.Upload(r.Context(), uploadReq)
	if err != nil {
		errMsg := err.Error()
		httpCode := 500
		errCode := "INTERNAL_ERROR"
		switch {
		case strings.Contains(errMsg, "409"):
			httpCode = 409
			errCode = "ALREADY_EXISTS"
		case strings.Contains(errMsg, "412"):
			httpCode = 409
			errCode = "ALREADY_EXISTS"
		case strings.Contains(errMsg, "connection is closing"),
			strings.Contains(errMsg, "transport is closing"):
			httpCode = 503
			errCode = "SERVICE_UNAVAILABLE"
		case strings.Contains(errMsg, "permission denied"),
			strings.Contains(errMsg, "PERMISSION_DENIED"):
			httpCode = 403
			errCode = "PERMISSION_DENIED"
		}
		log.Error().Err(err).Str("file", fileName).Str("method", h.uploader.Name()).Msg("upload failed")
		writeError(w, httpCode, errCode, "Upload failed: "+errMsg)
		return
	}

	// Build encoded ObjectID from Oc-Fileid (storageId$spaceId!opaqueId)
	// This is the same format that encodeObjectID produces, so GetDocument can decode it.
	newID := encodeObjectIDFromFileID(result.FileID)

	// Fallback: Stat via CS3 to get the canonical ResourceId
	if newID == "" && h.gw != nil && h.gw.Gateway != nil {
		spacesRes, serr := h.gw.Gateway.ListStorageSpaces(r.Context(), &provider.ListStorageSpacesRequest{})
		if serr == nil && spacesRes.Status.Code == rpc.Code_CODE_OK {
			for _, space := range spacesRes.StorageSpaces {
				if space.SpaceType == "personal" && space.Root != nil {
					ref := &provider.Reference{
						ResourceId: space.Root,
						Path:       "./" + escapeFolderName(fileName),
					}
					statRes, serr := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
					if serr == nil && statRes.Status.Code == rpc.Code_CODE_OK && statRes.Info.Id != nil {
						newID = encodeObjectID(statRes.Info.Id)
					}
					break
				}
			}
		}
	}
	if newID == "" {
		newID = "00000000-0000-0000-0000-000000000000"
	}

	// Store DocIndex as metadata via CS3 (using the uploaded file's reference)
	if h.gw != nil && h.gw.Gateway != nil && newID != "" && newID != "00000000-0000-0000-0000-000000000000" {
		login := ""
		if sess != nil {
			login = sess.Login
		}
		md := h.metaConfig().DocumentMetadata(login, r.URL.Query().Get("ConfigName"))
		for _, e := range importData.DocIndex.Entries {
			storageKey := mapMetaKeyForUser(h.metaCfg, login, e.Key)
			md[storageKey] = h.metaConfig().ToStorageValue(login, storageKey, e.Value)
		}
		if len(md) > 0 {
			if ref, err := refFromObjectID(newID); err == nil {
				h.gw.Gateway.SetArbitraryMetadata(r.Context(), &provider.SetArbitraryMetadataRequest{
					Ref:               ref,
					ArbitraryMetadata: &provider.ArbitraryMetadata{Metadata: md},
				})
			}
		}
	}

	// If OldId provided, update migration DB: OldId → newID
	if oldID != "" && newID != "" {
		migration.MapID(oldID, newID, "document", fileName)
		log.Info().Str("oldId", oldID).Str("newId", newID[:20]+"...").Msg("migration mapping created")
	}

	// WinYard clients get a GUID (legacy ID or OpenCloud opaque ID)
	if resourceID, err := decodeObjectID(newID); err == nil && newID != nilGUID {
		newID = clientID(resourceID)
	}

	log.Info().Str("file", fileName).Int("size", len(fileData)).Str("id", newID).Str("method", h.uploader.Name()).Msg("document imported")

	// Response: GUID as JSON string (legacy DMS format)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte(`"` + newID + `"`))
}

// encodeObjectIDFromFileID converts an Oc-Fileid header (storageId$spaceId!opaqueId)
// to a base64-encoded ObjectID that decodeObjectID can parse.
func encodeObjectIDFromFileID(fileID string) string {
	if fileID == "" {
		return ""
	}
	// Oc-Fileid is already in the CS3 triple format: storageId$spaceId!opaqueId
	// encodeObjectID expects a ResourceId, but the raw string IS what we base64-encode
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(fileID))
}
