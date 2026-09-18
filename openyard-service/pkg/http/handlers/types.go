package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	gocache "github.com/patrickmn/go-cache"

	"github.com/kosmos-eu/openyard/pkg/auth"
	"github.com/kosmos-eu/openyard/pkg/cs3client"
	"github.com/kosmos-eu/openyard/pkg/upload"
)

// serverStartTime is captured at package init for uptime reporting.
var serverStartTime = time.Now()

// Handlers holds shared dependencies for all endpoint handlers.
type Handlers struct {
	gw           *cs3client.Client
	sessions     *auth.SessionCache
	uploader     upload.Uploader
	uploadURL    string // Upload base URL (OC web proxy), e.g. "http://opencloud:9200"
	downloadURL  string // Download base URL (Reva data gateway), e.g. "http://opencloud:9216"
	workCache    *gocache.Cache // WorkID → *workObject for GetFolder pagination
}

// workObject stores remaining sub-elements for paginated GetFolder responses.
type workObject struct {
	FolderInfo map[string]interface{}
	SubFolders []map[string]interface{}
	SubDocs    []map[string]interface{}
}

func New(gw *cs3client.Client, sessions *auth.SessionCache, up upload.Uploader, uploadURL, downloadURL string) *Handlers {
	return &Handlers{
		gw:          gw,
		sessions:    sessions,
		uploader:    up,
		uploadURL:   uploadURL,
		downloadURL: downloadURL,
		workCache:   gocache.New(10*time.Minute, 5*time.Minute),
	}
}

// contextKey for session context propagation.
type contextKey string

const sessionCtxKey contextKey = "session"

func sessionFromCtx(ctx context.Context) *auth.SessionContext {
	if v, ok := ctx.Value(sessionCtxKey).(*auth.SessionContext); ok {
		return v
	}
	return nil
}

// --- Response helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
		"code":  code,
	})
}
