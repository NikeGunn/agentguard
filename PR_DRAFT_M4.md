# Milestone 4 — Observability & Visibility

Headline: AgentGuard now ships a **rich, live, localhost web dashboard**
plus the trust+attestation foundation that turns logs into actionable
signal.

## What landed

### `agentguard dashboard`
Spins up `http://127.0.0.1:7878` on demand. Localhost-only listener, no
auth surface, opens the user's browser automatically. Single-file
embedded UI (HTML + JS + CSS, <25 KB) with:

- **Overview**: 8 KPI tiles, per-minute calls/blocked sparkline (SVG,
  hand-rendered area + line), recent activity table
- **Tool calls**: full-history filter by verdict, live row-flash on SSE
- **Top tools**: ranked by call volume, with usage bars
- **MCP servers**: inventory with trust score, first/last seen, totals
- Dark + light themes, keyboard nav, mobile-friendly grid

Backed by the store's new broadcaster — every committed tool_call fans
out to subscribed SSE clients with sub-millisecond latency.

### Stage-2 server attestation
Inspects inbound `tools/list` responses, hashes the canonical
(name, description, inputSchema) triples, compares against the last
attestation row. Mismatch → `VerdictFlag` with `{old_hash, new_hash}`.
Detects "rug pull" tool-schema drift between sessions.

### Registry package
npm / PyPI / GitHub metadata fetchers with shared HTTP client + polite
UA. `TrustScore()` aggregates 0..100 from age, popularity, license,
repo, author, and recency.

### OTLP exporter
Hand-rolled OTLP/HTTP JSON span exporter, one span per ToolCall, empty
endpoint = no-op. No dependency on `go.opentelemetry.io` (kept the
binary lean).

### Interactive init
`agentguard init --interactive` shows a Bubble Tea checklist so the
user can deselect agents before patching. Esc aborts cleanly.

## Tests
`go test ./...` is green:
- `internal/dashboard/`: full API smoke + static serve
- `internal/pipeline/`: first-seen / drift / reorder-stable / outbound-skip
- `internal/registry/`: trust score floor/ceiling
- `internal/otel/`: end-to-end POST to an `httptest.Server` collector
- All M1-M3 tests still pass

## Deliberately deferred
- SvelteKit/Bun rebuild of the dashboard (vanilla UI hits the design bar)
- DuckDB analytics (CGO conflict; SQLite analytics scale fine for solo use)
- Full ML stage (M5)

## Try it
```bash
go install ./cmd/agentguard
agentguard init --interactive
agentguard dashboard
# wrap an MCP server in another terminal and watch tool_call events
# flash into the dashboard live.
```
