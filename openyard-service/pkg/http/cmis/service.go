package cmis

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	gateway "github.com/cs3org/go-cs3apis/cs3/gateway/v1beta1"
	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"google.golang.org/grpc/metadata"

	"github.com/kosmos-eu/openyard/pkg/auth"
	"github.com/kosmos-eu/openyard/pkg/config"
	"github.com/kosmos-eu/openyard/pkg/cs3client"
)

// Handler holds dependencies for CMIS endpoints.
type Handler struct {
	gw       *cs3client.Client
	sessions *auth.SessionCache
	baseURL  string
}

func New(gw *cs3client.Client, sessions *auth.SessionCache, cfg *config.Config) *Handler {
	return &Handler{
		gw:       gw,
		sessions: sessions,
		baseURL:  strings.TrimRight(cfg.HTTP.BaseURL, "/"),
	}
}

// Router returns a chi.Router mounted at /cmis.
func (h *Handler) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(h.AuthMiddleware)

	// Service document: list all repositories
	r.Get("/", h.GetRepositories)

	// Repository-scoped endpoints
	r.Route("/{repoId}", func(r chi.Router) {
		// Repository info & type services (dispatch by cmisselector)
		r.Get("/", h.dispatchRepository)
		// Query action
		r.Post("/", h.dispatchRepositoryAction)

		// Root folder
		r.Get("/root", h.dispatchRootGet)
		r.Post("/root", h.dispatchRootPost)

		// Object by path
		r.Get("/root/*", h.dispatchObjectGet)
		r.Post("/root/*", h.dispatchObjectPost)
	})

	return r
}

// --- Auth Middleware ---
// CMIS Browser Binding supports:
// 1. HTTP Basic Auth → authenticate against CS3
// 2. Bearer token
// 3. OpenYard SessionID cookie/header

type cmisContextKey string

const cmisSessionKey cmisContextKey = "cmisSession"

func sessionFromCtx(ctx context.Context) *auth.SessionContext {
	if v, ok := ctx.Value(cmisSessionKey).(*auth.SessionContext); ok {
		return v
	}
	return nil
}

func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sess *auth.SessionContext

		// Try Basic Auth
		if user, pass, ok := r.BasicAuth(); ok && user != "" {
			sess = h.authenticateBasic(r.Context(), user, pass)
			if sess == nil {
				w.Header().Set("WWW-Authenticate", `Basic realm="CMIS"`)
				writeError(w, 401, "unauthorized", "Invalid credentials")
				return
			}
		}

		// Try Bearer token (X-Access-Token or Authorization: Bearer ...)
		if sess == nil {
			token := r.Header.Get("X-Access-Token")
			if token == "" {
				if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
					token = strings.TrimPrefix(auth, "Bearer ")
				}
			}
			if token != "" {
				sess = h.authenticateToken(r.Context(), token)
			}
		}

		// Try SessionID from query or cookie
		if sess == nil {
			sid := r.URL.Query().Get("SessionID")
			if sid == "" {
				if c, err := r.Cookie("SessionID"); err == nil {
					sid = c.Value
				}
			}
			if sid != "" {
				sess = h.sessions.Get(sid)
			}
		}

		if sess == nil {
			w.Header().Set("WWW-Authenticate", `Basic realm="CMIS"`)
			writeError(w, 401, "unauthorized", "Authentication required")
			return
		}

		ctx := context.WithValue(r.Context(), cmisSessionKey, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) authenticateBasic(ctx context.Context, user, pass string) *auth.SessionContext {
	if h.gw == nil || h.gw.Gateway == nil {
		return nil
	}
	res, err := h.gw.Gateway.Authenticate(ctx, &gateway.AuthenticateRequest{
		Type:         "basic",
		ClientId:     user,
		ClientSecret: pass,
	})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		return nil
	}
	return &auth.SessionContext{
		User:     res.User,
		CS3Token: res.Token,
		Login:    user,
		Fullname: res.User.DisplayName,
		UserID:   res.User.Id.OpaqueId,
	}
}

func (h *Handler) authenticateToken(ctx context.Context, token string) *auth.SessionContext {
	if h.gw == nil || h.gw.Gateway == nil {
		return nil
	}
	res, err := h.gw.Gateway.WhoAmI(ctx, &gateway.WhoAmIRequest{Token: token})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		return nil
	}
	return &auth.SessionContext{
		User:     res.User,
		CS3Token: token,
		Login:    res.User.Username,
		Fullname: res.User.DisplayName,
		UserID:   res.User.Id.OpaqueId,
	}
}

// withCS3Token injects the CS3 access token into the gRPC outgoing context.
func (h *Handler) withCS3Token(r *http.Request) *http.Request {
	sess := sessionFromCtx(r.Context())
	if sess != nil && sess.CS3Token != "" {
		ctx := metadata.AppendToOutgoingContext(r.Context(), "x-access-token", sess.CS3Token)
		return r.WithContext(ctx)
	}
	return r
}

// --- Dispatchers ---

// dispatchRepository dispatches GET /cmis/{repoId} based on cmisselector.
func (h *Handler) dispatchRepository(w http.ResponseWriter, r *http.Request) {
	r = h.withCS3Token(r)
	repoID := chi.URLParam(r, "repoId")
	selector := r.URL.Query().Get("cmisselector")

	switch selector {
	case "repositoryInfo", "":
		h.GetRepositoryInfo(w, r, repoID)
	case "typeChildren":
		h.GetTypeChildren(w, r)
	case "typeDefinition":
		h.GetTypeDefinition(w, r)
	case "typeDescendants":
		h.GetTypeDescendants(w, r)
	default:
		writeError(w, 400, "invalidArgument", "Unknown cmisselector: "+selector)
	}
}

// dispatchRepositoryAction dispatches POST /cmis/{repoId} based on cmisaction.
func (h *Handler) dispatchRepositoryAction(w http.ResponseWriter, r *http.Request) {
	r = h.withCS3Token(r)
	r.ParseMultipartForm(32 << 20)
	action := r.FormValue("cmisaction")

	repoID := chi.URLParam(r, "repoId")

	switch action {
	case "query":
		h.Query(w, r, repoID)
	default:
		writeError(w, 400, "invalidArgument", "Unknown cmisaction: "+action)
	}
}

// dispatchRootGet dispatches GET /cmis/{repoId}/root based on cmisselector.
func (h *Handler) dispatchRootGet(w http.ResponseWriter, r *http.Request) {
	r = h.withCS3Token(r)
	repoID := chi.URLParam(r, "repoId")
	selector := r.URL.Query().Get("cmisselector")

	ref, err := refFromObjectID(repoID)
	if err != nil {
		writeError(w, 400, "invalidArgument", "Invalid repository ID")
		return
	}

	switch selector {
	case "object", "":
		h.GetObject(w, r, ref)
	case "children":
		h.GetChildren(w, r, ref)
	case "parents":
		h.GetObjectParents(w, r, ref)
	case "parent":
		h.GetFolderParent(w, r, ref)
	case "allowableActions":
		h.getAllowableActions(w, r, ref)
	case "acl":
		h.GetACL(w, r, ref)
	case "versions":
		h.GetAllVersions(w, r, ref)
	case "descendants":
		h.GetDescendants(w, r, ref)
	case "foldertree":
		h.GetFolderTree(w, r, ref)
	case "renditions":
		h.GetRenditions(w, r, ref)
	default:
		writeError(w, 400, "invalidArgument", "Unknown cmisselector: "+selector)
	}
}

// dispatchRootPost dispatches POST /cmis/{repoId}/root based on cmisaction.
func (h *Handler) dispatchRootPost(w http.ResponseWriter, r *http.Request) {
	r = h.withCS3Token(r)
	repoID := chi.URLParam(r, "repoId")
	r.ParseMultipartForm(32 << 20)
	action := r.FormValue("cmisaction")

	ref, err := refFromObjectID(repoID)
	if err != nil {
		writeError(w, 400, "invalidArgument", "Invalid repository ID")
		return
	}

	h.dispatchAction(w, r, ref, action)
}

// dispatchObjectGet dispatches GET /cmis/{repoId}/root/{path} based on cmisselector.
func (h *Handler) dispatchObjectGet(w http.ResponseWriter, r *http.Request) {
	r = h.withCS3Token(r)
	repoID := chi.URLParam(r, "repoId")
	objPath := chi.URLParam(r, "*")
	selector := r.URL.Query().Get("cmisselector")

	ref := h.buildPathRef(repoID, objPath)

	switch selector {
	case "object", "":
		h.GetObject(w, r, ref)
	case "children":
		h.GetChildren(w, r, ref)
	case "content":
		h.GetContentStream(w, r, ref)
	case "parents":
		h.GetObjectParents(w, r, ref)
	case "parent":
		h.GetFolderParent(w, r, ref)
	case "versions":
		h.GetAllVersions(w, r, ref)
	case "allowableActions":
		h.getAllowableActions(w, r, ref)
	case "acl":
		h.GetACL(w, r, ref)
	case "descendants":
		h.GetDescendants(w, r, ref)
	case "foldertree":
		h.GetFolderTree(w, r, ref)
	case "renditions":
		h.GetRenditions(w, r, ref)
	default:
		writeError(w, 400, "invalidArgument", "Unknown cmisselector: "+selector)
	}
}

// dispatchObjectPost dispatches POST /cmis/{repoId}/root/{path} based on cmisaction.
func (h *Handler) dispatchObjectPost(w http.ResponseWriter, r *http.Request) {
	r = h.withCS3Token(r)
	repoID := chi.URLParam(r, "repoId")
	objPath := chi.URLParam(r, "*")
	r.ParseMultipartForm(32 << 20)
	action := r.FormValue("cmisaction")

	ref := h.buildPathRef(repoID, objPath)
	h.dispatchAction(w, r, ref, action)
}

// dispatchAction routes a POST cmisaction to the right handler.
func (h *Handler) dispatchAction(w http.ResponseWriter, r *http.Request, ref *provider.Reference, action string) {
	switch action {
	case "createDocument":
		h.CreateDocument(w, r, ref)
	case "createFolder":
		h.CreateFolder(w, r, ref)
	case "update":
		h.UpdateProperties(w, r, ref)
	case "delete":
		h.DeleteObject(w, r, ref)
	case "deleteTree":
		h.DeleteTree(w, r, ref)
	case "setContent":
		h.SetContentStream(w, r, ref)
	case "checkOut":
		h.CheckOut(w, r, ref)
	case "checkIn":
		h.CheckIn(w, r, ref)
	case "cancelCheckOut":
		h.CancelCheckOut(w, r, ref)
	case "applyACL":
		h.ApplyACL(w, r, ref)
	case "move":
		h.MoveObject(w, r, ref)
	case "deleteContent":
		h.DeleteContentStream(w, r, ref)
	default:
		writeError(w, 400, "invalidArgument", "Unknown cmisaction: "+action)
	}
}

// buildPathRef creates a CS3 Reference for a path relative to a repository root.
func (h *Handler) buildPathRef(repoID, objPath string) *provider.Reference {
	rid, err := decodeObjectID(repoID)
	if err != nil {
		return &provider.Reference{Path: "/" + objPath}
	}
	relPath := "./" + objPath
	if objPath == "" {
		relPath = "."
	}
	return &provider.Reference{
		ResourceId: rid,
		Path:       relPath,
	}
}

// getAllowableActions returns just the allowable actions for an object.
func (h *Handler) getAllowableActions(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	res, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "objectNotFound", "Object not found")
		return
	}
	writeJSON(w, 200, mapAllowableActions(res.Info))
}

// --- Response helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if status != 0 {
		w.WriteHeader(status)
	}
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, exception, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"exception": exception,
		"message":   message,
	})
}
