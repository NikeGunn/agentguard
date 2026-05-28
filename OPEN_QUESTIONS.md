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
