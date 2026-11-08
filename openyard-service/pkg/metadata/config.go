// Package metadata provides configurable metadata key mapping
// between DMS clients (Saskia IFR, etc.) and OpenCloud storage (xattrs).
package metadata

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config defines how client metadata keys are mapped to storage keys.
type Config struct {
	// Namespaces defines client-specific namespaces.
	// Each namespace stores its keys under a prefix on the storage layer.
	Namespaces map[string]NamespaceConfig `yaml:"namespaces"`

	// Users maps login names to namespace names.
	// If a user is not listed, the "default" namespace is used.
	//   saskia: ctrl
	//   worker: ctrl
	//   import_hr: hr
	Users map[string]string `yaml:"users"`

	// KnownKeys maps specific client keys to fixed storage keys.
	// These override the namespace fallback. Example: "Betreff" → "oy.subject"
	KnownKeys map[string]string `yaml:"known_keys"`

	// InfoKeys maps "info:"-prefixed client keys to info.* storage keys.
	InfoKeys map[string]string `yaml:"info_keys"`

	// SharedPrefixes are storage key prefixes shared with Taki enrichment
	// (doc., sender., amounts., ...). Client keys starting with one of them
	// are stored lowercased without a namespace prefix: "doc.reference"
	// stays "doc.reference" instead of becoming "ctrl.doc.reference".
	// An entry without a trailing dot matches the exact key ("source").
	SharedPrefixes []string `yaml:"shared_prefixes"`
}

// NamespaceConfig defines a client namespace.
type NamespaceConfig struct {
	// Prefix is prepended to all keys: "ctrl" → "ctrl.beleg_nr"
	Prefix string `yaml:"prefix"`

	// StorageCase controls how keys are stored: "lower" (default) or "preserve"
	StorageCase string `yaml:"storage_case"`

	// ClientCase controls how keys are returned to clients: "upper" (default) or "preserve"
	ClientCase string `yaml:"client_case"`

	// Source is written as "source" on every document stored by a user of
	// this namespace, e.g. "saskia". Empty: no source key.
	Source string `yaml:"source"`

	// Defaults are values for storage keys a document does not carry, used
	// when OpenYard answers this namespace (search and DocIndex), e.g.
	// booking.status: U — documents from the mail room count as unprocessed
	// until the client sets a status. Not written to the document.
	Defaults map[string]string `yaml:"defaults"`

	// Formats converts dates and amounts between storage and client format
	// (see FormatConfig), e.g. Saskia: date "02.01.2006", decimal ",".
	Formats FormatConfig `yaml:"formats"`

	// Folders maps a document type (ConfigName of ImportDocumentDynamic) to a
	// folder template, see Config.FolderPath:
	//   IfrAnordnung: Anordnungen/{HH_JAHR}/{KZ_DKS}/{AO_NR|block:1000}
	// Only used when the client does not pass FolderId/SpaceId itself.
	Folders map[string]string `yaml:"folders"`

	// FolderDefault is the template for document types without an entry in
	// Folders, e.g. Ablage/{doc.type}/{now|date:2006}/{now|date:01}. Empty:
	// such documents land in the space root.
	FolderDefault string `yaml:"folder_default"`

	// UniqueKeys names, per document type, the client metadata keys that
	// identify a document, e.g. IfrAnordnung: [Mand, HH_JAHR, AO_NR]. An import
	// whose values match an existing document of that type in the target
	// folder replaces its content (new version) instead of filing a second copy.
	UniqueKeys map[string][]string `yaml:"unique_keys"`

	// KeySeparator replaces "_" in client keys when building storage keys, so
	// that each part becomes a level of its own: "." turns PROD_G_BEZ into
	// ctrl.prod.g.bez instead of ctrl.prod_g_bez. Empty: "_" is kept. Changing
	// it on a namespace with stored documents needs a re-keying of their
	// metadata; the reverse mapping (ToClientKey) turns the separator back
	// into "_".
	KeySeparator string `yaml:"key_separator"`

	// ReferencePlan is the file reference syntax used when the lookup creates
	// numbered folders, one pattern per level ({n} or {n:W} after a
	// separator). Empty: DefaultReferencePlan, the documented syntax
	// 11 / 11.12 / 11.12.01 / 11.12.01.03 / -01 (Akte) / /1 (Vorgang) / #1 (Register).
	ReferencePlan ReferencePlan `yaml:"reference_plan"`

	// Lookups maps a document type to a file plan lookup (see LookupConfig):
	// the folder is found by a metadata value instead of being built from a
	// path template. Tried before Folders; falls back to Folders when the
	// document carries no code.
	Lookups map[string]LookupConfig `yaml:"lookups"`
}

// LookupConfig files a document type below a folder that is found by a
// metadata value, e.g. the file plan node carrying the product number of an
// Anordnung:
//
//	IfrAnordnung:
//	  keys: [PROD, PROD_G]            # DocIndex keys, first non-empty value is the code
//	  names: [PROD_BEZ, PROD_G_BEZ]   # designation belonging to each key (for create)
//	  match: oy.fileReference         # folder metadata the code is compared with
//	  spaces: [Aktenplan]             # spaces searched; empty: the import's target space
//	  create: "{lookup.code} {lookup.name}"  # name of the folder created when none matches
//	  subpath: "{doc.type}/{HH_JAHR}"        # levels below the matched folder
//	  numbered_levels: 1              # created subpath levels that get <parent>-NN as match value
type LookupConfig struct {
	// Keys are the client metadata keys tried in order; the first non-empty
	// value is the code looked up (Saskia: PROD for income, PROD_G for the
	// counter booking side of expenses).
	Keys []string `yaml:"keys"`
	// Names are the client keys with the designation belonging to Keys
	// (same position), available as {lookup.name} in Create.
	Names []string `yaml:"names"`
	// Match is the storage metadata key of the folder carrying the code.
	// Default oy.fileReference (file plan Kennung).
	Match string `yaml:"match"`
	// Spaces names the storage spaces searched for the folder, in order,
	// e.g. [Innere Verwaltung, Sicherheit und Ordnung]. Empty or "*": every
	// project space the user sees.
	Spaces []string `yaml:"spaces"`
	// Create is the name template for a folder that is created when no
	// folder carries the code; placeholders as in Folders plus {lookup.code}
	// and {lookup.name}. The folder is created below the nearest existing
	// ancestor by code prefix (11.16.13.01 → 11.16.13 → 11.16 → 11), else
	// below the space root, and gets the code as Match metadata. Empty: no
	// creation, the document falls back to Folders.
	Create string `yaml:"create"`
	// Subpath is the path template below the matched folder, e.g.
	// {doc.type}/{HH_JAHR}. Empty: the document lands in the matched folder.
	Subpath string `yaml:"subpath"`
	// NumberedLevels is the number of Subpath levels (from the top) that,
	// when created, continue the file plan numbering by the namespace's
	// ReferencePlan (below an Aktenschrank 11.16.13.01: Akte 11.16.13.01-01,
	// then Vorgang 11.16.13.01-01/1), each with the lowest free number among
	// its siblings. 0: none.
	NumberedLevels int `yaml:"numbered_levels"`
}

// MatchKey returns the folder metadata key compared with the code.
func (l LookupConfig) MatchKey() string {
	if l.Match == "" {
		return "oy.fileReference"
	}
	return l.Match
}

// Lookup returns the file plan lookup of a document type for the user's
// namespace, nil if none is configured. The type is matched case-insensitively
// like the other per-type maps.
func (c *Config) Lookup(login, docType string) *LookupConfig {
	ns := c.NamespaceForUser(login)
	if docType == "" || len(ns.Lookups) == 0 {
		return nil
	}
	if lookup, ok := ns.Lookups[docType]; ok {
		return &lookup
	}
	for configured, lookup := range ns.Lookups {
		if strings.EqualFold(configured, docType) {
			return &lookup
		}
	}
	return nil
}

// UniqueKeys returns the identifying client keys of a document type for the
// user's namespace, nil if none are configured.
func (c *Config) UniqueKeys(login, docType string) []string {
	ns := c.NamespaceForUser(login)
	if docType == "" || len(ns.UniqueKeys) == 0 {
		return nil
	}
	if keys, ok := ns.UniqueKeys[docType]; ok {
		return keys
	}
	for configured, keys := range ns.UniqueKeys {
		if strings.EqualFold(configured, docType) {
			return keys
		}
	}
	return nil
}

// DefaultConfig returns the built-in configuration.
// Used when no config file is provided.
func DefaultConfig() *Config {
	return &Config{
		Namespaces: map[string]NamespaceConfig{
			"default": {
				Prefix:      "ctrl",
				StorageCase: "lower",
				ClientCase:  "upper",
			},
		},
		KnownKeys:      defaultKnownKeys(),
		InfoKeys:       defaultInfoKeys(),
		SharedPrefixes: defaultSharedPrefixes(),
	}
}

// LoadConfig reads metadata mapping configuration from a YAML file.
// Falls back to DefaultConfig if the file does not exist.
func LoadConfig(path string) *Config {
	if path == "" {
		path = os.Getenv("OPENYARD_METADATA_CONFIG")
	}
	if path == "" {
		return DefaultConfig()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultConfig()
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return DefaultConfig()
	}
	return cfg
}

// NamespaceForUser returns the namespace config for a given login name.
// Falls back to "default" namespace if the user is not mapped.
func (c *Config) NamespaceForUser(login string) NamespaceConfig {
	if login != "" && c.Users != nil {
		if nsName, ok := c.Users[login]; ok {
			if ns, ok := c.Namespaces[nsName]; ok {
				return ns
			}
		}
	}
	if ns, ok := c.Namespaces["default"]; ok {
		return ns
	}
	return NamespaceConfig{Prefix: "ctrl", StorageCase: "lower", ClientCase: "upper"}
}

// ToStorageKey converts a client metadata key to a storage key.
// The login parameter determines which namespace is used.
//
//	ToStorageKey("saskia", "BELEG_NR")    → "ctrl.beleg_nr"
//	ToStorageKey("", "BELEG_NR")          → "ctrl.beleg_nr"  (default)
//	ToStorageKey("", "info:Aktenzeichen") → "info.fileReference"
//	ToStorageKey("", "Betreff")           → "oy.subject"  (known key)
func (c *Config) ToStorageKey(login, clientKey string) string {
	// info: prefix → info namespace (user-independent)
	if strings.HasPrefix(clientKey, "info:") {
		topic := strings.TrimPrefix(clientKey, "info:")
		if mapped, ok := c.InfoKeys[topic]; ok {
			return mapped
		}
		sanitized := sanitizeInfoKey(topic)
		return "info." + sanitized
	}

	// Known keys → fixed mapping (user-independent)
	if mapped, ok := c.KnownKeys[clientKey]; ok {
		return mapped
	}

	// Shared keys (Taki enrichment) → lowercased, no namespace prefix
	if c.IsSharedKey(clientKey) {
		return strings.ToLower(clientKey)
	}

	// User-specific namespace: prefix + case transformation + separator
	ns := c.NamespaceForUser(login)
	key := clientKey
	if ns.StorageCase == "lower" {
		key = strings.ToLower(key)
	}
	if ns.KeySeparator != "" {
		key = strings.ReplaceAll(key, "_", ns.KeySeparator)
	}
	return ns.Prefix + "." + key
}

// ToClientKey converts a storage key back to a client key.
// Checks all configured namespaces, not just the user's.
//
//	"ctrl.beleg_nr"      → "BELEG_NR"
//	"hr.mitarbeiter_nr"  → "MITARBEITER_NR"
//	"oy.subject"         → "oy.subject"  (known keys pass through)
//	"doc.type"           → "doc.type"    (Taki fields pass through)
func (c *Config) ToClientKey(storageKey string) string {
	// Namespaces may share a prefix (default and a client namespace both use
	// "ctrl"); the separator of any of them applies, since only a namespace
	// with a separator produces keys containing it.
	matched := false
	separator, clientCase, key := "", "", ""
	for _, ns := range c.Namespaces {
		prefix := ns.Prefix + "."
		if !strings.HasPrefix(storageKey, prefix) {
			continue
		}
		if !matched {
			matched = true
			key = strings.TrimPrefix(storageKey, prefix)
			clientCase = ns.ClientCase
		}
		if ns.KeySeparator != "" {
			separator = ns.KeySeparator
		}
	}
	if !matched {
		// Non-namespace keys pass through unchanged
		return storageKey
	}
	if separator != "" {
		key = strings.ReplaceAll(key, separator, "_")
	}
	if clientCase == "upper" {
		key = strings.ToUpper(key)
	}
	return key
}

// IsSharedKey reports whether a client key belongs to the shared key space
// (case-insensitive), e.g. "doc.type", "Sender.Company", "source".
func (c *Config) IsSharedKey(clientKey string) bool {
	key := strings.ToLower(clientKey)
	for _, prefix := range c.SharedPrefixes {
		if strings.HasSuffix(prefix, ".") {
			if strings.HasPrefix(key, prefix) {
				return true
			}
		} else if key == prefix {
			return true
		}
	}
	return false
}

// DocumentMetadata returns the storage keys OpenYard adds to a document
// stored by login with the client document type docType:
// source and doc.type.
func (c *Config) DocumentMetadata(login, docType string) map[string]string {
	ns := c.NamespaceForUser(login)
	documentMeta := make(map[string]string)
	if ns.Source != "" {
		documentMeta["source"] = ns.Source
	}
	// The client configures the target value itself (Saskia: KS_DMS_META
	// DOKTYP row, FELD_DMS), OpenYard stores it unchanged.
	if docType != "" {
		documentMeta["doc.type"] = docType
	}
	return documentMeta
}

// WithDefaults returns the metadata of a document as seen by login: the
// namespace defaults fill keys that are missing or empty. The input map is
// not modified.
func (c *Config) WithDefaults(login string, storedMeta map[string]string) map[string]string {
	defaults := c.NamespaceForUser(login).Defaults
	if len(defaults) == 0 {
		return storedMeta
	}
	merged := make(map[string]string, len(storedMeta)+len(defaults))
	for key, value := range storedMeta {
		merged[key] = value
	}
	for key, value := range defaults {
		if merged[key] == "" {
			merged[key] = value
		}
	}
	return merged
}

// IsNamespacedKey returns true if the storage key belongs to a client namespace.
func (c *Config) IsNamespacedKey(storageKey string) bool {
	for _, ns := range c.Namespaces {
		if strings.HasPrefix(storageKey, ns.Prefix+".") {
			return true
		}
	}
	return false
}

func sanitizeInfoKey(topic string) string {
	var b strings.Builder
	for _, r := range topic {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}
