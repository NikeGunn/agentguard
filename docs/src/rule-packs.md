# Rule packs

Rule packs are YAML files that describe what to allow, block, flag, or
redact. AgentGuard ships two built-ins:

- `default` — secret redaction (AWS, GitHub, Stripe, OpenAI, Anthropic,
  Google, Slack, SSH, JWT) + injection flags + destructive shell blocks.
- `strict` — same plus blocks on inbound injection signatures.

## Layout

```yaml
name: my-pack
version: 1
rules:
  - id: block_destructive_psql
    when:
      tool:    "execute_sql"
      content_regex: "DROP\\s+TABLE"
    action: block
    reason: "destructive SQL"

  - id: redact_anthropic_keys
    when:
      direction:     inbound
      content_regex: "sk-ant-[A-Za-z0-9_-]{20,}"
    action: redact

  - id: require_approval_writes
    when:
      tool: "write_file"
    action: require_approval
```

## Actions

| Action             | Effect                                                     |
|--------------------|------------------------------------------------------------|
| `allow`            | Always allow. Useful as a positive override.               |
| `block`            | Terminate the frame with a JSON-RPC error to the agent.    |
| `flag`             | Record the verdict but pass the frame through.             |
| `redact`           | Hand off to the content scanner for byte-level rewrite.    |
| `require_approval` | Pause and wait for human approval (M6+).                   |
| `rate_limit`       | Apply a tighter per-rule rate limit.                       |

## User packs

Drop YAML files into `~/.agentguard/packs/`. Verify with:

```bash
agentguard pack verify user/my-pack
agentguard pack show   user/my-pack
agentguard pack list
```

## Replay

Test a new pack against your historic traffic before rolling it out:

```bash
agentguard replay --pack my-pack --since 24h
```

Prints a verdict-diff table for every call in the window.
