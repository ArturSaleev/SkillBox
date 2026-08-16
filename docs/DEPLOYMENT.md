# Deployment

Build and run with the minimal YAML config:

```bash
GOWORK=off go build -trimpath -o skillbox ./cmd/skillbox
./skillbox -config ./configs/skillbox.yaml
```

For Docker with SQLite:

```bash
docker compose -f docker-compose.sqlite.yml up --build
```

To use MySQL or PostgreSQL, set the driver and DSN directly in the YAML file. Migrations run automatically.

SkillBox has no application-level authentication. Bind to `127.0.0.1:8081` or protect the listener at the network boundary when it must not be publicly reachable.
