# Database schema

AgentGuard stores everything in a single SQLite file at
`~/.agentguard/data/agentguard.db`. WAL mode, mode 0600.

Tables you can query directly with `sqlite3`:

| Table                  | Purpose                                          |
|------------------------|--------------------------------------------------|
| `agents`               | Detected installed AI agents.                    |
| `sessions`             | One row per `agentguard wrap` invocation.        |
| `mcp_servers`          | Servers we've seen (joined to per-call rows).    |
| `server_tools`         | Tool catalog snapshot per server.                |
| `server_attestations`  | Tool-schema-hash history (drift detection).      |
| `tool_calls`           | Every JSON-RPC frame the chain inspected.        |
| `tool_call_stages`     | Per-stage trace for each tool call.              |
| `policies`             | Compiled rule-pack snapshots.                    |
| `audit_log`            | Init/uninstall/pack-change events.               |

Full DDL: see `internal/store/migrations/0001_initial.sql`.

## Common queries

```sql
-- Top blocked tools in the last day
SELECT s.name, tc.tool_name, COUNT(*) AS blocks
FROM tool_calls tc JOIN mcp_servers s ON s.id = tc.server_id
WHERE tc.verdict = 'block'
  AND tc.started_at >= (strftime('%s','now')*1000 - 86400000)
GROUP BY s.name, tc.tool_name
ORDER BY blocks DESC
LIMIT 10;
```
