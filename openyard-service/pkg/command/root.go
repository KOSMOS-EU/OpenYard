package command

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/kosmos-eu/openyard/pkg/auth"
	"github.com/kosmos-eu/openyard/pkg/config"
	"github.com/kosmos-eu/openyard/pkg/cs3client"
	httpservice "github.com/kosmos-eu/openyard/pkg/http"
	"github.com/kosmos-eu/openyard/pkg/migration"
)

func Execute(cfg *config.Config) error {
	if len(os.Args) < 2 || os.Args[1] != "server" {
		fmt.Println("Usage: openyard server")
		return nil
	}
	return runServer(cfg)
}

func runServer(cfg *config.Config) error {
	// Logging
	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	log.Info().Str("addr", cfg.HTTP.Addr).Str("gateway", cfg.Reva.GatewayAddr).Msg("starting openyard")

	// Migration DB (ID mapping WinYard → OpenYard)
	migration.Init()

	// CS3 Gateway Client
	gw, err := cs3client.New(cfg.Reva.GatewayAddr)
	if err != nil {
		return fmt.Errorf("cs3 gateway: %w", err)
	}
	defer gw.Close()

	// Auth Bridge
	sessions := auth.NewSessionCache(cfg.Session.TTL)

	// HTTP Service
	svc := httpservice.NewService(gw, sessions, cfg)

	srv := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      svc,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// HTTPS server (optional, parallel to HTTP)
	var srvTLS *http.Server
	if cfg.TLS.Addr != "" && cfg.TLS.Cert != "" && cfg.TLS.Key != "" {
		srvTLS = &http.Server{
			Addr:         cfg.TLS.Addr,
			Handler:      svc,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  120 * time.Second,
		}
		log.Info().Str("addr", cfg.TLS.Addr).Msg("starting HTTPS")
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)
	go func() { errCh <- srv.ListenAndServe() }()
	if srvTLS != nil {
		go func() { errCh <- srvTLS.ListenAndServeTLS(cfg.TLS.Cert, cfg.TLS.Key) }()
	}

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info().Msg("shutting down")
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if srvTLS != nil {
			srvTLS.Shutdown(shutCtx)
		}
		return srv.Shutdown(shutCtx)
	}
}
