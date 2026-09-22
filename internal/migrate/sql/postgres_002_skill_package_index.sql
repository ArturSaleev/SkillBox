ALTER TABLE skills ADD COLUMN package_path TEXT NULL;
ALTER TABLE skills ADD COLUMN package_hash VARCHAR(64) NULL;
ALTER TABLE skills ADD COLUMN package_indexed_at VARCHAR(40) NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_skills_package_path ON skills(package_path) WHERE package_path IS NOT NULL;
