# MCP

SkillBox has exactly two MCP JSON-RPC routes:

```text
POST /mcp/{project_id}
POST /mcp/{project_id}/teacher
```

Neither route requires an API key or another authorization header. There are no MCP connection records or configurable profiles.

Requests use `Content-Type: application/json`. A client must initialize its URL project before calling tools:

```bash
curl -s http://127.0.0.1:8081/mcp/demo \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
```

The first `initialize` atomically creates the URL project when it does not exist. Repeated and concurrent initialization reuse the same project. The server injects the internal project scope into every tool call, so model-supplied `project_id` or `workspace_id` values cannot change access scope.

Project IDs are trimmed, limited to 128 characters, and may contain ASCII letters, digits, `.`, `_`, and `-`. Empty values, special path segments, separators, and traversal are rejected.

## Student

Student exposes exactly:

- `search_skills`
- `prepare_skill`
- `report_skill_result`

Student sees active global Skills and active Skills belonging to its URL project.

## Teacher

Teacher owns the complete Skill lifecycle. It can create and update drafts, validate them, create and review proposals, publish or roll back versions, and inspect execution evidence and statistics. This combines the former Teacher and Reviewer responsibilities into one endpoint.

Teacher exposes exactly 19 tools:

- `search_skills`
- `get_skill`
- `create_skill_draft`
- `update_skill_draft`
- `validate_skill`
- `create_skill_proposal`
- `create_skill_version`
- `get_skill_statistics`
- `list_recent_executions`
- `get_execution`
- `get_execution_trajectory`
- `get_skill_successes`
- `get_skill_failures`
- `get_skill_proposal`
- `list_skill_proposals`
- `approve_skill_proposal`
- `reject_skill_proposal`
- `publish_skill`
- `rollback_skill_version`

The publication workflow is:

```text
create/update draft
    -> validate
    -> create proposal
    -> approve or reject proposal
    -> publish approved proposal
```

Only drafts can be updated or submitted as proposals. Publishing requires an approved proposal. Rollback does not erase history; it creates a new current version from an immutable snapshot.

## Tool result envelope

Successful `tools/call` responses contain both MCP text content and typed structured content:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "content": [{ "type": "text", "text": "{...}" }],
    "structuredContent": {},
    "isError": false
  }
}
```

Dashboard code reads `structuredContent`; agents may use the normal MCP content representation.

## Dashboard use

The embedded Dashboard reads database-wide administrative views from the same-origin `/admin/api`. For validation, authoring, proposals, publication, rollback, and compiled preview it selects `/mcp/{project_id}/teacher` or `/mcp/{project_id}` from the Skill's owning project. Scoped MCP clients initialize independently and retry initialization after a failed attempt.
