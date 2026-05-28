# Milestone 3 — CLI completeness + the install story

## Summary

The 10-second install loop now works end-to-end. `agentguard init` finds
every Claude Code / Cursor / Codex / Gemini CLI / Windsurf config on the
machine, backs each one up byte-for-byte, and rewrites the MCP entries to
route through `agentguard wrap`. `agentguard uninstall` restores everything
byte-identical. `doctor`, `tail`, and `scan` ship in usable form.

## What landed

### Agent detection (`internal/agent_detect/`)

| Agent       | Path(s) checked                                                          | Format |
|-------------|--------------------------------------------------------------------------|--------|
| Claude Code | `~/.claude.json`, `~/.claude/mcp_servers.json`, `~/.claude/settings.json`, `~/.config/claude/config.json` | JSON |
| Cursor      | `~/.cursor/mcp.json`                                                      | JSON |
| Codex CLI   | `~/.codex/config.toml`                                                    | TOML |
| Gemini CLI  | `~/.gemini/settings.json`, `~/.config/gemini-cli/mcp.json`                | JSON |
| Windsurf    | `~/.codeium/windsurf/mcp_config.json`, `~/.config/windsurf/mcp_servers.json` | JSON |

Each detector implements a tiny interface. `DetectAll(home)` runs every
detector and returns the union — used by `init` and `doctor`.

### Config patcher (`internal/agent_detect/patcher.go`)

- Reads original bytes → writes them verbatim to `<path>.agentguard.bak`
  before touching the live file (idempotent: if the backup already matches,
  it's a no-op; if it differs, a `.2` suffix avoids clobbering).
- Rewrites each stdio entry from
  `{command: "npx", args: ["-y", "srv"]}`
  to
  `{command: "/abs/path/to/agentguard", args: ["wrap", "--upstream-name", "<name>", "--", "npx", "-y", "srv"]}`.
- Leaves HTTP entries (URL set, command unset) untouched — those land in M4.
- `AlreadyWrapped` check makes `init` idempotent across re-runs.
- `Restore(path, hash)` reads the backup, atomic-renames it over the live
  file, optionally verifies the post-restore sha256, then removes the backup.
- Atomic writes: tempfile in the same dir → `os.Rename`.

### CLI surface (`internal/cli/`)

- **`init.go`** — 10-second install: makes ~/.agentguard/{bin,data,logs,
  packs,config} mode 0700, opens the store (migrations), detects every
  agent, applies patches, writes audit + agent rows. `--non-interactive`
  default for M3; `--dry-run` previews; `--skip-server` opts an entry out.
- **`uninstall.go`** — reads `agents` rows, calls `Restore` on each, marks
  the rows inactive. `--purge` removes `~/.agentguard`. Confirmation
  prompt unless `--yes`.
- **`doctor.go`** — homebrew-style ✓ / ⚠ / ✗ report: directory layout,
  PRAGMA integrity_check, agents counts, tool_calls counts, per-agent
  re-detection (catches "user edited their config and broke our routing"),
  pack count. Exits non-zero on any ✗.
- **`tail.go`** — Bubble Tea TUI polling SQLite every 500 ms. Verdict
  glyph colour-coded. `--once` for CI/scripts.
- **`scan.go`** — spawns an upstream MCP server, runs 8 cheap canned
  injection probes against its first echo-like tool, prints per-payload
  pass/fail and exits non-zero if any reflected. Designed for CI.
- **`paths.go`** — shared `~/.agentguard` resolver used by the three.

### Tests

```
$ go vet ./...                   → clean
$ go test ./...                  → all green
  ok  github.com/agentguard/agentguard/e2e                10.007s
  ok  github.com/agentguard/agentguard/internal/agent_detect  0.099s
  ok  github.com/agentguard/agentguard/internal/pipeline      cached
  ok  github.com/agentguard/agentguard/internal/policy        cached
  ok  github.com/agentguard/agentguard/internal/store         cached
```

**Unit:** 13 new tests in `internal/agent_detect/` covering each detector,
idempotent patching, HTTP skip, `--skip-server`, byte-identical backup +
restore, and TOML/Codex.

**e2e:** `TestInitAndUninstallRoundtrip` — real binary, fake `HOME`, init
→ uninstall, asserts the rewritten config invokes `agentguard wrap …` and
the post-uninstall file is **byte-identical** to the original (the §13
definition-of-done promise). `TestDoctorReportsHealthyAfterInit` —
post-init doctor returns 0 and reports the active agent + builtin packs.

### Live smoke

```
$ ./bin/agentguard --version
agentguard v0.3.0-m3 (<commit>, …)

$ ./bin/agentguard scan --timeout 10s -- ./bin/mock_mcp_server
AgentGuard scan — target: ./bin/mock_mcp_server
  ✗ ignore-previous        reflects 'ignore previous instructions' verbatim
  ✗ system-prompt-override reflects a fake new-system-prompt override
  ✗ base64-marker          echoes a base64-wrapped attack marker
  ✗ zero-width-marker      echoes a payload containing zero-width spaces
  ✗ you-are-now            reflects 'you are now ...' role override
  ✗ data-exfil-marker      echoes a fake AWS access key the server should not store
  ✗ policy-override        reflects 'disregard the safety policy' phrase
  ✓ html-script            reflects an HTML script tag (rendered-content vector)
Result: 1/8 passed. 7 reflected payload(s).
exit 1
```

(The mock server is a naïve echo — 7/8 reflections are the expected
"this is what a vulnerable server looks like" output, and the non-zero
exit code is what CI consumers want.)

## How to review

1. `internal/agent_detect/patcher.go` — the byte-identical backup +
   restore logic is the most important code in M3. `TestBackup...Roundtrip`
   locks the contract.
2. `internal/cli/init.go` — read `RunE` top to bottom; the order matters
   (paths → migrations → detect → patch → audit).
3. `internal/cli/doctor.go` — every check is self-contained; adding new
   ones is a 5-line patch.
4. Run `go test ./e2e/... -run Roundtrip -v` — the byte-identical test
   prints which paths it touched.

## Deliberately deferred (per spec)

- **Stage 2 server attestation** — binary hashing + tool-schema drift.
  Needs the registry package M4 introduces.
- **Bubble Tea init checklist** — current M3 init logs each step; the
  interactive picker arrives with the dashboard in M4.
- **`agentguard policy/pack/config/replay`** — file stubs, M4/M5.
- **`agentguard daemon`** — process supervisor + launchd/systemd/Win svc.
  Lands in M6 alongside the release pipeline.

## What's next (Milestone 4)

Dashboard MVP (overview + calls + servers pages), SSE event stream,
DuckDB-backed analytics queries, the OpenTelemetry exporter, and the real
interactive `init` checklist. Plus Stage 2 attestation now that the
registry can ship.

---

**Milestone 3 complete. Ready for review before Milestone 4.**
