package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	benchmarkconfig "github.com/aibox/skillbox/benchmark/internal/config"
	"github.com/aibox/skillbox/benchmark/internal/httpapi"
	"github.com/aibox/skillbox/benchmark/internal/storage"
	benchmarkweb "github.com/aibox/skillbox/benchmark/internal/web"
	"github.com/go-chi/chi/v5"
)

func main() {
	configPath := flag.String("config", "./benchmark/config.yaml", "SkillBox Bench YAML configuration path")
	flag.Parse()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	manager, err := benchmarkconfig.NewManager(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(1)
	}
	cfg := manager.Current()
	store, err := storage.Open(context.Background(), cfg.Database.Path)
	if err != nil {
		logger.Error("open benchmark database", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	router := chi.NewRouter()
	router.Mount("/api", httpapi.Handler(manager, store))
	router.Handle("/", benchmarkweb.Handler())
	router.Handle("/*", benchmarkweb.Handler())
	server := &http.Server{
		Addr:              cfg.Server.Address,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      180 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		logger.Info("SkillBox Bench started", "address", cfg.Server.Address, "database", cfg.Database.Path, "config", *configPath)
		if serveErr := server.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			logger.Error("server failed", "error", serveErr)
			os.Exit(1)
		}
	}()
	stop, release := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer release()
	<-stop.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("SkillBox Bench stopped")
}
