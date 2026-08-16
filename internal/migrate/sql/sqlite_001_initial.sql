CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS workspaces (
  id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS projects (
  id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, external_id TEXT NULL, slug TEXT NOT NULL, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', auto_created INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
  UNIQUE(workspace_id, slug)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_workspace_external_id ON projects(workspace_id, external_id) WHERE external_id IS NOT NULL;
CREATE TABLE IF NOT EXISTS skills (
  id TEXT PRIMARY KEY, workspace_id TEXT NULL REFERENCES workspaces(id), project_id TEXT NULL REFERENCES projects(id), slug TEXT NOT NULL, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', purpose TEXT NOT NULL DEFAULT '', when_to_use TEXT NOT NULL DEFAULT '', when_not_to_use TEXT NOT NULL DEFAULT '', instructions TEXT NOT NULL DEFAULT '', success_criteria TEXT NOT NULL DEFAULT '[]', scope TEXT NOT NULL, status TEXT NOT NULL, priority INTEGER NOT NULL DEFAULT 0, current_version INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_skills_slug_scope ON skills(slug, IFNULL(workspace_id, ''), IFNULL(project_id, ''));
CREATE INDEX IF NOT EXISTS idx_skills_lookup ON skills(status, scope, workspace_id, project_id, priority);
CREATE TABLE IF NOT EXISTS skill_domains (skill_id TEXT NOT NULL REFERENCES skills(id) ON DELETE CASCADE, value TEXT NOT NULL, PRIMARY KEY(skill_id,value));
CREATE TABLE IF NOT EXISTS skill_intents (skill_id TEXT NOT NULL REFERENCES skills(id) ON DELETE CASCADE, value TEXT NOT NULL, PRIMARY KEY(skill_id,value));
CREATE TABLE IF NOT EXISTS skill_object_types (skill_id TEXT NOT NULL REFERENCES skills(id) ON DELETE CASCADE, value TEXT NOT NULL, PRIMARY KEY(skill_id,value));
CREATE TABLE IF NOT EXISTS skill_tags (skill_id TEXT NOT NULL REFERENCES skills(id) ON DELETE CASCADE, value TEXT NOT NULL, PRIMARY KEY(skill_id,value));
CREATE TABLE IF NOT EXISTS skill_keywords (skill_id TEXT NOT NULL REFERENCES skills(id) ON DELETE CASCADE, value TEXT NOT NULL, PRIMARY KEY(skill_id,value));
CREATE TABLE IF NOT EXISTS skill_capabilities (skill_id TEXT NOT NULL REFERENCES skills(id) ON DELETE CASCADE, value TEXT NOT NULL, PRIMARY KEY(skill_id,value));
CREATE TABLE IF NOT EXISTS skill_compatibility (skill_id TEXT NOT NULL REFERENCES skills(id) ON DELETE CASCADE, value TEXT NOT NULL, PRIMARY KEY(skill_id,value));
CREATE TABLE IF NOT EXISTS skill_steps (
  id TEXT PRIMARY KEY, skill_id TEXT NOT NULL REFERENCES skills(id) ON DELETE CASCADE, position INTEGER NOT NULL, title TEXT NOT NULL, instruction TEXT NOT NULL, condition_text TEXT NULL, is_required INTEGER NOT NULL, expected_result TEXT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, UNIQUE(skill_id,position)
);
CREATE TABLE IF NOT EXISTS skill_tools (
  id TEXT PRIMARY KEY, skill_id TEXT NOT NULL REFERENCES skills(id) ON DELETE CASCADE, tool_name TEXT NOT NULL, tool_namespace TEXT NULL, requirement TEXT NOT NULL, purpose TEXT NOT NULL DEFAULT '', usage_hint TEXT NULL
);
CREATE TABLE IF NOT EXISTS skill_context_requirements (
  id TEXT PRIMARY KEY, skill_id TEXT NOT NULL REFERENCES skills(id) ON DELETE CASCADE, type TEXT NOT NULL, query_text TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', required INTEGER NOT NULL, priority INTEGER NOT NULL DEFAULT 0, max_tokens INTEGER NULL
);
CREATE TABLE IF NOT EXISTS skill_dependencies (
  id TEXT PRIMARY KEY, skill_id TEXT NOT NULL REFERENCES skills(id) ON DELETE CASCADE, depends_on_skill_id TEXT NOT NULL REFERENCES skills(id), type TEXT NOT NULL, position INTEGER NOT NULL DEFAULT 0, UNIQUE(skill_id,depends_on_skill_id,type)
);
CREATE TABLE IF NOT EXISTS skill_examples (
  id TEXT PRIMARY KEY, skill_id TEXT NOT NULL REFERENCES skills(id) ON DELETE CASCADE, title TEXT NOT NULL, input_example TEXT NOT NULL, expected_behavior TEXT NOT NULL, bad_behavior TEXT NULL, priority INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS skill_versions (
  id TEXT PRIMARY KEY, skill_id TEXT NOT NULL REFERENCES skills(id) ON DELETE CASCADE, version INTEGER NOT NULL, snapshot TEXT NOT NULL, change_summary TEXT NOT NULL DEFAULT '', created_by TEXT NULL, created_at TEXT NOT NULL, UNIQUE(skill_id,version)
);
CREATE TABLE IF NOT EXISTS skill_executions (
  id TEXT PRIMARY KEY, skill_id TEXT NOT NULL REFERENCES skills(id), skill_version INTEGER NOT NULL, workspace_id TEXT NULL, project_id TEXT NULL, agent_id TEXT NULL, model_provider TEXT NULL, model_name TEXT NULL, task_summary TEXT NOT NULL, task_hash TEXT NULL, started_at TEXT NOT NULL, finished_at TEXT NULL, duration_ms INTEGER NULL, status TEXT NOT NULL, success INTEGER NOT NULL, tool_calls_count INTEGER NULL, input_tokens INTEGER NULL, output_tokens INTEGER NULL, error_type TEXT NULL, error_message TEXT NULL, feedback TEXT NULL, created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_executions_skill_model ON skill_executions(skill_id, model_provider, model_name, success);
CREATE TABLE IF NOT EXISTS skill_proposals (
  id TEXT PRIMARY KEY, skill_id TEXT NOT NULL REFERENCES skills(id), base_version INTEGER NOT NULL, proposed_snapshot TEXT NOT NULL, summary TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, created_by TEXT NULL, reviewed_by TEXT NULL, review_note TEXT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, reviewed_at TEXT NULL
);
CREATE INDEX IF NOT EXISTS idx_skill_proposals_status ON skill_proposals(status, skill_id, created_at);
CREATE TABLE IF NOT EXISTS execution_events (
  id TEXT PRIMARY KEY, execution_id TEXT NOT NULL REFERENCES skill_executions(id) ON DELETE CASCADE, position INTEGER NOT NULL, event_type TEXT NOT NULL, event_data TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, UNIQUE(execution_id, position)
);
