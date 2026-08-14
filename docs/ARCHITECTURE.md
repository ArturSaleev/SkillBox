# Architecture

SkillBox uses a ports-and-adapters layout. `internal/domain` owns portable entities and validation. `internal/application`, `internal/search`, and `internal/compiler` own use cases. They depend only on `internal/ports.Storage`, never on `database/sql`, HTTP, MCP, or a database dialect.

`internal/storage/sqlstore` implements identical repository behavior over standard SQL. The `sqlite`, `mysql`, and `postgres` packages select drivers and migrations. REST and MCP are sibling adapters over the same application service. REST administration auth and MCP connection auth are intentionally separate boundaries.

```text
REST / MCP
    -> application service
       -> structured search + pluggable scorer
       -> dependency resolver + compiler + token budget
       -> Storage port
          -> SQLite | MySQL | PostgreSQL adapter
```

Search returns only candidate metadata. Compilation loads the selected Skill and non-fallback dependencies, detects cycles, combines their procedures, deduplicates requirements, adapts examples to the model size, and removes optional material before trimming instructions. Required steps and context are retained even if a caller chooses an unrealistically small budget.

The current search implementation evaluates portable relational metadata in Go. This keeps behavior identical across databases. A later indexed candidate repository or semantic provider can be added behind the search boundary without changing compiler or domain types. Embeddings are intentionally absent.

SkillBox emits tool and context requirement contracts; it does not call MCPBoxPro, RagBox, an LLM, or business tools. AgentBox or another orchestrator owns those actions.

## MCP access architecture

There is one SkillBox process, one database, and one set of workspaces, projects, and Skills. Each MCP request resolves this chain server-side:

```text
API key hash -> MCP connection -> profile -> permissions -> allowed tools
                         -> workspace/project scope
```

The URL suffix is only a connection hint and can never select a trusted role. A credential bound to `student` cannot become `teacher` by calling `/mcp/teacher`. `tools/list` is generated from the resolved profile and omits forbidden definitions completely, saving context for small models. `tools/call` independently repeats the permission check.

Built-in profiles are persisted data: Student has search/prepare/report, Teacher has draft/proposal/evidence tools but no publish permission, and Reviewer owns approve/reject/publish/rollback. Custom profiles use the same records and granular permissions. Connections automatically constrain search, reads, prepare, proposals, executions, and statistics to their workspace/project while retaining visible global and workspace-level rules.

The lifecycle is `Teacher draft -> immutable versions -> proposal -> Reviewer approval -> publication -> Student execution -> telemetry/trajectory -> Teacher analysis`. Publishing requires an approved proposal at the same base version, preventing stale review decisions.
