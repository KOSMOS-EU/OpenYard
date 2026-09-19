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

	// KnownKeys maps specific client keys to fixed storage keys.
	// These override the namespace fallback. Example: "Betreff" → "oy.subject"
	KnownKeys map[string]string `yaml:"known_keys"`

	// InfoKeys maps "info:"-prefixed client keys to info.* storage keys.
	InfoKeys map[string]string `yaml:"info_keys"`
}

// NamespaceConfig defines a client namespace.
type NamespaceConfig struct {
	// Prefix is prepended to all keys: "saskia" → "saskia.beleg_nr"
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

// defaultNamespace returns the "default" namespace config.
func (c *Config) defaultNamespace() NamespaceConfig {
	if ns, ok := c.Namespaces["default"]; ok {
		return ns
	}
	return NamespaceConfig{Prefix: "ctrl", StorageCase: "lower", ClientCase: "upper"}
}

// ToStorageKey converts a client metadata key to a storage key.
//
//	"BELEG_NR"         → "saskia.beleg_nr"
//	"info:Aktenzeichen" → "info.fileReference"
//	"Betreff"          → "oy.subject"  (known key)
func (c *Config) ToStorageKey(clientKey string) string {
	// info: prefix → info namespace
	if strings.HasPrefix(clientKey, "info:") {
		topic := strings.TrimPrefix(clientKey, "info:")
		if mapped, ok := c.InfoKeys[topic]; ok {
			return mapped
		}
		sanitized := sanitizeInfoKey(topic)
		return "info." + sanitized
	}

	// Known keys → fixed mapping
	if mapped, ok := c.KnownKeys[clientKey]; ok {
		return mapped
	}

	// Default namespace: prefix + case transformation
	ns := c.defaultNamespace()
	key := clientKey
	if ns.StorageCase == "lower" {
		key = strings.ToLower(key)
	}
	return ns.Prefix + "." + key
}

// ToClientKey converts a storage key back to a client key.
//
//	"saskia.beleg_nr" → "BELEG_NR"
//	"oy.subject"      → "oy.subject"  (known keys not reverse-mapped here)
//	"info.fileReference" → "info.fileReference"
func (c *Config) ToClientKey(storageKey string) string {
	ns := c.defaultNamespace()
	prefix := ns.Prefix + "."

	if strings.HasPrefix(storageKey, prefix) {
		key := strings.TrimPrefix(storageKey, prefix)
		if ns.ClientCase == "upper" {
			key = strings.ToUpper(key)
		}
		return key
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
