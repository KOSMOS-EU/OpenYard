package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	userpb "github.com/cs3org/go-cs3apis/cs3/identity/user/v1beta1"
	"github.com/go-chi/chi/v5"

	"github.com/kosmos-eu/openyard/pkg/auth"
)

// newTestHandlers creates handlers with nil gateway (for non-CS3 tests).
func newTestHandlers() (*Handlers, *auth.SessionCache) {
	sessions := auth.NewSessionCache(1 * time.Hour)
	h := &Handlers{gw: nil, sessions: sessions}
	return h, sessions
}

func TestIsListening(t *testing.T) {
	h, _ := newTestHandlers()

	req := httptest.NewRequest("GET", "/api/advancedGeneral/IsListening", nil)
	w := httptest.NewRecorder()

	h.IsListening(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	// legacy DMS returns plain "true"
	if w.Body.String() != "true" {
		t.Errorf("body = %q, want \"true\"", w.Body.String())
	}
}

func TestGetServerSettings(t *testing.T) {
	h, _ := newTestHandlers()

	req := httptest.NewRequest("GET", "/api/advancedGeneral/GetServerSettings", nil)
	w := httptest.NewRecorder()

	h.GetServerSettings(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)

	settings := body["Settings"].(map[string]interface{})
	if settings["ApiVersion"] != "Core 1.1" {
		t.Errorf("ApiVersion = %v", settings["ApiVersion"])
	}
	if settings["ProductName"] != "OpenYard DMS" {
		t.Errorf("ProductName = %v", settings["ProductName"])
	}
}

func TestSessionMiddlewareNoSession(t *testing.T) {
	h, _ := newTestHandlers()

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()

	h.SessionMiddleware(inner).ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if called {
		t.Error("inner handler should not be called")
	}
}

func TestSessionMiddlewareInvalidSession(t *testing.T) {
	h, _ := newTestHandlers()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest("GET", "/api/test?SessionID=bogus", nil)
	w := httptest.NewRecorder()

	h.SessionMiddleware(inner).ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestSessionMiddlewareValidSession(t *testing.T) {
	h, sessions := newTestHandlers()

	user := &userpb.User{
		Id:       &userpb.UserId{OpaqueId: "u1"},
		Username: "testuser",
	}
	sid := sessions.Create(&auth.SessionContext{User: user, CS3Token: "token", Login: user.Username, UserID: user.Id.OpaqueId})

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		sess := sessionFromCtx(r.Context())
		if sess == nil {
			t.Error("session not in context")
		}
		if sess.User.Username != "testuser" {
			t.Errorf("username = %q", sess.User.Username)
		}
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/api/test?SessionID="+sid, nil)
	w := httptest.NewRecorder()

	h.SessionMiddleware(inner).ServeHTTP(w, req)

	if !called {
		t.Error("inner handler not called")
	}
	if w.Code != 200 {
		t.Errorf("status = %d", w.Code)
	}
}

func TestLogoutDeletesSession(t *testing.T) {
	h, sessions := newTestHandlers()

	user := &userpb.User{
		Id:       &userpb.UserId{OpaqueId: "u2"},
		Username: "logouttest",
	}
	sid := sessions.Create(&auth.SessionContext{User: user, CS3Token: "token", Login: user.Username, UserID: user.Id.OpaqueId})

	req := httptest.NewRequest("GET", "/api/advancedUsers/Logout?SessionID="+sid, nil)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if sessions.Get(sid) != nil {
		t.Error("session should be deleted after logout")
	}
}

func TestGetSessionBySessionID(t *testing.T) {
	h, sessions := newTestHandlers()

	user := &userpb.User{
		Id:       &userpb.UserId{OpaqueId: "u3"},
		Username: "sessiontest",
	}
	sid := sessions.Create(&auth.SessionContext{User: user, CS3Token: "token", Login: user.Username, UserID: user.Id.OpaqueId})

	req := httptest.NewRequest("GET", "/api/advancedUsers/GetSessionBySessionID?SessionID="+sid, nil)
	w := httptest.NewRecorder()

	h.GetSessionBySessionID(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)

	if body["Login"] != "sessiontest" {
		t.Errorf("Login = %v", body["Login"])
	}
	if body["isActive"] != true {
		t.Errorf("isActive = %v", body["isActive"])
	}
}

func TestGetSessionBySessionIDNotFound(t *testing.T) {
	h, _ := newTestHandlers()

	req := httptest.NewRequest("GET", "/api/advancedUsers/GetSessionBySessionID?SessionID=nonexistent", nil)
	w := httptest.NewRecorder()

	h.GetSessionBySessionID(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestLoginDummyMode(t *testing.T) {
	h, _ := newTestHandlers()

	// Mode A: empty body or non-credentials body → accepted (OIDC dummy mode)
	req := httptest.NewRequest("POST", "/api/advancedUsers/Login", strings.NewReader("{}"))
	w := httptest.NewRecorder()

	h.Login(w, req)

	// Should succeed with dummy session (no CS3 gateway)
	if w.Code != 200 {
		t.Errorf("status = %d, want 200 (dummy mode)", w.Code)
	}
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["SessionId"] == nil {
		t.Error("SessionId should be present")
	}
}

func TestLoginDirectModeNoGateway(t *testing.T) {
	h, _ := newTestHandlers()

	// Mode B: Username/Password but no CS3 gateway → 500
	req := httptest.NewRequest("POST", "/api/advancedUsers/Login",
		strings.NewReader(`{"Username":"admin","Password":"secret"}`))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != 500 {
		t.Errorf("status = %d, want 500 (no gateway)", w.Code)
	}
}

func TestDMSUserSettings(t *testing.T) {
	h, _ := newTestHandlers()

	req := httptest.NewRequest("GET", "/api/basicConfig/GetOpenyardDMSUserSettings", nil)
	w := httptest.NewRecorder()

	h.GetDMSUserSettings(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestFolderTemplates(t *testing.T) {
	h, _ := newTestHandlers()

	req := httptest.NewRequest("GET", "/api/basicFolders/GetFolderTemplates", nil)
	w := httptest.NewRecorder()

	h.GetFolderTemplates(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["templates"] == nil {
		t.Error("templates should be present")
	}
}

func TestErrorFormat(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, 404, "NOT_FOUND", "Resource not found")

	if w.Code != 404 {
		t.Fatalf("status = %d", w.Code)
	}

	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)

	if body["error"] != "Resource not found" {
		t.Errorf("error = %q", body["error"])
	}
	if body["code"] != "NOT_FOUND" {
		t.Errorf("code = %q", body["code"])
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestFullRouterIsListening(t *testing.T) {
	sessions := auth.NewSessionCache(1 * time.Hour)
	h := New(nil, sessions, nil)

	r := chi.NewMux()
	r.Get("/api/advancedGeneral/IsListening", h.IsListening)

	req := httptest.NewRequest("GET", "/api/advancedGeneral/IsListening", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}
