# Quick start: publish the first Skill

This tutorial walks through the complete lifecycle with `curl` and `jq`: initialize a project, create a draft, validate it, approve publication, prepare it for an agent, and report an execution result.

You can perform the authoring steps from the Dashboard instead. The HTTP examples are useful for understanding and testing the MCP contract directly.

## 1. Start SkillBox

```bash
make build
./skillbox -config ./configs/skillbox.yaml
```

The examples use project `demo` and service URL `http://127.0.0.1:8081`.

## 2. Initialize Teacher

```bash
curl -s http://127.0.0.1:8081/mcp/demo/teacher \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | jq
```

Initialization creates the URL project when it does not exist. Repeating it is safe.

## 3. Create a draft

```bash
curl -s http://127.0.0.1:8081/mcp/demo/teacher \
  -H 'Content-Type: application/json' \
  -d @- <<'JSON' | tee /tmp/skillbox-draft.json | jq
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "create_skill_draft",
    "arguments": {
      "slug": "verified-file-summary",
      "name": "Summarize a file with verification",
      "description": "Read a local text file and produce a source-grounded summary.",
      "purpose": "Prevent summaries based on guessed or stale file contents.",
      "when_to_use": "When the user asks to summarize a local text document.",
      "instructions": "Read the requested file before drafting the answer. Distinguish facts in the file from your own inference.",
      "success_criteria": [
        "The requested file was read successfully.",
        "Every factual claim is supported by the file.",
        "The final response contains no internal tool protocol."
      ],
      "domains": ["documents"],
      "intents": ["summarize"],
      "steps": [
        {
          "position": 1,
          "title": "Read the source",
          "instruction": "Read the complete requested text file.",
          "is_required": true,
          "expected_result": "The current file contents are available."
        },
        {
          "position": 2,
          "title": "Draft the summary",
          "instruction": "Extract the central claims and supporting details without inventing facts.",
          "is_required": true,
          "expected_result": "A concise source-grounded draft."
        },
        {
          "position": 3,
          "title": "Verify the answer",
          "instruction": "Check each factual statement against the source and remove unsupported claims.",
          "is_required": true,
          "expected_result": "The final summary is traceable to the file."
        }
      ],
      "tools": [
        {
          "tool_name": "read_file",
          "requirement": "required",
          "purpose": "Load the authoritative source text."
        }
      ],
      "context_requirements": []
    }
  }
}
JSON
```

Save the generated Skill ID:

```bash
SKILL_ID=$(jq -r '.result.structuredContent.id' /tmp/skillbox-draft.json)
echo "$SKILL_ID"
```

The server forces the draft into project `demo` regardless of model-supplied scope fields.

## 4. Validate the draft

```bash
curl -s http://127.0.0.1:8081/mcp/demo/teacher \
  -H 'Content-Type: application/json' \
  -d "{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":{\"name\":\"validate_skill\",\"arguments\":{\"skill_id\":\"$SKILL_ID\"}}}" | jq
```

Continue only when `structuredContent.valid` is `true`. Validation issues are returned as structured data.

## 5. Propose, approve, and publish

Create a publication proposal:

```bash
curl -s http://127.0.0.1:8081/mcp/demo/teacher \
  -H 'Content-Type: application/json' \
  -d "{\"jsonrpc\":\"2.0\",\"id\":4,\"method\":\"tools/call\",\"params\":{\"name\":\"create_skill_proposal\",\"arguments\":{\"skill_id\":\"$SKILL_ID\",\"summary\":\"Publish the first verified summary procedure\"}}}" \
  | tee /tmp/skillbox-proposal.json | jq
```

```bash
PROPOSAL_ID=$(jq -r '.result.structuredContent.id' /tmp/skillbox-proposal.json)
```

Approve the proposal:

```bash
curl -s http://127.0.0.1:8081/mcp/demo/teacher \
  -H 'Content-Type: application/json' \
  -d "{\"jsonrpc\":\"2.0\",\"id\":5,\"method\":\"tools/call\",\"params\":{\"name\":\"approve_skill_proposal\",\"arguments\":{\"proposal_id\":\"$PROPOSAL_ID\",\"note\":\"Reviewed in quick start\"}}}" | jq
```

Publish it:

```bash
curl -s http://127.0.0.1:8081/mcp/demo/teacher \
  -H 'Content-Type: application/json' \
  -d "{\"jsonrpc\":\"2.0\",\"id\":6,\"method\":\"tools/call\",\"params\":{\"name\":\"publish_skill\",\"arguments\":{\"proposal_id\":\"$PROPOSAL_ID\"}}}" | jq
```

The Skill is now active and visible to Student in project `demo`.

## 6. Search as Student

Initialize Student:

```bash
curl -s http://127.0.0.1:8081/mcp/demo \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":7,"method":"initialize","params":{}}' | jq
```

Search returns compact candidates rather than complete Skill bodies:

```bash
curl -s http://127.0.0.1:8081/mcp/demo \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"search_skills","arguments":{"task":"Summarize this local file without inventing facts","limit":5}}}' | jq
```

## 7. Prepare the Skill

Preparation compiles the selected Skill for the current model and available tools:

```bash
curl -s http://127.0.0.1:8081/mcp/demo \
  -H 'Content-Type: application/json' \
  -d "{\"jsonrpc\":\"2.0\",\"id\":9,\"method\":\"tools/call\",\"params\":{\"name\":\"prepare_skill\",\"arguments\":{\"task\":\"Summarize this local file\",\"skill_id\":\"$SKILL_ID\",\"available_tools\":[\"read_file\"],\"model\":{\"provider\":\"local\",\"name\":\"example\",\"context_window\":8192},\"max_skill_tokens\":1200}}}" \
  | tee /tmp/skillbox-prepared.json | jq
```

The response contains compiled instructions, steps, tool requirements, context requirements, criteria, and estimated token usage. SkillBox does not execute the procedure; the agent does.

Save the exact prepared version for telemetry:

```bash
SKILL_VERSION=$(jq -r '.result.structuredContent.version' /tmp/skillbox-prepared.json)
```

## 8. Report the outcome

After execution, Student can record evidence:

```bash
curl -s http://127.0.0.1:8081/mcp/demo \
  -H 'Content-Type: application/json' \
  -d "{\"jsonrpc\":\"2.0\",\"id\":10,\"method\":\"tools/call\",\"params\":{\"name\":\"report_skill_result\",\"arguments\":{\"skill_id\":\"$SKILL_ID\",\"skill_version\":$SKILL_VERSION,\"task_summary\":\"Summarized a local text file\",\"status\":\"success\",\"success\":true,\"trajectory\":[]}}}" | jq
```

Use the Dashboard to inspect the execution and model statistics.

## Next steps

- Read the complete [MCP contract](MCP.md).
- Explore the [Skill model](SKILL_MODEL.md).
- Connect your agent to Student for runtime use and Teacher for controlled authoring.
- Compare the same task with and without the prepared Skill.
- Share the measured result in GitHub Discussions.
