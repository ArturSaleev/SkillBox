# Deployment

## Prerequisites

- Go 1.26 or newer.
- Node.js 24 and npm on the build machine.
- Docker only when building the container image.

Node.js is not required after the executable has been built.

## Local build

Build and run with the minimal YAML configuration:

```bash
make build
./skillbox -config ./configs/skillbox.yaml
```

`make build` first produces the static Next.js Dashboard, copies it into the Go embed package, and then builds one `skillbox` executable. The running service serves the Dashboard at `/` and MCP at `/mcp/{project_id}`; Node.js is not required at runtime.

Open:

```text
http://127.0.0.1:8081/
```

If another SkillBox process is already running, stop and restart it after rebuilding. Replacing the executable on disk does not update a process that is already in memory.

## Release builds

Build only the current host:

```bash
./build-release.sh host
```

Build every supported target:

```bash
./build-release.sh all
```

`all` produces:

```text
<repository-parent>/release/
├── darwin/arm64/SkillBox/
├── darwin/amd64/SkillBox/
├── linux/arm64/SkillBox/
└── linux/amd64/SkillBox/
```

Every directory contains a platform-specific `SkillBox` executable with the same platform-independent Dashboard embedded inside it. Use an explicit output directory when needed:

```bash
DIST_DIR=/absolute/release/path ./build-release.sh host
```

The release script creates `configs/skillbox.yaml` from the example only when the target bundle has no configuration yet. Subsequent builds preserve the existing file and its database path. Keep a backup before changing deployment layout.

The Dashboard is not bound to a project at build time. It lists all projects and Skills in the configured database and chooses the correct project-scoped MCP route for each mutation.

## Verification

Recommended source checks:

```bash
GOWORK=off go test ./...
GOWORK=off go vet ./...
(cd dashboard && npm run lint)
./build-release.sh host
```

After starting the built executable, verify both surfaces:

```bash
curl -I http://127.0.0.1:8081/
curl -s http://127.0.0.1:8081/mcp/dashboard/teacher \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
```

## Docker

For Docker with SQLite:

```bash
docker compose -f docker-compose.sqlite.yml up --build
```

The Dockerfile uses a Node build stage for the Dashboard and a Go build stage for the executable. The final Alpine image contains only SkillBox, its YAML configuration, and the data directory.

## Databases

To use MySQL or PostgreSQL, set the driver and DSN directly in the YAML file. Migrations run automatically.

For SQLite, keep the database path on persistent storage. Back up an existing database before replacing it or changing deployment layout. SkillBox never removes an existing database automatically.

## Network security

SkillBox has no application-level authentication. Bind to `127.0.0.1:8081` or protect the listener at the network boundary when it must not be publicly reachable.

The embedded Dashboard uses the privileged Teacher endpoint. Do not expose it to an untrusted network without an authenticated reverse proxy or equivalent network protection.
