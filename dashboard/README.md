# SkillBox Dashboard

Next.js database-wide admin panel for SkillBox. In release builds the browser calls the same SkillBox origin directly:

```text
${NEXT_PUBLIC_API_URL}/admin/api
${NEXT_PUBLIC_API_URL}/mcp/{project_id}/teacher
${NEXT_PUBLIC_API_URL}/mcp/{project_id}
```

## Run locally as one binary

From the repository root:

```bash
make build
./skillbox
```

Open `http://127.0.0.1:8081/`.

## Packaged release

`build-release.sh` creates a static Next.js export, embeds it into SkillBox with `go:embed`, and then compiles the executable. Start only SkillBox:

```bash
./SkillBox
```

Open `http://127.0.0.1:8081/`. The release contains one executable and does not require Node.js at runtime. Node.js and npm are build-time dependencies only.

The Dashboard lists every Skill in the configured database and follows the real project-scoped MCP lifecycle for create/update draft, validate, propose, approve, publish, and rollback. The current backend does not expose destructive deletion or a version-list tool, so the UI does not pretend those operations succeeded.
