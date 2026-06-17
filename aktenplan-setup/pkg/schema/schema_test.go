package schema

import (
	"testing"
)

func TestParseAktenplanFile(t *testing.T) {
	// Integration test — requires a real aktenplan YAML.
	// Skipped if file not present (e.g. in CI or public checkout).
	const path = "../../../docs/aktenplan.yaml"
	ap, err := LoadFromFile(path)
	if err != nil {
		t.Skipf("Skipping: %v", err)
	}

	count := CountKnoten(ap.Aktenplan.Knoten)
	if count < 10 {
		t.Errorf("CountKnoten = %d, want >= 10", count)
	}
}

func TestParseMinimal(t *testing.T) {
	yaml := `
aktenplan:
  version: "1.0"
  knoten:
    - kennung: "01"
      name: "Test"
      kinder:
        - kennung: "01.01"
          name: "Sub"
`
	ap, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if CountKnoten(ap.Aktenplan.Knoten) != 2 {
		t.Errorf("want 2 knoten")
	}
}

func TestValidateDuplicateKennung(t *testing.T) {
	yaml := `
aktenplan:
  knoten:
    - kennung: "01"
    - kennung: "01"
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected error for duplicate kennung")
	}
}

func TestValidateEmpty(t *testing.T) {
	yaml := `
aktenplan:
  knoten: []
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected error for empty knoten")
	}
}

func TestWalk(t *testing.T) {
	yaml := `
aktenplan:
  knoten:
    - kennung: "11"
      name: "Verwaltung"
      kinder:
        - kennung: "11.01"
          name: "Org"
          kinder:
            - kennung: "11.01.01"
              name: "Satzung"
`
	ap, _ := Parse([]byte(yaml))

	var paths []string
	Walk(ap.Aktenplan.Knoten, "/Aktenplan", func(path string, k *Knoten, depth int) {
		paths = append(paths, path)
	})

	if len(paths) != 3 {
		t.Fatalf("want 3 paths, got %d: %v", len(paths), paths)
	}
	if paths[0] != "/Aktenplan/11 Verwaltung" {
		t.Errorf("paths[0] = %q", paths[0])
	}
}
