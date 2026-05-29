# Open Questions

Track ambiguities in the spec here as they come up. Each entry should name the
section, restate the question, and record the default chosen so future work
can revisit it.

## M1 entries

- **JSON-RPC framing for stdio MCP.** Spec is silent on whether stdio MCP uses
  Content-Length headers (LSP-style) or newline-delimited JSON. The
  `modelcontextprotocol` reference servers ship newline-delimited JSON over
  stdio. **Default chosen:** newline-delimited JSON, one object per line. We
  forward bytes verbatim, so a framing change is contained to the parser.

- **SQLite path on Windows.** `~/.agentguard/data/agentguard.db` is the spec.
  On Windows we resolve `~` via `os.UserHomeDir()` which returns
  `C:\Users\<user>`. Documented.

- **Stage 5 (ML) skeleton.** Spec says "stub with `// TODO milestone 5`"; we
  leave a `Stage` interface implementation that always returns `pass`.

## Completion-run entries (2026-05-29)

- **DuckDB vs pure-Go SQLite (database.md §6, requirements.md §2).** The DB
  design calls for DuckDB to attach the SQLite file read-only for analytics.
  But the locked stack also mandates `modernc.org/sqlite` *specifically because
  it is pure-Go and avoids CGO*, and `marcboeker/go-duckdb` requires CGO + a C
  toolchain (it would also break cross-compilation and isn't currently a
  dependency). These two directives conflict. **Default chosen:** ship a
  pure-Go `store.Analytics` that answers analytics directly from SQLite (always
  available, CGO-free release binary), and provide the DuckDB attach behind a
  `-tags duckdb` opt-in build (`duckdb_cgo.go`) for users who add the dep and
  accept CGO. `Analytics.Engine()` reports which backend is live. This honours
  the spec's *intent* (DuckDB-accelerated analytics when available) without
  violating the pure-Go default. Revisit if go-duckdb ships a pure-Go driver.

- **SvelteKit dashboard vs embedded vanilla build (requirements.md §1, §3.10).**
  Section 1 lists a SvelteKit app under `web/`. The shipping dashboard is the
  hand-written HTML/JS/CSS embedded via `embed.FS` under
  `internal/dashboard/assets/` (live SSE, dark mode, overview/calls/servers).
  It satisfies the *behavioural* spec (§3.10) and the <200 KB bundle budget
  while keeping everything in the single Go binary with zero third-party
  assets. **Default chosen:** keep the embedded build as the shipped dashboard;
  the `web/` SvelteKit tree remains a scaffold for a future migration that can
  replace the embedded assets without changing the API/SSE surface.

- **Daemon service installers (requirements.md §12).** §12 says "no Windows
  service installer" and "nothing that requires root". §10 still describes a
  daemon supervisor with launchd/systemd/windows units. **Default chosen:** the
  `daemon` package *generates* the unit/plist/service file bodies and reports
  where they'd be installed, but does not perform a privileged install. Users
  copy the generated unit into place themselves. This respects §12 while
  implementing the §10 supervisor.
