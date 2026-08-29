# Dashboard

The Dashboard is the database-wide administrative interface for SkillBox. It is built with Next.js, TypeScript, Tailwind CSS, TanStack Query/Table, Recharts, Zustand, axios, and reusable UI components.

## Runtime

Production does not run a Next.js server. `next build` produces a static export, `build-dashboard.sh` copies it into `internal/dashboard/dist`, and Go embeds those files into the SkillBox executable.

The browser and MCP share one origin:

```text
GET  /                              Dashboard
GET  /admin/api/*                   Global read-only administration
POST /mcp/{project_id}/teacher      Project-scoped mutations
POST /mcp/{project_id}              Compiled preview via prepare_skill
```

The release build leaves `NEXT_PUBLIC_API_URL` empty so requests remain same-origin. No project ID is compiled into the static JavaScript: project ownership comes from the database, and new Skills require an explicit project selection.

## Pages

- `/` — overview cards, recent executions, top Skills and models.
- `/skills/` — searchable, sortable, project-filterable table containing every Skill in the database.
- `/skills/view/?id=<skill_id>` — definition, steps, tools, context, dependencies, examples, proposals, rollback, and execution statistics.
- `/editor/` — create a draft.
- `/editor/?id=<skill_id>` — edit an existing draft.
- `/executions/` — live execution feed with three-second refresh.
- `/analytics/` — domain, model, status, duration, and error charts.

## Authoring behavior

- Form changes are autosaved to browser `localStorage`.
- `Ctrl+S` or `Cmd+S` saves the draft through MCP.
- Preview can show local instructions or call the Student `prepare_skill` tool for an existing active Skill.
- Validate, propose, approve, publish, and rollback actions follow the real backend lifecycle.
- The MCP client initializes a separate Teacher or Student connection for each project it operates on.
- Query data is cached for five minutes unless the screen uses live refresh.

## Current backend limitations

The UI intentionally does not report unsupported actions as successful:

- there is no destructive delete tool;
- there is no MCP tool that lists immutable Skill versions;
- active Skills cannot be edited directly because `update_skill_draft` accepts drafts only;
- project scope is enforced by the Teacher URL regardless of client-supplied scope IDs.

## Admin API

The embedded UI uses read-only `GET` endpoints under `/admin/api` for projects, Skills, executions, statistics, and proposals. Responses are never cached. Writes are deliberately not implemented there: all lifecycle changes continue through the existing project-scoped Teacher MCP tools.

Proposal history and the current version remain visible. Rollback accepts a known version number and creates a new immutable current version.

## Source checks

Run frontend-only checks from `dashboard/`:

```bash
npm ci
npm run lint
npm run build
npm audit --omit=dev
```

To produce and exercise the actual embedded UI, build from the repository root with `make build` and open the Go service at `http://127.0.0.1:8081/`.
