package main

import (
	"context"
	"flag"
	"fmt"
	"os"

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
		fmt.Fprintf(os.Stderr, "Usage: aktenplan-apply [flags] <aktenplan.yaml>\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	yamlPath := flag.Arg(0)

	// Parse YAML
	ap, err := schema.LoadFromFile(yamlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	total := schema.CountKnoten(ap.Aktenplan.Knoten)
	fmt.Printf("Aktenplan geladen: %s (%d Knoten)\n", yamlPath, total)

	if *dryRun {
		fmt.Println()
	}

	// Apply
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
		os.Exit(1)
	}
	defer applier.Close()

	if err := applier.Run(context.Background(), ap); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
