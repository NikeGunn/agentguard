// Package store owns the SQLite layer: schema migrations, the batched writer
// goroutine that funnels proxy events into the DB, and the typed record
// definitions the rest of the binary uses.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/pressly/goose/v3"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// DriverName is the database/sql driver we use everywhere.
const DriverName = "sqlite"

// Pragmas are applied on every connection open. See database.md §4.
var Pragmas = []string{
	"PRAGMA journal_mode = WAL",
	"PRAGMA synchronous = NORMAL",
	"PRAGMA temp_store = MEMORY",
	"PRAGMA mmap_size = 268435456",
	"PRAGMA cache_size = -20000",
	"PRAGMA foreign_keys = ON",
	"PRAGMA busy_timeout = 5000",
}

// Store wraps the sql.DB plus the batched writer.
type Store struct {
	DB          *sql.DB
	path        string
	writer      *batchedWriter
	broadcaster *Broadcaster
	closeOnce   sync.Once
	closeErr    error
}

// DefaultPath returns the canonical on-disk location:
// ~/.agentguard/data/agentguard.db.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agentguard", "data", "agentguard.db"), nil
}

// Open creates (if needed) the directory, opens SQLite with the standard
// pragmas, runs all migrations, and starts the batched writer goroutine.
func Open(ctx context.Context, dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("mkdir db dir: %w", err)
	}
	dsn := "file:" + dbPath + "?_pragma=busy_timeout(5000)"
	db, err := sql.Open(DriverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	// Single connection for writes serialises access without juggling multiple
	// WAL writers. Reads on a different *sql.DB can still happen concurrently.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	for _, p := range Pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	if err := applyMigrations(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	if err := os.Chmod(dbPath, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Best effort — Windows may report ENOSYS for chmod, that's fine.
	}

	s := &Store{DB: db, path: dbPath, broadcaster: NewBroadcaster()}
	s.writer = newBatchedWriter(s)
	s.writer.start()
	return s, nil
}

// Broadcaster returns the per-store SSE broadcaster. The batched writer
// publishes every successfully-flushed ToolCall here; the dashboard
// subscribes per HTTP connection.
func (s *Store) Broadcaster() *Broadcaster { return s.broadcaster }

func applyMigrations(db *sql.DB) error {
	goose.SetBaseFS(embeddedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	goose.SetLogger(goose.NopLogger())
	return goose.Up(db, "migrations")
}

// Path returns the database file path the store was opened against.
func (s *Store) Path() string { return s.path }

// Close flushes the batched writer and closes the DB. Safe to call multiple
// times.
func (s *Store) Close() error {
	s.closeOnce.Do(func() {
		if s.writer != nil {
			s.writer.stop()
		}
		s.closeErr = s.DB.Close()
	})
	return s.closeErr
}

// Writer returns the batched writer so callers can submit events.
func (s *Store) Writer() *batchedWriter { return s.writer }

// NewID returns a fresh ULID string.
func NewID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// NowMS returns the current unix epoch in milliseconds, matching the column
// convention used across the schema.
func NowMS() int64 { return time.Now().UnixMilli() }
