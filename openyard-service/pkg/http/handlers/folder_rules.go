package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/rs/zerolog/log"
)

// ruleFolderID applies the namespace's folder templates to an import and
// returns the FolderID (encoded ObjectID) of the resulting folder, creating
// missing levels. Empty when no template applies or the folder cannot be
// prepared; the caller then uploads to the space root as before.
func (h *Handlers) ruleFolderID(ctx context.Context, login, docType, spaceID string, entries []DocIndexEntry) string {
	if h.gw == nil || h.gw.Gateway == nil {
		return ""
	}
	index := make(map[string]string, len(entries))
	for _, entry := range entries {
		index[entry.Key] = entry.Value
	}
	now := time.Now()
	// File plan lookup first (metadata config "lookups"), then the path templates
	if folderID, ok := h.lookupFolderID(ctx, login, docType, index, now); ok {
		return folderID
	}
	folderPath := h.metaConfig().FolderPath(login, docType, index, now)
	if folderPath == "" {
		return ""
	}
	folderID, err := h.ensureFolderPath(ctx, spaceID, folderPath)
	if err != nil {
		log.Warn().Err(err).Str("docType", docType).Str("folder", folderPath).
			Msg("folder rule: cannot prepare folder, importing to space root")
		return ""
	}
	log.Debug().Str("docType", docType).Str("folder", folderPath).Msg("folder rule applied")
	return folderID
}

// ensureFolderPath walks folderPath below the target space root (spaceID, or
// the personal/first project space like the uploader) and creates each
// missing level. Returns the encoded ObjectID of the last folder.
func (h *Handlers) ensureFolderPath(ctx context.Context, spaceID, folderPath string) (string, error) {
	root, err := h.importSpaceRoot(ctx, spaceID)
	if err != nil {
		return "", err
	}
	parent := root
	for _, segment := range strings.Split(folderPath, "/") {
		ref := &provider.Reference{ResourceId: parent, Path: "./" + escapeFolderName(segment)}
		statRes, err := h.gw.Gateway.Stat(ctx, &provider.StatRequest{Ref: ref})
		if err != nil {
			return "", fmt.Errorf("stat %s: %w", segment, err)
		}
		if statRes.Status.Code == rpc.Code_CODE_NOT_FOUND {
			createRes, err := h.gw.Gateway.CreateContainer(ctx, &provider.CreateContainerRequest{Ref: ref})
			if err != nil {
				return "", fmt.Errorf("create %s: %w", segment, err)
			}
			if createRes.Status.Code != rpc.Code_CODE_OK && createRes.Status.Code != rpc.Code_CODE_ALREADY_EXISTS {
				return "", fmt.Errorf("create %s: %s", segment, createRes.Status.Message)
			}
			if createRes.Status.Code == rpc.Code_CODE_OK {
				log.Info().Str("name", segment).Str("path", folderPath).Str("parent", encodeObjectID(parent)).
					Msg("folder rule: folder created")
			}
			statRes, err = h.gw.Gateway.Stat(ctx, &provider.StatRequest{Ref: ref})
			if err != nil {
				return "", fmt.Errorf("stat %s after create: %w", segment, err)
			}
		}
		if statRes.Status.Code != rpc.Code_CODE_OK || statRes.Info == nil || statRes.Info.Id == nil {
			return "", fmt.Errorf("stat %s: %s", segment, statRes.Status.GetMessage())
		}
		if statRes.Info.Type != provider.ResourceType_RESOURCE_TYPE_CONTAINER {
			return "", fmt.Errorf("%s exists and is not a folder", segment)
		}
		parent = statRes.Info.Id
	}
	return encodeObjectID(parent), nil
}

// importSpaceRoot resolves the space an import goes to: the given SpaceId, or
// the personal space, or the first project space (same order as upload.Reva).
func (h *Handlers) importSpaceRoot(ctx context.Context, spaceID string) (*provider.ResourceId, error) {
	if spaceID != "" {
		return decodeObjectID(spaceID)
	}
	spacesRes, err := h.gw.Gateway.ListStorageSpaces(ctx, &provider.ListStorageSpacesRequest{})
	if err != nil {
		return nil, fmt.Errorf("list spaces: %w", err)
	}
	if spacesRes.Status.Code != rpc.Code_CODE_OK {
		return nil, fmt.Errorf("list spaces: %s", spacesRes.Status.Message)
	}
	var selected *provider.StorageSpace
	for _, space := range spacesRes.StorageSpaces {
		if space.Root == nil {
			continue
		}
		if space.SpaceType == "personal" {
			selected = space
			break
		}
		if space.SpaceType == "project" && selected == nil {
			selected = space
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("no space for import")
	}
	return selected.Root, nil
}
