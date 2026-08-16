package application

import (
	"github.com/aibox/skillbox/internal/domain"
	"testing"
)

func TestProjectConnectionSeesWorkspaceAndOwnProjectOnly(t *testing.T) {
	t.Parallel()
	ws, project, other := "workspace-a", "project-a", "project-b"
	access := domain.MCPAccess{Scope: domain.MCPScope{WorkspaceID: &ws, ProjectID: &project}}
	workspaceSkill := domain.Skill{Scope: domain.ScopeWorkspace, WorkspaceID: &ws}
	own := domain.Skill{Scope: domain.ScopeProject, WorkspaceID: &ws, ProjectID: &project}
	foreign := domain.Skill{Scope: domain.ScopeProject, WorkspaceID: &ws, ProjectID: &other}
	if !Visible(access, &workspaceSkill) || !Visible(access, &own) || Visible(access, &foreign) {
		t.Fatal("project connection scope visibility is incorrect")
	}
}
