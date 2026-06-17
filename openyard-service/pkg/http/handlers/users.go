package handlers

import (
	"net/http"

	"google.golang.org/grpc/metadata"

	gateway "github.com/cs3org/go-cs3apis/cs3/gateway/v1beta1"
	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
)

// POST /api/advancedUsers/GetUserInfo
func (h *Handlers) GetUserInfo(w http.ResponseWriter, r *http.Request) {
	sess := sessionFromCtx(r.Context())
	if sess == nil {
		writeError(w, 401, "AUTH_REQUIRED", "Not authenticated")
		return
	}

	ctx := metadata.AppendToOutgoingContext(r.Context(), "x-access-token", sess.CS3Token)
	res, err := h.gw.Gateway.WhoAmI(ctx, &gateway.WhoAmIRequest{Token: sess.CS3Token})
	if err != nil || res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "INTERNAL_ERROR", "Could not retrieve user info")
		return
	}

	u := res.User
	writeJSON(w, 200, map[string]interface{}{
		"id":          u.Id.OpaqueId,
		"username":    u.Username,
		"displayName": u.DisplayName,
		"email":       u.Mail,
		"status":      "active",
		// Field-Default-Engine: WinYard-spezifische Felder mit Defaults
		"Funktion":             "",
		"Telefon":              "",
		"SachbearbeiterKennung": "",
		"Adresse":              "N/A",
		"Geburtsdatum":         nil,
	})
}
