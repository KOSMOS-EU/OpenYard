package cmis

import (
	"net/http"
	"strconv"
	"strings"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/rs/zerolog/log"
)

// GetObject handles cmisselector=object — returns a single CMIS object.
func (h *Handler) GetObject(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	res, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{
		Ref:                   ref,
		ArbitraryMetadataKeys: []string{"*"},
	})
	if err != nil {
		log.Error().Err(err).Msg("cmis: stat failed")
		writeError(w, 500, "runtime", "Storage error")
		return
	}
	if res.Status.Code == rpc.Code_CODE_NOT_FOUND {
		writeError(w, 404, "objectNotFound", "Object not found")
		return
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "runtime", res.Status.Message)
		return
	}

	writeJSON(w, 200, mapResourceToObject(res.Info))
}

// CreateDocument handles cmisaction=createDocument.
// Expects multipart/form-data with propertyValue[cmis:name] and optional file content.
func (h *Handler) CreateDocument(w http.ResponseWriter, r *http.Request, parentRef *provider.Reference) {
	name := cmisPropertyFromForm(r, "cmis:name")
	if name == "" {
		writeError(w, 400, "invalidArgument", "cmis:name is required")
		return
	}

	// Stat parent to get path
	parentStat, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: parentRef})
	if err != nil || parentStat.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "objectNotFound", "Parent folder not found")
		return
	}

	// Build reference for new file
	newRef := &provider.Reference{
		ResourceId: parentStat.Info.Id,
		Path:       "./" + name,
	}

	// Initiate upload to create the file
	uploadRes, err := h.gw.Gateway.InitiateFileUpload(r.Context(), &provider.InitiateFileUploadRequest{Ref: newRef})
	if err != nil {
		log.Error().Err(err).Msg("cmis: initiate upload failed")
		writeError(w, 500, "runtime", "Cannot create document")
		return
	}
	if uploadRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "runtime", uploadRes.Status.Message)
		return
	}

	if len(uploadRes.Protocols) == 0 {
		writeError(w, 500, "runtime", "No upload protocol available")
		return
	}

	// Upload content if provided
	file, _, fileErr := r.FormFile("content")
	if fileErr == nil {
		defer file.Close()
		if err := h.uploadContent(r, uploadRes.Protocols[0], file); err != nil {
			writeError(w, 500, "runtime", "Upload failed")
			return
		}
	}

	// Set additional properties as metadata
	h.setPropertiesFromForm(r, newRef)

	// Stat the new object
	statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: newRef})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK {
		// File was created but stat failed — return minimal response
		writeJSON(w, 201, Object{
			Properties: map[string]Property{
				"cmis:name": {ID: "cmis:name", Type: PropertyTypeString, Value: name},
			},
		})
		return
	}

	w.WriteHeader(201)
	writeJSON(w, 201, mapResourceToObject(statRes.Info))
}

// CreateFolder handles cmisaction=createFolder.
func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request, parentRef *provider.Reference) {
	name := cmisPropertyFromForm(r, "cmis:name")
	if name == "" {
		writeError(w, 400, "invalidArgument", "cmis:name is required")
		return
	}

	// Build reference for new folder
	newRef := &provider.Reference{
		ResourceId: parentRef.ResourceId,
		Path:       "./" + name,
	}

	// If parentRef is path-based, resolve to ResourceId
	if parentRef.ResourceId == nil && parentRef.Path != "" {
		parentStat, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: parentRef})
		if err != nil || parentStat.Status.Code != rpc.Code_CODE_OK {
			writeError(w, 404, "objectNotFound", "Parent folder not found")
			return
		}
		newRef = &provider.Reference{
			ResourceId: parentStat.Info.Id,
			Path:       "./" + name,
		}
	}

	res, err := h.gw.Gateway.CreateContainer(r.Context(), &provider.CreateContainerRequest{Ref: newRef})
	if err != nil {
		log.Error().Err(err).Msg("cmis: create folder failed")
		writeError(w, 500, "runtime", "Cannot create folder")
		return
	}
	if res.Status.Code == rpc.Code_CODE_ALREADY_EXISTS {
		writeError(w, 409, "contentAlreadyExists", "Folder already exists")
		return
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "runtime", res.Status.Message)
		return
	}

	// Set additional properties as metadata
	h.setPropertiesFromForm(r, newRef)

	// Stat the new folder
	statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: newRef})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK {
		writeJSON(w, 201, Object{
			Properties: map[string]Property{
				"cmis:name": {ID: "cmis:name", Type: PropertyTypeString, Value: name},
			},
		})
		return
	}

	writeJSON(w, 201, mapResourceToObject(statRes.Info))
}

// UpdateProperties handles cmisaction=update.
func (h *Handler) UpdateProperties(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	// Collect properties from form
	meta := extractPropertiesFromForm(r)
	if len(meta) == 0 {
		writeError(w, 400, "invalidArgument", "No properties to update")
		return
	}

	// Handle rename (cmis:name)
	if newName, ok := meta["cmis:name"]; ok {
		delete(meta, "cmis:name")
		h.renameObject(r, ref, newName)
	}

	// Set remaining properties as arbitrary metadata
	if len(meta) > 0 {
		metaRes, err := h.gw.Gateway.SetArbitraryMetadata(r.Context(), &provider.SetArbitraryMetadataRequest{
			Ref: ref,
			ArbitraryMetadata: &provider.ArbitraryMetadata{Metadata: meta},
		})
		if err != nil || metaRes.Status.Code != rpc.Code_CODE_OK {
			writeError(w, 500, "runtime", "Cannot update properties")
			return
		}
	}

	// Return updated object
	h.GetObject(w, r, ref)
}

// DeleteObject handles cmisaction=delete.
func (h *Handler) DeleteObject(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	res, err := h.gw.Gateway.Delete(r.Context(), &provider.DeleteRequest{Ref: ref})
	if err != nil {
		log.Error().Err(err).Msg("cmis: delete failed")
		writeError(w, 500, "runtime", "Cannot delete object")
		return
	}
	if res.Status.Code == rpc.Code_CODE_NOT_FOUND {
		writeError(w, 404, "objectNotFound", "Object not found")
		return
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "runtime", res.Status.Message)
		return
	}

	w.WriteHeader(204)
}

// DeleteTree handles cmisaction=deleteTree on a folder.
func (h *Handler) DeleteTree(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	// CS3 Delete on a container is recursive
	h.DeleteObject(w, r, ref)
}

// MoveObject handles cmisaction=move — moves an object to a new folder.
// Expects form fields: targetFolderId, sourceFolderId (optional).
func (h *Handler) MoveObject(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	targetFolderID := r.FormValue("targetFolderId")
	if targetFolderID == "" {
		writeError(w, 400, "invalidArgument", "targetFolderId is required")
		return
	}

	// Stat source to get name
	statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "objectNotFound", "Source object not found")
		return
	}

	// Resolve target folder
	targetRef, err := refFromObjectID(targetFolderID)
	if err != nil {
		writeError(w, 400, "invalidArgument", "Invalid targetFolderId")
		return
	}
	targetStat, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: targetRef})
	if err != nil || targetStat.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "objectNotFound", "Target folder not found")
		return
	}

	// Build destination reference
	dstRef := &provider.Reference{
		ResourceId: targetStat.Info.Id,
		Path:       "./" + statRes.Info.Name,
	}

	moveRes, err := h.gw.Gateway.Move(r.Context(), &provider.MoveRequest{
		Source:      ref,
		Destination: dstRef,
	})
	if err != nil {
		log.Error().Err(err).Msg("cmis: move failed")
		writeError(w, 500, "runtime", "Move failed")
		return
	}
	if moveRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "runtime", moveRes.Status.Message)
		return
	}

	// Return moved object
	newStat, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: dstRef})
	if err != nil || newStat.Status.Code != rpc.Code_CODE_OK {
		writeJSON(w, 200, mapResourceToObject(statRes.Info))
		return
	}
	writeJSON(w, 200, mapResourceToObject(newStat.Info))
}

// DeleteContentStream handles cmisaction=deleteContent — removes file content.
// CS3 doesn't support deleting content without deleting the file,
// so we upload an empty body to effectively clear content.
func (h *Handler) DeleteContentStream(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	uploadRes, err := h.gw.Gateway.InitiateFileUpload(r.Context(), &provider.InitiateFileUploadRequest{Ref: ref})
	if err != nil || uploadRes.Status.Code != rpc.Code_CODE_OK || len(uploadRes.Protocols) == 0 {
		writeError(w, 500, "runtime", "Cannot delete content stream")
		return
	}

	// Upload empty content
	if err := h.uploadContent(r, uploadRes.Protocols[0], strings.NewReader("")); err != nil {
		writeError(w, 500, "runtime", "Failed to clear content")
		return
	}

	w.WriteHeader(204)
}

// GetObjectOfLatestVersion handles cmisselector=object with versionSeriesId.
// Since CS3 always returns the latest version on Stat, this is equivalent to GetObject.
func (h *Handler) GetObjectOfLatestVersion(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	h.GetObject(w, r, ref)
}

// --- helpers ---

func (h *Handler) renameObject(r *http.Request, ref *provider.Reference, newName string) {
	statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK {
		return
	}
	if statRes.Info.ParentId == nil {
		return
	}
	dstRef := &provider.Reference{
		ResourceId: statRes.Info.ParentId,
		Path:       "./" + newName,
	}
	h.gw.Gateway.Move(r.Context(), &provider.MoveRequest{Source: ref, Destination: dstRef})
}

// cmisPropertyFromForm extracts a CMIS property value from form data.
// CMIS Browser Binding uses: propertyId[0]=cmis:name & propertyValue[0]=filename.txt
// or the shorthand: cmisaction=...&propertyValue[cmis:name]=filename.txt
func cmisPropertyFromForm(r *http.Request, propID string) string {
	if r.Form == nil {
		r.ParseMultipartForm(32 << 20)
	}

	// Direct form: propertyValue[cmis:name]=...
	if v := r.FormValue("propertyValue[" + propID + "]"); v != "" {
		return v
	}

	// Indexed form: propertyId[0]=cmis:name, propertyValue[0]=...
	for i := 0; i < 50; i++ {
		idx := strconv.Itoa(i)
		if r.FormValue("propertyId["+idx+"]") == propID {
			return r.FormValue("propertyValue[" + idx + "]")
		}
		if r.FormValue("propertyId["+idx+"]") == "" {
			break
		}
	}

	return ""
}

// extractPropertiesFromForm collects all CMIS properties from form data.
func extractPropertiesFromForm(r *http.Request) map[string]string {
	if r.Form == nil {
		r.ParseMultipartForm(32 << 20)
	}

	props := make(map[string]string)

	// Direct form: propertyValue[cmis:xxx]=...
	for key, values := range r.Form {
		if strings.HasPrefix(key, "propertyValue[") && strings.HasSuffix(key, "]") {
			propID := key[len("propertyValue[") : len(key)-1]
			if len(values) > 0 {
				props[propID] = values[0]
			}
		}
	}

	// Indexed form: propertyId[0]=..., propertyValue[0]=...
	for i := 0; i < 50; i++ {
		idx := strconv.Itoa(i)
		propID := r.FormValue("propertyId[" + idx + "]")
		if propID == "" {
			break
		}
		propVal := r.FormValue("propertyValue[" + idx + "]")
		props[propID] = propVal
	}

	return props
}

// setPropertiesFromForm sets CMIS properties as CS3 arbitrary metadata (excluding cmis:* system props).
func (h *Handler) setPropertiesFromForm(r *http.Request, ref *provider.Reference) {
	props := extractPropertiesFromForm(r)
	meta := make(map[string]string)
	for k, v := range props {
		if strings.HasPrefix(k, "cmis:") {
			continue // skip system properties
		}
		meta[k] = v
	}
	if len(meta) == 0 {
		return
	}

	h.gw.Gateway.SetArbitraryMetadata(r.Context(), &provider.SetArbitraryMetadataRequest{
		Ref:               ref,
		ArbitraryMetadata: &provider.ArbitraryMetadata{Metadata: meta},
	})
}

