# REST API

All management routes are below `/api/v1` and return JSON. Errors use `{"error":{"code":"...","message":"..."}}` with standard HTTP status codes.

| Method | Route | Purpose |
| --- | --- | --- |
| GET, POST | `/workspaces` | List/create workspaces |
| GET, POST | `/projects` | List/create projects |
| GET, POST | `/skills` | Metadata list/create complete Skill |
| GET, PUT | `/skills/{id}` | Read/update complete Skill |
| POST | `/skills/search` | Structured metadata candidate search |
| POST | `/skills/prepare` | Select and compile one Skill |
| GET | `/skills/{id}/versions` | Version history |
| GET | `/skills/{id}/versions/{version}` | Portable snapshot metadata and content |
| POST | `/skills/{id}/versions/{version}/rollback` | Restore snapshot as a new version |
| GET | `/skills/{id}/steps`, `/tools`, `/contexts`, `/dependencies`, `/examples` | Read component collections |
| GET, POST | `/executions` | List/report execution results |
| GET | `/statistics` | Success rates, optionally by `skill_id` |
| GET, POST | `/mcp-profiles` | List/upsert custom permission profiles |
| GET, POST | `/mcp-connections` | List/create scoped MCP connections; key returned once |
| GET | `/skill-proposals`, `/skill-proposals/{id}` | Proposal management reads |
| POST | `/skill-proposals/{id}/approve`, `/reject`, `/publish` | Administrative review/publication actions |

Create and update accept the complete Skill document. Updates replace component collections atomically and create a version snapshot. Include `change_summary` and optional `created_by` alongside Skill fields.

Search accepts `task`, ownership IDs, `scopes`, `domains`, `intents`, `object_types`, `tags`, `required_tool`, `available_tools`, `keywords`, and `limit`. Prepare accepts task, optional `skill_id`, ownership/metadata hints, available tools, model information, and `max_skill_tokens`.
