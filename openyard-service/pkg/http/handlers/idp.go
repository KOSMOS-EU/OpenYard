package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// --- IDP / Identity Server Endpoints ---
// Mapped from Winyard.Identity.WebApi (svb-dms01:52329)
// These run on the same port as the DMS API.

// GET /connect/authorize — OIDC Authorization
// WinYard client calls this to initiate OIDC flow.
// In OpenYard we redirect to OpenCloud's OIDC.
func (h *Handlers) ConnectAuthorize(w http.ResponseWriter, r *http.Request) {
	// Phase 1: stub — return 302 to simulate OIDC redirect
	// Phase 2: proxy to OpenCloud OIDC
	log.Info().Str("query", r.URL.RawQuery).Msg("connect.authorize called")
	writeJSON(w, 200, map[string]interface{}{
		"error":             "not_implemented",
		"error_description": "OIDC flow not yet proxied to OpenCloud",
	})
}

// GET /api/Licenses/GetLicensee
func (h *Handlers) GetLicensee(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"Licensee":    "OpenYard Community",
		"LicenseType": "community",
		"ValidUntil":  time.Now().AddDate(1, 0, 0).Format("2006-01-02"),
		"MaxUsers":    999,
		"Modules":     []string{"DMS", "Workflow", "Archive"},
	})
}

// GET /api/Licenses/GetActiveLicenses
func (h *Handlers) GetActiveLicenses(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, []interface{}{
		map[string]interface{}{
			"Name":      "OpenYard DMS",
			"IsActive":  true,
			"ExpiresAt": time.Now().AddDate(1, 0, 0).Format("2006-01-02"),
		},
	})
}

// GET /api/People — List all people
func (h *Handlers) ListPeople(w http.ResponseWriter, r *http.Request) {
	// Phase 1: return from session context or empty list
	// Phase 2: proxy to OpenCloud Graph API users
	sess := sessionFromCtx(r.Context())
	if sess != nil && sess.User != nil {
		writeJSON(w, 200, map[string]interface{}{
			"$values": []interface{}{
				map[string]interface{}{
					"UserId":    sess.UserID,
					"FirstName": sess.Login,
					"LastName":  sess.Fullname,
					"Email":     "",
					"Deleted":   false,
				},
			},
		})
		return
	}
	writeJSON(w, 200, map[string]interface{}{
		"$values": []interface{}{},
	})
}

// GET /api/People/{id}
func (h *Handlers) GetPerson(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"UserId":    "00000000-0000-0000-0000-000000000000",
		"FirstName": "",
		"LastName":  "",
		"Deleted":   false,
	})
}

// GET /api/People/GetPersonByUserId
func (h *Handlers) GetPersonByUserId(w http.ResponseWriter, r *http.Request) {
	h.GetPerson(w, r)
}

// GET /api/ApplicationRoles — List application roles
func (h *Handlers) ListApplicationRoles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"$values": []interface{}{
			map[string]interface{}{
				"Id":          8,
				"Name":        "Anwendungsrollen.Winyard-Benutzer",
				"Description": "Winyard Benutzer",
				"Guid":        "00000008-0000-0000-0000-000000000000",
			},
			map[string]interface{}{
				"Id":          10,
				"Name":        "Anwendungsrollen.Winyard-Benutzer.Winyard.DMS-Benutzer",
				"Description": "Winyard DMS Benutzer",
				"Guid":        "0000000a-0000-0000-0000-000000000000",
			},
		},
	})
}

// GET /api/ApplicationRoles/GetAll
func (h *Handlers) GetAllApplicationRoles(w http.ResponseWriter, r *http.Request) {
	h.ListApplicationRoles(w, r)
}

// GET /api/BusinessRoles — List business roles
func (h *Handlers) ListBusinessRoles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"$values": []interface{}{},
	})
}

// GET /api/Departments — List departments
func (h *Handlers) ListDepartments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"$values": []interface{}{},
	})
}

// GET /api/Substitutions — List substitutions (Vertretungen)
func (h *Handlers) ListSubstitutions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"$values": []interface{}{},
	})
}

// POST /api/ClientConfiguration — Client config endpoint
func (h *Handlers) PostClientConfiguration(w http.ResponseWriter, r *http.Request) {
	// Read body and return config
	var body json.RawMessage
	json.NewDecoder(r.Body).Decode(&body)
	writeJSON(w, 200, map[string]interface{}{})
}

// GET /api/Customers/GetIdSrvUrl/{prefix}
func (h *Handlers) GetIdSrvUrl(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"Url": "http://localhost:9201",
	})
}

// IDP Swagger stub
func (h *Handlers) IDPSwagger(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]interface{}{
		"info": map[string]string{
			"title":   "OpenYard Identity WebApi",
			"version": "v1",
		},
	})
}
