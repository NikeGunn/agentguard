package proxy

import (
	"context"
	"strings"
	"testing"

	"github.com/agentguard/agentguard/internal/pipeline"
)

func TestSSEScanner_PassForwardsVerbatim(t *testing.T) {
	in := "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"ok\":true}}\n\n"
	chain := pipeline.NewChain(testStage{name: "ok", verdict: pipeline.VerdictPass})
	var out strings.Builder
	s := NewSSEScanner("notion", NewRouter(chain))
	if err := s.Pump(context.Background(), strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != in {
		t.Fatalf("pass should forward verbatim:\n got %q\nwant %q", out.String(), in)
	}
}

func TestSSEScanner_BlockDropsEvent(t *testing.T) {
	// Two events; the scanner blocks the first and passes the second.
	in := "data: {\"jsonrpc\":\"2.0\",\"method\":\"evil\"}\n\ndata: {\"jsonrpc\":\"2.0\",\"id\":2,\"result\":1}\n\n"
	chain := pipeline.NewChain(blockEvil{})
	var out strings.Builder
	s := NewSSEScanner("notion", NewRouter(chain))
	if err := s.Pump(context.Background(), strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, "evil") {
		t.Fatalf("blocked event must be dropped, got: %q", got)
	}
	if !strings.Contains(got, `"id":2`) {
		t.Fatalf("subsequent clean event must pass, got: %q", got)
	}
}

func TestSSEScanner_TransformRewritesPayload(t *testing.T) {
	in := "data: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":\"sk-secret\"}\n\n"
	redacted := []byte(`{"jsonrpc":"2.0","id":1,"result":"[REDACTED]"}`)
	chain := pipeline.NewChain(testStage{name: "scan", verdict: pipeline.VerdictTransform, xform: redacted})
	var out strings.Builder
	s := NewSSEScanner("notion", NewRouter(chain))
	if err := s.Pump(context.Background(), strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "[REDACTED]") || strings.Contains(got, "sk-secret") {
		t.Fatalf("transform not applied: %q", got)
	}
}

func TestSSEScanner_MultiLineDataJoined(t *testing.T) {
	// SSE allows a payload split across multiple data: lines.
	in := "data: {\"jsonrpc\":\"2.0\",\ndata: \"id\":1}\n\n"
	var seen string
	chain := pipeline.NewChain(captureStage{onRun: func(m *pipeline.Message) { seen = string(m.Raw) }})
	var out strings.Builder
	s := NewSSEScanner("notion", NewRouter(chain))
	if err := s.Pump(context.Background(), strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(seen, `"jsonrpc":"2.0"`) || !strings.Contains(seen, `"id":1`) {
		t.Fatalf("multi-line data not joined: %q", seen)
	}
}

func TestSSEScanner_HeartbeatPassesThrough(t *testing.T) {
	in := ": keep-alive comment\n\n"
	chain := pipeline.NewChain(testStage{name: "ok", verdict: pipeline.VerdictPass})
	var out strings.Builder
	s := NewSSEScanner("notion", NewRouter(chain))
	if err := s.Pump(context.Background(), strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "keep-alive") {
		t.Fatalf("comment/heartbeat should pass through: %q", out.String())
	}
}

// blockEvil blocks any frame whose method is "evil".
type blockEvil struct{}

func (blockEvil) Name() string { return "block-evil" }
func (blockEvil) Run(_ context.Context, m *pipeline.Message) pipeline.StageResult {
	if m.Method == "evil" {
		return pipeline.StageResult{Verdict: pipeline.VerdictBlock, Reason: "evil method"}
	}
	return pipeline.StageResult{Verdict: pipeline.VerdictPass}
}

// captureStage records the message it saw, then passes.
type captureStage struct{ onRun func(*pipeline.Message) }

func (captureStage) Name() string { return "capture" }
func (c captureStage) Run(_ context.Context, m *pipeline.Message) pipeline.StageResult {
	if c.onRun != nil {
		c.onRun(m)
	}
	return pipeline.StageResult{Verdict: pipeline.VerdictPass}
}
