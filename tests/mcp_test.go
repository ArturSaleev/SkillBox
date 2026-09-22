package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/aibox/skillbox/internal/application"
	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
	skillstore "github.com/aibox/skillbox/internal/skills/store"
	"github.com/aibox/skillbox/internal/storage/sqlite"
	mcptransport "github.com/aibox/skillbox/internal/transport/mcp"
	"github.com/go-chi/chi/v5"
)

func localMCP(t *testing.T) (ports.Storage, *httptest.Server) {
	t.Helper()
	store := sqliteStore(t)
	workspace, err := store.EnsureWorkspace(context.Background(), "local", "Local Workspace")
	if err != nil {
		t.Fatal(err)
	}
	handler := mcptransport.New(application.New(store), mcptransport.NewLocalResolver(store, workspace.ID))
	router := chi.NewRouter()
	router.Handle("/mcp/{project}", handler)
	router.Handle("/mcp/{project}/teacher", handler)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return store, server
}

func TestMCPPrepareReadsFilesystemPackageWithoutEmbeddingAttachments(t *testing.T) {
	ctx := context.Background()
	index, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "skillbox.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = index.Close() })
	skillsRoot := filepath.Join(t.TempDir(), "skills")
	store := skillstore.New(index, skillsRoot)
	workspace, err := store.EnsureWorkspace(ctx, "local", "Local Workspace")
	if err != nil {
		t.Fatal(err)
	}
	project, err := store.EnsureProject(ctx, workspace.ID, "filesystem-project", "Filesystem Project")
	if err != nil {
		t.Fatal(err)
	}

	skill := sampleSkill("filesystem-skill")
	skill.Scope, skill.WorkspaceID, skill.ProjectID = domain.ScopeProject, &workspace.ID, &project.ID
	if err = store.CreateSkill(ctx, &skill, "initial", nil); err != nil {
		t.Fatal(err)
	}
	packageRoot := filepath.Join(skillsRoot, skill.PackagePath)
	skillFile := filepath.Join(packageRoot, "SKILL.md")
	raw, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), skill.Instructions, "FILESYSTEM_INSTRUCTIONS_LIVE\n", 1))
	if err = os.WriteFile(skillFile, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	attachments := map[string]string{
		"scripts/check.sh":      "SCRIPT_SECRET_MARKER",
		"references/private.md": "REFERENCE_SECRET_MARKER",
		"assets/sample.txt":     "ASSET_SECRET_MARKER",
	}
	for name, content := range attachments {
		path := filepath.Join(packageRoot, filepath.FromSlash(name))
		if err = os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	handler := mcptransport.New(application.New(store), mcptransport.NewLocalResolver(store, workspace.ID))
	router := chi.NewRouter()
	router.Handle("/mcp/{project}", handler)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	url := server.URL + "/mcp/filesystem-project"
	if out := initialize(t, url); out["error"] != nil {
		t.Fatalf("initialize=%#v", out)
	}
	call := mustJSON(t, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]any{"name": domain.ToolPrepareSkill, "arguments": map[string]any{"task": "prepare filesystem skill", "skill_id": skill.ID}},
	})
	prepared := toolResult(t, rpcClient(t, url)("", call))
	compiled := prepared["compiled_skill"].(map[string]any)
	if instructions, _ := compiled["instructions"].(string); !strings.Contains(instructions, "FILESYSTEM_INSTRUCTIONS_LIVE") {
		t.Fatalf("skill_prepare did not read live SKILL.md: %#v", compiled)
	}
	encoded, err := json.Marshal(prepared)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"SCRIPT_SECRET_MARKER", "REFERENCE_SECRET_MARKER", "ASSET_SECRET_MARKER"} {
		if bytes.Contains(encoded, []byte(marker)) {
			t.Fatalf("attachment content leaked into skill_prepare response: %s", marker)
		}
	}
}

func initialize(t *testing.T, url string) map[string]any {
	t.Helper()
	return rpcClient(t, url)("", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
}

func TestMCPInitializeCreatesAndReusesProject(t *testing.T) {
	store, server := localMCP(t)
	first := initialize(t, server.URL+"/mcp/new-project")
	if first["error"] != nil {
		t.Fatalf("initialize failed: %#v", first)
	}
	projects, err := store.ListProjects(context.Background(), nil)
	if err != nil || len(projects) != 1 {
		t.Fatalf("projects=%#v err=%v", projects, err)
	}
	created := projects[0]
	if created.Slug != "new-project" || created.ExternalID != "new-project" || !created.AutoCreated {
		t.Fatalf("created project=%#v", created)
	}
	if second := initialize(t, server.URL+"/mcp/new-project"); second["error"] != nil {
		t.Fatalf("second initialize failed: %#v", second)
	}
	projects, err = store.ListProjects(context.Background(), nil)
	if err != nil || len(projects) != 1 || projects[0].ID != created.ID {
		t.Fatalf("reinitialized projects=%#v err=%v", projects, err)
	}
}

func TestMCPParallelInitializeDoesNotDuplicateProject(t *testing.T) {
	store, server := localMCP(t)
	const requests = 16
	var wg sync.WaitGroup
	errs := make(chan error, requests)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Post(server.URL+"/mcp/concurrent", "application/json", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
			if err != nil {
				errs <- err
				return
			}
			defer resp.Body.Close()
			var body map[string]any
			if err = json.NewDecoder(resp.Body).Decode(&body); err != nil {
				errs <- err
				return
			}
			if body["error"] != nil {
				errs <- &rpcResponseError{body: body}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	projects, err := store.ListProjects(context.Background(), nil)
	if err != nil || len(projects) != 1 {
		t.Fatalf("projects=%#v err=%v", projects, err)
	}
}

type rpcResponseError struct{ body map[string]any }

func (e *rpcResponseError) Error() string {
	raw, _ := json.Marshal(e.body)
	return string(raw)
}

func TestOnlyStudentAndTeacherRoutesExistWithoutAuthentication(t *testing.T) {
	_, server := localMCP(t)
	studentURL := server.URL + "/mcp/project-a"
	teacherURL := server.URL + "/mcp/project-a/teacher"
	if out := initialize(t, studentURL); out["error"] != nil {
		t.Fatalf("student initialize=%#v", out)
	}
	student := rpcClient(t, studentURL)("unused-key", `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	studentTools := student["result"].(map[string]any)["tools"].([]any)
	if len(studentTools) != 3 {
		t.Fatalf("student tools=%#v", studentTools)
	}
	want := map[string]bool{domain.ToolSearchSkills: true, domain.ToolPrepareSkill: true, domain.ToolReportSkillResult: true}
	for _, item := range studentTools {
		delete(want, item.(map[string]any)["name"].(string))
	}
	if len(want) != 0 {
		t.Fatalf("missing Student tools: %#v", want)
	}
	if out := initialize(t, teacherURL); out["error"] != nil {
		t.Fatalf("teacher initialize=%#v", out)
	}
	teacher := rpcClient(t, teacherURL)("", `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}`)
	teacherTools := teacher["result"].(map[string]any)["tools"].([]any)
	if len(teacherTools) != 19 {
		t.Fatalf("teacher tools=%#v", teacherTools)
	}
	for _, path := range []string{"/mcp", "/mcp/project-a/reviewer", "/api/v1/projects"} {
		resp, err := http.Post(server.URL+path, "application/json", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s status=%d", path, resp.StatusCode)
		}
	}
}

func TestTeacherCanCreateApproveAndPublishSkill(t *testing.T) {
	_, server := localMCP(t)
	url := server.URL + "/mcp/teacher-project/teacher"
	initialize(t, url)
	rpc := rpcClient(t, url)
	draft := rpc("", mustJSON(t, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": domain.ToolCreateSkillDraft, "arguments": sampleSkill("teacher-skill")}}))
	skillID := toolResult(t, draft)["id"].(string)
	proposal := rpc("", mustJSON(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{"name": domain.ToolCreateSkillProposal, "arguments": map[string]any{"skill_id": skillID, "summary": "ready"}}}))
	proposalID := toolResult(t, proposal)["id"].(string)
	approved := rpc("", mustJSON(t, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": map[string]any{"name": domain.ToolApproveSkillProposal, "arguments": map[string]any{"proposal_id": proposalID}}}))
	if toolResult(t, approved)["status"] != "approved" {
		t.Fatalf("approve=%#v", approved)
	}
	published := rpc("", mustJSON(t, map[string]any{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": map[string]any{"name": domain.ToolPublishSkill, "arguments": map[string]any{"proposal_id": proposalID}}}))
	if toolResult(t, published)["status"] != "active" {
		t.Fatalf("publish=%#v", published)
	}
}

func TestMCPRejectsInvalidProjectID(t *testing.T) {
	_, server := localMCP(t)
	if out := initialize(t, server.URL+"/mcp/bad$id"); out["error"] == nil {
		t.Fatalf("invalid project accepted: %#v", out)
	}
	for _, value := range []string{"", ".", "..", "../demo", "demo/project"} {
		if _, err := mcptransport.NormalizeProjectID(value); err == nil {
			t.Fatalf("project_id %q unexpectedly accepted", value)
		}
	}
}

func TestMCPProjectScopeIsolation(t *testing.T) {
	store, server := localMCP(t)
	initialize(t, server.URL+"/mcp/project-a")
	initialize(t, server.URL+"/mcp/project-b")
	ctx := context.Background()
	workspace, err := store.GetWorkspace(ctx, "local")
	if err != nil {
		t.Fatal(err)
	}
	projectB, err := store.GetProject(ctx, "project-b", &workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	skill := sampleSkill("only-project-b")
	skill.Scope, skill.WorkspaceID, skill.ProjectID = domain.ScopeProject, &workspace.ID, &projectB.ID
	if err = store.CreateSkill(ctx, &skill, "project B", nil); err != nil {
		t.Fatal(err)
	}
	call := mustJSON(t, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{"name": domain.ToolSearchSkills, "arguments": map[string]any{"task": "handler", "project_id": projectB.ID}}})
	result := toolResult(t, rpcClient(t, server.URL+"/mcp/project-a")("", call))
	if skills := result["skills"]; skills != nil && len(skills.([]any)) != 0 {
		t.Fatalf("project A saw project B skills: %#v", skills)
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

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
