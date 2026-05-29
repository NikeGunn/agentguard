package store

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// seedCallWithStages inserts a session, server, one tool call, and two
// pipeline stages, then blocks until the batched writer has drained them.
// Returns the call id.
func seedCallWithStages(t *testing.T, s *Store) string {
	t.Helper()
	ctx := context.Background()
	sessID, srvID, callID := NewID(), NewID(), NewID()
	now := NowMS()

	s.Writer().Submit(Event{Kind: EventSessionStart, Session: &Session{
		ID: sessID, StartedAt: now, Mode: "enforce",
	}})
	s.Writer().Submit(Event{Kind: EventMCPServerUpsert, Server: &MCPServer{
		ID: srvID, Name: "github-mock", CanonicalURI: "test://github",
		Transport: "stdio", FirstSeenAt: now, LastSeenAt: now,
	}})
	reason := "prompt-injection: ignore-previous-instructions pattern"
	req := `{"method":"tools/call","params":{"name":"create_issue"}}`
	resp := `{"result":"[REDACTED]"}`
	risk := 0.92
	lat := int64(3)
	s.Writer().Submit(Event{Kind: EventToolCall, ToolCall: &ToolCall{
		ID: callID, SessionID: sessID, ServerID: srvID,
		ToolName: "create_issue", Direction: "outbound",
		StartedAt: now, Verdict: "block", VerdictReason: &reason,
		RiskScore: &risk, LatencyMSProxy: &lat,
		RequestInline: &req, ResponseInline: &resp,
	}})
	detail := "matched rule: injection.ignore_instructions"
	s.Writer().Submit(Event{Kind: EventToolCallStage, Stage: &ToolCallStage{
		ID: NewID(), ToolCallID: callID, Stage: "parse",
		StageOrder: 0, StartedAtNS: 0, DurationNS: 1200, Outcome: "pass",
	}})
	s.Writer().Submit(Event{Kind: EventToolCallStage, Stage: &ToolCallStage{
		ID: NewID(), ToolCallID: callID, Stage: "policy",
		StageOrder: 1, StartedAtNS: 1200, DurationNS: 45000, Outcome: "block",
		Detail: &detail,
	}})

	// Close drains the writer deterministically; reopen for reads.
	require.NoError(t, s.Close())
	_ = ctx
	return callID
}

func TestCallDetailReturnsStagesAndDiff(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/detail.db"
	s, err := Open(context.Background(), dbPath)
	require.NoError(t, err)
	callID := seedCallWithStages(t, s)

	s2, err := Open(context.Background(), dbPath)
	require.NoError(t, err)
	defer s2.Close()

	d, err := s2.CallDetail(context.Background(), callID)
	require.NoError(t, err)
	require.Equal(t, callID, d.ID)
	require.Equal(t, "github-mock", d.ServerName)
	require.Equal(t, "block", d.Verdict)
	require.InDelta(t, 0.92, d.RiskScore, 0.0001)
	require.Contains(t, d.VerdictReason, "prompt-injection")
	require.NotEmpty(t, d.RequestInline)
	require.NotEmpty(t, d.ResponseInline)

	require.Len(t, d.Stages, 2)
	require.Equal(t, "parse", d.Stages[0].Stage)
	require.Equal(t, "policy", d.Stages[1].Stage)
	require.Equal(t, "block", d.Stages[1].Outcome)
	require.Equal(t, int64(45000), d.Stages[1].DurationNS)
	require.Contains(t, d.Stages[1].Detail, "injection.ignore_instructions")
}

func TestCallDetailNotFound(t *testing.T) {
	s := tempStore(t)
	_, err := s.CallDetail(context.Background(), "nonexistent-id")
	require.True(t, errors.Is(err, ErrCallNotFound))
}
