# Security Policy

## Reporting a vulnerability

**Preferred:** Use GitHub's private vulnerability reporting →
[Report a vulnerability](https://github.com/NikeGunn/agentguard/security/advisories/new).
Only the maintainer sees the report.

If GitHub isn't an option, email **knewboy.nykhil@gmail.com** with the
subject line `[agentguard] security`. Do not open a public GitHub
issue for a security report.

We aim to:

- Acknowledge within **3 business days**.
- Ship a fix or mitigation within **14 days** for high-severity reports.

## Supported versions

| Version  | Supported          |
|----------|--------------------|
| 1.0.x    | ✅ security fixes  |
| < 1.0    | ❌ no longer       |

## Scope

**In scope:**
- The `agentguard` binary and every command it ships.
- The local dashboard served at `127.0.0.1:7878`.
- Official rule packs under `internal/policy/builtin/`.
- The install scripts in `scripts/`.
- The release workflow and cosign-signed artifacts.

**Out of scope:**
- LLMs and model providers themselves.
- Third-party MCP servers (please report to their maintainers).
- Community-published rule packs (please report to the pack author).
- Bugs in dependencies — report upstream; we'll bump the version.

## Disclosure

Once a fix is shipped, the original reporter is credited in
`CHANGELOG.md` and a GitHub Security Advisory is published from the
private report. CVE assignment via GitHub's CNA.

## Hall of fame

Credited reporters will be listed here once we ship our first
security-fix release.
