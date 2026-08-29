## Problem

What user, agent, operator, or contributor problem does this change solve?

## Solution

Describe the behavior and important tradeoffs. Keep implementation details focused on what reviewers need to verify.

## Verification

List only commands and manual checks that were actually run.

- [ ] `gofmt -w cmd internal tests`
- [ ] `GOWORK=off go test ./...`
- [ ] `GOWORK=off go vet ./...`
- [ ] `cd dashboard && npm run lint && npm run build` when Dashboard code changed
- [ ] `./build-release.sh host` when the embedded application or release flow changed

## Compatibility and data

- [ ] MCP routes, tools, schemas, and scope behavior are unchanged, or the change is documented below.
- [ ] Existing database data and configuration are preserved.
- [ ] Migrations are mirrored for every supported driver when applicable.
- [ ] Security and network-boundary implications were considered.

Compatibility, migration, or rollout notes:

## Documentation and UI

- [ ] Relevant documentation was updated.
- [ ] Visible Dashboard changes include a screenshot or short recording.
- [ ] Examples contain no secrets, personal data, or proprietary procedures.

## Related work

Closes #
Related Discussion:
