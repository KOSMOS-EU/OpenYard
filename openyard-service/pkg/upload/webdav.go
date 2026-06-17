package upload

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
)

// WebDAV uploads files via HTTP PUT to the OpenCloud WebDAV endpoint.
// Requires PROXY_ENABLE_BASIC_AUTH=true on the OpenCloud container.
type WebDAV struct {
	// BaseURL is the internal OpenCloud URL, e.g. "http://opencloud:9200"
	BaseURL string
	// If true, skip TLS verification (for self-signed certs)
	Insecure bool
}

func (w *WebDAV) Name() string { return "webdav" }

func (w *WebDAV) Upload(ctx context.Context, req *Request) (*Result, error) {
	url := w.BaseURL + "/remote.php/webdav/" + req.FileName

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(req.Data))
	if err != nil {
		return nil, fmt.Errorf("webdav: create request: %w", err)
	}

	// Basic auth
	creds := base64.StdEncoding.EncodeToString([]byte(req.Username + ":" + req.Password))
	httpReq.Header.Set("Authorization", "Basic "+creds)
	httpReq.Header.Set("Content-Type", "application/octet-stream")
	httpReq.ContentLength = int64(len(req.Data))

	log.Info().Str("url", url).Int("size", len(req.Data)).Str("user", req.Username).Msg("webdav upload")

	client := w.httpClient()
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("webdav: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200]
		}
		log.Error().Int("status", resp.StatusCode).Str("body", bodyStr).Msg("webdav upload rejected")
		return nil, fmt.Errorf("webdav: server returned %d", resp.StatusCode)
	}

	result := &Result{
		ETag: resp.Header.Get("Etag"),
	}

	// OC returns Oc-Fileid header: storageId$spaceId!opaqueId
	if fileID := resp.Header.Get("Oc-Fileid"); fileID != "" {
		result.FileID = fileID
	}

	log.Info().Str("file", req.FileName).Str("fileid", result.FileID).Msg("webdav upload ok")
	return result, nil
}

func (w *WebDAV) httpClient() *http.Client {
	if w.Insecure {
		return &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}
	return http.DefaultClient
}
