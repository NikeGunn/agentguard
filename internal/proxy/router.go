package proxy

import (
	"context"
	"strings"

	"github.com/agentguard/agentguard/internal/pipeline"
)

// Transport names the wire protocol a server speaks. It mirrors the
// mcp_servers.transport column (database.md §3.2).
type Transport string

const (
	TransportStdio          Transport = "stdio"
	TransportStreamableHTTP Transport = "streamable-http"
	TransportSSE            Transport = "sse"
)

// ParseTransport normalises a free-form transport string (as found in an agent
// config or a canonical_uri) into a Transport. Unknown values fall back to
// stdio, the most common MCP transport.
func ParseTransport(s string) Transport {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "streamable-http", "streamable_http", "http", "https":
		return TransportStreamableHTTP
	case "sse":
		return TransportSSE
	default:
		return TransportStdio
	}
}

// Router decides which inspection chain runs for a given message. All three
// transports share the same pipeline today — the security properties of a tool
// call don't depend on how the bytes arrived — but the router is the single
// seam where that could change (e.g. an HTTP-only transport guard that checks
// the Authorization header). Centralising the decision keeps every transport's
// proxy loop identical and DRY.
type Router struct {
	// chain is the inspection chain. A nil chain means "transparent
	// passthrough" (the --no-inspect / milestone-1 behaviour).
	chain *pipeline.Chain
}

// NewRouter binds a Router to one inspection chain. Pass nil for a transparent
// proxy that records traffic without inspecting it.
func NewRouter(chain *pipeline.Chain) *Router { return &Router{chain: chain} }

// Chain returns the inspection chain selected for a message. Both directions
// and all transports currently resolve to the same configured chain; the
// method exists so callers never reach past the router to pick a chain
// themselves.
func (r *Router) Chain(_ Transport, _ pipeline.Direction) *pipeline.Chain {
	return r.chain
}

// Inspect runs the selected chain against m and returns the traces and final
// verdict. When no chain is configured it returns an empty trace set and
// VerdictPass so callers have a single code path regardless of inspection
// being enabled. This is what the HTTP and SSE transports call per frame, the
// exact analogue of what stdio's pump does inline.
func (r *Router) Inspect(ctx context.Context, transport Transport, m *pipeline.Message) ([]pipeline.Trace, pipeline.Verdict) {
	chain := r.Chain(transport, m.Direction)
	if chain == nil {
		return nil, pipeline.VerdictPass
	}
	return chain.Run(ctx, m)
}
