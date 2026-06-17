package handlers

import (
	"context"
	"net/http"
)

// SessionMiddleware validates the SessionID query parameter
// and injects the session context into the request.
func (h *Handlers) SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.URL.Query().Get("SessionID")
		if sessionID == "" {
			writeError(w, 401, "AUTH_REQUIRED", "SessionID required")
			return
		}

		sess := h.sessions.Get(sessionID)
		if sess == nil {
			writeError(w, 401, "AUTH_REQUIRED", "Invalid or expired SessionID")
			return
		}

		ctx := context.WithValue(r.Context(), sessionCtxKey, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
