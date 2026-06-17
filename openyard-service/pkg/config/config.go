package config

import (
	"os"
	"time"
)

type Config struct {
	HTTP struct {
		Addr     string
		BaseURL  string // External URL (for service URIs in responses)
	}
	Reva struct {
		GatewayAddr string
	}
	Session struct {
		TTL time.Duration
	}
	ServiceAccount struct {
		User string
		Pass string
	}
	DB struct {
		DSN string
	}
	TLS struct {
		Addr string // HTTPS listen address, e.g. "0.0.0.0:9202"
		Cert string // Path to TLS certificate file
		Key  string // Path to TLS private key file
	}
	Upload struct {
		Method  string // "webdav" or "reva"
		BaseURL string // WebDAV base URL (internal OC), e.g. "http://opencloud:9200"
	}
	Log struct {
		Level string
	}
}

func Load() *Config {
	cfg := &Config{}
	cfg.HTTP.Addr = envOr("OPENYARD_HTTP_ADDR", "0.0.0.0:9201")
	cfg.HTTP.BaseURL = envOr("OPENYARD_BASE_URL", "http://localhost:9201")
	cfg.Reva.GatewayAddr = envOr("OPENYARD_REVA_GATEWAY", "127.0.0.1:9142")
	cfg.Session.TTL = envDuration("OPENYARD_SESSION_TTL", 8*time.Hour)
	cfg.ServiceAccount.User = os.Getenv("OPENYARD_SERVICE_USER")
	cfg.ServiceAccount.Pass = os.Getenv("OPENYARD_SERVICE_PASS")
	cfg.DB.DSN = os.Getenv("OPENYARD_DB_DSN")
	cfg.TLS.Addr = os.Getenv("OPENYARD_HTTPS_ADDR")
	cfg.TLS.Cert = os.Getenv("OPENYARD_TLS_CERT")
	cfg.TLS.Key = os.Getenv("OPENYARD_TLS_KEY")
	cfg.Upload.Method = envOr("OPENYARD_UPLOAD_METHOD", "reva")
	cfg.Upload.BaseURL = envOr("OPENYARD_UPLOAD_URL", "http://opencloud:9200")
	cfg.Log.Level = envOr("OPENYARD_LOG_LEVEL", "info")
	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
