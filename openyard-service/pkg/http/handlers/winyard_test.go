package handlers

// WinYard wire format tests. The flows follow the calls Saskia
// (SIS.Global.Dms DmsWinyard) and Archikart make; JSON names follow the
// WinYard client classes (DocInfo, DocIndex "topic"/"values", WDocInfo ...).

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kosmos-eu/openyard/pkg/auth"
	"github.com/kosmos-eu/openyard/pkg/metadata"
	"github.com/kosmos-eu/openyard/pkg/migration"
)

const saskiaMappingYAML = `
namespaces:
  saskia:
    prefix: ctrl
    storage_case: lower
    client_case: upper
    source: saskia
    defaults:
      booking.status: U
    formats:
      date: "02.01.2006"
      decimal: ","
users:
  saskia: saskia
`

// Client-side views of the WinYard classes (names as in the WinYard proxy).
type wyDocIndex struct {
	Topic  string `json:"topic"`
	Values struct {
		StringValue string `json:"StringValue"`
	} `json:"values"`
}

type wyDocInfo struct {
	DocId          string       `json:"DocId"`
	DocName        string       `json:"DocName"`
	FileExtension  string       `json:"FileExtension"`
	ParentFolderId string       `json:"ParentFolderId"`
	DocIndex       []wyDocIndex `json:"DocIndex"`
	FileKey        []byte       `json:"FileKey"`
	DocType        *struct {
		Name string `json:"Name"`
	} `json:"DocType"`
}

type wyDocument struct {
	DocInfo wyDocInfo `json:"DocInfo"`
	FileKey []byte    `json:"FileKey"`
	TaskLog struct {
		HasError bool `json:"hasError"`
	} `json:"TaskLog"`
}

func (d wyDocInfo) index() map[string]string {
	values := map[string]string{}
	for _, entry := range d.DocIndex {
		values[entry.Topic] = entry.Values.StringValue
	}
	return values
}

// winyardEnv is a Saskia-like setup: project space "Finanzen" with folder
// "Belege", one booked invoice and one Taki-enriched invoice.
type winyardEnv struct {
	t         *testing.T
	storage   *fakeStorage
	handlers  *Handlers
	router    http.Handler
	sessionID string
	space     *fakeNode
	belege    *fakeNode
	booked    *fakeNode
	enriched  *fakeNode
}

func newWinyardEnv(t *testing.T) *winyardEnv {
	t.Helper()
	os.Setenv("OPENYARD_MIGRATION_DB", filepath.Join(t.TempDir(), "migration.db"))
	migration.Init()
	guidResourceIDs.Flush()

	storage := newFakeStorage()
	storage.addSpace("Personal Admin")
	space := storage.addSpace("Finanzen")
	belege := storage.addNode(space, "Belege", true, nil, nil)
	booked := storage.addNode(belege, "RE-2026-0042.pdf", false, []byte("%PDF booked"), map[string]string{
		"source":        "saskia",
		"doc.type":      "rechnung",
		"ctrl.beleg_nr": "2026-0042",
		"amounts.total": "119.00",
	})
	enriched := storage.addNode(belege, "Scan Posteingang.pdf", false, []byte("%PDF scan"), map[string]string{
		"doc.type":        "rechnung",
		"sender.company":  "Stadtwerke",
		"doc.meta_source": "taki",
	})
	storage.addNode(belege, "Brief.pdf", false, []byte("%PDF letter"), map[string]string{"doc.type": "brief"})

	h := newFakeHandlers(t, storage, belege)
	configPath := filepath.Join(t.TempDir(), "mapping.yaml")
	if err := os.WriteFile(configPath, []byte(saskiaMappingYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	h.metaCfg = metadata.LoadConfig(configPath)
	sessionID := h.sessions.Create(&auth.SessionContext{Login: "saskia", CS3Token: "cs3-token"})

	router := chi.NewRouter()
	router.Post("/api/advancedUsers/Login", h.Login)
	router.Post("/api/advancedDocuments/GetFile", h.GetFile)
	router.Group(func(r chi.Router) {
		r.Use(h.SessionMiddleware)
		r.Post("/api/advancedDocuments/GetDocument", h.GetDocument)
		r.Post("/api/advancedDocuments/SetDocument", h.SetDocument)
		r.Get("/api/advancedDocuments/GetPreviewFile", h.GetPreviewFile)
		r.Post("/api/advancedFolders/GetFolder", h.GetFolder)
		r.Post("/api/basicDocuments/ImportDocumentDynamic", h.ImportDocumentDynamic)
		r.Put("/api/basicDocuments/SetDocIndex", h.SetDocIndex)
		r.Post("/api/basicDocuments/SearchForDocIndex", h.SearchForDocIndex)
		r.Post("/api/basicDocuments/UploadFile", h.UploadFile)
		r.Put("/api/basicDocuments/UpdateDocument", h.UpdateDocument)
		r.Post("/api/advancedCommon/SetTopic", h.SetTopic)
		r.Delete("/api/advancedCommon/DeleteTopic", h.DeleteTopic)
	})
	return &winyardEnv{t: t, storage: storage, handlers: h, router: router, sessionID: sessionID,
		space: space, belege: belege, booked: booked, enriched: enriched}
}

// call sends a request; query is appended to ?SessionID=<session>& unless
// it starts with "!" (no session).
func (e *winyardEnv) call(method, route, query string, contentType string, body []byte) *httptest.ResponseRecorder {
	e.t.Helper()
	url := route + "?SessionID=" + e.sessionID + "&" + query
	if strings.HasPrefix(query, "!") {
		url = route + "?" + strings.TrimPrefix(query, "!")
	}
	req := httptest.NewRequest(method, url, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	e.router.ServeHTTP(recorder, req)
	return recorder
}

func (e *winyardEnv) callJSON(method, route, query string, body interface{}) *httptest.ResponseRecorder {
	e.t.Helper()
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			e.t.Fatal(err)
		}
	}
	return e.call(method, route, query, "application/json", payload)
}

func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder, target interface{}) {
	t.Helper()
	if recorder.Code != 200 {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decode %s: %v", recorder.Body.String(), err)
	}
}

func (e *winyardEnv) getDocument(docID string) wyDocument {
	e.t.Helper()
	var document wyDocument
	decodeBody(e.t, e.call("POST", "/api/advancedDocuments/GetDocument",
		"DocID="+docID+"&LoadFileKey=True&LoadDocIndex=True&CheckOut=False", "", nil), &document)
	return document
}

func (e *winyardEnv) uploadFile(fileName string, content []byte) []byte {
	e.t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", fileName)
	part.Write(content)
	writer.Close()
	recorder := e.call("POST", "/api/basicDocuments/UploadFile",
		"Extension="+filepath.Ext(fileName)+"&CreationDate=2026-11-06T10:00:00&lastModifiedDate=2026-11-06T10:00:00",
		writer.FormDataContentType(), body.Bytes())
	var fileKey []byte
	decodeBody(e.t, recorder, &fileKey)
	if len(fileKey) == 0 {
		e.t.Fatal("UploadFile returned empty FileKey")
	}
	return fileKey
}

func TestExpiresInTicks(t *testing.T) {
	cases := []struct {
		ttl, expiresIn time.Duration
		want           int64
	}{
		{8 * time.Hour, 10 * time.Minute, 10 * 60 * 10_000_000},
		{8 * time.Hour, 0, (8*60 - 1) * 60 * 10_000_000},
		{5 * time.Minute, 10 * time.Minute, 4 * 60 * 10_000_000},
		{time.Minute, 10 * time.Minute, 0},
	}
	for _, testCase := range cases {
		if got := expiresInTicks(testCase.ttl, testCase.expiresIn); got != testCase.want {
			t.Errorf("expiresInTicks(%v, %v) = %d, want %d", testCase.ttl, testCase.expiresIn, got, testCase.want)
		}
	}
}

func TestWinyardLoginExpiresInTicks(t *testing.T) {
	env := newWinyardEnv(t)
	env.handlers.gw = nil // dummy login mode, no CS3 authentication
	var user struct {
		SessionId string `json:"SessionId"`
		ExpiresIn int64  `json:"ExpiresIn"`
	}
	decodeBody(t, env.call("POST", "/api/advancedUsers/Login", "!", "application/json",
		[]byte(`{"username":"","Password":""}`)), &user)
	if !isGUID(user.SessionId) {
		t.Errorf("SessionId = %q, want GUID", user.SessionId)
	}
	// default OPENYARD_SESSION_EXPIRES_IN 10 min (below TTL 1h - 1 min), in 100 ns ticks
	if want := int64(10 * 60 * 10_000_000); user.ExpiresIn != want {
		t.Errorf("ExpiresIn = %d, want %d ticks", user.ExpiresIn, want)
	}
}

// Saskia ReadAsync: GetDocument(LoadFileKey, LoadDocIndex) → GetFile(FileKey).
func TestWinyardReadDocument(t *testing.T) {
	env := newWinyardEnv(t)
	document := env.getDocument(env.booked.id)

	docInfo := document.DocInfo
	if docInfo.DocId != env.booked.id {
		t.Errorf("DocId = %q, want opaque ID %q", docInfo.DocId, env.booked.id)
	}
	if docInfo.DocName+"."+docInfo.FileExtension != "RE-2026-0042.pdf" {
		t.Errorf("file name = %q.%q", docInfo.DocName, docInfo.FileExtension)
	}
	if docInfo.ParentFolderId != env.belege.id {
		t.Errorf("ParentFolderId = %q, want %q", docInfo.ParentFolderId, env.belege.id)
	}
	if docInfo.DocType == nil || docInfo.DocType.Name != "rechnung" {
		t.Errorf("DocType = %+v", docInfo.DocType)
	}
	index := docInfo.index()
	if index["BELEG_NR"] != "2026-0042" || index["amounts.total"] != "119,00" || index["source"] != "saskia" {
		t.Errorf("DocIndex = %v", index)
	}
	if len(document.FileKey) == 0 {
		t.Fatal("FileKey missing")
	}

	// GetFile: FileKey as JSON base64 string, no SessionID
	fileKeyJSON, _ := json.Marshal(document.FileKey)
	recorder := env.call("POST", "/api/advancedDocuments/GetFile", "!", "application/json", fileKeyJSON)
	if recorder.Code != 200 || recorder.Body.String() != "%PDF booked" {
		t.Fatalf("GetFile = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestWinyardGetFileUnknownKeyIsNoContent(t *testing.T) {
	env := newWinyardEnv(t)
	fileKeyJSON, _ := json.Marshal([]byte("garbage"))
	if recorder := env.call("POST", "/api/advancedDocuments/GetFile", "!", "application/json", fileKeyJSON); recorder.Code != 204 {
		t.Errorf("GetFile(unknown) = %d, want 204", recorder.Code)
	}
	// valid key format, expired session
	expiredKey, _ := json.Marshal(downloadFileKey("11111111-1111-4111-8111-111111111111", env.storage.resourceID(env.booked)))
	if recorder := env.call("POST", "/api/advancedDocuments/GetFile", "!", "application/json", expiredKey); recorder.Code != 401 {
		t.Errorf("GetFile(expired session) = %d, want 401", recorder.Code)
	}
}

// Saskia CreateSubDocAsync expects "existiert nicht" for a missing parent.
func TestWinyardGetDocumentNotFound(t *testing.T) {
	env := newWinyardEnv(t)
	missingID := "99999999-9999-4999-8999-999999999999"
	recorder := env.call("POST", "/api/advancedDocuments/GetDocument", "DocID="+missingID, "", nil)
	if recorder.Code != 404 || !strings.Contains(recorder.Body.String(), missingID) || !strings.Contains(recorder.Body.String(), "existiert nicht") {
		t.Errorf("GetDocument(missing) = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestWinyardGuidResolutionIsCached(t *testing.T) {
	env := newWinyardEnv(t)
	env.getDocument(env.booked.id)
	statsAfterFirst := env.storage.statCalls
	env.getDocument(env.booked.id)
	// second call: one Stat for the document, no space probing
	if probes := env.storage.statCalls - statsAfterFirst; probes != 1 {
		t.Errorf("second lookup used %d Stat calls, want 1", probes)
	}
}

// Moving a document within its space keeps the opaque ID (and the GUID).
func TestWinyardIDSurvivesMoveWithinSpace(t *testing.T) {
	env := newWinyardEnv(t)
	archive := env.storage.addNode(env.space, "Archiv", true, nil, nil)
	env.getDocument(env.booked.id)
	env.storage.mu.Lock()
	env.booked.parentID = archive.id
	env.storage.mu.Unlock()
	if parent := env.getDocument(env.booked.id).DocInfo.ParentFolderId; parent != archive.id {
		t.Errorf("ParentFolderId after move = %q, want %q", parent, archive.id)
	}
}

// Migrated WinYard documents keep their legacy GUID.
func TestWinyardLegacyGUID(t *testing.T) {
	env := newWinyardEnv(t)
	legacyID := "a1b2c3d4-0000-4000-8000-00000000abcd"
	migration.MapID(legacyID, encodeObjectID(env.storage.resourceID(env.booked)), "document", "RE-2026-0042.pdf")
	if docID := env.getDocument(legacyID).DocInfo.DocId; docID != legacyID {
		t.Errorf("DocId = %q, want legacy GUID %q", docID, legacyID)
	}
}

// Saskia CreateAsync: ImportDocumentDynamic returns a GUID; doc type and
// FELD_DMS topics end up as shared / namespaced metadata.
func TestWinyardImportDocumentDynamic(t *testing.T) {
	env := newWinyardEnv(t)
	xmlBody := `<ImportDocDataDynamic><DocIndex>` +
		`<KeyValueOfstringstring><Key>BELEG_NR</Key><Value>2026-0077</Value></KeyValueOfstringstring>` +
		`<KeyValueOfstringstring><Key>doc.reference</Key><Value>R-77</Value></KeyValueOfstringstring>` +
		`</DocIndex><FileContent>JVBERiBuZXc=</FileContent></ImportDocDataDynamic>`
	recorder := env.call("POST", "/api/basicDocuments/ImportDocumentDynamic",
		"ConfigName=rechnung&FileName=RE-77.pdf", "application/xml", []byte(xmlBody))
	var newID string
	decodeBody(t, recorder, &newID)
	if !isGUID(newID) {
		t.Fatalf("ImportDocumentDynamic = %q, want GUID", newID)
	}
	node := env.storage.nodes[newID]
	if node == nil {
		t.Fatalf("no node %s", newID)
	}
	want := map[string]string{"ctrl.beleg_nr": "2026-0077", "doc.reference": "R-77", "source": "saskia",
		"doc.type": "rechnung"}
	for key, value := range want {
		if node.metadata[key] != value {
			t.Errorf("metadata[%s] = %q, want %q", key, node.metadata[key], value)
		}
	}
	if env.getDocument(newID).DocInfo.index()["BELEG_NR"] != "2026-0077" {
		t.Error("BELEG_NR not readable via GetDocument")
	}
}

// Saskia GetListAsync: SearchForDocIndex(SearchInFolder=Guid.Empty, DocType, Conjunction=true).
func TestWinyardSearchForDocIndex(t *testing.T) {
	env := newWinyardEnv(t)
	search := func(query string, conditions []map[string]string) []wyDocInfo {
		t.Helper()
		var results []wyDocInfo
		decodeBody(t, env.callJSON("POST", "/api/basicDocuments/SearchForDocIndex",
			"SearchInFolder="+nilGUID+"&DocType=rechnung&Conjunction=True&"+query, conditions), &results)
		return results
	}

	results := search("", []map[string]string{{"Topic": "BELEG_NR", "Compare": "Equal", "Value": "2026-0042"}})
	if len(results) != 1 || results[0].DocId != env.booked.id || results[0].index()["BELEG_NR"] != "2026-0042" {
		t.Fatalf("search BELEG_NR = %+v", results)
	}

	// no condition: booked invoice plus Taki-enriched invoice (doc.type rechnung), not the letter
	results = search("", nil)
	found := map[string]bool{}
	for _, result := range results {
		found[result.DocId] = true
	}
	if len(results) != 2 || !found[env.booked.id] || !found[env.enriched.id] {
		t.Errorf("search by doc type = %+v", results)
	}

	if results = search("MaxResults=1", nil); len(results) != 1 {
		t.Errorf("MaxResults=1 returned %d", len(results))
	}

	// no match → empty array, never null
	recorder := env.callJSON("POST", "/api/basicDocuments/SearchForDocIndex",
		"SearchInFolder="+nilGUID+"&DocType=rechnung",
		[]map[string]string{{"Topic": "BELEG_NR", "Compare": "Equal", "Value": "nope"}})
	if strings.TrimSpace(recorder.Body.String()) != "[]" {
		t.Errorf("empty search body = %s", recorder.Body.String())
	}
}

// Saskia GetListUnbearbeitetAsync adds STATUSDOK = U (FELD_DMS booking.status).
// A missing status counts as U (namespace default), a set status wins.
func TestWinyardSearchUnprocessedDefault(t *testing.T) {
	env := newWinyardEnv(t)
	env.storage.mu.Lock()
	env.enriched.metadata["booking.status"] = "E"
	env.storage.mu.Unlock()

	var results []wyDocInfo
	decodeBody(t, env.callJSON("POST", "/api/basicDocuments/SearchForDocIndex",
		"SearchInFolder="+nilGUID+"&DocType=rechnung&Conjunction=True",
		[]map[string]string{{"Topic": "booking.status", "Compare": "Equal", "Value": "U"}}), &results)
	if len(results) != 1 || results[0].DocId != env.booked.id {
		t.Fatalf("unprocessed = %+v, want only the booked invoice", results)
	}
	if status := results[0].index()["booking.status"]; status != "U" {
		t.Errorf("DocIndex booking.status = %q, want default U", status)
	}
	if _, stored := env.booked.metadata["booking.status"]; stored {
		t.Error("default must not be written to the document")
	}
}

// Saskia reads and writes German dates and decimal commas; storage stays
// canonical (ISO date, decimal point) for Taki and the search index.
func TestWinyardGermanFormats(t *testing.T) {
	env := newWinyardEnv(t)
	env.storage.mu.Lock()
	env.booked.metadata["doc.date"] = "2026-09-19"
	env.storage.mu.Unlock()

	index := env.getDocument(env.booked.id).DocInfo.index()
	if index["doc.date"] != "19.09.2026" || index["amounts.total"] != "119,00" {
		t.Errorf("client values = date %q total %q", index["doc.date"], index["amounts.total"])
	}

	var updated bool
	decodeBody(t, env.callJSON("PUT", "/api/basicDocuments/SetDocIndex", "DocId="+env.booked.id,
		map[string]string{"doc.date": "01.10.2026", "amounts.total": "1.234,50"}), &updated)
	if env.booked.metadata["doc.date"] != "2026-10-01" || env.booked.metadata["amounts.total"] != "1234.50" {
		t.Errorf("stored values = date %q total %q", env.booked.metadata["doc.date"], env.booked.metadata["amounts.total"])
	}

	var results []wyDocInfo
	decodeBody(t, env.callJSON("POST", "/api/basicDocuments/SearchForDocIndex",
		"SearchInFolder="+nilGUID+"&DocType=rechnung",
		[]map[string]string{{"Topic": "amounts.total", "Compare": "Equal", "Value": "1.234,50"}}), &results)
	if len(results) != 1 || results[0].DocId != env.booked.id {
		t.Errorf("search by German amount = %+v", results)
	}
}

// Saskia CreateSubDocAsync: GetDocument(parent doc) → UploadFile → SetDocument(WDocInfo).
func TestWinyardCreateSubDocument(t *testing.T) {
	env := newWinyardEnv(t)
	mainDoc := env.getDocument(env.booked.id)

	createSubDoc := func() wyDocInfo {
		t.Helper()
		fileKey := env.uploadFile("Anlage.pdf", []byte("%PDF attachment"))
		wDocInfo := map[string]interface{}{
			"DocName":        "Anlage",
			"DocType":        "rechnung",
			"ParentFolderId": mainDoc.DocInfo.ParentFolderId,
			"FileKey":        fileKey,
			"DocIndex":       []map[string]string{{"topic": "BELEG_NR", "value": "2026-0042"}},
		}
		var document wyDocument
		decodeBody(t, env.callJSON("POST", "/api/advancedDocuments/SetDocument", "", wDocInfo), &document)
		return document.DocInfo
	}

	first := createSubDoc()
	if !isGUID(first.DocId) || first.ParentFolderId != env.belege.id {
		t.Fatalf("SetDocument DocInfo = %+v", first)
	}
	if first.DocName != "Anlage" || first.FileExtension != "pdf" || first.index()["BELEG_NR"] != "2026-0042" {
		t.Errorf("SetDocument DocInfo = %+v", first)
	}
	if node := env.storage.nodes[first.DocId]; string(node.content) != "%PDF attachment" || node.metadata["doc.type"] != "rechnung" {
		t.Errorf("stored node = %+v", node)
	}

	// same name again: new document, never overwrite
	second := createSubDoc()
	if second.DocId == first.DocId || second.DocName != "Anlage (2)" {
		t.Errorf("second SetDocument = %+v", second)
	}
}

func TestWinyardSetDocumentUnknownFileKey(t *testing.T) {
	env := newWinyardEnv(t)
	wDocInfo := map[string]interface{}{"DocName": "x", "ParentFolderId": env.belege.id, "FileKey": []byte("oyup1|other|upload-1")}
	if recorder := env.callJSON("POST", "/api/advancedDocuments/SetDocument", "", wDocInfo); recorder.Code != 400 {
		t.Errorf("SetDocument(foreign FileKey) = %d, want 400", recorder.Code)
	}
}

// Saskia UpdateAsync with file: UploadFile → UpdateDocument → SetDocIndex.
func TestWinyardUpdateDocumentAndDocIndex(t *testing.T) {
	env := newWinyardEnv(t)
	fileKey := env.uploadFile("neu.pdf", []byte("%PDF v2"))
	var updated bool
	decodeBody(t, env.callJSON("PUT", "/api/basicDocuments/UpdateDocument", "DocId="+env.booked.id, fileKey), &updated)
	if !updated || string(env.booked.content) != "%PDF v2" {
		t.Fatalf("UpdateDocument = %v, content %q", updated, env.booked.content)
	}
	// FileKey is consumed
	if recorder := env.callJSON("PUT", "/api/basicDocuments/UpdateDocument", "DocId="+env.booked.id, fileKey); recorder.Code != 400 {
		t.Errorf("reused FileKey = %d, want 400", recorder.Code)
	}

	decodeBody(t, env.callJSON("PUT", "/api/basicDocuments/SetDocIndex", "DocId="+env.booked.id+"&DocType=rechnung",
		map[string]string{"BELEG_NR": "2026-0042-K", "amounts.net": "100.00"}), &updated)
	if env.booked.metadata["ctrl.beleg_nr"] != "2026-0042-K" || env.booked.metadata["amounts.net"] != "100.00" {
		t.Errorf("metadata after SetDocIndex = %v", env.booked.metadata)
	}
}

func TestWinyardPreviewWithoutRendering(t *testing.T) {
	env := newWinyardEnv(t)
	var preview struct {
		DocInfo wyDocInfo `json:"DocInfo"`
		Preview []byte    `json:"preview"`
	}
	decodeBody(t, env.call("GET", "/api/advancedDocuments/GetPreviewFile", "Doc_Id="+env.booked.id, "", nil), &preview)
	if preview.DocInfo.DocId != env.booked.id || len(preview.Preview) != 0 {
		t.Errorf("preview = %+v", preview)
	}
}

func TestWinyardTopics(t *testing.T) {
	env := newWinyardEnv(t)
	var result struct {
		ReturnValue bool `json:"ReturnValue"`
	}
	decodeBody(t, env.callJSON("POST", "/api/advancedCommon/SetTopic", "",
		map[string]interface{}{"index_id": nilGUID, "index_topic": "BELEG_NR", "data_type": 0}), &result)
	if !result.ReturnValue {
		t.Error("SetTopic ReturnValue false")
	}
	result.ReturnValue = false
	decodeBody(t, env.call("DELETE", "/api/advancedCommon/DeleteTopic", "TopicId="+nilGUID, "", nil), &result)
	if !result.ReturnValue {
		t.Error("DeleteTopic ReturnValue false")
	}
}

// Archikart / migration: GetFolder returns GUID folder IDs and WinYard SubDocs.
func TestWinyardGetFolder(t *testing.T) {
	env := newWinyardEnv(t)
	var folder struct {
		FolderInfo struct {
			FolderID string `json:"FolderID"`
		} `json:"FolderInfo"`
		SubDocs []wyDocInfo `json:"SubDocs"`
	}
	decodeBody(t, env.call("POST", "/api/advancedFolders/GetFolder", "FolderID="+env.belege.id, "", nil), &folder)
	if folder.FolderInfo.FolderID != env.belege.id || len(folder.SubDocs) != 3 {
		t.Fatalf("GetFolder = %+v", folder)
	}
	for _, doc := range folder.SubDocs {
		if !isGUID(doc.DocId) || len(doc.FileKey) == 0 || doc.ParentFolderId != env.belege.id {
			t.Errorf("SubDoc = %+v", doc)
		}
	}
}
