# Architecture

The runtime is a single path:

```text
Student or Teacher MCP request
    -> URL project resolution
    -> application service
    -> search / compiler / lifecycle
    -> SQLite, MySQL, or PostgreSQL storage
```

`internal/domain` contains Skill entities and validation. `internal/application`, `internal/search`, and `internal/compiler` contain business logic. `internal/storage/sqlstore` provides the shared SQL implementation, while database packages select the driver and embedded initial schema.

Profiles are fixed in application code. Student receives three execution tools. Teacher receives the complete authoring, review, publication, rollback, and evidence toolset. They are not stored or configured.

Projects are created from validated URL identifiers. The server applies project scope after decoding tool arguments, preventing the model from selecting another project.

SkillBox does not proxy an LLM, execute business tools, fetch knowledge, or expose an administrative REST API.
