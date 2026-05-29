//go:build !duckdb

package store

import "time"

// NewAnalytics returns the pure-Go analytics backend, which answers every
// query directly from SQLite. This is the default build: no CGO, builds and
// cross-compiles everywhere. Build with `-tags duckdb` to use the DuckDB
// accelerator instead (see duckdb_cgo.go).
func NewAnalytics(s *Store) (*Analytics, error) {
	return &Analytics{store: s, engine: EngineSQLite}, nil
}

// secs converts a window expressed in seconds into a Duration. A zero or
// negative window means "all of time", which the analytics queries treat as
// no lower bound.
func secs(windowSeconds int64) time.Duration {
	if windowSeconds <= 0 {
		return 0
	}
	return time.Duration(windowSeconds) * time.Second
}
