package cmis

import (
	"testing"

	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
)

func TestTokenize(t *testing.T) {
	tests := []struct {
		sql    string
		expect []string
	}{
		{
			sql:    "SELECT * FROM cmis:document",
			expect: []string{"SELECT", "*", "FROM", "cmis:document"},
		},
		{
			sql:    "SELECT cmis:name, cmis:objectId FROM cmis:folder",
			expect: []string{"SELECT", "cmis:name", ",", "cmis:objectId", "FROM", "cmis:folder"},
		},
		{
			sql:    "SELECT * FROM cmis:document WHERE cmis:name = 'test.pdf'",
			expect: []string{"SELECT", "*", "FROM", "cmis:document", "WHERE", "cmis:name", "=", "'test.pdf'"},
		},
		{
			sql:    "SELECT * FROM cmis:document WHERE IN_FOLDER('abc123')",
			expect: []string{"SELECT", "*", "FROM", "cmis:document", "WHERE", "IN_FOLDER", "(", "'abc123'", ")"},
		},
		{
			sql:    "SELECT * FROM cmis:document WHERE cmis:name <> 'draft'",
			expect: []string{"SELECT", "*", "FROM", "cmis:document", "WHERE", "cmis:name", "<>", "'draft'"},
		},
	}

	for _, tt := range tests {
		tokens := tokenize(tt.sql)
		if len(tokens) != len(tt.expect) {
			t.Errorf("tokenize(%q) = %v (len %d), want len %d", tt.sql, tokens, len(tokens), len(tt.expect))
			continue
		}
		for i, tok := range tokens {
			if tok != tt.expect[i] {
				t.Errorf("tokenize(%q)[%d] = %q, want %q", tt.sql, i, tok, tt.expect[i])
			}
		}
	}
}

func TestParseQuery(t *testing.T) {
	tests := []struct {
		sql      string
		wantType string
		wantErr  bool
	}{
		{
			sql:      "SELECT * FROM cmis:document",
			wantType: "cmis:document",
		},
		{
			sql:      "SELECT * FROM cmis:folder WHERE IN_FOLDER('abc')",
			wantType: "cmis:folder",
		},
		{
			sql:      "SELECT * FROM cmis:document d JOIN oy:documentMetadata m ON d.cmis:objectId = m.cmis:objectId WHERE cmis:name = 'test'",
			wantType: "cmis:document",
		},
		{
			sql:      "SELECT * FROM cmis:document ORDER BY cmis:name ASC",
			wantType: "cmis:document",
		},
		{
			sql:      "SELECT * FROM cmis:document WHERE cmis:name IS NOT NULL ORDER BY cmis:lastModificationDate DESC",
			wantType: "cmis:document",
		},
		{
			sql:     "",
			wantErr: true,
		},
		{
			sql:     "INSERT INTO cmis:document",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		q, err := parseQuery(tt.sql)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseQuery(%q) expected error", tt.sql)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseQuery(%q) unexpected error: %v", tt.sql, err)
			continue
		}
		if q.fromType != tt.wantType {
			t.Errorf("parseQuery(%q).fromType = %q, want %q", tt.sql, q.fromType, tt.wantType)
		}
	}
}

func TestParseQueryPredicates(t *testing.T) {
	q, err := parseQuery("SELECT * FROM cmis:document WHERE cmis:name = 'test.pdf' AND cmis:contentStreamMimeType = 'application/pdf'")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.predicates) != 2 {
		t.Fatalf("expected 2 predicates, got %d", len(q.predicates))
	}
	if q.predicates[0].property != "cmis:name" || q.predicates[0].value != "test.pdf" {
		t.Errorf("pred[0] = %+v", q.predicates[0])
	}
	if q.predicates[1].property != "cmis:contentStreamMimeType" || q.predicates[1].value != "application/pdf" {
		t.Errorf("pred[1] = %+v", q.predicates[1])
	}
}

func TestParseQueryINFolder(t *testing.T) {
	q, err := parseQuery("SELECT * FROM cmis:document WHERE IN_FOLDER('folderid123')")
	if err != nil {
		t.Fatal(err)
	}
	if q.inFolder != "folderid123" {
		t.Errorf("inFolder = %q, want %q", q.inFolder, "folderid123")
	}
}

func TestParseQueryINTree(t *testing.T) {
	q, err := parseQuery("SELECT * FROM cmis:document WHERE IN_TREE('treeid456')")
	if err != nil {
		t.Fatal(err)
	}
	if q.inTree != "treeid456" {
		t.Errorf("inTree = %q, want %q", q.inTree, "treeid456")
	}
}

func TestParseQueryOrderBy(t *testing.T) {
	q, err := parseQuery("SELECT * FROM cmis:document ORDER BY cmis:name ASC, cmis:lastModificationDate DESC")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.orderBy) != 2 {
		t.Fatalf("expected 2 orderBy clauses, got %d", len(q.orderBy))
	}
	if q.orderBy[0].property != "cmis:name" || q.orderBy[0].descending {
		t.Errorf("orderBy[0] = %+v", q.orderBy[0])
	}
	if q.orderBy[1].property != "cmis:lastModificationDate" || !q.orderBy[1].descending {
		t.Errorf("orderBy[1] = %+v", q.orderBy[1])
	}
}

func TestParseQueryISNull(t *testing.T) {
	q, err := parseQuery("SELECT * FROM cmis:document WHERE oy.category IS NULL")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.predicates) != 1 {
		t.Fatalf("expected 1 predicate, got %d", len(q.predicates))
	}
	if q.predicates[0].operator != "IS NULL" {
		t.Errorf("operator = %q, want %q", q.predicates[0].operator, "IS NULL")
	}
}

func TestParseQueryISNotNull(t *testing.T) {
	q, err := parseQuery("SELECT * FROM cmis:document WHERE oy.subject IS NOT NULL")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.predicates) != 1 {
		t.Fatalf("expected 1 predicate, got %d", len(q.predicates))
	}
	if q.predicates[0].operator != "IS NOT NULL" {
		t.Errorf("operator = %q, want %q", q.predicates[0].operator, "IS NOT NULL")
	}
}

func TestParseQueryContains(t *testing.T) {
	q, err := parseQuery("SELECT * FROM cmis:document WHERE CONTAINS('invoice')")
	if err != nil {
		t.Fatal(err)
	}
	if q.contains != "invoice" {
		t.Errorf("contains = %q, want %q", q.contains, "invoice")
	}
}

func TestParseQueryIN(t *testing.T) {
	q, err := parseQuery("SELECT * FROM cmis:document WHERE cmis:contentStreamMimeType IN ('application/pdf', 'image/png')")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.predicates) != 1 {
		t.Fatalf("expected 1 predicate, got %d", len(q.predicates))
	}
	if q.predicates[0].operator != "IN" {
		t.Errorf("operator = %q, want %q", q.predicates[0].operator, "IN")
	}
	if len(q.predicates[0].values) != 2 {
		t.Errorf("IN values = %v, want 2 values", q.predicates[0].values)
	}
}

func TestParseQueryJoin(t *testing.T) {
	q, err := parseQuery("SELECT * FROM cmis:document d JOIN oy:documentMetadata m ON d.cmis:objectId = m.cmis:objectId WHERE cmis:name LIKE '%test%'")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.joins) != 1 {
		t.Fatalf("expected 1 join, got %d", len(q.joins))
	}
	if q.joins[0].typeName != "oy:documentMetadata" {
		t.Errorf("join type = %q, want %q", q.joins[0].typeName, "oy:documentMetadata")
	}
}

func TestMatchLike(t *testing.T) {
	tests := []struct {
		s, pattern string
		want       bool
	}{
		{"hello", "%", true},
		{"hello", "hello", true},
		{"hello", "hell_", true},
		{"hello", "h%o", true},
		{"hello", "%llo", true},
		{"hello", "world", false},
		{"hello", "h_llo", true},
		{"hello", "h__lo", true},
		{"hello", "h___o", true},  // h + 3 single chars + o = 5 chars matches "hello"
		{"hello", "h_____", false}, // h + 5 underscores = 6 chars, "hello" only 5
		{"", "%", true},
		{"", "_", false},
		{"abc", "a%c", true},
		{"abc", "%b%", true},
		{"invoice_2024.pdf", "%invoice%", true},
		{"report.docx", "%.pdf", false},
	}

	for _, tt := range tests {
		got := matchLike(tt.s, tt.pattern)
		if got != tt.want {
			t.Errorf("matchLike(%q, %q) = %v, want %v", tt.s, tt.pattern, got, tt.want)
		}
	}
}

func TestMatchPredicate(t *testing.T) {
	tests := []struct {
		actual string
		pred   predicate
		want   bool
	}{
		{"hello", predicate{operator: "=", value: "hello"}, true},
		{"hello", predicate{operator: "=", value: "world"}, false},
		{"hello", predicate{operator: "<>", value: "world"}, true},
		{"hello", predicate{operator: "LIKE", value: "%llo"}, true},
		{"", predicate{operator: "IS NULL"}, true},
		{"value", predicate{operator: "IS NULL"}, false},
		{"value", predicate{operator: "IS NOT NULL"}, true},
		{"", predicate{operator: "IS NOT NULL"}, false},
		{"pdf", predicate{operator: "IN", values: []string{"pdf", "docx", "xlsx"}}, true},
		{"txt", predicate{operator: "IN", values: []string{"pdf", "docx"}}, false},
	}

	for _, tt := range tests {
		got := matchPredicate(tt.actual, tt.pred)
		if got != tt.want {
			t.Errorf("matchPredicate(%q, %+v) = %v, want %v", tt.actual, tt.pred, got, tt.want)
		}
	}
}

func TestBuiltinTypes(t *testing.T) {
	types := []string{BaseTypeDocument, BaseTypeFolder, BaseTypeRelationship, BaseTypePolicy, BaseTypeItem}
	for _, id := range types {
		td := builtinType(id)
		if td == nil {
			t.Errorf("builtinType(%q) = nil", id)
			continue
		}
		if td.ID != id {
			t.Errorf("builtinType(%q).ID = %q", id, td.ID)
		}
	}
}

func TestLookupType(t *testing.T) {
	// Base types
	for _, id := range []string{BaseTypeDocument, BaseTypeFolder} {
		if LookupType(id) == nil {
			t.Errorf("LookupType(%q) = nil", id)
		}
	}
	// Secondary types
	for _, id := range []string{TypeOYDocument, TypeOYFolder, TypeOYIndex} {
		if LookupType(id) == nil {
			t.Errorf("LookupType(%q) = nil", id)
		}
	}
	// Unknown
	if LookupType("cmis:unknown") != nil {
		t.Error("LookupType(cmis:unknown) should be nil")
	}
}

func TestEncodeDecodeObjectID(t *testing.T) {
	tests := []struct {
		storage, space, opaque string
	}{
		{"storage1", "space1", "opaque1"},
		{"abc", "def", "ghi"},
		{"s", "", "o"},
	}

	for _, tt := range tests {
		rid := &provider.ResourceId{
			StorageId: tt.storage,
			SpaceId:   tt.space,
			OpaqueId:  tt.opaque,
		}
		encoded := encodeObjectID(rid)
		decoded, err := decodeObjectID(encoded)
		if err != nil {
			t.Errorf("decodeObjectID(%q) error: %v", encoded, err)
			continue
		}
		if decoded.StorageId != tt.storage || decoded.SpaceId != tt.space || decoded.OpaqueId != tt.opaque {
			t.Errorf("roundtrip failed: got %+v, want storage=%s space=%s opaque=%s",
				decoded, tt.storage, tt.space, tt.opaque)
		}
	}
}

func TestMapResourceToObject(t *testing.T) {
	info := &provider.ResourceInfo{
		Id:       &provider.ResourceId{StorageId: "s", SpaceId: "sp", OpaqueId: "op"},
		Type:     provider.ResourceType_RESOURCE_TYPE_FILE,
		Path:     "/docs/test.pdf",
		MimeType: "application/pdf",
		Size:     12345,
		Etag:     "etag123",
	}

	obj := mapResourceToObject(info)

	if obj.Properties["cmis:name"].Value != "test.pdf" {
		t.Errorf("name = %v, want test.pdf", obj.Properties["cmis:name"].Value)
	}
	if obj.Properties["cmis:baseTypeId"].Value != BaseTypeDocument {
		t.Errorf("baseTypeId = %v, want %s", obj.Properties["cmis:baseTypeId"].Value, BaseTypeDocument)
	}
	if obj.Properties["cmis:contentStreamMimeType"].Value != "application/pdf" {
		t.Errorf("mimeType = %v", obj.Properties["cmis:contentStreamMimeType"].Value)
	}
	if obj.Properties["cmis:contentStreamLength"].Value != uint64(12345) {
		t.Errorf("size = %v, want 12345", obj.Properties["cmis:contentStreamLength"].Value)
	}

	// Folder
	folderInfo := &provider.ResourceInfo{
		Id:   &provider.ResourceId{StorageId: "s", SpaceId: "sp", OpaqueId: "f1"},
		Type: provider.ResourceType_RESOURCE_TYPE_CONTAINER,
		Path: "/docs/subfolder",
	}
	folderObj := mapResourceToObject(folderInfo)
	if folderObj.Properties["cmis:baseTypeId"].Value != BaseTypeFolder {
		t.Errorf("folder baseTypeId = %v", folderObj.Properties["cmis:baseTypeId"].Value)
	}
	if folderObj.Properties["cmis:path"].Value != "/docs/subfolder" {
		t.Errorf("folder path = %v", folderObj.Properties["cmis:path"].Value)
	}
}
