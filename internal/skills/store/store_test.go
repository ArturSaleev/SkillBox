package store_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/skills/store"
	"github.com/aibox/skillbox/internal/storage/sqlite"
)

func TestFilesystemPackageIsAuthoritativeAndRuntimeStateStaysInSQLite(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "skills")
	index, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "skillbox.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()

	fsStore := store.New(index, root)
	sk := domain.Skill{
		Slug: "review-code", Name: "Review code", Description: "Review changes",
		Instructions: "Original instructions\n", Scope: domain.ScopeGlobal, Status: domain.StatusDraft,
		Domains: []string{"engineering"}, Steps: []domain.Step{{Position: 1, Title: "Inspect", Instruction: "Read files", Required: true}},
	}
	if err = fsStore.CreateSkill(ctx, &sk, "initial", nil); err != nil {
		t.Fatal(err)
	}
	if sk.PackagePath != sk.ID || len(sk.PackageHash) != 64 || sk.PackageIndexedAt == nil {
		t.Fatalf("package index was not populated: %#v", sk)
	}
	skillFile := filepath.Join(root, sk.PackagePath, "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, runtimeField := range []string{"workspace_id:", "project_id:", "status:", "current_version:", "package_hash:"} {
		if strings.Contains(string(data), runtimeField) {
			t.Fatalf("runtime/index field %q leaked into SKILL.md:\n%s", runtimeField, data)
		}
	}
	changed := strings.Replace(string(data), "Original instructions", "Filesystem instructions", 1)
	if err = os.WriteFile(skillFile, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := fsStore.GetSkill(ctx, sk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Instructions != "Filesystem instructions\n" || got.Status != domain.StatusDraft || got.PackageHash == sk.PackageHash {
		t.Fatalf("filesystem content/runtime merge mismatch: %#v", got)
	}
	got.Instructions = "Version two\n"
	if err = fsStore.UpdateSkill(ctx, got, "version two", nil); err != nil {
		t.Fatal(err)
	}
	rolled, err := fsStore.RollbackSkill(ctx, sk.ID, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rolled.Instructions != "Original instructions\n" || rolled.CurrentVersion != 3 {
		t.Fatalf("package rollback mismatch: %#v", rolled)
	}
	got = rolled

	finished := got.UpdatedAt
	execution := domain.Execution{SkillID: got.ID, SkillVersion: got.CurrentVersion, TaskSummary: "test", StartedAt: finished, FinishedAt: &finished, Status: "success", Success: true}
	if err = fsStore.CreateExecution(ctx, &execution); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, sk.PackagePath, "execution.json")); !os.IsNotExist(err) {
		t.Fatalf("runtime evidence leaked into package: %v", err)
	}
	executions, err := fsStore.ListExecutions(ctx, &sk.ID)
	if err != nil || len(executions) != 1 {
		t.Fatalf("runtime evidence was not retained in SQLite: executions=%#v err=%v", executions, err)
	}
}

func TestLegacyDatabaseSkillRemainsReadableAndUpdatesWithoutImplicitMigration(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "skills")
	index, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "skillbox.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	legacy := domain.Skill{Slug: "legacy", Name: "Legacy", Instructions: "DB content", Scope: domain.ScopeGlobal, Status: domain.StatusDraft}
	if err = index.CreateSkill(ctx, &legacy, "legacy", nil); err != nil {
		t.Fatal(err)
	}
	fsStore := store.New(index, root)
	got, err := fsStore.GetSkill(ctx, legacy.ID)
	if err != nil || got.Instructions != "DB content" {
		t.Fatalf("legacy read: skill=%#v err=%v", got, err)
	}
	got.Name = "Legacy updated"
	if err = fsStore.UpdateSkill(ctx, got, "legacy update", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("legacy update unexpectedly created a package: %v", err)
	}
}

func TestMigrateLegacySkillsPreservesIdentityLifecycleHistoryAndEvidence(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "skills")
	index, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "skillbox.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	legacy := domain.Skill{Slug: "legacy", Name: "Legacy", Description: "DB only", Instructions: "Version one\n", Scope: domain.ScopeGlobal, Status: domain.StatusDraft}
	if err = index.CreateSkill(ctx, &legacy, "legacy v1", nil); err != nil {
		t.Fatal(err)
	}
	legacy.Instructions = "Version two\n"
	legacy.Status = domain.StatusActive
	if err = index.UpdateSkill(ctx, &legacy, "legacy v2", nil); err != nil {
		t.Fatal(err)
	}
	finished := legacy.UpdatedAt
	execution := domain.Execution{SkillID: legacy.ID, SkillVersion: 1, TaskSummary: "preserve evidence", StartedAt: finished, FinishedAt: &finished, Status: "success", Success: true, Trajectory: []domain.ExecutionEvent{{Position: 1, Type: "result", Data: `{"ok":true}`}}}
	if err = index.CreateExecution(ctx, &execution); err != nil {
		t.Fatal(err)
	}

	fsStore := store.New(index, root)
	result, err := fsStore.MigrateLegacySkills(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Migrated != 1 || result.Unchanged != 0 {
		t.Fatalf("migration result=%#v", result)
	}
	got, err := fsStore.GetSkill(ctx, legacy.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != legacy.ID || got.PackagePath != legacy.ID || got.CurrentVersion != 2 || got.Status != domain.StatusActive || got.Instructions != "Version two\n" {
		t.Fatalf("migrated lifecycle mismatch: %#v", got)
	}
	versions, err := fsStore.ListVersions(ctx, legacy.ID)
	if err != nil || len(versions) != 2 || versions[0].PackageHash == "" || versions[1].PackageHash == "" {
		t.Fatalf("version history mismatch: versions=%#v err=%v", versions, err)
	}
	rolled, err := fsStore.RollbackSkill(ctx, legacy.ID, 1, nil)
	if err != nil || rolled.Instructions != "Version one\n" || rolled.CurrentVersion != 3 {
		t.Fatalf("rollback after migration: skill=%#v err=%v", rolled, err)
	}
	executions, err := fsStore.ListExecutions(ctx, &legacy.ID)
	if err != nil || len(executions) != 1 || executions[0].ID != execution.ID {
		t.Fatalf("execution evidence changed: executions=%#v err=%v", executions, err)
	}
	trajectory, err := fsStore.GetExecutionTrajectory(ctx, execution.ID)
	if err != nil || len(trajectory) != 1 || trajectory[0].Data != `{"ok":true}` {
		t.Fatalf("trajectory changed: trajectory=%#v err=%v", trajectory, err)
	}
}

func TestMigrateLegacySkillsIsIdempotent(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "skills")
	index, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "skillbox.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	legacy := domain.Skill{Slug: "legacy", Name: "Legacy", Description: "DB only", Instructions: "Keep me\n", Scope: domain.ScopeGlobal, Status: domain.StatusActive}
	if err = index.CreateSkill(ctx, &legacy, "legacy", nil); err != nil {
		t.Fatal(err)
	}
	fsStore := store.New(index, root)
	if _, err = fsStore.MigrateLegacySkills(ctx); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, legacy.ID, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := fsStore.MigrateLegacySkills(ctx)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, legacy.ID, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Migrated != 0 || result.Unchanged != 1 || string(before) != string(after) {
		t.Fatalf("second migration changed data: result=%#v", result)
	}
	versions, err := fsStore.ListVersions(ctx, legacy.ID)
	if err != nil || len(versions) != 1 {
		t.Fatalf("idempotent migration changed history: versions=%#v err=%v", versions, err)
	}
}

func TestReindexRebuildsMissingDatabaseAndDoesNotCreateVersionChurn(t *testing.T) {
	ctx := context.Background()
	temp := t.TempDir()
	root := filepath.Join(temp, "skills")
	dbPath := filepath.Join(temp, "skillbox.db")
	index, err := sqlite.Open(ctx, dbPath, true)
	if err != nil {
		t.Fatal(err)
	}
	fsStore := store.New(index, root)
	sk := domain.Skill{Slug: "recoverable", Name: "Recoverable", Description: "Filesystem source", Instructions: "Version one\n", Scope: domain.ScopeGlobal, Status: domain.StatusActive}
	if err = fsStore.CreateSkill(ctx, &sk, "initial", nil); err != nil {
		t.Fatal(err)
	}
	if result, reindexErr := fsStore.Reindex(ctx); reindexErr != nil || result.Unchanged != 1 {
		t.Fatalf("unchanged reindex: result=%#v err=%v", result, reindexErr)
	}
	versions, err := fsStore.ListVersions(ctx, sk.ID)
	if err != nil || len(versions) != 1 {
		t.Fatalf("startup created version history: versions=%#v err=%v", versions, err)
	}
	if err = index.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(dbPath); err != nil {
		t.Fatal(err)
	}

	rebuiltIndex, err := sqlite.Open(ctx, dbPath, true)
	if err != nil {
		t.Fatal(err)
	}
	defer rebuiltIndex.Close()
	rebuilt := store.New(rebuiltIndex, root)
	result, err := rebuilt.Reindex(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.Discovered != 1 || result.Created != 1 {
		t.Fatalf("rebuild result=%#v", result)
	}
	got, err := rebuilt.GetSkill(ctx, sk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != sk.Name || got.Instructions != sk.Instructions || got.Status != domain.StatusDraft || got.Scope != domain.ScopeGlobal || got.PackageHash == "" {
		t.Fatalf("rebuilt skill=%#v", got)
	}
	versions, err = rebuilt.ListVersions(ctx, sk.ID)
	if err != nil || len(versions) != 1 || versions[0].PackageHash == "" {
		t.Fatalf("rebuild manufactured history: versions=%#v err=%v", versions, err)
	}
}

func TestReindexRefreshesChangedPackageWithoutDeletingRuntimeHistory(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "skills")
	index, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "skillbox.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	fsStore := store.New(index, root)
	sk := domain.Skill{Slug: "tracked", Name: "Tracked", Description: "Tracked", Instructions: "Before\n", Scope: domain.ScopeGlobal, Status: domain.StatusActive}
	if err = fsStore.CreateSkill(ctx, &sk, "initial", nil); err != nil {
		t.Fatal(err)
	}
	execution := domain.Execution{SkillID: sk.ID, SkillVersion: 1, TaskSummary: "preserve", Status: "success", Success: true}
	if err = fsStore.CreateExecution(ctx, &execution); err != nil {
		t.Fatal(err)
	}
	skillFile := filepath.Join(root, sk.PackagePath, "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(skillFile, []byte(strings.Replace(string(data), "Before", "After", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := fsStore.Reindex(ctx)
	if err != nil || result.Updated != 1 {
		t.Fatalf("changed reindex: result=%#v err=%v", result, err)
	}
	got, err := fsStore.GetSkill(ctx, sk.ID)
	if err != nil || got.Instructions != "After\n" || got.Status != domain.StatusActive || got.CurrentVersion != 2 {
		t.Fatalf("reindexed skill=%#v err=%v", got, err)
	}
	versions, _ := fsStore.ListVersions(ctx, sk.ID)
	executions, _ := fsStore.ListExecutions(ctx, &sk.ID)
	if len(versions) != 2 || len(executions) != 1 {
		t.Fatalf("history changed: versions=%d executions=%d", len(versions), len(executions))
	}
	if err = os.RemoveAll(filepath.Join(root, sk.PackagePath)); err != nil {
		t.Fatal(err)
	}
	if result, err = fsStore.Reindex(ctx); err != nil || result.Discovered != 0 {
		t.Fatalf("missing package reindex: result=%#v err=%v", result, err)
	}
	indexed, err := index.GetSkill(ctx, sk.ID)
	if err != nil || indexed.PackagePath == "" {
		t.Fatalf("missing package caused index deletion: skill=%#v err=%v", indexed, err)
	}
}

func TestReindexValidatesAllPackagesBeforeChangingIndex(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(filepath.Join(root, "valid"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "valid", "SKILL.md"), []byte("---\nname: Valid\ndescription: Valid\n---\nDo it\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "invalid"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "invalid", "SKILL.md"), []byte("not frontmatter\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	index, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "skillbox.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	fsStore := store.New(index, root)
	if _, err = fsStore.Reindex(ctx); err == nil {
		t.Fatal("invalid package was accepted")
	}
	items, err := index.ListSkills(ctx)
	if err != nil || len(items) != 0 {
		t.Fatalf("index changed before complete validation: items=%#v err=%v", items, err)
	}
}

func TestReindexCreatesAllSkillsBeforeDependencyRelations(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "skills")
	firstID := "11111111-1111-4111-8111-111111111111"
	secondID := "22222222-2222-4222-8222-222222222222"
	first := "---\nname: First\ndescription: First\nslug: first\ndependencies:\n  - id: dep-1\n    depends_on_skill_id: " + secondID + "\n    type: requires\n    position: 1\n---\nFirst\n"
	second := "---\nname: Second\ndescription: Second\nslug: second\n---\nSecond\n"
	for id, body := range map[string]string{firstID: first, secondID: second} {
		if err := os.MkdirAll(filepath.Join(root, id), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, id, "SKILL.md"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	index, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "skillbox.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	fsStore := store.New(index, root)
	result, err := fsStore.Reindex(ctx)
	if err != nil || result.Created != 2 {
		t.Fatalf("dependency reindex: result=%#v err=%v", result, err)
	}
	got, err := fsStore.GetSkill(ctx, firstID)
	if err != nil || len(got.Dependencies) != 1 || got.Dependencies[0].DependsOnSkillID != secondID {
		t.Fatalf("dependency not restored: skill=%#v err=%v", got, err)
	}
}

func TestPackageRollbackRestoresAllFilesAsOneVersion(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join(t.TempDir(), "skills")
	index, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "skillbox.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	fsStore := store.New(index, root)
	sk := domain.Skill{Slug: "whole-package", Name: "Whole package", Description: "Whole package", Instructions: "Instructions\n", Scope: domain.ScopeGlobal, Status: domain.StatusActive}
	if err = fsStore.CreateSkill(ctx, &sk, "initial", nil); err != nil {
		t.Fatal(err)
	}
	packageRoot := filepath.Join(root, sk.PackagePath)
	writeTestFile(t, filepath.Join(packageRoot, "scripts", "run.sh"), "version-two\n")
	writeTestFile(t, filepath.Join(packageRoot, "assets", "old.txt"), "old\n")
	if result, reindexErr := fsStore.Reindex(ctx); reindexErr != nil || result.Updated != 1 {
		t.Fatalf("record version two: result=%#v err=%v", result, reindexErr)
	}
	versionTwo, err := fsStore.GetVersion(ctx, sk.ID, 2)
	if err != nil || versionTwo.PackageHash == "" {
		t.Fatalf("version two=%#v err=%v", versionTwo, err)
	}
	writeTestFile(t, filepath.Join(packageRoot, "scripts", "run.sh"), "version-three\n")
	if err = os.Remove(filepath.Join(packageRoot, "assets", "old.txt")); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(packageRoot, "assets", "new.txt"), "new\n")
	if result, reindexErr := fsStore.Reindex(ctx); reindexErr != nil || result.Updated != 1 {
		t.Fatalf("record version three: result=%#v err=%v", result, reindexErr)
	}

	rolled, err := fsStore.RollbackSkill(ctx, sk.ID, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rolled.CurrentVersion != 4 || rolled.PackageHash != versionTwo.PackageHash {
		t.Fatalf("rollback metadata=%#v versionTwo=%#v", rolled, versionTwo)
	}
	assertTestFile(t, filepath.Join(packageRoot, "scripts", "run.sh"), "version-two\n")
	assertTestFile(t, filepath.Join(packageRoot, "assets", "old.txt"), "old\n")
	if _, err = os.Stat(filepath.Join(packageRoot, "assets", "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("rollback did not remove later file: %v", err)
	}
	versionFour, err := fsStore.GetVersion(ctx, sk.ID, 4)
	if err != nil || versionFour.PackageHash != versionTwo.PackageHash {
		t.Fatalf("rollback version=%#v err=%v", versionFour, err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertTestFile(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("%s=%q want %q", path, data, want)
	}
}
