package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	gateway "github.com/cs3org/go-cs3apis/cs3/gateway/v1beta1"
	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/metadata"

	"github.com/kosmos-eu/openyard/pkg/auth"
	"github.com/kosmos-eu/openyard/pkg/config"
)

// POST /api/advancedUsers/Login
//
// WinYard clients send:
//   - Authorization: Basic Og== (dummy, empty user:pass)
//   - Body: JSON search query (NOT username/password)
//   - Auth happens via OIDC beforehand (connect.authorize on IDP)
//
// For OpenYard we support two modes:
//   A) OIDC/dummy mode: Basic Og== + any body → auth via CS3 with configured service account
//   B) Direct mode: body with Username/Password → auth via CS3
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	// Read body (may be search query or login credentials)
	bodyBytes, _ := io.ReadAll(r.Body)

	var cs3User *gateway.AuthenticateResponse
	var loginName string

	// Try direct login — WinYard sends {"username":"...","Password":"..."}
	// Accept both lowercase and uppercase variants
	var directLogin map[string]interface{}
	if len(bodyBytes) > 0 {
		json.Unmarshal(bodyBytes, &directLogin)
	}

	// Extract username (case-insensitive)
	loginUser := ""
	loginPass := ""
	if directLogin != nil {
		for k, v := range directLogin {
			switch strings.ToLower(k) {
			case "username":
				loginUser, _ = v.(string)
			case "password":
				loginPass, _ = v.(string)
			}
		}
	}

	if loginUser != "" && loginPass != "" {
		// Mode B: Direct login
		if h.gw == nil || h.gw.Gateway == nil {
			writeError(w, 500, "INTERNAL_ERROR", "CS3 gateway not configured")
			return
		}
		res, err := h.gw.Gateway.Authenticate(r.Context(), &gateway.AuthenticateRequest{
			Type:         "basic",
			ClientId:     loginUser,
			ClientSecret: loginPass,
		})
		if err != nil {
			log.Error().Err(err).Msg("cs3 authenticate failed")
			writeError(w, 500, "INTERNAL_ERROR", "Authentication service unavailable")
			return
		}
		if res.Status.Code != rpc.Code_CODE_OK {
			writeError(w, 401, "AUTH_REQUIRED", "Invalid credentials")
			return
		}
		cs3User = res
		loginName = loginUser
	} else {
		// Mode A: OIDC/dummy — accept Basic Og== or any auth
		// In production this would validate the OIDC token from IDP
		// For now, authenticate with configured service account
		cfg := config.Load()
		if h.gw != nil && h.gw.Gateway != nil && cfg.ServiceAccount.User != "" {
			res, err := h.gw.Gateway.Authenticate(r.Context(), &gateway.AuthenticateRequest{
				Type:         "basic",
				ClientId:     cfg.ServiceAccount.User,
				ClientSecret: cfg.ServiceAccount.Pass,
			})
			if err == nil && res.Status.Code == rpc.Code_CODE_OK {
				cs3User = res
				loginName = cfg.ServiceAccount.User
			}
		}
		if cs3User == nil {
			// No gateway or service account — create session without CS3
			loginName = "openyard"
		}
	}

	// Build session context
	sc := &auth.SessionContext{
		Login:    loginName,
		Password: loginPass,
		Fullname: loginName,
		UserID:   "00000000-0000-0000-0000-000000000000",
	}
	if cs3User != nil {
		sc.User = cs3User.User
		sc.CS3Token = cs3User.Token
		sc.AccessToken = cs3User.Token
		if cs3User.User != nil {
			sc.Login = cs3User.User.Username
			sc.Fullname = cs3User.User.DisplayName
			sc.UserID = cs3User.User.Id.OpaqueId
		}
	}

	sessionID := h.sessions.Create(sc)

	// WinYard-compatible login response
	writeJSON(w, 200, buildLoginResponse(sessionID, sc))
	log.Info().Str("user", sc.Login).Str("session", sessionID[:8]+"...").Msg("login")
}

// buildLoginResponse creates a WinYard-compatible login response
// matching the structure observed in captures.
func buildLoginResponse(sessionID string, sc *auth.SessionContext) map[string]interface{} {
	return map[string]interface{}{
		"SessionId":       sessionID,
		"AccessToken":     sc.AccessToken,
		"RefreshToken":    nil,
		"ExpiresIn":       0,
		"Login":           sc.Login,
		"Fullname":        sc.Fullname,
		"Firstname":       "",
		"Lastname":        sc.Fullname,
		"UserId":          sc.UserID,
		"LoginUserId":     sc.UserID,
		"UserIdInCharge":  sc.UserID,
		"Id":              0,
		"IdInCharge":      0,
		"Admin":           sc.IsAdmin,
		"WinyardAdmin":    false,
		"Archive":         true,
		"Index":           false,
		"Metadata":        false,
		"DSGVO":           false,
		"SystemUser":      false,
		"Locked":          0,
		"Activity_Status": 1,
		"Email":           nil,
		"Department":      nil,
		"Department_ID":   "00000000-0000-0000-0000-000000000000",
		"InSubstitution":  false,
		"Recyclebin_Days": 0,
		"Description":     nil,
		"Manager":         nil,
		"PadSign":         nil,
		"IsSubscriptionAdmin": false,
		"IsTemplateAdmin":     false,
		"Publish_Address":     nil,
		"User_Folder_ID":      nil,
		"ClientValues": map[string]interface{}{
			"CurrentObject":    nil,
			"LastUsedFolder":   nil,
			"LastUsedDocument": nil,
		},
		"UserGroups": map[string]interface{}{},
		"Details":    map[string]interface{}{},
		"DetailTypes": []interface{}{},
		"TaskLog": map[string]interface{}{
			"Canceled":    false,
			"hasError":    false,
			"ErrorType":   0,
			"TaskName":    nil,
			"SessionUser": nil,
		},
		"myWait": map[string]interface{}{
			"MustWait":         false,
			"WaitReason":       nil,
			"WaitTimeDuration": 0,
		},
	}
}

// GET /api/advancedUsers/Logout
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("SessionID")
	if sessionID != "" {
		h.sessions.Delete(sessionID)
		log.Info().Str("session", sessionID[:8]+"...").Msg("logout")
	}
	// WinYard returns TaskLog on logout
	writeJSON(w, 200, map[string]interface{}{
		"TaskLog": map[string]interface{}{
			"Canceled":  false,
			"hasError":  false,
			"ErrorType": 0,
		},
	})
}

// GET /api/advancedUsers/Authenticate
func (h *Handlers) Authenticate(w http.ResponseWriter, r *http.Request) {
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

	// Optionally verify CS3 token still valid
	if h.gw != nil && h.gw.Gateway != nil && sess.CS3Token != "" {
		ctx := metadata.AppendToOutgoingContext(r.Context(), "x-access-token", sess.CS3Token)
		res, err := h.gw.Gateway.WhoAmI(ctx, &gateway.WhoAmIRequest{Token: sess.CS3Token})
		if err != nil || res.Status.Code != rpc.Code_CODE_OK {
			h.sessions.Delete(sessionID)
			writeError(w, 401, "AUTH_REQUIRED", "Session expired")
			return
		}
	}

	writeJSON(w, 200, map[string]interface{}{
		"authenticated": true,
		"username":      sess.Login,
	})
}

// GET /api/advancedUsers/GetSessionBySessionID
func (h *Handlers) GetSessionBySessionID(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("SessionID")
	if sessionID == "" {
		writeError(w, 400, "INVALID_REQUEST", "SessionID required")
		return
	}
	sess := h.sessions.Get(sessionID)
	if sess == nil {
		writeError(w, 404, "NOT_FOUND", "Session not found")
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"SessionId":      sessionID,
		"UserId":         sess.UserID,
		"Login":          sess.Login,
		"Fullname":       sess.Fullname,
		"LoginTime":      sess.Created.Format(time.RFC3339),
		"Activity_Status": 1,
		"isActive":       true,
	})
}
