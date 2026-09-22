package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestChatPersistence(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "benchmark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	session := Session{Title: "Test", ProviderID: "local", Model: "model", MCPServerIDs: []string{"tools"}, Mode: "no_skill"}
	if err := store.CreateSession(ctx, &session); err != nil {
		t.Fatal(err)
	}
	duration := int64(25)
	message := Message{SessionID: session.ID, Role: "assistant", Content: "done", DurationMS: &duration, ToolCallsCount: 1, ToolCalls: []ToolCall{{ServerID: "tools", ToolName: "read", Arguments: `{}`, Result: `{"ok":true}`, DurationMS: 4}}}
	if err := store.AddMessage(ctx, &message); err != nil {
		t.Fatal(err)
	}
	messages, err := store.ListMessages(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || len(messages[0].ToolCalls) != 1 || messages[0].Content != "done" {
		t.Fatalf("messages=%#v", messages)
	}
}

func TestBenchmarkPersistence(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "benchmark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	item := BenchmarkCase{Name: "Case", Task: "Do the task", MCPServerIDs: []string{"data"}, SkillMCPServerID: "skillbox", SkillID: "skill-1", Repetitions: 2, MaxTokens: 512, RequiredPhrases: []string{"done"}, Targets: []BenchmarkTarget{{ProviderID: "local", Model: "model"}}}
	if err := store.SaveBenchmarkCase(ctx, &item); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.GetBenchmarkCase(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Targets) != 1 || loaded.RequiredPhrases[0] != "done" {
		t.Fatalf("case=%#v", loaded)
	}
	run := BenchmarkRun{CaseID: item.ID, TotalTrials: 4}
	if err := store.CreateBenchmarkRun(ctx, &run); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	trial := BenchmarkTrial{RunID: run.ID, ProviderID: "local", Model: "model", Variant: "baseline", Repetition: 1, Status: "completed", Response: "done", QualityScore: 100, Passed: true, StartedAt: now, FinishedAt: now, ScoreChecks: []ScoreCheck{{Name: "required: done", Passed: true}}}
	if err := store.AddBenchmarkTrial(ctx, &trial); err != nil {
		t.Fatal(err)
	}
	loadedRun, err := store.GetBenchmarkRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loadedRun.Trials) != 1 || !loadedRun.Trials[0].Passed {
		t.Fatalf("run=%#v", loadedRun)
	}
}
