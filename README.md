# SkillBox

SkillBox stores, searches, versions, compiles, and analyzes reusable AI-agent procedures. In the AiBox ecosystem, RagBox answers **what** an agent knows, MCPBoxPro exposes **what it can do**, and SkillBox supplies **how to perform the current task**. SkillBox does not execute business tools, proxy models, or fetch context itself.

The runtime follows progressive disclosure:

```text
task -> structured candidate search -> select one Skill -> compact compilation -> agent
```

## Quick start (SQLite)

Requires Go 1.26 or a release binary.

```bash
GOWORK=off go build -o skillbox ./cmd/skillbox
./skillbox
```

The included config creates `./data/skillbox.db`, applies migrations, and seeds six demo Skills. Verify it:

```bash
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/ready
curl -s http://127.0.0.1:8080/api/v1/skills
```

Prepare a compact Skill through REST:

```bash
curl -s http://127.0.0.1:8080/api/v1/skills/prepare \
  -H 'Content-Type: application/json' \
  -d '{"task":"Add a Go HTTP handler","domains":["golang"],"intents":["create"],"available_tools":["search_files","read_file","write_file"],"model":{"provider":"ollama","name":"qwen:7b","context_window":4096},"max_skill_tokens":800}'
```

## Docker Compose

```bash
docker compose -f docker-compose.sqlite.yml up --build
docker compose -f docker-compose.mysql.yml up --build
docker compose -f docker-compose.postgres.yml up --build
```

Use one stack at a time; all examples publish port `8080`.

## Configuration

YAML is loaded from `-config` (default `./configs/skillbox.yaml`). Environment variables override YAML: `SKILLBOX_SERVER_ADDRESS`, `SKILLBOX_DATABASE_DRIVER`, `SKILLBOX_DATABASE_PATH`, `SKILLBOX_DATABASE_DSN`, `SKILLBOX_DATABASE_MIGRATE`, `SKILLBOX_AUTH_MODE`, `SKILLBOX_API_KEYS`, `SKILLBOX_LOG_LEVEL`, `SKILLBOX_LOG_FORMAT`, and `SKILLBOX_SEED_DEMO`.

For REST administration auth, set `SKILLBOX_AUTH_MODE=api_key` and a comma-separated `SKILLBOX_API_KEYS`. MCP authentication is deliberately separate: every MCP API key resolves to a persisted connection, profile, granular permission set, and workspace/project scope. Raw MCP keys are never stored.

Create a Student connection (the generated key is returned once):

```bash
curl -s http://127.0.0.1:8080/api/v1/mcp-connections \
  -H 'Content-Type: application/json' \
  -d '{"slug":"local-qwen","name":"Local Qwen 7B","profile_slug":"student"}'
```

Then discover only the three Student tools:

```bash
curl -s http://127.0.0.1:8080/mcp/local-qwen \
  -H 'Content-Type: application/json' \
  -H 'X-SkillBox-Key: <returned-api-key>' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

## Interfaces and operations

- REST: `/api/v1/...`
- Connection-authenticated MCP JSON-RPC: `POST /mcp` or `/mcp/{connection}`
- Health/readiness: `/health`, `/ready`
- Prometheus metrics: `/metrics`
- Graceful shutdown: `SIGINT` or `SIGTERM`

Documentation: [architecture](docs/ARCHITECTURE.md), [database](docs/DATABASE.md), [REST API](docs/API.md), [MCP](docs/MCP.md), [Skill model](docs/SKILL_MODEL.md), and [deployment](docs/DEPLOYMENT.md).

## Verification

```bash
GOWORK=off go test ./...
```

MySQL and PostgreSQL contract tests use dedicated databases only when `SKILLBOX_TEST_MYSQL_DSN` and `SKILLBOX_TEST_POSTGRES_DSN` are set. SQLite contracts always run.
