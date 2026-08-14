package ports

import (
	"context"
	"errors"

	"github.com/aibox/skillbox/internal/domain"
)

var ErrNotFound = errors.New("not found")

type WorkspaceRepository interface {
	CreateWorkspace(context.Context, *domain.Workspace) error
	ListWorkspaces(context.Context) ([]domain.Workspace, error)
}

type ProjectRepository interface {
	CreateProject(context.Context, *domain.Project) error
	ListProjects(context.Context, *string) ([]domain.Project, error)
}

type SkillRepository interface {
	CreateSkill(context.Context, *domain.Skill, string, *string) error
	UpdateSkill(context.Context, *domain.Skill, string, *string) error
	GetSkill(context.Context, string) (*domain.Skill, error)
	ListSkills(context.Context) ([]domain.Skill, error)
}

type SkillVersionRepository interface {
	ListVersions(context.Context, string) ([]domain.SkillVersion, error)
	GetVersion(context.Context, string, int) (*domain.SkillVersion, error)
	RollbackSkill(context.Context, string, int, *string) (*domain.Skill, error)
}

type ExecutionRepository interface {
	CreateExecution(context.Context, *domain.Execution) error
	ListExecutions(context.Context, *string) ([]domain.Execution, error)
	GetExecution(context.Context, string) (*domain.Execution, error)
	GetExecutionTrajectory(context.Context, string) ([]domain.ExecutionEvent, error)
	Statistics(context.Context, *string) ([]domain.Statistics, error)
}

type MCPAccessRepository interface {
	UpsertMCPProfile(context.Context, *domain.MCPProfile) error
	GetMCPProfile(context.Context, string) (*domain.MCPProfile, error)
	GetMCPProfileBySlug(context.Context, string) (*domain.MCPProfile, error)
	ListMCPProfiles(context.Context) ([]domain.MCPProfile, error)
	CreateMCPConnection(context.Context, *domain.MCPConnection) error
	GetMCPConnection(context.Context, string) (*domain.MCPConnection, error)
	ListMCPConnections(context.Context) ([]domain.MCPConnection, error)
	ResolveMCPConnection(context.Context, string) (*domain.MCPConnection, error)
	TouchMCPConnection(context.Context, string) error
}

type SkillProposalRepository interface {
	CreateSkillProposal(context.Context, *domain.SkillProposal) error
	GetSkillProposal(context.Context, string) (*domain.SkillProposal, error)
	ListSkillProposals(context.Context, *string, string) ([]domain.SkillProposal, error)
	ReviewSkillProposal(context.Context, string, string, *string, *string) (*domain.SkillProposal, error)
	MarkSkillProposalPublished(context.Context, string) error
}

type Storage interface {
	WorkspaceRepository
	ProjectRepository
	SkillRepository
	SkillVersionRepository
	ExecutionRepository
	MCPAccessRepository
	SkillProposalRepository
	Ping(context.Context) error
	Close() error
}
