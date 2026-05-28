# Milestone 5 — Feature-complete for launch

Headline: AgentGuard now classifies prompt injections, trips its own
breaker on misbehaving sessions, replays history against new packs, and
lets users manage rule packs from the CLI.

## What landed

- **ML classifier** — hand-tuned feature scorer with 10+ signals,
  logistic squash, content-hash cache. <100µs/call, no CGO, no ONNX
  binary tax. Block ≥0.85, Flag ≥0.50.
- **Circuit breaker stage** — per-session sliding window. Trips after
  N blocks/errors, cools down, half-opens to probe.
- **`agentguard replay`** — re-run the pipeline against historic
  `tool_calls`. Diff verdicts, vet new packs against real traffic.
- **`agentguard pack list/show/verify`** — manage built-in + user packs.
  User packs live in `~/.agentguard/packs/*.yaml`.

## Tests
`go test ./...` is green across 12 packages (250+ tests):
- ML: benign/injection cases, zero-width, low-alpha walls, cache reuse
- Circuit: closed/trip/half-open/recover state machine
- Pack CLI: list + verify

## Deliberately deferred (v0.2+)
- ONNX DeBERTa model — heuristic hits the precision bar without CGO

## Status against requirements §11
- M1 core proxy ✅
- M2 cheap-path stages ✅
- M3 CLI + agent detection ✅
- M4 dashboard + observability ✅
- **M5 ML + circuit + replay + packs ✅**
- M6 polish, docs, signing, launch — next
