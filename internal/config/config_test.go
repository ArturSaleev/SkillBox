package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMinimalDefaults(t *testing.T) {
	cfg := Default()
	if cfg.Server.Address != ":8081" || cfg.Skills.Directory != "./data/skills" || cfg.Database.Driver != "sqlite" || cfg.Database.Path != "./data/skillbox.db" {
		t.Fatalf("defaults=%#v", cfg)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSkillsDirectoryCanBeConfigured(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skillbox.yaml")
	raw := []byte("server:\n  address: ':8081'\nskills:\n  directory: /srv/skillbox/skills\ndatabase:\n  driver: sqlite\n  path: test.db\n  dsn: ''\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Skills.Directory != "/srv/skillbox/skills" {
		t.Fatalf("skills.directory=%q", cfg.Skills.Directory)
	}
}

func TestEmptySkillsDirectoryIsRejected(t *testing.T) {
	cfg := Default()
	cfg.Skills.Directory = "  "
	if err := cfg.Validate(); err == nil {
		t.Fatal("empty skills.directory was accepted")
	}
}

func TestPartialCodeReviewConfigurationIsRejected(t *testing.T) {
	cfg := Default()
	cfg.CodeReview.Provider = "openai-compatible"
	if err := cfg.Validate(); err == nil {
		t.Fatal("partial code-review configuration was accepted")
	}
	cfg.CodeReview.Endpoint = "http://127.0.0.1:8080/v1"
	cfg.CodeReview.Model = "local-review-model"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyConfigurationFieldsAreRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "skillbox.yaml")
	raw := []byte("server:\n  address: ':8081'\ndatabase:\n  driver: sqlite\n  path: test.db\n  dsn: ''\nauth:\n  mode: disabled\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("legacy auth configuration was accepted")
	}
}
