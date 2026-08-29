# Architecture

SkillBox is one Go process with two runtime surfaces: an embedded read-only web application and scoped MCP JSON-RPC endpoints.

```text
                         ┌─ GET / and static assets ─> embedded Next.js export
Browser ────────────────┤
                         └─ Teacher JSON-RPC / Student preview ─┐
                                                     │
Agent ── Student JSON-RPC ───────────────────────────┤
                                                     v
                                      URL project resolution
                                                     v
                                         application services
                                  search / compiler / lifecycle
                                                     v
                                     SQLite / MySQL / PostgreSQL
```

## Package boundaries

- `internal/domain` contains Skill entities, lifecycle values, execution telemetry, and validation.
- `internal/application` owns visibility, URL scope enforcement, validation, proposals, and publication.
- `internal/search` returns compact ranked candidates without exposing full Skill bodies.
- `internal/compiler` resolves dependencies and compacts one selected Skill to a token budget.
- `internal/storage/sqlstore` contains the shared SQL implementation; driver packages open SQLite, MySQL, or PostgreSQL.
- `internal/transport/mcp` implements MCP JSON-RPC and the fixed Student/Teacher tool definitions.
- `internal/dashboard` serves the static files embedded at compile time.
- `dashboard` contains the Next.js source. It is not a second production service.

## Build pipeline

```text
dashboard source
    -> npm ci
    -> Next.js static export (dashboard/out)
    -> copy to internal/dashboard/dist
    -> go:embed
    -> one SkillBox executable
```

Generated `dashboard/out` and embedded asset copies are ignored by Git. `internal/dashboard/dist/README.txt` is retained so ordinary Go package discovery works before the first frontend build. Use `make build` or `build-release.sh` when the executable must contain the current Dashboard.

The embed directive uses `all:dist`. The `all:` prefix is mandatory because Go otherwise excludes names beginning with `_`, including Next.js' `/_next/static` CSS, JavaScript, and font assets.

## Access and scope

Profiles are fixed in application code. Student receives three execution tools. Teacher receives the complete authoring, review, publication, rollback, and evidence toolset. They are not stored or configured.

Projects are created from validated URL identifiers. The server applies project scope after decoding tool arguments, preventing the model from selecting another project.

The Dashboard is database-wide rather than bound to one build-time project. Its read-only `/admin/api` handlers list projects, Skills, executions, statistics, and proposals through the same storage ports used by MCP. Mutations and compiled previews still use the selected Skill's URL-scoped Teacher or Student endpoint, preserving project isolation for MCP clients.

## HTTP behavior

- MCP routes accept `POST` only.
- Dashboard Admin API routes accept `GET` only and return `Cache-Control: no-store`.
- Dashboard files accept `GET` and `HEAD` only.
- Hashed `/_next/static/` assets are cached as immutable.
- HTML uses `no-cache`, allowing a replaced executable to expose its new embedded UI immediately after restart.
- Unknown UI routes return the embedded Next.js 404 page.

## Service boundary

SkillBox does not proxy an LLM, execute business tools, or fetch knowledge. Its narrow administrative HTTP surface exists only for the embedded Dashboard; the agent-facing contract remains MCP.
