package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
	"github.com/aibox/skillbox/internal/storage/mysql"
	"github.com/aibox/skillbox/internal/storage/postgres"
	"github.com/aibox/skillbox/internal/storage/sqlite"
)

func sqliteStore(t *testing.T) ports.Storage {
	t.Helper()
	store, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "skillbox.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
func sampleSkill(slug string) domain.Skill {
	return domain.Skill{Slug: slug, Name: "Go HTTP Handler", Description: "Create a handler", Purpose: "Create a Go HTTP endpoint", WhenToUse: "When adding an HTTP endpoint", WhenNotToUse: "For non-Go projects", Instructions: "Inspect routes and existing handlers. Implement the endpoint using the application boundary.", SuccessCriteria: []string{"Contract is preserved", "Tests pass"}, Scope: domain.ScopeGlobal, Status: domain.StatusActive, Priority: 5, Domains: []string{"golang", "backend"}, Intents: []string{"create"}, ObjectTypes: []string{"api_endpoint"}, Tags: []string{"http", "handler"}, Keywords: []string{"endpoint", "route"}, Steps: []domain.Step{{Position: 1, Title: "Inspect", Instruction: "Read routes", Required: true}, {Position: 2, Title: "Implement", Instruction: "Add handler", Required: true}, {Position: 3, Title: "Optional docs", Instruction: "Update nearby notes", Required: false}}, Tools: []domain.ToolRequirement{{Name: "read_file", Requirement: "required", Purpose: "Inspect code"}, {Name: "write_file", Requirement: "required", Purpose: "Edit code"}, {Name: "run_command", Requirement: "optional", Purpose: "Run tests"}}, Contexts: []domain.ContextRequirement{{Type: "project_structure", Query: "Go project tree", Required: true, Priority: 100}, {Type: "documentation", Query: "Local conventions", Required: false, Priority: 10}}, Examples: []domain.Example{{Title: "Handler", InputExample: "Add GET /users", ExpectedBehavior: "Follow existing response contract", Priority: 10}}}
}
func contractStores(t *testing.T) map[string]ports.Storage {
	t.Helper()
	out := map[string]ports.Storage{"sqlite": sqliteStore(t)}
	ctx := context.Background()
	if dsn := os.Getenv("SKILLBOX_TEST_MYSQL_DSN"); dsn != "" {
		s, err := mysql.Open(ctx, dsn, true)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close() })
		out["mysql"] = s
	}
	if dsn := os.Getenv("SKILLBOX_TEST_POSTGRES_DSN"); dsn != "" {
		s, err := postgres.Open(ctx, dsn, true)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close() })
		out["postgres"] = s
	}
	return out
}
