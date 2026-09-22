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

Optional AI-assisted package review is configured without placing a secret in
browser storage:

```yaml
code_review:
  provider: openai-compatible
  endpoint: http://127.0.0.1:8080/v1
  model: local-review-model
  api_key_env: SKILLBOX_CODE_REVIEW_API_KEY
```

Leave the section empty to disable AI review. Loopback endpoints run without a
transmission prompt; external endpoints require explicit consent in the
Dashboard for every review. The static scanner and manual-review warning remain
active independently of AI configuration.

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
<skillbox-repository>/release/
├── darwin/arm64/SkillBox/
├── darwin/amd64/SkillBox/
├── linux/arm64/SkillBox/
└── linux/amd64/SkillBox/
```

By default, this is the local `release/` directory inside the SkillBox repository. It is excluded from Git.

Every directory contains platform-specific `SkillBox` and `skillbox-bench` executables. Both Web interfaces are embedded in their respective binaries. The bundle also includes `benchmark/config.example.yaml`, but never copies the local `benchmark/config.yaml` containing connection secrets. Use an explicit output directory when needed:

```bash
DIST_DIR=/absolute/release/path ./build-release.sh host
```

The release script creates `configs/skillbox.yaml` from the example only when the target bundle has no configuration yet. Subsequent builds preserve the existing file and its database path. SkillBox Bench creates its local `benchmark/config.yaml` on first run; this file is not overwritten by release builds. Keep a backup before changing deployment layout.

The Dashboard is not bound to a project at build time. It lists all projects and Skills in the configured database and chooses the correct project-scoped MCP route for each mutation.

## Skill packages directory

`skills.directory` configures the filesystem root for portable Skill packages and defaults to `./data/skills`. SkillBox creates this directory during startup if it does not exist. Relative paths are resolved from the process working directory, like the existing SQLite path.

The Docker image uses `/app` as its working directory, so the default resolves to `/app/data/skills` inside the persistent `/app/data` volume. Release bundles should be started from their bundle directory when using the default relative paths; use absolute paths when the process is managed from a different working directory.

## Migrating legacy DB-only Skills

Back up the database and the configured `skills.directory`, stop the running
SkillBox process, and run the one-shot migration with the same configuration:

```bash
./skillbox -config ./configs/skillbox.yaml -migrate-legacy-skills
```

The command exits after migration and does not start the HTTP server. It keeps
each Skill ID, scope, status, current version, version records, proposals, and
execution evidence in SQL. Current content becomes `<skills.directory>/<skill-id>/SKILL.md`;
every existing version is also materialized under the immutable `.history`
tree so rollback continues to work. SQL is switched to the package only after
all files for that Skill have been validated.

The operation is idempotent: already migrated Skills are reported as unchanged
and no new lifecycle versions are created. If the target package path already
contains different content, migration stops instead of overwriting it. After a
successful run, start SkillBox normally and retain the backup until the Skills
and their execution history have been verified.

## Importing Skill packages

Stop the running process and choose exactly one source:

```bash
./skillbox -config ./configs/skillbox.yaml -import-skill-directory /absolute/path/to/package
./skillbox -config ./configs/skillbox.yaml -import-skill-zip /absolute/path/to/package.zip
./skillbox -config ./configs/skillbox.yaml -import-skill-git https://example.com/team/skill.git -import-git-revision main
```

The command validates the complete package before atomically moving it into
`skills.directory`, synchronizes the SQL index, and exits without starting the
server. ZIP and Git paths are checked for traversal, and symlinks and special
files are rejected. Git is cloned as a bare repository into a temporary
directory and exported with `git archive`; repository checkout hooks,
submodules, package scripts, and imported executables are never run.

Import preview detects common executable source extensions, build/command file
names, and interpreter shebangs. Findings are warnings rather than execution:
SkillBox reads only a bounded file prefix and never invokes detected code.

Each imported package receives a `.skillbox-source.json` evidence file with its
source type and URL. Git imports also store the resolved commit SHA. Importing
the same unchanged source again is idempotent; changed content at the same
source-derived destination is rejected instead of overwriting the package.

## Exporting Skill packages

Select a package by its path relative to `skills.directory` and choose one
portable output format:

```bash
./skillbox -config ./configs/skillbox.yaml \
  -export-skill-package portable-a1b2c3d4e5f6 \
  -export-skill-directory /absolute/output/portable

./skillbox -config ./configs/skillbox.yaml \
  -export-skill-package portable-a1b2c3d4e5f6 \
  -export-skill-zip /absolute/output/portable.zip
```

Export runs before SkillBox opens its database and therefore depends only on
the filesystem package. The complete validated package is copied, including
`SKILL.md`, `scripts`, `references`, `assets`, source evidence, and any other
regular package files. Directory and ZIP outputs are committed atomically and
an existing destination is never overwritten.

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
