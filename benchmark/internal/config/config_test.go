package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerPreservesMaskedSecretsAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	manager, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	next := manager.Current()
	next.Providers = []Provider{{ID: "openrouter", Name: "OpenRouter", Type: "openai_compatible", Enabled: true, BaseURL: "https://openrouter.ai/api/v1", APIKey: "secret"}}
	if _, err := manager.Update(next); err != nil {
		t.Fatal(err)
	}
	public := manager.Public()
	if !public.Providers[0].APIKeyConfigured || public.Providers[0].APIKey != maskedSecret {
		t.Fatalf("secret was not masked: %#v", public.Providers[0])
	}
	update := manager.Current()
	update.Providers[0].APIKey = maskedSecret
	update.Providers[0].Name = "Updated"
	if _, err := manager.Update(update); err != nil {
		t.Fatal(err)
	}
	if manager.Current().Providers[0].APIKey != "secret" {
		t.Fatal("masked update replaced the stored secret")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}

func TestConfigValidationRejectsDuplicateIDs(t *testing.T) {
	cfg := Default()
	cfg.Providers = []Provider{
		{ID: "same", Name: "One", Type: "openai_compatible", BaseURL: "http://one"},
		{ID: "same", Name: "Two", Type: "openai_compatible", BaseURL: "http://two"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected duplicate id error")
	}
}
