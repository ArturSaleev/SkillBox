# SkillBox

SkillBox is a standalone MCP service for reusable AI procedures. A Skill describes when a procedure applies, which context and tools it needs, the ordered steps to execute, and how success is measured.

The runtime exposes two MCP endpoints:

- `POST /mcp/{project_id}` — Student: search, prepare, and report a Skill result.
- `POST /mcp/{project_id}/teacher` — Teacher: create, validate, review, publish, inspect, and roll back Skills.

The management Dashboard is available at `GET /`. The static Next.js export is compiled into the same Go executable, so production requires only one SkillBox process and no Node.js runtime.

There is no public business REST API, API key, connection registry, configurable profile, Reviewer endpoint, metrics endpoint, or authentication mode. The embedded Dashboard has a same-origin, read-only `/admin/api` surface for database-wide administration.

## Runtime layout

```text
Browser ── GET / ──────────────────────────────┐
Browser ── POST /mcp/{project}/teacher ────────┤
Agent   ── POST /mcp/{project} ────────────────┼─> SkillBox binary
                                               └─> SQLite / MySQL / PostgreSQL
```

The Dashboard lists every project and Skill in the database. Read-only administration uses `/admin/api`; lifecycle mutations are sent to the selected Skill's project-scoped Teacher MCP endpoint.

## Requirements

- Go 1.26 or newer.
- Node.js 24 and npm for source and release builds.
- No Node.js installation is required on the target runtime machine.

## Configuration

`configs/skillbox.yaml` contains the complete configuration:

```yaml
server:
  address: ":8081"

database:
  driver: sqlite # sqlite, mysql, postgres
  path: ./data/skillbox.db
  dsn: ""
```

SQLite uses `path`. MySQL and PostgreSQL use `dsn`. Migrations always run automatically at startup.

## Build and run

```bash
make build
./skillbox -config ./configs/skillbox.yaml
```

Open `http://127.0.0.1:8081/` for the Dashboard. Node.js and npm are required only while building the executable.

`make build` performs these steps:

1. installs the locked Dashboard dependencies with `npm ci`;
2. exports the static Next.js application;
3. copies generated assets into `internal/dashboard/dist`;
4. compiles the assets into `skillbox` with `go:embed`.

## Release bundles

Build the current platform or all supported targets:

```bash
./build-release.sh host
./build-release.sh all
```

Supported targets are macOS and Linux on ARM64 and AMD64. Each bundle contains one executable plus configuration and documentation:

```text
release/<os>/<arch>/SkillBox/
├── SkillBox
├── configs/skillbox.yaml
├── docs/
└── README.md
```

The default `release` directory is created beside the SkillBox repository. Override it with `DIST_DIR=/absolute/path`.

Initialize Student; the project is created automatically on the first request:

```bash
curl -s http://127.0.0.1:8081/mcp/demo \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
```

Initialize Teacher:

```bash
curl -s http://127.0.0.1:8081/mcp/demo/teacher \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
```

No authorization header is required. `project_id` comes only from the URL, is validated by the server, and cannot be replaced through tool arguments.

`address: ":8081"` listens on all available interfaces. Use `address: "127.0.0.1:8081"` when SkillBox must be reachable only from the same machine.

## Documentation

- [Dashboard](docs/DASHBOARD.md)
- [MCP contract](docs/MCP.md)
- [Skill model](docs/SKILL_MODEL.md)
- [Database](docs/DATABASE.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Build and deployment](docs/DEPLOYMENT.md)
