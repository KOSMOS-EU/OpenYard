package cmis

import (
	"encoding/base64"
	"fmt"
	"path"
	"strings"
	"time"

	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	typesv1 "github.com/cs3org/go-cs3apis/cs3/types/v1beta1"
)

// --- ObjectID encoding (shared with legacy handler logic) ---

func encodeObjectID(rid *provider.ResourceId) string {
	if rid == nil {
		return ""
	}
	raw := rid.StorageId + "$" + rid.SpaceId + "!" + rid.OpaqueId
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(raw))
}

func decodeObjectID(objectID string) (*provider.ResourceId, error) {
	raw, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(objectID)
	if err != nil {
		return nil, fmt.Errorf("invalid objectId encoding: %w", err)
	}
	s := string(raw)

	dollarIdx := strings.Index(s, "$")
	bangIdx := strings.Index(s, "!")
	if bangIdx < 0 {
		return nil, fmt.Errorf("invalid objectId format")
	}

	rid := &provider.ResourceId{}
	if dollarIdx >= 0 && dollarIdx < bangIdx {
		rid.StorageId = s[:dollarIdx]
		rid.SpaceId = s[dollarIdx+1 : bangIdx]
		rid.OpaqueId = s[bangIdx+1:]
	} else {
		rid.StorageId = s[:bangIdx]
		rid.OpaqueId = s[bangIdx+1:]
	}
	return rid, nil
}

// --- CS3 ResourceInfo → CMIS Object ---

func mapResourceToObject(info *provider.ResourceInfo) Object {
	props := make(map[string]Property)

	objectID := encodeObjectID(info.Id)
	displayName := path.Base(info.Path)

	// cmis:objectId
	props["cmis:objectId"] = Property{
		ID: "cmis:objectId", DisplayName: "Object ID",
		Type: PropertyTypeID, Value: objectID,
	}

	// cmis:baseTypeId
	baseType := BaseTypeFolder
	if info.Type == provider.ResourceType_RESOURCE_TYPE_FILE {
		baseType = BaseTypeDocument
	}
	props["cmis:baseTypeId"] = Property{
		ID: "cmis:baseTypeId", DisplayName: "Base Type ID",
		Type: PropertyTypeID, Value: baseType,
	}

	// cmis:objectTypeId (same as base type for now)
	props["cmis:objectTypeId"] = Property{
		ID: "cmis:objectTypeId", DisplayName: "Object Type ID",
		Type: PropertyTypeID, Value: baseType,
	}

	// cmis:name
	props["cmis:name"] = Property{
		ID: "cmis:name", DisplayName: "Name",
		Type: PropertyTypeString, Value: displayName,
	}

	// cmis:path (folders only, per spec)
	if info.Type == provider.ResourceType_RESOURCE_TYPE_CONTAINER {
		props["cmis:path"] = Property{
			ID: "cmis:path", DisplayName: "Path",
			Type: PropertyTypeString, Value: info.Path,
		}
	}

	// cmis:createdBy / cmis:lastModifiedBy
	if info.Owner != nil {
		props["cmis:createdBy"] = Property{
			ID: "cmis:createdBy", DisplayName: "Created By",
			Type: PropertyTypeString, Value: info.Owner.OpaqueId,
		}
		props["cmis:lastModifiedBy"] = Property{
			ID: "cmis:lastModifiedBy", DisplayName: "Last Modified By",
			Type: PropertyTypeString, Value: info.Owner.OpaqueId,
		}
	}

	// cmis:creationDate / cmis:lastModificationDate
	if info.Mtime != nil {
		ts := formatTimestamp(info.Mtime)
		props["cmis:creationDate"] = Property{
			ID: "cmis:creationDate", DisplayName: "Creation Date",
			Type: PropertyTypeDateTime, Value: ts,
		}
		props["cmis:lastModificationDate"] = Property{
			ID: "cmis:lastModificationDate", DisplayName: "Last Modification Date",
			Type: PropertyTypeDateTime, Value: ts,
		}
	}

	// cmis:parentId
	if info.ParentId != nil {
		props["cmis:parentId"] = Property{
			ID: "cmis:parentId", DisplayName: "Parent ID",
			Type: PropertyTypeID, Value: encodeObjectID(info.ParentId),
		}
	}

	// Document-specific properties
	if info.Type == provider.ResourceType_RESOURCE_TYPE_FILE {
		props["cmis:contentStreamLength"] = Property{
			ID: "cmis:contentStreamLength", DisplayName: "Content Stream Length",
			Type: PropertyTypeInteger, Value: info.Size,
		}
		props["cmis:contentStreamMimeType"] = Property{
			ID: "cmis:contentStreamMimeType", DisplayName: "Content Stream MIME Type",
			Type: PropertyTypeString, Value: info.MimeType,
		}
		props["cmis:contentStreamFileName"] = Property{
			ID: "cmis:contentStreamFileName", DisplayName: "Content Stream File Name",
			Type: PropertyTypeString, Value: displayName,
		}
		props["cmis:isLatestVersion"] = Property{
			ID: "cmis:isLatestVersion", DisplayName: "Is Latest Version",
			Type: PropertyTypeBoolean, Value: true,
		}
		props["cmis:isMajorVersion"] = Property{
			ID: "cmis:isMajorVersion", DisplayName: "Is Major Version",
			Type: PropertyTypeBoolean, Value: true,
		}
		props["cmis:isLatestMajorVersion"] = Property{
			ID: "cmis:isLatestMajorVersion", DisplayName: "Is Latest Major Version",
			Type: PropertyTypeBoolean, Value: true,
		}
		props["cmis:versionLabel"] = Property{
			ID: "cmis:versionLabel", DisplayName: "Version Label",
			Type: PropertyTypeString, Value: "1.0",
		}
		props["cmis:isVersionSeriesCheckedOut"] = Property{
			ID: "cmis:isVersionSeriesCheckedOut", DisplayName: "Is Checked Out",
			Type: PropertyTypeBoolean, Value: false,
		}
	}

	// cmis:changeToken (etag)
	if info.Etag != "" {
		props["cmis:changeToken"] = Property{
			ID: "cmis:changeToken", DisplayName: "Change Token",
			Type: PropertyTypeString, Value: info.Etag,
		}
	}

	// Arbitrary metadata → custom properties
	if info.ArbitraryMetadata != nil {
		for k, v := range info.ArbitraryMetadata.Metadata {
			props[k] = Property{
				ID: k, DisplayName: k,
				Type: PropertyTypeString, Value: v,
			}
		}
	}

	return Object{
		Properties:       props,
		AllowableActions: mapAllowableActions(info),
	}
}

// mapAllowableActions converts CS3 PermissionSet to CMIS AllowableActions.
func mapAllowableActions(info *provider.ResourceInfo) map[string]bool {
	p := info.PermissionSet
	isFolder := info.Type == provider.ResourceType_RESOURCE_TYPE_CONTAINER
	isDoc := info.Type == provider.ResourceType_RESOURCE_TYPE_FILE

	actions := map[string]bool{
		"canGetProperties":       true,
		"canGetObjectParents":    true,
	}

	if p != nil {
		actions["canUpdateProperties"] = p.InitiateFileUpload || p.CreateContainer
		actions["canDeleteObject"] = p.Delete
		actions["canMoveObject"] = p.Move
		actions["canGetAllVersions"] = p.Stat && isDoc
		actions["canSetContentStream"] = p.InitiateFileUpload && isDoc
		actions["canGetContentStream"] = p.InitiateFileDownload && isDoc
		actions["canDeleteContentStream"] = p.Delete && isDoc
		actions["canCreateDocument"] = p.InitiateFileUpload && isFolder
		actions["canCreateFolder"] = p.CreateContainer && isFolder
		actions["canGetChildren"] = p.ListContainer && isFolder
		actions["canGetFolderParent"] = isFolder
		actions["canAddObjectToFolder"] = p.Move && isFolder
		actions["canRemoveObjectFromFolder"] = p.Move && isFolder
	} else {
		// No permission info — allow read operations
		actions["canGetContentStream"] = isDoc
		actions["canGetAllVersions"] = isDoc
		actions["canGetChildren"] = isFolder
		actions["canGetFolderParent"] = isFolder
	}

	return actions
}

// mapSpaceToObject converts a CS3 StorageSpace to a CMIS folder object (repository root).
func mapSpaceToObject(space *provider.StorageSpace) Object {
	rootID := encodeObjectID(space.Root)

	props := map[string]Property{
		"cmis:objectId": {
			ID: "cmis:objectId", DisplayName: "Object ID",
			Type: PropertyTypeID, Value: rootID,
		},
		"cmis:baseTypeId": {
			ID: "cmis:baseTypeId", DisplayName: "Base Type ID",
			Type: PropertyTypeID, Value: BaseTypeFolder,
		},
		"cmis:objectTypeId": {
			ID: "cmis:objectTypeId", DisplayName: "Object Type ID",
			Type: PropertyTypeID, Value: BaseTypeFolder,
		},
		"cmis:name": {
			ID: "cmis:name", DisplayName: "Name",
			Type: PropertyTypeString, Value: space.Name,
		},
		"cmis:path": {
			ID: "cmis:path", DisplayName: "Path",
			Type: PropertyTypeString, Value: "/",
		},
	}

	if space.Mtime != nil {
		ts := formatTimestamp(space.Mtime)
		props["cmis:creationDate"] = Property{
			ID: "cmis:creationDate", DisplayName: "Creation Date",
			Type: PropertyTypeDateTime, Value: ts,
		}
		props["cmis:lastModificationDate"] = Property{
			ID: "cmis:lastModificationDate", DisplayName: "Last Modification Date",
			Type: PropertyTypeDateTime, Value: ts,
		}
	}

	return Object{
		Properties: props,
		AllowableActions: map[string]bool{
			"canGetProperties":    true,
			"canGetChildren":      true,
			"canCreateDocument":   true,
			"canCreateFolder":     true,
			"canGetFolderParent":  false, // root has no parent
			"canGetObjectParents": false,
		},
	}
}

func formatTimestamp(ts *typesv1.Timestamp) string {
	if ts == nil {
		return ""
	}
	return time.Unix(int64(ts.Seconds), int64(ts.Nanos)).UTC().Format(time.RFC3339)
}

// refFromObjectID creates a CS3 Reference from an encoded object ID.
func refFromObjectID(objectID string) (*provider.Reference, error) {
	rid, err := decodeObjectID(objectID)
	if err != nil {
		return nil, err
	}
	return &provider.Reference{ResourceId: rid, Path: "."}, nil
}
