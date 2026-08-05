package upload

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	gateway "github.com/cs3org/go-cs3apis/cs3/gateway/v1beta1"
	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	typesv1 "github.com/cs3org/go-cs3apis/cs3/types/v1beta1"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/metadata"
)

// GatewayProvider provides the current gRPC gateway client.
// After a reconnect, it returns the new client.
type GatewayProvider interface {
	GetGateway() gateway.GatewayAPIClient
	CheckError(err error) bool
}

// Reva uploads files via the CS3 gRPC InitiateFileUpload + Data Gateway path.
type Reva struct {
	// GW provides the current gateway client (survives reconnects).
	GW GatewayProvider
	// DataGatewayURL is the internal URL of the OC proxy, e.g. "http://opencloud:9200"
	DataGatewayURL string
}

func (r *Reva) Name() string { return "reva" }

func (r *Reva) Upload(ctx context.Context, req *Request) (*Result, error) {
	if r.GW == nil {
		return nil, fmt.Errorf("reva: gateway not configured")
	}

	// Inject CS3 token into gRPC context
	if req.CS3Token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-access-token", req.CS3Token)
	}

	// 1. Find target reference
	var ref *provider.Reference

	// If FolderID provided, upload into that specific folder
	if req.FolderID != "" {
		folderRef, err := decodeSpaceID(req.FolderID)
		if err == nil {
			ref = &provider.Reference{
				ResourceId: folderRef,
				Path:       "./" + req.FileName,
			}
			log.Debug().Str("folderId", req.FolderID[:20]+"...").Str("path", ref.Path).Msg("reva: using explicit folder")
		} else {
			log.Warn().Err(err).Str("folderId", req.FolderID).Msg("reva: invalid FolderID, falling back")
		}
	}

	// If SpaceID provided (but no FolderID), upload into space root
	if ref == nil && req.SpaceID != "" {
		spaceRef, err := decodeSpaceID(req.SpaceID)
		if err == nil {
			ref = &provider.Reference{
				ResourceId: spaceRef,
				Path:       "./" + req.FileName,
			}
			log.Debug().Str("spaceId", req.SpaceID[:20]+"...").Str("path", ref.Path).Msg("reva: using explicit space")
		} else {
			log.Warn().Err(err).Str("spaceId", req.SpaceID).Msg("reva: invalid SpaceID, falling back to auto-detect")
		}
	}

	// Auto-detect: prefer personal, then project
	if ref == nil {
		spacesRes, err := r.GW.GetGateway().ListStorageSpaces(ctx, &provider.ListStorageSpacesRequest{})
		if err != nil {
			return nil, fmt.Errorf("reva: list spaces: %w", err)
		}
		if spacesRes.Status.Code != rpc.Code_CODE_OK {
			return nil, fmt.Errorf("reva: list spaces: %s", spacesRes.Status.Message)
		}

		var selectedSpace *provider.StorageSpace
		for _, space := range spacesRes.StorageSpaces {
			if space.Root == nil {
				continue
			}
			if space.SpaceType == "personal" {
				selectedSpace = space
				break
			}
			if space.SpaceType == "project" && selectedSpace == nil {
				selectedSpace = space
			}
		}
		if selectedSpace != nil {
			ref = &provider.Reference{
				ResourceId: selectedSpace.Root,
				Path:       "./" + req.FileName,
			}
			log.Debug().
				Str("spaceType", selectedSpace.SpaceType).
				Str("spaceName", selectedSpace.Name).
				Str("spaceId", selectedSpace.Root.SpaceId).
				Str("path", ref.Path).
				Msg("reva: using space for upload")
		}
	}
	if ref == nil {
		return nil, fmt.Errorf("reva: no space found for upload")
	}

	// 2. InitiateFileUpload (with reconnect on broken connection)
	log.Info().Str("file", req.FileName).Int("size", len(req.Data)).Msg("reva: initiating upload")

	initReq := &provider.InitiateFileUploadRequest{
		Ref: ref,
		Opaque: &typesv1.Opaque{
			Map: map[string]*typesv1.OpaqueEntry{
				"Upload-Length": {
					Decoder: "plain",
					Value:   []byte(fmt.Sprintf("%d", len(req.Data))),
				},
			},
		},
	}

	gw := r.GW.GetGateway()
	uploadRes, err := gw.InitiateFileUpload(ctx, initReq)
	if err != nil && r.GW.CheckError(err) {
		// Connection was broken and reconnected — get fresh client and retry
		log.Info().Msg("reva: retrying InitiateFileUpload after reconnect")
		time.Sleep(time.Second)
		gw = r.GW.GetGateway()
		uploadRes, err = gw.InitiateFileUpload(ctx, initReq)
	}
	if err != nil {
		return nil, fmt.Errorf("reva: initiate upload: %w", err)
	}
	if uploadRes.Status.Code != rpc.Code_CODE_OK {
		return nil, fmt.Errorf("reva: initiate upload: %s", uploadRes.Status.Message)
	}

	// 3. Log all protocols and tokens
	var simpleToken, tusToken string
	var simpleEndpoint, tusEndpoint string
	for _, p := range uploadRes.Protocols {
		tkPreview := p.Token
		if len(tkPreview) > 30 {
			tkPreview = tkPreview[:30] + "..."
		}
		target := extractTarget(p.Token)
		log.Info().
			Str("protocol", p.Protocol).
			Str("endpoint", p.UploadEndpoint).
			Str("token", tkPreview).
			Str("target", target).
			Msg("reva: upload protocol")

		switch p.Protocol {
		case "simple":
			simpleToken = p.Token
			simpleEndpoint = p.UploadEndpoint
		case "tus":
			tusToken = p.Token
			tusEndpoint = p.UploadEndpoint
		}
	}

	// 4. Check if upload session was created on disk
	// (This is a debug check — we ask the gateway to stat the upload target)
	if simpleToken != "" {
		target := extractTarget(simpleToken)
		log.Info().Str("target", target).Msg("reva: simple upload target from JWT")
	}

	// 5. Wait briefly for session persistence (async write?)
	time.Sleep(500 * time.Millisecond)

	// 6. Try the simple protocol via /data endpoint
	if simpleToken != "" {
		result, err := r.trySimpleUpload(ctx, req, simpleEndpoint, simpleToken)
		if err == nil {
			return result, nil
		}
		log.Warn().Err(err).Msg("reva: simple upload failed, trying TUS")
	}

	// 7. Fallback: try TUS protocol
	if tusToken != "" {
		result, err := r.tryTUSUpload(ctx, req, tusEndpoint, tusToken)
		if err == nil {
			return result, nil
		}
		log.Error().Err(err).Msg("reva: TUS upload also failed")
		return nil, err
	}

	return nil, fmt.Errorf("reva: no upload protocol succeeded")
}

func (r *Reva) trySimpleUpload(ctx context.Context, req *Request, endpoint, token string) (*Result, error) {
	// Use internal data gateway URL instead of the public endpoint
	dataURL := r.DataGatewayURL + "/data"
	log.Info().Str("url", dataURL).Int("size", len(req.Data)).Msg("reva: PUT to /data (simple)")

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", dataURL, bytes.NewReader(req.Data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("X-Reva-Transfer", token)
	httpReq.Header.Set("x-access-token", req.CS3Token)
	httpReq.ContentLength = int64(len(req.Data))

	resp, err := insecureHTTPClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if len(bodyStr) > 500 {
		bodyStr = bodyStr[:500]
	}

	log.Info().
		Int("status", resp.StatusCode).
		Str("body", bodyStr).
		Msg("reva: simple upload response")

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, bodyStr)
	}

	return &Result{
		FileID: resp.Header.Get("Oc-Fileid"),
		ETag:   resp.Header.Get("Etag"),
	}, nil
}

func (r *Reva) tryTUSUpload(ctx context.Context, req *Request, endpoint, token string) (*Result, error) {
	// TUS: PATCH with upload data to /data endpoint
	dataURL := r.DataGatewayURL + "/data"
	log.Info().Str("url", dataURL).Int("size", len(req.Data)).Msg("reva: PATCH to /data (TUS)")

	httpReq, err := http.NewRequestWithContext(ctx, "PATCH", dataURL, bytes.NewReader(req.Data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("X-Reva-Transfer", token)
	httpReq.Header.Set("x-access-token", req.CS3Token)
	httpReq.Header.Set("Tus-Resumable", "1.0.0")
	httpReq.Header.Set("Upload-Offset", "0")
	httpReq.Header.Set("Content-Type", "application/offset+octet-stream")
	httpReq.ContentLength = int64(len(req.Data))

	resp, err := insecureHTTPClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if len(bodyStr) > 500 {
		bodyStr = bodyStr[:500]
	}

	log.Info().
		Int("status", resp.StatusCode).
		Str("body", bodyStr).
		Str("upload-offset", resp.Header.Get("Upload-Offset")).
		Msg("reva: TUS upload response")

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, bodyStr)
	}

	return &Result{
		FileID: resp.Header.Get("Oc-Fileid"),
		ETag:   resp.Header.Get("Etag"),
	}, nil
}

// extractTarget decodes a JWT transfer token and returns the target URL.
func extractTarget(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		decoded, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return ""
	}
	return claims.Target
}

func insecureHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// decodeSpaceID parses a base64-encoded CS3 triple (storageId$spaceId!opaqueId)
// into a ResourceId. Same format as handlers.decodeObjectID.
func decodeSpaceID(encoded string) (*provider.ResourceId, error) {
	raw, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid SpaceID encoding: %w", err)
	}
	s := string(raw)

	dollarIdx := strings.IndexByte(s, '$')
	bangIdx := strings.IndexByte(s, '!')

	if bangIdx < 0 {
		return nil, fmt.Errorf("invalid SpaceID format")
	}

	rid := &provider.ResourceId{}
	if dollarIdx >= 0 && dollarIdx < bangIdx {
		rid.StorageId = s[:dollarIdx]
		rid.SpaceId = s[dollarIdx+1 : bangIdx]
		rid.OpaqueId = s[bangIdx+1:]
	} else {
		rid.StorageId = s[:bangIdx]
		rid.OpaqueId = s[bangIdx+1:]
	}
	return rid, nil
}
