package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	benchmarkconfig "github.com/aibox/skillbox/benchmark/internal/config"
)

func TestClientListsModelsAndCallsChat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatal("missing authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/models":
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"id": "model", "supported_parameters": []string{"tools"}}}})
		case "/chat/completions":
			json.NewEncoder(w).Encode(map[string]any{"id": "1", "model": "model", "choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "ok"}}}, "usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := New(benchmarkconfig.Provider{BaseURL: server.URL, APIKey: "token", RequestTimeout: "2s"})
	models, err := client.Models(context.Background())
	if err != nil || len(models) != 1 || !models[0].ToolCapable {
		t.Fatalf("models=%#v err=%v", models, err)
	}
	response, err := client.Chat(context.Background(), ChatRequest{Model: "model", Messages: []Message{{Role: "user", Content: "hello"}}})
	if err != nil || response.Choices[0].Message.Content != "ok" {
		t.Fatalf("response=%#v err=%v", response, err)
	}
}
