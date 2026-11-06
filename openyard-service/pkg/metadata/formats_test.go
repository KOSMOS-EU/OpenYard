package metadata

import (
	"os"
	"path/filepath"
	"testing"
)

func germanFormatsConfig(t *testing.T) *Config {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "mapping.yaml")
	yamlText := "namespaces:\n  saskia:\n    prefix: ctrl\n    formats:\n      date: \"02.01.2006\"\n      decimal: \",\"\nusers:\n  saskia: saskia\n"
	if err := os.WriteFile(configPath, []byte(yamlText), 0o600); err != nil {
		t.Fatal(err)
	}
	return LoadConfig(configPath)
}

func TestToClientValue(t *testing.T) {
	cfg := germanFormatsConfig(t)
	cases := []struct{ key, stored, want string }{
		{"doc.date", "2026-09-19", "19.09.2026"},
		{"amounts.total", "535.50", "535,50"},
		{"amounts.total", "-12.5", "-12,5"},
		{"amounts.total", "119", "119"},
		{"doc.version", "1.5", "1.5"},
		{"doc.title", "2026-09-19 Stadtwerke Rechnung", "2026-09-19 Stadtwerke Rechnung"},
		{"doc.reference", "SI-2026-0042", "SI-2026-0042"},
	}
	for _, testCase := range cases {
		if got := cfg.ToClientValue("saskia", testCase.key, testCase.stored); got != testCase.want {
			t.Errorf("ToClientValue(%s, %q) = %q, want %q", testCase.key, testCase.stored, got, testCase.want)
		}
	}
	if got := cfg.ToClientValue("worker", "doc.date", "2026-09-19"); got != "2026-09-19" {
		t.Errorf("other namespace converted: %q", got)
	}
}

func TestToStorageValue(t *testing.T) {
	cfg := germanFormatsConfig(t)
	cases := []struct{ key, client, want string }{
		{"doc.date", "19.09.2026", "2026-09-19"},
		{"doc.date", "2026-09-19", "2026-09-19"},
		{"amounts.total", "535,50", "535.50"},
		{"amounts.total", "1.234,50", "1234.50"},
		{"amounts.total", "535.50", "535.50"},
		{"booking.er_nr", "12,5", "12,5"},
		{"doc.reference", "31.12.2026-A", "31.12.2026-A"},
	}
	for _, testCase := range cases {
		if got := cfg.ToStorageValue("saskia", testCase.key, testCase.client); got != testCase.want {
			t.Errorf("ToStorageValue(%s, %q) = %q, want %q", testCase.key, testCase.client, got, testCase.want)
		}
	}
}
