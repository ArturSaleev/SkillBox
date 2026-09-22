# SkillBox Benchmarks

SkillBox Bench compares the same model on the same task in two paired variants:

```text
Baseline    = task + task MCP tools
With Skill  = task + the same task MCP tools + one frozen prepared Skill
```

The SkillBox Student connection is used by the runner, not exposed as a task tool. This isolates the effect of the compiled procedure. The Student connection therefore cannot also be selected as a task MCP server.

## Create a case

Open `http://127.0.0.1:8091`, configure providers and MCP servers, then select **Benchmarks → New benchmark**.

A case contains:

- the exact task prompt;
- one dedicated SkillBox Student MCP connection;
- the published Skill ID;
- zero or more task/data MCP servers;
- one or more provider/model targets;
- 1–20 paired repetitions;
- required phrases, forbidden phrases, and required tools for deterministic grading.

Required tools use either a raw tool name or a qualified `server_id.tool_name` value.

## Paired execution

Every repetition executes both variants. Their order alternates between repetitions to reduce ordering bias:

```text
repetition 1: Baseline → With Skill
repetition 2: With Skill → Baseline
```

For `With Skill`, the runner calls `prepare_skill` with the task, selected model, and discovered task-tool names. The returned Skill version and compiled procedure are frozen into that trial. Preparation time is included in end-to-end duration and also stored separately.

Each run stores a JSON snapshot of the complete case so later edits do not change the meaning of historical results.

## Grading

The deterministic grader creates one check for successful completion and one check for every configured rule:

- required phrase is present, case-insensitively;
- forbidden phrase is absent;
- required MCP tool appears in the trajectory.

Quality is the percentage of passed checks. A trial passes only when every check passes. Provider errors, MCP errors, timeouts, and agent step-limit failures are stored as failed trials rather than aborting the complete comparison.

## Evidence

Every trial records:

- requested provider and model;
- variant and repetition;
- final response or error;
- end-to-end and Skill preparation duration;
- input and output token counts;
- MCP tool trajectory;
- quality score and individual grader checks;
- exact prepared Skill ID and version for `With Skill`.

The Dashboard groups results by provider/model and shows Baseline quality, With Skill quality, improvement, pass rates, duration, tokens, and tool-call counts.

## Current boundaries

- Runs are sequential to avoid cross-trial contamination in mutable MCP test environments.
- Use isolated test MCP servers or resettable fixtures for write operations.
- Model output is not trusted to grade itself.
- The current grader is deterministic; blind LLM judging and human review can be added as separate score sources later.
- Skill authoring through Teacher remains separate from evaluation. Publish and freeze the Skill before starting a run.
