package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnalytics_DefaultEngineIsSQLite(t *testing.T) {
	s := tempStore(t)
	defer s.Close()

	a, err := NewAnalytics(s)
	require.NoError(t, err)
	defer func() { _ = a.Close() }()

	// On the default (pure-Go) build the SQLite engine answers directly.
	require.Equal(t, EngineSQLite, a.Engine())
	require.False(t, a.Available())
}

func TestAnalytics_PassThroughQueries(t *testing.T) {
	s := tempStore(t)
	ctx := context.Background()
	now := NowMS()
	sessID, srvID := NewID(), NewID()

	s.Writer().Submit(Event{Kind: EventSessionStart, Session: &Session{ID: sessID, StartedAt: now, Mode: "enforce"}})
	s.Writer().Submit(Event{Kind: EventMCPServerUpsert, Server: &MCPServer{
		ID: srvID, Name: "github", CanonicalURI: "test://gh", Transport: "stdio", FirstSeenAt: now, LastSeenAt: now,
	}})
	for i := 0; i < 3; i++ {
		s.Writer().Submit(Event{Kind: EventToolCall, ToolCall: &ToolCall{
			ID: NewID(), SessionID: sessID, ServerID: srvID,
			ToolName: "create_issue", Direction: "outbound", StartedAt: now, Verdict: "allow",
		}})
	}
	require.NoError(t, s.Close())

	s2 := reopen(t, s.path)
	defer s2.Close()
	a, err := NewAnalytics(s2)
	require.NoError(t, err)
	defer func() { _ = a.Close() }()

	tools, err := a.TopTools(ctx, 0, 10)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	require.Equal(t, "create_issue", tools[0].ToolName)
	require.Equal(t, int64(3), tools[0].Calls)

	servers, err := a.Servers(ctx, 10)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	require.Equal(t, int64(3), servers[0].TotalCalls)
}

func TestSecs(t *testing.T) {
	require.Zero(t, secs(0))
	require.Zero(t, secs(-5))
	require.Equal(t, int64(60), int64(secs(60).Seconds()))
}

// reopen opens an existing db path for reading after the writer drained.
func reopen(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Open(context.Background(), path)
	require.NoError(t, err)
	return s
}
