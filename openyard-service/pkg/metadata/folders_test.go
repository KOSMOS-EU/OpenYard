package metadata

import (
	"testing"
	"time"
)

func folderConfig(t *testing.T) *Config {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Users = map[string]string{"saskia": "saskia"}
	cfg.Namespaces["saskia"] = NamespaceConfig{
		Prefix: "ctrl", StorageCase: "lower", ClientCase: "upper",
		Formats: FormatConfig{Date: "02.01.2006", Decimal: ","},
		Folders: map[string]string{
			"IfrAnordnung":      "Anordnungen/{HH_JAHR}/{KZ_DKS}/{AO_NR|block:1000}",
			"IfrAnordnungListe": "Anordnungen/{HH_JAHR}/Listen",
			"IfrVeran":          "Veranlagung/{HH_JAHR}/{BART|upper}/{DAT_BELEG|date:01}",
			"broken":            "x/{NICHT_DA}",
		},
		FolderDefault: "Ablage/{doc.type}/{now|date:2006}/{now|date:01}",
	}
	return cfg
}

func TestFolderPathTemplates(t *testing.T) {
	cfg := folderConfig(t)
	now := time.Date(2026, 11, 8, 16, 0, 0, 0, time.UTC)
	cases := []struct {
		docType string
		index   map[string]string
		want    string
	}{
		{"IfrAnordnung", map[string]string{"Mand": "5530", "HH_JAHR": "2026", "AO_NR": "7151", "KZ_DKS": "D"}, "Anordnungen/2026/D/7001-8000"},
		{"IfrAnordnung", map[string]string{"HH_JAHR": "2026", "AO_NR": "1000", "KZ_DKS": "K"}, "Anordnungen/2026/K/1-1000"},
		{"IfrAnordnung", map[string]string{"HH_JAHR": "2026", "AO_NR": "1001", "KZ_DKS": "S"}, "Anordnungen/2026/S/1001-2000"},
		// keys are matched case-insensitively, a slash in a value never adds a level
		{"IfrAnordnung", map[string]string{"hh_jahr": "2026", "ao_nr": "7151", "kz_dks": "D/K"}, "Anordnungen/2026/D∕K/7001-8000"},
		{"IfrAnordnungListe", map[string]string{"HH_JAHR": "2026", "AO_NR": "7080"}, "Anordnungen/2026/Listen"},
		// client date format of the namespace
		{"IfrVeran", map[string]string{"HH_JAHR": "2026", "BART": "gs", "DAT_BELEG": "15.03.2026"}, "Veranlagung/2026/GS/03"},
		// no template for the type: folder_default with import date
		{"rechnung", map[string]string{"BELEG_NR": "1"}, "Ablage/rechnung/2026/11"},
		// template with a missing key: fall back to folder_default
		{"broken", map[string]string{}, "Ablage/broken/2026/11"},
	}
	for _, tc := range cases {
		if got := cfg.FolderPath("saskia", tc.docType, tc.index, now); got != tc.want {
			t.Errorf("FolderPath(%s, %v) = %q, want %q", tc.docType, tc.index, got, tc.want)
		}
	}

	// missing value for the Anordnung template and no default → empty (space root)
	cfg.Namespaces["saskia"] = NamespaceConfig{Folders: map[string]string{"IfrAnordnung": "A/{HH_JAHR}"}}
	if got := cfg.FolderPath("saskia", "IfrAnordnung", map[string]string{}, now); got != "" {
		t.Errorf("without default = %q, want empty", got)
	}
	// unknown user → default namespace without templates → empty
	if got := folderConfig(t).FolderPath("someone", "IfrAnordnung", map[string]string{"HH_JAHR": "2026"}, now); got != "" {
		t.Errorf("default namespace = %q, want empty", got)
	}
}

func TestExpandPlaceholderModifiers(t *testing.T) {
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	lookup := map[string]string{"nr": "42", "name": "Müller", "leer": ""}
	cases := map[string]string{
		"{nr|pad:5}":               "00042",
		"{name|left:3}":            "Mül",
		"{name|right:4|lower}":     "ller",
		"{leer|default:ohne}":      "ohne",
		"{now|date:2006-01}":       "2026-01",
		"{now}":                    "2026-01-02",
		"{nr|block:10}":            "41-50",
		"{nr|block:100|default:x}": "1-100",
	}
	for template, want := range cases {
		got, err := expandFolderTemplate(template, "", lookup, now, "")
		if err != nil || got != want {
			t.Errorf("%s = %q (%v), want %q", template, got, err, want)
		}
	}
	for _, template := range []string{"{leer}", "{nr|block:0}", "{nr|unknown}", "{name|block:10}", "{nr"} {
		if got, err := expandFolderTemplate(template, "", lookup, now, ""); err == nil {
			t.Errorf("%s = %q, want error", template, got)
		}
	}
}
