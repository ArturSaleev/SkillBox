ALTER TABLE skills ADD COLUMN owner_user_id VARCHAR(191) NULL;
ALTER TABLE skills DROP INDEX uq_skills_slug_scope;
ALTER TABLE skills ADD COLUMN workspace_key VARCHAR(36) GENERATED ALWAYS AS (IFNULL(workspace_id, '')) STORED;
ALTER TABLE skills ADD COLUMN project_key VARCHAR(36) GENERATED ALWAYS AS (IFNULL(project_id, '')) STORED;
ALTER TABLE skills ADD COLUMN owner_user_key VARCHAR(191) GENERATED ALWAYS AS (IFNULL(owner_user_id, '')) STORED;
ALTER TABLE skills ADD UNIQUE KEY uq_skills_slug_scope(slug, workspace_key, project_key, owner_user_key);
CREATE INDEX idx_skills_owner_user ON skills(owner_user_id, status, priority);
