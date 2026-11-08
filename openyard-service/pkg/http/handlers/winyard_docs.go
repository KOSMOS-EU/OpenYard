package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/rs/zerolog/log"

	"github.com/kosmos-eu/openyard/pkg/auth"
)

// defaultSearchResults limits SearchForDocIndex without MaxResults.
const defaultSearchResults = 500

// maxSearchVisits bounds the folder walk of SearchForDocIndex.
const maxSearchVisits = 50000

// fileRequest reads the GetFile body. WinYard sends the FileKey as JSON
// base64 string; OpenYard 1.x sends {"ObjectId": ...} plus ?SessionID=.
// found is false if the body names no object. sess is nil if the session
// is unknown or expired.
func (h *Handlers) fileRequest(r *http.Request) (sess *auth.SessionContext, objectID string, found bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		return nil, "", false
	}
	var fileKey []byte
	if json.Unmarshal(body, &fileKey) == nil && len(fileKey) > 0 {
		kind, sessionID, payload, ok := parseFileKey(fileKey)
		if !ok || kind != downloadKeyPrefix {
			return nil, "", false
		}
		return h.sessionByID(sessionID), payload, true
	}
	var legacyBody struct {
		ObjectId string `json:"ObjectId"`
	}
	if json.Unmarshal(body, &legacyBody) != nil || legacyBody.ObjectId == "" {
		return nil, "", false
	}
	return h.sessionByID(sessionIDFromRequest(r)), legacyBody.ObjectId, true
}

// documentNotFound answers like WinYard; Saskia looks for "existiert nicht".
func documentNotFound(w http.ResponseWriter, docID string) {
	writeError(w, 404, "NOT_FOUND", fmt.Sprintf("Dokument %s existiert nicht", docID))
}

// POST /api/advancedDocuments/GetDocument?SessionID=&DocID=&LoadFileKey=&LoadDocIndex=&CheckOut=
//
// Response: WinYard Document {DocInfo, FileKey, Rights, TaskLog}.
func (h *Handlers) GetDocument(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	docID := queryValue(r, "DocID")
	if docID == "" {
		docID = queryValue(r, "ObjectId")
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

	ref, err := h.refFromID(r.Context(), docID)
	if err != nil {
		documentNotFound(w, docID)
		return
	}
	info, err := h.statWithMetadata(r.Context(), ref)
	if err != nil {
		if err == errObjectNotFound {
			documentNotFound(w, docID)
			return
		}
		log.Error().Err(err).Str("docId", docID).Msg("GetDocument: stat failed")
		writeError(w, 500, "INTERNAL_ERROR", "Storage error")
		return
	}

	login := ""
	if sess := sessionFromCtx(r.Context()); sess != nil {
		login = sess.Login
	}
	sessionID := sessionIDFromRequest(r)
	docInfo := h.winyardDocInfo(r.Context(), info, login, queryBool(r, "LoadDocIndex", true), "")
	document := map[string]interface{}{
		"DocInfo": docInfo,
		"Rights":  fullRights(),
		"TaskLog": taskLogOK(),
	}
	if queryBool(r, "LoadFileKey", true) {
		fileKey := downloadFileKey(sessionID, info.Id)
		document["FileKey"] = fileKey
		docInfo["FileKey"] = fileKey
	}
	writeJSON(w, 200, document)
}

// GET /api/advancedDocuments/GetPreviewFile?SessionID=&Doc_Id=
//
// Response: WinYard Preview {DocInfo, preview, TaskLog}. OpenYard renders no
// previews, "preview" stays empty (Saskia then shows the original file).
func (h *Handlers) GetPreviewFile(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	docID := queryValue(r, "Doc_Id")
	if docID == "" {
		docID = queryValue(r, "DocId")
	}
	ref, err := h.refFromID(r.Context(), docID)
	if err != nil {
		documentNotFound(w, docID)
		return
	}
	info, err := h.statWithMetadata(r.Context(), ref)
	if err != nil {
		documentNotFound(w, docID)
		return
	}
	login := ""
	if sess := sessionFromCtx(r.Context()); sess != nil {
		login = sess.Login
	}
	writeJSON(w, 200, map[string]interface{}{
		"DocInfo": h.winyardDocInfo(r.Context(), info, login, false, ""),
		"TaskLog": taskLogOK(),
	})
}

// POST /api/basicDocuments/UploadFile?SessionID=&Extension=&CreationDate=&lastModifiedDate=&FileHash=
//
// Multipart field "file". Stores the content until SetDocument or
// UpdateDocument references it. Response: FileKey (JSON base64).
// With ObjectId (query or form) the OpenYard 1.x upload (SetFile) runs.
func (h *Handlers) UploadFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, 400, "INVALID_REQUEST", "Multipart form required")
		return
	}
	if r.URL.Query().Get("ObjectId") != "" || r.FormValue("ObjectId") != "" {
		h.SetFile(w, r)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", "File part required")
		return
	}
	defer file.Close()

	extension := queryValue(r, "Extension")
	if extension == "" {
		extension = path.Ext(header.Filename)
	}
	if extension != "" && !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}
	sessionID := sessionIDFromRequest(r)
	uploadID, err := pendingUploads.save(file, extension, sessionID)
	if err != nil {
		log.Error().Err(err).Msg("UploadFile: store failed")
		writeError(w, 500, "INTERNAL_ERROR", "Store upload failed")
		return
	}
	writeJSON(w, 200, []byte(uploadKeyPrefix+"|"+sessionID+"|"+uploadID))
}

// winyardDocIndexEntry is a WDocIndex entry of SetDocument.
type winyardDocIndexEntry struct {
	Topic string `json:"topic"`
	Value string `json:"value"`
}

// winyardWDocInfo is the SetDocument body (fields OpenYard evaluates).
type winyardWDocInfo struct {
	DocName        string                 `json:"DocName"`
	DocId          string                 `json:"DocId"`
	DocType        string                 `json:"DocType"`
	ParentFolderId string                 `json:"ParentFolderId"`
	DocIndex       []winyardDocIndexEntry `json:"DocIndex"`
	FileKey        []byte                 `json:"FileKey"`
}

// POST /api/advancedDocuments/SetDocument?SessionID=
//
// Body: WinYard WDocInfo. Without DocId a new document is created in
// ParentFolderId from the uploaded FileKey; with DocId the existing
// document gets the new content (if FileKey) and the DocIndex.
// Response: WinYard Document. Multipart requests run SetFile (OpenYard 1.x).
func (h *Handlers) SetDocument(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") || r.URL.Query().Get("ObjectId") != "" {
		h.SetFile(w, r)
		return
	}
	r = withCS3Token(r)
	sess := sessionFromCtx(r.Context())
	if sess == nil {
		writeError(w, 401, "AUTH_REQUIRED", "No session")
		return
	}
	sessionID := sessionIDFromRequest(r)

	var wDocInfo winyardWDocInfo
	if err := json.NewDecoder(r.Body).Decode(&wDocInfo); err != nil {
		writeError(w, 400, "INVALID_REQUEST", "WDocInfo JSON required")
		return
	}

	var upload *pendingUpload
	if len(wDocInfo.FileKey) > 0 {
		found := false
		upload, found = pendingUploads.get(wDocInfo.FileKey, sessionID)
		if !found {
			writeError(w, 400, "INVALID_REQUEST", "FileKey unknown or expired")
			return
		}
	}

	var targetRef *provider.Reference
	if wDocInfo.DocId != "" && wDocInfo.DocId != nilGUID {
		docRef, err := h.refFromID(r.Context(), wDocInfo.DocId)
		if err != nil {
			documentNotFound(w, wDocInfo.DocId)
			return
		}
		targetRef = docRef
	} else {
		if upload == nil {
			writeError(w, 400, "INVALID_REQUEST", "FileKey required for new document")
			return
		}
		parentID, err := h.resourceIDFromID(r.Context(), wDocInfo.ParentFolderId)
		if err != nil {
			writeError(w, 404, "NOT_FOUND", fmt.Sprintf("Ordner %s existiert nicht", wDocInfo.ParentFolderId))
			return
		}
		// Saskia CreateSubDocAsync passes the main document's ParentFolderId; if
		// that resolves to a file (see parentFolderID), file next to it instead
		// of uploading "into" the document (brandis 2026-11-08: 500 from the gateway).
		parentInfo, err := h.statWithMetadata(r.Context(), &provider.Reference{ResourceId: parentID})
		if err != nil {
			writeError(w, 404, "NOT_FOUND", fmt.Sprintf("Ordner %s existiert nicht", wDocInfo.ParentFolderId))
			return
		}
		if parentInfo.Type != provider.ResourceType_RESOURCE_TYPE_CONTAINER {
			folder := h.parentFolderID(r.Context(), parentInfo)
			if folder == nil || folder.OpaqueId == parentInfo.Id.OpaqueId {
				writeError(w, 400, "INVALID_REQUEST", fmt.Sprintf("%s ist kein Ordner", wDocInfo.ParentFolderId))
				return
			}
			log.Info().Str("parent", wDocInfo.ParentFolderId).Msg("SetDocument: parent is a document, filing next to it")
			parentID = folder
		}
		fileName := wDocInfo.DocName
		if upload.extension != "" && !strings.EqualFold(path.Ext(fileName), upload.extension) {
			fileName += upload.extension
		}
		// unique key of the document type: replace the existing document (new version)
		index := make(map[string]string, len(wDocInfo.DocIndex))
		for _, entry := range wDocInfo.DocIndex {
			index[entry.Topic] = entry.Value
		}
		if existing := h.findDuplicate(r.Context(), sess.Login, wDocInfo.DocType, index, parentID); existing != nil {
			targetRef = &provider.Reference{ResourceId: existing.Id}
		} else {
			targetRef = h.freeChildRef(r, parentID, fileName)
		}
	}

	if upload != nil {
		content, err := os.Open(upload.filePath)
		if err != nil {
			writeError(w, 400, "INVALID_REQUEST", "FileKey unknown or expired")
			return
		}
		size := int64(0)
		if fileInfo, statErr := content.Stat(); statErr == nil {
			size = fileInfo.Size()
		}
		err = h.putContent(r.Context(), sess, targetRef, content, size)
		content.Close()
		if err != nil {
			log.Error().Err(err).Msg("SetDocument: upload failed")
			writeError(w, 500, "INTERNAL_ERROR", "Upload failed")
			return
		}
		pendingUploads.release(wDocInfo.FileKey)
	}

	info, err := h.statWithMetadata(r.Context(), targetRef)
	if err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Stat after upload failed")
		return
	}
	docIndex := map[string]string{}
	for _, entry := range wDocInfo.DocIndex {
		docIndex[entry.Topic] = entry.Value
	}
	if err := h.storeDocIndex(r, sess.Login, info.Id, wDocInfo.DocType, docIndex); err != nil {
		log.Error().Err(err).Msg("SetDocument: set metadata failed")
		writeError(w, 500, "INTERNAL_ERROR", "Set metadata failed")
		return
	}
	if info, err = h.statWithMetadata(r.Context(), &provider.Reference{ResourceId: info.Id}); err != nil {
		writeError(w, 500, "INTERNAL_ERROR", "Stat after metadata failed")
		return
	}
	log.Info().Str("docId", clientID(info.Id)).Str("path", info.Path).Msg("SetDocument")
	writeJSON(w, 200, map[string]interface{}{
		"DocInfo": h.winyardDocInfo(r.Context(), info, sess.Login, true, ""),
		"FileKey": downloadFileKey(sessionID, info.Id),
		"Rights":  fullRights(),
		"TaskLog": taskLogOK(),
	})
}

// freeChildRef returns a reference to fileName in parent, adding " (2)",
// " (3)", ... if the name is taken (a new document never overwrites).
func (h *Handlers) freeChildRef(r *http.Request, parentID *provider.ResourceId, fileName string) *provider.Reference {
	extension := path.Ext(fileName)
	baseName := strings.TrimSuffix(fileName, extension)
	candidate := fileName
	for attempt := 2; attempt < 1000; attempt++ {
		ref := &provider.Reference{ResourceId: parentID, Path: "./" + escapeFolderName(candidate)}
		statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
		if err != nil || statRes.Status.Code == rpc.Code_CODE_NOT_FOUND {
			return ref
		}
		candidate = fmt.Sprintf("%s (%d)%s", baseName, attempt, extension)
	}
	return &provider.Reference{ResourceId: parentID, Path: "./" + escapeFolderName(candidate)}
}

// storeDocIndex writes source / doc type metadata and the DocIndex topics.
func (h *Handlers) storeDocIndex(r *http.Request, login string, resourceID *provider.ResourceId, docType string, docIndex map[string]string) error {
	storageMeta := h.metaConfig().DocumentMetadata(login, docType)
	for topic, value := range docIndex {
		storageKey := mapMetaKeyForUser(h.metaCfg, login, topic)
		storageMeta[storageKey] = h.metaConfig().ToStorageValue(login, storageKey, value)
	}
	if len(storageMeta) == 0 {
		return nil
	}
	res, err := h.gw.Gateway.SetArbitraryMetadata(r.Context(), &provider.SetArbitraryMetadataRequest{
		Ref:               &provider.Reference{ResourceId: resourceID},
		ArbitraryMetadata: &provider.ArbitraryMetadata{Metadata: storageMeta},
	})
	if err != nil {
		return err
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		return fmt.Errorf("set metadata: %s", res.Status.Message)
	}
	return nil
}

// PUT /api/basicDocuments/UpdateDocument?SessionID=&DocId=&CheckIn=&Freeze=
//
// Body: FileKey from UploadFile (JSON base64). Replaces the content of the
// document (new version). Response: true.
func (h *Handlers) UpdateDocument(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)
	sess := sessionFromCtx(r.Context())
	if sess == nil {
		writeError(w, 401, "AUTH_REQUIRED", "No session")
		return
	}
	docID := queryValue(r, "DocId")
	var fileKey []byte
	if err := json.NewDecoder(r.Body).Decode(&fileKey); err != nil || len(fileKey) == 0 {
		writeError(w, 400, "INVALID_REQUEST", "FileKey required")
		return
	}
	upload, found := pendingUploads.get(fileKey, sessionIDFromRequest(r))
	if !found {
		writeError(w, 400, "INVALID_REQUEST", "FileKey unknown or expired")
		return
	}
	ref, err := h.refFromID(r.Context(), docID)
	if err != nil {
		documentNotFound(w, docID)
		return
	}
	content, err := os.Open(upload.filePath)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", "FileKey unknown or expired")
		return
	}
	defer content.Close()
	size := int64(0)
	if fileInfo, statErr := content.Stat(); statErr == nil {
		size = fileInfo.Size()
	}
	if err := h.putContent(r.Context(), sess, ref, content, size); err != nil {
		log.Error().Err(err).Str("docId", docID).Msg("UpdateDocument: upload failed")
		writeError(w, 500, "INTERNAL_ERROR", "Upload failed")
		return
	}
	pendingUploads.release(fileKey)
	writeJSON(w, 200, true)
}

// searchCondition is one SearchForDocIndex body entry.
type searchCondition struct {
	Topic   string `json:"Topic"`
	Compare string `json:"Compare"`
	Value   string `json:"Value"`
}

// matches compares a stored value with the condition (case-insensitive).
// Supported: Equal (default), NotEqual, Like/Contains ("*" wildcards ignored),
// StartsWith.
func (c searchCondition) matches(storedValue string, present bool) bool {
	stored := strings.ToLower(strings.TrimSpace(storedValue))
	wanted := strings.ToLower(strings.TrimSpace(c.Value))
	switch strings.ToLower(c.Compare) {
	case "notequal", "unequal":
		return !present || stored != wanted
	case "like", "contains":
		return present && strings.Contains(stored, strings.Trim(wanted, "*%"))
	case "startswith", "beginswith":
		return present && strings.HasPrefix(stored, strings.Trim(wanted, "*%"))
	default:
		return present && stored == wanted
	}
}

// docSearch holds a prepared SearchForDocIndex query.
type docSearch struct {
	docType     string                     // requested client doc type ("" = any)
	conditions  map[string]searchCondition // storage key → condition
	conjunction bool                       // true: all conditions (AND), false: any (OR)
}

// matches reports whether a document's metadata fulfils the search.
func (s docSearch) matches(metadata map[string]string) bool {
	if s.docType != "" {
		// doc.type is shared with Taki: invoices from the mail room match too
		if !strings.EqualFold(metadata["doc.type"], s.docType) {
			return false
		}
	}
	if len(s.conditions) == 0 {
		return true
	}
	for storageKey, condition := range s.conditions {
		storedValue, present := metadata[storageKey]
		conditionMatches := condition.matches(storedValue, present)
		if s.conjunction && !conditionMatches {
			return false
		}
		if !s.conjunction && conditionMatches {
			return true
		}
	}
	return s.conjunction
}

// POST /api/basicDocuments/SearchForDocIndex?SessionID=&SearchInFolder=&DocType=&Conjunction=&MaxResults=
//
// Body: [{"Topic", "Compare", "Value"}]. Walks SearchInFolder (empty GUID:
// all spaces of the user) and returns matching documents as WinYard
// DocInfo list with DocIndex. Response is never null (Saskia iterates it).
func (h *Handlers) SearchForDocIndex(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)
	login := ""
	if sess := sessionFromCtx(r.Context()); sess != nil {
		login = sess.Login
	}

	var conditions []searchCondition
	if body, _ := io.ReadAll(r.Body); len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &conditions); err != nil {
			writeError(w, 400, "INVALID_REQUEST", "Search conditions JSON array required")
			return
		}
	}
	cfg := h.metaConfig()
	search := docSearch{
		docType:     queryValue(r, "DocType"),
		conditions:  map[string]searchCondition{},
		conjunction: queryBool(r, "Conjunction", true),
	}
	for _, condition := range conditions {
		if condition.Topic == "" {
			continue
		}
		storageKey := cfg.ToStorageKey(login, condition.Topic)
		condition.Value = cfg.ToStorageValue(login, storageKey, condition.Value)
		search.conditions[storageKey] = condition
	}
	maxResults := defaultSearchResults
	if value, err := strconv.Atoi(queryValue(r, "MaxResults")); err == nil && value > 0 {
		maxResults = value
	}

	roots, err := h.searchRoots(r, queryValue(r, "SearchInFolder"))
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "SearchInFolder not found")
		return
	}

	results := make([]map[string]interface{}, 0)
	visited := 0
	queue := roots
	for len(queue) > 0 && len(results) < maxResults && visited < maxSearchVisits {
		folderRef := queue[0]
		queue = queue[1:]
		listRes, err := h.gw.Gateway.ListContainer(r.Context(), &provider.ListContainerRequest{
			Ref:                   folderRef,
			ArbitraryMetadataKeys: []string{"*"},
		})
		if err != nil || listRes.Status.Code != rpc.Code_CODE_OK {
			continue
		}
		for _, info := range listRes.Infos {
			visited++
			if info.Type == provider.ResourceType_RESOURCE_TYPE_CONTAINER {
				queue = append(queue, &provider.Reference{ResourceId: info.Id})
				continue
			}
			metadata := cfg.WithDefaults(login, info.GetArbitraryMetadata().GetMetadata())
			if search.matches(metadata) {
				results = append(results, h.winyardDocInfo(r.Context(), info, login, true, ""))
				if len(results) >= maxResults {
					break
				}
			}
		}
	}
	if visited >= maxSearchVisits {
		log.Warn().Int("visited", visited).Msg("SearchForDocIndex: walk limit reached, results may be incomplete")
	}
	log.Info().Str("docType", search.docType).Int("conditions", len(search.conditions)).Int("visited", visited).Int("found", len(results)).Msg("SearchForDocIndex")
	writeJSON(w, 200, results)
}

// searchRoots returns the folders SearchForDocIndex walks: the given folder,
// or the roots of all personal and project spaces of the user.
func (h *Handlers) searchRoots(r *http.Request, folderID string) ([]*provider.Reference, error) {
	folderID = strings.Trim(folderID, "{}")
	if folderID != "" && folderID != nilGUID {
		folderRef, err := h.refFromID(r.Context(), folderID)
		if err != nil {
			return nil, err
		}
		return []*provider.Reference{folderRef}, nil
	}
	spacesRes, err := h.gw.Gateway.ListStorageSpaces(r.Context(), &provider.ListStorageSpacesRequest{})
	if err != nil {
		return nil, err
	}
	roots := make([]*provider.Reference, 0, len(spacesRes.StorageSpaces))
	for _, space := range spacesRes.StorageSpaces {
		if space.Root == nil || (space.SpaceType != "personal" && space.SpaceType != "project") {
			continue
		}
		roots = append(roots, &provider.Reference{ResourceId: space.Root})
	}
	return roots, nil
}

// boolReturn is the WinYard BoolReturn answer.
func boolReturn(value bool) map[string]interface{} {
	return map[string]interface{}{"ReturnValue": value, "TaskLog": taskLogOK()}
}

// POST /api/advancedCommon/SetTopic?SessionID=
//
// Body: WinYard Topic {index_id, index_topic, data_type}. OpenCloud metadata
// needs no topic registry, so the topic is only logged.
func (h *Handlers) SetTopic(w http.ResponseWriter, r *http.Request) {
	var topic struct {
		IndexID    string      `json:"index_id"`
		IndexTopic string      `json:"index_topic"`
		DataType   interface{} `json:"data_type"`
	}
	json.NewDecoder(r.Body).Decode(&topic)
	log.Info().Str("topic", topic.IndexTopic).Str("id", topic.IndexID).Interface("type", topic.DataType).Msg("SetTopic (no registry, accepted)")
	writeJSON(w, 200, boolReturn(true))
}

// DELETE /api/advancedCommon/DeleteTopic?SessionID=&TopicId=
func (h *Handlers) DeleteTopic(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("id", queryValue(r, "TopicId")).Msg("DeleteTopic (no registry, accepted)")
	writeJSON(w, 200, boolReturn(true))
}
