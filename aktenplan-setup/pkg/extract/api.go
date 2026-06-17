// Package extract implements Aktenplan extraction from a WinYard API.
package extract

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/kosmos-eu/openyard/aktenplan-setup/pkg/schema"
)

// APIOptions configures API-mode extraction.
type APIOptions struct {
	BaseURL   string // e.g. "http://winyard:8080"
	Username  string
	Password  string
	StartPath string // e.g. "/Aktenplan"
	MaxDepth  int    // 0 = unlimited
	Output    io.Writer
}

// APIExtractor crawls a WinYard/OpenYard API to build an Aktenplan YAML.
type APIExtractor struct {
	opts      APIOptions
	client    *http.Client
	sessionID string
}

// NewAPIExtractor creates an extractor for API mode.
func NewAPIExtractor(opts APIOptions) *APIExtractor {
	if opts.MaxDepth == 0 {
		opts.MaxDepth = 20
	}
	return &APIExtractor{
		opts:   opts,
		client: &http.Client{},
	}
}

// Extract performs login, recursive crawl, and returns the Aktenplan.
func (e *APIExtractor) Extract() (*schema.Aktenplan, error) {
	if err := e.login(); err != nil {
		return nil, err
	}
	defer e.logout()

	fmt.Fprintf(e.opts.Output, "Crawling from %s ...\n", e.opts.StartPath)

	knoten, err := e.crawl(e.opts.StartPath, 0)
	if err != nil {
		return nil, fmt.Errorf("crawl %s: %w", e.opts.StartPath, err)
	}

	ap := &schema.Aktenplan{
		Aktenplan: schema.AktenplanData{
			Version: "1.0",
			Quelle:  "api_extract",
			Knoten:  knoten,
		},
	}

	total := schema.CountKnoten(knoten)
	fmt.Fprintf(e.opts.Output, "Extracted %d Knoten\n", total)
	return ap, nil
}

func (e *APIExtractor) login() error {
	body := fmt.Sprintf(`{"Username":"%s","Password":"%s"}`, e.opts.Username, e.opts.Password)
	resp, err := e.client.Post(
		e.opts.BaseURL+"/api/advancedUsers/Login",
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("login: HTTP %d", resp.StatusCode)
	}

	var result struct {
		SessionID string `json:"SessionID"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("login response: %w", err)
	}
	if result.SessionID == "" {
		return fmt.Errorf("login: no SessionID returned")
	}

	e.sessionID = result.SessionID
	fmt.Fprintf(e.opts.Output, "Logged in, SessionID obtained\n")
	return nil
}

func (e *APIExtractor) logout() {
	req, _ := http.NewRequest("GET", e.opts.BaseURL+"/api/advancedUsers/Logout?SessionID="+url.QueryEscape(e.sessionID), nil)
	e.client.Do(req)
}

func (e *APIExtractor) crawl(folderPath string, depth int) ([]schema.Knoten, error) {
	if depth >= e.opts.MaxDepth {
		return nil, nil
	}

	// Get folder contents
	body := fmt.Sprintf(`{"FolderPath":"%s"}`, folderPath)
	reqURL := fmt.Sprintf("%s/api/advancedFolders/GetFolderByFolderpath?SessionID=%s",
		e.opts.BaseURL, url.QueryEscape(e.sessionID))

	resp, err := e.client.Post(reqURL, "application/json", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GetFolderByFolderpath %s: HTTP %d", folderPath, resp.StatusCode)
	}

	var folder struct {
		Contents []struct {
			Name string `json:"name"`
			Type string `json:"type"`
			Path string `json:"path"`
		} `json:"contents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&folder); err != nil {
		return nil, err
	}

	// Filter and sort: only folders
	var dirs []struct {
		Name string
		Path string
	}
	for _, c := range folder.Contents {
		if c.Type == "folder" {
			dirs = append(dirs, struct {
				Name string
				Path string
			}{c.Name, c.Path})
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })

	var knoten []schema.Knoten
	for _, d := range dirs {
		// Parse kennung from folder name (first part before space)
		kennung, name := parseKennung(d.Name)

		k := schema.Knoten{
			Kennung: kennung,
			Name:    name,
		}

		// Recurse
		children, err := e.crawl(d.Path, depth+1)
		if err != nil {
			fmt.Fprintf(e.opts.Output, "  WARN: crawl %s: %v\n", d.Path, err)
		}
		k.Kinder = children

		knoten = append(knoten, k)
		fmt.Fprintf(e.opts.Output, "%s%s\n", strings.Repeat("  ", depth), d.Name)
	}

	return knoten, nil
}

// parseKennung splits "11.13.05.02 Rathaus Markt 1-3" into
// kennung="11.13.05.02" and name="Rathaus Markt 1-3".
// If no space, kennung=whole string, name="".
func parseKennung(s string) (string, string) {
	idx := strings.IndexByte(s, ' ')
	if idx < 0 {
		return s, ""
	}
	return s[:idx], s[idx+1:]
}
