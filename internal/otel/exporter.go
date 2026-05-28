// Package otelexport implements a minimal OTLP/HTTP span exporter for
// AgentGuard tool calls. One span per ToolCall, posted as OTLP/HTTP
// JSON to a collector endpoint (e.g. http://localhost:4318/v1/traces).
//
// Hand-rolled against the OTLP/HTTP JSON schema to avoid pulling the
// full go.opentelemetry.io tree — too big for our single-binary goal.
package otelexport

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Exporter ships tool-call spans to an OTLP/HTTP endpoint.
type Exporter struct {
	Endpoint    string
	ServiceName string
	HTTP        *http.Client
	Timeout     time.Duration

	mu    sync.Mutex
	batch []span
}

// NewExporter returns a configured exporter. Empty endpoint = no-op.
func NewExporter(endpoint, serviceName string) *Exporter {
	return &Exporter{
		Endpoint:    endpoint,
		ServiceName: serviceName,
		HTTP:        &http.Client{Timeout: 5 * time.Second},
		Timeout:     5 * time.Second,
	}
}

// ToolCallSpan is the subset of fields the exporter needs.
type ToolCallSpan struct {
	ID         string
	SessionID  string
	ServerName string
	ToolName   string
	Verdict    string
	Reason     string
	StartedAt  time.Time
	EndedAt    time.Time
	Direction  string
}

// Record adds one span to the in-memory batch.
func (e *Exporter) Record(tc ToolCallSpan) {
	if e == nil || e.Endpoint == "" {
		return
	}
	e.mu.Lock()
	e.batch = append(e.batch, toSpan(tc))
	e.mu.Unlock()
}

// PendingCount returns batch size (diagnostics / tests).
func (e *Exporter) PendingCount() int {
	if e == nil {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.batch)
}

// Flush ships the current batch and resets it.
func (e *Exporter) Flush(ctx context.Context) error {
	if e == nil || e.Endpoint == "" {
		return nil
	}
	e.mu.Lock()
	spans := e.batch
	e.batch = nil
	e.mu.Unlock()
	if len(spans) == 0 {
		return nil
	}

	payload := otlpRequest{
		ResourceSpans: []resourceSpans{{
			Resource: resource{Attributes: []kv{
				{Key: "service.name", Value: anyValue{StringValue: e.ServiceName}},
			}},
			ScopeSpans: []scopeSpans{{
				Scope: scope{Name: "agentguard", Version: "1.0"},
				Spans: spans,
			}},
		}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("otlp: status %d", resp.StatusCode)
	}
	return nil
}

func toSpan(tc ToolCallSpan) span {
	start := tc.StartedAt.UnixNano()
	end := tc.EndedAt.UnixNano()
	if end < start {
		end = start
	}
	status := statusOK
	if tc.Verdict == "block" || tc.Verdict == "error" {
		status = statusErr
	}
	return span{
		TraceID:           hex.EncodeToString(randBytes(16)),
		SpanID:            hex.EncodeToString(randBytes(8)),
		Name:              "mcp.tool_call",
		Kind:              spanKindClient,
		StartTimeUnixNano: fmt.Sprintf("%d", start),
		EndTimeUnixNano:   fmt.Sprintf("%d", end),
		Status:            spanStatus{Code: status},
		Attributes: []kv{
			{Key: "tool_call.id", Value: anyValue{StringValue: tc.ID}},
			{Key: "session.id", Value: anyValue{StringValue: tc.SessionID}},
			{Key: "mcp.server", Value: anyValue{StringValue: tc.ServerName}},
			{Key: "mcp.tool", Value: anyValue{StringValue: tc.ToolName}},
			{Key: "mcp.direction", Value: anyValue{StringValue: tc.Direction}},
			{Key: "verdict", Value: anyValue{StringValue: tc.Verdict}},
			{Key: "verdict.reason", Value: anyValue{StringValue: tc.Reason}},
		},
	}
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return b
}

type otlpRequest struct {
	ResourceSpans []resourceSpans `json:"resourceSpans"`
}
type resourceSpans struct {
	Resource   resource     `json:"resource"`
	ScopeSpans []scopeSpans `json:"scopeSpans"`
}
type scopeSpans struct {
	Scope scope  `json:"scope"`
	Spans []span `json:"spans"`
}
type resource struct {
	Attributes []kv `json:"attributes"`
}
type scope struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type span struct {
	TraceID           string     `json:"traceId"`
	SpanID            string     `json:"spanId"`
	Name              string     `json:"name"`
	Kind              int        `json:"kind"`
	StartTimeUnixNano string     `json:"startTimeUnixNano"`
	EndTimeUnixNano   string     `json:"endTimeUnixNano"`
	Status            spanStatus `json:"status"`
	Attributes        []kv       `json:"attributes"`
}
type kv struct {
	Key   string   `json:"key"`
	Value anyValue `json:"value"`
}
type anyValue struct {
	StringValue string `json:"stringValue,omitempty"`
}
type spanStatus struct {
	Code int `json:"code"`
}

const (
	spanKindClient = 3
	statusOK       = 1
	statusErr      = 2
)
