package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/kosmos-eu/openyard/pkg/auth"
	"github.com/kosmos-eu/openyard/pkg/cs3client"
	"github.com/kosmos-eu/openyard/pkg/upload"
)

// Handlers holds shared dependencies for all endpoint handlers.
type Handlers struct {
	gw       *cs3client.Client
	sessions *auth.SessionCache
	uploader upload.Uploader
}

func New(gw *cs3client.Client, sessions *auth.SessionCache, up upload.Uploader) *Handlers {
	return &Handlers{gw: gw, sessions: sessions, uploader: up}
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
