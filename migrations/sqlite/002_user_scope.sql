ALTER TABLE skills ADD COLUMN owner_user_id TEXT NULL;
DROP INDEX IF EXISTS idx_skills_slug_scope;
CREATE UNIQUE INDEX IF NOT EXISTS idx_skills_slug_scope ON skills(slug, IFNULL(workspace_id, ''), IFNULL(project_id, ''), IFNULL(owner_user_id, ''));
CREATE INDEX IF NOT EXISTS idx_skills_owner_user ON skills(owner_user_id, status, priority);
