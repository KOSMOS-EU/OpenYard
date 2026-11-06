package handlers

// In-memory CS3 gateway and data server for handler tests: one storage,
// spaces with folders and files, arbitrary metadata, upload/download via
// transfer tokens like the OpenCloud data gateway.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	gateway "github.com/cs3org/go-cs3apis/cs3/gateway/v1beta1"
	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	typesv1 "github.com/cs3org/go-cs3apis/cs3/types/v1beta1"
	"google.golang.org/grpc"

	"github.com/kosmos-eu/openyard/pkg/cs3client"
	"github.com/kosmos-eu/openyard/pkg/upload"
)

const fakeStorageID = "storage-users-1"

type fakeNode struct {
	id       string
	spaceID  string
	parentID string // "" for space roots
	name     string
	isDir    bool
	content  []byte
	metadata map[string]string
}

type fakeStorage struct {
	gateway.GatewayAPIClient // unimplemented calls panic

	mu        sync.Mutex
	nodes     map[string]*fakeNode
	spaces    []*fakeNode
	transfers map[string]string // token → target ("node:<id>" or "new:<parentID>/<name>")
	statCalls int
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{nodes: map[string]*fakeNode{}, transfers: map[string]string{}}
}

// fakeNodeCounter makes node IDs unique across tests (the migration DB is
// process-wide, see migration.Init).
var fakeNodeCounter atomic.Int64

// newGUID returns UUID-shaped node IDs.
func (s *fakeStorage) newGUID() string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", fakeNodeCounter.Add(1))
}

func (s *fakeStorage) addSpace(name string) *fakeNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.newGUID()
	space := &fakeNode{id: id, spaceID: id, name: name, isDir: true, metadata: map[string]string{}}
	s.nodes[id] = space
	s.spaces = append(s.spaces, space)
	return space
}

func (s *fakeStorage) addNode(parent *fakeNode, name string, isDir bool, content []byte, metadata map[string]string) *fakeNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addNodeLocked(parent, name, isDir, content, metadata)
}

func (s *fakeStorage) addNodeLocked(parent *fakeNode, name string, isDir bool, content []byte, metadata map[string]string) *fakeNode {
	if metadata == nil {
		metadata = map[string]string{}
	}
	node := &fakeNode{id: s.newGUID(), spaceID: parent.spaceID, parentID: parent.id, name: name, isDir: isDir, content: content, metadata: metadata}
	s.nodes[node.id] = node
	return node
}

func (s *fakeStorage) resourceID(node *fakeNode) *provider.ResourceId {
	return &provider.ResourceId{StorageId: fakeStorageID, SpaceId: node.spaceID, OpaqueId: node.id}
}

func (s *fakeStorage) nodePath(node *fakeNode) string {
	if node.parentID == "" {
		return "/" + node.name
	}
	return path.Join(s.nodePath(s.nodes[node.parentID]), node.name)
}

func (s *fakeStorage) childByName(parent *fakeNode, name string) *fakeNode {
	for _, node := range s.nodes {
		if node.parentID == parent.id && node.name == name {
			return node
		}
	}
	return nil
}

// resolve finds the node of a reference. Nodes are only found in their own
// space (SpaceId must match, like decomposedfs).
func (s *fakeStorage) resolve(ref *provider.Reference) (*fakeNode, string) {
	if ref == nil || ref.ResourceId == nil {
		return nil, ""
	}
	node, ok := s.nodes[ref.ResourceId.OpaqueId]
	if !ok || node.spaceID != ref.ResourceId.SpaceId {
		return nil, ""
	}
	relative := strings.TrimPrefix(strings.TrimPrefix(ref.Path, "."), "/")
	if relative == "" {
		return node, ""
	}
	if child := s.childByName(node, relative); child != nil {
		return child, ""
	}
	return node, relative // parent plus missing child name
}

func (s *fakeStorage) info(node *fakeNode, withMetadata bool) *provider.ResourceInfo {
	info := &provider.ResourceInfo{
		Id:    s.resourceID(node),
		Path:  s.nodePath(node),
		Size:  uint64(len(node.content)),
		Mtime: &typesv1.Timestamp{Seconds: 1762430400},
		Type:  provider.ResourceType_RESOURCE_TYPE_FILE,
	}
	if node.isDir {
		info.Type = provider.ResourceType_RESOURCE_TYPE_CONTAINER
	}
	if node.parentID != "" {
		info.ParentId = s.resourceID(s.nodes[node.parentID])
	}
	if withMetadata {
		metadata := map[string]string{}
		for key, value := range node.metadata {
			metadata[key] = value
		}
		info.ArbitraryMetadata = &provider.ArbitraryMetadata{Metadata: metadata}
	}
	return info
}

func statusOK() *rpc.Status { return &rpc.Status{Code: rpc.Code_CODE_OK} }

func statusNotFound() *rpc.Status {
	return &rpc.Status{Code: rpc.Code_CODE_NOT_FOUND, Message: "not found"}
}

func (s *fakeStorage) ListStorageSpaces(_ context.Context, _ *provider.ListStorageSpacesRequest, _ ...grpc.CallOption) (*provider.ListStorageSpacesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	spaces := make([]*provider.StorageSpace, 0, len(s.spaces))
	for _, space := range s.spaces {
		spaces = append(spaces, &provider.StorageSpace{Root: s.resourceID(space), Name: space.name, SpaceType: "project"})
	}
	return &provider.ListStorageSpacesResponse{Status: statusOK(), StorageSpaces: spaces}, nil
}

func (s *fakeStorage) Stat(_ context.Context, req *provider.StatRequest, _ ...grpc.CallOption) (*provider.StatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statCalls++
	node, missing := s.resolve(req.Ref)
	if node == nil || missing != "" {
		return &provider.StatResponse{Status: statusNotFound()}, nil
	}
	return &provider.StatResponse{Status: statusOK(), Info: s.info(node, len(req.ArbitraryMetadataKeys) > 0)}, nil
}

func (s *fakeStorage) ListContainer(_ context.Context, req *provider.ListContainerRequest, _ ...grpc.CallOption) (*provider.ListContainerResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	folder, missing := s.resolve(req.Ref)
	if folder == nil || missing != "" {
		return &provider.ListContainerResponse{Status: statusNotFound()}, nil
	}
	infos := []*provider.ResourceInfo{}
	for _, node := range s.nodes {
		if node.parentID == folder.id {
			infos = append(infos, s.info(node, len(req.ArbitraryMetadataKeys) > 0))
		}
	}
	return &provider.ListContainerResponse{Status: statusOK(), Infos: infos}, nil
}

func (s *fakeStorage) SetArbitraryMetadata(_ context.Context, req *provider.SetArbitraryMetadataRequest, _ ...grpc.CallOption) (*provider.SetArbitraryMetadataResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, missing := s.resolve(req.Ref)
	if node == nil || missing != "" {
		return &provider.SetArbitraryMetadataResponse{Status: statusNotFound()}, nil
	}
	for key, value := range req.ArbitraryMetadata.Metadata {
		node.metadata[key] = value
	}
	return &provider.SetArbitraryMetadataResponse{Status: statusOK()}, nil
}

func (s *fakeStorage) InitiateFileUpload(_ context.Context, req *provider.InitiateFileUploadRequest, _ ...grpc.CallOption) (*gateway.InitiateFileUploadResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, missing := s.resolve(req.Ref)
	if node == nil {
		return &gateway.InitiateFileUploadResponse{Status: statusNotFound()}, nil
	}
	token := fmt.Sprintf("up-%d", len(s.transfers)+1)
	if missing != "" {
		s.transfers[token] = "new:" + node.id + "/" + missing
	} else {
		s.transfers[token] = "node:" + node.id
	}
	return &gateway.InitiateFileUploadResponse{
		Status:    statusOK(),
		Protocols: []*gateway.FileUploadProtocol{{Protocol: "simple", Token: token}},
	}, nil
}

func (s *fakeStorage) InitiateFileDownload(_ context.Context, req *provider.InitiateFileDownloadRequest, _ ...grpc.CallOption) (*gateway.InitiateFileDownloadResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, missing := s.resolve(req.Ref)
	if node == nil || missing != "" {
		return &gateway.InitiateFileDownloadResponse{Status: statusNotFound()}, nil
	}
	token := fmt.Sprintf("down-%d", len(s.transfers)+1)
	s.transfers[token] = "node:" + node.id
	return &gateway.InitiateFileDownloadResponse{
		Status:    statusOK(),
		Protocols: []*gateway.FileDownloadProtocol{{Protocol: "spaces", Token: token}},
	}, nil
}

// ServeHTTP is the data gateway (/data): GET downloads, PUT uploads.
func (s *fakeStorage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	target, ok := s.transfers[r.Header.Get("X-Reva-Transfer")]
	if !ok || r.Header.Get("X-Access-Token") == "" {
		http.Error(w, "invalid transfer token", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		node := s.nodes[strings.TrimPrefix(target, "node:")]
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(node.content)
	case http.MethodPut:
		content, _ := io.ReadAll(r.Body)
		if strings.HasPrefix(target, "new:") {
			parentID, name, _ := strings.Cut(strings.TrimPrefix(target, "new:"), "/")
			s.addNodeLocked(s.nodes[parentID], name, false, content, nil)
		} else {
			s.nodes[strings.TrimPrefix(target, "node:")].content = content
		}
		w.WriteHeader(http.StatusCreated)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// fakeUploader stands in for the WebDAV/Reva uploader of ImportDocumentDynamic:
// files land in the given folder.
type fakeUploader struct {
	storage *fakeStorage
	folder  *fakeNode
}

func (u *fakeUploader) Name() string { return "fake" }

func (u *fakeUploader) Upload(_ context.Context, req *upload.Request) (*upload.Result, error) {
	node := u.storage.addNode(u.folder, req.FileName, false, req.Data, nil)
	return &upload.Result{FileID: fakeStorageID + "$" + node.spaceID + "!" + node.id}, nil
}

// newFakeHandlers wires handlers to the fake storage; returns the data server.
func newFakeHandlers(t *testing.T, storage *fakeStorage, folder *fakeNode) *Handlers {
	t.Helper()
	dataServer := httptest.NewServer(storage)
	t.Cleanup(dataServer.Close)
	h, _ := newTestHandlers()
	h.gw = &cs3client.Client{Gateway: storage}
	h.uploader = &fakeUploader{storage: storage, folder: folder}
	h.uploadURL = dataServer.URL
	return h
}
