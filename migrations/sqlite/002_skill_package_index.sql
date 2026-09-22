ALTER TABLE skills ADD COLUMN package_path TEXT NULL;
ALTER TABLE skills ADD COLUMN package_hash TEXT NULL;
ALTER TABLE skills ADD COLUMN package_indexed_at TEXT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_skills_package_path ON skills(package_path) WHERE package_path IS NOT NULL;
