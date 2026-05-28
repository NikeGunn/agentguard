package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Tunables for the batched writer. database.md §5 calls for "100 events queued
// or 10ms elapsed, whichever comes first".
const (
	BatchMaxEvents = 100
	BatchMaxWait   = 10 * time.Millisecond
	// Queue capacity sized so a transient stall in the writer doesn't drop
	// events on the floor under bursty load.
	queueCap = 4096
)

// EventKind identifies which insert a queued Event represents.
type EventKind int

const (
	EventToolCall EventKind = iota + 1
	EventToolCallStage
	EventSessionStart
	EventSessionEnd
	EventMCPServerUpsert
	EventAuditLog
)

// Event is the unit submitted to the writer. Concrete typed payloads keep
// callers from constructing raw SQL.
type Event struct {
	Kind     EventKind
	ToolCall *ToolCall
	Stage    *ToolCallStage
	Session  *Session
	Server   *MCPServer
	Audit    *AuditEntry
}

// ToolCall mirrors the tool_calls row. Pointer fields encode SQL NULL.
type ToolCall struct {
	ID                string
	SessionID         string
	ServerID          string
	ToolName          string
	Direction         string // outbound | inbound
	StartedAt         int64
	CompletedAt       *int64
	Verdict           string
	VerdictReason     *string
	RiskScore         *float64
	RequestSizeBytes  *int64
	ResponseSizeBytes *int64
	LatencyMSProxy    *int64
	LatencyMSUpstream *int64
	CostUSD           float64
	TokenCount        int64
	RequestInline     *string
	ResponseInline    *string
	ErrorInline       *string
}

// ToolCallStage mirrors tool_call_stages.
type ToolCallStage struct {
	ID          string
	ToolCallID  string
	Stage       string
	StageOrder  int
	StartedAtNS int64
	DurationNS  int64
	Outcome     string
	Detail      *string
}

// Session mirrors sessions. Update vs insert is determined by EndedAt: if nil
// (and the row is new) we INSERT; if set, we UPDATE.
type Session struct {
	ID           string
	AgentID      *string
	StartedAt    int64
	EndedAt      *int64
	ClientPID    *int
	ClientUser   *string
	Mode         string
	TotalCalls   int64
	TotalBlocked int64
	TotalCostUSD float64
	TotalTokens  int64
}

// MCPServer mirrors mcp_servers. Inserted on first sighting; subsequent
// sightings update last_seen_at via ON CONFLICT.
type MCPServer struct {
	ID              string
	Name            string
	CanonicalURI    string
	Transport       string
	UpstreamCommand *string
	UpstreamURL     *string
	AuthRequired    bool
	FirstSeenAt     int64
	LastSeenAt      int64
}

// AuditEntry mirrors audit_log.
type AuditEntry struct {
	ID         string
	OccurredAt int64
	Actor      string
	Action     string
	TargetType *string
	TargetID   *string
	Detail     *string
}

// batchedWriter funnels Events from the proxy goroutines into the DB in
// transactional batches.
type batchedWriter struct {
	s        *Store
	queue    chan Event
	stopCh   chan struct{}
	doneCh   chan struct{}
	dropOnce sync.Once
	stopOnce sync.Once
}

func newBatchedWriter(s *Store) *batchedWriter {
	return &batchedWriter{
		s:      s,
		queue:  make(chan Event, queueCap),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (w *batchedWriter) start() { go w.run() }

func (w *batchedWriter) stop() {
	w.stopOnce.Do(func() { close(w.stopCh) })
	<-w.doneCh
}

// Submit enqueues an event. If the queue is saturated the event is dropped
// and a single warning is logged — we will never block the proxy hot path.
func (w *batchedWriter) Submit(e Event) {
	select {
	case w.queue <- e:
	default:
		w.dropOnce.Do(func() {
			slog.Warn("store: writer queue full, dropping event",
				slog.String("kind", fmt.Sprintf("%d", e.Kind)))
		})
	}
}

func (w *batchedWriter) run() {
	defer close(w.doneCh)
	batch := make([]Event, 0, BatchMaxEvents)
	timer := time.NewTimer(BatchMaxWait)
	defer timer.Stop()
	// flushNow drains and persists the current batch.
	flushNow := func() {
		if len(batch) == 0 {
			return
		}
		if err := w.flush(batch); err != nil {
			slog.Error("store: batch flush failed", slog.Any("err", err))
		}
		batch = batch[:0]
	}
	resetTimer := func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(BatchMaxWait)
	}
	resetTimer()
	for {
		select {
		case <-w.stopCh:
			// Drain remaining queue then exit.
			for {
				select {
				case e := <-w.queue:
					batch = append(batch, e)
					if len(batch) >= BatchMaxEvents {
						flushNow()
					}
				default:
					flushNow()
					return
				}
			}
		case e := <-w.queue:
			batch = append(batch, e)
			if len(batch) >= BatchMaxEvents {
				flushNow()
				resetTimer()
			}
		case <-timer.C:
			flushNow()
			resetTimer()
		}
	}
}

func (w *batchedWriter) flush(batch []Event) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := w.s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for i := range batch {
		if err := writeOne(ctx, tx, &batch[i]); err != nil {
			return fmt.Errorf("event %d: %w", i, err)
		}
	}
	return tx.Commit()
}

func writeOne(ctx context.Context, tx *sql.Tx, e *Event) error {
	switch e.Kind {
	case EventSessionStart:
		s := e.Session
		_, err := tx.ExecContext(ctx, `
			INSERT INTO sessions (id, agent_id, started_at, client_pid, client_user, mode)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO NOTHING`,
			s.ID, s.AgentID, s.StartedAt, s.ClientPID, s.ClientUser, s.Mode)
		return err
	case EventSessionEnd:
		s := e.Session
		_, err := tx.ExecContext(ctx, `
			UPDATE sessions SET
				ended_at = ?,
				total_calls = ?,
				total_blocked = ?,
				total_cost_usd = ?,
				total_tokens = ?
			WHERE id = ?`,
			s.EndedAt, s.TotalCalls, s.TotalBlocked, s.TotalCostUSD, s.TotalTokens, s.ID)
		return err
	case EventMCPServerUpsert:
		m := e.Server
		auth := 0
		if m.AuthRequired {
			auth = 1
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO mcp_servers (id, name, canonical_uri, transport,
				upstream_command, upstream_url, auth_required,
				first_seen_at, last_seen_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(canonical_uri) DO UPDATE SET
				last_seen_at = excluded.last_seen_at`,
			m.ID, m.Name, m.CanonicalURI, m.Transport,
			m.UpstreamCommand, m.UpstreamURL, auth,
			m.FirstSeenAt, m.LastSeenAt)
		return err
	case EventToolCall:
		t := e.ToolCall
		_, err := tx.ExecContext(ctx, `
			INSERT INTO tool_calls (id, session_id, server_id, tool_name, direction,
				started_at, completed_at, verdict, verdict_reason, risk_score,
				request_size_bytes, response_size_bytes,
				latency_ms_proxy, latency_ms_upstream,
				cost_usd, token_count,
				request_inline, response_inline, error_inline)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.ID, t.SessionID, t.ServerID, t.ToolName, t.Direction,
			t.StartedAt, t.CompletedAt, t.Verdict, t.VerdictReason, t.RiskScore,
			t.RequestSizeBytes, t.ResponseSizeBytes,
			t.LatencyMSProxy, t.LatencyMSUpstream,
			t.CostUSD, t.TokenCount,
			t.RequestInline, t.ResponseInline, t.ErrorInline)
		return err
	case EventToolCallStage:
		st := e.Stage
		_, err := tx.ExecContext(ctx, `
			INSERT INTO tool_call_stages (id, tool_call_id, stage, stage_order,
				started_at_ns, duration_ns, outcome, detail)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			st.ID, st.ToolCallID, st.Stage, st.StageOrder,
			st.StartedAtNS, st.DurationNS, st.Outcome, st.Detail)
		return err
	case EventAuditLog:
		a := e.Audit
		_, err := tx.ExecContext(ctx, `
			INSERT INTO audit_log (id, occurred_at, actor, action, target_type, target_id, detail)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			a.ID, a.OccurredAt, a.Actor, a.Action, a.TargetType, a.TargetID, a.Detail)
		return err
	default:
		return errors.New("unknown event kind")
	}
}
