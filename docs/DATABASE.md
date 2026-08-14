# Database

The schema is normalized and UUIDs are generated in the application. Status, scope, dependency type, and requirement type are strings validated by the domain layer; there are no database-specific enums.

Core tables:

- `workspaces`, `projects`, `skills`
- `skill_domains`, `skill_intents`, `skill_object_types`, `skill_tags`, `skill_keywords`, `skill_capabilities`, `skill_compatibility`
- `skill_steps`, `skill_tools`, `skill_context_requirements`, `skill_dependencies`, `skill_examples`
- `skill_versions` with portable JSON text snapshots
- `skill_executions` for per-Skill and per-model analytics
- `execution_events` for ordered execution trajectories
- `mcp_profiles`, `mcp_connections` for capability and scope isolation
- `skill_proposals` for review-controlled publication

User-scoped Skills require `owner_user_id`. Callers must derive it from trusted authenticated runtime identity, never from model-generated identity claims. MCP credentials are stored only as SHA-256 hashes; REST-generated keys contain 256 bits of randomness and user-supplied keys must be at least 32 characters.

Migrations live in `migrations/sqlite`, `migrations/mysql`, and `migrations/postgres`. Embedded runtime copies under `internal/migrate/sql` are verified byte-for-byte by tests. Migrations run transactionally at startup when `database.migrate` is true; tables are never inferred from Go structs.

SQLite enables foreign keys, WAL, a five-second busy timeout, and one writer connection. MySQL 8.4 and PostgreSQL 17 are the Compose-tested target families. Repository contract tests are identical across drivers; external DSNs must point to disposable dedicated test databases.
