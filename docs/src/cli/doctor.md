# agentguard doctor

Homebrew-style health check.

```
agentguard doctor
```

Verifies the directory tree, runs `PRAGMA integrity_check`, counts
active agents and recorded `tool_calls`, re-detects each agent and
reports how many of its servers are still routed through us, and
lists the loaded rule packs.

Exit code is 0 unless any check fails outright.
