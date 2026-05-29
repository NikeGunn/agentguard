# AgentGuard — Database Design

> **Stack:** SQLite (modernc.org/sqlite, pure-Go) for OLTP + DuckDB (in-process) for analytical queries.
> **Location:** `~/.agentguard/data/agentguard.db` (mode 0600).
> **Migration tool:** [goose](https://github.com/pressly/goose) — versioned SQL migrations checked into the repo.

---

## 1. Why SQLite + DuckDB

| Concern | Choice | Reason |
|---------|--------|--------|
| Embedded, single-binary | SQLite | No separate server process. Zero ops for the user. |
| Cross-compile | modernc.org/sqlite | Pure Go — no CGO, no platform builds gone wrong. |
| Concurrent reads while writing | WAL mode | Dashboard reads never block the proxy's writes. |
| Analytical rollups (GROUP BY over millions of rows) | DuckDB | SQLite is row-oriented and slow at this; DuckDB reads SQLite files natively in 2026. |
| Encryption at rest (Pro tier) | SQLCipher fork | Optional. Off by default for performance. |

DuckDB is used **read-only** against the SQLite file. We do not write through DuckDB. This avoids consistency headaches.

---

## 2. Schema Overview

10 tables. Every event, policy, and configuration the sidecar needs.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            CORE OPERATIONAL                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│  sessions ──────► tool_calls ──────► tool_call_stages                        │
│      │                │                                                       │
│      │                └──────► tool_call_artifacts                            │
│      ▼                                                                        │
│  agents                                                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                            CONFIGURATION                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│  mcp_servers ──────► server_attestations                                      │
│      │                                                                        │
│      └──────► server_tools                                                    │
│                                                                               │
│  policies ──────► policy_rules                                                │
│                                                                               │
│  rule_packs                                                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                            META                                              │
├─────────────────────────────────────────────────────────────────────────────┤
│  schema_migrations                                                            │
│  audit_log                                                                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Table Definitions

### 3.1 `agents`
The agents the user has connected (Claude Code, Cursor, etc.). Auto-discovered on `init`.

```sql
CREATE TABLE agents (
    id              TEXT PRIMARY KEY,            -- ULID
    kind            TEXT NOT NULL,                -- 'claude-code' | 'cursor' | 'codex' | 'gemini-cli' | 'windsurf' | 'custom'
    display_name    TEXT NOT NULL,
    config_path     TEXT NOT NULL,                -- path to the agent's MCP config we patched
    config_backup   TEXT NOT NULL,                -- path to our pre-patch backup
    detected_at     INTEGER NOT NULL,             -- unix epoch ms
    last_seen_at    INTEGER,
    active          INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX idx_agents_active ON agents(active);
```

### 3.2 `mcp_servers`
Every MCP server the sidecar has ever seen, by canonical identity.

```sql
CREATE TABLE mcp_servers (
    id              TEXT PRIMARY KEY,            -- ULID
    name            TEXT NOT NULL,                -- 'github', 'filesystem', etc.
    canonical_uri   TEXT NOT NULL,                -- 'npm:@modelcontextprotocol/server-github@2.4.1'
                                                  -- or 'https://mcp.notion.com/sse'
    transport       TEXT NOT NULL,                -- 'stdio' | 'streamable-http' | 'sse'
    upstream_command TEXT,                        -- the actual command we shell out to (stdio)
    upstream_url    TEXT,                         -- the actual URL we proxy to (http)
    auth_required   INTEGER NOT NULL DEFAULT 0,
    first_seen_at   INTEGER NOT NULL,
    last_seen_at    INTEGER NOT NULL,
    trust_score     INTEGER,                      -- 0-100, NULL if not yet computed
    trust_score_updated_at INTEGER,
    UNIQUE(canonical_uri)
);

CREATE INDEX idx_mcp_servers_name ON mcp_servers(name);
CREATE INDEX idx_mcp_servers_trust ON mcp_servers(trust_score);
```

### 3.3 `server_tools`
The tools each server exposes, with their latest known schema. Used for rug-pull detection.

```sql
CREATE TABLE server_tools (
    id              TEXT PRIMARY KEY,            -- ULID
    server_id       TEXT NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    tool_name       TEXT NOT NULL,
    description     TEXT NOT NULL,                -- human-readable description from the server
    description_hash TEXT NOT NULL,               -- SHA-256 of description (rug-pull detection)
    input_schema    TEXT NOT NULL,                -- JSON Schema, as text
    input_schema_hash TEXT NOT NULL,
    classification  TEXT NOT NULL DEFAULT 'unknown', -- 'read' | 'write' | 'execute' | 'network' | 'unknown'
    first_seen_at   INTEGER NOT NULL,
    last_seen_at    INTEGER NOT NULL,
    approved_at     INTEGER,                      -- when the user explicitly approved this version
    UNIQUE(server_id, tool_name)
);

CREATE INDEX idx_server_tools_server ON server_tools(server_id);
```

### 3.4 `server_attestations`
History of cryptographic & metadata attestations performed on a server. One row per attestation event.

```sql
CREATE TABLE server_attestations (
    id              TEXT PRIMARY KEY,            -- ULID
    server_id       TEXT NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    attested_at     INTEGER NOT NULL,
    binary_sha256   TEXT,                         -- stdio servers: hash of the resolved binary
    cosign_verified INTEGER NOT NULL DEFAULT 0,
    package_version TEXT,                         -- 'npm:@modelcontextprotocol/server-github@2.4.1'
    last_commit_at  INTEGER,                      -- from GitHub API for the source repo (abandonment signal)
    open_cves_count INTEGER NOT NULL DEFAULT 0,
    raw_attestation TEXT                          -- JSON blob for forensic inspection
);

CREATE INDEX idx_server_attestations_server_time ON server_attestations(server_id, attested_at DESC);
```

### 3.5 `sessions`
One agent invocation. A "session" is the lifetime of one MCP/A2A connection.

```sql
CREATE TABLE sessions (
    id              TEXT PRIMARY KEY,            -- ULID
    agent_id        TEXT REFERENCES agents(id),
    started_at      INTEGER NOT NULL,
    ended_at        INTEGER,
    client_pid      INTEGER,                      -- PID of the agent process (when known)
    client_user     TEXT,                         -- OS user
    mode            TEXT NOT NULL DEFAULT 'enforce', -- 'enforce' | 'monitor' | 'bypass'
    total_calls     INTEGER NOT NULL DEFAULT 0,
    total_blocked   INTEGER NOT NULL DEFAULT 0,
    total_cost_usd  REAL NOT NULL DEFAULT 0,
    total_tokens    INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_sessions_agent_time ON sessions(agent_id, started_at DESC);
CREATE INDEX idx_sessions_open ON sessions(ended_at) WHERE ended_at IS NULL;
```

### 3.6 `tool_calls`
**The single most important table.** Every MCP/A2A request the sidecar has ever proxied.

```sql
CREATE TABLE tool_calls (
    id              TEXT PRIMARY KEY,            -- ULID, sortable by time
    session_id      TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    server_id       TEXT NOT NULL REFERENCES mcp_servers(id),
    tool_name       TEXT NOT NULL,
    direction       TEXT NOT NULL,                -- 'outbound' (agent->tool) | 'inbound' (tool->agent)
    started_at      INTEGER NOT NULL,             -- unix epoch ms
    completed_at    INTEGER,
    verdict         TEXT NOT NULL DEFAULT 'pending', -- 'allow' | 'block' | 'transform' | 'flag' | 'pending'
    verdict_reason  TEXT,                         -- structured reason code
    risk_score      REAL,                         -- 0.0 to 1.0
    request_size_bytes  INTEGER,
    response_size_bytes INTEGER,
    latency_ms_proxy    INTEGER,                  -- AgentGuard's own overhead
    latency_ms_upstream INTEGER,                  -- upstream server's time
    cost_usd        REAL NOT NULL DEFAULT 0,
    token_count     INTEGER NOT NULL DEFAULT 0,
    -- payloads stored separately if large (see tool_call_artifacts)
    request_inline  TEXT,                         -- request body, NULL if > inline threshold
    response_inline TEXT,                         -- response body, NULL if > inline threshold
    error_inline    TEXT
);

CREATE INDEX idx_tool_calls_session ON tool_calls(session_id, started_at);
CREATE INDEX idx_tool_calls_time ON tool_calls(started_at DESC);
CREATE INDEX idx_tool_calls_verdict ON tool_calls(verdict, started_at DESC);
CREATE INDEX idx_tool_calls_server_tool ON tool_calls(server_id, tool_name, started_at DESC);
```

> Retention: rolling 30-day default, configurable. Older rows are vacuumed by a nightly job. Pro tier streams older rows to encrypted cloud archive before deletion.

### 3.7 `tool_call_stages`
The full inspection pipeline trail for one tool call. Each stage that ran on the call produces a row.

```sql
CREATE TABLE tool_call_stages (
    id              TEXT PRIMARY KEY,            -- ULID
    tool_call_id    TEXT NOT NULL REFERENCES tool_calls(id) ON DELETE CASCADE,
    stage           TEXT NOT NULL,                -- 'transport' | 'schema' | 'attestation' | 'policy' | 'content_scan' | 'ml_classify' | 'circuit_breaker'
    stage_order     INTEGER NOT NULL,
    started_at_ns   INTEGER NOT NULL,             -- nanos, monotonic
    duration_ns     INTEGER NOT NULL,
    outcome         TEXT NOT NULL,                -- 'pass' | 'block' | 'flag' | 'transform' | 'skip' | 'error'
    detail          TEXT                          -- JSON: what fired, score, matched rule, etc.
);

CREATE INDEX idx_stages_call ON tool_call_stages(tool_call_id, stage_order);
```

### 3.8 `tool_call_artifacts`
Large request/response bodies stored separately to keep the hot table fast.

```sql
CREATE TABLE tool_call_artifacts (
    id              TEXT PRIMARY KEY,            -- ULID
    tool_call_id    TEXT NOT NULL REFERENCES tool_calls(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL,                -- 'request' | 'response' | 'error' | 'redacted_diff'
    content_type    TEXT NOT NULL DEFAULT 'application/json',
    encoding        TEXT NOT NULL DEFAULT 'utf8', -- 'utf8' | 'base64' | 'zstd'
    size_bytes      INTEGER NOT NULL,
    body            BLOB NOT NULL
);

CREATE INDEX idx_artifacts_call ON tool_call_artifacts(tool_call_id);
```

Inline-vs-artifact decision: if a payload is ≤ 4 KB, it goes inline on `tool_calls`. Otherwise it's zstd-compressed and stored here.

### 3.9 `policies`
Top-level policy groupings. A user can have multiple active.

```sql
CREATE TABLE policies (
    id              TEXT PRIMARY KEY,            -- ULID
    name            TEXT NOT NULL,
    description     TEXT,
    source          TEXT NOT NULL,                -- 'builtin' | 'user' | 'pack' | 'cloud'
    pack_id         TEXT,                         -- nullable, references rule_packs
    scope           TEXT NOT NULL DEFAULT 'global', -- 'global' | 'project' | 'session'
    scope_path      TEXT,                         -- path of the project if scope='project'
    enabled         INTEGER NOT NULL DEFAULT 1,
    priority        INTEGER NOT NULL DEFAULT 100, -- lower = higher priority
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);

CREATE INDEX idx_policies_enabled ON policies(enabled, priority);
```

### 3.10 `policy_rules`
The individual rules inside a policy.

```sql
CREATE TABLE policy_rules (
    id              TEXT PRIMARY KEY,            -- ULID
    policy_id       TEXT NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    rule_order      INTEGER NOT NULL,
    match           TEXT NOT NULL,                -- JSON: {server?: '...', tool?: '...', direction?: '...', content_match?: '...'}
    action          TEXT NOT NULL,                -- 'allow' | 'block' | 'require_approval' | 'flag' | 'redact' | 'rate_limit'
    parameters      TEXT,                         -- JSON: action-specific config (e.g. {max_cost_usd: 2})
    enabled         INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX idx_policy_rules_policy ON policy_rules(policy_id, rule_order);
```

### 3.11 `rule_packs`
Installed community rule packs.

```sql
CREATE TABLE rule_packs (
    id              TEXT PRIMARY KEY,            -- ULID
    slug            TEXT NOT NULL UNIQUE,         -- 'fintech', 'opensource-maintainer'
    version         TEXT NOT NULL,
    publisher       TEXT NOT NULL,
    publisher_verified INTEGER NOT NULL DEFAULT 0,
    description     TEXT,
    source_url      TEXT NOT NULL,
    installed_at    INTEGER NOT NULL,
    pack_sha256     TEXT NOT NULL,                -- of the source YAML, for integrity
    raw_yaml        TEXT NOT NULL                 -- the original pack content
);
```

### 3.12 `audit_log`
Append-only record of policy/config changes for forensics.

```sql
CREATE TABLE audit_log (
    id              TEXT PRIMARY KEY,            -- ULID
    occurred_at     INTEGER NOT NULL,
    actor           TEXT NOT NULL,                -- 'cli' | 'dashboard' | 'cloud-sync' | 'system'
    action          TEXT NOT NULL,                -- 'policy.create' | 'policy.update' | 'server.approve' | etc.
    target_type     TEXT,                         -- 'policy' | 'server' | 'agent'
    target_id       TEXT,
    detail          TEXT                          -- JSON
);

CREATE INDEX idx_audit_log_time ON audit_log(occurred_at DESC);
```

### 3.13 `schema_migrations`
Managed by `goose`.

```sql
CREATE TABLE goose_db_version (
    id          INTEGER PRIMARY KEY,
    version_id  INTEGER NOT NULL,
    is_applied  INTEGER NOT NULL,
    tstamp      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

---

## 4. Pragmas & Configuration

Set at every connection open:

```sql
PRAGMA journal_mode = WAL;          -- concurrent reads while writing
PRAGMA synchronous = NORMAL;         -- fsync less often, still safe with WAL
PRAGMA temp_store = MEMORY;
PRAGMA mmap_size = 268435456;        -- 256 MB mmap window for cold reads
PRAGMA cache_size = -20000;          -- 20 MB page cache per connection
PRAGMA foreign_keys = ON;
PRAGMA busy_timeout = 5000;          -- 5s wait before raising SQLITE_BUSY
```

---

## 5. Write Path (Hot)

The proxy writes one row per tool call, plus N rows per stage. Naive single-row inserts at 1000 req/s overwhelm SQLite even with WAL.

**Solution: batched writer goroutine.**

```
proxy goroutine ──► events channel ──► writer goroutine ──► single transaction per batch
                                              │
                                              └── batch closes on:
                                                  - 100 events queued, OR
                                                  - 10 ms elapsed since last flush
```

Single-statement INSERTs are replaced with multi-row VALUES tuples (SQLite handles thousands per statement). Each batch is one transaction. End-to-end this delivers >20k inserts/sec on modest hardware.

---

## 6. Read Path (Dashboard)

The dashboard does two kinds of queries:

1. **Live tail**: latest 200 `tool_calls` joined to `mcp_servers` and `sessions`. Indexed; sub-ms.
2. **Analytics**: "top 10 most-called tools in the last 7 days", "spend by server by day", "block rate by tool". These are slow in row-oriented SQLite.

**Solution for #2: DuckDB attaches the SQLite file read-only.**

```sql
-- inside the dashboard's analytics handler (Go calls DuckDB)
ATTACH '~/.agentguard/data/agentguard.db' AS aguard (TYPE SQLITE, READ_ONLY);

SELECT
    s.name AS server,
    tc.tool_name,
    COUNT(*) AS call_count,
    SUM(tc.cost_usd) AS total_cost,
    AVG(tc.latency_ms_proxy) AS avg_proxy_ms,
    SUM(CASE WHEN tc.verdict = 'block' THEN 1 ELSE 0 END) AS blocks
FROM aguard.tool_calls tc
JOIN aguard.mcp_servers s ON s.id = tc.server_id
WHERE tc.started_at > strftime('%s', 'now', '-7 days') * 1000
GROUP BY 1, 2
ORDER BY call_count DESC
LIMIT 10;
```

DuckDB reads the SQLite pages directly. The proxy doesn't see DuckDB at all.

---

## 7. Retention & Vacuum

- Default retention: 30 days for `tool_calls`, `tool_call_stages`, `tool_call_artifacts`.
- Configurable via `agentguard config set retention.days 90`.
- Nightly job at 03:00 local: `DELETE FROM tool_calls WHERE started_at < ?` then `PRAGMA wal_checkpoint(TRUNCATE)` and `VACUUM` once a week.
- `audit_log` retained forever (small, append-only).
- `mcp_servers`, `policies`, `agents` never auto-deleted.

---

## 8. Encryption (Pro Tier)

Off by default; performance-sensitive. When enabled:

- Database file encrypted with SQLCipher (256-bit AES-CBC + HMAC-SHA512).
- Key derived from OS keyring (macOS Keychain, Linux Secret Service, Windows DPAPI). User never types a passphrase unless `--passphrase` is explicitly set.
- The `tool_call_artifacts.body` BLOB is additionally encrypted with a per-row XChaCha20-Poly1305 envelope when in cloud-sync mode (zero-knowledge: cloud only sees ciphertext).

---

## 9. Cloud Sync Schema (Pro)

The Cloudflare D1 database mirrors a subset, with org/team scoping. Tables prefixed `c_`:

- `c_orgs`, `c_users`, `c_memberships`, `c_api_keys`
- `c_audit_log_batches` (encrypted blobs in R2 indexed by manifest in D1)
- `c_policies` (the source of truth when a team uses centralized policy)
- `c_rule_packs` (published packs)

Local sidecar pulls `c_policies` every 5 minutes when authenticated. Pushes encrypted audit batches every 1 minute. All transports HTTPS + mTLS via Cloudflare Access optional.

---

## 10. Indexes Summary (the only ones that matter)

| Query | Index |
|-------|-------|
| Live tail (latest N calls) | `idx_tool_calls_time` |
| Calls for a session | `idx_tool_calls_session` |
| Blocks by time | `idx_tool_calls_verdict` |
| Tool-specific history | `idx_tool_calls_server_tool` |
| Stage detail for a call | `idx_stages_call` |
| Open sessions | `idx_sessions_open` (partial) |
| Trust score lookups | `idx_mcp_servers_trust` |

All other tables are small enough to be covered by the implicit primary-key index.

---

## 11. Sample Migration File Layout

```
migrations/
├── 0001_initial.sql              -- everything above, fresh install
├── 0002_add_token_count.sql      -- example: adding token_count column
├── 0003_rule_pack_signing.sql
└── ...
```

Each migration has an `-- +goose Up` and `-- +goose Down` section. CI verifies every Up applies cleanly and every Down rolls back without data loss on a synthetic dataset.

---

## 12. Backup & Recovery

- `agentguard backup` produces a single `.tar.zst` of `~/.agentguard/data/` and the policy YAML.
- `agentguard restore <file>` validates and replaces. The current DB is moved to `.bak.<timestamp>` first.
- Cloud Pro: daily snapshot to R2, 30-day point-in-time restore.

---

## 13. Things explicitly NOT in the schema

- Per-row encryption keys for the cloud (managed elsewhere, in the user's OS keyring)
- LLM tokens / API keys (never written to DB; if seen in payloads, redacted before write)
- Raw prompts (those belong to the agent, not to AgentGuard — we see tool calls only)

This boundary is critical. A user installing AgentGuard must be able to truthfully say: "AgentGuard never sees my LLM prompts. It only sees what my agent tries to do with tools."
