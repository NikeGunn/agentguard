// Package policy implements AgentGuard's rule-pack-driven policy engine.
//
// A rule pack is a YAML document with an ordered list of rules. Each rule
// has a `match` predicate (server / tool / direction / regex over content)
// and an `action` (allow / block / flag / redact / require_approval /
// rate_limit). The engine compiles a pack into an internal AST once at load
// time so per-message evaluation is allocation-free for the common path.
//
// This file owns the runtime: the Engine type, Decision, and Evaluate.
// Pack parsing and AST construction live in compiler.go.
package policy

import (
	"regexp"
	"strings"
	"sync"
)

// Action is what a rule does when its match predicate fires.
type Action string

const (
	ActionAllow           Action = "allow"
	ActionBlock           Action = "block"
	ActionFlag            Action = "flag"
	ActionRedact          Action = "redact"
	ActionRequireApproval Action = "require_approval"
	ActionRateLimit       Action = "rate_limit"
)

// Direction mirrors pipeline.Direction without importing it (avoids a cycle).
type Direction string

const (
	Outbound Direction = "outbound"
	Inbound  Direction = "inbound"
	AnyDir   Direction = ""
)

// Input is what the engine evaluates against. It's filled by the policy
// pipeline stage from a pipeline.Message.
type Input struct {
	Server    string
	Tool      string
	Method    string
	Direction Direction
	Raw       []byte
}

// Decision is the engine's verdict for one Input.
type Decision struct {
	Action  Action
	RuleID  string
	Reason  string
	Params  map[string]any
	Matched bool
}

// Rule is one compiled policy rule.
type Rule struct {
	ID           string
	Server       string         // exact match, "" means any
	Tool         string         // exact match, "" means any
	Direction    Direction
	ContentMatch *regexp.Regexp // optional
	Action       Action
	Params       map[string]any
}

// Engine evaluates rules in deterministic order. Higher-priority (lower
// integer) policies and lower rule_order values fire first; the first match
// wins unless the action is Flag, which is non-terminal and lets evaluation
// continue.
type Engine struct {
	mu    sync.RWMutex
	rules []Rule
}

// NewEngine returns an engine seeded with the given (already-compiled) rules.
func NewEngine(rules []Rule) *Engine { return &Engine{rules: rules} }

// Replace swaps the rule set atomically.
func (e *Engine) Replace(rules []Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = rules
}

// Rules returns a snapshot of the active rules, primarily for introspection
// and tests.
func (e *Engine) Rules() []Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Rule, len(e.rules))
	copy(out, e.rules)
	return out
}

// Evaluate runs the rule list against the input. The returned Decision is
// the first terminal match (block / allow / redact / require_approval /
// rate_limit). Flags accumulate but never short-circuit; if only flags fire,
// the Decision is the last flag seen (callers can re-evaluate flags later).
//
// If no rule matches, Action is empty and Matched is false.
func (e *Engine) Evaluate(in Input) Decision {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var lastFlag Decision
	for _, r := range e.rules {
		if !ruleMatches(&r, &in) {
			continue
		}
		d := Decision{
			Action: r.Action, RuleID: r.ID,
			Reason: ruleReason(&r, &in),
			Params: r.Params, Matched: true,
		}
		if r.Action == ActionFlag {
			lastFlag = d
			continue
		}
		return d
	}
	return lastFlag
}

func ruleMatches(r *Rule, in *Input) bool {
	if r.Server != "" && !equalFold(r.Server, in.Server) {
		return false
	}
	if r.Tool != "" && !equalFold(r.Tool, in.Tool) {
		return false
	}
	if r.Direction != AnyDir && r.Direction != in.Direction {
		return false
	}
	if r.ContentMatch != nil && !r.ContentMatch.Match(in.Raw) {
		return false
	}
	return true
}

func ruleReason(r *Rule, in *Input) string {
	parts := make([]string, 0, 4)
	if r.Server != "" {
		parts = append(parts, "server="+r.Server)
	}
	if r.Tool != "" {
		parts = append(parts, "tool="+r.Tool)
	}
	if r.Direction != AnyDir {
		parts = append(parts, "dir="+string(r.Direction))
	}
	if r.ContentMatch != nil {
		parts = append(parts, "content_match")
	}
	if len(parts) == 0 {
		parts = append(parts, "rule="+r.ID)
	}
	_ = in
	return strings.Join(parts, ",")
}

func equalFold(a, b string) bool { return strings.EqualFold(a, b) }
