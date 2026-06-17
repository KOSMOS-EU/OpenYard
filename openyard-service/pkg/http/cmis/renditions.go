package cmis

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
)

// Rendition represents a CMIS rendition (e.g. thumbnail).
type Rendition struct {
	StreamID string `json:"streamId"`
	MimeType string `json:"mimeType"`
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	Length   int64  `json:"length"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// Supported thumbnail MIME types
var thumbnailMimeTypes = map[string]bool{
	"image/jpeg": true, "image/png": true, "image/gif": true,
	"image/webp": true, "image/svg+xml": true, "image/bmp": true,
	"application/pdf": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
	"application/vnd.oasis.opendocument.text":                                   true,
	"application/vnd.oasis.opendocument.spreadsheet":                            true,
	"application/vnd.oasis.opendocument.presentation":                           true,
	"text/plain": true,
}

// GetRenditions handles cmisselector=renditions — returns available renditions.
func (h *Handler) GetRenditions(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "objectNotFound", "Object not found")
		return
	}

	renditions := buildRenditions(statRes.Info, h.baseURL)
	writeJSON(w, 200, renditions)
}

// buildRenditions creates rendition entries for a resource.
func buildRenditions(info *provider.ResourceInfo, baseURL string) []Rendition {
	if info.Type != provider.ResourceType_RESOURCE_TYPE_FILE {
		return []Rendition{}
	}

	var renditions []Rendition

	// Thumbnail rendition (if MIME type is supported)
	if hasThumbnail(info.MimeType) {
		objectID := encodeObjectID(info.Id)
		renditions = append(renditions, Rendition{
			StreamID: fmt.Sprintf("cmis:thumbnail:%s", objectID),
			MimeType: "image/png",
			Kind:     "cmis:thumbnail",
			Title:    "Thumbnail",
			Width:    256,
			Height:   256,
		})
	}

	// Icon rendition (always available for files)
	renditions = append(renditions, Rendition{
		StreamID: "cmis:icon",
		MimeType: "image/svg+xml",
		Kind:     "oc:icon",
		Title:    iconForMimeType(info.MimeType),
	})

	return renditions
}

func hasThumbnail(mimeType string) bool {
	return thumbnailMimeTypes[mimeType]
}

func iconForMimeType(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.HasPrefix(mimeType, "video/"):
		return "video"
	case strings.HasPrefix(mimeType, "audio/"):
		return "audio"
	case mimeType == "application/pdf":
		return "pdf"
	case strings.Contains(mimeType, "word") || strings.Contains(mimeType, "opendocument.text"):
		return "document"
	case strings.Contains(mimeType, "spreadsheet") || strings.Contains(mimeType, "excel"):
		return "spreadsheet"
	case strings.Contains(mimeType, "presentation") || strings.Contains(mimeType, "powerpoint"):
		return "presentation"
	case strings.Contains(mimeType, "zip") || strings.Contains(mimeType, "tar") || strings.Contains(mimeType, "compressed"):
		return "archive"
	case strings.HasPrefix(mimeType, "text/"):
		return "text"
	default:
		return "file"
	}
}

// addRenditionsToObject enriches a CMIS Object with rendition information.
func addRenditionsToObject(obj *Object, info *provider.ResourceInfo, baseURL string) {
	renditions := buildRenditions(info, baseURL)
	if len(renditions) > 0 {
		obj.Properties["cmis:secondaryObjectTypeIds"] = Property{
			ID: "cmis:secondaryObjectTypeIds", Type: PropertyTypeID,
			Cardinality: "multi",
			Value:       []string{TypeOYDocument},
		}
	}
	// Renditions are returned separately via cmisselector=renditions,
	// but we also include the rendition filter hint in the object
	if len(renditions) > 0 {
		ext := path.Ext(path.Base(info.Path))
		if ext != "" {
			obj.Properties["cmis:contentStreamId"] = Property{
				ID: "cmis:contentStreamId", Type: PropertyTypeID,
				Value: encodeObjectID(info.Id),
			}
		}
	}
}
