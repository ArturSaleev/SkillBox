# Database

Each supported database has one fresh schema file:

- `migrations/sqlite/001_initial.sql`
- `migrations/mysql/001_initial.sql`
- `migrations/postgres/001_initial.sql`

The matching embedded copies under `internal/migrate/sql` are verified byte-for-byte by tests. The schema is applied automatically at startup.

The database contains only project and Skill business data: projects/workspace scope, Skills and their structured components, versions, execution telemetry, trajectories, and publication proposals. It does not contain MCP profiles, connections, credentials, or API keys.

The logical groups are:

- scope: `workspaces`, `projects`;
- Skill definition: `skills`, metadata tables, steps, tools, context requirements, dependencies, examples;
- history: `skill_versions`, `skill_proposals`;
- evidence: `skill_executions`, `execution_events`.

SQLite runs in WAL mode. Keep the database file and its `-wal`/`-shm` sidecars together when copying a live database, or stop SkillBox before taking a file-level backup.

MySQL and PostgreSQL use `database.dsn`; SQLite uses `database.path`. The unused field may remain empty. SkillBox opens the configured database and runs migrations before accepting requests.

This compacted initial schema is intended for a new empty database. If an older development database is no longer needed, stop SkillBox and remove that database yourself before starting the new build. SkillBox does not delete database files or user data automatically.
