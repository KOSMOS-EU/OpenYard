package metadata

import (
	"os"
	"path/filepath"
	"testing"
)

func saskiaConfig(t *testing.T) *Config {
	t.Helper()
	yamlText := `
namespaces:
  saskia:
    prefix: ctrl
    storage_case: lower
    client_case: upper
    source: saskia
users:
  saskia: saskia
`
	configPath := filepath.Join(t.TempDir(), "mapping.yaml")
	if err := os.WriteFile(configPath, []byte(yamlText), 0o600); err != nil {
		t.Fatal(err)
	}
	return LoadConfig(configPath)
}

func TestToStorageKeySharedPrefixes(t *testing.T) {
	cfg := saskiaConfig(t)
	cases := map[string]string{
		"doc.reference":       "doc.reference",
		"Sender.Company":      "sender.company",
		"amounts.total":       "amounts.total",
		"booking.fiscal_year": "booking.fiscal_year",
		"source":              "source",
		"sourcefile":          "ctrl.sourcefile",
		"documents.x":         "ctrl.documents.x",
		"BELEG_NR":            "ctrl.beleg_nr",
		"Betreff":             "oy.subject",
	}
	for clientKey, want := range cases {
		if got := cfg.ToStorageKey("saskia", clientKey); got != want {
			t.Errorf("ToStorageKey(%q) = %q, want %q", clientKey, got, want)
		}
	}
	if got := cfg.ToClientKey("doc.reference"); got != "doc.reference" {
		t.Errorf("ToClientKey(doc.reference) = %q", got)
	}
}

func TestWithDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "mapping.yaml")
	yamlText := "namespaces:\n  saskia:\n    prefix: ctrl\n    defaults:\n      booking.status: U\nusers:\n  saskia: saskia\n"
	if err := os.WriteFile(configPath, []byte(yamlText), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := LoadConfig(configPath)

	stored := map[string]string{"doc.type": "rechnung"}
	merged := cfg.WithDefaults("saskia", stored)
	if merged["booking.status"] != "U" || merged["doc.type"] != "rechnung" {
		t.Errorf("WithDefaults = %v", merged)
	}
	if _, changed := stored["booking.status"]; changed {
		t.Error("input map modified")
	}
	if cfg.WithDefaults("saskia", map[string]string{"booking.status": "E"})["booking.status"] != "E" {
		t.Error("stored status must win over the default")
	}
	if _, ok := cfg.WithDefaults("worker", stored)["booking.status"]; ok {
		t.Error("other namespaces get no default")
	}
}

func TestDocumentMetadata(t *testing.T) {
	cfg := saskiaConfig(t)

	// Saskia sends the configured target value (KS_DMS_META DOKTYP FELD_DMS)
	documentMeta := cfg.DocumentMetadata("saskia", "rechnung")
	want := map[string]string{
		"source":   "saskia",
		"doc.type": "rechnung",
	}
	if len(documentMeta) != len(want) {
		t.Fatalf("DocumentMetadata = %v, want %v", documentMeta, want)
	}
	for key, value := range want {
		if documentMeta[key] != value {
			t.Errorf("%s = %q, want %q", key, documentMeta[key], value)
		}
	}

	// Without doc type only the source
	if withoutType := cfg.DocumentMetadata("saskia", ""); len(withoutType) != 1 {
		t.Errorf("DocumentMetadata without type = %v", withoutType)
	}

	// Default namespace: no source, doc type unchanged
	other := cfg.DocumentMetadata("worker", "brief")
	if _, ok := other["source"]; ok || other["doc.type"] != "brief" {
		t.Errorf("default namespace metadata = %v", other)
	}
}
