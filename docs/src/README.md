# AgentGuard

> Zero-trust security sidecar for AI agents — Claude Code, Cursor, Codex, Gemini CLI, Windsurf.

AgentGuard sits transparently between your AI agent and the MCP/A2A tools it
calls. Every request and response goes through a five-stage inspection
pipeline that catches **prompt injection**, **tool poisoning**, **rug
pulls**, **runaway loops**, and **credential exfiltration** before they
reach your model — or your customers' data.

## In 30 seconds

```bash
curl -fsSL https://agentguard.dev/install | sh
agentguard init        # detects + patches every installed agent
agentguard dashboard   # opens http://127.0.0.1:7878
```

That's it. The next tool call your agent makes is now inspected.

## What it is

- **Single Go binary**, no daemon, no Docker, no kernel modules.
- **Local-only**. No cloud, no telemetry, no account. Your secrets stay
  on your machine.
- **MIT licensed**.
- **<5ms p99 overhead** on the cheap inspection path.

## What it isn't

- A replacement for your IDE's safety prompts. AgentGuard is a hard
  enforcement layer; your IDE still asks for approval where it should.
- A model provider proxy. We sit between agent and tool, not between
  agent and LLM.
- A SaaS. There is no cloud version. We don't want your data.
