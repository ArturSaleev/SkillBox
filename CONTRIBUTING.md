# Contributing to SkillBox

Thank you for helping build a practical procedural-memory layer for AI agents. Contributions may be code, documentation, reproducible evaluations, integrations, UX feedback, or well-described real-world failures.

## Start with the right channel

- Use **GitHub Discussions** for questions, early ideas, architecture proposals, and use cases.
- Use **GitHub Issues** for reproducible bugs and scoped work that is ready to implement.
- Use **Pull Requests** for focused changes with tests and documentation.
- Use the private process in [SECURITY.md](SECURITY.md) for vulnerabilities.

For a large or compatibility-breaking change, open a Discussion before implementation. This prevents multiple contributors from solving the same problem in incompatible ways.

## Development setup

Requirements:

- Go 1.26 or newer;
- Node.js 24 and npm;
- Git;
- Docker only for container-specific work;
- MySQL or PostgreSQL only when testing those drivers.

```bash
git clone https://github.com/ArturSaleev/SkillBox.git
cd SkillBox
npm ci --prefix dashboard
GOWORK=off go test ./...
```

Build the complete single-binary application:

```bash
make build
./skillbox -config ./configs/skillbox.yaml
```

## Repository map

```text
cmd/skillbox/              process entrypoint and HTTP routing
internal/application/      lifecycle and scope rules
internal/compiler/         Skill compilation and token budgeting
internal/domain/           core models and validation
internal/ports/            storage contracts
internal/storage/          SQLite, MySQL, and PostgreSQL adapters
internal/transport/mcp/    Student and Teacher JSON-RPC transport
internal/dashboard/        embedded files and read-only Admin API
dashboard/                 Next.js administrative interface
migrations/                source migrations for each database
tests/                     cross-driver and MCP contract tests
docs/                      architecture and operator documentation
```

Keep dependencies pointing inward: transport and storage depend on domain/application contracts, not the reverse.

## Working on a change

1. Search Issues and Discussions for existing work.
2. Keep the change focused on one problem.
3. Add or update tests for observable behavior.
4. Update the nearest documentation when a contract changes.
5. Preserve existing database files and configuration during upgrades.
6. Run the full verification commands.
7. Explain the motivation, behavior, and verification in the Pull Request.

Do not include secrets, real customer data, private prompts, production database dumps, or proprietary procedures in examples or tests.

## Required checks

Backend:

```bash
gofmt -w cmd internal tests
GOWORK=off go test ./...
GOWORK=off go vet ./...
```

Dashboard:

```bash
cd dashboard
npm ci
npm run lint
npm run build
```

Complete embedded release:

```bash
./build-release.sh host
```

Do not state that a command passed unless you ran it. If a platform or external database was not available, say so in the Pull Request.

## Database changes

- Keep `migrations/<driver>/001_initial.sql` and `internal/migrate/sql/<driver>_001_initial.sql` mirrored.
- Cover SQLite in ordinary tests.
- Set `SKILLBOX_TEST_MYSQL_DSN` or `SKILLBOX_TEST_POSTGRES_DSN` for optional driver contract runs.
- Avoid destructive migration behavior and preserve existing user data.
- Document backup or rollout requirements.

The current migration set is a compact initial schema. Discuss the migration strategy before introducing a second migration series.

## MCP compatibility

The current public contract has exactly two project-scoped routes:

```text
POST /mcp/{project_id}
POST /mcp/{project_id}/teacher
```

Changes to tool names, schemas, scope rules, lifecycle rules, or result envelopes are compatibility changes. They require contract tests and documentation updates.

Project identity must remain server-owned. Never trust model-supplied workspace, project, user, credential, or authorization values.

## Pull Request guidelines

A useful Pull Request contains:

- a short problem statement;
- the chosen behavior and relevant tradeoffs;
- tests added or changed;
- commands actually run;
- screenshots for visible Dashboard changes;
- migration and compatibility notes when applicable.

Small commits are welcome, but the final branch should tell one coherent story. Maintainers may ask for a change to be split when review or rollback would otherwise be risky.

By contributing, you agree that your contribution is licensed under the repository's [MIT License](LICENSE).
