package upload

import (
	"context"
)

// Result contains the outcome of a file upload.
type Result struct {
	FileID string // OpaqueId of the uploaded file (CS3 ResourceId)
	ETag   string
}

// Request describes a file to upload.
type Request struct {
	FileName string // Target filename (already escaped for POSIX)
	Data     []byte // File content
	SpaceID  string // CS3 space to upload to (base64-encoded ObjectID of space root)
	FolderID string // Target folder (base64-encoded ObjectID) — file lands here
	Username string // For auth
	Password string // For auth
	CS3Token string // Reva token (for reva-weg)
}

// Uploader uploads a file to OpenCloud storage.
type Uploader interface {
	// Upload stores the file and returns the new file's resource ID.
	Upload(ctx context.Context, req *Request) (*Result, error)

	// Name returns a human-readable name for logging ("webdav", "reva").
	Name() string
}
