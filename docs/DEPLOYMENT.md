# Deployment

For standalone SQLite, copy `configs/skillbox.example.yaml` to `configs/skillbox.yaml`, choose a writable database path, and run `./SkillBox -config ./configs/skillbox.yaml`. Back up the SQLite file before upgrades. Migrations are forward-only and run on startup by default.

For containers, use exactly one supplied Compose file. Replace example database passwords and enable REST API-key auth before exposing the service outside a trusted network. Create separate high-entropy MCP connections for every client; never share Teacher, Reviewer, and Student credentials. Mount persistent storage for SQLite or the database service. `/health` checks the process; `/ready` checks database reachability; `/metrics` exposes Prometheus metrics.

Build a service-owned AiBox release bundle:

```bash
./build-release.sh host
# ../release/<os>/<arch>/SkillBox/
```

`all` cross-builds macOS and Linux arm64/amd64 binaries. Each bundle contains only SkillBox, its config example, README, and docs. The binary is CGO-free.

Shutdown sends no new work after the HTTP server begins graceful termination and uses the configured timeout. Structured logs include request ID, route or MCP method, duration, status, and configured database driver; Skill contents are not logged.
