package pipeline

import (
	"context"

	"github.com/agentguard/agentguard/internal/policy"
)

// PolicyStage is stage 3: evaluates the user's rule pack against the message.
// Block/allow/redact/require_approval/rate_limit decisions are terminal;
// flag decisions are informational and let the chain continue.
//
// PolicyStage does NOT perform the redaction itself — it returns Verdict
// Transform/Block/Flag verdicts and lets Stage 4 (content scanner) do the
// actual byte rewriting. This keeps responsibility crisp: the policy engine
// decides WHAT should happen, the scanner decides HOW.
type PolicyStage struct {
	Engine *policy.Engine
}

// Name implements Stage.
func (*PolicyStage) Name() string { return "policy" }

// Run implements Stage.
func (p *PolicyStage) Run(_ context.Context, m *Message) StageResult {
	if p == nil || p.Engine == nil {
		return StageResult{Verdict: VerdictSkip, Reason: "no_engine"}
	}
	d := p.Engine.Evaluate(policy.Input{
		Server:    m.ServerName,
		Tool:      m.ToolName,
		Method:    m.Method,
		Direction: policy.Direction(m.Direction),
		Raw:       m.Raw,
	})
	if !d.Matched {
		return StageResult{Verdict: VerdictPass}
	}
	switch d.Action {
	case policy.ActionBlock, policy.ActionRequireApproval:
		return StageResult{
			Verdict: VerdictBlock,
			Reason:  string(d.Action) + ":" + d.RuleID,
			Detail:  detailJSON(map[string]any{"rule": d.RuleID, "match": d.Reason}),
		}
	case policy.ActionRedact:
		// Hand off to Stage 4 by flagging with the rule that fired; the
		// scanner reads m.Raw and produces the redacted Transform. Flag
		// keeps the chain running so the scanner stage executes.
		return StageResult{
			Verdict: VerdictFlag,
			Reason:  "redact:" + d.RuleID,
			Detail:  detailJSON(map[string]any{"rule": d.RuleID, "match": d.Reason}),
		}
	case policy.ActionFlag:
		return StageResult{
			Verdict: VerdictFlag,
			Reason:  "flag:" + d.RuleID,
			Detail:  detailJSON(map[string]any{"rule": d.RuleID, "match": d.Reason}),
		}
	case policy.ActionAllow:
		return StageResult{Verdict: VerdictPass, Reason: "allow:" + d.RuleID}
	case policy.ActionRateLimit:
		// Honoured by stage 0's bucket per session+tool — Milestone 3 widens
		// the transport guard to read these params. For now record but pass.
		return StageResult{Verdict: VerdictFlag, Reason: "rate_limit:" + d.RuleID}
	}
	return StageResult{Verdict: VerdictPass}
}
