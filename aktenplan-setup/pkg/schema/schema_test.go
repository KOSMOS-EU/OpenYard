package schema

import (
	"testing"
)

func TestParseBrandisAktenplan(t *testing.T) {
	ap, err := LoadFromFile("../../../docs/brandis-aktenplan.yaml")
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}

	if ap.Aktenplan.Kommune != "Brandis" {
		t.Errorf("Kommune = %q, want %q", ap.Aktenplan.Kommune, "Brandis")
	}

	count := CountKnoten(ap.Aktenplan.Knoten)
	if count < 100 {
		t.Errorf("CountKnoten = %d, want >= 100", count)
	}

	// Check first top-level node
	if len(ap.Aktenplan.Knoten) == 0 {
		t.Fatal("no top-level knoten")
	}
	first := ap.Aktenplan.Knoten[0]
	if first.Kennung != "11" {
		t.Errorf("first Kennung = %q, want %q", first.Kennung, "11")
	}
	if first.Name != "Innere Verwaltung" {
		t.Errorf("first Name = %q, want %q", first.Name, "Innere Verwaltung")
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
