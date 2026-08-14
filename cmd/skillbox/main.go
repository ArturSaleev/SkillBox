package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/aibox/skillbox/internal/application"
	"github.com/aibox/skillbox/internal/auth"
	"github.com/aibox/skillbox/internal/config"
	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/metrics"
	"github.com/aibox/skillbox/internal/observability"
	"github.com/aibox/skillbox/internal/ports"
	"github.com/aibox/skillbox/internal/seed"
	"github.com/aibox/skillbox/internal/storage/mysql"
	"github.com/aibox/skillbox/internal/storage/postgres"
	"github.com/aibox/skillbox/internal/storage/sqlite"
	"github.com/aibox/skillbox/internal/transport/httpapi"
	"github.com/aibox/skillbox/internal/transport/mcp"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	configPath := flag.String("config", "./configs/skillbox.yaml", "YAML configuration path")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(1)
	}
	logger := newLogger(cfg)
	ctx := context.Background()
	store, err := openStore(ctx, cfg)
	if err != nil {
		logger.Error("open database", "driver", cfg.Database.Driver, "error", err)
		os.Exit(1)
	}
	defer store.Close()
	if cfg.SeedDemo {
		if err = seed.Demo(ctx, store); err != nil {
			logger.Error("seed demo skills", "error", err)
			os.Exit(1)
		}
	}
	reg := prometheus.NewRegistry()
	met := metrics.New(reg)
	app := application.New(store)
	if err = app.EnsureBuiltInProfiles(ctx); err != nil {
		logger.Error("initialize MCP profiles", "error", err)
		os.Exit(1)
	}
	for _, configured := range cfg.MCPProfiles {
		enabled := true
		if configured.Enabled != nil {
			enabled = *configured.Enabled
		}
		profile := domain.MCPProfile{Slug: configured.Slug, Name: configured.Name, Description: configured.Description, Permissions: configured.Permissions, Tools: configured.Tools, Enabled: enabled}
		if err = application.ValidateProfile(&profile); err != nil {
			logger.Error("invalid configured MCP profile", "profile", configured.Slug, "error", err)
			os.Exit(1)
		}
		if existing, e := store.GetMCPProfileBySlug(ctx, configured.Slug); e == nil {
			profile.BuiltIn = existing.BuiltIn
		}
		if err = store.UpsertMCPProfile(ctx, &profile); err != nil {
			logger.Error("configure MCP profile", "profile", configured.Slug, "error", err)
			os.Exit(1)
		}
	}
	guard := auth.New(cfg.Auth.Mode, cfg.Auth.APIKeys)
	router := chi.NewRouter()
	router.Use(observability.HTTP(logger, met))
	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) { jsonOK(w, map[string]string{"status": "ok"}) })
	router.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if err := store.Ping(r.Context()); err != nil {
			met.DBErrors.Inc()
			http.Error(w, "not ready", 503)
			return
		}
		jsonOK(w, map[string]string{"status": "ready"})
	})
	router.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	router.Mount("/api", guard.Wrap(http.StripPrefix("/api", httpapi.New(app, met).Routes())))
	mcpServer := mcp.New(app, met, logger, mcp.NewConnectionResolver(store))
	router.Handle("/mcp", mcpServer)
	router.Handle("/mcp/{connection}", mcpServer)
	srv := &http.Server{Addr: cfg.Server.Address, Handler: router, ReadTimeout: cfg.Server.ReadTimeout, WriteTimeout: cfg.Server.WriteTimeout}
	go func() {
		logger.Info("SkillBox started", "address", cfg.Server.Address, "database_driver", cfg.Database.Driver, "auth_mode", cfg.Auth.Mode)
		if e := srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			logger.Error("server failed", "error", e)
			os.Exit(1)
		}
	}()
	stop, release := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer release()
	<-stop.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err = srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("SkillBox stopped")
}
func openStore(ctx context.Context, c config.Config) (ports.Storage, error) {
	switch c.Database.Driver {
	case "sqlite":
		if err := os.MkdirAll(filepath.Dir(c.Database.Path), 0750); err != nil {
			return nil, err
		}
		return sqlite.Open(ctx, c.Database.Path, c.Database.Migrate)
	case "mysql":
		return mysql.Open(ctx, c.Database.DSN, c.Database.Migrate)
	case "postgres":
		return postgres.Open(ctx, c.Database.DSN, c.Database.Migrate)
	default:
		return nil, fmt.Errorf("unsupported driver %q", c.Database.Driver)
	}
}
func newLogger(c config.Config) *slog.Logger {
	level := slog.LevelInfo
	if c.Observability.Level == "debug" {
		level = slog.LevelDebug
	} else if c.Observability.Level == "warn" {
		level = slog.LevelWarn
	} else if c.Observability.Level == "error" {
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	if c.Observability.Format == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
