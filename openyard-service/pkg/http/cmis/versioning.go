package cmis

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/rs/zerolog/log"
)

// GetAllVersions handles cmisselector=versions — returns full version history.
func (h *Handler) GetAllVersions(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	// Stat current version
	statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{
		Ref:                   ref,
		ArbitraryMetadataKeys: []string{"*"},
	})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "objectNotFound", "Object not found")
		return
	}
	if statRes.Info.Type != provider.ResourceType_RESOURCE_TYPE_FILE {
		writeError(w, 400, "constraint", "Versions only available for documents")
		return
	}

	// List all versions from CS3
	versRes, err := h.gw.Gateway.ListFileVersions(r.Context(), &provider.ListFileVersionsRequest{Ref: ref})
	if err != nil {
		log.Error().Err(err).Msg("cmis: list versions failed")
		// Return just the current version
		writeJSON(w, 200, []ObjectEntry{{Object: mapResourceToObject(statRes.Info)}})
		return
	}

	objects := make([]ObjectEntry, 0, len(versRes.Versions)+1)

	// Current version (latest)
	currentObj := mapResourceToObject(statRes.Info)
	currentObj.Properties["cmis:isLatestVersion"] = Property{
		ID: "cmis:isLatestVersion", Type: PropertyTypeBoolean, Value: true,
	}
	objects = append(objects, ObjectEntry{Object: currentObj})

	// Historical versions
	if versRes.Status.Code == rpc.Code_CODE_OK {
		for i, v := range versRes.Versions {
			vObj := mapVersionToObject(statRes.Info, v, i+1, len(versRes.Versions))
			objects = append(objects, ObjectEntry{Object: vObj})
		}
	}

	writeJSON(w, 200, objects)
}

// CheckOut handles cmisaction=checkOut — locks the document for private working copy.
// Maps to CS3 SetLock with an exclusive write lock.
func (h *Handler) CheckOut(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	// Verify it's a document
	statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "objectNotFound", "Object not found")
		return
	}
	if statRes.Info.Type != provider.ResourceType_RESOURCE_TYPE_FILE {
		writeError(w, 400, "constraint", "Only documents can be checked out")
		return
	}

	// Check if already locked
	lockRes, err := h.gw.Gateway.GetLock(r.Context(), &provider.GetLockRequest{Ref: ref})
	if err == nil && lockRes.Status.Code == rpc.Code_CODE_OK && lockRes.Lock != nil {
		writeError(w, 409, "versioning", "Document is already checked out")
		return
	}

	// Set exclusive lock
	sess := sessionFromCtx(r.Context())
	lockID := generateLockID()
	lock := &provider.Lock{
		LockId: lockID,
		Type:   provider.LockType_LOCK_TYPE_EXCL,
	}
	if sess != nil && sess.User != nil {
		lock.User = sess.User.Id
	}

	setRes, err := h.gw.Gateway.SetLock(r.Context(), &provider.SetLockRequest{
		Ref:  ref,
		Lock: lock,
	})
	if err != nil {
		log.Error().Err(err).Msg("cmis: set lock failed")
		writeError(w, 500, "runtime", "Cannot check out document")
		return
	}
	if setRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "runtime", setRes.Status.Message)
		return
	}

	// Mark as checked out in metadata
	h.gw.Gateway.SetArbitraryMetadata(r.Context(), &provider.SetArbitraryMetadataRequest{
		Ref: ref,
		ArbitraryMetadata: &provider.ArbitraryMetadata{
			Metadata: map[string]string{
				"oy.checkedOut":     "true",
				"oy.checkedOutDate": time.Now().UTC().Format(time.RFC3339),
				"oy.lockId":         lockID,
			},
		},
	})

	// Return the checked-out object (PWC = Private Working Copy)
	obj := mapResourceToObject(statRes.Info)
	obj.Properties["cmis:isVersionSeriesCheckedOut"] = Property{
		ID: "cmis:isVersionSeriesCheckedOut", Type: PropertyTypeBoolean, Value: true,
	}
	if sess != nil {
		obj.Properties["cmis:versionSeriesCheckedOutBy"] = Property{
			ID: "cmis:versionSeriesCheckedOutBy", Type: PropertyTypeString, Value: sess.Login,
		}
	}
	obj.Properties["cmis:versionSeriesCheckedOutId"] = Property{
		ID: "cmis:versionSeriesCheckedOutId", Type: PropertyTypeID,
		Value: obj.Properties["cmis:objectId"].Value,
	}

	writeJSON(w, 200, obj)
}

// CancelCheckOut handles cmisaction=cancelCheckOut — releases the lock without creating a new version.
func (h *Handler) CancelCheckOut(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	if err := h.unlockDocument(r, ref); err != nil {
		writeError(w, 500, "runtime", "Cannot cancel check out")
		return
	}
	w.WriteHeader(204)
}

// CheckIn handles cmisaction=checkIn — uploads new version content and releases the lock.
func (h *Handler) CheckIn(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	// Upload new content if provided
	file, _, fileErr := r.FormFile("content")
	if fileErr == nil {
		defer file.Close()

		uploadRes, err := h.gw.Gateway.InitiateFileUpload(r.Context(), &provider.InitiateFileUploadRequest{Ref: ref})
		if err != nil || uploadRes.Status.Code != rpc.Code_CODE_OK || len(uploadRes.Protocols) == 0 {
			writeError(w, 500, "runtime", "Cannot upload new version")
			return
		}
		if err := h.uploadContent(r, uploadRes.Protocols[0], file); err != nil {
			writeError(w, 500, "runtime", "Upload failed")
			return
		}
	}

	// Set checkin comment if provided
	checkinComment := r.FormValue("checkinComment")
	if checkinComment != "" {
		h.gw.Gateway.SetArbitraryMetadata(r.Context(), &provider.SetArbitraryMetadataRequest{
			Ref: ref,
			ArbitraryMetadata: &provider.ArbitraryMetadata{
				Metadata: map[string]string{
					"oy.checkinComment": checkinComment,
				},
			},
		})
	}

	// Set properties from form
	h.setPropertiesFromForm(r, ref)

	// Unlock
	if err := h.unlockDocument(r, ref); err != nil {
		log.Warn().Err(err).Msg("cmis: unlock after checkin failed (continuing)")
	}

	// Return updated object
	h.GetObject(w, r, ref)
}

// unlockDocument removes the lock and clears checkout metadata.
func (h *Handler) unlockDocument(r *http.Request, ref *provider.Reference) error {
	// Get current lock to obtain lockId
	lockRes, err := h.gw.Gateway.GetLock(r.Context(), &provider.GetLockRequest{Ref: ref})
	if err != nil || lockRes.Status.Code != rpc.Code_CODE_OK || lockRes.Lock == nil {
		// No lock found — just clear metadata
		h.clearCheckoutMetadata(r, ref)
		return nil
	}

	unlockRes, err := h.gw.Gateway.Unlock(r.Context(), &provider.UnlockRequest{
		Ref:  ref,
		Lock: lockRes.Lock,
	})
	if err != nil {
		return err
	}
	if unlockRes.Status.Code != rpc.Code_CODE_OK {
		return fmt.Errorf("unlock failed: %s", unlockRes.Status.Message)
	}

	h.clearCheckoutMetadata(r, ref)
	return nil
}

func (h *Handler) clearCheckoutMetadata(r *http.Request, ref *provider.Reference) {
	h.gw.Gateway.UnsetArbitraryMetadata(r.Context(), &provider.UnsetArbitraryMetadataRequest{
		Ref:                   ref,
		ArbitraryMetadataKeys: []string{"oy.checkedOut", "oy.checkedOutDate", "oy.lockId"},
	})
}

// mapVersionToObject creates a CMIS object from a CS3 FileVersion.
func mapVersionToObject(current *provider.ResourceInfo, v *provider.FileVersion, idx, total int) Object {
	objectID := encodeObjectID(current.Id)
	versionLabel := fmt.Sprintf("%d.0", total-idx)

	props := map[string]Property{
		"cmis:objectId": {
			ID: "cmis:objectId", Type: PropertyTypeID,
			Value: objectID + ";v" + v.Key,
		},
		"cmis:baseTypeId": {
			ID: "cmis:baseTypeId", Type: PropertyTypeID, Value: BaseTypeDocument,
		},
		"cmis:objectTypeId": {
			ID: "cmis:objectTypeId", Type: PropertyTypeID, Value: BaseTypeDocument,
		},
		"cmis:name": {
			ID: "cmis:name", Type: PropertyTypeString,
			Value: current.Name,
		},
		"cmis:versionLabel": {
			ID: "cmis:versionLabel", Type: PropertyTypeString, Value: versionLabel,
		},
		"cmis:isLatestVersion": {
			ID: "cmis:isLatestVersion", Type: PropertyTypeBoolean, Value: false,
		},
		"cmis:isMajorVersion": {
			ID: "cmis:isMajorVersion", Type: PropertyTypeBoolean, Value: true,
		},
		"cmis:isLatestMajorVersion": {
			ID: "cmis:isLatestMajorVersion", Type: PropertyTypeBoolean, Value: false,
		},
		"cmis:contentStreamLength": {
			ID: "cmis:contentStreamLength", Type: PropertyTypeInteger, Value: v.Size,
		},
		"cmis:isVersionSeriesCheckedOut": {
			ID: "cmis:isVersionSeriesCheckedOut", Type: PropertyTypeBoolean, Value: false,
		},
	}

	if v.Mtime > 0 {
		ts := time.Unix(int64(v.Mtime), 0).UTC().Format(time.RFC3339)
		props["cmis:lastModificationDate"] = Property{
			ID: "cmis:lastModificationDate", Type: PropertyTypeDateTime, Value: ts,
		}
	}
	if v.Etag != "" {
		props["cmis:changeToken"] = Property{
			ID: "cmis:changeToken", Type: PropertyTypeString, Value: v.Etag,
		}
	}

	return Object{Properties: props}
}

func generateLockID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("cmis-lock-%x", b)
}
