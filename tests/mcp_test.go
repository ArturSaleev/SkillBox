package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aibox/skillbox/internal/application"
	"github.com/aibox/skillbox/internal/auth"
	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/metrics"
	mcptransport "github.com/aibox/skillbox/internal/transport/mcp"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMCPProfilesFilterDiscoveryAndEnforceCalls(t *testing.T) {
	store := sqliteStore(t)
	app := application.New(store)
	if err := app.EnsureBuiltInProfiles(context.Background()); err != nil {
		t.Fatal(err)
	}
	studentKey := "student-secret"
	studentProfile, err := store.GetMCPProfileBySlug(context.Background(), "student")
	if err != nil {
		t.Fatal(err)
	}
	student := domain.MCPConnection{Slug: "student", Name: "Student", ProfileID: studentProfile.ID, AuthType: "api_key", CredentialHash: auth.HashAPIKey(studentKey), Enabled: true}
	if err = store.CreateMCPConnection(context.Background(), &student); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(mcptransport.New(app, metrics.New(prometheus.NewRegistry()), slog.New(slog.NewTextHandler(io.Discard, nil)), mcptransport.NewConnectionResolver(store)))
	defer server.Close()
	rpc := rpcClient(t, server.URL)
	listed := rpc(studentKey, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	tools := listed["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 3 {
		t.Fatalf("student tools=%#v", tools)
	}
	for _, item := range tools {
		if item.(map[string]any)["name"] == domain.ToolGetSkill {
			t.Fatal("student must not discover get_skill")
		}
	}
	forbidden := rpc(studentKey, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_skill","arguments":{"skill_id":"x"}}}`)
	if forbidden["error"] == nil {
		t.Fatalf("forbidden call succeeded: %#v", forbidden)
	}
	missing := rpc("", `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`)
	if missing["error"] == nil {
		t.Fatal("unauthenticated MCP discovery must fail")
	}
}

func TestTeacherReviewerStudentLifecycle(t *testing.T) {
	store := sqliteStore(t)
	app := application.New(store)
	ctx := context.Background()
	if err := app.EnsureBuiltInProfiles(ctx); err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{"teacher": "teacher-key", "reviewer": "reviewer-key", "student": "student-key"}
	for slug, key := range keys {
		profile, err := store.GetMCPProfileBySlug(ctx, slug)
		if err != nil {
			t.Fatal(err)
		}
		connection := domain.MCPConnection{Slug: slug, Name: slug, ProfileID: profile.ID, AuthType: "api_key", CredentialHash: auth.HashAPIKey(key), Enabled: true}
		if err = store.CreateMCPConnection(ctx, &connection); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(mcptransport.New(app, metrics.New(prometheus.NewRegistry()), slog.New(slog.NewTextHandler(io.Discard, nil)), mcptransport.NewConnectionResolver(store)))
	defer server.Close()
	rpc := rpcClient(t, server.URL)
	draftCall := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": domain.ToolCreateSkillDraft, "arguments": sampleSkill("profile-lifecycle")}}
	draft := rpc(keys["teacher"], mustJSON(t, draftCall))
	draftResult := toolResult(t, draft)
	skillID := draftResult["id"].(string)
	if draftResult["status"] != "draft" {
		t.Fatalf("draft=%#v", draftResult)
	}
	proposal := rpc(keys["teacher"], mustJSON(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{"name": domain.ToolCreateSkillProposal, "arguments": map[string]any{"skill_id": skillID, "summary": "ready"}}}))
	proposalID := toolResult(t, proposal)["id"].(string)
	blocked := rpc(keys["student"], mustJSON(t, map[string]any{"jsonrpc": "2.0", "id": 20, "method": "tools/call", "params": map[string]any{"name": domain.ToolPrepareSkill, "arguments": map[string]any{"task": "draft", "skill_id": skillID}}}))
	if blocked["error"] == nil {
		t.Fatal("student prepared unpublished draft")
	}
	approved := rpc(keys["reviewer"], mustJSON(t, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": map[string]any{"name": domain.ToolApproveSkillProposal, "arguments": map[string]any{"proposal_id": proposalID}}}))
	if toolResult(t, approved)["status"] != "approved" {
		t.Fatalf("approve=%#v", approved)
	}
	published := rpc(keys["reviewer"], mustJSON(t, map[string]any{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": map[string]any{"name": domain.ToolPublishSkill, "arguments": map[string]any{"proposal_id": proposalID}}}))
	if toolResult(t, published)["status"] != "active" {
		t.Fatalf("publish=%#v", published)
	}
	persistedProposal, err := store.GetSkillProposal(ctx, proposalID)
	if err != nil || persistedProposal.Status != "published" {
		t.Fatalf("proposal status=%#v err=%v", persistedProposal, err)
	}
	prepared := rpc(keys["student"], mustJSON(t, map[string]any{"jsonrpc": "2.0", "id": 5, "method": "tools/call", "params": map[string]any{"name": domain.ToolPrepareSkill, "arguments": map[string]any{"task": "create go endpoint", "skill_id": skillID, "model": map[string]any{"context_window": 4096}}}}))
	if toolResult(t, prepared)["skill_id"] != skillID {
		t.Fatalf("prepare=%#v", prepared)
	}
}

func rpcClient(t *testing.T, url string) func(string, string) map[string]any {
	return func(key, payload string) map[string]any {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("X-SkillBox-Key", key)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		if err = json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatal(err)
		}
		return out
	}
}
func toolResult(t *testing.T, response map[string]any) map[string]any {
	t.Helper()
	if response["error"] != nil {
		t.Fatalf("rpc error: %#v", response)
	}
	return response["result"].(map[string]any)["structuredContent"].(map[string]any)
}
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
