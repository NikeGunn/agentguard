# Prompt for Claude Code — Build AgentGuard

> Paste this **entire file** into Claude Code in an empty directory. It will scaffold and start implementing.

---

You are going to build **AgentGuard**, an open-source, MIT-licensed, single-binary security sidecar for AI agents (Claude Code, Cursor, Codex, Gemini CLI, Windsurf). It proxies MCP and A2A traffic and blocks prompt injection, tool poisoning, rug-pulls, runaway loops, and credential exfiltration.

## How you should work

1. **Read all three spec files first, top to bottom, before writing any code:**
   - `system-design.md` — architecture and design principles
   - `database.md` — SQLite schema and the read/write paths
   - `requirements.md` — the binding build spec, repo layout, milestones, definition of done

2. **Work in milestones.** The plan in `requirements.md` Section 11 is six one-week milestones. Do **Milestone 1 only** in this session. Stop and ask for review before starting Milestone 2.

3. **Tech stack is locked.** Do not introduce dependencies outside `requirements.md` Section 2 without proposing the change first and getting explicit approval. If a locked dep is genuinely broken or abandoned, raise it, don't silently swap.

4. **Use the skills available to you.** When you create files, follow the skill conventions. For Go code, idiomatic Go with `gofmt`, `go vet`, and `golangci-lint` clean.

5. **One feature per commit, Conventional Commits style.** Branch off `main`; do not commit to `main` directly. Each commit message must answer: what changed, why, and how it was tested.

6. **Write tests as you go.** Coverage target in Section 7 of `requirements.md`. A function without a test does not count as done.

7. **No silent scope expansion.** If something in the spec is ambiguous, ask. If something is missing, propose an addition in a `OPEN_QUESTIONS.md` at the repo root and continue with the most defensible default.

## Milestone 1 deliverables (this session only)

By the end of this session, the repo should have:

- [ ] Full directory structure from `requirements.md` Section 1 created (empty files where the implementation is for later milestones, but the tree is complete)
- [ ] `go.mod` with Go 1.23+, all locked dependencies from Section 2 added
- [ ] `LICENSE` (MIT), `README.md` (placeholder but with hero line + install snippet), `CODE_OF_CONDUCT.md`, `CONTRIBUTING.md`, `SECURITY.md`, `CHANGELOG.md`, `.gitignore`
- [ ] `.github/workflows/ci.yml` — lint + test + cross-build matrix (darwin/linux/windows × amd64/arm64)
- [ ] `Makefile` with targets: `build`, `test`, `lint`, `bench`, `clean`, `install-tools`
- [ ] `internal/store/` — SQLite layer with WAL mode, all PRAGMAs from `database.md` Section 4, batched writer goroutine, initial goose migration creating every table from `database.md` Section 3
- [ ] `internal/proxy/stdio.go` — working stdio JSON-RPC proxy that spawns an upstream command, reads/writes JSON-RPC messages in both directions, and forwards them transparently (no inspection yet — just the pipe)
- [ ] `cmd/agentguard/main.go` + `internal/cli/root.go` + `internal/cli/wrap.go` + `internal/cli/version.go` — the `agentguard wrap` and `agentguard --version` commands fully functional via cobra
- [ ] `internal/version/version.go` — version string injected via `-ldflags` at build time
- [ ] A mock MCP server in `e2e/mock_mcp_server/` (tiny Go program that speaks JSON-RPC and answers a few canned tools)
- [ ] An e2e test in `e2e/wrap_test.sh` that:
  1. Builds the binary
  2. Builds the mock server
  3. Runs `agentguard wrap -- ./mock_mcp_server` and sends 3 tool calls via stdin
  4. Asserts the responses come back correctly and a `sessions` row + 3 `tool_calls` rows exist in the SQLite DB
- [ ] `bench/proxy_bench_test.go` — benchmarks the wrap roundtrip on a no-op call. Goal documented in the comments: p99 < 5ms once inspection is added (we're not there yet — record current baseline)
- [ ] One PR description as a draft in `PR_DRAFT.md` summarizing what landed, what's tested, what's next

## What "good" looks like at the end of this session

I should be able to:

```bash
git clone <this-repo>
cd agentguard
make install-tools
make build
make test
./bin/agentguard wrap -- ./bin/mock_mcp_server < some-test-input.jsonl
```

…and have it work cleanly, with the database at `~/.agentguard/data/agentguard.db` containing real rows after the run.

## What you should NOT do this session

- Do not implement the inspection pipeline stages beyond their interface definitions and `stage.go` skeleton.
- Do not build the dashboard yet (only the empty `web/` skeleton).
- Do not implement `agentguard init` agent detection — that's Milestone 3.
- Do not build the TUI yet.
- Do not implement the ML stage. Leave `internal/pipeline/ml.go` with a stubbed interface and a `// TODO milestone 5` comment.
- Do not write the marketing landing page yet.

## When you finish

1. Run `make test` and `make lint` and paste the final output.
2. Run `make bench` and paste the baseline numbers.
3. Update `CHANGELOG.md` with `## [Unreleased] — Milestone 1` and a bulleted list of what landed.
4. Write the `PR_DRAFT.md`.
5. Tell me explicitly: "Milestone 1 complete. Ready for review before Milestone 2."

## A note on quality

You are building the foundation. Every line of code from Milestone 1 will be touched a hundred times over the next six weeks. Make the abstractions clean. Name things well. Write tests that future-you will thank present-you for. If you cut a corner, leave a `// XXX:` comment and add an entry to `OPEN_QUESTIONS.md`.

Begin.
