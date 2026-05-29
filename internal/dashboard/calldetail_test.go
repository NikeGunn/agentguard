package dashboard

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agentguard/agentguard/internal/store"
)

func TestCallDetailHandler(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "h.db")
	s, err := store.Open(context.Background(), dbPath)
	require.NoError(t, err)

	sessID, srvID, callID := store.NewID(), store.NewID(), store.NewID()
	now := store.NowMS()
	s.Writer().Submit(store.Event{Kind: store.EventSessionStart, Session: &store.Session{
		ID: sessID, StartedAt: now, Mode: "enforce",
	}})
	s.Writer().Submit(store.Event{Kind: store.EventMCPServerUpsert, Server: &store.MCPServer{
		ID: srvID, Name: "mock", CanonicalURI: "test://mock",
		Transport: "stdio", FirstSeenAt: now, LastSeenAt: now,
	}})
	s.Writer().Submit(store.Event{Kind: store.EventToolCall, ToolCall: &store.ToolCall{
		ID: callID, SessionID: sessID, ServerID: srvID,
		ToolName: "echo", Direction: "outbound", StartedAt: now, Verdict: "allow",
	}})
	s.Writer().Submit(store.Event{Kind: store.EventToolCallStage, Stage: &store.ToolCallStage{
		ID: store.NewID(), ToolCallID: callID, Stage: "parse",
		StageOrder: 0, StartedAtNS: 0, DurationNS: 900, Outcome: "pass",
	}})
	require.NoError(t, s.Close())

	s2, err := store.Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer s2.Close()

	srv := New(Config{Store: s2})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	t.Run("found", func(t *testing.T) {
		resp, err := ts.Client().Get(ts.URL + "/api/calls/" + callID)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, 200, resp.StatusCode)
		var d store.CallDetail
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&d))
		require.Equal(t, callID, d.ID)
		require.Len(t, d.Stages, 1)
		require.Equal(t, "parse", d.Stages[0].Stage)
	})

	t.Run("not found", func(t *testing.T) {
		resp, err := ts.Client().Get(ts.URL + "/api/calls/does-not-exist")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, 404, resp.StatusCode)
	})
}
