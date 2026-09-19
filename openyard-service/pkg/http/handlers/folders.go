package handlers

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"

	userpb "github.com/cs3org/go-cs3apis/cs3/identity/user/v1beta1"
	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	gocache "github.com/patrickmn/go-cache"
	"github.com/rs/zerolog/log"
)

// POST /api/advancedFolders/GetFolder
// legacy DMS uses FolderID as query parameter, body is empty {}.
// FolderID=00000000-0000-0000-0000-000000000000 means root (list all spaces/volumes).
func (h *Handlers) GetFolder(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	folderID := r.URL.Query().Get("FolderID")
	if folderID == "" {
		// Try reading from JSON body (OpenYard client sends it this way)
		var body struct {
			FolderId string `json:"FolderId"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		folderID = body.FolderId
	}

	// Root request: list all spaces as volumes
	if folderID == "" || folderID == "00000000-0000-0000-0000-000000000000" {
		h.getFolderRoot(w, r)
		return
	}

	if h.gw == nil || h.gw.Gateway == nil {
		writeError(w, 500, "INTERNAL_ERROR", "CS3 gateway not configured")
		return
	}

	// Resolve ObjectID to CS3 Reference (handles spaces and regular folders)
	ref, err := h.resolveRef(r.Context(), folderID)
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "Folder not found")
		return
	}

	// Stat the folder
	statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "NOT_FOUND", "Folder not found")
		return
	}

	// List contents
	listRes, err := h.gw.Gateway.ListContainer(r.Context(), &provider.ListContainerRequest{Ref: ref})
	if err != nil || listRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "INTERNAL_ERROR", "Could not list folder contents")
		return
	}

	// Build legacy DMS-compatible response
	allFolders := make([]map[string]interface{}, 0)
	allDocs := make([]map[string]interface{}, 0)
	for _, info := range listRes.Infos {
		mapped := h.mapResourceInfo(info)
		if info.Type == provider.ResourceType_RESOURCE_TYPE_CONTAINER {
			allFolders = append(allFolders, map[string]interface{}{
				"FolderID":   encodeObjectID(info.Id),
				"Foldername": unescapeFolderName(path.Base(info.Path)),
				"Aktz":       "",
			})
		} else {
			allDocs = append(allDocs, mapped)
		}
	}

	// Pagination via MaxResultPerRequestForWorkObject (default: return all)
	maxPerRequest := 0
	if v := r.URL.Query().Get("MaxResultPerRequestForWorkObject"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxPerRequest = n
		}
	}

	folderInfo := mapFolderInfo(statRes.Info)
	folderInfo["SubFoldersCount"] = len(allFolders)
	folderInfo["SubDocsCount"] = len(allDocs)

	subFolders, subDocs, nextWorkID := h.paginateSubElements(folderInfo, allFolders, allDocs, maxPerRequest)

	folderInfo["SubElementsLoaded"] = len(subFolders) + len(subDocs)

	writeJSON(w, 200, map[string]interface{}{
		"FolderInfo":        folderInfo,
		"SubFolders":        subFolders,
		"SubDocs":           subDocs,
		"TotalResultsCount": len(allFolders) + len(allDocs),
		"NextWorkId":        nextWorkID,
		"Notes":             nil,
		"TaskLog":           taskLogOK(),
	})
}

// GET /api/advancedFolders/GetFolderByWorkID?WorkID={WorkID}
// Returns the next batch of sub-elements for a paginated GetFolder response.
func (h *Handlers) GetFolderByWorkID(w http.ResponseWriter, r *http.Request) {
	workID := r.URL.Query().Get("WorkID")
	if workID == "" || workID == "00000000-0000-0000-0000-000000000000" {
		writeError(w, 400, "INVALID_REQUEST", "WorkID required")
		return
	}

	v, ok := h.workCache.Get(workID)
	if !ok {
		writeError(w, 404, "NOT_FOUND", "WorkID expired or unknown")
		return
	}
	h.workCache.Delete(workID)
	wo := v.(*workObject)

	// MaxResultPerRequestForWorkObject from original request carries over
	maxPerRequest := 0
	if v := r.URL.Query().Get("MaxResultPerRequestForWorkObject"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxPerRequest = n
		}
	}

	folderInfo := wo.FolderInfo
	subFolders, subDocs, nextWorkID := h.paginateSubElements(folderInfo, wo.SubFolders, wo.SubDocs, maxPerRequest)

	folderInfo["SubElementsLoaded"] = len(subFolders) + len(subDocs)

	writeJSON(w, 200, map[string]interface{}{
		"FolderInfo":        folderInfo,
		"SubFolders":        subFolders,
		"SubDocs":           subDocs,
		"TotalResultsCount": len(wo.SubFolders) + len(wo.SubDocs),
		"NextWorkId":        nextWorkID,
		"Notes":             nil,
		"TaskLog":           taskLogOK(),
	})
}

// paginateSubElements returns the first maxPerRequest elements and stores the rest
// under a new WorkID. Returns all elements if maxPerRequest is 0 or total fits.
func (h *Handlers) paginateSubElements(
	folderInfo map[string]interface{},
	allFolders, allDocs []map[string]interface{},
	maxPerRequest int,
) (subFolders, subDocs []map[string]interface{}, nextWorkID string) {
	total := len(allFolders) + len(allDocs)
	nextWorkID = "00000000-0000-0000-0000-000000000000"

	if maxPerRequest <= 0 || total <= maxPerRequest {
		return allFolders, allDocs, nextWorkID
	}

	// Take folders first, then docs up to the limit
	if len(allFolders) >= maxPerRequest {
		subFolders = allFolders[:maxPerRequest]
		subDocs = nil
		remainFolders := allFolders[maxPerRequest:]
		remainDocs := allDocs
		nextWorkID = h.storeWorkObject(folderInfo, remainFolders, remainDocs)
	} else {
		subFolders = allFolders
		docsLimit := maxPerRequest - len(allFolders)
		if docsLimit > len(allDocs) {
			docsLimit = len(allDocs)
		}
		subDocs = allDocs[:docsLimit]
		remainDocs := allDocs[docsLimit:]
		if len(remainDocs) > 0 {
			nextWorkID = h.storeWorkObject(folderInfo, nil, remainDocs)
		}
	}
	return
}

func (h *Handlers) storeWorkObject(folderInfo map[string]interface{}, folders, docs []map[string]interface{}) string {
	wid := generateWorkID()
	h.workCache.Set(wid, &workObject{
		FolderInfo: folderInfo,
		SubFolders: folders,
		SubDocs:    docs,
	}, gocache.DefaultExpiration)
	return wid
}

func generateWorkID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// getFolderRoot lists all spaces as legacy DMS root volumes.
func (h *Handlers) getFolderRoot(w http.ResponseWriter, r *http.Request) {
	if h.gw == nil || h.gw.Gateway == nil {
		// No gateway — return empty root
		writeJSON(w, 200, map[string]interface{}{
			"FolderInfo":        map[string]interface{}{"FolderID": "00000000-0000-0000-0000-000000000000"},
			"SubFolders":        []interface{}{},
			"SubDocs":           []interface{}{},
			"TotalResultsCount": 0,
			"TaskLog":           taskLogOK(),
		})
		return
	}

	// List all storage spaces via CS3
	res, err := h.gw.Gateway.ListStorageSpaces(r.Context(), &provider.ListStorageSpacesRequest{})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		log.Error().Err(err).Msg("cs3 list spaces failed")
		writeJSON(w, 200, map[string]interface{}{
			"FolderInfo":        map[string]interface{}{"FolderID": "00000000-0000-0000-0000-000000000000"},
			"SubFolders":        []interface{}{},
			"SubDocs":           []interface{}{},
			"TotalResultsCount": 0,
			"TaskLog":           taskLogOK(),
		})
		return
	}

	subFolders := make([]map[string]interface{}, 0, len(res.StorageSpaces))
	for _, space := range res.StorageSpaces {
		fid := encodeObjectID(space.Root)
		subFolders = append(subFolders, map[string]interface{}{
			"FolderID":       fid,
			"Foldername":     space.Name,
			"Aktz":           "",
			"FolderLevel":    0,
			"ParentFolderID": "00000000-0000-0000-0000-000000000000",
			"Rights": map[string]interface{}{
				"RightsDigit": 4095,
			},
		})
	}

	writeJSON(w, 200, map[string]interface{}{
		"FolderInfo": map[string]interface{}{
			"FolderID":       "00000000-0000-0000-0000-000000000000",
			"Foldername":     nil,
			"SubFoldersCount": len(subFolders),
			"SubDocsCount":   0,
		},
		"SubFolders":        subFolders,
		"SubDocs":           []interface{}{},
		"TotalResultsCount": len(subFolders),
		"TaskLog":           taskLogOK(),
	})
}

// POST /api/advancedFolders/GetFolderByFolderpath
func (h *Handlers) GetFolderByFolderpath(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	folderPath := r.URL.Query().Get("FolderPath")
	if folderPath == "" {
		var body struct {
			FolderPath string `json:"FolderPath"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		folderPath = body.FolderPath
	}
	if folderPath == "" {
		writeError(w, 400, "INVALID_REQUEST", "FolderPath required")
		return
	}

	// "/" means root — list all spaces (same as GetFolder with root ID)
	if folderPath == "/" || folderPath == "\\" {
		spacesRes, err := h.gw.Gateway.ListStorageSpaces(r.Context(), &provider.ListStorageSpacesRequest{})
		if err != nil || spacesRes.Status.Code != rpc.Code_CODE_OK {
			writeError(w, 500, "INTERNAL_ERROR", "Cannot list spaces")
			return
		}
		subs := make([]map[string]interface{}, 0)
		for _, space := range spacesRes.StorageSpaces {
			if space.Root == nil {
				continue
			}
			subs = append(subs, mapFolderInfo(&provider.ResourceInfo{
				Id:   space.Root,
				Path: "/" + space.Name,
				Type: provider.ResourceType_RESOURCE_TYPE_CONTAINER,
			}))
		}
		writeJSON(w, 200, map[string]interface{}{
			"FolderInfo": mapFolderInfo(&provider.ResourceInfo{
				Path: "/",
				Type: provider.ResourceType_RESOURCE_TYPE_CONTAINER,
			}),
			"SubFolders": subs,
			"TaskLog":    taskLogOK(),
		})
		return
	}

	ref := refFromPath(folderPath)

	statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "NOT_FOUND", "Folder not found")
		return
	}

	listRes, err := h.gw.Gateway.ListContainer(r.Context(), &provider.ListContainerRequest{Ref: ref})
	if err != nil || listRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "INTERNAL_ERROR", "Could not list folder contents")
		return
	}

	contents := make([]map[string]interface{}, 0, len(listRes.Infos))
	for _, info := range listRes.Infos {
		contents = append(contents, h.mapResourceInfo(info))
	}

	folder := h.mapResourceInfo(statRes.Info)
	folder["childCount"] = len(contents)
	folder["contents"] = contents

	writeJSON(w, 200, folder)
}

// GET /api/advancedFolders/IsFolder
func (h *Handlers) IsFolder(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, 200, map[string]bool{"isFolder": false})
		return
	}

	writeJSON(w, 200, map[string]bool{
		"isFolder": res.Info.Type == provider.ResourceType_RESOURCE_TYPE_CONTAINER,
	})
}

// POST /api/advancedFolders/SetFolder
// Accepts both Path-based and ParentFolderID+FolderName-based creation.
func (h *Handlers) SetFolder(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	if h.gw == nil || h.gw.Gateway == nil {
		writeError(w, 500, "INTERNAL_ERROR", "CS3 gateway not configured")
		return
	}

	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body)
	if body == nil {
		body = map[string]interface{}{}
	}

	var ref *provider.Reference

	// Mode A: ParentFolderID + FolderName (OpenYard format)
	parentID, _ := body["ParentFolderID"].(string)
	folderName, _ := body["FolderName"].(string)
	if parentID != "" && folderName != "" {
		var err error
		ref, err = h.resolveRefWithChild(r.Context(), parentID, folderName)
		if err != nil {
			writeError(w, 400, "INVALID_REQUEST", err.Error())
			return
		}
	} else if p, ok := body["Path"].(string); ok && p != "" {
		// Mode B: Direct path
		ref = refFromPath(p)
	} else {
		writeError(w, 400, "INVALID_REQUEST", "ParentFolderID+FolderName or Path required")
		return
	}

	res, err := h.gw.Gateway.CreateContainer(r.Context(), &provider.CreateContainerRequest{Ref: ref})
	if err != nil {
		log.Error().Err(err).Msg("cs3 create container failed")
		writeError(w, 500, "INTERNAL_ERROR", "Create folder failed")
		return
	}
	if res.Status.Code != rpc.Code_CODE_OK && res.Status.Code != rpc.Code_CODE_ALREADY_EXISTS {
		writeError(w, 409, "CONFLICT", res.Status.Message)
		return
	}

	// Stat the new folder to get its FolderID
	folderInfo := map[string]interface{}{
		"Foldername": folderName,
	}
	statRes, serr := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if serr == nil && statRes.Status.Code == rpc.Code_CODE_OK && statRes.Info != nil {
		folderInfo = mapFolderInfo(statRes.Info)
	}

	writeJSON(w, 200, map[string]interface{}{
		"FolderInfo": folderInfo,
		"TaskLog":    taskLogOK(),
	})
}

// POST /api/advancedFolders/DeleteFolders
func (h *Handlers) DeleteFolders(w http.ResponseWriter, r *http.Request) {
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

// GET /api/advancedFolders/GetFolderRights
func (h *Handlers) GetFolderRights(w http.ResponseWriter, r *http.Request) {
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

	// Stat to get permissions
	res, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "NOT_FOUND", "Folder not found")
		return
	}

	perms := res.Info.PermissionSet
	writeJSON(w, 200, map[string]interface{}{
		"id": objectID,
		"permissions": map[string]bool{
			"canRead":     perms != nil && perms.Stat,
			"canWrite":    perms != nil && perms.InitiateFileUpload,
			"canDelete":   perms != nil && perms.Delete,
			"canShare":    perms != nil && perms.AddGrant,
			"canCreate":   perms != nil && perms.CreateContainer,
		},
	})
}

// POST /api/advancedFolders/SetFolderRights
func (h *Handlers) SetFolderRights(w http.ResponseWriter, r *http.Request) {
	// Spec: Impl-Kategorie opencloud-framework — OCS Share API + CS3 AddGrant
	// Phase 1: Stub
	log.Warn().Msg("SetFolderRights called — stub, grants not yet implemented")
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// POST /api/basicFolders/CreateFolderByParentFolderID
func (h *Handlers) CreateFolderByParentFolderID(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	var body struct {
		ParentFolderId string `json:"ParentFolderId"`
		Name           string `json:"Name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ParentFolderId == "" || body.Name == "" {
		writeError(w, 400, "INVALID_REQUEST", "ParentFolderId and Name required")
		return
	}

	// Resolve parent
	parentRef, err := refFromObjectID(body.ParentFolderId)
	if err != nil {
		writeError(w, 400, "INVALID_REQUEST", err.Error())
		return
	}

	parentStat, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: parentRef})
	if err != nil || parentStat.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "NOT_FOUND", "Parent folder not found")
		return
	}

	newPath := parentStat.Info.Path + "/" + body.Name
	ref := refFromPath(newPath)
	res, err := h.gw.Gateway.CreateContainer(r.Context(), &provider.CreateContainerRequest{Ref: ref})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "INTERNAL_ERROR", "Create folder failed")
		return
	}

	writeJSON(w, 200, map[string]interface{}{"path": newPath, "status": "ok"})
}

// POST /api/basicFolders/CreateFolderByParentFolderPath
func (h *Handlers) CreateFolderByParentFolderPath(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	var body struct {
		ParentPath string `json:"ParentPath"`
		Name       string `json:"Name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ParentPath == "" || body.Name == "" {
		writeError(w, 400, "INVALID_REQUEST", "ParentPath and Name required")
		return
	}

	newPath := body.ParentPath + "/" + body.Name
	ref := refFromPath(newPath)
	res, err := h.gw.Gateway.CreateContainer(r.Context(), &provider.CreateContainerRequest{Ref: ref})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "INTERNAL_ERROR", "Create folder failed")
		return
	}

	writeJSON(w, 200, map[string]interface{}{"path": newPath, "status": "ok"})
}

// POST /api/basicFolders/RenameFolder
func (h *Handlers) RenameFolder(w http.ResponseWriter, r *http.Request) {
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

	// Stat to get parent info
	statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: srcRef})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "NOT_FOUND", "Folder not found")
		return
	}

	// Build destination ref using the parent's ResourceId + new name
	dstRef := &provider.Reference{
		ResourceId: statRes.Info.ParentId,
		Path:       "./" + escapeFolderName(body.NewName),
	}

	res, err := h.gw.Gateway.Move(r.Context(), &provider.MoveRequest{Source: srcRef, Destination: dstRef})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		msg := "Rename failed"
		if res != nil && res.Status != nil {
			msg = res.Status.Message
		}
		writeError(w, 500, "INTERNAL_ERROR", msg)
		return
	}

	writeJSON(w, 200, map[string]interface{}{"status": "ok"})
}

func path_base(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

// resolveRef resolves an encoded ObjectID to a CS3 Reference.
// For space roots, returns a Reference with ResourceId + relative path ".".
// For regular objects, tries Stat to get the ResourceId.
func (h *Handlers) resolveRef(ctx context.Context, objectID string) (*provider.Reference, error) {
	rid, err := decodeObjectID(objectID)
	if err != nil {
		return nil, err
	}

	// Return a ResourceId-based reference (works for spaces and regular folders)
	return &provider.Reference{
		ResourceId: rid,
		Path:       ".",
	}, nil
}

// resolveRefWithChild resolves a parent ObjectID and appends a child name.
// Escapes "/" in folder names to Unicode fraction slash (U+2215) for storage.
func (h *Handlers) resolveRefWithChild(ctx context.Context, parentID, childName string) (*provider.Reference, error) {
	rid, err := decodeObjectID(parentID)
	if err != nil {
		return nil, err
	}

	return &provider.Reference{
		ResourceId: rid,
		Path:       "./" + escapeFolderName(childName),
	}, nil
}

// escapeFolderName replaces "/" in folder names with Unicode fraction slash (U+2215)
// so they can be stored on POSIX filesystems. Reversed by unescapeFolderName.
func escapeFolderName(name string) string {
	return strings.ReplaceAll(name, "/", "\u2215")
}

// unescapeFolderName reverses escapeFolderName.
func unescapeFolderName(name string) string {
	return strings.ReplaceAll(name, "\u2215", "/")
}

// GET /api/basicFolders/GetFolderTemplates
func (h *Handlers) GetFolderTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{"templates": []interface{}{}})
}

// POST /api/basicFolders/CreateRootFolder
// Creates a new root volume/space.
// legacy DMS: ?SessionID=...&Foldername=...&Aktz=...&FolderType=...
func (h *Handlers) CreateRootFolder(w http.ResponseWriter, r *http.Request) {
	r = withCS3Token(r)

	folderName := r.URL.Query().Get("Foldername")
	if folderName == "" {
		writeError(w, 400, "INVALID_REQUEST", "Foldername required")
		return
	}

	aktz := r.URL.Query().Get("Aktz")

	if h.gw == nil || h.gw.Gateway == nil {
		writeError(w, 500, "INTERNAL_ERROR", "CS3 gateway not configured")
		return
	}

	// Create a new project space via CS3
	sess := sessionFromCtx(r.Context())
	var owner *userpb.User
	if sess != nil {
		owner = sess.User
	}

	res, err := h.gw.Gateway.CreateStorageSpace(r.Context(), &provider.CreateStorageSpaceRequest{
		Type:  "project",
		Name:  folderName,
		Owner: owner,
	})
	if err != nil {
		log.Error().Err(err).Str("name", folderName).Msg("cs3 create space failed")
		writeError(w, 500, "INTERNAL_ERROR", "Create root folder failed")
		return
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "INTERNAL_ERROR", res.Status.Message)
		return
	}

	spaceID := ""
	if res.StorageSpace != nil && res.StorageSpace.Root != nil {
		spaceID = encodeObjectID(res.StorageSpace.Root)
	}

	// Set Aktz metadata if provided
	if aktz != "" && res.StorageSpace != nil && res.StorageSpace.Root != nil {
		h.gw.Gateway.SetArbitraryMetadata(r.Context(), &provider.SetArbitraryMetadataRequest{
			Ref: &provider.Reference{ResourceId: res.StorageSpace.Root},
			ArbitraryMetadata: &provider.ArbitraryMetadata{
				Metadata: map[string]string{
					"oy.fileReference": aktz,
				},
			},
		})
	}

	log.Info().Str("name", folderName).Str("aktz", aktz).Str("id", spaceID).Msg("root folder created")

	writeJSON(w, 200, map[string]interface{}{
		"FolderInfo": map[string]interface{}{
			"FolderID":   spaceID,
			"Foldername": folderName,
			"Aktz":       aktz,
		},
		"TaskLog": taskLogOK(),
	})
}
