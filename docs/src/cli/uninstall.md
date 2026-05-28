# agentguard uninstall

Roll every patched config back, byte-for-byte.

```
agentguard uninstall [--purge]
```

| Flag       | Default | Description                                  |
|------------|---------|----------------------------------------------|
| `--purge`  | `false` | Also remove `~/.agentguard` recursively.     |

Without `--purge`, audit data (SQLite DB, logs) is preserved.
