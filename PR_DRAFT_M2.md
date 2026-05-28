# Milestone 2 — Cheap-path inspection: stages 0/1/3/4 + policy engine

## Summary

The pipeline now does something. Every JSON-RPC frame through
`agentguard wrap` runs the **transport guard → schema validator → policy
engine → content scanner** chain. Secrets are redacted on the wire,
malformed frames are rejected, configurable rule packs block or flag
matches, and a 100 req/s per-session token bucket keeps runaway agents in
check. All four stages combined cost **~33 μs/op** on a clean frame — two
orders of magnitude under the 5 ms p99 budget.

## What landed

### Stages

- **`internal/pipeline/transport.go`** — token-bucket rate limit per
  session, hard max-frame-bytes cap. Defaults: 100 req/s, 8 MiB.
- **`internal/pipeline/schema.go`** — JSON-RPC 2.0 envelope validation;
  rejects bad version, missing `method` on outbound, both `result` and
  `error` set on inbound.
- **`internal/pipeline/policy.go`** — adapter that calls
  `policy.Engine.Evaluate` and maps Actions to Verdicts. Redact actions
  flag the frame (the scanner does the byte rewrite); block actions
  short-circuit.
- **`internal/pipeline/scanner.go`** — independent built-in regex bank for
  AWS/GitHub/Stripe/OpenAI/Anthropic/Google/Slack/SSH/JWT keys plus
  injection signatures. Secret hits return `Transform` with the match
  replaced by `[REDACTED:<kind>]`; injection hits return `Flag`. Runs even
  when no pack is loaded.

### Policy engine

- **`internal/policy/engine.go`** — `Engine` with snapshot-copy semantics
  for hot reload, `Evaluate(Input) Decision` first-terminal-wins with flag
  accumulation, rule pre-compilation (regex compiled once at load).
- **`internal/policy/compiler.go`** — YAML pack parsing (goccy/go-yaml),
  validates action names, compiles `content_regex` patterns, friendly
  errors with pack name + rule index.
- **`internal/policy/packs.go`** — `embed.FS` over `builtin/*.yaml` plus
  `LoadBuiltin(name)` and `ListBuiltin()`.
- **`internal/policy/builtin/{default,strict}.yaml`** — two production-ready
  rule packs that ship in the binary.

### Wiring

- **`internal/cli/wrap.go`** — `buildChain` assembles the real cheap-path
  chain from the chosen pack. New flags: `--pack` (default | strict),
  `--rate-per-second`, `--max-frame-bytes`, `--no-inspect`.
- **`internal/proxy/stdio.go`** — chain message now carries
  `SessionID`/`ServerName`/`Method`; honours `VerdictTransform` by
  forwarding the rewritten bytes; records `verdict` and `verdict_reason`
  for transformed/flagged/blocked frames on either direction.
- **`internal/pipeline/stage.go`** — `Chain.Run` propagates Transforms into
  the message for downstream stages.

### Tests

```
$ go vet ./...   → clean
$ go test ./...  → all green

ok  github.com/agentguard/agentguard/e2e                6.628s
ok  github.com/agentguard/agentguard/internal/pipeline  0.040s
ok  github.com/agentguard/agentguard/internal/policy    0.032s
ok  github.com/agentguard/agentguard/internal/store     0.139s
```

New unit suites:
- `transport_test.go` (3 cases): under-rate, burst exhaustion, frame size.
- `schema_test.go`    (6 cases): valid request, valid response, bad JSON,
  bad version, outbound w/o method, result+error both set.
- `scanner_test.go`   (5 cases): clean, AKIA redaction, GH redaction,
  injection flag, secret-beats-injection.
- `engine_test.go`    (5 cases): tool match, no match, first-terminal-wins,
  flag accumulation, content regex.
- `compiler_test.go`  (5 cases): happy path, bad action, bad regex,
  builtin load, builtin list.

New e2e tests:
- `TestWrapRedactsSecretInToolResponse` — drives the real binary, mock
  emits a fake AKIA key, asserts the agent receives
  `[REDACTED:aws_access_key]` and that the SQLite row has
  `verdict='transform'`.
- `TestWrapBlocksOnPolicyMatch` — strict pack, mock emits "ignore previous
  instructions", asserts the agent gets a `-32099` block error and the
  injection payload never reaches it.

### Bench M2 baselines

```
goos: windows  goarch: amd64  cpu: AMD Ryzen 5 4600H
BenchmarkM2ChainCleanFrame-12     35,326    32,623 ns/op   2,325 B/op   21 allocs
BenchmarkM2ChainSecretFrame-12    48,951    23,381 ns/op   3,902 B/op   45 allocs
BenchmarkPipelineChainEmpty-12   184M       7.05 ns/op         0 B/op    0 allocs (regression -3%)
BenchmarkStoreSubmit-12          2.07M      521 ns/op        224 B/op    2 allocs
```

The cheap-path target is `< 5 ms p99` for the full proxy roundtrip
(`requirements.md §5.1`). Inspection alone is **0.033 ms** on a clean
frame; we are ~150× under budget with three stages still to land. The 10%
regression gate is now anchored on these two new benches.

### Live smoke test

```
$ printf '{"id":1,"method":"tools/call","params":{"name":"leak_secret"},"jsonrpc":"2.0"}\n' \
  | ./bin/agentguard wrap --upstream-name mock -- ./bin/mock_mcp_server
{"jsonrpc":"2.0","id":1,"result":{"content":[{"text":"here is the key: [REDACTED:aws_access_key]","type":"text"}]}}

$ printf '{"id":1,"method":"tools/call","params":{"name":"injection"},"jsonrpc":"2.0"}\n' \
  | ./bin/agentguard wrap --pack strict -- ./bin/mock_mcp_server
{"jsonrpc":"2.0","id":1,"error":{"code":-32099,"message":"Blocked by AgentGuard: policy:block:block-ignore-previous-instructions"}}
```

## Deliberately deferred

- **Stage 2 — server attestation** (`internal/pipeline/attestation.go`):
  binary hashing + schema-drift / rug-pull detection. Needs the registry
  package from Milestone 3 (npm/PyPI/GitHub metadata fetchers) and the
  `server_tools` upsert path. M3 work.
- **Stage 5 — ML classifier**: still a `VerdictSkip` stub, `// TODO M5`.
- **Stage 6 — circuit breaker**: identical-call loop detector, per-tool /
  per-session spend caps. Needs telemetry from previous calls — designed
  in M5 alongside replay.
- **`require_approval`** action: returns Block today. Real approval flow
  needs the dashboard SSE channel from Milestone 4.
- **`rate_limit`** policy action: parsed and recorded, but per-rule
  granularity (vs per-session) waits on the dashboard policy UI in M4.

## How to review

1. `cat internal/policy/builtin/default.yaml` — the actual rules users get
   on day one.
2. `internal/policy/engine.go` lines 80-115 — the evaluation loop.
3. `internal/pipeline/scanner.go` line 50-65 — secret-vs-flag precedence
   logic; the test `TestScannerSecretBeatsInjection` locks this.
4. `internal/proxy/stdio.go` `handleFrame` — how Transform/Block/Flag map
   to bytes-on-wire + DB rows.
5. `make bench` — record the numbers for the regression gate.

## What's next (Milestone 3)

CLI completeness and the 10-second install story:
`agentguard init`, agent auto-detection (Claude Code / Cursor / Codex /
Gemini / Windsurf), config-file rewriting + backup, `agentguard tail` TUI,
`agentguard scan`, `agentguard doctor`, and the daemon supervisor. Stage 2
attestation gets bolted on top of the registry package M3 introduces.

---

**Milestone 2 complete. Ready for review before Milestone 3.**
