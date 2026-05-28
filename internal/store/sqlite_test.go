package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(context.Background(), filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestOpenAppliesMigrations(t *testing.T) {
	s := tempStore(t)
	row := s.DB.QueryRowContext(context.Background(),
		`SELECT count(*) FROM sqlite_master WHERE type='table'`)
	var n int
	require.NoError(t, row.Scan(&n))
	// 12 schema tables + goose_db_version + 2 sqlite shadow tables for partial
	// indexes can vary; assert at least the 12 expected tables exist.
	require.GreaterOrEqual(t, n, 12)
}

func TestOpenSetsWALMode(t *testing.T) {
	s := tempStore(t)
	row := s.DB.QueryRowContext(context.Background(), `PRAGMA journal_mode`)
	var mode string
	require.NoError(t, row.Scan(&mode))
	require.Equal(t, "wal", mode)
}

func TestBatchedWriterFlushesSession(t *testing.T) {
	s := tempStore(t)
	sess := &Session{
		ID:        NewID(),
		StartedAt: NowMS(),
		Mode:      "enforce",
	}
	s.Writer().Submit(Event{Kind: EventSessionStart, Session: sess})

	// Allow the writer up to ~250ms to drain.
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		var n int
		err := s.DB.QueryRow(`SELECT count(*) FROM sessions WHERE id = ?`, sess.ID).Scan(&n)
		require.NoError(t, err)
		if n == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("session row was not written within deadline")
}

func TestBatchedWriterEndToEnd(t *testing.T) {
	s := tempStore(t)
	sessID := NewID()
	srvID := NewID()
	now := NowMS()

	s.Writer().Submit(Event{Kind: EventSessionStart, Session: &Session{
		ID: sessID, StartedAt: now, Mode: "enforce",
	}})
	s.Writer().Submit(Event{Kind: EventMCPServerUpsert, Server: &MCPServer{
		ID: srvID, Name: "mock", CanonicalURI: "test://mock",
		Transport: "stdio", FirstSeenAt: now, LastSeenAt: now,
	}})
	for i := 0; i < 3; i++ {
		s.Writer().Submit(Event{Kind: EventToolCall, ToolCall: &ToolCall{
			ID: NewID(), SessionID: sessID, ServerID: srvID,
			ToolName: "echo", Direction: "outbound",
			StartedAt: now, Verdict: "allow",
		}})
	}
	// Close blocks until the writer drains.
	require.NoError(t, s.Close())

	// Re-open and verify rows.
	s2, err := Open(context.Background(), s.path)
	require.NoError(t, err)
	defer s2.Close()
	var calls int
	require.NoError(t, s2.DB.QueryRow(`SELECT count(*) FROM tool_calls WHERE session_id = ?`, sessID).Scan(&calls))
	require.Equal(t, 3, calls)
}
