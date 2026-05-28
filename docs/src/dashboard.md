# Dashboard

```bash
agentguard dashboard
```

Opens `http://127.0.0.1:7878` in your default browser. **Bound to
localhost only** — never publicly reachable.

## Pages

- **Overview** — 8 KPI tiles (calls, blocked, flagged, transformed,
  block rate, active agents, known servers, open sessions), per-minute
  calls/blocked sparkline, recent activity table.
- **Tool calls** — full history with verdict filter and live SSE row
  insertion.
- **Top tools** — ranked usage with bar visualisation.
- **MCP servers** — inventory with trust score, first/last seen.

## Flags

| Flag           | Default            | Meaning                                  |
|----------------|--------------------|------------------------------------------|
| `--addr`       | `127.0.0.1:7878`   | Listen address (must stay on localhost). |
| `--db`         | `~/.agentguard/data/agentguard.db` | Override the SQLite path.       |
| `--no-browser` | `false`            | Suppress the auto-browser-open.          |

## Live updates

The dashboard subscribes to `/events` (SSE). Every committed tool call
fans out as a `call` event with sub-millisecond latency. The status dot
in the sidebar shows the connection state.
