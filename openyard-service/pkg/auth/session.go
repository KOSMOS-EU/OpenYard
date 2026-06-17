package auth

import (
	"crypto/rand"
	"fmt"
	"time"

	userpb "github.com/cs3org/go-cs3apis/cs3/identity/user/v1beta1"
	gocache "github.com/patrickmn/go-cache"
)

// SessionContext holds user context for an active session.
type SessionContext struct {
	User        *userpb.User
	CS3Token    string
	AccessToken string // JWT from IDP or generated
	Created     time.Time
	Login       string // Username used for login
	Password    string // Password for WebDAV Basic auth
	Fullname    string
	UserID      string // GUID-style user ID
	IsAdmin     bool
}

// SessionCache manages SessionID → SessionContext mappings.
type SessionCache struct {
	cache *gocache.Cache
}

func NewSessionCache(ttl time.Duration) *SessionCache {
	return &SessionCache{
		cache: gocache.New(ttl, ttl/2),
	}
}

// Create generates a new SessionID (GUID format) and stores the context.
func (s *SessionCache) Create(sc *SessionContext) string {
	sid := generateGUID()
	sc.Created = time.Now()
	s.cache.Set(sid, sc, gocache.DefaultExpiration)
	return sid
}

// Get retrieves a session by ID. Returns nil if expired or not found.
func (s *SessionCache) Get(sessionID string) *SessionContext {
	v, ok := s.cache.Get(sessionID)
	if !ok {
		return nil
	}
	return v.(*SessionContext)
}

// Delete removes a session.
func (s *SessionCache) Delete(sessionID string) {
	s.cache.Delete(sessionID)
}

// generateGUID creates a UUID v4 string matching WinYard SessionID format.
func generateGUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	// Set version 4 and variant bits
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
