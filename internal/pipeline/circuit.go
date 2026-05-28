package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitConfig tunes the breaker.
type CircuitConfig struct {
	// Window is the sliding-time window we track per session.
	Window time.Duration
	// MaxBlocks: trip if more than this many blocks observed in Window.
	MaxBlocks int
	// MaxErrors: trip if more than this many errors observed in Window.
	MaxErrors int
	// Cooldown is how long the breaker stays open before half-open.
	Cooldown time.Duration
}

// DefaultCircuit returns a conservative default config.
func DefaultCircuit() CircuitConfig {
	return CircuitConfig{
		Window:    30 * time.Second,
		MaxBlocks: 5,
		MaxErrors: 3,
		Cooldown:  60 * time.Second,
	}
}

// CircuitState is the per-session breaker state.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // normal — traffic flows
	CircuitOpen                         // tripped — block everything
	CircuitHalfOpen                     // probing — let one through
)

// CircuitBreakerStage blocks traffic on sessions that have racked up too
// many block/error verdicts in a sliding window. The stage records
// outcomes via Record (called by the chain runner downstream of the
// other stages) and itself decides at the top of the chain whether to
// short-circuit the message.
type CircuitBreakerStage struct {
	cfg CircuitConfig

	mu       sync.Mutex
	sessions map[string]*sessionCircuit
	now      func() time.Time
}

type sessionCircuit struct {
	state      CircuitState
	openedAt   time.Time
	blocks     []time.Time
	errors     []time.Time
}

// NewCircuitBreakerStage returns a configured breaker.
func NewCircuitBreakerStage(cfg CircuitConfig) *CircuitBreakerStage {
	if cfg.Window <= 0 {
		cfg = DefaultCircuit()
	}
	return &CircuitBreakerStage{
		cfg:      cfg,
		sessions: make(map[string]*sessionCircuit),
		now:      time.Now,
	}
}

func (s *CircuitBreakerStage) Name() string { return "circuit_breaker" }

// Run inspects the breaker state for m.SessionID and short-circuits the
// chain when open. The stage itself never trips on the message it's
// currently inspecting; tripping is recorded after the chain decides
// via Record.
func (s *CircuitBreakerStage) Run(_ context.Context, m *Message) StageResult {
	if m.SessionID == "" {
		return StageResult{Verdict: VerdictPass}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sc := s.sessions[m.SessionID]
	if sc == nil {
		return StageResult{Verdict: VerdictPass}
	}
	switch sc.state {
	case CircuitOpen:
		if s.now().Sub(sc.openedAt) >= s.cfg.Cooldown {
			sc.state = CircuitHalfOpen
			return StageResult{Verdict: VerdictPass, Reason: "circuit half-open (probe)"}
		}
		return StageResult{
			Verdict: VerdictBlock,
			Reason:  "circuit open",
			Detail:  fmt.Sprintf(`{"opened_at":"%s","cooldown_seconds":%d}`, sc.openedAt.UTC().Format(time.RFC3339), int(s.cfg.Cooldown.Seconds())),
		}
	case CircuitHalfOpen:
		// Allow exactly one probe; the next message comes back to this
		// branch and we treat its result via Record.
		return StageResult{Verdict: VerdictPass}
	}
	return StageResult{Verdict: VerdictPass}
}

// Record updates the breaker after the chain finished with the given
// final verdict. Call this from the proxy once per inspected message.
func (s *CircuitBreakerStage) Record(sessionID string, v Verdict) {
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sc := s.sessions[sessionID]
	if sc == nil {
		sc = &sessionCircuit{}
		s.sessions[sessionID] = sc
	}
	now := s.now()
	cutoff := now.Add(-s.cfg.Window)

	prune := func(ts []time.Time) []time.Time {
		i := 0
		for i < len(ts) && ts[i].Before(cutoff) {
			i++
		}
		return ts[i:]
	}
	sc.blocks = prune(sc.blocks)
	sc.errors = prune(sc.errors)

	switch v {
	case VerdictBlock:
		sc.blocks = append(sc.blocks, now)
	case VerdictError:
		sc.errors = append(sc.errors, now)
	case VerdictPass, VerdictTransform, VerdictFlag:
		if sc.state == CircuitHalfOpen {
			sc.state = CircuitClosed
			sc.blocks = nil
			sc.errors = nil
			return
		}
	}

	if sc.state == CircuitOpen {
		return
	}
	if len(sc.blocks) > s.cfg.MaxBlocks || len(sc.errors) > s.cfg.MaxErrors {
		sc.state = CircuitOpen
		sc.openedAt = now
	}
}

// State returns the breaker state for a session (diagnostics / tests).
func (s *CircuitBreakerStage) State(sessionID string) CircuitState {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc := s.sessions[sessionID]
	if sc == nil {
		return CircuitClosed
	}
	return sc.state
}
