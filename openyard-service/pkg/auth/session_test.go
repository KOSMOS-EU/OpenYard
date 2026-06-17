package auth

import (
	"strings"
	"testing"
	"time"

	userpb "github.com/cs3org/go-cs3apis/cs3/identity/user/v1beta1"
)

func testContext(user *userpb.User, token string) *SessionContext {
	return &SessionContext{
		User:     user,
		CS3Token: token,
		Login:    user.Username,
		Fullname: user.DisplayName,
		UserID:   user.Id.OpaqueId,
	}
}

func TestSessionCreateAndGet(t *testing.T) {
	cache := NewSessionCache(1 * time.Hour)

	user := &userpb.User{
		Id:          &userpb.UserId{OpaqueId: "user-123"},
		Username:    "testuser",
		DisplayName: "Test User",
		Mail:        "test@example.com",
	}

	sid := cache.Create(testContext(user, "cs3-token-abc"))
	if sid == "" {
		t.Fatal("Create returned empty SessionID")
	}
	// Should be GUID format: 8-4-4-4-12
	if len(sid) != 36 || strings.Count(sid, "-") != 4 {
		t.Errorf("SessionID not GUID format: %q", sid)
	}

	sess := cache.Get(sid)
	if sess == nil {
		t.Fatal("Get returned nil")
	}
	if sess.User.Username != "testuser" {
		t.Errorf("Username = %q", sess.User.Username)
	}
	if sess.CS3Token != "cs3-token-abc" {
		t.Errorf("CS3Token = %q", sess.CS3Token)
	}
	if sess.Login != "testuser" {
		t.Errorf("Login = %q", sess.Login)
	}
	if sess.Created.IsZero() {
		t.Error("Created is zero")
	}
}

func TestSessionGetNotFound(t *testing.T) {
	cache := NewSessionCache(1 * time.Hour)

	if sess := cache.Get("nonexistent"); sess != nil {
		t.Error("expected nil for nonexistent session")
	}
}

func TestSessionDelete(t *testing.T) {
	cache := NewSessionCache(1 * time.Hour)

	user := &userpb.User{
		Id:       &userpb.UserId{OpaqueId: "user-1"},
		Username: "deletetest",
	}

	sid := cache.Create(testContext(user, "token"))
	if cache.Get(sid) == nil {
		t.Fatal("session should exist after create")
	}

	cache.Delete(sid)
	if cache.Get(sid) != nil {
		t.Error("session should be nil after delete")
	}
}

func TestSessionExpiry(t *testing.T) {
	cache := NewSessionCache(50 * time.Millisecond)

	user := &userpb.User{
		Id:       &userpb.UserId{OpaqueId: "user-2"},
		Username: "expirytest",
	}

	sid := cache.Create(testContext(user, "token"))
	if cache.Get(sid) == nil {
		t.Fatal("session should exist immediately")
	}

	time.Sleep(100 * time.Millisecond)
	if cache.Get(sid) != nil {
		t.Error("session should have expired")
	}
}

func TestSessionUniqueIDs(t *testing.T) {
	cache := NewSessionCache(1 * time.Hour)

	user := &userpb.User{
		Id:       &userpb.UserId{OpaqueId: "user-3"},
		Username: "unique",
	}

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		sid := cache.Create(testContext(user, "token"))
		if seen[sid] {
			t.Fatalf("duplicate SessionID at iteration %d: %s", i, sid)
		}
		seen[sid] = true
	}
}

func TestSessionGUIDFormat(t *testing.T) {
	cache := NewSessionCache(1 * time.Hour)
	user := &userpb.User{
		Id:       &userpb.UserId{OpaqueId: "u1"},
		Username: "test",
	}
	sid := cache.Create(testContext(user, "t"))

	// GUID: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	parts := strings.Split(sid, "-")
	if len(parts) != 5 {
		t.Fatalf("GUID should have 5 parts: %q", sid)
	}
	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		t.Errorf("GUID part lengths wrong: %q", sid)
	}
	// Version 4
	if parts[2][0] != '4' {
		t.Errorf("GUID version should be 4: %q", sid)
	}
}
