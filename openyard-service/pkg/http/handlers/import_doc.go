package handlers

import (
	"encoding/base64"
	"encoding/xml"
	"io"
	"net/http"

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

// POST /api/basicDocuments/ImportDocumentDynamic
//
// Query: ?SessionID=...&ConfigName=...&FileName=...&OldId=...&SpaceId=...
// Body: application/xml with ImportDocDataDynamic (WCF DataContract)
// Response: "guid-of-new-document" (JSON string)
//
// OldId: optional WinYard document ID. If provided, the migration DB is
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

	// Parse XML body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", "Cannot read body")
		return
	}

	var importData ImportDocDataDynamic
	if err := xml.Unmarshal(bodyBytes, &importData); err != nil {
		writeError(w, 400, "INVALID_REQUEST", "Invalid XML body")
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
		log.Error().Err(err).Str("file", fileName).Str("method", h.uploader.Name()).Msg("upload failed")
		writeError(w, 500, "INTERNAL_ERROR", "Upload failed: "+err.Error())
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
		md := make(map[string]string)
		for _, e := range importData.DocIndex.Entries {
			md[mapMetaKey(e.Key)] = e.Value
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

	log.Info().Str("file", fileName).Int("size", len(fileData)).Str("id", newID).Str("method", h.uploader.Name()).Msg("document imported")

	// Response: GUID as JSON string (WinYard format)
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
