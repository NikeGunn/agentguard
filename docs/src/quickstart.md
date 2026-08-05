# Quickstart

```bash
agentguard init         # detect + patch every installed agent
agentguard dashboard    # open http://127.0.0.1:7878
agentguard tail         # live TUI of tool calls
agentguard scan github  # send the canned attack corpus through the github MCP
agentguard uninstall    # roll everything back, byte-identical
```

## What `init` actually does

1. Creates `~/.agentguard/{bin,data,logs,packs,config}` with mode 0700.
2. Runs the SQLite migrations.
3. Detects every supported agent (Claude Code, Cline, Cursor, Codex CLI,
   Gemini CLI, Windsurf).
4. Backs up each agent's config to `<path>.agentguard.bak` byte-for-byte.
5. Rewrites every stdio MCP entry to invoke `agentguard wrap`
   transparently.

Idempotent. Re-running does nothing. `--dry-run` previews without
writing.

## Interactive checklist

```bash
agentguard init --interactive
```

Shows a Bubble Tea TUI so you can pick exactly which agents to patch.
Esc aborts cleanly.

## Verifying the install

```bash
agentguard doctor
```

Runs a homebrew-style health check: directory tree, database integrity,
detected agents, packs loaded, routing coverage.
