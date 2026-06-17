package cmis

import (
	"io"
	"net/http"
	"path"

	gateway "github.com/cs3org/go-cs3apis/cs3/gateway/v1beta1"
	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/rs/zerolog/log"
)

// GetContentStream handles cmisselector=content — downloads the file content.
func (h *Handler) GetContentStream(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	// Stat to verify it's a document
	statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "objectNotFound", "Object not found")
		return
	}
	if statRes.Info.Type != provider.ResourceType_RESOURCE_TYPE_FILE {
		writeError(w, 400, "streamNotSupported", "Content stream not available for folders")
		return
	}

	res, err := h.gw.Gateway.InitiateFileDownload(r.Context(), &provider.InitiateFileDownloadRequest{Ref: ref})
	if err != nil {
		log.Error().Err(err).Msg("cmis: download init failed")
		writeError(w, 500, "runtime", "Download error")
		return
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "runtime", res.Status.Message)
		return
	}
	if len(res.Protocols) == 0 {
		writeError(w, 500, "runtime", "No download protocol available")
		return
	}

	// Proxy download from data gateway
	sess := sessionFromCtx(r.Context())
	downloadReq, _ := http.NewRequestWithContext(r.Context(), "GET", res.Protocols[0].DownloadEndpoint, nil)
	if sess != nil {
		downloadReq.Header.Set("X-Access-Token", sess.CS3Token)
	}
	if res.Protocols[0].Token != "" {
		downloadReq.Header.Set("X-Reva-Transfer", res.Protocols[0].Token)
	}

	resp, err := http.DefaultClient.Do(downloadReq)
	if err != nil {
		writeError(w, 500, "runtime", "Download failed")
		return
	}
	defer resp.Body.Close()

	// Set content headers
	if statRes.Info.MimeType != "" {
		w.Header().Set("Content-Type", statRes.Info.MimeType)
	} else {
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	w.Header().Set("Content-Disposition", "inline; filename=\""+path.Base(statRes.Info.Path)+"\"")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// SetContentStream handles cmisaction=setContent — uploads/replaces file content.
func (h *Handler) SetContentStream(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	uploadRes, err := h.gw.Gateway.InitiateFileUpload(r.Context(), &provider.InitiateFileUploadRequest{Ref: ref})
	if err != nil {
		log.Error().Err(err).Msg("cmis: upload init failed")
		writeError(w, 500, "runtime", "Upload error")
		return
	}
	if uploadRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 500, "runtime", uploadRes.Status.Message)
		return
	}
	if len(uploadRes.Protocols) == 0 {
		writeError(w, 500, "runtime", "No upload protocol available")
		return
	}

	// Get content from multipart form or raw body
	var reader io.Reader
	file, _, fileErr := r.FormFile("content")
	if fileErr == nil {
		defer file.Close()
		reader = file
	} else {
		reader = r.Body
	}

	if err := h.uploadContent(r, uploadRes.Protocols[0], reader); err != nil {
		writeError(w, 500, "runtime", "Upload failed")
		return
	}

	// Return updated object
	h.GetObject(w, r, ref)
}

// uploadContent sends content to the data gateway.
func (h *Handler) uploadContent(r *http.Request, proto *gateway.FileUploadProtocol, reader io.Reader) error {
	uploadReq, _ := http.NewRequestWithContext(r.Context(), "PUT", proto.UploadEndpoint, reader)
	sess := sessionFromCtx(r.Context())
	if sess != nil {
		uploadReq.Header.Set("X-Access-Token", sess.CS3Token)
	}
	if proto.Token != "" {
		uploadReq.Header.Set("X-Reva-Transfer", proto.Token)
	}

	resp, err := http.DefaultClient.Do(uploadReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return io.ErrUnexpectedEOF
	}
	return nil
}
