package cmis

// Secondary type definitions for OpenYard-specific metadata.
// These types expose the oy.* and info.* metadata namespaces as
// CMIS secondary types that can be attached to documents and folders.

const (
	TypeOYDocument = "oy:documentMetadata"
	TypeOYFolder   = "oy:folderMetadata"
	TypeOYIndex    = "oy:indexData"
)

func init() {
	// Register secondary types so they're discoverable via type services
	secondaryTypes = []TypeDefinition{
		oyDocumentType(),
		oyFolderType(),
		oyIndexType(),
	}
}

var secondaryTypes []TypeDefinition

// AllTypeDefinitions returns all built-in and secondary type definitions.
func AllTypeDefinitions() []TypeDefinition {
	base := []TypeDefinition{
		*builtinType(BaseTypeDocument),
		*builtinType(BaseTypeFolder),
		*builtinType(BaseTypeRelationship),
		*builtinType(BaseTypePolicy),
		*builtinType(BaseTypeItem),
	}
	return append(base, secondaryTypes...)
}

// SecondaryTypeDefinitions returns only the secondary types.
func SecondaryTypeDefinitions() []TypeDefinition {
	return secondaryTypes
}

// LookupType finds a type by ID across all registered types.
func LookupType(typeID string) *TypeDefinition {
	// Check base types
	if td := builtinType(typeID); td != nil {
		return td
	}
	// Check secondary types
	for i := range secondaryTypes {
		if secondaryTypes[i].ID == typeID {
			return &secondaryTypes[i]
		}
	}
	return nil
}

func oyDocumentType() TypeDefinition {
	return TypeDefinition{
		ID:          TypeOYDocument,
		LocalName:   "documentMetadata",
		DisplayName: "OpenYard Document Metadata",
		QueryName:   "oy:documentMetadata",
		Description: "Extended metadata for documents managed by OpenYard DMS",
		BaseID:      BaseTypeSecondary,
		ParentID:    BaseTypeSecondary,
		Creatable:   false,
		Fileable:    false,
		Queryable:   true,
		PropertyDefinitions: map[string]PropertyDefinition{
			"oy.subject": {
				ID: "oy.subject", LocalName: "subject", DisplayName: "Subject",
				QueryName: "oy.subject", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true, Orderable: true,
			},
			"oy.description": {
				ID: "oy.description", LocalName: "description", DisplayName: "Description",
				QueryName: "oy.description", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true,
			},
			"oy.category": {
				ID: "oy.category", LocalName: "category", DisplayName: "Category",
				QueryName: "oy.category", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true,
			},
			"oy.fileReference": {
				ID: "oy.fileReference", LocalName: "fileReference", DisplayName: "File Reference (Aktenzeichen)",
				QueryName: "oy.fileReference", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true, Orderable: true,
			},
			"oy.status": {
				ID: "oy.status", LocalName: "status", DisplayName: "Status",
				QueryName: "oy.status", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true,
			},
			"oy.docName": {
				ID: "oy.docName", LocalName: "docName", DisplayName: "Document Name",
				QueryName: "oy.docName", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readonly", Queryable: true,
			},
			"oy.fileExtension": {
				ID: "oy.fileExtension", LocalName: "fileExtension", DisplayName: "File Extension",
				QueryName: "oy.fileExtension", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readonly", Queryable: true,
			},
			"oy.creatorName": {
				ID: "oy.creatorName", LocalName: "creatorName", DisplayName: "Creator Name",
				QueryName: "oy.creatorName", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readonly", Queryable: true,
			},
			"oy.type.name": {
				ID: "oy.type.name", LocalName: "typeName", DisplayName: "Document Type Name",
				QueryName: "oy.type.name", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true,
			},
			"oy.type.id": {
				ID: "oy.type.id", LocalName: "typeId", DisplayName: "Document Type ID",
				QueryName: "oy.type.id", PropertyType: PropertyTypeID,
				Cardinality: "single", Updatability: "readwrite", Queryable: true,
			},
			"oy.checkedOut": {
				ID: "oy.checkedOut", LocalName: "checkedOut", DisplayName: "Checked Out",
				QueryName: "oy.checkedOut", PropertyType: PropertyTypeBoolean,
				Cardinality: "single", Updatability: "readonly", Queryable: true,
			},
			"oy.checkinComment": {
				ID: "oy.checkinComment", LocalName: "checkinComment", DisplayName: "Checkin Comment",
				QueryName: "oy.checkinComment", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readonly",
			},
			"oy.fullPath": {
				ID: "oy.fullPath", LocalName: "fullPath", DisplayName: "Full Path",
				QueryName: "oy.fullPath", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readonly", Queryable: true,
			},
		},
	}
}

func oyFolderType() TypeDefinition {
	return TypeDefinition{
		ID:          TypeOYFolder,
		LocalName:   "folderMetadata",
		DisplayName: "OpenYard Folder Metadata",
		QueryName:   "oy:folderMetadata",
		Description: "Extended metadata for folders managed by OpenYard DMS",
		BaseID:      BaseTypeSecondary,
		ParentID:    BaseTypeSecondary,
		Creatable:   false,
		Fileable:    false,
		Queryable:   true,
		PropertyDefinitions: map[string]PropertyDefinition{
			"oy.fileReference": {
				ID: "oy.fileReference", LocalName: "fileReference", DisplayName: "File Reference (Aktenzeichen)",
				QueryName: "oy.fileReference", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true, Orderable: true,
			},
			"oy.subject": {
				ID: "oy.subject", LocalName: "subject", DisplayName: "Subject",
				QueryName: "oy.subject", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true,
			},
			"oy.description": {
				ID: "oy.description", LocalName: "description", DisplayName: "Description",
				QueryName: "oy.description", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true,
			},
		},
	}
}

func oyIndexType() TypeDefinition {
	return TypeDefinition{
		ID:          TypeOYIndex,
		LocalName:   "indexData",
		DisplayName: "OpenYard Index Data",
		QueryName:   "oy:indexData",
		Description: "User-defined index fields (info.* namespace) for search and classification",
		BaseID:      BaseTypeSecondary,
		ParentID:    BaseTypeSecondary,
		Creatable:   false,
		Fileable:    false,
		Queryable:   true,
		PropertyDefinitions: map[string]PropertyDefinition{
			"info.ident": {
				ID: "info.ident", LocalName: "ident", DisplayName: "Ident",
				QueryName: "info.ident", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true, Orderable: true,
			},
			"info.fileReference": {
				ID: "info.fileReference", LocalName: "fileReference", DisplayName: "File Reference",
				QueryName: "info.fileReference", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true,
			},
			"info.subject": {
				ID: "info.subject", LocalName: "subject", DisplayName: "Subject",
				QueryName: "info.subject", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true,
			},
			"info.register": {
				ID: "info.register", LocalName: "register", DisplayName: "Register",
				QueryName: "info.register", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true,
			},
			"info.sender": {
				ID: "info.sender", LocalName: "sender", DisplayName: "Sender",
				QueryName: "info.sender", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true,
			},
			"info.recipient": {
				ID: "info.recipient", LocalName: "recipient", DisplayName: "Recipient",
				QueryName: "info.recipient", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true,
			},
			"info.remark": {
				ID: "info.remark", LocalName: "remark", DisplayName: "Remark",
				QueryName: "info.remark", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true,
			},
			"info.designation": {
				ID: "info.designation", LocalName: "designation", DisplayName: "Designation",
				QueryName: "info.designation", PropertyType: PropertyTypeString,
				Cardinality: "single", Updatability: "readwrite", Queryable: true,
			},
		},
	}
}
