package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	benchmarkconfig "github.com/aibox/skillbox/benchmark/internal/config"
	"github.com/aibox/skillbox/benchmark/internal/storage"
)

func TestSendRunsMCPToolLoopAndPersistsTrajectory(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		hasToolResult := false
		for _, message := range request.Messages {
			hasToolResult = hasToolResult || message.Role == "tool"
		}
		message := map[string]any{"role": "assistant", "content": "The answer is 42."}
		if !hasToolResult {
			message = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"id": "call-1", "type": "function", "function": map[string]any{"name": "data__lookup", "arguments": `{"query":"answer"}`}}}}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "response", "model": "test-model", "choices": []any{map[string]any{"index": 0, "message": message}}, "usage": map[string]any{"prompt_tokens": 4, "completion_tokens": 2}})
	}))
	defer providerServer.Close()

	mcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Method == "notifications/initialized" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var result any
		switch request.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-06-18"}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "lookup", "description": "Lookup data", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			result = map[string]any{"content": []any{map[string]any{"type": "text", "text": `{"answer":42}`}}}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	defer mcpServer.Close()

	manager, err := benchmarkconfig.NewManager(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := manager.Current()
	cfg.Providers = []benchmarkconfig.Provider{{ID: "model", Name: "Model", Type: "openai_compatible", Enabled: true, BaseURL: providerServer.URL}}
	cfg.MCPServers = []benchmarkconfig.MCPServer{{ID: "data", Name: "Data", Enabled: true, Transport: "streamable_http", URL: mcpServer.URL}}
	if _, err := manager.Update(cfg); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "benchmark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := NewService(manager, store)
	response, err := service.Send(context.Background(), SendRequest{ProviderID: "model", Model: "test-model", MCPServerIDs: []string{"data"}, Message: "Find the answer"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Message.Content != "The answer is 42." || len(response.Message.ToolCalls) != 1 {
		t.Fatalf("response=%#v", response)
	}
	messages, err := store.ListMessages(context.Background(), response.Session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[1].ToolCalls[0].ToolName != "lookup" {
		t.Fatalf("messages=%#v", messages)
	}
}
