package store

import (
	"context"
	"database/sql"
	"errors"
)

// ErrCallNotFound is returned by CallDetail when no tool_call has the given id.
var ErrCallNotFound = errors.New("tool call not found")

// StageRow is one entry in the pipeline waterfall for a single tool call.
// Durations are nanoseconds so the dashboard can render sub-millisecond
// stages without losing precision; the frontend formats ns/µs/ms itself.
type StageRow struct {
	Stage       string `json:"stage"`
	Order       int    `json:"order"`
	StartedAtNS int64  `json:"started_at_ns"`
	DurationNS  int64  `json:"duration_ns"`
	Outcome     string `json:"outcome"`
	Detail      string `json:"detail,omitempty"`
}

// ArtifactRow is a stored request/response/diff blob attached to a tool call.
// Body is returned as a string; binary artifacts are base64 by the caller's
// convention (we store utf8 JSON for the common case).
type ArtifactRow struct {
	Kind        string `json:"kind"`
	ContentType string `json:"content_type"`
	Encoding    string `json:"encoding"`
	SizeBytes   int64  `json:"size_bytes"`
	Body        string `json:"body"`
}

// CallDetail is the full record behind the /calls/:id dashboard page: the
// call header, its inline request/response (for the diff view), the ordered
// stage waterfall, and any stored artifacts.
type CallDetail struct {
	ID              string  `json:"id"`
	SessionID       string  `json:"session_id"`
	ServerID        string  `json:"server_id"`
	ServerName      string  `json:"server_name"`
	ToolName        string  `json:"tool_name"`
	Direction       string  `json:"direction"`
	Verdict         string  `json:"verdict"`
	VerdictReason   string  `json:"verdict_reason,omitempty"`
	RiskScore       float64 `json:"risk_score,omitempty"`
	StartedAt       int64   `json:"started_at"`
	CompletedAt     int64   `json:"completed_at,omitempty"`
	LatencyMSProxy  int64   `json:"latency_ms_proxy,omitempty"`
	LatencyMSUp     int64   `json:"latency_ms_upstream,omitempty"`
	CostUSD         float64 `json:"cost_usd"`
	TokenCount      int64   `json:"token_count"`
	RequestInline   string  `json:"request_inline,omitempty"`
	ResponseInline  string  `json:"response_inline,omitempty"`
	ErrorInline     string  `json:"error_inline,omitempty"`
	RequestSizeB    int64   `json:"request_size_bytes,omitempty"`
	ResponseSizeB   int64   `json:"response_size_bytes,omitempty"`

	Stages    []StageRow    `json:"stages"`
	Artifacts []ArtifactRow `json:"artifacts"`
}

// CallDetail loads the full detail for one tool call. Returns ErrCallNotFound
// if the id does not exist so the handler can map it to a 404.
func (s *Store) CallDetail(ctx context.Context, id string) (*CallDetail, error) {
	d := &CallDetail{Stages: []StageRow{}, Artifacts: []ArtifactRow{}}
	var (
		reason, reqInline, respInline, errInline sql.NullString
		risk, cost                                sql.NullFloat64
		completedAt, latProxy, latUp              sql.NullInt64
		reqSize, respSize, tokens                 sql.NullInt64
	)
	row := s.DB.QueryRowContext(ctx, `
		SELECT
			tc.id, tc.session_id, tc.server_id, COALESCE(srv.name, ''),
			tc.tool_name, tc.direction, tc.verdict, tc.verdict_reason,
			tc.risk_score, tc.started_at, tc.completed_at,
			tc.latency_ms_proxy, tc.latency_ms_upstream,
			tc.cost_usd, tc.token_count,
			tc.request_size_bytes, tc.response_size_bytes,
			tc.request_inline, tc.response_inline, tc.error_inline
		FROM tool_calls tc
		LEFT JOIN mcp_servers srv ON srv.id = tc.server_id
		WHERE tc.id = ?`, id)
	err := row.Scan(
		&d.ID, &d.SessionID, &d.ServerID, &d.ServerName,
		&d.ToolName, &d.Direction, &d.Verdict, &reason,
		&risk, &d.StartedAt, &completedAt,
		&latProxy, &latUp,
		&cost, &tokens,
		&reqSize, &respSize,
		&reqInline, &respInline, &errInline,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCallNotFound
	}
	if err != nil {
		return nil, err
	}
	d.VerdictReason = reason.String
	d.RiskScore = risk.Float64
	d.CompletedAt = completedAt.Int64
	d.LatencyMSProxy = latProxy.Int64
	d.LatencyMSUp = latUp.Int64
	d.CostUSD = cost.Float64
	d.TokenCount = tokens.Int64
	d.RequestSizeB = reqSize.Int64
	d.ResponseSizeB = respSize.Int64
	d.RequestInline = reqInline.String
	d.ResponseInline = respInline.String
	d.ErrorInline = errInline.String

	if err := s.loadStages(ctx, d); err != nil {
		return nil, err
	}
	if err := s.loadArtifacts(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *Store) loadStages(ctx context.Context, d *CallDetail) error {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT stage, stage_order, started_at_ns, duration_ns, outcome, COALESCE(detail, '')
		FROM tool_call_stages
		WHERE tool_call_id = ?
		ORDER BY stage_order ASC`, d.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var st StageRow
		if err := rows.Scan(&st.Stage, &st.Order, &st.StartedAtNS, &st.DurationNS, &st.Outcome, &st.Detail); err != nil {
			return err
		}
		d.Stages = append(d.Stages, st)
	}
	return rows.Err()
}

func (s *Store) loadArtifacts(ctx context.Context, d *CallDetail) error {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT kind, content_type, encoding, size_bytes, body
		FROM tool_call_artifacts
		WHERE tool_call_id = ?`, d.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var a ArtifactRow
		var body []byte
		if err := rows.Scan(&a.Kind, &a.ContentType, &a.Encoding, &a.SizeBytes, &body); err != nil {
			return err
		}
		a.Body = string(body)
		d.Artifacts = append(d.Artifacts, a)
	}
	return rows.Err()
}
