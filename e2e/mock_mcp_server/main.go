// Command mock_mcp_server is a minimal newline-delimited JSON-RPC server used
// by the e2e suite to exercise `agentguard wrap` without needing a real MCP
// implementation.
//
// It implements a tiny subset:
//   - `initialize`        → returns a fixed protocolVersion + capabilities.
//   - `tools/list`        → returns three canned tools (echo, add, ping).
//   - `tools/call`        → dispatches to those three tools.
//
// Anything else returns a -32601 method-not-found error so the wrapper test
// can still measure routing.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type req struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type resp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcErr         `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	out := bufio.NewWriter(os.Stdout)
	enc := json.NewEncoder(out)
	for {
		line, err := in.ReadBytes('\n')
		if len(line) > 0 {
			handle(line, enc, out)
		}
		if err != nil {
			return
		}
	}
}

func handle(line []byte, enc *json.Encoder, w *bufio.Writer) {
	var r req
	if err := json.Unmarshal(line, &r); err != nil {
		return
	}
	if len(r.ID) == 0 {
		// notification: nothing to reply.
		return
	}
	res := resp{JSONRPC: "2.0", ID: r.ID}
	switch r.Method {
	case "initialize":
		res.Result = map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "mock-mcp", "version": "0.1.0"},
		}
	case "tools/list":
		res.Result = map[string]any{
			"tools": []map[string]any{
				{"name": "echo", "description": "echoes back its argument", "inputSchema": map[string]any{"type": "object"}},
				{"name": "add", "description": "adds two integers", "inputSchema": map[string]any{"type": "object"}},
				{"name": "ping", "description": "returns pong", "inputSchema": map[string]any{"type": "object"}},
			},
		}
	case "tools/call":
		res.Result = handleToolCall(r.Params)
	default:
		res.Error = &rpcErr{Code: -32601, Message: "method not found: " + r.Method}
	}
	if err := enc.Encode(res); err != nil {
		fmt.Fprintln(os.Stderr, "mock encode:", err)
	}
	_ = w.Flush()
}

func handleToolCall(params json.RawMessage) any {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return map[string]any{"isError": true, "content": []any{textBlock("bad params")}}
	}
	switch p.Name {
	case "echo":
		var a struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(p.Arguments, &a)
		return map[string]any{"content": []any{textBlock(a.Message)}}
	case "add":
		var a struct {
			A int `json:"a"`
			B int `json:"b"`
		}
		_ = json.Unmarshal(p.Arguments, &a)
		return map[string]any{"content": []any{textBlock(fmt.Sprintf("%d", a.A+a.B))}}
	case "ping":
		return map[string]any{"content": []any{textBlock("pong")}}
	}
	return map[string]any{"isError": true, "content": []any{textBlock("unknown tool: " + p.Name)}}
}

func textBlock(s string) map[string]any {
	return map[string]any{"type": "text", "text": s}
}
