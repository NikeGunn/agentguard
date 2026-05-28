package pipeline

import (
	"context"
	"sync"
	"time"
)

// TransportGuard is stage 0: per-session token-bucket rate limit and a hard
// upper bound on inspected frame size. Locally trusted stdio sessions get no
// auth check (the OS user boundary is the trust root, per requirements §4.1);
// HTTP transports add auth verification in a later milestone.
type TransportGuard struct {
	// RatePerSecond is the steady-state allowance per session. Defaults to
	// 100 when zero.
	RatePerSecond float64
	// Burst is the bucket capacity. Defaults to RatePerSecond when zero.
	Burst float64
	// MaxBytes caps the size of an inspected frame. 0 disables the cap.
	// 8 MiB is a sane default — single MCP frames over this should be rare.
	MaxBytes int

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

// Name implements Stage.
func (*TransportGuard) Name() string { return "transport" }

// Run implements Stage.
func (g *TransportGuard) Run(_ context.Context, m *Message) StageResult {
	if g.MaxBytes > 0 && len(m.Raw) > g.MaxBytes {
		return StageResult{
			Verdict: VerdictBlock,
			Reason:  "frame_too_large",
			Detail:  detailJSON(map[string]any{"size": len(m.Raw), "max": g.MaxBytes}),
		}
	}
	if !g.take(m.SessionID) {
		return StageResult{
			Verdict: VerdictBlock,
			Reason:  "rate_limited",
			Detail:  detailJSON(map[string]any{"session_id": m.SessionID, "rate_per_second": g.rate()}),
		}
	}
	return StageResult{Verdict: VerdictPass}
}

func (g *TransportGuard) rate() float64 {
	if g.RatePerSecond > 0 {
		return g.RatePerSecond
	}
	return 100
}

func (g *TransportGuard) burst() float64 {
	if g.Burst > 0 {
		return g.Burst
	}
	return g.rate()
}

func (g *TransportGuard) take(sessionID string) bool {
	rate := g.rate()
	burst := g.burst()
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.buckets == nil {
		g.buckets = make(map[string]*bucket)
	}
	b, ok := g.buckets[sessionID]
	now := time.Now()
	if !ok {
		b = &bucket{tokens: burst, last: now}
		g.buckets[sessionID] = b
	} else {
		elapsed := now.Sub(b.last).Seconds()
		b.tokens += elapsed * rate
		if b.tokens > burst {
			b.tokens = burst
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
