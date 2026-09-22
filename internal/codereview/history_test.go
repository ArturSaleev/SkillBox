package codereview_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/aibox/skillbox/internal/codereview"
	"github.com/aibox/skillbox/internal/domain"
	skillstore "github.com/aibox/skillbox/internal/skills/store"
	"github.com/aibox/skillbox/internal/storage/sqlite"
)

func TestReviewHistoryBecomesOutdatedWhenPackageHashChanges(t *testing.T) {
	ctx := context.Background()
	index, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "skillbox.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	store := skillstore.New(index, filepath.Join(t.TempDir(), "skills"))
	skill := domain.Skill{
		Slug: "safe-skill", Name: "Safe skill", Description: "Review target",
		Instructions: "Inspect input\n", Scope: domain.ScopeGlobal, Status: domain.StatusDraft,
	}
	if err = store.CreateSkill(ctx, &skill, "initial", nil); err != nil {
		t.Fatal(err)
	}

	reviewer := historyReviewer{}
	_, err = (codereview.History{Reviews: index}).ReviewAndRecord(ctx, reviewer, codereview.Request{
		SkillID: skill.ID, PackageHash: skill.PackageHash, PackageName: skill.Slug,
	})
	if err != nil {
		t.Fatal(err)
	}
	reviews, err := index.ListSecurityReviews(ctx, skill.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 1 || reviews[0].Outdated || reviews[0].Provider != "local" || reviews[0].Model != "review-model" || len(reviews[0].Findings) == 0 || reviews[0].ReviewedAt.IsZero() {
		t.Fatalf("unexpected current review: %#v", reviews)
	}

	skill.Instructions = "Inspect input and output\n"
	if err = store.UpdateSkill(ctx, &skill, "change package", nil); err != nil {
		t.Fatal(err)
	}
	reviews, err = index.ListSecurityReviews(ctx, skill.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reviews) != 1 || !reviews[0].Outdated || reviews[0].PackageHash == skill.PackageHash {
		t.Fatalf("review was not invalidated after hash change: review=%#v skill_hash=%s", reviews, skill.PackageHash)
	}
}

type historyReviewer struct{}

func (historyReviewer) Name() string                        { return "local" }
func (historyReviewer) Destination() codereview.Destination { return codereview.Destination{} }
func (historyReviewer) Review(context.Context, codereview.Request) (codereview.Result, error) {
	return codereview.Result{
		Provider: "local", Model: "review-model", Summary: "reviewed",
		Findings:       []codereview.Finding{{Severity: "low", Category: "quality", Title: "Example"}},
		Recommendation: "approve",
	}, nil
}
