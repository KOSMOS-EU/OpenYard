package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kosmos-eu/openyard/aktenplan-setup/pkg/extract"
	"github.com/kosmos-eu/openyard/aktenplan-setup/pkg/schema"
)

func main() {
	mode := flag.String("mode", "api", "Extraction mode: api or sql")
	apiURL := flag.String("url", envOr("WINYARD_URL", "http://localhost:8080"), "WinYard API base URL")
	user := flag.String("user", envOr("WINYARD_USER", ""), "WinYard username")
	pass := flag.String("pass", envOr("WINYARD_PASS", ""), "WinYard password")
	startPath := flag.String("start-path", "/", "Start path for recursive crawl")
	output := flag.String("output", "aktenplan.yaml", "Output YAML file")
	kommune := flag.String("kommune", "", "Kommune name for metadata")
	maxDepth := flag.Int("max-depth", 20, "Maximum crawl depth")
	flag.Parse()

	switch *mode {
	case "api":
		if *user == "" || *pass == "" {
			fmt.Fprintf(os.Stderr, "Error: --user and --pass required for API mode\n")
			flag.PrintDefaults()
			os.Exit(1)
		}

		extractor := extract.NewAPIExtractor(extract.APIOptions{
			BaseURL:   *apiURL,
			Username:  *user,
			Password:  *pass,
			StartPath: *startPath,
			MaxDepth:  *maxDepth,
			Output:    os.Stderr,
		})

		ap, err := extractor.Extract()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if *kommune != "" {
			ap.Aktenplan.Kommune = *kommune
		}

		if err := schema.SaveToFile(ap, *output); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", *output, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Written to %s\n", *output)

	case "sql":
		fmt.Fprintf(os.Stderr, "SQL-Modus noch nicht implementiert.\n")
		fmt.Fprintf(os.Stderr, "Benötigt: --dsn, --table, --column-mapping\n")
		fmt.Fprintf(os.Stderr, "Siehe docs/openyard-aktenplan-task.md, Modus A\n")
		os.Exit(1)

	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s (use 'api' or 'sql')\n", *mode)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
