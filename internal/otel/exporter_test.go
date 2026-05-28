package otelexport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNoOpWhenEndpointEmpty(t *testing.T) {
	e := NewExporter("", "agentguard")
	e.Record(ToolCallSpan{ID: "x"})
	require.NoError(t, e.Flush(context.Background()))
	require.Equal(t, 0, e.PendingCount())
}

func TestExportPostsSpans(t *testing.T) {
	var received otlpRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		w.WriteHeader(200)
	}))
	defer srv.Close()

	e := NewExporter(srv.URL, "agentguard-test")
	start := time.Now()
	e.Record(ToolCallSpan{
		ID:         "01H",
		SessionID:  "s1",
		ServerName: "github-mcp",
		ToolName:   "create_issue",
		Verdict:    "block",
		Reason:     "secret",
		StartedAt:  start,
		EndedAt:    start.Add(50 * time.Millisecond),
		Direction:  "outbound",
	})
	require.Equal(t, 1, e.PendingCount())
	require.NoError(t, e.Flush(context.Background()))
	require.Equal(t, 0, e.PendingCount())
	require.Len(t, received.ResourceSpans, 1)
	require.Len(t, received.ResourceSpans[0].ScopeSpans[0].Spans, 1)
	sp := received.ResourceSpans[0].ScopeSpans[0].Spans[0]
	require.Equal(t, "mcp.tool_call", sp.Name)
	require.Equal(t, statusErr, sp.Status.Code)
}
