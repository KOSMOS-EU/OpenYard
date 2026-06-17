package http

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	userpb "github.com/cs3org/go-cs3apis/cs3/identity/user/v1beta1"

	"github.com/kosmos-eu/openyard/pkg/auth"
	"github.com/kosmos-eu/openyard/pkg/config"
)

func newTestService() (*Service, *auth.SessionCache) {
	sessions := auth.NewSessionCache(1 * time.Hour)
	cfg := config.Load()
	svc := NewService(nil, sessions, cfg)
	return svc, sessions
}

func TestServiceIsListeningRoute(t *testing.T) {
	svc, _ := newTestService()

	req := httptest.NewRequest("GET", "/api/advancedGeneral/IsListening", nil)
	w := httptest.NewRecorder()
	svc.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "true" {
		t.Errorf("body = %q, want \"true\"", w.Body.String())
	}
}

func TestServiceGetServerSettingsRoute(t *testing.T) {
	svc, _ := newTestService()

	req := httptest.NewRequest("GET", "/api/advancedGeneral/GetServerSettings", nil)
	w := httptest.NewRecorder()
	svc.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestServiceProtectedRouteNoSession(t *testing.T) {
	svc, _ := newTestService()

	// All protected endpoints should return 401 without SessionID
	endpoints := []struct {
		method string
		path   string
	}{
		{"POST", "/api/advancedDocuments/GetDocument"},
		{"POST", "/api/advancedFolders/GetFolder"},
		{"POST", "/api/advancedCommon/CopyObjects"},
		{"POST", "/api/advancedUsers/GetUserInfo"},
		{"GET", "/api/advancedDocuments/IsDocument"},
		{"GET", "/api/advancedFolders/IsFolder"},
		{"GET", "/api/advancedCommon/ExistObjectId"},
		{"GET", "/api/basicConfig/GetOpenyardDMSUserSettings"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, nil)
		w := httptest.NewRecorder()
		svc.ServeHTTP(w, req)

		if w.Code != 401 {
			t.Errorf("%s %s: status = %d, want 401", ep.method, ep.path, w.Code)
		}
	}
}

func TestServiceProtectedRouteWithSession(t *testing.T) {
	svc, sessions := newTestService()

	user := &userpb.User{
		Id:       &userpb.UserId{OpaqueId: "u1"},
		Username: "testuser",
		Mail:     "test@example.com",
	}
	sid := sessions.Create(&auth.SessionContext{User: user, CS3Token: "token", Login: user.Username, UserID: user.Id.OpaqueId})

	// These endpoints need a session but no real CS3 gateway
	// They should not return 401
	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/basicConfig/GetOpenyardDMSUserSettings"},
		{"GET", "/api/basicFolders/GetFolderTemplates"},
		{"GET", "/api/ApiExplorer/GetApiDescriptions"},
		{"GET", "/api/basicGeneral/GetEnumerationAsDictionary"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path+"?SessionID="+sid, nil)
		w := httptest.NewRecorder()
		svc.ServeHTTP(w, req)

		if w.Code == 401 {
			t.Errorf("%s %s: got 401 with valid session", ep.method, ep.path)
		}
		if w.Code != 200 {
			t.Errorf("%s %s: status = %d, want 200", ep.method, ep.path, w.Code)
		}
	}
}

func TestServiceLoginRoute(t *testing.T) {
	svc, _ := newTestService()

	// Login without gateway should return 400 or 500, not panic
	body := `{"Username": "admin", "Password": "secret"}`
	req := httptest.NewRequest("POST", "/api/advancedUsers/Login", strings.NewReader(body))
	w := httptest.NewRecorder()
	svc.ServeHTTP(w, req)

	// Should fail gracefully (no CS3 gateway), not 401 (that's for bad creds)
	if w.Code == 200 {
		t.Error("Login should not succeed without CS3 gateway")
	}
	// Should not panic
	if w.Code == 0 {
		t.Error("No response")
	}
}

func TestServiceLogoutRoute(t *testing.T) {
	svc, sessions := newTestService()

	user := &userpb.User{
		Id:       &userpb.UserId{OpaqueId: "u2"},
		Username: "logoutuser",
	}
	sid := sessions.Create(&auth.SessionContext{User: user, CS3Token: "token", Login: user.Username, UserID: user.Id.OpaqueId})

	req := httptest.NewRequest("GET", "/api/advancedUsers/Logout?SessionID="+sid, nil)
	w := httptest.NewRecorder()
	svc.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	if sessions.Get(sid) != nil {
		t.Error("session should be deleted")
	}
}

func TestService404(t *testing.T) {
	svc, _ := newTestService()

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	svc.ServeHTTP(w, req)

	if w.Code != 404 && w.Code != 405 {
		t.Errorf("status = %d, want 404 or 405", w.Code)
	}
}

func TestServiceAuthenticateRoute(t *testing.T) {
	svc, _ := newTestService()

	// Without SessionID
	req := httptest.NewRequest("GET", "/api/advancedUsers/Authenticate", nil)
	w := httptest.NewRecorder()
	svc.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestServiceContentType(t *testing.T) {
	svc, _ := newTestService()

	req := httptest.NewRequest("GET", "/api/advancedGeneral/IsListening", nil)
	w := httptest.NewRecorder()
	svc.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
