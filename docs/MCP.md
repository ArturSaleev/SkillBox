# MCP profiles and connections

SkillBox accepts JSON-RPC 2.0 at `POST /mcp` and `POST /mcp/{connection}`. Every request requires `X-SkillBox-Key`, `X-API-Key`, or a Bearer credential tied to an enabled `mcp_connections` record. The optional path ID/slug must match that authenticated connection; a client cannot submit a trusted profile name.

During `initialize`, server information identifies the resolved profile. `tools/list` contains only tools present in both the profile allowlist and its granular permissions. `tools/call` repeats that check.

## Built-in profiles

Student discovers exactly:

- `search_skills`
- `prepare_skill`
- `report_skill_result`

Student searches only active Skills, cannot prepare a draft even by UUID, and may write only scoped execution telemetry.

Teacher discovers:

- `search_skills`, `get_skill`
- `create_skill_draft`, `update_skill_draft`, `validate_skill`
- `create_skill_proposal`, `create_skill_version`
- `get_skill_statistics`
- `list_recent_executions`, `get_execution`, `get_execution_trajectory`
- `get_skill_successes`, `get_skill_failures`

Teacher cannot publish by default.

Reviewer discovers:

- `search_skills`, `get_skill`, `validate_skill`
- `get_skill_proposal`, `list_skill_proposals`
- `approve_skill_proposal`, `reject_skill_proposal`
- `publish_skill`, `rollback_skill_version`

An approved proposal can be published only if the Skill remains at the reviewed base version. Publication creates a new active Skill version and marks the proposal `published`.

## Granular permissions

Profiles combine a tool allowlist with permissions such as `skill.read`, `skill.search`, `skill.prepare`, `skill.create`, `skill.update`, `skill.validate`, `skill.propose`, `skill.version.create`, `skill.publish`, `skill.rollback`, `execution.report`, `execution.read`, `execution.trajectory.read`, and `statistics.read`. A tool must pass both checks. Custom profiles can be created through YAML or REST.

## Scope isolation

A connection may bind `workspace_id` and `project_id`. The server injects these values rather than relying on model arguments. A project connection sees global Skills, matching workspace Skills, and its own project Skills; it cannot read another project or workspace. The same rule covers proposals, executions, trajectories, and statistics.

## Connection setup

```bash
response=$(curl -s http://127.0.0.1:8080/api/v1/mcp-connections \
  -H 'Content-Type: application/json' \
  -d '{"slug":"claude-teacher","name":"Claude Teacher","profile_slug":"teacher","workspace_id":"<uuid>","project_id":"<uuid>"}')
```

Store the returned `api_key`; SkillBox cannot recover it later. Discover tools with:

```bash
curl -s http://127.0.0.1:8080/mcp/claude-teacher \
  -H 'Content-Type: application/json' \
  -H 'X-SkillBox-Key: <api-key>' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```
