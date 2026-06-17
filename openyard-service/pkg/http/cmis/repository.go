package cmis

import (
	"fmt"
	"net/http"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/rs/zerolog/log"
)

// GetRepositories handles GET /cmis — returns all repositories (one per storage space).
func (h *Handler) GetRepositories(w http.ResponseWriter, r *http.Request) {
	r = h.withCS3Token(r)

	spaces, err := h.listSpaces(r)
	if err != nil {
		writeError(w, 500, "runtime", "Cannot list repositories")
		return
	}

	repos := make(map[string]RepositoryInfo, len(spaces))
	for _, space := range spaces {
		repoID := encodeObjectID(space.Root)
		repos[repoID] = h.buildRepositoryInfo(space)
	}

	writeJSON(w, 200, repos)
}

// GetRepositoryInfo handles GET /cmis/{repoId}?cmisselector=repositoryInfo
func (h *Handler) GetRepositoryInfo(w http.ResponseWriter, r *http.Request, repoID string) {
	r = h.withCS3Token(r)

	space, err := h.findSpace(r, repoID)
	if err != nil {
		writeError(w, 404, "objectNotFound", "Repository not found")
		return
	}

	writeJSON(w, 200, h.buildRepositoryInfo(space))
}

// GetTypeDefinition handles GET /cmis/{repoId}?cmisselector=typeDefinition&typeId=...
func (h *Handler) GetTypeDefinition(w http.ResponseWriter, r *http.Request) {
	typeID := r.URL.Query().Get("typeId")
	td := LookupType(typeID)
	if td == nil {
		writeError(w, 404, "objectNotFound", "Type not found: "+typeID)
		return
	}
	writeJSON(w, 200, td)
}

// GetTypeChildren handles GET /cmis/{repoId}?cmisselector=typeChildren
func (h *Handler) GetTypeChildren(w http.ResponseWriter, r *http.Request) {
	parentID := r.URL.Query().Get("typeId")

	var types []TypeDefinition
	switch parentID {
	case "":
		// Root: return all base types
		types = []TypeDefinition{
			*builtinType(BaseTypeDocument),
			*builtinType(BaseTypeFolder),
			*builtinType(BaseTypeRelationship),
			*builtinType(BaseTypePolicy),
			*builtinType(BaseTypeItem),
			{
				ID: BaseTypeSecondary, LocalName: "secondary",
				DisplayName: "Secondary Type", QueryName: "cmis:secondary",
				BaseID: BaseTypeSecondary, Queryable: true,
			},
		}
	case BaseTypeSecondary:
		// Children of cmis:secondary → OpenYard secondary types
		types = SecondaryTypeDefinitions()
	default:
		types = []TypeDefinition{}
	}

	writeJSON(w, 200, TypeDefinitionList{
		Types:        types,
		HasMoreItems: false,
		NumItems:     len(types),
	})
}

// GetTypeDescendants handles GET /cmis/{repoId}?cmisselector=typeDescendants
func (h *Handler) GetTypeDescendants(w http.ResponseWriter, r *http.Request) {
	// Return all types as flat list
	writeJSON(w, 200, AllTypeDefinitions())
}

// --- helpers ---

func (h *Handler) buildRepositoryInfo(space *provider.StorageSpace) RepositoryInfo {
	repoID := encodeObjectID(space.Root)
	return RepositoryInfo{
		RepositoryID:         repoID,
		RepositoryName:       space.Name,
		VendorName:           "OpenYard",
		ProductName:          "OpenYard DMS-Adapter",
		ProductVersion:       "1.0",
		RootFolderID:         repoID,
		CMISVersionSupported: "1.1",
		RepositoryURL:        h.baseURL + "/cmis/" + repoID,
		RootFolderURL:        h.baseURL + "/cmis/" + repoID + "/root",
		Capabilities:         defaultCapabilities(),
		ACLCapability: ACLCapability{
			SupportedPermissions: "basic",
			Propagation:          "repositorydetermined",
		},
	}
}

func defaultCapabilities() Capabilities {
	return Capabilities{
		CapabilityContentStreamUpdatability: "anytime",
		CapabilityChanges:                   "none",
		CapabilityRenditions:                "read",
		CapabilityGetDescendants:            true,
		CapabilityGetFolderTree:             true,
		CapabilityMultifiling:               false,
		CapabilityUnfiling:                  false,
		CapabilityVersionSpecificFiling:     false,
		CapabilityPWCSearchable:             false,
		CapabilityPWCUpdatable:              true,
		CapabilityAllVersionsSearchable:     false,
		CapabilityQuery:                     "bothcombined",
		CapabilityJoin:                      "none",
		CapabilityACL:                       "manage",
		CapabilityOrderBy:                   "common",
		CapabilityCreatablePropertyTypes: CreatablePropertyTypes{
			CanCreate: []string{"string", "integer", "boolean", "datetime"},
		},
		CapabilityNewTypeSettableAttributes: NewTypeSettableAttributes{},
	}
}

func (h *Handler) listSpaces(r *http.Request) ([]*provider.StorageSpace, error) {
	res, err := h.gw.Gateway.ListStorageSpaces(r.Context(), &provider.ListStorageSpacesRequest{})
	if err != nil {
		log.Error().Err(err).Msg("cmis: list spaces failed")
		return nil, err
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		return nil, fmt.Errorf("list spaces: %s", res.Status.Message)
	}
	return res.StorageSpaces, nil
}

func (h *Handler) findSpace(r *http.Request, repoID string) (*provider.StorageSpace, error) {
	spaces, err := h.listSpaces(r)
	if err != nil {
		return nil, err
	}
	for _, space := range spaces {
		if encodeObjectID(space.Root) == repoID {
			return space, nil
		}
	}
	return nil, fmt.Errorf("repository not found: %s", repoID)
}

// --- Built-in CMIS Type Definitions ---

func builtinType(typeID string) *TypeDefinition {
	switch typeID {
	case BaseTypeDocument:
		v := true
		return &TypeDefinition{
			ID:          BaseTypeDocument,
			LocalName:   "document",
			DisplayName: "Document",
			QueryName:   "cmis:document",
			BaseID:      BaseTypeDocument,
			Creatable:   true,
			Fileable:    true,
			Queryable:   true,
			ControllableACL: true,
			Versionable:     &v,
			ContentStreamAllowed: "allowed",
			PropertyDefinitions:  documentPropertyDefs(),
		}
	case BaseTypeFolder:
		return &TypeDefinition{
			ID:          BaseTypeFolder,
			LocalName:   "folder",
			DisplayName: "Folder",
			QueryName:   "cmis:folder",
			BaseID:      BaseTypeFolder,
			Creatable:   true,
			Fileable:    true,
			Queryable:   true,
			ControllableACL: true,
			PropertyDefinitions: folderPropertyDefs(),
		}
	case BaseTypeRelationship:
		return &TypeDefinition{
			ID:          BaseTypeRelationship,
			LocalName:   "relationship",
			DisplayName: "Relationship",
			QueryName:   "cmis:relationship",
			BaseID:      BaseTypeRelationship,
			Creatable:   false,
			Fileable:    false,
			Queryable:   false,
			PropertyDefinitions: relationshipPropertyDefs(),
		}
	case BaseTypePolicy:
		return &TypeDefinition{
			ID:          BaseTypePolicy,
			LocalName:   "policy",
			DisplayName: "Policy",
			QueryName:   "cmis:policy",
			BaseID:      BaseTypePolicy,
			Creatable:   false,
			Fileable:    false,
			Queryable:   false,
			PropertyDefinitions: commonPropertyDefs(),
		}
	case BaseTypeItem:
		return &TypeDefinition{
			ID:          BaseTypeItem,
			LocalName:   "item",
			DisplayName: "Item",
			QueryName:   "cmis:item",
			BaseID:      BaseTypeItem,
			Creatable:   false,
			Fileable:    false,
			Queryable:   false,
			PropertyDefinitions: commonPropertyDefs(),
		}
	default:
		return nil
	}
}

func relationshipPropertyDefs() map[string]PropertyDefinition {
	defs := commonPropertyDefs()
	defs["cmis:sourceId"] = PropertyDefinition{
		ID: "cmis:sourceId", LocalName: "sourceId", DisplayName: "Source ID",
		QueryName: "cmis:sourceId", PropertyType: PropertyTypeID,
		Cardinality: "single", Updatability: "oncreate", Required: true, Queryable: true,
	}
	defs["cmis:targetId"] = PropertyDefinition{
		ID: "cmis:targetId", LocalName: "targetId", DisplayName: "Target ID",
		QueryName: "cmis:targetId", PropertyType: PropertyTypeID,
		Cardinality: "single", Updatability: "oncreate", Required: true, Queryable: true,
	}
	return defs
}

func commonPropertyDefs() map[string]PropertyDefinition {
	return map[string]PropertyDefinition{
		"cmis:objectId": {
			ID: "cmis:objectId", LocalName: "objectId", DisplayName: "Object ID",
			QueryName: "cmis:objectId", PropertyType: PropertyTypeID,
			Cardinality: "single", Updatability: "readonly", Queryable: true, Orderable: true,
		},
		"cmis:baseTypeId": {
			ID: "cmis:baseTypeId", LocalName: "baseTypeId", DisplayName: "Base Type ID",
			QueryName: "cmis:baseTypeId", PropertyType: PropertyTypeID,
			Cardinality: "single", Updatability: "readonly", Queryable: true,
		},
		"cmis:objectTypeId": {
			ID: "cmis:objectTypeId", LocalName: "objectTypeId", DisplayName: "Object Type ID",
			QueryName: "cmis:objectTypeId", PropertyType: PropertyTypeID,
			Cardinality: "single", Updatability: "oncreate", Queryable: true,
		},
		"cmis:name": {
			ID: "cmis:name", LocalName: "name", DisplayName: "Name",
			QueryName: "cmis:name", PropertyType: PropertyTypeString,
			Cardinality: "single", Updatability: "readwrite", Required: true, Queryable: true, Orderable: true,
		},
		"cmis:createdBy": {
			ID: "cmis:createdBy", LocalName: "createdBy", DisplayName: "Created By",
			QueryName: "cmis:createdBy", PropertyType: PropertyTypeString,
			Cardinality: "single", Updatability: "readonly", Queryable: true, Orderable: true,
		},
		"cmis:creationDate": {
			ID: "cmis:creationDate", LocalName: "creationDate", DisplayName: "Creation Date",
			QueryName: "cmis:creationDate", PropertyType: PropertyTypeDateTime,
			Cardinality: "single", Updatability: "readonly", Queryable: true, Orderable: true,
		},
		"cmis:lastModifiedBy": {
			ID: "cmis:lastModifiedBy", LocalName: "lastModifiedBy", DisplayName: "Last Modified By",
			QueryName: "cmis:lastModifiedBy", PropertyType: PropertyTypeString,
			Cardinality: "single", Updatability: "readonly", Queryable: true,
		},
		"cmis:lastModificationDate": {
			ID: "cmis:lastModificationDate", LocalName: "lastModificationDate", DisplayName: "Last Modification Date",
			QueryName: "cmis:lastModificationDate", PropertyType: PropertyTypeDateTime,
			Cardinality: "single", Updatability: "readonly", Queryable: true, Orderable: true,
		},
		"cmis:changeToken": {
			ID: "cmis:changeToken", LocalName: "changeToken", DisplayName: "Change Token",
			QueryName: "cmis:changeToken", PropertyType: PropertyTypeString,
			Cardinality: "single", Updatability: "readonly",
		},
	}
}

func documentPropertyDefs() map[string]PropertyDefinition {
	defs := commonPropertyDefs()
	defs["cmis:contentStreamLength"] = PropertyDefinition{
		ID: "cmis:contentStreamLength", LocalName: "contentStreamLength", DisplayName: "Content Stream Length",
		QueryName: "cmis:contentStreamLength", PropertyType: PropertyTypeInteger,
		Cardinality: "single", Updatability: "readonly", Orderable: true,
	}
	defs["cmis:contentStreamMimeType"] = PropertyDefinition{
		ID: "cmis:contentStreamMimeType", LocalName: "contentStreamMimeType", DisplayName: "Content Stream MIME Type",
		QueryName: "cmis:contentStreamMimeType", PropertyType: PropertyTypeString,
		Cardinality: "single", Updatability: "readonly", Queryable: true,
	}
	defs["cmis:contentStreamFileName"] = PropertyDefinition{
		ID: "cmis:contentStreamFileName", LocalName: "contentStreamFileName", DisplayName: "Content Stream File Name",
		QueryName: "cmis:contentStreamFileName", PropertyType: PropertyTypeString,
		Cardinality: "single", Updatability: "readonly", Queryable: true,
	}
	defs["cmis:isLatestVersion"] = PropertyDefinition{
		ID: "cmis:isLatestVersion", LocalName: "isLatestVersion", DisplayName: "Is Latest Version",
		QueryName: "cmis:isLatestVersion", PropertyType: PropertyTypeBoolean,
		Cardinality: "single", Updatability: "readonly", Queryable: true,
	}
	defs["cmis:versionLabel"] = PropertyDefinition{
		ID: "cmis:versionLabel", LocalName: "versionLabel", DisplayName: "Version Label",
		QueryName: "cmis:versionLabel", PropertyType: PropertyTypeString,
		Cardinality: "single", Updatability: "readonly", Queryable: true,
	}
	return defs
}

func folderPropertyDefs() map[string]PropertyDefinition {
	defs := commonPropertyDefs()
	defs["cmis:parentId"] = PropertyDefinition{
		ID: "cmis:parentId", LocalName: "parentId", DisplayName: "Parent ID",
		QueryName: "cmis:parentId", PropertyType: PropertyTypeID,
		Cardinality: "single", Updatability: "readonly", Queryable: true,
	}
	defs["cmis:path"] = PropertyDefinition{
		ID: "cmis:path", LocalName: "path", DisplayName: "Path",
		QueryName: "cmis:path", PropertyType: PropertyTypeString,
		Cardinality: "single", Updatability: "readonly", Queryable: true,
	}
	return defs
}
