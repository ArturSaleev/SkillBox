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
	"strings"
	"syscall"
	"time"

	"github.com/aibox/skillbox/internal/application"
	"github.com/aibox/skillbox/internal/codereview"
	"github.com/aibox/skillbox/internal/config"
	"github.com/aibox/skillbox/internal/dashboard"
	"github.com/aibox/skillbox/internal/ports"
	skillexporter "github.com/aibox/skillbox/internal/skills/exporter"
	skillimporter "github.com/aibox/skillbox/internal/skills/importer"
	skillpackage "github.com/aibox/skillbox/internal/skills/package"
	skillstore "github.com/aibox/skillbox/internal/skills/store"
	"github.com/aibox/skillbox/internal/storage/mysql"
	"github.com/aibox/skillbox/internal/storage/postgres"
	"github.com/aibox/skillbox/internal/storage/sqlite"
	"github.com/aibox/skillbox/internal/transport/mcp"
	"github.com/go-chi/chi/v5"
)

func main() {
	configPath := flag.String("config", "./configs/skillbox.yaml", "YAML configuration path")
	migrateLegacy := flag.Bool("migrate-legacy-skills", false, "migrate DB-only Skills to filesystem packages and exit")
	importDirectory := flag.String("import-skill-directory", "", "import one Skill package from a local directory and exit")
	importZIP := flag.String("import-skill-zip", "", "import one Skill package from a ZIP archive and exit")
	importGit := flag.String("import-skill-git", "", "import one Skill package from a Git repository and exit")
	importGitRevision := flag.String("import-git-revision", "HEAD", "Git revision used with -import-skill-git")
	exportPackage := flag.String("export-skill-package", "", "package path inside skills.directory to export")
	exportDirectory := flag.String("export-skill-directory", "", "export the selected Skill package to a portable directory and exit")
	exportZIP := flag.String("export-skill-zip", "", "export the selected Skill package to a portable ZIP and exit")
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(1)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	if err = skillpackage.EnsureRoot(cfg.Skills.Directory); err != nil {
		logger.Error("initialize skills directory", "directory", cfg.Skills.Directory, "error", err)
		os.Exit(1)
	}
	exportOutputs := 0
	for _, value := range []string{*exportDirectory, *exportZIP} {
		if strings.TrimSpace(value) != "" {
			exportOutputs++
		}
	}
	if strings.TrimSpace(*exportPackage) != "" || exportOutputs > 0 {
		if *migrateLegacy || strings.TrimSpace(*importDirectory) != "" || strings.TrimSpace(*importZIP) != "" || strings.TrimSpace(*importGit) != "" {
			logger.Error("export cannot be combined with migration or import")
			os.Exit(2)
		}
		if strings.TrimSpace(*exportPackage) == "" || exportOutputs != 1 {
			logger.Error("export requires -export-skill-package and exactly one export destination")
			os.Exit(2)
		}
		if err = skillpackage.ValidateRelativePath(*exportPackage); err != nil {
			logger.Error("invalid export package path", "error", err)
			os.Exit(2)
		}
		packageRoot := filepath.Join(cfg.Skills.Directory, filepath.FromSlash(*exportPackage))
		var exported skillexporter.Result
		if strings.TrimSpace(*exportDirectory) != "" {
			exported, err = skillexporter.Directory(packageRoot, *exportDirectory)
		} else {
			exported, err = skillexporter.ZIP(packageRoot, *exportZIP)
		}
		if err != nil {
			logger.Error("export Skill package", "package_path", *exportPackage, "error", err)
			os.Exit(1)
		}
		logger.Info("Skill package exported", "package_path", *exportPackage, "destination", exported.Destination, "hash", exported.Hash, "files", exported.Files)
		return
	}
	indexStore, err := openStore(ctx, cfg)
	if err != nil {
		logger.Error("open database", "driver", cfg.Database.Driver, "error", err)
		os.Exit(1)
	}
	defer indexStore.Close()
	store := skillstore.New(indexStore, cfg.Skills.Directory)
	importSources := 0
	for _, value := range []string{*importDirectory, *importZIP, *importGit} {
		if strings.TrimSpace(value) != "" {
			importSources++
		}
	}
	if importSources > 1 || (*migrateLegacy && importSources > 0) {
		logger.Error("choose exactly one migration or import operation")
		os.Exit(2)
	}
	if *migrateLegacy {
		migration, migrateErr := store.MigrateLegacySkills(ctx)
		if migrateErr != nil {
			logger.Error("migrate legacy Skills", "error", migrateErr)
			os.Exit(1)
		}
		logger.Info("legacy Skill migration completed", "discovered", migration.Discovered, "migrated", migration.Migrated, "unchanged", migration.Unchanged)
		return
	}
	if importSources == 1 {
		service := skillimporter.New(cfg.Skills.Directory)
		var imported skillimporter.Result
		switch {
		case strings.TrimSpace(*importDirectory) != "":
			imported, err = service.LocalDirectory(*importDirectory)
		case strings.TrimSpace(*importZIP) != "":
			imported, err = service.ZIP(*importZIP)
		default:
			imported, err = service.Git(*importGit, *importGitRevision)
		}
		if err != nil {
			logger.Error("import Skill package", "error", err)
			os.Exit(1)
		}
		reindex, reindexErr := store.Reindex(ctx)
		if reindexErr != nil {
			logger.Error("index imported Skill package", "package_path", imported.PackagePath, "error", reindexErr)
			os.Exit(1)
		}
		logger.Info("Skill package imported", "package_path", imported.PackagePath, "hash", imported.Hash, "source_type", imported.Source.Type, "source_url", imported.Source.URL, "source_revision", imported.Source.Revision, "unchanged", imported.Unchanged, "indexed_created", reindex.Created, "indexed_updated", reindex.Updated)
		return
	}
	reindex, err := store.Reindex(ctx)
	if err != nil {
		logger.Error("reindex Skill packages", "directory", cfg.Skills.Directory, "error", err)
		os.Exit(1)
	}
	logger.Info("Skill package index synchronized", "discovered", reindex.Discovered, "created", reindex.Created, "updated", reindex.Updated, "unchanged", reindex.Unchanged)
	workspace, err := store.EnsureWorkspace(ctx, "local", "Local Workspace")
	if err != nil {
		logger.Error("initialize local workspace", "error", err)
		os.Exit(1)
	}

	handler := mcp.New(application.New(store), mcp.NewLocalResolver(store, workspace.ID))
	var reviewer codereview.CodeReviewer
	if strings.TrimSpace(cfg.CodeReview.Provider) != "" {
		apiKey := ""
		if envName := strings.TrimSpace(cfg.CodeReview.APIKeyEnv); envName != "" {
			apiKey = os.Getenv(envName)
			if apiKey == "" {
				logger.Error("configure code-review provider", "error", fmt.Sprintf("environment variable %s is empty", envName))
				os.Exit(1)
			}
		}
		reviewer, err = codereview.DefaultRegistry().Create(codereview.Config{
			Provider: cfg.CodeReview.Provider,
			Endpoint: cfg.CodeReview.Endpoint,
			Model:    cfg.CodeReview.Model,
			APIKey:   apiKey,
		})
		if err != nil {
			logger.Error("configure code-review provider", "error", err)
			os.Exit(1)
		}
	}
	router := chi.NewRouter()
	router.Handle("/mcp/{project}", handler)
	router.Handle("/mcp/{project}/teacher", handler)
	router.Mount("/admin/api", dashboard.AdminHandlerWithServices(store, cfg.Skills.Directory, workspace.ID, store.Reindex, reviewer))
	router.Handle("/", dashboard.Handler())
	router.Handle("/*", dashboard.Handler())
	srv := &http.Server{Addr: cfg.Server.Address, Handler: router, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second}
	go func() {
		logger.Info("SkillBox started", "address", cfg.Server.Address, "database_driver", cfg.Database.Driver, "skills_directory", cfg.Skills.Directory)
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
