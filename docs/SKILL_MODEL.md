# Skill model

A Skill combines human-readable routing metadata with an executable procedure.

## Scope and status

Supported scopes are exactly:

- `global` — no workspace or project IDs;
- `workspace` — requires `workspace_id`;
- `project` — requires both `workspace_id` and `project_id`.

There is no `user` scope or `owner_user_id` field in the current model.

Lifecycle status is `draft`, `active`, `deprecated`, or `archived`. Drafts created through the current URL-scoped Teacher route are forced into that URL project's scope; client-supplied workspace/project IDs cannot change it.

## Content

Routing metadata includes domains, intents, object types, tags, keywords, capabilities, compatibility, priority, and tool requirements.

Procedure data includes:

- instructions;
- ordered steps with condition, required flag, and expected result;
- success criteria;
- required or optional tools;
- context requirements with type, query, priority, required flag, and optional token limit;
- examples of input, expected behavior, and optional bad behavior;
- typed dependencies: `requires`, `extends`, `uses`, or `fallback`.

Tool requirement must be `required` or `optional`. A Skill cannot depend on itself. Validation also checks dependency compilation and detects cycles.

## Versions and publication

Every create/update/rollback writes an immutable JSON snapshot and advances `current_version`. Rollback never destroys history; it creates a new current version from the selected snapshot.

Publication uses a snapshot proposal. A proposal records its base version and moves through `pending`, `approved` or `rejected`, and `published`. Only an approved proposal can be published.

## Compilation

Compilation applies dependencies before the selected Skill, detects cycles, and deduplicates rules. For models with context windows up to 8K, the highest-priority example is retained. Larger models omit examples by default. Budget reduction removes examples, optional context, and optional steps before trimming prose. Search and compilation remain two separate stages so a future reranker can select among compact candidates without exposing all Skill bodies.

`fallback` dependencies are stored for selection strategies but are not merged into a successful compiled Skill. Required tools missing from `available_tools` are returned in `missing_tools`; compilation does not execute them.
