package tests

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aibox/skillbox/internal/domain"
	"github.com/google/uuid"
)

func TestStorageContract(t *testing.T) {
	for name, store := range contractStores(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			suffix := uuid.NewString()
			w := domain.Workspace{Slug: "ws-" + suffix, Name: "Workspace"}
			if err := store.CreateWorkspace(ctx, &w); err != nil {
				t.Fatal(err)
			}
			p := domain.Project{WorkspaceID: w.ID, Slug: "project-" + suffix, Name: "Project"}
			if err := store.CreateProject(ctx, &p); err != nil {
				t.Fatal(err)
			}
			sk := sampleSkill("handler-" + suffix)
			sk.Scope = domain.ScopeProject
			sk.WorkspaceID = &w.ID
			sk.ProjectID = &p.ID
			if err := store.CreateSkill(ctx, &sk, "initial", nil); err != nil {
				t.Fatal(err)
			}
			got, err := store.GetSkill(ctx, sk.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Domains) != 2 || len(got.Steps) != 3 || got.CurrentVersion != 1 {
				t.Fatalf("relations/version mismatch: %#v", got)
			}
			got.Name = "Updated"
			if err = store.UpdateSkill(ctx, got, "rename", nil); err != nil {
				t.Fatal(err)
			}
			versions, err := store.ListVersions(ctx, sk.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(versions) != 2 || versions[0].Version != 2 {
				t.Fatalf("versions=%#v", versions)
			}
			rolled, err := store.RollbackSkill(ctx, sk.ID, 1, nil)
			if err != nil {
				t.Fatal(err)
			}
			if rolled.Name != "Go HTTP Handler" || rolled.CurrentVersion != 3 {
				t.Fatalf("rollback=%#v", rolled)
			}
			finished := time.Now().UTC()
			duration := int64(50)
			provider, model := "ollama", "qwen:7b"
			exec := domain.Execution{SkillID: sk.ID, SkillVersion: 3, ModelProvider: &provider, ModelName: &model, TaskSummary: "test", StartedAt: finished.Add(-time.Second), FinishedAt: &finished, DurationMS: &duration, Status: "success", Success: true}
			exec.Trajectory = []domain.ExecutionEvent{{Position: 1, Type: "tool_call", Data: `{"tool":"read_file"}`}, {Position: 2, Type: "result", Data: "verified"}}
			if err = store.CreateExecution(ctx, &exec); err != nil {
				t.Fatal(err)
			}
			stats, err := store.Statistics(ctx, &sk.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(stats) != 1 || stats[0].Runs != 1 || stats[0].SuccessRate != 100 {
				t.Fatalf("stats=%#v", stats)
			}
			trajectory, err := store.GetExecutionTrajectory(ctx, exec.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(trajectory) != 2 {
				t.Fatalf("trajectory=%#v", trajectory)
			}
			profile := domain.MCPProfile{Slug: "profile-" + suffix, Name: "Profile", Permissions: []string{domain.PermissionSkillRead}, Tools: []string{domain.ToolGetSkill}, Enabled: true}
			if err = store.UpsertMCPProfile(ctx, &profile); err != nil {
				t.Fatal(err)
			}
			connection := domain.MCPConnection{Slug: "connection-" + suffix, Name: "Connection", ProfileID: profile.ID, AuthType: "api_key", CredentialHash: "hash-" + suffix, Enabled: true}
			if err = store.CreateMCPConnection(ctx, &connection); err != nil {
				t.Fatal(err)
			}
			resolved, err := store.ResolveMCPConnection(ctx, connection.CredentialHash)
			if err != nil || resolved.ID != connection.ID {
				t.Fatalf("resolved=%#v err=%v", resolved, err)
			}
			proposal := domain.SkillProposal{SkillID: sk.ID, BaseVersion: 3, ProposedSnapshot: `{}`, Summary: "contract"}
			if err = store.CreateSkillProposal(ctx, &proposal); err != nil {
				t.Fatal(err)
			}
			if _, err = store.ReviewSkillProposal(ctx, proposal.ID, "approved", nil, nil); err != nil {
				t.Fatal(err)
			}
			if err = store.MarkSkillProposalPublished(ctx, proposal.ID); err != nil {
				t.Fatal(err)
			}
			published, err := store.GetSkillProposal(ctx, proposal.ID)
			if err != nil || published.Status != "published" {
				t.Fatalf("proposal=%#v err=%v", published, err)
			}
		})
	}
}
func TestMigrationFilesStayMirrored(t *testing.T) {
	for _, driver := range []string{"sqlite", "mysql", "postgres"} {
		for _, name := range []string{"001_initial.sql", "002_user_scope.sql", "003_mcp_profiles.sql"} {
			source := fmt.Sprintf("../migrations/%s/%s", driver, name)
			embedded := fmt.Sprintf("../internal/migrate/sql/%s_%s", driver, name)
			a, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(embedded)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(a, b) {
				t.Fatalf("%s %s migration embed copy is stale", driver, name)
			}
		}
	}
}
