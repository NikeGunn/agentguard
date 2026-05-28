-- +goose Up
-- +goose StatementBegin

CREATE TABLE agents (
    id              TEXT PRIMARY KEY,
    kind            TEXT NOT NULL,
    display_name    TEXT NOT NULL,
    config_path     TEXT NOT NULL,
    config_backup   TEXT NOT NULL,
    detected_at     INTEGER NOT NULL,
    last_seen_at    INTEGER,
    active          INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX idx_agents_active ON agents(active);

CREATE TABLE mcp_servers (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    canonical_uri   TEXT NOT NULL,
    transport       TEXT NOT NULL,
    upstream_command TEXT,
    upstream_url    TEXT,
    auth_required   INTEGER NOT NULL DEFAULT 0,
    first_seen_at   INTEGER NOT NULL,
    last_seen_at    INTEGER NOT NULL,
    trust_score     INTEGER,
    trust_score_updated_at INTEGER,
    UNIQUE(canonical_uri)
);
CREATE INDEX idx_mcp_servers_name ON mcp_servers(name);
CREATE INDEX idx_mcp_servers_trust ON mcp_servers(trust_score);

CREATE TABLE server_tools (
    id              TEXT PRIMARY KEY,
    server_id       TEXT NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    tool_name       TEXT NOT NULL,
    description     TEXT NOT NULL,
    description_hash TEXT NOT NULL,
    input_schema    TEXT NOT NULL,
    input_schema_hash TEXT NOT NULL,
    classification  TEXT NOT NULL DEFAULT 'unknown',
    first_seen_at   INTEGER NOT NULL,
    last_seen_at    INTEGER NOT NULL,
    approved_at     INTEGER,
    UNIQUE(server_id, tool_name)
);
CREATE INDEX idx_server_tools_server ON server_tools(server_id);

CREATE TABLE server_attestations (
    id              TEXT PRIMARY KEY,
    server_id       TEXT NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    attested_at     INTEGER NOT NULL,
    binary_sha256   TEXT,
    cosign_verified INTEGER NOT NULL DEFAULT 0,
    package_version TEXT,
    last_commit_at  INTEGER,
    open_cves_count INTEGER NOT NULL DEFAULT 0,
    raw_attestation TEXT
);
CREATE INDEX idx_server_attestations_server_time ON server_attestations(server_id, attested_at DESC);

CREATE TABLE sessions (
    id              TEXT PRIMARY KEY,
    agent_id        TEXT REFERENCES agents(id),
    started_at      INTEGER NOT NULL,
    ended_at        INTEGER,
    client_pid      INTEGER,
    client_user     TEXT,
    mode            TEXT NOT NULL DEFAULT 'enforce',
    total_calls     INTEGER NOT NULL DEFAULT 0,
    total_blocked   INTEGER NOT NULL DEFAULT 0,
    total_cost_usd  REAL NOT NULL DEFAULT 0,
    total_tokens    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_sessions_agent_time ON sessions(agent_id, started_at DESC);
CREATE INDEX idx_sessions_open ON sessions(ended_at) WHERE ended_at IS NULL;

CREATE TABLE tool_calls (
    id              TEXT PRIMARY KEY,
    session_id      TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    server_id       TEXT NOT NULL REFERENCES mcp_servers(id),
    tool_name       TEXT NOT NULL,
    direction       TEXT NOT NULL,
    started_at      INTEGER NOT NULL,
    completed_at    INTEGER,
    verdict         TEXT NOT NULL DEFAULT 'pending',
    verdict_reason  TEXT,
    risk_score      REAL,
    request_size_bytes  INTEGER,
    response_size_bytes INTEGER,
    latency_ms_proxy    INTEGER,
    latency_ms_upstream INTEGER,
    cost_usd        REAL NOT NULL DEFAULT 0,
    token_count     INTEGER NOT NULL DEFAULT 0,
    request_inline  TEXT,
    response_inline TEXT,
    error_inline    TEXT
);
CREATE INDEX idx_tool_calls_session ON tool_calls(session_id, started_at);
CREATE INDEX idx_tool_calls_time ON tool_calls(started_at DESC);
CREATE INDEX idx_tool_calls_verdict ON tool_calls(verdict, started_at DESC);
CREATE INDEX idx_tool_calls_server_tool ON tool_calls(server_id, tool_name, started_at DESC);

CREATE TABLE tool_call_stages (
    id              TEXT PRIMARY KEY,
    tool_call_id    TEXT NOT NULL REFERENCES tool_calls(id) ON DELETE CASCADE,
    stage           TEXT NOT NULL,
    stage_order     INTEGER NOT NULL,
    started_at_ns   INTEGER NOT NULL,
    duration_ns     INTEGER NOT NULL,
    outcome         TEXT NOT NULL,
    detail          TEXT
);
CREATE INDEX idx_stages_call ON tool_call_stages(tool_call_id, stage_order);

CREATE TABLE tool_call_artifacts (
    id              TEXT PRIMARY KEY,
    tool_call_id    TEXT NOT NULL REFERENCES tool_calls(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL,
    content_type    TEXT NOT NULL DEFAULT 'application/json',
    encoding        TEXT NOT NULL DEFAULT 'utf8',
    size_bytes      INTEGER NOT NULL,
    body            BLOB NOT NULL
);
CREATE INDEX idx_artifacts_call ON tool_call_artifacts(tool_call_id);

CREATE TABLE policies (
    id              TEXT PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT,
    source          TEXT NOT NULL,
    pack_id         TEXT,
    scope           TEXT NOT NULL DEFAULT 'global',
    scope_path      TEXT,
    enabled         INTEGER NOT NULL DEFAULT 1,
    priority        INTEGER NOT NULL DEFAULT 100,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);
CREATE INDEX idx_policies_enabled ON policies(enabled, priority);

CREATE TABLE policy_rules (
    id              TEXT PRIMARY KEY,
    policy_id       TEXT NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    rule_order      INTEGER NOT NULL,
    match           TEXT NOT NULL,
    action          TEXT NOT NULL,
    parameters      TEXT,
    enabled         INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX idx_policy_rules_policy ON policy_rules(policy_id, rule_order);

CREATE TABLE rule_packs (
    id              TEXT PRIMARY KEY,
    slug            TEXT NOT NULL UNIQUE,
    version         TEXT NOT NULL,
    publisher       TEXT NOT NULL,
    publisher_verified INTEGER NOT NULL DEFAULT 0,
    description     TEXT,
    source_url      TEXT NOT NULL,
    installed_at    INTEGER NOT NULL,
    pack_sha256     TEXT NOT NULL,
    raw_yaml        TEXT NOT NULL
);

CREATE TABLE audit_log (
    id              TEXT PRIMARY KEY,
    occurred_at     INTEGER NOT NULL,
    actor           TEXT NOT NULL,
    action          TEXT NOT NULL,
    target_type     TEXT,
    target_id       TEXT,
    detail          TEXT
);
CREATE INDEX idx_audit_log_time ON audit_log(occurred_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS rule_packs;
DROP TABLE IF EXISTS policy_rules;
DROP TABLE IF EXISTS policies;
DROP TABLE IF EXISTS tool_call_artifacts;
DROP TABLE IF EXISTS tool_call_stages;
DROP TABLE IF EXISTS tool_calls;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS server_attestations;
DROP TABLE IF EXISTS server_tools;
DROP TABLE IF EXISTS mcp_servers;
DROP TABLE IF EXISTS agents;

-- +goose StatementEnd
