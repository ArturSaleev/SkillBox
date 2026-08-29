export type Scope = "global" | "workspace" | "project";
export type SkillStatus = "draft" | "active" | "deprecated" | "archived";

export interface Workspace { id: string; slug: string; name: string; description?: string; created_at: string; updated_at: string }
export interface Project { id: string; workspace_id: string; external_id?: string; slug: string; name: string; description?: string; auto_created: boolean; workspace?: Workspace; created_at: string; updated_at: string }
export interface Step { id: string; skill_id?: string; position: number; title: string; instruction: string; condition?: string; is_required: boolean; expected_result?: string; created_at?: string; updated_at?: string }
export interface ToolRequirement { id: string; skill_id?: string; tool_name: string; tool_namespace?: string; requirement: "required" | "optional"; purpose?: string; usage_hint?: string }
export interface ContextRequirement { id: string; skill_id?: string; type: string; query: string; description?: string; required: boolean; priority: number; max_tokens?: number }
export interface Dependency { id: string; skill_id?: string; depends_on_skill_id: string; type: string; position: number }
export interface Example { id: string; skill_id?: string; title: string; input_example: string; expected_behavior: string; bad_behavior?: string; priority: number }

export interface Skill {
  id: string; workspace_id?: string; project_id?: string; slug: string; name: string; description: string;
  purpose: string; when_to_use: string; when_not_to_use?: string; instructions: string; success_criteria: string[];
  scope: Scope; status: SkillStatus; priority: number; current_version: number; domains: string[]; intents: string[];
  object_types: string[]; tags: string[]; keywords: string[]; capabilities: string[]; compatibility: string[];
  steps: Step[]; tools: ToolRequirement[]; context_requirements: ContextRequirement[]; dependencies: Dependency[];
  examples: Example[]; created_at: string; updated_at: string; project?: Project; mcp_project: string;
}

export interface ExecutionEvent { id: string; execution_id?: string; position: number; type: string; data?: string; created_at?: string }
export interface Execution { id: string; skill_id: string; skill_version: number; workspace_id?: string; project_id?: string; agent_id?: string; model_provider?: string; model_name?: string; task_summary: string; task_hash?: string; started_at: string; finished_at?: string; duration_ms?: number; status: string; success: boolean; tool_calls_count?: number; input_tokens?: number; output_tokens?: number; error_type?: string; error_message?: string; feedback?: string; trajectory?: ExecutionEvent[]; created_at: string }
export interface ModelStatistic { provider: string; model: string; runs: number; successes: number; success_rate: number }
export interface Statistics { skill_id: string; runs: number; successes: number; success_rate: number; by_model: ModelStatistic[] }
export interface Candidate { id: string; slug: string; name: string; description: string; purpose: string; domains: string[]; intents: string[]; object_types: string[]; score: number; version: number }
export interface SearchFilter { task?: string; scopes?: Scope[]; domains?: string[]; intents?: string[]; object_types?: string[]; tags?: string[]; status?: SkillStatus; required_tool?: string; available_tools?: string[]; keywords?: string[]; limit?: number }
export interface SkillProposal { id: string; skill_id: string; base_version: number; summary: string; status: "pending" | "approved" | "rejected" | "published"; created_by?: string; reviewed_by?: string; review_note?: string; created_at: string; updated_at: string; reviewed_at?: string }
export interface ValidationResult { valid: boolean; issues: string[] }
export interface CompiledStep { title: string; instruction: string; required: boolean }
export interface CompiledSkill { instructions: string; steps: CompiledStep[]; required_tools: ToolRequirement[]; optional_tools: ToolRequirement[]; missing_tools?: string[]; context_requirements: ContextRequirement[]; success_criteria: string[]; examples?: Example[] }
export interface PreparedSkill { skill_id: string; version: number; name: string; compiled_skill: CompiledSkill; estimated_tokens: number }
export type SkillInput = Omit<Skill, "id" | "workspace_id" | "project_id" | "project" | "current_version" | "created_at" | "updated_at"> & { id?: string; current_version?: number };
