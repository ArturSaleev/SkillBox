package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	benchmarkconfig "github.com/aibox/skillbox/benchmark/internal/config"
	"github.com/aibox/skillbox/benchmark/internal/storage"
)

func TestConfigurationAPIAlwaysMasksAndPreservesSecrets(t *testing.T) {
	dir := t.TempDir()
	manager, err := benchmarkconfig.NewManager(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := manager.Current()
	cfg.Providers = []benchmarkconfig.Provider{{ID: "provider", Name: "Provider", Type: "openai_compatible", Enabled: true, BaseURL: "http://provider.invalid/v1", APIKey: "top-secret"}}
	if _, err := manager.Update(cfg); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(context.Background(), filepath.Join(dir, "benchmark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := Handler(manager, store)

	request := httptest.NewRequest(http.MethodGet, "/config", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "top-secret") || !strings.Contains(response.Body.String(), "••••••••") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}

	public := manager.Public()
	public.Providers[0].Name = "Updated Provider"
	raw, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPut, "/config", bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	current := manager.Current()
	if current.Providers[0].APIKey != "top-secret" || current.Providers[0].Name != "Updated Provider" {
		t.Fatalf("config=%#v", current.Providers[0])
	}
}
