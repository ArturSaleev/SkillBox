package domain

import "time"

const (
	PermissionSkillRead               = "skill.read"
	PermissionSkillSearch             = "skill.search"
	PermissionSkillPrepare            = "skill.prepare"
	PermissionSkillCreate             = "skill.create"
	PermissionSkillUpdate             = "skill.update"
	PermissionSkillValidate           = "skill.validate"
	PermissionSkillPropose            = "skill.propose"
	PermissionSkillVersionCreate      = "skill.version.create"
	PermissionSkillPublish            = "skill.publish"
	PermissionSkillDeprecate          = "skill.deprecate"
	PermissionSkillRollback           = "skill.rollback"
	PermissionExecutionReport         = "execution.report"
	PermissionExecutionRead           = "execution.read"
	PermissionExecutionTrajectoryRead = "execution.trajectory.read"
	PermissionStatisticsRead          = "statistics.read"
)

func ToolPermission(tool string) (string, bool) {
	permissions := map[string]string{
		ToolSearchSkills: PermissionSkillSearch, ToolGetSkill: PermissionSkillRead, ToolPrepareSkill: PermissionSkillPrepare, ToolReportSkillResult: PermissionExecutionReport,
		ToolCreateSkillDraft: PermissionSkillCreate, ToolUpdateSkillDraft: PermissionSkillUpdate, ToolValidateSkill: PermissionSkillValidate, ToolCreateSkillProposal: PermissionSkillPropose, ToolCreateSkillVersion: PermissionSkillVersionCreate,
		ToolGetSkillStatistics: PermissionStatisticsRead, ToolListRecentExecutions: PermissionExecutionRead, ToolGetExecution: PermissionExecutionRead, ToolGetExecutionTrajectory: PermissionExecutionTrajectoryRead, ToolGetSkillSuccesses: PermissionExecutionRead, ToolGetSkillFailures: PermissionExecutionRead,
		ToolGetSkillProposal: PermissionSkillRead, ToolListSkillProposals: PermissionSkillRead, ToolApproveSkillProposal: PermissionSkillPublish, ToolRejectSkillProposal: PermissionSkillPublish, ToolPublishSkill: PermissionSkillPublish, ToolRollbackSkillVersion: PermissionSkillRollback,
	}
	permission, ok := permissions[tool]
	return permission, ok
}

const (
	ToolSearchSkills           = "search_skills"
	ToolGetSkill               = "get_skill"
	ToolPrepareSkill           = "prepare_skill"
	ToolReportSkillResult      = "report_skill_result"
	ToolCreateSkillDraft       = "create_skill_draft"
	ToolUpdateSkillDraft       = "update_skill_draft"
	ToolValidateSkill          = "validate_skill"
	ToolCreateSkillProposal    = "create_skill_proposal"
	ToolCreateSkillVersion     = "create_skill_version"
	ToolGetSkillStatistics     = "get_skill_statistics"
	ToolListRecentExecutions   = "list_recent_executions"
	ToolGetExecution           = "get_execution"
	ToolGetExecutionTrajectory = "get_execution_trajectory"
	ToolGetSkillSuccesses      = "get_skill_successes"
	ToolGetSkillFailures       = "get_skill_failures"
	ToolGetSkillProposal       = "get_skill_proposal"
	ToolListSkillProposals     = "list_skill_proposals"
	ToolApproveSkillProposal   = "approve_skill_proposal"
	ToolRejectSkillProposal    = "reject_skill_proposal"
	ToolPublishSkill           = "publish_skill"
	ToolRollbackSkillVersion   = "rollback_skill_version"
)

type MCPProfile struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	Tools       []string `json:"tools"`
}

func (p MCPProfile) Allows(permission string) bool {
	for _, value := range p.Permissions {
		if value == permission {
			return true
		}
	}
	return false
}

func (p MCPProfile) AllowsTool(name string) bool {
	for _, value := range p.Tools {
		if value == name {
			return true
		}
	}
	return false
}

type MCPScope struct {
	ActorID     string  `json:"actor_id"`
	WorkspaceID *string `json:"workspace_id,omitempty"`
	ProjectID   *string `json:"project_id,omitempty"`
}

type MCPAccess struct {
	Scope   MCPScope   `json:"scope"`
	Profile MCPProfile `json:"profile"`
}

type SkillProposal struct {
	ID               string     `json:"id"`
	SkillID          string     `json:"skill_id"`
	BaseVersion      int        `json:"base_version"`
	ProposedSnapshot string     `json:"proposed_snapshot,omitempty"`
	Summary          string     `json:"summary"`
	Status           string     `json:"status"`
	CreatedBy        *string    `json:"created_by,omitempty"`
	ReviewedBy       *string    `json:"reviewed_by,omitempty"`
	ReviewNote       *string    `json:"review_note,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
}

type ExecutionEvent struct {
	ID          string    `json:"id"`
	ExecutionID string    `json:"execution_id,omitempty"`
	Position    int       `json:"position"`
	Type        string    `json:"type"`
	Data        string    `json:"data,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
}
