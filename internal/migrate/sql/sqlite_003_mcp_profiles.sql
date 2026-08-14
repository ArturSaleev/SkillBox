CREATE TABLE IF NOT EXISTS mcp_profiles (
  id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', permissions TEXT NOT NULL DEFAULT '[]', tools TEXT NOT NULL DEFAULT '[]', built_in INTEGER NOT NULL, enabled INTEGER NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS mcp_connections (
  id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, workspace_id TEXT NULL REFERENCES workspaces(id), project_id TEXT NULL REFERENCES projects(id), profile_id TEXT NOT NULL REFERENCES mcp_profiles(id), auth_type TEXT NOT NULL, credential_hash TEXT NOT NULL UNIQUE, enabled INTEGER NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, last_used_at TEXT NULL
);
CREATE INDEX IF NOT EXISTS idx_mcp_connections_scope ON mcp_connections(workspace_id, project_id, enabled);
CREATE TABLE IF NOT EXISTS skill_proposals (
  id TEXT PRIMARY KEY, skill_id TEXT NOT NULL REFERENCES skills(id), base_version INTEGER NOT NULL, proposed_snapshot TEXT NOT NULL, summary TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, created_by TEXT NULL, reviewed_by TEXT NULL, review_note TEXT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, reviewed_at TEXT NULL
);
CREATE INDEX IF NOT EXISTS idx_skill_proposals_status ON skill_proposals(status, skill_id, created_at);
CREATE TABLE IF NOT EXISTS execution_events (
  id TEXT PRIMARY KEY, execution_id TEXT NOT NULL REFERENCES skill_executions(id) ON DELETE CASCADE, position INTEGER NOT NULL, event_type TEXT NOT NULL, event_data TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, UNIQUE(execution_id, position)
);
