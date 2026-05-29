package proxy

import (
	"context"
	"testing"

	"github.com/agentguard/agentguard/internal/pipeline"
)

// testStage returns a fixed verdict; used across the proxy transport tests.
type testStage struct {
	name    string
	verdict pipeline.Verdict
	reason  string
	xform   []byte
}

func (s testStage) Name() string { return s.name }
func (s testStage) Run(_ context.Context, _ *pipeline.Message) pipeline.StageResult {
	return pipeline.StageResult{Verdict: s.verdict, Reason: s.reason, Transform: s.xform}
}

func TestParseTransport(t *testing.T) {
	cases := map[string]Transport{
		"stdio":           TransportStdio,
		"STDIO":           TransportStdio,
		"streamable-http": TransportStreamableHTTP,
		"streamable_http": TransportStreamableHTTP,
		"http":            TransportStreamableHTTP,
		"https":           TransportStreamableHTTP,
		"sse":             TransportSSE,
		"":                TransportStdio,
		"nonsense":        TransportStdio,
	}
	for in, want := range cases {
		if got := ParseTransport(in); got != want {
			t.Errorf("ParseTransport(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRouter_NilChainPasses(t *testing.T) {
	r := NewRouter(nil)
	traces, verdict := r.Inspect(context.Background(), TransportStdio, &pipeline.Message{})
	if verdict != pipeline.VerdictPass {
		t.Fatalf("nil chain should pass, got %s", verdict)
	}
	if traces != nil {
		t.Fatalf("nil chain should produce no traces, got %d", len(traces))
	}
}

func TestRouter_RunsChain(t *testing.T) {
	chain := pipeline.NewChain(testStage{name: "deny", verdict: pipeline.VerdictBlock, reason: "nope"})
	r := NewRouter(chain)
	traces, verdict := r.Inspect(context.Background(), TransportStreamableHTTP, &pipeline.Message{Direction: pipeline.Outbound})
	if verdict != pipeline.VerdictBlock {
		t.Fatalf("want block, got %s", verdict)
	}
	if len(traces) != 1 || traces[0].Stage != "deny" {
		t.Fatalf("unexpected traces: %+v", traces)
	}
}

func TestRouter_ChainSelection(t *testing.T) {
	chain := pipeline.NewChain()
	r := NewRouter(chain)
	// All transports/directions resolve to the same configured chain today.
	for _, tr := range []Transport{TransportStdio, TransportStreamableHTTP, TransportSSE} {
		for _, d := range []pipeline.Direction{pipeline.Inbound, pipeline.Outbound} {
			if r.Chain(tr, d) != chain {
				t.Errorf("Chain(%s,%s) returned a different chain", tr, d)
			}
		}
	}
}
