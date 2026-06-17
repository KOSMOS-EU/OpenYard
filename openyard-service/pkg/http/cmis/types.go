package cmis

// CMIS 1.1 Browser JSON Binding – Type Definitions
// Reference: OASIS CMIS 1.1 Specification, Section 5.4 (Browser Binding)

// --- Property Types ---

const (
	PropertyTypeString   = "string"
	PropertyTypeBoolean  = "boolean"
	PropertyTypeInteger  = "integer"
	PropertyTypeDecimal  = "decimal"
	PropertyTypeDateTime = "datetime"
	PropertyTypeID       = "id"
	PropertyTypeHTML     = "html"
	PropertyTypeURI      = "uri"
)

// --- Base Type IDs ---

const (
	BaseTypeDocument     = "cmis:document"
	BaseTypeFolder       = "cmis:folder"
	BaseTypeRelationship = "cmis:relationship"
	BaseTypePolicy       = "cmis:policy"
	BaseTypeItem         = "cmis:item"
	BaseTypeSecondary    = "cmis:secondary"
)

// --- CMIS Property ---

type Property struct {
	ID          string      `json:"id"`
	LocalName   string      `json:"localName,omitempty"`
	DisplayName string      `json:"displayName,omitempty"`
	QueryName   string      `json:"queryName,omitempty"`
	Type        string      `json:"type"`
	Cardinality string      `json:"cardinality,omitempty"` // "single" or "multi"
	Value       interface{} `json:"value"`
}

// --- CMIS Object ---

type Object struct {
	Properties       map[string]Property `json:"properties"`
	AllowableActions map[string]bool     `json:"allowableActions,omitempty"`
}

// ObjectEntry wraps an Object in a list response.
type ObjectEntry struct {
	Object Object `json:"object"`
}

// --- CMIS Object List (getChildren response) ---

type ObjectList struct {
	Objects      []ObjectEntry `json:"objects"`
	HasMoreItems bool          `json:"hasMoreItems"`
	NumItems     int           `json:"numItems"`
}

// --- CMIS Object in Folder (getChildren container) ---

type ObjectInFolderList struct {
	Objects      []ObjectInFolderData `json:"objects"`
	HasMoreItems bool                 `json:"hasMoreItems"`
	NumItems     int                  `json:"numItems"`
}

type ObjectInFolderData struct {
	Object   Object `json:"object"`
	PathSegment string `json:"pathSegment,omitempty"`
}

// --- Repository Info ---

type RepositoryInfo struct {
	RepositoryID          string       `json:"repositoryId"`
	RepositoryName        string       `json:"repositoryName"`
	RepositoryDescription string       `json:"repositoryDescription,omitempty"`
	VendorName            string       `json:"vendorName"`
	ProductName           string       `json:"productName"`
	ProductVersion        string       `json:"productVersion"`
	RootFolderID          string       `json:"rootFolderId"`
	CMISVersionSupported  string       `json:"cmisVersionSupported"`
	RepositoryURL         string       `json:"repositoryUrl"`
	RootFolderURL         string       `json:"rootFolderUrl"`
	Capabilities          Capabilities `json:"capabilities"`
	ACLCapability         ACLCapability `json:"aclCapability"`
}

// Capabilities declares what the repository supports.
type Capabilities struct {
	CapabilityContentStreamUpdatability string `json:"capabilityContentStreamUpdatability"` // "anytime"
	CapabilityChanges                   string `json:"capabilityChanges"`                   // "none"
	CapabilityRenditions                string `json:"capabilityRenditions"`                // "none"
	CapabilityGetDescendants            bool   `json:"capabilityGetDescendants"`
	CapabilityGetFolderTree             bool   `json:"capabilityGetFolderTree"`
	CapabilityMultifiling               bool   `json:"capabilityMultifiling"`
	CapabilityUnfiling                  bool   `json:"capabilityUnfiling"`
	CapabilityVersionSpecificFiling     bool   `json:"capabilityVersionSpecificFiling"`
	CapabilityPWCSearchable             bool   `json:"capabilityPWCSearchable"`
	CapabilityPWCUpdatable              bool   `json:"capabilityPWCUpdatable"`
	CapabilityAllVersionsSearchable     bool   `json:"capabilityAllVersionsSearchable"`
	CapabilityQuery                     string `json:"capabilityQuery"`       // "none","metadataonly","fulltextonly","bothseparate","bothcombined"
	CapabilityJoin                      string `json:"capabilityJoin"`        // "none"
	CapabilityACL                       string `json:"capabilityACL"`         // "none","discover","manage"
	CapabilityCreatablePropertyTypes    CreatablePropertyTypes `json:"capabilityCreatablePropertyTypes"`
	CapabilityNewTypeSettableAttributes NewTypeSettableAttributes `json:"capabilityNewTypeSettableAttributes"`
	CapabilityOrderBy                   string `json:"capabilityOrderBy"` // "none","common","custom"
}

type CreatablePropertyTypes struct {
	CanCreate []string `json:"canCreate"` // e.g. ["string","integer","boolean","datetime"]
}

type NewTypeSettableAttributes struct {
	ID                       bool `json:"id"`
	LocalName                bool `json:"localName"`
	LocalNamespace           bool `json:"localNamespace"`
	DisplayName              bool `json:"displayName"`
	QueryName                bool `json:"queryName"`
	Description              bool `json:"description"`
	Creatable                bool `json:"creatable"`
	Fileable                 bool `json:"fileable"`
	Queryable                bool `json:"queryable"`
	FulltextIndexed          bool `json:"fulltextIndexed"`
	IncludedInSupertypeQuery bool `json:"includedInSupertypeQuery"`
	ControllablePolicy       bool `json:"controllablePolicy"`
	ControllableACL          bool `json:"controllableACL"`
}

// ACLCapability describes ACL support.
type ACLCapability struct {
	SupportedPermissions string `json:"supportedPermissions"` // "basic","repository","both"
	Propagation          string `json:"propagation"`          // "repositorydetermined"
}

// --- Type Definition ---

type TypeDefinition struct {
	ID              string                      `json:"id"`
	LocalName       string                      `json:"localName"`
	LocalNamespace  string                      `json:"localNamespace,omitempty"`
	DisplayName     string                      `json:"displayName"`
	QueryName       string                      `json:"queryName"`
	Description     string                      `json:"description,omitempty"`
	BaseID          string                      `json:"baseId"`
	ParentID        string                      `json:"parentId,omitempty"`
	Creatable       bool                        `json:"creatable"`
	Fileable        bool                        `json:"fileable"`
	Queryable       bool                        `json:"queryable"`
	FulltextIndexed bool                        `json:"fulltextIndexed"`
	ControllableACL bool                        `json:"controllableACL"`
	ControllablePolicy bool                     `json:"controllablePolicy"`
	PropertyDefinitions map[string]PropertyDefinition `json:"propertyDefinitions,omitempty"`
	// Document-specific
	Versionable         *bool  `json:"versionable,omitempty"`
	ContentStreamAllowed string `json:"contentStreamAllowed,omitempty"` // "notallowed","allowed","required"
}

type PropertyDefinition struct {
	ID           string `json:"id"`
	LocalName    string `json:"localName"`
	DisplayName  string `json:"displayName"`
	QueryName    string `json:"queryName"`
	Description  string `json:"description,omitempty"`
	PropertyType string `json:"propertyType"`
	Cardinality  string `json:"cardinality"` // "single" or "multi"
	Updatability string `json:"updatability"` // "readonly","readwrite","oncreate"
	Required     bool   `json:"required"`
	Queryable    bool   `json:"queryable"`
	Orderable    bool   `json:"orderable"`
}

// TypeDefinitionList for getTypeChildren response.
type TypeDefinitionList struct {
	Types        []TypeDefinition `json:"types"`
	HasMoreItems bool             `json:"hasMoreItems"`
	NumItems     int              `json:"numItems"`
}
