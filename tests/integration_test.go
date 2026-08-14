package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/aibox/skillbox/internal/application"
	"github.com/aibox/skillbox/internal/compiler"
	"github.com/aibox/skillbox/internal/domain"
)

func TestCreateSearchCompileReportStatistics(t *testing.T) {
	store := sqliteStore(t)
	ctx := context.Background()
	sk := sampleSkill("go-handler")
	if err := store.CreateSkill(ctx, &sk, "initial", nil); err != nil {
		t.Fatal(err)
	}
	app := application.New(store)
	candidates, err := app.Search(ctx, domain.SearchFilter{Task: "create golang api endpoint", Domains: []string{"golang"}, Intents: []string{"create"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].ID != sk.ID {
		t.Fatalf("candidates=%#v", candidates)
	}
	prepared, err := app.Prepare(ctx, domain.PrepareRequest{Task: "create endpoint", AvailableTools: []string{"read_file", "write_file"}, Model: domain.ModelInfo{Name: "qwen:7b", ContextWindow: 4096}, MaxSkillTokens: 300})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.SkillID != sk.ID || len(prepared.CompiledSkill.Examples) != 1 || len(prepared.CompiledSkill.MissingTools) != 0 {
		t.Fatalf("prepared=%#v", prepared)
	}
	large, err := app.Prepare(ctx, domain.PrepareRequest{Task: "create endpoint", SkillID: &sk.ID, Model: domain.ModelInfo{ContextWindow: 32768}})
	if err != nil {
		t.Fatal(err)
	}
	if len(large.CompiledSkill.Examples) != 0 {
		t.Fatal("large model should not receive redundant example")
	}
}
func TestCompilerDetectsDependencyCycle(t *testing.T) {
	store := sqliteStore(t)
	ctx := context.Background()
	a := sampleSkill("a")
	b := sampleSkill("b")
	if err := store.CreateSkill(ctx, &a, "a", nil); err != nil {
		t.Fatal(err)
	}
	b.Dependencies = []domain.Dependency{{DependsOnSkillID: a.ID, Type: "requires"}}
	if err := store.CreateSkill(ctx, &b, "b", nil); err != nil {
		t.Fatal(err)
	}
	a.Dependencies = []domain.Dependency{{DependsOnSkillID: b.ID, Type: "requires"}}
	if err := store.UpdateSkill(ctx, &a, "cycle", nil); err != nil {
		t.Fatal(err)
	}
	_, err := compiler.New(store).Compile(ctx, &a, domain.PrepareRequest{Task: "x"})
	if !errors.Is(err, compiler.ErrDependencyCycle) {
		t.Fatalf("expected cycle, got %v", err)
	}
}
