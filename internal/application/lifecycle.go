package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aibox/skillbox/internal/domain"
	"github.com/aibox/skillbox/internal/ports"
)

func StudentProfile() domain.MCPProfile {
	return domain.MCPProfile{Slug: "student", Name: "Student", Permissions: []string{domain.PermissionSkillSearch, domain.PermissionSkillPrepare, domain.PermissionExecutionReport}, Tools: []string{domain.ToolSearchSkills, domain.ToolPrepareSkill, domain.ToolReportSkillResult}}
}

func TeacherProfile() domain.MCPProfile {
	return domain.MCPProfile{
		Slug: "teacher",
		Name: "Teacher",
		Permissions: []string{
			domain.PermissionSkillRead, domain.PermissionSkillSearch, domain.PermissionSkillCreate,
			domain.PermissionSkillUpdate, domain.PermissionSkillValidate, domain.PermissionSkillPropose,
			domain.PermissionSkillVersionCreate, domain.PermissionSkillPublish, domain.PermissionSkillRollback,
			domain.PermissionStatisticsRead, domain.PermissionExecutionRead, domain.PermissionExecutionTrajectoryRead,
		},
		Tools: []string{
			domain.ToolSearchSkills, domain.ToolGetSkill, domain.ToolCreateSkillDraft, domain.ToolUpdateSkillDraft,
			domain.ToolValidateSkill, domain.ToolCreateSkillProposal, domain.ToolCreateSkillVersion,
			domain.ToolGetSkillStatistics, domain.ToolListRecentExecutions, domain.ToolGetExecution,
			domain.ToolGetExecutionTrajectory, domain.ToolGetSkillSuccesses, domain.ToolGetSkillFailures,
			domain.ToolGetSkillProposal, domain.ToolListSkillProposals, domain.ToolApproveSkillProposal,
			domain.ToolRejectSkillProposal, domain.ToolPublishSkill, domain.ToolRollbackSkillVersion,
		},
	}
}

func Visible(access domain.MCPAccess, skill *domain.Skill) bool {
	switch skill.Scope {
	case domain.ScopeGlobal:
		return true
	case domain.ScopeWorkspace:
		return access.Scope.WorkspaceID != nil && skill.WorkspaceID != nil && *access.Scope.WorkspaceID == *skill.WorkspaceID
	case domain.ScopeProject:
		return access.Scope.WorkspaceID != nil && skill.WorkspaceID != nil && *access.Scope.WorkspaceID == *skill.WorkspaceID && access.Scope.ProjectID != nil && skill.ProjectID != nil && *access.Scope.ProjectID == *skill.ProjectID
	default:
		return false
	}
}

func ApplyScope(access domain.MCPAccess, filter *domain.SearchFilter) {
	filter.WorkspaceID = access.Scope.WorkspaceID
	filter.ProjectID = access.Scope.ProjectID
}

func ApplyPrepareScope(access domain.MCPAccess, request *domain.PrepareRequest) {
	request.WorkspaceID = access.Scope.WorkspaceID
	request.ProjectID = access.Scope.ProjectID
}

func ApplySkillScope(access domain.MCPAccess, skill *domain.Skill) {
	if access.Scope.ProjectID != nil {
		skill.Scope = domain.ScopeProject
		skill.WorkspaceID = access.Scope.WorkspaceID
		skill.ProjectID = access.Scope.ProjectID
	} else if access.Scope.WorkspaceID != nil {
		skill.Scope = domain.ScopeWorkspace
		skill.WorkspaceID = access.Scope.WorkspaceID
		skill.ProjectID = nil
	}
}

func (s *Service) GetVisibleSkill(ctx context.Context, access domain.MCPAccess, id string) (*domain.Skill, error) {
	skill, err := s.Store.GetSkill(ctx, id)
	if err != nil {
		return nil, err
	}
	if !Visible(access, skill) {
		return nil, ports.ErrNotFound
	}
	return skill, nil
}
func (s *Service) CreateDraft(ctx context.Context, access domain.MCPAccess, skill *domain.Skill) (*domain.Skill, error) {
	skill.ID = ""
	skill.Status = domain.StatusDraft
	ApplySkillScope(access, skill)
	actor := access.Scope.ActorID
	if err := s.Store.CreateSkill(ctx, skill, "teacher draft", &actor); err != nil {
		return nil, err
	}
	return skill, nil
}
func (s *Service) UpdateDraft(ctx context.Context, access domain.MCPAccess, skill *domain.Skill, summary string) (*domain.Skill, error) {
	existing, err := s.GetVisibleSkill(ctx, access, skill.ID)
	if err != nil {
		return nil, err
	}
	if existing.Status != domain.StatusDraft {
		return nil, errors.New("only draft skills can be updated by teacher")
	}
	skill.Status = domain.StatusDraft
	skill.Scope = existing.Scope
	skill.WorkspaceID = existing.WorkspaceID
	skill.ProjectID = existing.ProjectID
	actor := access.Scope.ActorID
	if err = s.Store.UpdateSkill(ctx, skill, summary, &actor); err != nil {
		return nil, err
	}
	return skill, nil
}
func (s *Service) ValidateVisibleSkill(ctx context.Context, access domain.MCPAccess, id string) ([]string, error) {
	skill, err := s.GetVisibleSkill(ctx, access, id)
	if err != nil {
		return nil, err
	}
	var issues []string
	if err = skill.Validate(); err != nil {
		issues = append(issues, err.Error())
	}
	if len(skill.Steps) == 0 {
		issues = append(issues, "at least one step is recommended")
	}
	if len(skill.SuccessCriteria) == 0 {
		issues = append(issues, "success_criteria is required")
	}
	for _, dep := range skill.Dependencies {
		if _, err = s.Compiler.Compile(ctx, skill, domain.PrepareRequest{Task: "validation", MaxSkillTokens: 800}); err != nil {
			issues = append(issues, fmt.Sprintf("dependency %s: %v", dep.DependsOnSkillID, err))
			break
		}
	}
	return issues, nil
}
func (s *Service) CreateProposal(ctx context.Context, access domain.MCPAccess, skillID, summary string) (*domain.SkillProposal, error) {
	skill, err := s.GetVisibleSkill(ctx, access, skillID)
	if err != nil {
		return nil, err
	}
	if skill.Status != domain.StatusDraft {
		return nil, errors.New("proposal source must be a draft")
	}
	raw, err := json.Marshal(skill)
	if err != nil {
		return nil, err
	}
	actor := access.Scope.ActorID
	p := &domain.SkillProposal{SkillID: skill.ID, BaseVersion: skill.CurrentVersion, ProposedSnapshot: string(raw), Summary: summary, Status: "pending", CreatedBy: &actor}
	if err = s.Store.CreateSkillProposal(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}
func (s *Service) PublishProposal(ctx context.Context, access domain.MCPAccess, proposalID string) (*domain.Skill, error) {
	p, err := s.Store.GetSkillProposal(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	if p.Status != "approved" {
		return nil, errors.New("proposal must be approved before publish")
	}
	current, err := s.GetVisibleSkill(ctx, access, p.SkillID)
	if err != nil {
		return nil, err
	}
	return s.publishProposal(ctx, p, current, access.Scope.ActorID)
}

func (s *Service) PublishProposalAdmin(ctx context.Context, proposalID, actor string) (*domain.Skill, error) {
	p, err := s.Store.GetSkillProposal(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	if p.Status != "approved" {
		return nil, errors.New("proposal must be approved before publish")
	}
	current, err := s.Store.GetSkill(ctx, p.SkillID)
	if err != nil {
		return nil, err
	}
	return s.publishProposal(ctx, p, current, actor)
}

func (s *Service) publishProposal(ctx context.Context, p *domain.SkillProposal, current *domain.Skill, actorID string) (*domain.Skill, error) {
	if current.CurrentVersion != p.BaseVersion {
		return nil, errors.New("proposal base version is stale")
	}
	var skill domain.Skill
	if err := json.Unmarshal([]byte(p.ProposedSnapshot), &skill); err != nil {
		return nil, err
	}
	skill.Status = domain.StatusActive
	if err := s.Store.UpdateSkill(ctx, &skill, "publish approved proposal", &actorID); err != nil {
		return nil, err
	}
	if err := s.Store.MarkSkillProposalPublished(ctx, p.ID); err != nil {
		return nil, err
	}
	return &skill, nil
}
func (s *Service) ScopedExecutions(ctx context.Context, access domain.MCPAccess, skillID *string, status string, limit int) ([]domain.Execution, error) {
	items, err := s.Store.ListExecutions(ctx, skillID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Execution, 0)
	for _, e := range items {
		skill, err := s.Store.GetSkill(ctx, e.SkillID)
		if err != nil || !Visible(access, skill) {
			continue
		}
		if status != "" && e.Status != status {
			continue
		}
		e.Trajectory = nil
		out = append(out, e)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}
