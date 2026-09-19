package handlers

import (
	"testing"

	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	typesv1 "github.com/cs3org/go-cs3apis/cs3/types/v1beta1"
	"github.com/kosmos-eu/openyard/pkg/metadata"
)

func TestEncodeDecodeObjectID(t *testing.T) {
	rid := &provider.ResourceId{
		StorageId: "storage-abc",
		OpaqueId:  "node-xyz-123",
	}

	encoded := encodeObjectID(rid)
	if encoded == "" {
		t.Fatal("encodeObjectID returned empty")
	}

	decoded, err := decodeObjectID(encoded)
	if err != nil {
		t.Fatalf("decodeObjectID: %v", err)
	}
	if decoded.StorageId != rid.StorageId {
		t.Errorf("StorageId = %q, want %q", decoded.StorageId, rid.StorageId)
	}
	if decoded.OpaqueId != rid.OpaqueId {
		t.Errorf("OpaqueId = %q, want %q", decoded.OpaqueId, rid.OpaqueId)
	}
}

func TestEncodeObjectIDNil(t *testing.T) {
	if got := encodeObjectID(nil); got != "" {
		t.Errorf("encodeObjectID(nil) = %q, want empty", got)
	}
}

func TestDecodeObjectIDInvalid(t *testing.T) {
	cases := []string{
		"",
		"not-base64-!!!",
		// valid base64 but no '!' separator
		"YWJj", // "abc"
	}
	for _, c := range cases {
		_, err := decodeObjectID(c)
		if err == nil {
			t.Errorf("decodeObjectID(%q) should fail", c)
		}
	}
}

func TestMapResourceInfoFile(t *testing.T) {
	info := &provider.ResourceInfo{
		Id:       &provider.ResourceId{StorageId: "s1", OpaqueId: "n1"},
		Path:     "/docs/contract.pdf",
		Type:     provider.ResourceType_RESOURCE_TYPE_FILE,
		Size:     102400,
		MimeType: "application/pdf",
		Etag:     "abc123",
		Mtime:    &typesv1.Timestamp{Seconds: 1700000000},
	}

	h := &Handlers{metaCfg: metadata.DefaultConfig()}
	m := h.mapResourceInfo(info)

	if m["type"] != "file" {
		t.Errorf("type = %v, want file", m["type"])
	}
	if m["name"] != "contract.pdf" {
		t.Errorf("name = %v, want contract.pdf", m["name"])
	}
	if m["path"] != "/docs/contract.pdf" {
		t.Errorf("path = %v", m["path"])
	}
	if m["size"] != uint64(102400) {
		t.Errorf("size = %v", m["size"])
	}
	if m["mimeType"] != "application/pdf" {
		t.Errorf("mimeType = %v", m["mimeType"])
	}
	if m["etag"] != "abc123" {
		t.Errorf("etag = %v", m["etag"])
	}
	if m["id"] == "" {
		t.Error("id is empty")
	}
}

func TestMapResourceInfoFolder(t *testing.T) {
	info := &provider.ResourceInfo{
		Id:   &provider.ResourceId{StorageId: "s1", OpaqueId: "n2"},
		Path: "/docs/contracts",
		Type: provider.ResourceType_RESOURCE_TYPE_CONTAINER,
		Etag: "def456",
	}

	h := &Handlers{metaCfg: metadata.DefaultConfig()}
	m := h.mapResourceInfo(info)

	if m["type"] != "folder" {
		t.Errorf("type = %v, want folder", m["type"])
	}
	if m["name"] != "contracts" {
		t.Errorf("name = %v, want contracts", m["name"])
	}
	if _, ok := m["size"]; ok {
		t.Error("folder should not have size")
	}
}

func TestMapResourceInfoWithMetadata(t *testing.T) {
	info := &provider.ResourceInfo{
		Id:   &provider.ResourceId{StorageId: "s1", OpaqueId: "n3"},
		Path: "/akte",
		Type: provider.ResourceType_RESOURCE_TYPE_CONTAINER,
		ArbitraryMetadata: &provider.ArbitraryMetadata{
			Metadata: map[string]string{
				"oy.aktenzeichen":  "11.12.02.01-15",
				"oy.object_type": "Personalakte",
			},
		},
	}

	h := &Handlers{metaCfg: metadata.DefaultConfig()}
	m := h.mapResourceInfo(info)

	if m["oy.aktenzeichen"] != "11.12.02.01-15" {
		t.Errorf("aktenzeichen = %v", m["oy.aktenzeichen"])
	}
	if m["oy.object_type"] != "Personalakte" {
		t.Errorf("object_type = %v", m["oy.object_type"])
	}
}

func TestFormatTimestamp(t *testing.T) {
	ts := &typesv1.Timestamp{Seconds: 1700000000, Nanos: 0}
	got := formatTimestamp(ts)
	want := "2023-11-14T22:13:20Z"
	if got != want {
		t.Errorf("formatTimestamp = %q, want %q", got, want)
	}
}

func TestFormatTimestampNil(t *testing.T) {
	if got := formatTimestamp(nil); got != "" {
		t.Errorf("formatTimestamp(nil) = %q, want empty", got)
	}
}

func TestRefFromObjectID(t *testing.T) {
	rid := &provider.ResourceId{StorageId: "s1", OpaqueId: "n1"}
	encoded := encodeObjectID(rid)

	ref, err := refFromObjectID(encoded)
	if err != nil {
		t.Fatalf("refFromObjectID: %v", err)
	}
	if ref.ResourceId.StorageId != "s1" || ref.ResourceId.OpaqueId != "n1" {
		t.Errorf("ref = %+v", ref.ResourceId)
	}
}

func TestRefFromPath(t *testing.T) {
	ref := refFromPath("/documents/test")
	if ref.Path != "/documents/test" {
		t.Errorf("path = %q", ref.Path)
	}
}
