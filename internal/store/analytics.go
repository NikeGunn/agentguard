package store

import (
	"context"
	"database/sql"
	"time"
)

// Analytics holds the read-only queries the dashboard issues. Every method
// takes the context so the dashboard can cancel a slow query when the SSE
// connection drops. Returned slices are bounded by `limit` arguments — no
// query ever returns more than a few hundred rows.

// OverviewStats is the single object the dashboard's home page renders.
// It's intentionally small (sub-kilobyte JSON) so the SSE channel can
// re-broadcast it cheaply on every change.
type OverviewStats struct {
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

// CallsPerMinute is one bucket in the home-page sparkline.
type CallsPerMinute struct {
	Bucket  int64 `json:"bucket"` // unix epoch ms at minute start
	Calls   int64 `json:"calls"`
	Blocked int64 `json:"blocked"`
}

// ToolUsage is one row in the "top tools" panel.
type ToolUsage struct {
	ServerName string  `json:"server_name"`
	ToolName   string  `json:"tool_name"`
	Calls      int64   `json:"calls"`
	Blocks     int64   `json:"blocks"`
	AvgLatency float64 `json:"avg_latency_ms"`
}

// ServerSummary is one row on the /servers page.
type ServerSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Transport   string `json:"transport"`
	TrustScore  *int   `json:"trust_score,omitempty"`
	FirstSeenAt int64  `json:"first_seen_at"`
	LastSeenAt  int64  `json:"last_seen_at"`
	TotalCalls  int64  `json:"total_calls"`
	Blocks      int64  `json:"blocks"`
}

// CallRow is one row on the /calls page; identical shape to the tail TUI.
type CallRow struct {
	ID         string  `json:"id"`
	ServerName string  `json:"server_name"`
	ToolName   string  `json:"tool_name"`
	Direction  string  `json:"direction"`
	Verdict    string  `json:"verdict"`
	Reason     string  `json:"reason,omitempty"`
	StartedAt  int64   `json:"started_at"`
	LatencyMS  int64   `json:"latency_ms"`
	RiskScore  float64 `json:"risk_score,omitempty"`
}

// Overview returns dashboard home-page stats for the last `window` time
// span. window=0 means "all time".
func (s *Store) Overview(ctx context.Context, window time.Duration) (*OverviewStats, error) {
	out := &OverviewStats{WindowSeconds: int64(window.Seconds())}
	since := windowStart(window)
	row := s.DB.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			SUM(CASE WHEN verdict='block'     THEN 1 ELSE 0 END),
			SUM(CASE WHEN verdict='flag'      THEN 1 ELSE 0 END),
			SUM(CASE WHEN verdict='transform' THEN 1 ELSE 0 END)
		FROM tool_calls
		WHERE started_at >= ?`, since)
	var blocked, flagged, xform sql.NullInt64
	if err := row.Scan(&out.TotalCalls, &blocked, &flagged, &xform); err != nil {
		return nil, err
	}
	out.TotalBlocked = blocked.Int64
	out.TotalFlagged = flagged.Int64
	out.TotalTransform = xform.Int64
	if out.TotalCalls > 0 {
		out.BlockRate = float64(out.TotalBlocked) / float64(out.TotalCalls)
	}
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM agents WHERE active = 1`).Scan(&out.ActiveAgents); err != nil {
		return nil, err
	}
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM mcp_servers`).Scan(&out.KnownServers); err != nil {
		return nil, err
	}
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sessions WHERE ended_at IS NULL`).Scan(&out.OpenSessions); err != nil {
		return nil, err
	}
	return out, nil
}

// CallsByMinute returns a per-minute timeseries for the last `window`. Each
// bucket is the unix epoch ms at the START of the minute (UTC).
func (s *Store) CallsByMinute(ctx context.Context, window time.Duration) ([]CallsPerMinute, error) {
	since := windowStart(window)
	rows, err := s.DB.QueryContext(ctx, `
		SELECT
			(started_at / 60000) * 60000 AS bucket,
			COUNT(*),
			SUM(CASE WHEN verdict='block' THEN 1 ELSE 0 END)
		FROM tool_calls
		WHERE started_at >= ?
		GROUP BY bucket
		ORDER BY bucket ASC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CallsPerMinute
	for rows.Next() {
		var c CallsPerMinute
		var blocked sql.NullInt64
		if err := rows.Scan(&c.Bucket, &c.Calls, &blocked); err != nil {
			return nil, err
		}
		c.Blocked = blocked.Int64
		out = append(out, c)
	}
	return out, rows.Err()
}

// TopTools returns the most-called tools in the window, ordered by call
// count descending. Use limit to bound the result.
func (s *Store) TopTools(ctx context.Context, window time.Duration, limit int) ([]ToolUsage, error) {
	if limit <= 0 {
		limit = 10
	}
	since := windowStart(window)
	rows, err := s.DB.QueryContext(ctx, `
		SELECT
			COALESCE(s.name, '') AS server_name,
			tc.tool_name,
			COUNT(*) AS calls,
			SUM(CASE WHEN tc.verdict='block' THEN 1 ELSE 0 END) AS blocks,
			COALESCE(AVG(tc.latency_ms_proxy), 0) AS avg_lat
		FROM tool_calls tc
		LEFT JOIN mcp_servers s ON s.id = tc.server_id
		WHERE tc.started_at >= ?
		GROUP BY server_name, tc.tool_name
		ORDER BY calls DESC
		LIMIT ?`, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ToolUsage
	for rows.Next() {
		var t ToolUsage
		if err := rows.Scan(&t.ServerName, &t.ToolName, &t.Calls, &t.Blocks, &t.AvgLatency); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Servers returns the full mcp_servers list joined with per-server call
// totals. Sorted by last_seen_at descending.
func (s *Store) Servers(ctx context.Context, limit int) ([]ServerSummary, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT
			s.id, s.name, s.transport, s.trust_score,
			s.first_seen_at, s.last_seen_at,
			COALESCE(stats.total, 0)   AS total,
			COALESCE(stats.blocks, 0)  AS blocks
		FROM mcp_servers s
		LEFT JOIN (
			SELECT server_id,
			       COUNT(*) AS total,
			       SUM(CASE WHEN verdict='block' THEN 1 ELSE 0 END) AS blocks
			FROM tool_calls
			GROUP BY server_id
		) stats ON stats.server_id = s.id
		ORDER BY s.last_seen_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ServerSummary
	for rows.Next() {
		var s ServerSummary
		var trust sql.NullInt64
		if err := rows.Scan(&s.ID, &s.Name, &s.Transport, &trust,
			&s.FirstSeenAt, &s.LastSeenAt, &s.TotalCalls, &s.Blocks); err != nil {
			return nil, err
		}
		if trust.Valid {
			t := int(trust.Int64)
			s.TrustScore = &t
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// RecentCalls returns the latest N tool_calls joined to server name. Used
// by the /calls page and as the SSE backfill on connect.
func (s *Store) RecentCalls(ctx context.Context, limit int, verdictFilter string) ([]CallRow, error) {
	if limit <= 0 {
		limit = 100
	}
	q := `
		SELECT
			tc.id,
			COALESCE(s.name, ''),
			tc.tool_name,
			tc.direction,
			tc.verdict,
			COALESCE(tc.verdict_reason, ''),
			tc.started_at,
			COALESCE(tc.latency_ms_proxy, 0),
			COALESCE(tc.risk_score, 0)
		FROM tool_calls tc
		LEFT JOIN mcp_servers s ON s.id = tc.server_id`
	args := []any{}
	if verdictFilter != "" {
		q += ` WHERE tc.verdict = ?`
		args = append(args, verdictFilter)
	}
	q += ` ORDER BY tc.started_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CallRow
	for rows.Next() {
		var r CallRow
		if err := rows.Scan(&r.ID, &r.ServerName, &r.ToolName, &r.Direction,
			&r.Verdict, &r.Reason, &r.StartedAt, &r.LatencyMS, &r.RiskScore); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// windowStart returns the unix-ms cutoff for "now minus window". A zero
// window means "since the dawn of time".
func windowStart(window time.Duration) int64 {
	if window <= 0 {
		return 0
	}
	return time.Now().Add(-window).UnixMilli()
}
