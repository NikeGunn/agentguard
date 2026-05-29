package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/agentguard/agentguard/internal/pipeline"
)

// HTTPProxy is the streamable-HTTP transport. An agent configured for an HTTP
// MCP server is rewritten to point at AgentGuard's local proxy
// (http://127.0.0.1:7879/mcp/<server-name>, system-design.md §6.4); this
// handler runs the inspection pipeline over the JSON-RPC request body and, if
// the verdict allows, reverse-proxies to the real upstream.
//
// It shares the same Router (and therefore the same chain) as the stdio
// transport, so a blocked tool call looks identical no matter the wire.
type HTTPProxy struct {
	// ServerName is the canonical name used in tool_calls/sessions.
	ServerName string
	// Upstream is the real MCP server URL we forward allowed requests to.
	Upstream *url.URL
	// Router selects and runs the inspection chain.
	Router *Router
	// OnVerdict is an optional hook fired after each inspection so the caller
	// can persist a tool_call. Kept as a hook (rather than wiring the store in
	// here) so the transport stays storage-agnostic and trivially testable.
	OnVerdict func(m *pipeline.Message, verdict pipeline.Verdict, traces []pipeline.Trace)

	reverse *httputil.ReverseProxy
}

// NewHTTPProxy builds an HTTPProxy. upstream must be an absolute URL.
func NewHTTPProxy(serverName string, upstream *url.URL, router *Router) *HTTPProxy {
	p := &HTTPProxy{ServerName: serverName, Upstream: upstream, Router: router}
	p.reverse = httputil.NewSingleHostReverseProxy(upstream)
	return p
}

// blockedResponse is the JSON-RPC error AgentGuard returns to the agent when a
// call is blocked — identical shape to the stdio transport's errorResponse.
func blockedResponse(id json.RawMessage, reason string) ([]byte, int) {
	type rpcErr struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id,omitempty"`
		Error   struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	var e rpcErr
	e.JSONRPC = "2.0"
	e.ID = id
	e.Error.Code = -32099
	e.Error.Message = "Blocked by AgentGuard: " + reason
	b, _ := json.Marshal(e)
	// 200 with a JSON-RPC error body is the MCP-correct way to signal an
	// application-level block; the transport succeeded, the call did not.
	return b, http.StatusOK
}

// ServeHTTP implements http.Handler. It buffers the request body (MCP messages
// are small JSON-RPC frames), inspects it, and either forwards to upstream or
// returns a JSON-RPC block error.
func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20)) // 8 MiB cap; tool calls are tiny, results can be larger
	_ = r.Body.Close()
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	msg := parseRPC(body)
	pm := &pipeline.Message{
		ServerName: p.ServerName,
		ToolName:   msg.toolName(),
		Method:     msg.Method,
		Direction:  pipeline.Outbound,
		Raw:        body,
	}
	traces, verdict := p.Router.Inspect(r.Context(), TransportStreamableHTTP, pm)
	if p.OnVerdict != nil {
		p.OnVerdict(pm, verdict, traces)
	}

	switch verdict {
	case pipeline.VerdictBlock, pipeline.VerdictError:
		out, code := blockedResponse(msg.ID, firstReason(traces, verdict))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write(out)
		return
	case pipeline.VerdictTransform:
		// A stage may have redacted the body; forward the rewritten bytes.
		body = pm.Raw
	}

	// Forward to upstream. Restore the (possibly transformed) body and let the
	// reverse proxy stream the response back unchanged.
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	p.reverse.ServeHTTP(w, r)
}

// rpcFrame is the minimal JSON-RPC envelope the HTTP transport needs.
type rpcFrame struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func parseRPC(b []byte) rpcFrame {
	var f rpcFrame
	_ = json.Unmarshal(bytes.TrimSpace(b), &f)
	return f
}

func (f rpcFrame) toolName() string {
	if f.Method != "tools/call" || len(f.Params) == 0 {
		return ""
	}
	var p struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(f.Params, &p)
	return p.Name
}

// firstReason mirrors stdio's reason extraction: the first trace whose verdict
// matches, else a generic label.
func firstReason(traces []pipeline.Trace, verdict pipeline.Verdict) string {
	for _, t := range traces {
		if t.Result.Verdict == verdict && t.Result.Reason != "" {
			return t.Stage + ":" + t.Result.Reason
		}
	}
	return strings.TrimSpace(string(verdict))
}
