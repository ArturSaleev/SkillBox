# Security policy

## Supported versions

SkillBox is currently an early-stage project. Security fixes are applied to the latest code on `main`; older commits and independently packaged builds are not maintained as separate supported versions yet.

## Reporting a vulnerability

Please do not disclose a suspected vulnerability in a public Issue or Discussion.

1. Use GitHub's private vulnerability reporting for this repository when it is available under the **Security** tab.
2. If private reporting is unavailable, contact the maintainer through the [ArturSaleev GitHub profile](https://github.com/ArturSaleev) and request a private channel. Do not include exploit details in a public message.
3. Include affected versions, reproduction steps, impact, and any suggested mitigation.
4. Remove credentials, personal information, customer data, and proprietary procedures from the report.

The maintainer will acknowledge a complete report when possible, investigate it, coordinate a fix, and credit the reporter unless anonymity is requested. No fixed response-time SLA is promised while the project is maintained by a small community.

## Current security boundary

SkillBox currently has no application-level authentication or authorization challenge. The Student endpoint, Teacher endpoint, Admin API, and Dashboard must be treated as trusted-network services.

Safe deployment options include:

- bind to `127.0.0.1` for local-only use;
- place SkillBox behind an authenticated reverse proxy;
- restrict access through a private network, VPN, firewall, or service mesh;
- use separate instances when trust boundaries require physical isolation.

Do not expose `:8081` directly to the public Internet. The Teacher endpoint can author and publish procedures, and the Dashboard provides administrative visibility across the configured database.

## Sensitive data

Skills and execution trajectories may contain operational instructions or task summaries. Avoid storing secrets, raw credentials, access tokens, personal data, or complete proprietary documents. External context should be represented by a requirement or trusted lookup, not copied into Skill instructions when it is sensitive.

Database files, DSNs, configuration, release bundles, and backups must be protected according to the sensitivity of the stored procedures and evidence.

## Security design principles

- Project scope is derived from the URL and applied server-side.
- Model-supplied workspace and project identifiers do not override the route scope.
- Student sees only active global or route-project Skills.
- Dashboard administrative HTTP endpoints are read-only; lifecycle writes use Teacher MCP.
- Unknown YAML fields are rejected to reduce silent configuration drift.
- SQLite foreign keys, busy timeout, and WAL mode are enabled.

Security improvements are welcome through private reports and scoped design Discussions.
