package handlers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/rs/zerolog/log"

	"github.com/kosmos-eu/openyard/pkg/metadata"
)

// File plan lookup (metadata config "lookups"): an import is filed below the
// folder whose metadata (default oy.fileReference) equals a code from the
// DocIndex, e.g. the product number of an Anordnung. Folders of the searched
// spaces are indexed by a walk over the tree and cached; a miss refreshes the
// cache before a folder is created, so a folder created by someone else is
// found without waiting for the search index.

const (
	lookupCacheTTL       = 10 * time.Minute
	lookupRefreshMinGap  = 20 * time.Second
	lookupMaxDepth       = 16
	lookupMaxFolders     = 100000
	lookupNumberPadding  = 2
	lookupReferenceJoin  = "-"
	lookupCodeLevelJoin  = "."
	lookupSpaceTypeMatch = "project"
)

// refIndex is the cached folder index of one space: match value → folder.
type refIndex struct {
	loaded  time.Time
	byValue map[string]*provider.ResourceId
}

type lookupCache struct {
	mu      sync.Mutex
	indexes map[string]*refIndex // key: encoded space root + match key
}

func newLookupCache() *lookupCache {
	return &lookupCache{indexes: map[string]*refIndex{}}
}

// lookupCacheRef returns the handlers' folder index cache, creating it on
// first use (handlers built outside New).
func (h *Handlers) lookupCacheRef() *lookupCache {
	if h.lookups == nil {
		h.lookups = newLookupCache()
	}
	return h.lookups
}

// lookupFolderID resolves the target folder of an import through the
// namespace's lookup rule. ok is false when no rule applies or the document
// carries no code; the caller then uses the path templates.
func (h *Handlers) lookupFolderID(ctx context.Context, login, docType, spaceID string, index map[string]string, now time.Time) (string, bool) {
	rule := h.metaConfig().Lookup(login, docType)
	if rule == nil || len(rule.Keys) == 0 {
		return "", false
	}
	code, name := lookupCodeAndName(rule, index)
	if code == "" {
		log.Debug().Str("docType", docType).Strs("keys", rule.Keys).Msg("lookup: document carries no code, using folder templates")
		return "", false
	}
	roots, err := h.lookupRoots(ctx, rule, spaceID)
	if err != nil {
		log.Warn().Err(err).Str("docType", docType).Msg("lookup: cannot resolve spaces")
		return "", false
	}
	matchKey := rule.MatchKey()
	folder, root := h.findByValue(ctx, roots, matchKey, code, true)
	if folder == nil {
		if rule.Create == "" {
			log.Warn().Str("docType", docType).Str("code", code).Str("match", matchKey).
				Msg("lookup: no folder carries the code and create is not configured, using folder templates")
			return "", false
		}
		folder, root, err = h.createLookupFolder(ctx, login, rule, docType, code, name, index, roots, now)
		if err != nil {
			log.Warn().Err(err).Str("docType", docType).Str("code", code).Msg("lookup: cannot create folder, using folder templates")
			return "", false
		}
	}
	target := folder
	if rule.Subpath != "" {
		subpath, err := h.metaConfig().ExpandTemplate(login, rule.Subpath, docType, index,
			map[string]string{"lookup.code": code, "lookup.name": name}, now)
		if err != nil {
			log.Warn().Err(err).Str("docType", docType).Msg("lookup: subpath template failed, filing into the matched folder")
		} else {
			target, err = h.ensureSubpath(ctx, root, folder, code, subpath, matchKey, rule.NumberedLevels, docType)
			if err != nil {
				log.Warn().Err(err).Str("docType", docType).Str("subpath", subpath).Msg("lookup: cannot prepare subpath, using folder templates")
				return "", false
			}
		}
	}
	log.Debug().Str("space", encodeObjectID(root)).Str("docType", docType).Str("code", code).Str("folder", encodeObjectID(target)).Msg("lookup applied")
	return encodeObjectID(target), true
}

// lookupCodeAndName returns the first non-empty code of the rule's keys and
// the designation configured at the same position.
func lookupCodeAndName(rule *metadata.LookupConfig, index map[string]string) (string, string) {
	lower := make(map[string]string, len(index))
	for key, value := range index {
		lower[strings.ToLower(key)] = strings.TrimSpace(value)
	}
	for position, key := range rule.Keys {
		code := lower[strings.ToLower(key)]
		if code == "" || code == "­" || code == "-" {
			continue
		}
		name := ""
		if position < len(rule.Names) {
			name = lower[strings.ToLower(rule.Names[position])]
		}
		return code, name
	}
	return "", ""
}

// lookupRoots returns the roots of the spaces a rule searches: the named
// spaces, or the import's target space.
func (h *Handlers) lookupRoots(ctx context.Context, rule *metadata.LookupConfig, spaceID string) ([]*provider.ResourceId, error) {
	if len(rule.Spaces) == 0 {
		root, err := h.importSpaceRoot(ctx, spaceID)
		if err != nil {
			return nil, err
		}
		return []*provider.ResourceId{root}, nil
	}
	spacesRes, err := h.gw.Gateway.ListStorageSpaces(ctx, &provider.ListStorageSpacesRequest{})
	if err != nil {
		return nil, fmt.Errorf("list spaces: %w", err)
	}
	if spacesRes.Status.Code != rpc.Code_CODE_OK {
		return nil, fmt.Errorf("list spaces: %s", spacesRes.Status.Message)
	}
	var roots []*provider.ResourceId
	for _, wanted := range rule.Spaces {
		for _, space := range spacesRes.StorageSpaces {
			if space.Root != nil && strings.EqualFold(space.Name, wanted) {
				roots = append(roots, space.Root)
				break
			}
		}
	}
	if len(roots) == 0 {
		return nil, fmt.Errorf("none of the spaces %v is visible to the user", rule.Spaces)
	}
	return roots, nil
}

// findByValue returns the folder whose matchKey metadata equals value in one
// of the roots, and that root. With refresh, a miss on a cached index
// re-walks the space (at most every lookupRefreshMinGap) before giving up.
func (h *Handlers) findByValue(ctx context.Context, roots []*provider.ResourceId, matchKey, value string, refresh bool) (*provider.ResourceId, *provider.ResourceId) {
	for _, root := range roots {
		idx := h.lookupIndex(ctx, root, matchKey, false)
		if folder := idx.byValue[value]; folder != nil {
			return folder, root
		}
	}
	if !refresh {
		return nil, nil
	}
	for _, root := range roots {
		idx := h.lookupIndex(ctx, root, matchKey, true)
		if folder := idx.byValue[value]; folder != nil {
			return folder, root
		}
	}
	return nil, nil
}

// lookupIndex returns the cached folder index of a space, walking the tree
// when the cache is empty, stale, or a refresh is requested.
func (h *Handlers) lookupIndex(ctx context.Context, root *provider.ResourceId, matchKey string, refresh bool) *refIndex {
	cache := h.lookupCacheRef()
	cacheKey := encodeObjectID(root) + "|" + matchKey
	cache.mu.Lock()
	idx := cache.indexes[cacheKey]
	stale := idx == nil || time.Since(idx.loaded) > lookupCacheTTL
	tooRecent := idx != nil && time.Since(idx.loaded) < lookupRefreshMinGap
	cache.mu.Unlock()
	if idx != nil && !stale && !(refresh && !tooRecent) {
		return idx
	}
	fresh := &refIndex{loaded: time.Now(), byValue: map[string]*provider.ResourceId{}}
	count := h.walkFolders(ctx, root, matchKey, fresh.byValue, 0, 0)
	log.Info().Str("space", encodeObjectID(root)).Str("match", matchKey).Int("folders", count).Int("withValue", len(fresh.byValue)).
		Msg("lookup: folder index built")
	cache.mu.Lock()
	cache.indexes[cacheKey] = fresh
	cache.mu.Unlock()
	return fresh
}

// walkFolders lists the folders below parent recursively and records those
// carrying matchKey. Returns the number of folders visited.
func (h *Handlers) walkFolders(ctx context.Context, parent *provider.ResourceId, matchKey string, byValue map[string]*provider.ResourceId, depth, visited int) int {
	if depth > lookupMaxDepth || visited > lookupMaxFolders {
		return visited
	}
	listRes, err := h.gw.Gateway.ListContainer(ctx, &provider.ListContainerRequest{
		Ref:                   &provider.Reference{ResourceId: parent},
		ArbitraryMetadataKeys: []string{matchKey},
	})
	if err != nil || listRes.Status.Code != rpc.Code_CODE_OK {
		log.Warn().Err(err).Str("folder", encodeObjectID(parent)).Msg("lookup: cannot list folder")
		return visited
	}
	for _, info := range listRes.Infos {
		if info.Type != provider.ResourceType_RESOURCE_TYPE_CONTAINER || info.Id == nil {
			continue
		}
		visited++
		if value := strings.TrimSpace(info.ArbitraryMetadata.GetMetadata()[matchKey]); value != "" {
			if _, dup := byValue[value]; !dup {
				byValue[value] = info.Id
			}
		}
		visited = h.walkFolders(ctx, info.Id, matchKey, byValue, depth+1, visited)
	}
	return visited
}

// rememberFolder adds a created folder to the cached index of its root.
func (h *Handlers) rememberFolder(root *provider.ResourceId, matchKey, value string, folder *provider.ResourceId) {
	cache := h.lookupCacheRef()
	cacheKey := encodeObjectID(root) + "|" + matchKey
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if idx := cache.indexes[cacheKey]; idx != nil {
		idx.byValue[value] = folder
	}
}

// createLookupFolder creates the folder for a code below the nearest existing
// ancestor by code prefix (11.16.13.01 → 11.16.13 → 11.16 → 11), else below
// the first root, names it by the rule's create template and writes the code
// as match metadata.
func (h *Handlers) createLookupFolder(ctx context.Context, login string, rule *metadata.LookupConfig, docType, code, name string,
	index map[string]string, roots []*provider.ResourceId, now time.Time) (*provider.ResourceId, *provider.ResourceId, error) {
	matchKey := rule.MatchKey()
	folderName, err := h.metaConfig().ExpandTemplate(login, rule.Create, docType, index,
		map[string]string{"lookup.code": code, "lookup.name": name}, now)
	if err != nil {
		return nil, nil, fmt.Errorf("create template: %w", err)
	}
	folderName = strings.ReplaceAll(folderName, "/", "∕")

	parent, root := roots[0], roots[0]
	parentCode := ""
	for prefix := parentPrefix(code); prefix != ""; prefix = parentPrefix(prefix) {
		if folder, foundRoot := h.findByValue(ctx, roots, matchKey, prefix, false); folder != nil {
			parent, root, parentCode = folder, foundRoot, prefix
			break
		}
	}
	folder, created, err := h.ensureChildFolder(ctx, parent, folderName)
	if err != nil {
		return nil, nil, err
	}
	if err := h.setFolderMetadata(ctx, folder, map[string]string{matchKey: code}); err != nil {
		return nil, nil, err
	}
	h.rememberFolder(root, matchKey, code, folder)
	log.Info().Bool("created", created).Str("docType", docType).Str("name", folderName).Str("code", code).
		Str("match", matchKey).Str("parentCode", parentCode).Str("parent", encodeObjectID(parent)).Str("folder", encodeObjectID(folder)).
		Msg("lookup: folder for code prepared")
	return folder, root, nil
}

// parentPrefix cuts the last level of a dotted code: 11.16.13.01 → 11.16.13; "" at the top.
func parentPrefix(code string) string {
	if cut := strings.LastIndex(code, lookupCodeLevelJoin); cut > 0 {
		return code[:cut]
	}
	return ""
}

// ensureSubpath creates the subpath levels below the matched folder. The
// first numberedLevels created levels get matchKey = <parent value>-NN with
// the lowest free NN among their siblings; existing folders are left as they
// are. Returns the last folder.
func (h *Handlers) ensureSubpath(ctx context.Context, root, folder *provider.ResourceId, code, subpath, matchKey string, numberedLevels int, docType string) (*provider.ResourceId, error) {
	parent, parentValue := folder, code
	for level, segment := range strings.Split(subpath, "/") {
		child, created, err := h.ensureChildFolder(ctx, parent, segment)
		if err != nil {
			return nil, err
		}
		childValue := ""
		if level < numberedLevels {
			if created {
				childValue, err = h.nextFreeReference(ctx, parent, matchKey, parentValue)
				if err != nil {
					return nil, err
				}
				if err := h.setFolderMetadata(ctx, child, map[string]string{matchKey: childValue}); err != nil {
					return nil, err
				}
				h.rememberFolder(root, matchKey, childValue, child)
			} else {
				childValue = h.folderValue(ctx, child, matchKey)
			}
		}
		if created {
			log.Info().Str("docType", docType).Str("name", segment).Str("reference", childValue).Str("parentReference", parentValue).
				Str("parent", encodeObjectID(parent)).Str("folder", encodeObjectID(child)).Int("level", level).
				Msg("lookup: subfolder created")
		}
		if childValue == "" {
			childValue = parentValue
		}
		parent, parentValue = child, childValue
	}
	return parent, nil
}

// nextFreeReference returns <parentValue>-NN with the lowest NN not used by a
// child folder of parent (two digits, more when needed).
func (h *Handlers) nextFreeReference(ctx context.Context, parent *provider.ResourceId, matchKey, parentValue string) (string, error) {
	listRes, err := h.gw.Gateway.ListContainer(ctx, &provider.ListContainerRequest{
		Ref:                   &provider.Reference{ResourceId: parent},
		ArbitraryMetadataKeys: []string{matchKey},
	})
	if err != nil {
		return "", fmt.Errorf("list siblings: %w", err)
	}
	if listRes.Status.Code != rpc.Code_CODE_OK {
		return "", fmt.Errorf("list siblings: %s", listRes.Status.Message)
	}
	prefix := parentValue + lookupReferenceJoin
	used := map[int]bool{}
	for _, info := range listRes.Infos {
		value := strings.TrimSpace(info.ArbitraryMetadata.GetMetadata()[matchKey])
		if !strings.HasPrefix(value, prefix) {
			continue
		}
		if number, err := strconv.Atoi(strings.TrimPrefix(value, prefix)); err == nil {
			used[number] = true
		}
	}
	next := 1
	for used[next] {
		next++
	}
	return prefix + fmt.Sprintf("%0*d", lookupNumberPadding, next), nil
}

// ensureChildFolder returns the child folder name below parent, creating it
// when missing; created reports whether it was created now.
func (h *Handlers) ensureChildFolder(ctx context.Context, parent *provider.ResourceId, name string) (*provider.ResourceId, bool, error) {
	ref := &provider.Reference{ResourceId: parent, Path: "./" + escapeFolderName(name)}
	statRes, err := h.gw.Gateway.Stat(ctx, &provider.StatRequest{Ref: ref})
	if err != nil {
		return nil, false, fmt.Errorf("stat %s: %w", name, err)
	}
	created := false
	if statRes.Status.Code == rpc.Code_CODE_NOT_FOUND {
		createRes, err := h.gw.Gateway.CreateContainer(ctx, &provider.CreateContainerRequest{Ref: ref})
		if err != nil {
			return nil, false, fmt.Errorf("create %s: %w", name, err)
		}
		if createRes.Status.Code != rpc.Code_CODE_OK && createRes.Status.Code != rpc.Code_CODE_ALREADY_EXISTS {
			return nil, false, fmt.Errorf("create %s: %s", name, createRes.Status.Message)
		}
		created = createRes.Status.Code == rpc.Code_CODE_OK
		statRes, err = h.gw.Gateway.Stat(ctx, &provider.StatRequest{Ref: ref})
		if err != nil {
			return nil, false, fmt.Errorf("stat %s after create: %w", name, err)
		}
	}
	if statRes.Status.Code != rpc.Code_CODE_OK || statRes.Info == nil || statRes.Info.Id == nil {
		return nil, false, fmt.Errorf("stat %s: %s", name, statRes.Status.GetMessage())
	}
	if statRes.Info.Type != provider.ResourceType_RESOURCE_TYPE_CONTAINER {
		return nil, false, fmt.Errorf("%s exists and is not a folder", name)
	}
	return statRes.Info.Id, created, nil
}

// folderValue reads one metadata value of a folder ("" when unset).
func (h *Handlers) folderValue(ctx context.Context, folder *provider.ResourceId, matchKey string) string {
	statRes, err := h.gw.Gateway.Stat(ctx, &provider.StatRequest{
		Ref:                   &provider.Reference{ResourceId: folder},
		ArbitraryMetadataKeys: []string{matchKey},
	})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK || statRes.Info == nil {
		return ""
	}
	return strings.TrimSpace(statRes.Info.ArbitraryMetadata.GetMetadata()[matchKey])
}

func (h *Handlers) setFolderMetadata(ctx context.Context, folder *provider.ResourceId, values map[string]string) error {
	res, err := h.gw.Gateway.SetArbitraryMetadata(ctx, &provider.SetArbitraryMetadataRequest{
		Ref:               &provider.Reference{ResourceId: folder},
		ArbitraryMetadata: &provider.ArbitraryMetadata{Metadata: values},
	})
	if err != nil {
		return fmt.Errorf("set metadata: %w", err)
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		return fmt.Errorf("set metadata: %s", res.Status.Message)
	}
	return nil
}

// sortedValues is used by tests and diagnostics: the cached match values of a space.
func (c *lookupCache) sortedValues(root *provider.ResourceId, matchKey string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	idx := c.indexes[encodeObjectID(root)+"|"+matchKey]
	if idx == nil {
		return nil
	}
	values := make([]string, 0, len(idx.byValue))
	for value := range idx.byValue {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}
