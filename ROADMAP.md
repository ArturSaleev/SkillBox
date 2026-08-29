# Roadmap

SkillBox is developed in public. This roadmap describes direction, not a promise of dates or a guarantee that every item will be implemented exactly as written.

## Current foundation

- [x] Student and Teacher MCP routes
- [x] URL-owned project isolation
- [x] Structured Skill model
- [x] Draft, validation, proposal, approval, publication, and rollback lifecycle
- [x] Dependency-aware compilation and token budgeting
- [x] Execution evidence and model statistics
- [x] SQLite, MySQL, and PostgreSQL adapters
- [x] Database-wide embedded Dashboard
- [x] Single-binary macOS and Linux releases
- [x] Docker build with persistent SQLite volume
- [x] Cross-driver storage and MCP contract tests

## Near term: make adoption easy

- [ ] Publish signed release artifacts and checksums
- [ ] Add automated GitHub CI for Go, Dashboard, and release smoke tests
- [ ] Provide copy-paste integration examples for popular MCP clients and agent frameworks
- [ ] Add a small import/export format for portable Skills
- [ ] Add example Skills that contain no proprietary data
- [ ] Improve empty states, error recovery, and accessibility in the Dashboard
- [ ] Define stable versioning and compatibility policy

## Evaluation and quality

- [ ] Publish a reproducible weak-model baseline vs. weak-model-plus-Skill benchmark
- [ ] Add evaluation datasets and result comparison tools
- [ ] Make execution trajectories easier to inspect and redact
- [ ] Surface Skill quality signals without turning popularity into correctness
- [ ] Add duplicate detection and controlled Skill merging
- [ ] Explore reranking after compact candidate search

## Operations and security

- [ ] Design optional authentication without weakening URL-owned scope
- [ ] Add audit events for administrative lifecycle actions
- [ ] Document reverse-proxy recipes and secure multi-user deployments
- [ ] Add backup, restore, and migration tooling
- [ ] Define retention and redaction controls for execution evidence
- [ ] Add health and operational observability suitable for deployment

## Community and ecosystem

- [ ] Publish contributor-friendly architectural decision records
- [ ] Label and maintain `good first issue` tasks
- [ ] Add adapters and SDK examples in multiple languages
- [ ] Establish a neutral Skill interchange discussion with other procedural-memory projects
- [ ] Collect real-world use cases and measurable before/after results

## Non-goals for the current core

- Running an LLM inside SkillBox
- Executing an agent's business tools
- Replacing a knowledge base or document RAG system
- Trusting model-supplied identity or authorization context
- Claiming that procedures eliminate the capability limits of the underlying model

## Influence the roadmap

Open a GitHub Discussion with:

1. the workflow your agent repeatedly fails to complete;
2. the model and tools involved;
3. the procedure you believe would help;
4. how success can be measured;
5. whether the change belongs in core, an adapter, or documentation.

Evidence from real workflows has more weight than feature count.
