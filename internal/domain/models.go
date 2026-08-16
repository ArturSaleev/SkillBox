package domain

import "time"

type Scope string

const (
	ScopeGlobal    Scope = "global"
	ScopeWorkspace Scope = "workspace"
	ScopeProject   Scope = "project"
)

type SkillStatus string

const (
	StatusDraft      SkillStatus = "draft"
	StatusActive     SkillStatus = "active"
	StatusDeprecated SkillStatus = "deprecated"
	StatusArchived   SkillStatus = "archived"
)

type Workspace struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Project struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	ExternalID  string     `json:"external_id,omitempty"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	AutoCreated bool       `json:"auto_created"`
	Workspace   *Workspace `json:"workspace,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Step struct {
	ID             string    `json:"id"`
	SkillID        string    `json:"skill_id,omitempty"`
	Position       int       `json:"position"`
	Title          string    `json:"title"`
	Instruction    string    `json:"instruction"`
	Condition      string    `json:"condition,omitempty"`
	Required       bool      `json:"is_required"`
	ExpectedResult string    `json:"expected_result,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

type ToolRequirement struct {
	ID          string `json:"id"`
	SkillID     string `json:"skill_id,omitempty"`
	Name        string `json:"tool_name"`
	Namespace   string `json:"tool_namespace,omitempty"`
	Requirement string `json:"requirement"`
	Purpose     string `json:"purpose,omitempty"`
	UsageHint   string `json:"usage_hint,omitempty"`
}

type ContextRequirement struct {
	ID          string `json:"id"`
	SkillID     string `json:"skill_id,omitempty"`
	Type        string `json:"type"`
	Query       string `json:"query"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
	Priority    int    `json:"priority"`
	MaxTokens   *int   `json:"max_tokens,omitempty"`
}

type Dependency struct {
	ID               string `json:"id"`
	SkillID          string `json:"skill_id,omitempty"`
	DependsOnSkillID string `json:"depends_on_skill_id"`
	Type             string `json:"type"`
	Position         int    `json:"position"`
}

type Example struct {
	ID               string `json:"id"`
	SkillID          string `json:"skill_id,omitempty"`
	Title            string `json:"title"`
	InputExample     string `json:"input_example"`
	ExpectedBehavior string `json:"expected_behavior"`
	BadBehavior      string `json:"bad_behavior,omitempty"`
	Priority         int    `json:"priority"`
}

type Skill struct {
	ID              string               `json:"id"`
	WorkspaceID     *string              `json:"workspace_id,omitempty"`
	ProjectID       *string              `json:"project_id,omitempty"`
	Slug            string               `json:"slug"`
	Name            string               `json:"name"`
	Description     string               `json:"description"`
	Purpose         string               `json:"purpose"`
	WhenToUse       string               `json:"when_to_use"`
	WhenNotToUse    string               `json:"when_not_to_use,omitempty"`
	Instructions    string               `json:"instructions"`
	SuccessCriteria []string             `json:"success_criteria"`
	Scope           Scope                `json:"scope"`
	Status          SkillStatus          `json:"status"`
	Priority        int                  `json:"priority"`
	CurrentVersion  int                  `json:"current_version"`
	Domains         []string             `json:"domains"`
	Intents         []string             `json:"intents"`
	ObjectTypes     []string             `json:"object_types"`
	Tags            []string             `json:"tags"`
	Keywords        []string             `json:"keywords"`
	Capabilities    []string             `json:"capabilities"`
	Compatibility   []string             `json:"compatibility"`
	Steps           []Step               `json:"steps"`
	Tools           []ToolRequirement    `json:"tools"`
	Contexts        []ContextRequirement `json:"context_requirements"`
	Dependencies    []Dependency         `json:"dependencies"`
	Examples        []Example            `json:"examples"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
}

type SkillVersion struct {
	ID            string    `json:"id"`
	SkillID       string    `json:"skill_id"`
	Version       int       `json:"version"`
	Snapshot      string    `json:"snapshot,omitempty"`
	ChangeSummary string    `json:"change_summary"`
	CreatedBy     *string   `json:"created_by,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type Execution struct {
	ID             string           `json:"id"`
	SkillID        string           `json:"skill_id"`
	SkillVersion   int              `json:"skill_version"`
	WorkspaceID    *string          `json:"workspace_id,omitempty"`
	ProjectID      *string          `json:"project_id,omitempty"`
	AgentID        *string          `json:"agent_id,omitempty"`
	ModelProvider  *string          `json:"model_provider,omitempty"`
	ModelName      *string          `json:"model_name,omitempty"`
	TaskSummary    string           `json:"task_summary"`
	TaskHash       *string          `json:"task_hash,omitempty"`
	StartedAt      time.Time        `json:"started_at"`
	FinishedAt     *time.Time       `json:"finished_at,omitempty"`
	DurationMS     *int64           `json:"duration_ms,omitempty"`
	Status         string           `json:"status"`
	Success        bool             `json:"success"`
	ToolCallsCount *int             `json:"tool_calls_count,omitempty"`
	InputTokens    *int             `json:"input_tokens,omitempty"`
	OutputTokens   *int             `json:"output_tokens,omitempty"`
	ErrorType      *string          `json:"error_type,omitempty"`
	ErrorMessage   *string          `json:"error_message,omitempty"`
	Feedback       *string          `json:"feedback,omitempty"`
	Trajectory     []ExecutionEvent `json:"trajectory,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
}

type ModelInfo struct {
	Provider      string `json:"provider,omitempty"`
	Name          string `json:"name,omitempty"`
	ContextWindow int    `json:"context_window,omitempty"`
}

type SearchFilter struct {
	Task           string   `json:"task,omitempty"`
	WorkspaceID    *string  `json:"workspace_id,omitempty"`
	ProjectID      *string  `json:"project_id,omitempty"`
	Scopes         []Scope  `json:"scopes,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	Intents        []string `json:"intents,omitempty"`
	ObjectTypes    []string `json:"object_types,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	Status         string   `json:"status,omitempty"`
	RequiredTool   string   `json:"required_tool,omitempty"`
	AvailableTools []string `json:"available_tools,omitempty"`
	Keywords       []string `json:"keywords,omitempty"`
	Limit          int      `json:"limit,omitempty"`
}

type Candidate struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Purpose     string   `json:"purpose"`
	Domains     []string `json:"domains"`
	Intents     []string `json:"intents"`
	ObjectTypes []string `json:"object_types"`
	Score       int      `json:"score"`
	Version     int      `json:"version"`
}

type PrepareRequest struct {
	Task           string    `json:"task"`
	WorkspaceID    *string   `json:"workspace_id,omitempty"`
	ProjectID      *string   `json:"project_id,omitempty"`
	SkillID        *string   `json:"skill_id,omitempty"`
	Domains        []string  `json:"domains,omitempty"`
	Intents        []string  `json:"intents,omitempty"`
	ObjectTypes    []string  `json:"object_types,omitempty"`
	AvailableTools []string  `json:"available_tools,omitempty"`
	Model          ModelInfo `json:"model"`
	MaxSkillTokens int       `json:"max_skill_tokens,omitempty"`
}

type CompiledStep struct {
	Title       string `json:"title"`
	Instruction string `json:"instruction"`
	Required    bool   `json:"required"`
}

type CompiledSkill struct {
	Instructions        string               `json:"instructions"`
	Steps               []CompiledStep       `json:"steps"`
	RequiredTools       []ToolRequirement    `json:"required_tools"`
	OptionalTools       []ToolRequirement    `json:"optional_tools"`
	MissingTools        []string             `json:"missing_tools,omitempty"`
	ContextRequirements []ContextRequirement `json:"context_requirements"`
	SuccessCriteria     []string             `json:"success_criteria"`
	Examples            []Example            `json:"examples,omitempty"`
}

type PreparedSkill struct {
	SkillID         string        `json:"skill_id"`
	Version         int           `json:"version"`
	Name            string        `json:"name"`
	CompiledSkill   CompiledSkill `json:"compiled_skill"`
	EstimatedTokens int           `json:"estimated_tokens"`
}

type Statistics struct {
	SkillID     string           `json:"skill_id"`
	Runs        int              `json:"runs"`
	Successes   int              `json:"successes"`
	SuccessRate float64          `json:"success_rate"`
	ByModel     []ModelStatistic `json:"by_model"`
}

type ModelStatistic struct {
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	Runs        int     `json:"runs"`
	Successes   int     `json:"successes"`
	SuccessRate float64 `json:"success_rate"`
}
