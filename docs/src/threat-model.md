# Threat model

## In scope

AgentGuard defends against compromised or malicious **MCP servers**
attacking your AI agent or your data:

- **Prompt injection** in tool responses (`ignore previous`, role
  takeover, DAN, encoded payloads, zero-width chars)
- **Tool poisoning** — silently changing tool descriptions or input
  schemas between sessions (rug pull)
- **Credential exfiltration** — secrets surfacing in tool args or
  responses (AWS, GitHub, Stripe, OpenAI, Anthropic, Google, Slack,
  SSH, JWT)
- **Runaway loops** — repeated block/error sessions tripping the
  per-session circuit breaker
- **Destructive shell** in tool args (`rm -rf /`, `DROP TABLE`, fork
  bomb, etc.)
- **Frame-size DoS** — hard 8 MiB cap on inspected frames

## Out of scope

AgentGuard does NOT defend against:

- A **compromised AI agent binary**. AgentGuard runs as a subprocess
  of the agent; the agent has full control of its own stdin/stdout
  outside our wrap.
- A **malicious LLM provider**. We don't sit between agent and LLM.
- **HTTP-only MCP servers**, except via the URL-rewrite path (M6+).
  Stdio is the primary surface today.
- **Disk-level attacks on `~/.agentguard`**. Mode-0600 helps; root
  reads it anyway. If your home directory is hostile, AgentGuard is
  not your top problem.

## Assumptions

- The user's machine is not actively compromised at root level.
- Go's TLS and JSON parsers are correct.
- modernc.org/sqlite implements WAL safely.
