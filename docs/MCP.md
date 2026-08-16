# MCP

SkillBox has exactly two MCP JSON-RPC routes:

```text
POST /mcp/{project_id}
POST /mcp/{project_id}/teacher
```

Neither route requires an API key or another authorization header. There are no MCP connection records or configurable profiles.

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
