package metadata

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const lookupMappingYAML = `
namespaces:
  saskia:
    prefix: ctrl
    storage_case: lower
    client_case: upper
    key_separator: "."
    folders:
      IfrAnordnung: Anordnungen/{HH_JAHR}/{KZ_DKS}
    lookups:
      IfrAnordnung:
        keys: [PROD, PROD_G]
        names: [PROD_BEZ, PROD_G_BEZ]
        spaces: [Aktenplan]
        create: "{lookup.code} {lookup.name|default:ohne Bezeichnung}"
        subpath: "{doc.type}/{HH_JAHR}"
        numbered_levels: 1
users:
  saskia: saskia
`

func loadLookupConfig(t *testing.T) *Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mapping.yaml")
	if err := os.WriteFile(path, []byte(lookupMappingYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	return LoadConfig(path)
}

func TestKeySeparator(t *testing.T) {
	cfg := loadLookupConfig(t)
	cases := map[string]string{
		"PROD_G_BEZ": "ctrl.prod.g.bez",
		"HH_JAHR":    "ctrl.hh.jahr",
		"Mand":       "ctrl.mand",
	}
	for client, storage := range cases {
		if got := cfg.ToStorageKey("saskia", client); got != storage {
			t.Errorf("ToStorageKey(%s) = %q, want %q", client, got, storage)
		}
		if got := cfg.ToClientKey(storage); got != toUpper(client) {
			t.Errorf("ToClientKey(%s) = %q, want %q", storage, got, toUpper(client))
		}
	}
	// a namespace without separator keeps the underscore
	plain := DefaultConfig()
	if got := plain.ToStorageKey("", "PROD_G_BEZ"); got != "ctrl.prod_g_bez" {
		t.Errorf("default ToStorageKey = %q", got)
	}
}

func toUpper(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'a' && r <= 'z' {
			out[i] = r - 'a' + 'A'
		}
	}
	return string(out)
}

func TestLookupConfig(t *testing.T) {
	cfg := loadLookupConfig(t)
	rule := cfg.Lookup("saskia", "ifranordnung")
	if rule == nil {
		t.Fatal("Lookup(IfrAnordnung) = nil")
	}
	if rule.MatchKey() != "oy.fileReference" {
		t.Errorf("MatchKey = %q", rule.MatchKey())
	}
	if len(rule.Keys) != 2 || rule.Keys[1] != "PROD_G" || rule.Names[1] != "PROD_G_BEZ" || rule.NumberedLevels != 1 {
		t.Errorf("rule = %+v", rule)
	}
	if cfg.Lookup("saskia", "IfrVeran") != nil {
		t.Error("Lookup(IfrVeran) should be nil")
	}
	if cfg.Lookup("other", "IfrAnordnung") != nil {
		t.Error("Lookup for the default namespace should be nil")
	}

	index := map[string]string{"PROD": "", "PROD_G": "12.22.01.01", "PROD_G_BEZ": "Meldewesen", "HH_JAHR": "2026"}
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	name, err := cfg.ExpandTemplate("saskia", rule.Create, "IfrAnordnung", index,
		map[string]string{"lookup.code": "12.22.01.01", "lookup.name": "Meldewesen"}, now)
	if err != nil || name != "12.22.01.01 Meldewesen" {
		t.Errorf("create name = %q, %v", name, err)
	}
	sub, err := cfg.ExpandTemplate("saskia", rule.Subpath, "IfrAnordnung", index, nil, now)
	if err != nil || sub != "IfrAnordnung/2026" {
		t.Errorf("subpath = %q, %v", sub, err)
	}
	name, err = cfg.ExpandTemplate("saskia", rule.Create, "IfrAnordnung", index,
		map[string]string{"lookup.code": "12.22.01.01", "lookup.name": ""}, now)
	if err != nil || name != "12.22.01.01 ohne Bezeichnung" {
		t.Errorf("create name without designation = %q, %v", name, err)
	}
}
