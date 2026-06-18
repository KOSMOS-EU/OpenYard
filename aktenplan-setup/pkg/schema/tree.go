package schema

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// TreeExport represents the trees/*.yaml format produced by the WinYard scanner.
type TreeExport struct {
	Volume       string `yaml:"volume"`
	ID           string `yaml:"id"`
	Aktz         string `yaml:"aktz"`
	TotalFolders int    `yaml:"total_folders"`
	TotalDocs    int    `yaml:"total_docs"`
	AktzCount    int    `yaml:"aktz_count"`
	Tree         TreeNode `yaml:"tree"`
}

type TreeNode struct {
	Name    string     `yaml:"name"`
	Aktz    string     `yaml:"aktz,omitempty"`
	Docs    int        `yaml:"docs,omitempty"`
	Folders []TreeNode `yaml:"folders,omitempty"`
}

// Aktenplan folder types derived from Aktenzeichen patterns.
const (
	TypeHauptgruppe = "hauptgruppe" // XX
	TypeGruppe      = "gruppe"      // XX.YY
	TypeSachgruppe  = "sachgruppe"  // XX.YY.ZZ
	TypeAkte        = "akte"        // XX.YY.ZZ.NN
	TypeRegister    = "register"    // XX.YY.ZZ.NN-MM
	TypeVorgang     = "vorgang"     // XX.YY.ZZ.NN-MM/N
	TypeBand        = "band"        // XX.YY.ZZ.NN-MM/N#N
)

// LoadTreeFromFile reads a trees/*.yaml file and converts it to the Aktenplan format.
func LoadTreeFromFile(path string) (*Aktenplan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseTree(data)
}

// ParseTree parses a trees/*.yaml and converts it to the internal Aktenplan model.
func ParseTree(data []byte) (*Aktenplan, error) {
	var tree TreeExport
	if err := yaml.Unmarshal(data, &tree); err != nil {
		return nil, fmt.Errorf("parse tree yaml: %w", err)
	}
	if tree.Volume == "" {
		return nil, fmt.Errorf("tree yaml: volume name missing")
	}

	// Build space name: "Aktz VolumeName"
	spaceName := tree.Volume
	if tree.Aktz != "" {
		spaceName = tree.Aktz + " " + tree.Volume
	}

	// Convert tree nodes to Knoten
	knoten := convertTreeNodes(tree.Tree.Folders, tree.Aktz)

	ap := &Aktenplan{
		Aktenplan: AktenplanData{
			Version: "1.0",
			Quelle:  "WinYard-Scan",
			Space: &Space{
				Name: spaceName,
			},
			Knoten: knoten,
		},
	}

	return ap, nil
}

// convertTreeNodes recursively converts TreeNode slices to Knoten slices.
func convertTreeNodes(nodes []TreeNode, parentAktz string) []Knoten {
	var result []Knoten
	for _, n := range nodes {
		k := Knoten{
			Kennung: n.Aktz,
			Name:    n.Name,
		}

		// Nodes without Aktz get a synthetic kennung from the name
		if k.Kennung == "" {
			k.Kennung = sanitizeKennung(n.Name)
		}

		// Determine type from Aktenzeichen pattern
		k.Typ = ClassifyAktenzeichen(n.Aktz)

		// Non-leaf nodes with structural types are protected (immutable)
		k.Immutable = isProtectedType(k.Typ)

		// Recurse into children
		if len(n.Folders) > 0 {
			k.Kinder = convertTreeNodes(n.Folders, n.Aktz)
		}

		result = append(result, k)
	}
	return result
}

// ClassifyAktenzeichen determines the folder type from an Aktenzeichen pattern.
//
//	XX            → hauptgruppe
//	XX.YY         → gruppe
//	XX.YY.ZZ      → sachgruppe
//	XX.YY.ZZ.NN   → akte
//	...-MM         → register
//	.../N          → vorgang
//	...#N          → band
func ClassifyAktenzeichen(aktz string) string {
	if aktz == "" {
		return ""
	}

	// Check for band marker (#)
	if strings.Contains(aktz, "#") {
		return TypeBand
	}
	// Check for vorgang marker (/)
	if strings.Contains(aktz, "/") {
		return TypeVorgang
	}
	// Check for register marker (-)
	if strings.Contains(aktz, "-") {
		return TypeRegister
	}

	// Count dot-separated segments
	parts := strings.Split(aktz, ".")
	switch len(parts) {
	case 1:
		return TypeHauptgruppe
	case 2:
		return TypeGruppe
	case 3:
		return TypeSachgruppe
	case 4:
		return TypeAkte
	default:
		return TypeAkte
	}
}

// isProtectedType returns true for structural types that should be immutable.
// Vorgänge and Bände are open for document filing.
func isProtectedType(typ string) bool {
	switch typ {
	case TypeHauptgruppe, TypeGruppe, TypeSachgruppe, TypeAkte, TypeRegister:
		return true
	default:
		return false
	}
}

func sanitizeKennung(name string) string {
	// Replace problematic chars, keep it short
	r := strings.NewReplacer("/", "_", "\\", "_", " ", "_")
	s := r.Replace(name)
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}
