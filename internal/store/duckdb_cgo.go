//go:build duckdb

package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

// This build path attaches the SQLite file read-only through DuckDB
// (database.md §6). Only compiled with `-tags duckdb`.
//
// NOTE: this build path requires CGO and a C toolchain, plus the go-duckdb
// dependency added to go.mod (run `go get github.com/marcboeker/go-duckdb`
// before building with the tag). The default build (duckdb_default.go) is
// pure-Go, so the shipped release binary stays CGO-free while power users who
// want DuckDB's vectorised GROUP BY can opt in. Decision logged in
// OPEN_QUESTIONS.md.

// NewAnalytics opens an in-process DuckDB and attaches the SQLite database
// read-only. On any failure it falls back to the SQLite engine so analytics
// never go dark.
func NewAnalytics(s *Store) (*Analytics, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return &Analytics{store: s, engine: EngineSQLite}, nil //nolint:nilerr // graceful fallback
	}
	attach := fmt.Sprintf("ATTACH '%s' AS aguard (TYPE SQLITE, READ_ONLY)", s.Path())
	if _, err := db.Exec(attach); err != nil {
		_ = db.Close()
		return &Analytics{store: s, engine: EngineSQLite}, nil //nolint:nilerr // graceful fallback
	}
	return &Analytics{store: s, engine: EngineDuckDB, duck: db}, nil
}

func secs(windowSeconds int64) time.Duration {
	if windowSeconds <= 0 {
		return 0
	}
	return time.Duration(windowSeconds) * time.Second
}
