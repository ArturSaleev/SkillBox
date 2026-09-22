# Dashboard

The Dashboard is the database-wide administrative interface for SkillBox. It is built with Next.js, TypeScript, Tailwind CSS, TanStack Query/Table, Recharts, Zustand, axios, and reusable UI components.

## Runtime

Production does not run a Next.js server. `next build` produces a static export, `build-dashboard.sh` copies it into `internal/dashboard/dist`, and Go embeds those files into the SkillBox executable.

The browser and MCP share one origin:

```text
GET  /                              Dashboard
GET/POST /admin/api/*              Global administration and package review
POST /mcp/{project_id}/teacher      Project-scoped mutations
POST /mcp/{project_id}              Compiled preview via prepare_skill
```

The release build leaves `NEXT_PUBLIC_API_URL` empty so requests remain same-origin. No project ID is compiled into the static JavaScript. New Skills default to global availability and require a project only when the administrator selects project-only availability.

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
- `Ctrl+S` or `Cmd+S` creates or edits a filesystem-backed draft through the admin API. Existing publish/approval lifecycle mutations continue through MCP.
- Preview can show local instructions or call the Student `prepare_skill` tool for an existing active Skill.
- Validate, propose, approve, publish, and rollback actions follow the real backend lifecycle.
- The MCP client initializes a separate Teacher or Student connection for each project it operates on.
- Query data is cached for five minutes unless the screen uses live refresh.

## Current backend limitations

The UI intentionally does not report unsupported actions as successful:

- there is no destructive delete tool;
- there is no MCP tool that lists immutable Skill versions;
- active Skills cannot be edited directly because `update_skill_draft` accepts drafts only;
- existing project-scoped Skills retain the scope enforced at creation; Teacher updates cannot move a Skill to another scope.

## Admin API

The embedded UI uses `GET` endpoints under `/admin/api` for projects, Skills, executions, statistics, proposals, and security-review history. Responses are never cached. The narrowly scoped admin writes create or edit a filesystem-backed draft, import packages, and run a configured AI review. Publish, approval, rollback, and other lifecycle changes continue through the existing project-scoped Teacher MCP tools.

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
## Import and export

The Skills Library provides an Import panel for ZIP uploads and Git repository
URLs. The browser never pretends to read arbitrary local directories. Preview
validates the package on the server and displays its name, description, file
manifest, script/reference/asset counts, source, revision, and package hash
before the separate Import action is enabled.

Preview also runs the extensible executable-code detector. Potential source or
command files are listed with their detected language and reason in a warning
panel. Detection is informational: the preview and import paths never execute
the files.

Each filesystem-backed Skill also has an Export ZIP action in the library row.
The response contains the complete portable package and does not depend on
database content for the archive itself.

## AI security review

The Security tab runs the configured reviewer against bounded text files from
the immutable package hash; package code is never executed. Every result is
stored with that hash and is marked outdated when the package changes. An
external endpoint requires an explicit per-review confirmation in the UI.

Configure the provider in `configs/skillbox.yaml`. API-key values are read only
from the environment variable named by `code_review.api_key_env`; neither the
key nor an editable secret field is exposed by the unauthenticated Dashboard.
