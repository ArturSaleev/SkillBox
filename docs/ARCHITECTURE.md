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
                              filesystem packages + SQL runtime/index
```

## Skill storage and package format

The filesystem is authoritative for Skill contents. `skills.directory`
(default `./data/skills`) contains one direct child directory per portable
Skill package. SQL is deliberately not the content authority: it stores the
search index, project/workspace bindings, lifecycle and review state, version
metadata, execution evidence, and analytics. Startup discovery validates and
hashes every package before updating that index; a missing package is marked
missing rather than silently deleting its SQL history.

```text
<skills.directory>/
├── <package>/
│   ├── SKILL.md              # required YAML frontmatter + instructions
│   ├── scripts/              # optional, stored as untrusted data
│   ├── references/           # optional supporting material
│   ├── assets/               # optional binary/text assets
│   └── .skillbox-source.json # optional import provenance
└── .history/                 # immutable whole-package snapshots
```

`SKILL.md` requires string `name` and `description` frontmatter fields;
additional metadata is preserved. Package hashes cover sorted relative paths
and file bytes, so changes to scripts, references, assets, or other files also
create a distinct package version. Symlinks, special files, traversal,
absolute paths, and non-normalized paths are rejected. Whole-package rollback
atomically restores the selected snapshot, including file additions and
deletions.

Local directory, ZIP, and Git imports are staged and fully validated before an
atomic commit. Git hooks and recursive submodules are disabled. Directory and
ZIP exports copy the complete validated package without requiring the SQL
database.

## Imported-code security model

All imported Python, JavaScript, shell, build files, and other executable
content is untrusted stored data. SkillBox may detect executable-looking files,
run conservative static pattern scans, or send code to an explicitly approved
review provider, but it **never executes package code**. Student preparation
reads only `SKILL.md`; scripts, references, and assets are not embedded into the
compiled procedure. Static and AI review are advisory and cannot prove safety.

Sandboxing, runners, and runtime permission enforcement are outside the current
service boundary and must not be inferred from import or review results.

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

The Dashboard is database-wide rather than bound to one build-time project. Its `/admin/api` handlers list runtime data and provide narrow package-management operations: create a filesystem-backed draft, import/export packages, and run security reviews. Existing lifecycle mutations and compiled previews still use the selected Skill's URL-scoped Teacher or Student endpoint, preserving project isolation for MCP clients.

## HTTP behavior

- MCP routes accept `POST` only.
- Dashboard Admin API routes accept `GET` only and return `Cache-Control: no-store`.
- Dashboard files accept `GET` and `HEAD` only.
- Hashed `/_next/static/` assets are cached as immutable.
- HTML uses `no-cache`, allowing a replaced executable to expose its new embedded UI immediately after restart.
- Unknown UI routes return the embedded Next.js 404 page.

## Service boundary

SkillBox does not proxy an LLM, execute business tools, or fetch knowledge. Its narrow administrative HTTP surface exists only for the embedded Dashboard; the agent-facing contract remains MCP.

## AI code-review provider boundary

`internal/codereview` defines the `CodeReviewer` interface and a registry of
provider factories. The built-in `openai-compatible` adapter supports local
loopback endpoints and remote OpenAI-compatible services. Future OpenAI,
Anthropic, or other adapters can register without changing callers.

Every provider exposes its destination before review. Loopback addresses are
classified as local; all other HTTP(S) destinations are external. Requests to
external destinations are rejected before creating an HTTP request unless the
caller sets `external_transmission_approved` explicitly. This flag represents
an informed user decision that the submitted package code will leave the
machine; provider configuration alone is not consent.

AI review is advisory and always recommends manual review. Provider output is
never treated as proof that a package has no risk, and reviewed code is never
executed by the review pipeline.

Successful reviews are recorded through `codereview.History` with the Skill ID,
the exact package hash, provider, model, timestamp, findings, summary, and
recommendation. When the indexed package hash changes, the SQL update marks
reviews for every other hash as `outdated` in the same transaction. The
read-only Dashboard Admin API exposes the history at
`GET /admin/api/security-reviews?skill_id=<id>`.
