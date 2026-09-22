package benchmark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	benchmarkconfig "github.com/aibox/skillbox/benchmark/internal/config"
	"github.com/aibox/skillbox/benchmark/internal/storage"
)

func TestPairedRunComparesBaselineWithPreparedSkill(t *testing.T) {
	providerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Messages []struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		hasTool, hasSkill := false, false
		for _, message := range request.Messages {
			hasTool = hasTool || message.Role == "tool"
			if message.Role == "system" && strings.Contains(message.Content.(string), "frozen Skill") {
				hasSkill = true
			}
		}
		message := map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"id": "call", "type": "function", "function": map[string]any{"name": "data__lookup", "arguments": `{}`}}}}
		if hasTool {
			content := "unverified guess"
			if hasSkill {
				content = "verified answer"
			}
			message = map[string]any{"role": "assistant", "content": content}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"model": "test-model", "choices": []any{map[string]any{"message": message}}, "usage": map[string]any{"prompt_tokens": 2, "completion_tokens": 1}})
	}))
	defer providerServer.Close()

	dataServer := newMCPTestServer(t, func(method string) any {
		switch method {
		case "initialize":
			return map[string]any{"protocolVersion": "2025-06-18"}
		case "tools/list":
			return map[string]any{"tools": []any{map[string]any{"name": "lookup", "description": "Lookup", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			return map[string]any{"content": []any{map[string]any{"type": "text", "text": `{"answer":42}`}}}
		}
		return nil
	})
	defer dataServer.Close()

	skillServer := newMCPTestServer(t, func(method string) any {
		switch method {
		case "initialize":
			return map[string]any{"protocolVersion": "2025-06-18"}
		case "tools/list":
			return map[string]any{"tools": []any{map[string]any{"name": "prepare_skill", "inputSchema": map[string]any{"type": "object"}}}}
		case "tools/call":
			prepared := map[string]any{"skill_id": "skill-1", "version": 3, "name": "Verification", "compiled_skill": map[string]any{"instructions": "Verify the answer.", "steps": []any{map[string]any{"title": "Lookup", "instruction": "Use lookup.", "required": true}}, "success_criteria": []string{"Answer is verified"}}}
			raw, _ := json.Marshal(prepared)
			return map[string]any{"content": []any{map[string]any{"type": "text", "text": string(raw)}}}
		}
		return nil
	})
	defer skillServer.Close()

	dir := t.TempDir()
	manager, err := benchmarkconfig.NewManager(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := manager.Current()
	cfg.Providers = []benchmarkconfig.Provider{{ID: "provider", Name: "Provider", Type: "openai_compatible", Enabled: true, BaseURL: providerServer.URL}}
	cfg.MCPServers = []benchmarkconfig.MCPServer{{ID: "data", Name: "Data", Enabled: true, Transport: "streamable_http", URL: dataServer.URL}, {ID: "skillbox", Name: "SkillBox", Enabled: true, Transport: "streamable_http", URL: skillServer.URL}}
	if _, err := manager.Update(cfg); err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(context.Background(), filepath.Join(dir, "benchmark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := NewService(manager, store)
	item := storage.BenchmarkCase{Name: "Paired", Task: "Find the answer", MCPServerIDs: []string{"data"}, SkillMCPServerID: "skillbox", SkillID: "skill-1", Repetitions: 1, MaxTokens: 256, RequiredPhrases: []string{"verified answer"}, RequiredTools: []string{"data.lookup"}, Targets: []storage.BenchmarkTarget{{ProviderID: "provider", Model: "test-model"}}}
	if err := service.SaveCase(context.Background(), &item); err != nil {
		t.Fatal(err)
	}
	run, err := service.Start(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		run, err = store.GetBenchmarkRun(context.Background(), run.ID)
		if err != nil {
			t.Fatal(err)
		}
		if run.Status == "completed" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if run.Status != "completed" || len(run.Trials) != 2 {
		t.Fatalf("run=%#v", run)
	}
	var baseline, withSkill storage.BenchmarkTrial
	for _, trial := range run.Trials {
		if trial.Variant == "baseline" {
			baseline = trial
		} else {
			withSkill = trial
		}
	}
	if baseline.Passed || !withSkill.Passed || withSkill.QualityScore != 100 || withSkill.SkillVersion == nil || *withSkill.SkillVersion != 3 {
		t.Fatalf("baseline=%#v withSkill=%#v", baseline, withSkill)
	}
}

func newMCPTestServer(t *testing.T, result func(string) any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result(request.Method)})
	}))
}
