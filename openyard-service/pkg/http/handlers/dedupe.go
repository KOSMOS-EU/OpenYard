package handlers

import (
	"context"
	"strings"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/rs/zerolog/log"
)

// findDuplicate returns the document in folder that has the same type and the
// same values for the namespace's unique keys (metadata config "unique_keys")
// as the import described by docType and index. nil when no unique keys are
// configured, a key value is missing, or nothing matches. The folder is the
// rule folder of the import (same metadata → same folder), so one listing
// suffices; no search index involved.
func (h *Handlers) findDuplicate(ctx context.Context, login, docType string, index map[string]string, folder *provider.ResourceId) *provider.ResourceInfo {
	if folder == nil || h.gw == nil || h.gw.Gateway == nil {
		return nil
	}
	keys := h.metaConfig().UniqueKeys(login, docType)
	if len(keys) == 0 {
		return nil
	}
	lookup := make(map[string]string, len(index))
	for key, value := range index {
		lookup[strings.ToLower(key)] = strings.TrimSpace(value)
	}
	wanted := make(map[string]string, len(keys))
	for _, key := range keys {
		value := lookup[strings.ToLower(key)]
		if value == "" {
			return nil
		}
		storageKey := mapMetaKeyForUser(h.metaCfg, login, key)
		wanted[storageKey] = h.metaConfig().ToStorageValue(login, storageKey, value)
	}
	listRes, err := h.gw.Gateway.ListContainer(ctx, &provider.ListContainerRequest{
		Ref:                   &provider.Reference{ResourceId: folder},
		ArbitraryMetadataKeys: []string{"*"},
	})
	if err != nil || listRes.Status.Code != rpc.Code_CODE_OK {
		return nil
	}
	for _, info := range listRes.Infos {
		if info.Type == provider.ResourceType_RESOURCE_TYPE_CONTAINER || info.Id == nil {
			continue
		}
		stored := info.GetArbitraryMetadata().GetMetadata()
		if stored == nil || !strings.EqualFold(stored["doc.type"], docType) {
			continue
		}
		match := true
		for storageKey, value := range wanted {
			if stored[storageKey] != value {
				match = false
				break
			}
		}
		if match {
			log.Info().Str("docType", docType).Interface("keys", wanted).Str("existing", info.Id.OpaqueId).
				Msg("unique key matches existing document, replacing content as new version")
			return info
		}
	}
	return nil
}
