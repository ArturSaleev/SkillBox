ALTER TABLE skills ADD COLUMN owner_user_id VARCHAR(191) NULL;
DROP INDEX IF EXISTS idx_skills_global_slug;
DROP INDEX IF EXISTS idx_skills_scoped_slug;
CREATE UNIQUE INDEX IF NOT EXISTS idx_skills_global_slug ON skills(slug) WHERE scope = 'global';
CREATE UNIQUE INDEX IF NOT EXISTS idx_skills_workspace_slug ON skills(slug, workspace_id) WHERE scope = 'workspace';
CREATE UNIQUE INDEX IF NOT EXISTS idx_skills_project_slug ON skills(slug, workspace_id, project_id) WHERE scope = 'project';
CREATE UNIQUE INDEX IF NOT EXISTS idx_skills_user_slug ON skills(slug, owner_user_id) WHERE scope = 'user';
CREATE INDEX IF NOT EXISTS idx_skills_owner_user ON skills(owner_user_id, status, priority);
