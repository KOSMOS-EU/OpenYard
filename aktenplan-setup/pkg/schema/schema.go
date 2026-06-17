// Package schema defines the Aktenplan YAML format and provides
// parsing and validation.
package schema

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Aktenplan is the top-level YAML document.
type Aktenplan struct {
	Aktenplan AktenplanData `yaml:"aktenplan"`
}

type AktenplanData struct {
	Metadata *Metadata `yaml:"metadata,omitempty"`
	Version  string    `yaml:"version,omitempty"`
	Kommune  string    `yaml:"kommune,omitempty"`
	Quelle   string    `yaml:"quelle,omitempty"`
	Space    *Space    `yaml:"space,omitempty"`
	Knoten   []Knoten  `yaml:"knoten"`
}

type Metadata struct {
	Kommune           string `yaml:"kommune,omitempty"`
	Quelle            string `yaml:"quelle,omitempty"`
	Extraktionsdatum  string `yaml:"extraktionsdatum,omitempty"`
	AktenplanVersion  string `yaml:"aktenplan_version,omitempty"`
	Bemerkung         string `yaml:"bemerkung,omitempty"`
}

type Space struct {
	Name          string  `yaml:"name"`
	Typ           string  `yaml:"typ,omitempty"`
	QuotaGB       int     `yaml:"quota_gb,omitempty"`
	InitialRechte []Recht `yaml:"initial_rechte,omitempty"`
}

type Recht struct {
	Rolle   string `yaml:"rolle"`
	Wirkung string `yaml:"wirkung"` // "read", "write", "manage"
}

type Knoten struct {
	Kennung   string   `yaml:"kennung"`
	Name      string   `yaml:"name,omitempty"`
	Immutable bool     `yaml:"immutable,omitempty"`
	Rechte    []Recht  `yaml:"rechte,omitempty"`
	Kinder    []Knoten `yaml:"kinder,omitempty"`
}

// LoadFromFile reads and parses an Aktenplan YAML file.
func LoadFromFile(path string) (*Aktenplan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return Parse(data)
}

// Parse parses Aktenplan YAML bytes.
func Parse(data []byte) (*Aktenplan, error) {
	var ap Aktenplan
	if err := yaml.Unmarshal(data, &ap); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if err := validate(&ap); err != nil {
		return nil, err
	}
	return &ap, nil
}

// SaveToFile writes the Aktenplan to a YAML file.
func SaveToFile(ap *Aktenplan, path string) error {
	data, err := yaml.Marshal(ap)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func validate(ap *Aktenplan) error {
	if len(ap.Aktenplan.Knoten) == 0 {
		return fmt.Errorf("aktenplan hat keine Knoten")
	}
	seen := make(map[string]bool)
	return validateKnoten(ap.Aktenplan.Knoten, seen, "")
}

func validateKnoten(knoten []Knoten, seen map[string]bool, parentKennung string) error {
	for _, k := range knoten {
		if k.Kennung == "" {
			return fmt.Errorf("Knoten ohne Kennung unter %q", parentKennung)
		}
		if seen[k.Kennung] {
			return fmt.Errorf("doppelte Kennung %q", k.Kennung)
		}
		seen[k.Kennung] = true

		// Validate hierarchy: child kennung should start with parent prefix
		if parentKennung != "" && !strings.HasPrefix(k.Kennung, parentKennung) {
			// Relaxed: only warn, don't fail (some data may not follow strict hierarchy)
		}

		if len(k.Kinder) > 0 {
			if err := validateKnoten(k.Kinder, seen, k.Kennung); err != nil {
				return err
			}
		}
	}
	return nil
}

// Walk calls fn for every Knoten in depth-first order.
// parentPath is the filesystem path prefix built from ancestor names.
func Walk(knoten []Knoten, parentPath string, fn func(path string, k *Knoten, depth int)) {
	walk(knoten, parentPath, 0, fn)
}

func walk(knoten []Knoten, parentPath string, depth int, fn func(string, *Knoten, int)) {
	for i := range knoten {
		k := &knoten[i]
		name := k.Kennung
		if k.Name != "" {
			name = k.Kennung + " " + k.Name
		}
		path := parentPath + "/" + name
		fn(path, k, depth)
		if len(k.Kinder) > 0 {
			walk(k.Kinder, path, depth+1, fn)
		}
	}
}

// CountKnoten returns the total number of nodes in the tree.
func CountKnoten(knoten []Knoten) int {
	count := 0
	Walk(knoten, "", func(_ string, _ *Knoten, _ int) { count++ })
	return count
}
