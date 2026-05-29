// Package client is AgentGuard's public, read-only Go client for the local
// dashboard API (http://127.0.0.1:7878/api/*). It's the supported surface for
// the npm wrapper and any third-party tooling that wants to read what the
// sidecar has observed — recent calls, server trust scores, overview stats —
// without touching the SQLite file directly.
//
// It is read-only by design: mutating policies or packs goes through the
// `agentguard` CLI, never this client.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// DefaultBaseURL is where the dashboard binds by default (system-design.md §10).
const DefaultBaseURL = "http://127.0.0.1:7878"

// Client talks to a running AgentGuard dashboard.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New returns a Client for baseURL (empty → DefaultBaseURL).
func New(baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL: baseURL,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Overview mirrors store.OverviewStats — the home-page stat block. Defined
// here (rather than imported from internal/) so this public package has no
// dependency on internal code.
type Overview struct {
	WindowSeconds  int64   `json:"window_seconds"`
	TotalCalls     int64   `json:"total_calls"`
	TotalBlocked   int64   `json:"total_blocked"`
	TotalFlagged   int64   `json:"total_flagged"`
	TotalTransform int64   `json:"total_transform"`
	BlockRate      float64 `json:"block_rate"`
	ActiveAgents   int64   `json:"active_agents"`
	KnownServers   int64   `json:"known_servers"`
	OpenSessions   int64   `json:"open_sessions"`
}

// Call is one row from /api/calls.
type Call struct {
	ID         string  `json:"id"`
	ServerName string  `json:"server_name"`
	ToolName   string  `json:"tool_name"`
	Direction  string  `json:"direction"`
	Verdict    string  `json:"verdict"`
	Reason     string  `json:"reason"`
	StartedAt  int64   `json:"started_at"`
	LatencyMS  int64   `json:"latency_ms"`
	RiskScore  float64 `json:"risk_score"`
}

// Server is one row from /api/servers.
type Server struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Transport  string `json:"transport"`
	TrustScore *int   `json:"trust_score"`
	FirstSeen  int64  `json:"first_seen_at"`
	LastSeen   int64  `json:"last_seen_at"`
	TotalCalls int64  `json:"total_calls"`
	Blocks     int64  `json:"blocks"`
}

// StageRow is one entry in a call's pipeline waterfall (durations in ns).
type StageRow struct {
	Stage       string `json:"stage"`
	Order       int    `json:"order"`
	StartedAtNS int64  `json:"started_at_ns"`
	DurationNS  int64  `json:"duration_ns"`
	Outcome     string `json:"outcome"`
	Detail      string `json:"detail,omitempty"`
}

// ArtifactRow is a stored request/response/diff blob attached to a call.
type ArtifactRow struct {
	Kind        string `json:"kind"`
	ContentType string `json:"content_type"`
	Encoding    string `json:"encoding"`
	SizeBytes   int64  `json:"size_bytes"`
	Body        string `json:"body"`
}

// CallDetail is the full single-call record from /api/calls/{id}: the call
// header, inline request/response, the ordered stage waterfall, and artifacts.
type CallDetail struct {
	ID             string  `json:"id"`
	SessionID      string  `json:"session_id"`
	ServerID       string  `json:"server_id"`
	ServerName     string  `json:"server_name"`
	ToolName       string  `json:"tool_name"`
	Direction      string  `json:"direction"`
	Verdict        string  `json:"verdict"`
	VerdictReason  string  `json:"verdict_reason,omitempty"`
	RiskScore      float64 `json:"risk_score,omitempty"`
	StartedAt      int64   `json:"started_at"`
	CompletedAt    int64   `json:"completed_at,omitempty"`
	LatencyMSProxy int64   `json:"latency_ms_proxy,omitempty"`
	LatencyMSUp    int64   `json:"latency_ms_upstream,omitempty"`
	CostUSD        float64 `json:"cost_usd"`
	TokenCount     int64   `json:"token_count"`
	RequestInline  string  `json:"request_inline,omitempty"`
	ResponseInline string  `json:"response_inline,omitempty"`
	ErrorInline    string  `json:"error_inline,omitempty"`
	RequestSizeB   int64   `json:"request_size_bytes,omitempty"`
	ResponseSizeB  int64   `json:"response_size_bytes,omitempty"`

	Stages    []StageRow    `json:"stages"`
	Artifacts []ArtifactRow `json:"artifacts"`
}

// Overview fetches the dashboard's overview stats.
func (c *Client) Overview(ctx context.Context) (*Overview, error) {
	var o Overview
	if err := c.getJSON(ctx, "/api/overview", nil, &o); err != nil {
		return nil, err
	}
	return &o, nil
}

// Calls fetches the most recent calls. verdictFilter is optional ("" = all);
// limit ≤ 0 uses the server default.
func (c *Client) Calls(ctx context.Context, limit int, verdictFilter string) ([]Call, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if verdictFilter != "" {
		q.Set("verdict", verdictFilter)
	}
	var calls []Call
	if err := c.getJSON(ctx, "/api/calls", q, &calls); err != nil {
		return nil, err
	}
	return calls, nil
}

// Call fetches the full detail for a single tool call by id.
func (c *Client) Call(ctx context.Context, id string) (*CallDetail, error) {
	if id == "" {
		return nil, fmt.Errorf("client: empty call id")
	}
	var d CallDetail
	if err := c.getJSON(ctx, "/api/calls/"+url.PathEscape(id), nil, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// Servers fetches known MCP servers with their trust scores.
func (c *Client) Servers(ctx context.Context) ([]Server, error) {
	var servers []Server
	if err := c.getJSON(ctx, "/api/servers", nil, &servers); err != nil {
		return nil, err
	}
	return servers, nil
}

// getJSON is the shared GET+decode path. It returns a descriptive error for
// non-2xx responses so callers can tell "daemon down" from "bad request".
func (c *Client) getJSON(ctx context.Context, path string, q url.Values, out any) error {
	u := c.BaseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("client: GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("client: GET %s: unexpected status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("client: decode %s: %w", path, err)
	}
	return nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}
