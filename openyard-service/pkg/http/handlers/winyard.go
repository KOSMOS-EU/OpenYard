package handlers

// WinYard wire format shared by the legacy DMS API handlers.
//
// IDs: clients (Saskia, Archikart) parse document and folder IDs as GUID.
// OpenYard hands out the OpenCloud opaque ID (a UUID, stable across moves
// within a space). Migrated documents keep their legacy WinYard GUID via
// the migration DB. Incoming IDs are resolved in this order: migration DB,
// base64 ObjectID, opaque ID probed in the user's spaces (cached).
//
// FileKeys: byte arrays the client passes back verbatim (JSON base64).
//   "oyfk1|<sessionID>|<objectID>" — download key (GetDocument, GetFolder)
//   "oyup1|<sessionID>|<uploadID>" — uploaded file (UploadFile)

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	typesv1 "github.com/cs3org/go-cs3apis/cs3/types/v1beta1"
	gocache "github.com/patrickmn/go-cache"
	"github.com/rs/zerolog/log"

	"github.com/kosmos-eu/openyard/pkg/auth"
	"github.com/kosmos-eu/openyard/pkg/migration"
)

// nilGUID is the WinYard "no object" ID (also: root folder, search everywhere).
const nilGUID = "00000000-0000-0000-0000-000000000000"

const (
	downloadKeyPrefix = "oyfk1"
	uploadKeyPrefix   = "oyup1"
)

var errObjectNotFound = errors.New("object not found")

// guidResourceIDs caches opaque ID → full ResourceId found by probing spaces.
// Opaque IDs do not change on move/rename within a space; a document moved
// to another space gets a new ID, so stale entries only point to deleted nodes.
var guidResourceIDs = gocache.New(24*time.Hour, time.Hour)

// pendingUploads holds files from UploadFile until SetDocument/UpdateDocument
// picks them up. Evicted files are removed from disk.
var pendingUploads = newUploadStore(2 * time.Hour)

// clientID returns the WinYard ID handed to clients for a resource: the
// legacy GUID for migrated objects, otherwise the OpenCloud opaque ID.
// Falls back to the base64 ObjectID if the opaque ID is no UUID.
func clientID(resourceID *provider.ResourceId) string {
	if resourceID == nil {
		return nilGUID
	}
	objectID := encodeObjectID(resourceID)
	if legacyID := migration.LookupLegacyID(objectID); legacyID != "" {
		return legacyID
	}
	if isGUID(resourceID.OpaqueId) {
		return strings.ToLower(resourceID.OpaqueId)
	}
	return objectID
}

// resourceIDFromID resolves a client ID (legacy GUID, opaque ID or base64
// ObjectID) to a CS3 ResourceId. ctx must carry the user's CS3 token.
func (h *Handlers) resourceIDFromID(ctx context.Context, id string) (*provider.ResourceId, error) {
	id = strings.Trim(strings.TrimSpace(id), "{}")
	if !isGUID(id) {
		return decodeObjectID(id)
	}
	if objectID := migration.LookupOpenYardID(id); objectID != "" {
		return decodeObjectID(objectID)
	}
	opaqueID := strings.ToLower(id)
	if cached, ok := guidResourceIDs.Get(opaqueID); ok {
		return cached.(*provider.ResourceId), nil
	}
	if h.gw == nil || h.gw.Gateway == nil {
		return nil, errObjectNotFound
	}
	spacesRes, err := h.gw.Gateway.ListStorageSpaces(ctx, &provider.ListStorageSpacesRequest{})
	if err != nil {
		return nil, err
	}
	if spacesRes.Status.Code != rpc.Code_CODE_OK {
		return nil, errors.New(spacesRes.Status.Message)
	}
	for _, space := range spacesRes.StorageSpaces {
		if space.Root == nil {
			continue
		}
		candidate := &provider.ResourceId{
			StorageId: space.Root.StorageId,
			SpaceId:   space.Root.SpaceId,
			OpaqueId:  opaqueID,
		}
		statRes, statErr := h.gw.Gateway.Stat(ctx, &provider.StatRequest{Ref: &provider.Reference{ResourceId: candidate}})
		if statErr != nil || statRes.Status.Code != rpc.Code_CODE_OK {
			continue
		}
		guidResourceIDs.SetDefault(opaqueID, candidate)
		return candidate, nil
	}
	return nil, errObjectNotFound
}

// refFromID resolves a client ID to a CS3 Reference (see resourceIDFromID).
func (h *Handlers) refFromID(ctx context.Context, id string) (*provider.Reference, error) {
	resourceID, err := h.resourceIDFromID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &provider.Reference{ResourceId: resourceID}, nil
}

// downloadFileKey builds the FileKey GetFile accepts without SessionID.
func downloadFileKey(sessionID string, resourceID *provider.ResourceId) []byte {
	if sessionID == "" || resourceID == nil {
		return nil
	}
	return []byte(downloadKeyPrefix + "|" + sessionID + "|" + encodeObjectID(resourceID))
}

// parseFileKey splits a FileKey into kind prefix, session ID and payload.
func parseFileKey(fileKey []byte) (kind, sessionID, payload string, ok bool) {
	parts := strings.SplitN(string(fileKey), "|", 3)
	if len(parts) != 3 || parts[1] == "" || parts[2] == "" {
		return "", "", "", false
	}
	if parts[0] != downloadKeyPrefix && parts[0] != uploadKeyPrefix {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// sessionIDFromRequest returns the SessionID query parameter (any case).
func sessionIDFromRequest(r *http.Request) string {
	for key, values := range r.URL.Query() {
		if strings.EqualFold(key, "SessionID") && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// queryValue returns a query parameter, matching the name case-insensitively
// (WinYard clients mix DocID/DocId/docId).
func queryValue(r *http.Request, name string) string {
	if value := r.URL.Query().Get(name); value != "" {
		return value
	}
	for key, values := range r.URL.Query() {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// queryBool parses a .NET bool query value ("True"/"true"/"1"); def if absent.
func queryBool(r *http.Request, name string, def bool) bool {
	switch strings.ToLower(queryValue(r, name)) {
	case "true", "1":
		return true
	case "false", "0":
		return false
	}
	return def
}

// splitDocName splits a storage file name into WinYard DocName and
// FileExtension (without dot): "Beleg 1.pdf" → "Beleg 1", "pdf".
func splitDocName(fileName string) (docName, extension string) {
	fileName = unescapeFolderName(fileName)
	ext := path.Ext(fileName)
	if ext == "" || ext == fileName {
		return fileName, ""
	}
	return strings.TrimSuffix(fileName, ext), strings.TrimPrefix(ext, ".")
}

// winyardTime formats a CS3 timestamp as .NET DateTime (UTC).
func winyardTime(info *provider.ResourceInfo) string {
	if info == nil || info.Mtime == nil {
		return ""
	}
	return formatTimestamp(info.Mtime)
}

// fullRights is the WinYard Rights object; OpenCloud enforces the real
// permissions on each CS3 call.
func fullRights() map[string]interface{} {
	return map[string]interface{}{
		"RightsDigit":              4095,
		"FolderCreate":             true,
		"FolderDelete":             true,
		"FolderChange":             true,
		"DocumentsRead":            true,
		"DocumentsCreateAndChange": true,
		"DocumentsRename":          true,
		"DocumentsDelete":          true,
		"FolderSetRights":          true,
	}
}

// docIndexFromMetadata converts stored metadata into the WinYard DocIndex
// list [{"topic": ..., "values": {"StringValue": ...}}], topics in client
// notation, sorted, each topic once (clients build a dictionary from it).
func (h *Handlers) docIndexFromMetadata(info *provider.ResourceInfo, login string) []map[string]interface{} {
	docIndex := make([]map[string]interface{}, 0)
	if info == nil {
		return docIndex
	}
	cfg := h.metaConfig()
	storedMeta := cfg.WithDefaults(login, info.GetArbitraryMetadata().GetMetadata())
	storageKeys := make([]string, 0, len(storedMeta))
	for storageKey := range storedMeta {
		storageKeys = append(storageKeys, storageKey)
	}
	sort.Strings(storageKeys)
	seenTopics := map[string]bool{}
	for _, storageKey := range storageKeys {
		topic := cfg.ToClientKey(storageKey)
		if seenTopics[topic] {
			continue
		}
		seenTopics[topic] = true
		docIndex = append(docIndex, map[string]interface{}{
			"topic":  topic,
			"values": map[string]interface{}{"StringValue": cfg.ToClientValue(login, storageKey, storedMeta[storageKey])},
		})
	}
	return docIndex
}

// winyardDocInfo converts a CS3 file into a WinYard DocInfo.
// withIndex adds DocIndex (needs a Stat/List with ArbitraryMetadataKeys "*"),
// a non-empty sessionID adds the download FileKey.
// parentFolderID returns the folder a resource lives in. Right after an
// upload the gateway may report no ParentId, or the resource's own ID (seen on
// brandis 2026-11-08: Saskia then tried to file a sub-document "into" the main
// document). In that case the parent is looked up by path below the space root.
func (h *Handlers) parentFolderID(ctx context.Context, info *provider.ResourceInfo) *provider.ResourceId {
	if info == nil || info.Id == nil {
		return nil
	}
	if info.ParentId != nil && info.ParentId.OpaqueId != "" && info.ParentId.OpaqueId != info.Id.OpaqueId {
		return info.ParentId
	}
	if h.gw == nil || h.gw.Gateway == nil {
		return info.ParentId
	}
	root := &provider.ResourceId{StorageId: info.Id.StorageId, SpaceId: info.Id.SpaceId, OpaqueId: info.Id.SpaceId}
	relative := strings.Trim(path.Dir(strings.TrimPrefix(strings.TrimPrefix(info.Path, "."), "/")), "/")
	// info.Path is relative to the space root; some gateways prefix the space name
	candidates := []string{relative}
	if first, rest, found := strings.Cut(relative, "/"); found && first != "" {
		candidates = append(candidates, rest)
	} else if relative != "" && relative != "." {
		candidates = append(candidates, "")
	}
	for _, candidate := range candidates {
		ref := &provider.Reference{ResourceId: root, Path: "."}
		if candidate != "" && candidate != "." {
			ref.Path = "./" + candidate
		}
		statRes, err := h.gw.Gateway.Stat(ctx, &provider.StatRequest{Ref: ref})
		if err == nil && statRes.Status.Code == rpc.Code_CODE_OK && statRes.Info != nil && statRes.Info.Id != nil &&
			statRes.Info.Type == provider.ResourceType_RESOURCE_TYPE_CONTAINER && statRes.Info.Id.OpaqueId != info.Id.OpaqueId {
			return statRes.Info.Id
		}
	}
	log.Warn().Str("path", info.Path).Msg("parentFolderID: parent not resolvable by path")
	return info.ParentId
}

func (h *Handlers) winyardDocInfo(ctx context.Context, info *provider.ResourceInfo, login string, withIndex bool, sessionID string) map[string]interface{} {
	docName, extension := splitDocName(path.Base(info.Path))
	modified := winyardTime(info)
	docInfo := map[string]interface{}{
		"DocId":          clientID(info.Id),
		"DocName":        docName,
		"FileExtension":  extension,
		"ParentFolderId": clientID(h.parentFolderID(ctx, info)),
		"FullpathString": unescapeFolderName(info.Path),
		"FileSize":       info.Size,
		"Version":        1,
		"LatestVersion":  true,
		"Checked_Out":    false,
		"HasNotes":       0,
	}
	if modified != "" {
		docInfo["Created"] = modified
		docInfo["LastUpdated"] = modified
		docInfo["FileCreated"] = modified
	}
	if info.ArbitraryMetadata != nil {
		if docType := info.ArbitraryMetadata.Metadata["doc.type"]; docType != "" {
			docInfo["DocType"] = map[string]interface{}{"Name": docType}
		}
	}
	if withIndex {
		docInfo["DocIndex"] = h.docIndexFromMetadata(info, login)
	}
	if fileKey := downloadFileKey(sessionID, info.Id); fileKey != nil {
		docInfo["FileKey"] = fileKey
	}
	return docInfo
}

// --- Upload store (UploadFile → SetDocument / UpdateDocument) ---

type pendingUpload struct {
	filePath  string
	extension string // with dot, as sent by the client (".pdf")
	sessionID string
}

type uploadStore struct {
	cache *gocache.Cache
	dir   string
}

func newUploadStore(ttl time.Duration) *uploadStore {
	store := &uploadStore{
		cache: gocache.New(ttl, ttl/4),
		dir:   path.Join(os.TempDir(), "openyard-uploads"),
	}
	store.cache.OnEvicted(func(_ string, value interface{}) {
		if upload, ok := value.(*pendingUpload); ok {
			os.Remove(upload.filePath)
		}
	})
	return store
}

// save stores the content and returns the upload ID.
func (s *uploadStore) save(content io.Reader, extension, sessionID string) (string, error) {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return "", err
	}
	tempFile, err := os.CreateTemp(s.dir, "upload-*")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(tempFile, content); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", err
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}
	uploadID := path.Base(tempFile.Name())
	s.cache.SetDefault(uploadID, &pendingUpload{filePath: tempFile.Name(), extension: extension, sessionID: sessionID})
	return uploadID, nil
}

// get returns the upload for a FileKey created by UploadFile in the same session.
func (s *uploadStore) get(fileKey []byte, sessionID string) (*pendingUpload, bool) {
	kind, keySessionID, uploadID, ok := parseFileKey(fileKey)
	if !ok || kind != uploadKeyPrefix || keySessionID != sessionID {
		return nil, false
	}
	value, found := s.cache.Get(uploadID)
	if !found {
		return nil, false
	}
	return value.(*pendingUpload), true
}

// release removes a consumed upload (file is deleted by OnEvicted).
func (s *uploadStore) release(fileKey []byte) {
	if _, _, uploadID, ok := parseFileKey(fileKey); ok {
		if value, found := s.cache.Get(uploadID); found {
			s.cache.Delete(uploadID)
			os.Remove(value.(*pendingUpload).filePath)
		}
	}
}

// sessionByID returns the session context for a session ID, or nil.
func (h *Handlers) sessionByID(sessionID string) *auth.SessionContext {
	if sessionID == "" || h.sessions == nil {
		return nil
	}
	return h.sessions.Get(sessionID)
}

// statWithMetadata stats a reference including all arbitrary metadata.
func (h *Handlers) statWithMetadata(ctx context.Context, ref *provider.Reference) (*provider.ResourceInfo, error) {
	statRes, err := h.gw.Gateway.Stat(ctx, &provider.StatRequest{Ref: ref, ArbitraryMetadataKeys: []string{"*"}})
	if err != nil {
		return nil, err
	}
	if statRes.Status.Code == rpc.Code_CODE_NOT_FOUND {
		return nil, errObjectNotFound
	}
	if statRes.Status.Code != rpc.Code_CODE_OK {
		return nil, errors.New(statRes.Status.Message)
	}
	return statRes.Info, nil
}

// putContent uploads content to ref (new file or new version) via the
// data gateway, like SetFile.
func (h *Handlers) putContent(ctx context.Context, sess *auth.SessionContext, ref *provider.Reference, content io.Reader, size int64) error {
	gw := h.gw.GetGateway()
	// Upload-Length is required: without it decomposedfs creates no upload
	// session for the "simple" protocol and the data PUT fails with
	// ERR_UPLOAD_NOT_FOUND (brandis 2026-11-08, Saskia Unterdokumente).
	initReq := &provider.InitiateFileUploadRequest{
		Ref: ref,
		Opaque: &typesv1.Opaque{Map: map[string]*typesv1.OpaqueEntry{
			"Upload-Length": {Decoder: "plain", Value: []byte(strconv.FormatInt(size, 10))},
		}},
	}
	res, err := gw.InitiateFileUpload(ctx, initReq)
	if err != nil && h.gw.CheckError(err) {
		log.Info().Msg("cs3: retrying InitiateFileUpload after reconnect")
		time.Sleep(time.Second)
		gw = h.gw.GetGateway()
		res, err = gw.InitiateFileUpload(ctx, initReq)
	}
	if err != nil {
		return err
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		return errors.New(res.Status.Message)
	}
	var uploadToken string
	for _, protocol := range res.Protocols {
		if protocol.Protocol == "simple" {
			uploadToken = protocol.Token
			break
		}
		if uploadToken == "" {
			uploadToken = protocol.Token
		}
	}
	if uploadToken == "" {
		return errors.New("no upload protocol available")
	}
	uploadReq, err := http.NewRequestWithContext(ctx, "PUT", h.uploadURL+"/data", content)
	if err != nil {
		return err
	}
	uploadReq.ContentLength = size
	uploadReq.Header.Set("X-Access-Token", sess.CS3Token)
	uploadReq.Header.Set("X-Reva-Transfer", uploadToken)
	resp, err := insecureClient.Do(uploadReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return errors.New("upload rejected: " + resp.Status + " " + string(errBody))
	}
	return nil
}
