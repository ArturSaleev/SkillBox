package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMinimalDefaults(t *testing.T) {
	cfg := Default()
	if cfg.Server.Address != ":8081" || cfg.Database.Driver != "sqlite" || cfg.Database.Path != "./data/skillbox.db" {
		t.Fatalf("defaults=%#v", cfg)
	}
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
