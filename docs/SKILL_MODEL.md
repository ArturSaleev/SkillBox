# Skill model

A Skill combines human-readable routing metadata with an executable procedure. Scope is `global`, `workspace`, `project`, or `user`; lifecycle status is `draft`, `active`, `deprecated`, or `archived`. User scope requires an explicit `owner_user_id` supplied from trusted runtime identity.

Routing metadata includes domains, intents, object types, tags, keywords, capabilities, compatibility, ownership, priority, and tool requirements. Procedure data includes instructions, ordered required/optional steps, success criteria, tool requirements, context requirements, examples, and typed dependencies (`requires`, `extends`, `uses`, `fallback`). Fallback edges are recorded for selection strategies but are not merged into a successful compile.

Every create/update/rollback writes an immutable JSON snapshot and advances `current_version`. Rollback never destroys history; it creates a new current version from the selected snapshot.

Compilation applies dependencies before the selected Skill, detects cycles, and deduplicates rules. For models with context windows up to 8K, the highest-priority example is retained. Larger models omit examples by default. Budget reduction removes examples, optional context, and optional steps before trimming prose. Search and compilation remain two separate stages so a future reranker can select among compact candidates without exposing all Skill bodies.
