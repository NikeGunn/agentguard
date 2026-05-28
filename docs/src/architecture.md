# Architecture

```
cmd/agentguard            — main entry point
internal/cli              — every CLI command (cobra)
internal/proxy            — stdio JSON-RPC bidirectional pump
internal/pipeline         — Stage interface + Chain runner + each stage
  transport.go            — rate limit + frame cap
  schema.go               — JSON-RPC 2.0 validation
  attestation.go          — tool-schema drift detector
  policy.go               — rule-pack evaluator
  scanner.go              — regex secret redactor
  ml.go                   — heuristic injection scorer
  circuit.go              — per-session failure breaker
internal/ml               — classifier (no CGO, no ONNX)
internal/policy           — YAML compiler + engine + builtin packs (embed.FS)
internal/store            — SQLite (modernc.org/sqlite) + WAL + batched writer
internal/dashboard        — chi HTTP server + SSE + embedded SPA assets
internal/agent_detect     — per-agent config detectors + atomic patcher
internal/registry         — npm / PyPI / GitHub metadata + trust score
internal/otel             — minimal OTLP/HTTP span exporter
```

## Single-process design

There is no daemon. `agentguard wrap` is the only long-lived process,
and it's spawned by the AI agent itself — it lives exactly as long as
the agent's session.

## Storage

- `~/.agentguard/data/agentguard.db` — SQLite, WAL mode, mode 0600
- `~/.agentguard/logs/` — rotating JSONL logs (M6+)
- `~/.agentguard/packs/` — user rule packs
- `~/.agentguard/config/` — reserved
- `~/.agentguard/bin/` — installed binary

## Concurrency model

- One goroutine per direction (stdin→upstream, upstream→stdout).
- One batched-writer goroutine per `Store`. Buffers up to 100 events
  for up to 10 ms, then commits in a single transaction.
- One broadcaster goroutine fan-outs committed tool calls to SSE
  subscribers.

Everything else is request-scoped.
