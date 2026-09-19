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
}

// NamespaceConfig defines a client namespace.
type NamespaceConfig struct {
	// Prefix is prepended to all keys: "ctrl" → "ctrl.beleg_nr"
	Prefix string `yaml:"prefix"`

	// StorageCase controls how keys are stored: "lower" (default) or "preserve"
	StorageCase string `yaml:"storage_case"`

	// ClientCase controls how keys are returned to clients: "upper" (default) or "preserve"
	ClientCase string `yaml:"client_case"`
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
		KnownKeys: defaultKnownKeys(),
		InfoKeys:  defaultInfoKeys(),
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

	// User-specific namespace: prefix + case transformation
	ns := c.NamespaceForUser(login)
	key := clientKey
	if ns.StorageCase == "lower" {
		key = strings.ToLower(key)
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
	for _, ns := range c.Namespaces {
		prefix := ns.Prefix + "."
		if strings.HasPrefix(storageKey, prefix) {
			key := strings.TrimPrefix(storageKey, prefix)
			if ns.ClientCase == "upper" {
				key = strings.ToUpper(key)
			}
			return key
		}
	}
	// Non-namespace keys pass through unchanged
	return storageKey
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
