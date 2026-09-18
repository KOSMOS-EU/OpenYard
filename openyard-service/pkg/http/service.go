package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/rs/zerolog/log"

	"github.com/kosmos-eu/openyard/pkg/auth"
	"github.com/kosmos-eu/openyard/pkg/config"
	"github.com/kosmos-eu/openyard/pkg/cs3client"
	"github.com/kosmos-eu/openyard/pkg/http/cmis"
	"github.com/kosmos-eu/openyard/pkg/http/handlers"
	"github.com/kosmos-eu/openyard/pkg/upload"
)

type Service struct {
	mux *chi.Mux
}

func NewService(gw *cs3client.Client, sessions *auth.SessionCache, cfg *config.Config) *Service {
	m := chi.NewMux()
	m.Use(middleware.RealIP)
	m.Use(middleware.Recoverer)
	m.Use(middleware.RequestID)
	m.Use(requestLogger)

	// Select upload method
	var up upload.Uploader
	switch cfg.Upload.Method {
	case "reva":
		up = &upload.Reva{GW: gw, DataGatewayURL: cfg.Upload.BaseURL}
	default:
		up = &upload.WebDAV{BaseURL: cfg.Upload.BaseURL}
	}
	log.Info().Str("method", up.Name()).Msg("upload method")

	h := handlers.New(gw, sessions, up, cfg.Upload.BaseURL, cfg.Upload.DownloadURL)

	m.Route("/api", func(r chi.Router) {
		// Auth endpoints (no session required)
		r.Post("/advancedUsers/Login", h.Login)
		r.Get("/advancedUsers/Logout", h.Logout)
		r.Get("/advancedUsers/Authenticate", h.Authenticate)
		r.Get("/advancedUsers/GetSessionBySessionID", h.GetSessionBySessionID)

		// System endpoints (no session required)
		r.Get("/advancedGeneral/IsListening", h.IsListening)
		r.Get("/advancedGeneral/GetServerSettings", h.GetServerSettings)
		r.Get("/ApiExplorer/GetApiDescriptions", h.GetApiDescriptions)

		// IDP endpoints (no session required — called before login)
		r.Get("/Licenses/GetLicensee", h.GetLicensee)
		r.Get("/Licenses/GetActiveLicenses", h.GetActiveLicenses)
		r.Get("/Licenses/GetActiveLicensesNoMeta", h.GetActiveLicenses)
		r.Get("/Licenses/HasLicenseKey", func(w http.ResponseWriter, rr *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(200)
			w.Write([]byte("true"))
		})
		r.Post("/ClientConfiguration", h.PostClientConfiguration)
		r.Get("/Customers/GetIdSrvUrl/{prefix}", h.GetIdSrvUrl)

		// Protected endpoints (SessionID required)
		r.Group(func(r chi.Router) {
			r.Use(h.SessionMiddleware)

			// Users
			r.Post("/advancedUsers/GetUserInfo", h.GetUserInfo)

			// Documents
			r.Post("/advancedDocuments/GetDocument", h.GetDocument)
			r.Post("/advancedDocuments/SetDocument", h.SetDocument)
			r.Post("/advancedDocuments/GetFile", h.GetFile)
			r.Post("/advancedDocuments/SetFile", h.SetFile)
			r.Get("/advancedDocuments/IsDocument", h.IsDocument)
			r.Post("/advancedDocuments/BinDocuments", h.BinDocuments)
			r.Post("/advancedDocuments/GetDeletedFiles", h.GetDeletedFiles)
			r.Get("/advancedDocuments/GetDocumentVersions", h.GetDocumentVersions)
			r.Get("/advancedDocuments/GetPreviewFile", h.GetPreviewFile)
			r.Get("/advancedDocuments/GetMetaData", h.GetDocumentMetaData)

			// Folders
			r.Post("/advancedFolders/GetFolder", h.GetFolder)
			r.Post("/advancedFolders/GetFolderByFolderpath", h.GetFolderByFolderpath)
			r.Get("/advancedFolders/GetFolderByWorkID", h.GetFolderByWorkID)
			r.Get("/advancedFolders/IsFolder", h.IsFolder)
			r.Post("/advancedFolders/SetFolder", h.SetFolder)
			r.Post("/advancedFolders/DeleteFolders", h.DeleteFolders)
			r.Get("/advancedFolders/GetFolderRights", h.GetFolderRights)
			r.Post("/advancedFolders/SetFolderRights", h.SetFolderRights)

			// Common Operations
			r.Post("/advancedCommon/CopyObjects", h.CopyObjects)
			r.Post("/advancedCommon/MoveObjects", h.MoveObjects)
			r.Post("/advancedCommon/RecoverObjects", h.RecoverObjects)
			r.Get("/advancedCommon/ExistObjectId", h.ExistObjectId)
			r.Get("/advancedCommon/ExistObjectPath", h.ExistObjectPath)
			r.Post("/advancedCommon/Search", h.Search)

			// Basic Common
			r.Post("/basicCommon/SearchForTitle", h.SearchForTitle)
			r.Get("/basicCommon/GetMetaData", h.GetBasicMetaData)
			r.Get("/basicCommon/GetIndexData", h.GetIndexData)
			r.Get("/basicCommon/GetExistingPermalinksByObjId", h.GetExistingPermalinksByObjId)
			r.Get("/basicCommon/DeletePermalink", h.DeletePermalink)
			r.Get("/basicCommon/GetObjectIdByPermalink", h.GetObjectIdByPermalink)
			r.Get("/basicCommon/GetPreviewFileByPermalink", h.GetPreviewFileByPermalink)
			r.Get("/basicCommon/GetFolderSubElementsByMemberId", h.GetFolderSubElementsByMemberId)

			// Basic Config
			r.Get("/basicConfig/GetOpenyardDMSUserSettings", h.GetDMSUserSettings)

			// AppConfig management
			r.Post("/advancedConfig/GetAppConfigs", h.GetAppConfigs)
			r.Post("/advancedConfig/SetAppConfig", h.SetAppConfig)
			r.Post("/advancedConfig/DeleteAppConfigs", h.DeleteAppConfigs)
			r.Post("/advancedConfig/GetAppConfigDetailsAsJson", h.GetAppConfigDetailsAsJson)
			r.Post("/advancedConfig/GetAppConfigDetailsAsFile", h.GetAppConfigDetailsAsFile)

			// New endpoints discovered from captures
			r.Post("/advancedDocuments/GetDocTypes", h.GetDocTypes)
			r.Get("/advancedCommon/GetSearchTopics", h.GetSearchTopics)
			r.Get("/basicDocuments/HasPreviewFile", h.HasPreviewFile)
			r.Get("/basicDocuments/DownloadPreviewFile", h.DownloadPreviewFile)
			r.Post("/basicDocuments/ImportDocumentDynamic", h.ImportDocumentDynamic)

			// Extended Documents
			r.Post("/basicDocuments/ImportDocumentByParentFolderID", h.ImportDocumentByParentFolderID)
			r.Post("/basicDocuments/ImportDocumentByParentFolderPath", h.ImportDocumentByParentFolderPath)
			r.Post("/basicDocuments/RenameDocument", h.RenameDocument)
			r.Post("/basicDocuments/UploadFile", h.UploadFile)
			r.Post("/basicDocuments/CheckInAndIgnoreChanges", h.CheckInAndIgnoreChanges)
			r.Post("/basicDocuments/CreateNewDocVersion", h.CreateNewDocVersion)

			// Extended Folders
			r.Post("/basicFolders/CreateFolderByParentFolderID", h.CreateFolderByParentFolderID)
			r.Post("/basicFolders/CreateFolderByParentFolderPath", h.CreateFolderByParentFolderPath)
			r.Post("/basicFolders/RenameFolder", h.RenameFolder)
			r.Get("/basicFolders/GetFolderTemplates", h.GetFolderTemplates)

			// Extended Search & Metadata
			r.Post("/basicCommon/SearchForIndex", h.SearchForIndex)
			r.Post("/basicCommon/SetIndex", h.SetIndex)

			// System Extended
			r.Get("/basicGeneral/GetEnumerationAsDictionary", h.GetEnumerationAsDictionary)

			// Root Folder / Volume / Space management
			r.Post("/basicFolders/CreateRootFolder", h.CreateRootFolder)

			// IDP endpoints (session required)
			r.Get("/People", h.ListPeople)
			r.Get("/People/{id}", h.GetPerson)
			r.Get("/People/GetPersonByUserId", h.GetPersonByUserId)
			r.Get("/ApplicationRoles", h.ListApplicationRoles)
			r.Get("/ApplicationRoles/GetAll", h.GetAllApplicationRoles)
			r.Get("/BusinessRoles", h.ListBusinessRoles)
			r.Get("/BusinessRoles/GetAll", h.ListBusinessRoles)
			r.Get("/Departments", h.ListDepartments)
			r.Get("/Departments/GetAll", h.ListDepartments)
			r.Get("/Substitutions", h.ListSubstitutions)
		})
	})

	// --- Management API (OpenYard-eigene Endpoints) ---
	m.Route("/api/management", func(r chi.Router) {
		r.Use(h.SessionMiddleware)
		r.Get("/migration/status", h.GetMigrationStatus)
		r.Post("/migration/map", h.MapMigrationID)
		r.Post("/migration/persist", h.PersistMigration)
		r.Post("/migration/missing", h.FilterMissingIDs)
		r.Post("/migration/verify", h.VerifyMigration)
		r.Get("/migration/folder-meta", h.GetFolderMeta)
		r.Post("/migration/grant", h.GrantSpaceAccess)
	})

	// --- Health/Stats Endpoints (ohne Session) ---
	m.Get("/openyard/health", h.Health)
	m.Get("/openyard/stats", h.Stats)

	// --- Debug: InitiateFileDownload and return token+target (session required) ---
	m.Route("/openyard/debug", func(r chi.Router) {
		r.Use(h.SessionMiddleware)
		r.Post("/dl-token", h.DLTokenDebug)
	})

	// --- IDP / Identity Server Endpoints ---
	// OIDC endpoint (no session required, separate path)
	m.Get("/connect/authorize", h.ConnectAuthorize)

	// Swagger UI stub
	m.Get("/swagger/", h.IDPSwagger)
	m.Get("/swagger/docs/v1", h.IDPSwagger)

	// --- CMIS 1.1 Browser JSON Binding ---
	cmisHandler := cmis.New(gw, sessions, cfg)
	m.Mount("/cmis", cmisHandler.Router())
	log.Info().Msg("CMIS 1.1 browser binding enabled at /cmis")

	return &Service{mux: m}
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", ww.Status()).
			Str("duration", time.Since(start).String()).
			Str("remote", r.RemoteAddr).
			Msg("request")
	})
}
