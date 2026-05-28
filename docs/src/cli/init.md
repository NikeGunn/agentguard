# agentguard init

Detect installed AI agents and route their MCP traffic through
AgentGuard.

```
agentguard init [--interactive] [--dry-run] [--skip-server name,...]
```

| Flag                | Default | Description                                   |
|---------------------|---------|-----------------------------------------------|
| `--interactive`     | `false` | Bubble Tea checklist to pick agents.          |
| `--non-interactive` | `true`  | Accept all detections silently.               |
| `--dry-run`         | `false` | Report what would change without writing.     |
| `--skip-server`     | —       | Comma-separated MCP server names to leave alone. |
| `--home`            | `$HOME` | Override the home directory (tests).          |

Every original config is preserved at `<path>.agentguard.bak` and
restored byte-for-byte by `agentguard uninstall`.
