package search

import (
	"github.com/aibox/skillbox/internal/domain"
	"testing"
)

func TestProjectCandidateRanksAboveGlobal(t *testing.T) {
	t.Parallel()
	ws, project := "w", "p"
	global := domain.Skill{ID: "g", Name: "Global", Status: domain.StatusActive, Scope: domain.ScopeGlobal, Domains: []string{"golang"}, Intents: []string{"create"}}
	local := global
	local.ID = "p"
	local.Name = "Project"
	local.Scope = domain.ScopeProject
	local.WorkspaceID = &ws
	local.ProjectID = &project
	items := Candidates([]domain.Skill{global, local}, domain.SearchFilter{WorkspaceID: &ws, ProjectID: &project, Domains: []string{"golang"}, Intents: []string{"create"}}, DefaultScorer(), nil)
	if len(items) != 2 || items[0].ID != "p" {
		t.Fatalf("unexpected ranking: %#v", items)
	}
}
func TestSearchReturnsMetadataWithoutInstructions(t *testing.T) {
	t.Parallel()
	sk := domain.Skill{ID: "1", Name: "Go", Description: "short", Purpose: "create handler", Instructions: "large secret procedure", Status: domain.StatusActive, Scope: domain.ScopeGlobal}
	items := Candidates([]domain.Skill{sk}, domain.SearchFilter{}, DefaultScorer(), nil)
	if len(items) != 1 || items[0].Purpose != "create handler" {
		t.Fatalf("unexpected candidate: %#v", items)
	}
}
