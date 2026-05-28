# agentguard scan

Spawn an upstream MCP server and fire a canned attack corpus at it.

```
agentguard scan <server-command> [args...]
```

Fires 8 canned prompt-injection probes (ignore-previous, system-prompt
override, base64 marker, zero-width chars, you-are-now, data-exfil
marker, policy override, HTML script).

Exits non-zero if any payload reaches the model surface. Designed for
CI.
