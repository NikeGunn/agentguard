package store

import (
	"context"
	"database/sql"
)

// Analytics is the read-only query surface the dashboard uses for analytical
// rollups (database.md §6). It exists so the dashboard can be pointed at a
// faster column-store (DuckDB attaching the SQLite file read-only) without the
// rest of the binary knowing or caring which engine answered.
//
// Two implementations exist, selected at build time:
//
//   - default (this file): a pure-Go pass-through to the existing SQLite
//     analytics on *Store. No CGO, so cross-compilation stays trivial — the
//     locked tech stack mandates modernc.org/sqlite precisely to avoid CGO
//     (requirements.md §2, system-design.md §4.2). This is always available.
//
//   - `duckdb` build tag (duckdb_cgo.go): attaches the SQLite database
//     read-only through marcboeker/go-duckdb and answers the heavy GROUP BY
//     queries from DuckDB's vectorised engine. Opt-in because go-duckdb needs
//     CGO and a C toolchain.
//
// Both satisfy the same query methods, so callers (and tests) are identical.
// The split is documented in OPEN_QUESTIONS.md.
type Analytics struct {
	store  *Store
	engine string  // "sqlite" or "duckdb" — surfaced by Engine() for `doctor`.
	duck   *sql.DB // non-nil only on the `duckdb` build; the attached connection.
}

// EngineSQLite / EngineDuckDB name the active analytics backend.
const (
	EngineSQLite = "sqlite"
	EngineDuckDB = "duckdb"
)

// Engine reports which backend is answering analytics queries. `agentguard
// doctor` prints this so users know whether the DuckDB accelerator is active.
func (a *Analytics) Engine() string {
	if a == nil {
		return ""
	}
	return a.engine
}

// Available reports whether an accelerated (DuckDB) engine is in use. False on
// the default pure-Go build, where SQLite answers directly.
func (a *Analytics) Available() bool { return a != nil && a.engine == EngineDuckDB }

// TopTools proxies to the underlying engine.
func (a *Analytics) TopTools(ctx context.Context, windowSeconds int64, limit int) ([]ToolUsage, error) {
	return a.store.TopTools(ctx, secs(windowSeconds), limit)
}

// CallsByMinute proxies to the underlying engine.
func (a *Analytics) CallsByMinute(ctx context.Context, windowSeconds int64) ([]CallsPerMinute, error) {
	return a.store.CallsByMinute(ctx, secs(windowSeconds))
}

// Servers proxies to the underlying engine.
func (a *Analytics) Servers(ctx context.Context, limit int) ([]ServerSummary, error) {
	return a.store.Servers(ctx, limit)
}

// Close releases any engine-specific resources. No-op for the SQLite
// pass-through; on the DuckDB build it closes the attached connection.
func (a *Analytics) Close() error {
	if a != nil && a.duck != nil {
		return a.duck.Close()
	}
	return nil
}
