# agentguard replay

Re-run the inspection pipeline against historic tool calls.

```
agentguard replay [--pack name] [--since 24h] [--session id] [--server name] [--limit 100]
```

Prints a diff table:

```
=  14:03:12  github          create_issue        allow -> allow
≠  14:03:14  github          read_file           allow -> block
```

Use it to vet a new rule pack against real traffic before rolling it
out.
