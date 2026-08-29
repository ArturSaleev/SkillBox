# SkillBox

> Procedural memory for AI agents, exposed through MCP.

[![License: MIT](https://img.shields.io/badge/License-MIT-22c55e.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)
[![MCP](https://img.shields.io/badge/protocol-MCP-8b5cf6)](docs/MCP.md)
[![Dashboard](https://img.shields.io/badge/dashboard-embedded-06b6d4)](docs/DASHBOARD.md)

**English** · [Русский](README.ru.md)

SkillBox is an open-source service for storing, reviewing, compiling, and measuring reusable AI procedures. A Skill tells an agent:

- when a procedure should be used;
- what trusted context and tools it needs;
- which steps to execute and in what order;
- what a successful result looks like;
- which failures and bad behaviors to avoid.

SkillBox does not try to replace an LLM or a knowledge base. It gives agents a durable procedural layer so a successful workflow can be reused, versioned, evaluated, and improved instead of rediscovered in every conversation.

## Why SkillBox?

Prompts are easy to copy, but hard to operate as a system. They usually lack ownership, scope, review, versions, tool requirements, execution evidence, and a reliable way to select only the relevant procedure.

SkillBox turns procedures into managed records:

```text
task
  │
  ├─ search compact Skill candidates
  ├─ select one relevant procedure
  ├─ compile it for the current model and tools
  ├─ execute it in the agent
  └─ report the outcome for future improvement
```

This is especially useful for local and smaller models, where concise, explicit procedures can improve workflow consistency. SkillBox does not claim to make a small model equivalent to a frontier model; the value should be measured against a baseline for each real task.

## Highlights

- **MCP-native** — Student and Teacher endpoints use JSON-RPC over HTTP.
- **Project isolation** — the project is owned by the URL and cannot be replaced by model-supplied arguments.
- **Reviewed lifecycle** — draft, validate, propose, approve or reject, publish, and roll back.
- **Progressive disclosure** — search returns compact candidates; preparation compiles one selected Skill.
- **Structured procedures** — ordered steps, context requirements, tools, dependencies, examples, and success criteria.
- **Execution evidence** — record success, failure, model information, duration, tool calls, and trajectories.
- **Database choice** — SQLite, MySQL, or PostgreSQL.
- **Embedded admin panel** — every project and Skill can be managed from the browser.
- **One production binary** — the static Next.js Dashboard is embedded in the Go executable.
- **Portable releases** — macOS and Linux builds for ARM64 and AMD64.

## Admin Dashboard

![SkillBox global admin overview](docs/assets/dashboard-overview.jpg)

*Global overview with database-wide Skill metrics, execution evidence, and lifecycle actions.*

![SkillBox database-wide Skills library](docs/assets/dashboard-skills-library.jpg)

*A searchable Skills library across every MCP project, with project, status, and scope filters.*

## Architecture

```text
                                      ┌──────────────────────────────┐
Browser ── GET / ────────────────────>│                              │
Browser ── GET /admin/api/* ─────────>│       SkillBox binary        │──> SQLite
Teacher ── POST /mcp/{project}/teacher│                              │──> MySQL
Student ── POST /mcp/{project} ──────>│  Go API + embedded Dashboard │──> PostgreSQL
                                      └──────────────────────────────┘
```

The two agent-facing routes are:

| Route | Role | Purpose |
| --- | --- | --- |
| `POST /mcp/{project_id}` | Student | Search, prepare, and report Skill results |
| `POST /mcp/{project_id}/teacher` | Teacher | Author, review, publish, inspect, and roll back Skills |

The browser Dashboard is database-wide. MCP clients remain project-scoped.

## Quick start

### Option 1: build locally

Requirements: Go 1.26+, Node.js 24+, and npm.

```bash
git clone https://github.com/ArturSaleev/SkillBox.git
cd SkillBox
make build
./skillbox -config ./configs/skillbox.yaml
```

Open [http://127.0.0.1:8081](http://127.0.0.1:8081).

Node.js is required only while building. The resulting `skillbox` executable contains the complete Dashboard.

### Option 2: Docker with persistent SQLite

```bash
docker compose -f docker-compose.sqlite.yml up --build
```

The named Docker volume keeps the SQLite database across container recreation.

### Verify the MCP endpoint

The first `initialize` request atomically creates the URL project:

```bash
curl -s http://127.0.0.1:8081/mcp/demo \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
```

List Student tools:

```bash
curl -s http://127.0.0.1:8081/mcp/demo \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
```

Continue with the [ten-minute tutorial](docs/QUICKSTART.md) to create, validate, publish, prepare, and evaluate your first Skill.

## Skill lifecycle

```text
create draft
    │
    ├─ update draft
    └─ validate
         │
         └─ create proposal
                │
                ├─ reject ──> improve draft
                └─ approve
                     │
                     └─ publish ──> active Skill
                                         │
                                         ├─ report execution evidence
                                         └─ roll back as a new version
```

Every create, update, publication, and rollback produces immutable version history. Rollback never erases previous versions.

## Configuration

SkillBox uses one small YAML file:

```yaml
server:
  address: "127.0.0.1:8081"

database:
  driver: sqlite # sqlite, mysql, postgres
  path: ./data/skillbox.db
  dsn: ""
```

- SQLite uses `path`.
- MySQL and PostgreSQL use `dsn`.
- Migrations run automatically at startup.
- Relative SQLite paths are resolved from the process working directory.

`address: ":8081"` listens on every network interface. Keep `127.0.0.1:8081` for local-only use.

## Release builds

```bash
./build-release.sh host # current platform
./build-release.sh all  # macOS/Linux, ARM64/AMD64
```

Each release contains one executable plus configuration and documentation:

```text
release/<os>/<arch>/SkillBox/
├── SkillBox
├── configs/skillbox.yaml
├── docs/
└── README.md
```

Existing release configuration is preserved across rebuilds.

## Project status

SkillBox is usable today and is also an early-stage open-source project. The core storage, MCP lifecycle, embedded Dashboard, migrations, release builds, and integration tests are implemented. The project is looking for real-world feedback and contributors before claiming production maturity.

Important current boundary: **SkillBox has no built-in authentication**. Do not expose Student, Teacher, or the Dashboard directly to an untrusted network. Bind locally or place the service behind an authenticated reverse proxy or another trusted network boundary.

See the [Roadmap](ROADMAP.md) for planned work and [Security Policy](SECURITY.md) for responsible reporting.

## Documentation

| Guide | Description |
| --- | --- |
| [Quick start](docs/QUICKSTART.md) | Publish and use the first Skill |
| [MCP contract](docs/MCP.md) | Routes, roles, tools, and JSON-RPC envelopes |
| [Skill model](docs/SKILL_MODEL.md) | Scope, structured content, versions, and compilation |
| [Dashboard](docs/DASHBOARD.md) | Embedded admin panel and authoring behavior |
| [Architecture](docs/ARCHITECTURE.md) | Packages, trust boundaries, and build pipeline |
| [Database](docs/DATABASE.md) | Schema, drivers, migrations, and backups |
| [Deployment](docs/DEPLOYMENT.md) | Local, release, Docker, and network deployment |
| [Community launch](docs/COMMUNITY_LAUNCH.md) | Ready-to-post discussion and outreach checklist |

## Community

Good contributions are not limited to code. Real procedures, evaluation results, UX feedback, documentation, adapters, and failure reports are all valuable.

- Read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.
- Use GitHub Issues for reproducible bugs and scoped feature proposals.
- Use GitHub Discussions for questions, ideas, use cases, and design conversations.
- Look for issues labeled `good first issue` or `help wanted` when those labels become available.
- Follow the [Code of Conduct](CODE_OF_CONDUCT.md).

If the core idea resonates with you, open a Discussion and describe the workflow you want your agent to remember. That is the most useful place to start.

## FAQ

### Is SkillBox a prompt library?

It can store instructions, but the model is broader: triggers, scope, context, tools, ordered steps, dependencies, examples, versions, review state, and execution evidence are first-class data.

### Does SkillBox execute tools or call an LLM?

No. The agent calls SkillBox to select and compile a procedure, then executes the procedure with its own model and tools.

### Is SkillBox a RAG or knowledge-base server?

No. Knowledge systems store facts and documents. SkillBox stores procedures for using context and tools reliably. They are complementary.

### Are project IDs trusted from the model?

No. Project scope comes from `/mcp/{project_id}`. Server-side scope application overwrites model-supplied workspace and project arguments.

### Is the Dashboard a separate service?

No. It is statically built and embedded into the same Go binary.

### Can I use SkillBox with a local model?

Yes. The protocol is model-independent. Measure the model with and without the relevant Skill to understand the actual improvement.

## License

[MIT](LICENSE) © 2026 Artur Saleev.
