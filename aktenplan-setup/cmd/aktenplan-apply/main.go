package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/kosmos-eu/openyard/aktenplan-setup/pkg/apply"
	"github.com/kosmos-eu/openyard/aktenplan-setup/pkg/schema"
)

func main() {
	gateway := flag.String("gateway", envOr("OPENYARD_REVA_GATEWAY", "127.0.0.1:9142"), "CS3 gateway address")
	user := flag.String("user", envOr("OPENYARD_ADMIN_USER", "admin"), "Admin username")
	pass := flag.String("pass", envOr("OPENYARD_ADMIN_PASS", ""), "Admin password")
	basePath := flag.String("base-path", "", "Override base path (default: from YAML space name)")
	dryRun := flag.Bool("dry-run", false, "Show what would be created without making changes")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: aktenplan-apply [flags] <file.yaml> [file2.yaml ...]\n\n")
		fmt.Fprintf(os.Stderr, "Supports two YAML formats:\n")
		fmt.Fprintf(os.Stderr, "  - Aktenplan format (aktenplan: knoten: [...])\n")
		fmt.Fprintf(os.Stderr, "  - Tree scan format (volume: ... tree: folders: [...])\n\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Process all YAML files
	for _, yamlPath := range flag.Args() {
		fmt.Printf("=== %s ===\n", yamlPath)

		ap, err := loadYAML(yamlPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", yamlPath, err)
			continue
		}

		total := schema.CountKnoten(ap.Aktenplan.Knoten)
		spaceName := ""
		if ap.Aktenplan.Space != nil {
			spaceName = ap.Aktenplan.Space.Name
		}
		fmt.Printf("  Space: %s, Knoten: %d\n", spaceName, total)

		if *dryRun {
			fmt.Println()
		}

		applier, err := apply.New(apply.Options{
			GatewayAddr: *gateway,
			Username:    *user,
			Password:    *pass,
			BasePath:    *basePath,
			DryRun:      *dryRun,
			Output:      os.Stdout,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		if err := applier.Run(context.Background(), ap); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		applier.Close()
		fmt.Println()
	}
}

// loadYAML auto-detects the YAML format and loads accordingly.
func loadYAML(path string) (*schema.Aktenplan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)

	// Detect format: tree scan has "volume:" at top level, aktenplan has "aktenplan:"
	if strings.Contains(content[:min(200, len(content))], "volume:") {
		return schema.ParseTree(data)
	}
	return schema.Parse(data)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
