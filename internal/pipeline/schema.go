package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
)

// SchemaValidator is stage 1: JSON-RPC well-formedness. It enforces the bits
// of the MCP / A2A frame shape that all sane upstreams agree on so we reject
// malformed traffic before it reaches the policy engine or the upstream.
//
// We deliberately do NOT validate full per-method param schemas here — that
// requires per-server schema knowledge that lives in `server_tools` and is
// stage 2's job. Stage 1 only validates the JSON-RPC envelope itself.
type SchemaValidator struct{}

// Name implements Stage.
func (SchemaValidator) Name() string { return "schema" }

// envelope is the minimum every JSON-RPC frame must satisfy.
type envelope struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

// Run implements Stage.
func (SchemaValidator) Run(_ context.Context, m *Message) StageResult {
	raw := bytes.TrimRight(m.Raw, "\r\n")
	if len(raw) == 0 {
		return StageResult{Verdict: VerdictBlock, Reason: "empty_frame"}
	}
	var env envelope
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields() // false negatives are fine; we want strictness on the envelope
	// MCP servers in the wild include extra envelope keys, so accept those.
	dec = json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(&env); err != nil {
		return StageResult{
			Verdict: VerdictBlock,
			Reason:  "invalid_json",
			Detail:  detailJSON(map[string]any{"error": err.Error()}),
		}
	}
	if env.JSONRPC != "2.0" {
		return StageResult{
			Verdict: VerdictBlock,
			Reason:  "bad_jsonrpc_version",
			Detail:  detailJSON(map[string]any{"got": env.JSONRPC}),
		}
	}
	// Outbound from agent: must be a request or notification (has Method).
	// Inbound from tool: must be a response (Result or Error) OR a server
	// notification (Method).
	switch m.Direction {
	case Outbound:
		if env.Method == "" {
			return StageResult{Verdict: VerdictBlock, Reason: "outbound_without_method"}
		}
	case Inbound:
		if env.Method == "" && len(env.Result) == 0 && len(env.Error) == 0 {
			return StageResult{Verdict: VerdictBlock, Reason: "inbound_without_method_result_or_error"}
		}
		if len(env.Result) > 0 && len(env.Error) > 0 {
			return StageResult{Verdict: VerdictBlock, Reason: "result_and_error_both_set"}
		}
	}
	return StageResult{Verdict: VerdictPass}
}
