ALTER TABLE skills ADD COLUMN package_path VARCHAR(512) NULL;
ALTER TABLE skills ADD COLUMN package_hash VARCHAR(64) NULL;
ALTER TABLE skills ADD COLUMN package_indexed_at VARCHAR(40) NULL;
CREATE UNIQUE INDEX idx_skills_package_path ON skills(package_path);
