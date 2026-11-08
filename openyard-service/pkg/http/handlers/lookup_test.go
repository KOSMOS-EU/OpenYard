package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosmos-eu/openyard/pkg/metadata"
)

const lookupMappingYAML = `
namespaces:
  saskia:
    prefix: ctrl
    storage_case: lower
    client_case: upper
    source: saskia
    folders:
      IfrAnordnung: Anordnungen/{HH_JAHR}/{KZ_DKS}
    folder_default: Ablage/{doc.type}
    lookups:
      IfrAnordnung:
        keys: [PROD, PROD_G]
        names: [PROD_BEZ, PROD_G_BEZ]
        spaces: [Aktenplan]
        create: "{lookup.code} {lookup.name}"
        subpath: "Anordnungen/{HH_JAHR}"
        numbered_levels: 1
users:
  saskia: saskia
`

// lookupEnv is a WinYard environment with a file plan space:
//
//	Aktenplan/
//	  11 Finanzen                [oy.fileReference 11]
//	    11.16 Beschaffung        [11.16]
//	      11.16.13 Zentrale Dienste  [11.16.13]
//	  12 Ordnung                 [12]
//	    12.22 Einwohner          [12.22]
//	      12.22.01 Meldestelle   [12.22.01]
//	        12.22.01.01 Meldewesen [12.22.01.01]
//	          Sonstiges          [12.22.01.01-01]
func newLookupEnv(t *testing.T) (*winyardEnv, *fakeNode) {
	t.Helper()
	env := newWinyardEnv(t)
	configPath := filepath.Join(t.TempDir(), "mapping.yaml")
	if err := os.WriteFile(configPath, []byte(lookupMappingYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	env.handlers.metaCfg = metadata.LoadConfig(configPath)

	plan := env.storage.addSpace("Aktenplan")
	ref := func(code string) map[string]string { return map[string]string{"oy.fileReference": code} }
	fin := env.storage.addNode(plan, "11 Finanzen", true, nil, ref("11"))
	besch := env.storage.addNode(fin, "11.16 Beschaffung", true, nil, ref("11.16"))
	env.storage.addNode(besch, "11.16.13 Zentrale Dienste", true, nil, ref("11.16.13"))
	ord := env.storage.addNode(plan, "12 Ordnung", true, nil, ref("12"))
	einw := env.storage.addNode(ord, "12.22 Einwohner", true, nil, ref("12.22"))
	melde := env.storage.addNode(einw, "12.22.01 Meldestelle", true, nil, ref("12.22.01"))
	meldewesen := env.storage.addNode(melde, "12.22.01.01 Meldewesen", true, nil, ref("12.22.01.01"))
	env.storage.addNode(meldewesen, "Sonstiges", true, nil, ref("12.22.01.01-01"))
	return env, plan
}

func (env *winyardEnv) importAnordnung(t *testing.T, aoNr, docIndexJSON string) *fakeNode {
	t.Helper()
	body := `{"FileContent":"JVBERiBuZXc=","DocIndex":{"Mand":"5530","HH_JAHR":"2026","AO_NR":"` + aoNr + `","KZ_DKS":"K",` + docIndexJSON + `}}`
	recorder := env.call("POST", "/api/basicDocuments/ImportDocumentDynamic",
		"ConfigName=IfrAnordnung&FileName=AO"+aoNr+".pdf", "application/json", []byte(body))
	var id string
	decodeBody(t, recorder, &id)
	node := env.storage.nodes[id]
	if node == nil {
		t.Fatalf("import AO %s: no node for %q, body %s", aoNr, id, recorder.Body.String())
	}
	return node
}

func TestLookupFilesIntoExistingPlanFolder(t *testing.T) {
	env, _ := newLookupEnv(t)
	// counter booking side carries the product (expense), PROD itself is empty
	node := env.importAnordnung(t, "6001", `"PROD":"","PROD_BEZ":"","PROD_G":"12.22.01.01","PROD_G_BEZ":"Meldewesen"`)
	wantPath := "/Aktenplan/12 Ordnung/12.22 Einwohner/12.22.01 Meldestelle/12.22.01.01 Meldewesen/Anordnungen/2026/AO6001.pdf"
	if got := env.storage.nodePath(node); got != wantPath {
		t.Fatalf("filed to %q, want %q", got, wantPath)
	}
	// the created type level continues the numbering after the existing -01, the year level is not numbered
	anordnungen := env.storage.nodes[env.storage.nodes[node.parentID].parentID]
	if anordnungen.metadata["oy.fileReference"] != "12.22.01.01-02" {
		t.Errorf("Anordnungen reference = %q, want 12.22.01.01-02", anordnungen.metadata["oy.fileReference"])
	}
	if year := env.storage.nodes[node.parentID]; year.metadata["oy.fileReference"] != "" {
		t.Errorf("year folder reference = %q, want none", year.metadata["oy.fileReference"])
	}

	// second Anordnung of the same product reuses the folders, no new numbering
	second := env.importAnordnung(t, "6002", `"PROD":"12.22.01.01","PROD_BEZ":"Meldewesen"`)
	if second.parentID != node.parentID {
		t.Errorf("second import in %q, want same folder", env.storage.nodePath(second))
	}
	count := 0
	for _, n := range env.storage.nodes {
		if n.isDir && n.parentID == anordnungen.parentID {
			count++
		}
	}
	if count != 2 { // Sonstiges + Anordnungen
		t.Errorf("product folder has %d subfolders, want 2", count)
	}
}

func TestLookupCreatesMissingProductFolder(t *testing.T) {
	env, _ := newLookupEnv(t)
	node := env.importAnordnung(t, "6376", `"PROD":"11.16.13.01","PROD_BEZ":"Zentrale Beschaffungsstelle"`)
	wantPath := "/Aktenplan/11 Finanzen/11.16 Beschaffung/11.16.13 Zentrale Dienste/11.16.13.01 Zentrale Beschaffungsstelle/Anordnungen/2026/AO6376.pdf"
	if got := env.storage.nodePath(node); got != wantPath {
		t.Fatalf("filed to %q, want %q", got, wantPath)
	}
	anordnungen := env.storage.nodes[env.storage.nodes[node.parentID].parentID]
	product := env.storage.nodes[anordnungen.parentID]
	if product.metadata["oy.fileReference"] != "11.16.13.01" {
		t.Errorf("product folder reference = %q", product.metadata["oy.fileReference"])
	}
	if anordnungen.metadata["oy.fileReference"] != "11.16.13.01-01" {
		t.Errorf("Anordnungen reference = %q, want 11.16.13.01-01", anordnungen.metadata["oy.fileReference"])
	}

	// a product with no known ancestor is created below the space root
	orphan := env.importAnordnung(t, "6377", `"PROD":"99.01","PROD_BEZ":"Unbekannt"`)
	if got := env.storage.nodePath(orphan); !strings.HasPrefix(got, "/Aktenplan/99.01 Unbekannt/Anordnungen/2026/") {
		t.Errorf("orphan product filed to %q", got)
	}
}

func TestLookupFallsBackToTemplates(t *testing.T) {
	env, _ := newLookupEnv(t)
	// no product at all: Saskia space rule as before
	node := env.importAnordnung(t, "6500", `"PROD":"","PROD_G":""`)
	if got := env.storage.nodePath(node); got != "/Personal Admin/Anordnungen/2026/K/AO6500.pdf" {
		t.Errorf("fallback filed to %q", got)
	}
	// soft hyphen as Saskia sends it for an empty product
	node = env.importAnordnung(t, "6501", `"PROD":"­","PROD_G":"­"`)
	if got := env.storage.nodePath(node); got != "/Personal Admin/Anordnungen/2026/K/AO6501.pdf" {
		t.Errorf("soft hyphen fallback filed to %q", got)
	}
}

func TestLookupCacheSeesFoldersCreatedElsewhere(t *testing.T) {
	env, plan := newLookupEnv(t)
	// first import builds the index
	env.importAnordnung(t, "7001", `"PROD":"12.22.01.01","PROD_BEZ":"Meldewesen"`)
	// a product folder appears outside OpenYard
	ord := env.storage.childByName(plan, "12 Ordnung")
	env.storage.addNode(ord, "12.23 Standesamt", true, nil, map[string]string{"oy.fileReference": "12.23"})
	// the miss refreshes the index (the gap check is bypassed by resetting the load time)
	for _, idx := range env.handlers.lookupCacheRef().indexes {
		idx.loaded = idx.loaded.Add(-lookupRefreshMinGap)
	}
	node := env.importAnordnung(t, "7002", `"PROD":"12.23","PROD_BEZ":"Standesamt"`)
	if got := env.storage.nodePath(node); got != "/Aktenplan/12 Ordnung/12.23 Standesamt/Anordnungen/2026/AO7002.pdf" {
		t.Errorf("filed to %q, want the folder created elsewhere", got)
	}
	// below a Gruppe (depth 2) the plan continues with a dotted level, not an Akte
	anordnungen := env.storage.nodes[env.storage.nodes[node.parentID].parentID]
	if anordnungen.metadata["oy.fileReference"] != "12.23.01" {
		t.Errorf("Anordnungen below 12.23 got %q, want 12.23.01", anordnungen.metadata["oy.fileReference"])
	}
	values := env.handlers.lookupCacheRef().sortedValues(env.storage.resourceID(plan), "oy.fileReference")
	if !containsString(values, "12.23") || !containsString(values, "12.23.01") {
		t.Errorf("cache values = %v", values)
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
