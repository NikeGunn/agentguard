# Security Policy

## Reporting a vulnerability

Please email **security@agentguard.dev**. Do not open a public GitHub issue.

We aim to acknowledge within **3 business days** and to ship a fix or
mitigation within **14 days** for high-severity reports.

GPG key: published at `https://agentguard.dev/.well-known/security.asc`
once the project goes public.

## Supported versions

AgentGuard is pre-1.0. Only the latest minor release receives security fixes.

## Scope

In scope:
- The `agentguard` binary
- The dashboard served at `localhost:7878`
- Official rule packs under `packs/`
- The npm wrapper at `@agentguard/cli`

Out of scope:
- The LLMs themselves
- Third-party MCP servers (please report to their maintainers)
- User-published community rule packs (please report to the pack author)

## Bug bounty

A formal bounty program will launch alongside the Pro tier. Until then, the
project lists every credited reporter in `CHANGELOG.md` and on the website.
