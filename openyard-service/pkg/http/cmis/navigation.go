package cmis

import (
	"net/http"
	"path"
	"strconv"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/rs/zerolog/log"
)

// GetChildren handles cmisselector=children on a folder.
// Supports pagination via skipCount and maxItems query parameters.
func (h *Handler) GetChildren(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	skipCount := queryInt(r, "skipCount", 0)
	maxItems := queryInt(r, "maxItems", 100)

	res, err := h.gw.Gateway.ListContainer(r.Context(), &provider.ListContainerRequest{Ref: ref})
	if err != nil {
		log.Error().Err(err).Msg("cmis: list container failed")
		writeError(w, 500, "runtime", "Cannot list children")
		return
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "objectNotFound", "Folder not found")
		return
	}

	total := len(res.Infos)

	// Apply pagination
	start := skipCount
	if start > total {
		start = total
	}
	end := start + maxItems
	if end > total {
		end = total
	}
	hasMore := end < total

	objects := make([]ObjectInFolderData, 0, end-start)
	for _, info := range res.Infos[start:end] {
		objects = append(objects, ObjectInFolderData{
			Object:      mapResourceToObject(info),
			PathSegment: path.Base(info.Path),
		})
	}

	writeJSON(w, 200, ObjectInFolderList{
		Objects:      objects,
		HasMoreItems: hasMore,
		NumItems:     total,
	})
}

// GetFolderParent handles cmisselector=parent on a folder.
func (h *Handler) GetFolderParent(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	// Stat the folder to get parentId
	res, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "objectNotFound", "Object not found")
		return
	}

	if res.Info.ParentId == nil {
		// Root folder has no parent
		writeError(w, 400, "constraint", "Root folder has no parent")
		return
	}

	parentRef := &provider.Reference{ResourceId: res.Info.ParentId, Path: "."}
	parentRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: parentRef})
	if err != nil || parentRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "objectNotFound", "Parent not found")
		return
	}

	writeJSON(w, 200, mapResourceToObject(parentRes.Info))
}

// GetObjectParents handles cmisselector=parents on any object.
func (h *Handler) GetObjectParents(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	res, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "objectNotFound", "Object not found")
		return
	}

	if res.Info.ParentId == nil {
		writeJSON(w, 200, []ObjectInFolderData{})
		return
	}

	parentRef := &provider.Reference{ResourceId: res.Info.ParentId, Path: "."}
	parentRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: parentRef})
	if err != nil || parentRes.Status.Code != rpc.Code_CODE_OK {
		writeJSON(w, 200, []ObjectInFolderData{})
		return
	}

	writeJSON(w, 200, []ObjectInFolderData{
		{
			Object:      mapResourceToObject(parentRes.Info),
			PathSegment: path.Base(res.Info.Path),
		},
	})
}

// --- Descendants / Folder Tree types ---

// ObjectInFolderContainer represents a node in the descendants tree.
type ObjectInFolderContainer struct {
	Object   Object                    `json:"object"`
	PathSegment string                 `json:"pathSegment,omitempty"`
	Children []ObjectInFolderContainer `json:"children,omitempty"`
}

// GetDescendants handles cmisselector=descendants — returns a recursive tree.
func (h *Handler) GetDescendants(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	depth := queryInt(r, "depth", -1) // -1 = unlimited
	if depth == 0 {
		writeJSON(w, 200, []ObjectInFolderContainer{})
		return
	}

	tree := h.buildDescendantsTree(r, ref, depth, false)
	writeJSON(w, 200, tree)
}

// GetFolderTree handles cmisselector=foldertree — returns only folders recursively.
func (h *Handler) GetFolderTree(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	depth := queryInt(r, "depth", -1)
	if depth == 0 {
		writeJSON(w, 200, []ObjectInFolderContainer{})
		return
	}

	tree := h.buildDescendantsTree(r, ref, depth, true)
	writeJSON(w, 200, tree)
}

func (h *Handler) buildDescendantsTree(r *http.Request, ref *provider.Reference, depth int, foldersOnly bool) []ObjectInFolderContainer {
	if depth == 0 {
		return nil
	}

	res, err := h.gw.Gateway.ListContainer(r.Context(), &provider.ListContainerRequest{Ref: ref})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		return nil
	}

	nextDepth := depth
	if nextDepth > 0 {
		nextDepth--
	}

	var nodes []ObjectInFolderContainer
	for _, info := range res.Infos {
		isFolder := info.Type == provider.ResourceType_RESOURCE_TYPE_CONTAINER
		if foldersOnly && !isFolder {
			continue
		}

		node := ObjectInFolderContainer{
			Object:      mapResourceToObject(info),
			PathSegment: path.Base(info.Path),
		}

		if isFolder && nextDepth != 0 {
			childRef := &provider.Reference{ResourceId: info.Id, Path: "."}
			node.Children = h.buildDescendantsTree(r, childRef, nextDepth, foldersOnly)
		}

		nodes = append(nodes, node)
	}
	return nodes
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}
