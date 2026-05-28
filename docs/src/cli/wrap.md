# agentguard wrap

Run an MCP server with AgentGuard inspection. You normally never call
this directly — `init` rewrites your agent configs to do it for you.

```
agentguard wrap --upstream-name <name> [flags] -- <command> [args...]
```

| Flag                  | Default              | Description                            |
|-----------------------|----------------------|----------------------------------------|
| `--upstream-name`     | required             | Server identifier (recorded in DB).    |
| `--pack`              | `default`            | Rule pack to evaluate.                 |
| `--rate-per-second`   | `100`                | Per-session token-bucket rate.         |
| `--max-frame-bytes`   | `8388608` (8 MiB)    | Hard cap on inspected frame size.      |
| `--no-inspect`        | `false`              | Bypass the inspection chain.           |
