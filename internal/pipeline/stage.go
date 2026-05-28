// Package pipeline defines the inspection-stage interface and the chain
// runner. Real stages land in milestones 2-5; this milestone only fixes the
// shape so callers can compile against it.
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

// Verdict is the per-stage outcome. Stages can return any of these; the
// chain runner aggregates them into a final verdict for the call.
type Verdict string

const (
	VerdictPass      Verdict = "pass"
	VerdictBlock     Verdict = "block"
	VerdictFlag      Verdict = "flag"
	VerdictTransform Verdict = "transform"
	VerdictSkip      Verdict = "skip"
	VerdictError     Verdict = "error"
)

// Message is the unit a stage inspects. The proxy fills these from raw
// JSON-RPC frames before invoking the chain.
type Message struct {
	ServerName string
	ToolName   string
	Direction  Direction
	Raw        []byte // the JSON-RPC bytes as seen on the wire
}

// StageResult is what a stage returns. Detail is JSON-encoded for storage.
type StageResult struct {
	Verdict   Verdict
	Reason    string
	Detail    string
	Transform []byte // populated only when Verdict == VerdictTransform
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
	Stage      string
	Order      int
	StartedAt  time.Time
	Duration   time.Duration
	Result     StageResult
}

// Run executes the chain in order. It stops on the first non-pass verdict
// other than Flag (which is informational and lets execution continue).
// Milestone 1 ships with an empty chain so this always returns ({}, pass).
func (c *Chain) Run(ctx context.Context, m *Message) ([]Trace, Verdict) {
	traces := make([]Trace, 0, len(c.stages))
	final := VerdictPass
	for i, s := range c.stages {
		started := time.Now()
		res := s.Run(ctx, m)
		traces = append(traces, Trace{
			Stage:     s.Name(),
			Order:     i,
			StartedAt: started,
			Duration:  time.Since(started),
			Result:    res,
		})
		switch res.Verdict {
		case VerdictBlock, VerdictError:
			return traces, res.Verdict
		case VerdictTransform:
			final = VerdictTransform
		case VerdictFlag:
			if final == VerdictPass {
				final = VerdictFlag
			}
		}
	}
	return traces, final
}
