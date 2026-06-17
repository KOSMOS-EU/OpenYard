// Package migration provides ID mapping between legacy DMS and OpenYard ObjectIDs.
//
// Used for document migration: when importing documents from legacy DMS,
// the old legacy DMS ID is mapped to the new OpenYard ID so that subsequent
// lookups with the old ID still work.
//
// Uses a simple JSON file as backend (no CGO/SQLite dependency).
// Format: {"legacy-guid": {"oy": "openyard-id", "t": "folder", "n": "name"}, ...}
package migration

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/rs/zerolog/log"
)

type entry struct {
	OpenYardID string `json:"oy"`
	Type       string `json:"t"`
	Name       string `json:"n"`
}

var (
	idMap  map[string]entry
	dbPath string
	dirty  bool
	mu     sync.RWMutex
	once   sync.Once
)

// Init loads the migration map from JSON file.
func Init() {
	once.Do(func() {
		dbPath = os.Getenv("OPENYARD_MIGRATION_DB")
		if dbPath == "" {
			dbPath = "/etc/openyard/migration.db"
		}

		data, err := os.ReadFile(dbPath)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Error().Err(err).Msg("migration db read failed")
			} else {
				log.Info().Str("path", dbPath).Msg("migration db not found, ID mapping disabled")
			}
			idMap = make(map[string]entry)
			return
		}

		idMap = make(map[string]entry)
		if err := json.Unmarshal(data, &idMap); err != nil {
			log.Error().Err(err).Msg("migration db parse failed")
			idMap = make(map[string]entry)
			return
		}

		log.Info().Int("entries", len(idMap)).Msg("migration db loaded")
	})
}

// LookupOpenYardID translates a legacy DMS ID to an OpenYard ID.
func LookupOpenYardID(legacyID string) string {
	mu.RLock()
	defer mu.RUnlock()
	if e, ok := idMap[legacyID]; ok {
		return e.OpenYardID
	}
	return ""
}

// LookupLegacyID reverse-translates an OpenYard ID to legacy DMS ID.
func LookupLegacyID(openyardID string) string {
	mu.RLock()
	defer mu.RUnlock()
	for wyID, e := range idMap {
		if e.OpenYardID == openyardID {
			return wyID
		}
	}
	return ""
}

// MapID creates or updates a legacy DMS→OpenYard mapping in memory.
// Call Persist() to write to disk.
func MapID(legacyID, openyardID, objType, name string) {
	mu.Lock()
	defer mu.Unlock()

	idMap[legacyID] = entry{
		OpenYardID: openyardID,
		Type:       objType,
		Name:       name,
	}
	dirty = true
}

// Persist writes the current map to disk (atomic via temp file).
func Persist() error {
	mu.Lock()
	defer mu.Unlock()

	if !dirty {
		return nil
	}

	data, err := json.Marshal(idMap)
	if err != nil {
		return err
	}

	// Write directly (rename fails on bind-mounted files)
	if err := os.WriteFile(dbPath, data, 0644); err != nil {
		return err
	}

	dirty = false
	log.Info().Int("entries", len(idMap)).Int("mapped", mappedCount()).Msg("migration db persisted")
	return nil
}

// IsDirty returns true if there are unsaved changes.
func IsDirty() bool {
	mu.RLock()
	defer mu.RUnlock()
	return dirty
}

func mappedCount() int {
	n := 0
	for _, e := range idMap {
		if e.OpenYardID != "" {
			n++
		}
	}
	return n
}

// IsAvailable returns true if the migration map is loaded.
func IsAvailable() bool {
	mu.RLock()
	defer mu.RUnlock()
	return idMap != nil && len(idMap) > 0
}

// Count returns the number of entries.
func Count() int {
	mu.RLock()
	defer mu.RUnlock()
	return len(idMap)
}

// MappedCount returns the number of entries that have an OpenYard ID.
func MappedCount() int {
	mu.RLock()
	defer mu.RUnlock()
	return mappedCount()
}
