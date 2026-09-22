# SkillBox Bench

SkillBox Bench is the model, MCP, and evaluation laboratory that lives beside SkillBox while remaining operationally independent. It uses its own executable, configuration, SQLite database, and embedded Web UI.

The current application provides:

- hot-editable provider and MCP connections;
- OpenAI-compatible model providers, including OpenRouter, Gemini compatibility, and local endpoints;
- Streamable HTTP MCP discovery and tool calls;
- a multi-turn chat with an MCP agent loop;
- persisted conversations, token counts, latency, and tool trajectories;
- masked secrets and an atomically written local YAML configuration.
- reusable benchmark cases with multiple model targets;
- paired Baseline / With Skill trials;
- frozen Skill preparation through a dedicated Student MCP connection;
- deterministic phrase and required-tool grading;
- persisted trial evidence and comparison charts.

See the full [Benchmark guide](../docs/BENCHMARKS.md).

## Run

```bash
make benchmark
./skillbox-bench -config ./benchmark/config.yaml
```

Open [http://127.0.0.1:8091](http://127.0.0.1:8091).

On first run, SkillBox Bench creates a minimal `benchmark/config.yaml` with file mode `0600`. The file and `benchmark/data/` are ignored by Git. Connections can then be entered through the Web UI and take effect for new requests immediately.

To begin with predefined connection placeholders:

```bash
cp benchmark/config.example.yaml benchmark/config.yaml
chmod 600 benchmark/config.yaml
```

Never commit `benchmark/config.yaml`. The example contains no credentials.

## State ownership

| State | Location |
| --- | --- |
| Provider and MCP settings | `benchmark/config.yaml` |
| API keys and authorization values | `benchmark/config.yaml` or environment variables |
| Chat sessions and messages | `benchmark/data/skillbox-benchmark.db` |
| MCP tool trajectories and response metrics | `benchmark/data/skillbox-benchmark.db` |

Secrets are never written to SQLite and are never returned by the configuration API. Existing chats retain only non-secret provider/model identifiers and execution evidence.

## Hot configuration

Saving a connection through the Web UI performs validation, writes a temporary file, syncs it, atomically replaces `config.yaml`, and activates the new in-memory snapshot. New chats use it immediately. In-flight requests keep their original snapshot.

Changing `server.address` or `database.path` is persisted but requires a restart because those resources are already open.

## Security boundary

SkillBox Bench is local-first and currently has no authentication. Keep it bound to `127.0.0.1`. Use only test MCP servers or isolated workspaces for tools that mutate data. Tool outputs are treated as untrusted data by the agent system prompt.
