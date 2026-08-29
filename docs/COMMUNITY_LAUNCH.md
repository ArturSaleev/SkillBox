# Community launch kit

This file contains a ready-to-post first GitHub Discussion and a practical checklist for inviting early users and contributors. Adapt the personal introduction, but keep technical claims aligned with the repository.

## Recommended first Discussion

**Category:** General

**Title:** Can procedural memory make local AI agents more reliable?

**Body:**

> I have open-sourced SkillBox, an MCP service for reusable AI procedures.
>
> The idea came from a recurring problem: an agent can complete a workflow successfully once and then fail to reproduce the same sequence later. A prompt snippet alone is usually not enough. The agent also needs trigger conditions, trusted context requirements, tool requirements, ordered steps, failure prevention, success criteria, versions, and execution evidence.
>
> SkillBox stores that procedural layer separately from the model and exposes it through MCP:
>
> - Student searches and compiles a relevant published Skill for execution.
> - Teacher creates, validates, reviews, publishes, and rolls back Skills.
> - Project scope comes from the URL and is enforced by the server.
> - A database-wide Dashboard is embedded into the same Go binary.
> - SQLite, MySQL, and PostgreSQL are supported.
>
> My main interest is local and smaller models. I do not believe a procedure magically turns a small model into a frontier model. The useful question is measurable: does the same model complete a real workflow more consistently with a well-designed Skill than without one?
>
> I would love feedback from people building agents, MCP servers, local-model systems, evaluations, or workflow automation:
>
> 1. Which repeated agent failure would you want to turn into a reusable procedure?
> 2. What should a portable Skill format contain?
> 3. Which MCP clients or frameworks should get the first integration examples?
> 4. What benchmark would convince you that procedural memory is useful?
> 5. Which part of the current architecture would you challenge?
>
> The project is early, MIT-licensed, and looking for practical use cases, critical feedback, documentation help, integrations, and contributors. If you try it, please share both successes and failures.

## Suggested follow-up Discussions

1. **Show and tell:** “Share a workflow your agent should remember”
2. **Ideas:** “What belongs in a portable Skill interchange format?”
3. **Q&A:** “Help us test SkillBox with MCP clients and local models”
4. **Evaluation:** “Designing a fair baseline vs. Skill-assisted benchmark”
5. **Security:** “Authentication and multi-user deployment without trusting model identity”

## Good early contributor tasks

- Test the ten-minute quick start on a clean machine.
- Add one integration guide for an MCP client or agent framework.
- Contribute a non-proprietary example Skill with a measurable outcome.
- Review Dashboard accessibility and keyboard behavior.
- Run MySQL or PostgreSQL contract tests and report environment details.
- Design a benchmark that compares the same model with and without a Skill.
- Review English or Russian documentation for clarity.
- Identify ambiguous names, schemas, or error messages.

## Launch checklist

### Repository

- [ ] Make the repository public.
- [ ] Confirm the MIT License is visible on GitHub.
- [ ] Enable Issues.
- [ ] Enable Discussions and create categories with these slugs: General (`general`), Ideas (`ideas`), Q&A (`q-a`), Show and tell (`show-and-tell`). The filenames in `.github/DISCUSSION_TEMPLATE/` must match those slugs.
- [ ] Enable private vulnerability reporting if available.
- [ ] Add repository description and topics such as `mcp`, `ai-agents`, `local-llm`, `procedural-memory`, `golang`, and `nextjs`.
- [ ] Add a social preview image when the visual identity is ready.
- [ ] Publish the first tagged release with checksums.

### Credibility

- [ ] Run all documented verification commands on a clean checkout.
- [ ] Record the exact platforms and databases tested.
- [ ] Publish at least one end-to-end example Skill.
- [ ] Publish one honest before/after evaluation.
- [ ] Keep the no-authentication warning prominent.
- [ ] Avoid claims that a small model becomes equivalent to a cloud frontier model.

### Outreach

- [ ] Post the first GitHub Discussion.
- [ ] Invite a small number of relevant builders personally instead of mass messaging.
- [ ] Share one technical problem and result, not only a repository link.
- [ ] Ask specific questions that make feedback easy.
- [ ] Respond quickly and respectfully to the first Issues and Discussions.
- [ ] Turn accepted ideas into labeled, scoped Issues.

## Short announcement

> I open-sourced SkillBox: an MCP service for versioned AI procedures with project-scoped Student/Teacher endpoints, execution evidence, and an embedded admin Dashboard. I am exploring whether explicit procedural memory can make local and smaller models complete real workflows more consistently. The project is early and I am looking for critical feedback, integration tests, real use cases, and contributors: https://github.com/ArturSaleev/SkillBox

## What not to promise

- Do not call the project production-ready before authentication, upgrade policy, and release processes mature.
- Do not claim guaranteed quality improvements without a task-specific baseline.
- Do not describe SkillBox as an LLM, autonomous agent runtime, or knowledge base.
- Do not imply that execution telemetry is safe for sensitive data by default.
- Do not hide known limitations; early contributors value clear boundaries.
