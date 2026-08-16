package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/aibox/skillbox/internal/application"
	"github.com/aibox/skillbox/internal/config"
	"github.com/aibox/skillbox/internal/ports"
	"github.com/aibox/skillbox/internal/storage/mysql"
	"github.com/aibox/skillbox/internal/storage/postgres"
	"github.com/aibox/skillbox/internal/storage/sqlite"
	"github.com/aibox/skillbox/internal/transport/mcp"
	"github.com/go-chi/chi/v5"
)

func main() {
	configPath := flag.String("config", "./configs/skillbox.yaml", "YAML configuration path")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	store, err := openStore(ctx, cfg)
	if err != nil {
		logger.Error("open database", "driver", cfg.Database.Driver, "error", err)
		os.Exit(1)
	}
	defer store.Close()
	workspace, err := store.EnsureWorkspace(ctx, "local", "Local Workspace")
	if err != nil {
		logger.Error("initialize local workspace", "error", err)
		os.Exit(1)
	}

	handler := mcp.New(application.New(store), mcp.NewLocalResolver(store, workspace.ID))
	router := chi.NewRouter()
	router.Handle("/mcp/{project}", handler)
	router.Handle("/mcp/{project}/teacher", handler)
	srv := &http.Server{Addr: cfg.Server.Address, Handler: router, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second}
	go func() {
		logger.Info("SkillBox started", "address", cfg.Server.Address, "database_driver", cfg.Database.Driver)
		if serveErr := srv.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			logger.Error("server failed", "error", serveErr)
			os.Exit(1)
		}
	}()
	stop, release := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer release()
	<-stop.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
		return sqlite.Open(ctx, c.Database.Path, true)
	case "mysql":
		return mysql.Open(ctx, c.Database.DSN, true)
	case "postgres":
		return postgres.Open(ctx, c.Database.DSN, true)
	default:
		return nil, fmt.Errorf("unsupported driver %q", c.Database.Driver)
	}
}
