// Package pipeline defines the inspection-stage interface and the chain
// runner. Milestone 1 shipped the empty chain. Milestone 2 adds the cheap
// path: transport guard, schema validator, policy engine, content scanner.
package pipeline

import (
	"context"
	"time"
)

// Direction identifies which side of the proxy the message is travelling.
type Direction string

const (
	Outbound Direction = "outbound" // agent → tool
	Inbound  Direction = "inbound"  // tool → agent
)

// Verdict is the per-stage outcome. The chain aggregates them into a final
// verdict for the call.
type Verdict string

const (
	VerdictPass      Verdict = "pass"
	VerdictBlock     Verdict = "block"
	VerdictFlag      Verdict = "flag"
	VerdictTransform Verdict = "transform"
	VerdictSkip      Verdict = "skip"
	VerdictError     Verdict = "error"
)

// Message is the unit a stage inspects. The proxy parses the JSON-RPC frame
// once and fills these fields before invoking the chain so stages don't each
// re-parse. Raw always holds the exact bytes received from the wire (newline
// included); transforms replace Raw via StageResult.Transform.
type Message struct {
	SessionID  string
	ServerName string
	ToolName   string // resolved from tools/call params, "" otherwise
	Method     string // JSON-RPC method
	Direction  Direction
	Raw        []byte
}

// StageResult is what a stage returns. Detail is JSON-encoded for storage.
// Transform, when non-nil and Verdict == VerdictTransform, replaces Raw on
// the message before the next stage runs and is what the proxy forwards.
type StageResult struct {
	Verdict   Verdict
	Reason    string
	Detail    string
	Transform []byte
}

// Stage is the interface every inspection stage implements.
type Stage interface {
	Name() string
	Run(ctx context.Context, m *Message) StageResult
}

// Chain is an ordered set of Stages.
type Chain struct {
	stages []Stage
}

// NewChain builds a Chain.
func NewChain(stages ...Stage) *Chain { return &Chain{stages: stages} }

// Stages returns the ordered stage list, primarily for introspection.
func (c *Chain) Stages() []Stage { return c.stages }

// Trace is the per-stage record produced by Run, suitable for persisting to
// tool_call_stages.
type Trace struct {
	Stage     string
	Order     int
	StartedAt time.Time
	Duration  time.Duration
	Result    StageResult
}

// Run executes the chain in order. It short-circuits on the first Block or
// Error verdict (Flag is informational, Transform mutates the message but
// continues). The final verdict is the most severe outcome seen.
func (c *Chain) Run(ctx context.Context, m *Message) ([]Trace, Verdict) {
	traces := make([]Trace, 0, len(c.stages))
	final := VerdictPass
	for i, s := range c.stages {
		started := time.Now()
		res := s.Run(ctx, m)
		traces = append(traces, Trace{
			Stage: s.Name(), Order: i,
			StartedAt: started, Duration: time.Since(started),
			Result: res,
		})
		switch res.Verdict {
		case VerdictBlock, VerdictError:
			return traces, res.Verdict
		case VerdictTransform:
			if len(res.Transform) > 0 {
				m.Raw = res.Transform
			}
			if final == VerdictPass || final == VerdictFlag {
				final = VerdictTransform
			}
		case VerdictFlag:
			if final == VerdictPass {
				final = VerdictFlag
			}
		}
	}
	return traces, final
}
