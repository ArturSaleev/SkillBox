# SkillBox

SkillBox is a small MCP service that stores reusable agent procedures. It exposes only two endpoints:

- `POST /mcp/{project_id}` — Student: search, prepare, and report a Skill result.
- `POST /mcp/{project_id}/teacher` — Teacher: create, validate, review, publish, inspect, and roll back Skills.

There is no REST API, API key, connection registry, configurable profile, Reviewer endpoint, metrics endpoint, or authentication mode.

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

## Run

```bash
GOWORK=off go build -o skillbox ./cmd/skillbox
./skillbox
```

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

Documentation: [MCP](docs/MCP.md), [database](docs/DATABASE.md), [architecture](docs/ARCHITECTURE.md), and [deployment](docs/DEPLOYMENT.md).
