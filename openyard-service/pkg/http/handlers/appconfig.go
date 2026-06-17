package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// AppConfig storage — flat JSON files in a directory.
// Each config is stored as {ConfigId}.json.
var (
	appConfigDir  = envOrDefault("OPENYARD_APPCONFIG_DIR", "/etc/openyard/appconfigs")
	appConfigLock sync.RWMutex
)

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// POST /api/advancedConfig/GetAppConfigs
func (h *Handlers) GetAppConfigs(w http.ResponseWriter, r *http.Request) {
	var filter map[string]interface{}
	json.NewDecoder(r.Body).Decode(&filter)

	filterDetailId, _ := filter["DetailId"].(string)

	appConfigLock.RLock()
	defer appConfigLock.RUnlock()

	configs := loadAllConfigs()

	// Filter by DetailId if specified
	if filterDetailId != "" {
		var filtered []map[string]interface{}
		for _, cfg := range configs {
			if cfg["DetailId"] == filterDetailId {
				filtered = append(filtered, cfg)
			}
		}
		configs = filtered
	}

	writeJSON(w, 200, map[string]interface{}{
		"AppConfigsList": configs,
		"TaskLog":        taskLogOK(),
	})
}

// POST /api/advancedConfig/SetAppConfig
func (h *Handlers) SetAppConfig(w http.ResponseWriter, r *http.Request) {
	var cfg map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeError(w, 400, "INVALID_REQUEST", "Invalid JSON body")
		return
	}

	// Accept both flat {ConfigId:...} and wrapped {meta:{ConfigId:...}, content:[...]}
	configId, _ := cfg["ConfigId"].(string)
	if configId == "" {
		if meta, ok := cfg["meta"].(map[string]interface{}); ok {
			configId, _ = meta["ConfigId"].(string)
		}
	}
	if configId == "" {
		writeError(w, 400, "INVALID_REQUEST", "ConfigId required")
		return
	}

	appConfigLock.Lock()
	defer appConfigLock.Unlock()

	os.MkdirAll(appConfigDir, 0755)
	fpath := filepath.Join(appConfigDir, sanitizeFilename(configId)+".json")
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(fpath, data, 0644); err != nil {
		log.Error().Err(err).Str("path", fpath).Msg("write appconfig failed")
		writeError(w, 500, "INTERNAL_ERROR", "Cannot save config")
		return
	}

	detailName := ""
	if d, ok := cfg["DetailName"].(string); ok {
		detailName = d
	} else if meta, ok := cfg["meta"].(map[string]interface{}); ok {
		detailName, _ = meta["DetailName"].(string)
	}
	log.Info().Str("id", configId).Str("detail", detailName).Msg("appconfig saved")
	writeJSON(w, 200, map[string]interface{}{
		"TaskLog": taskLogOK(),
	})
}

// POST /api/advancedConfig/GetAppConfigDetailsAsJson
func (h *Handlers) GetAppConfigDetailsAsJson(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	json.NewDecoder(r.Body).Decode(&req)
	configId, _ := req["ConfigId"].(string)

	if configId == "" {
		writeError(w, 400, "INVALID_REQUEST", "ConfigId required")
		return
	}

	appConfigLock.RLock()
	defer appConfigLock.RUnlock()

	fpath := filepath.Join(appConfigDir, sanitizeFilename(configId)+".json")
	data, err := os.ReadFile(fpath)
	if err != nil {
		writeJSON(w, 200, []interface{}{})
		return
	}

	// The stored file contains the meta + content from legacy DMS
	var stored map[string]interface{}
	if err := json.Unmarshal(data, &stored); err != nil {
		writeJSON(w, 200, []interface{}{})
		return
	}

	// If stored with "content" wrapper (from our migration script)
	if content, ok := stored["content"]; ok {
		writeJSON(w, 200, content)
		return
	}

	// Otherwise return as-is
	writeJSON(w, 200, stored)
}

// POST /api/advancedConfig/GetAppConfigDetailsAsFile
func (h *Handlers) GetAppConfigDetailsAsFile(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	json.NewDecoder(r.Body).Decode(&req)
	configId, _ := req["ConfigId"].(string)

	if configId == "" {
		writeError(w, 400, "INVALID_REQUEST", "ConfigId required")
		return
	}

	appConfigLock.RLock()
	defer appConfigLock.RUnlock()

	fpath := filepath.Join(appConfigDir, sanitizeFilename(configId)+".json")
	data, err := os.ReadFile(fpath)
	if err != nil {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(200)
		w.Write([]byte("[]"))
		return
	}

	var stored map[string]interface{}
	if err := json.Unmarshal(data, &stored); err == nil {
		if content, ok := stored["content"]; ok {
			out, _ := json.Marshal(content)
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(200)
			w.Write(out)
			return
		}
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(200)
	w.Write(data)
}

// POST /api/advancedConfig/DeleteAppConfigs
func (h *Handlers) DeleteAppConfigs(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	json.NewDecoder(r.Body).Decode(&req)
	configId, _ := req["ConfigId"].(string)

	if configId != "" {
		appConfigLock.Lock()
		defer appConfigLock.Unlock()
		fpath := filepath.Join(appConfigDir, sanitizeFilename(configId)+".json")
		os.Remove(fpath)
	}

	writeJSON(w, 200, map[string]interface{}{"TaskLog": taskLogOK()})
}

// loadAllConfigs reads all config files from the appconfig directory.
func loadAllConfigs() []map[string]interface{} {
	var configs []map[string]interface{}

	entries, err := os.ReadDir(appConfigDir)
	if err != nil {
		return configs
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(appConfigDir, entry.Name()))
		if err != nil {
			continue
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		// If stored with "meta" wrapper, use meta for listing
		if meta, ok := cfg["meta"].(map[string]interface{}); ok {
			configs = append(configs, meta)
		} else {
			configs = append(configs, cfg)
		}
	}

	return configs
}

func sanitizeFilename(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, s)
}
