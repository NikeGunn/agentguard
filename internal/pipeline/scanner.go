package pipeline

import (
	"context"
	"regexp"
)

// ContentScanner is stage 4: a cheap regex bank that redacts well-known
// secret formats and flags content that matches obvious prompt-injection
// signatures. It is the last line of defense before the (expensive) ML
// classifier in stage 5 — anything detected here means we never have to pay
// for the model on this frame.
//
// The scanner does its own pattern matching (independent of the policy
// engine's content_regex rules) because requirements §4.5 calls out a
// built-in bank that ships regardless of which rule pack is installed.
// Combined effect: even a policy-free deployment still redacts AWS keys.
type ContentScanner struct {
	patterns []scanPattern
}

type scanPattern struct {
	name string
	kind string // "secret" or "injection"
	re   *regexp.Regexp
}

// NewContentScanner returns the built-in scanner with the default pattern
// bank.
func NewContentScanner() *ContentScanner {
	return &ContentScanner{patterns: defaultPatterns()}
}

// Name implements Stage.
func (*ContentScanner) Name() string { return "content_scan" }

// Run implements Stage. On any secret hit it returns Transform with the
// match replaced by [REDACTED:<name>]. On any injection-signature hit it
// returns Flag with detail naming the pattern. Both can fire on the same
// frame — secret wins (Transform), injection becomes part of Detail.
func (c *ContentScanner) Run(_ context.Context, m *Message) StageResult {
	transformed := m.Raw
	hitSecret := false
	hits := make([]string, 0, 2)
	for _, p := range c.patterns {
		if !p.re.Match(transformed) {
			continue
		}
		hits = append(hits, p.name)
		if p.kind == "secret" {
			transformed = p.re.ReplaceAll(transformed, []byte("[REDACTED:"+p.name+"]"))
			hitSecret = true
		}
	}
	if len(hits) == 0 {
		return StageResult{Verdict: VerdictPass}
	}
	if hitSecret {
		return StageResult{
			Verdict:   VerdictTransform,
			Reason:    "secret_redacted",
			Detail:    detailJSON(map[string]any{"hits": hits}),
			Transform: transformed,
		}
	}
	return StageResult{
		Verdict: VerdictFlag,
		Reason:  "injection_signature",
		Detail:  detailJSON(map[string]any{"hits": hits}),
	}
}

// defaultPatterns is the cheap-path regex bank. Patterns are intentionally
// conservative — false positives in tool results would break agent flows.
// The ML stage (M5) catches the trickier cases.
func defaultPatterns() []scanPattern {
	return []scanPattern{
		// ── secrets ─────────────────────────────────────────────────────
		{"aws_access_key", "secret", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
		{"github_token", "secret", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{30,}`)},
		{"stripe_live_key", "secret", regexp.MustCompile(`(?:sk|rk)_live_[A-Za-z0-9]{20,}`)},
		{"openai_key", "secret", regexp.MustCompile(`sk-(?:proj-)?[A-Za-z0-9_-]{20,}`)},
		{"anthropic_key", "secret", regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`)},
		{"google_api_key", "secret", regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`)},
		{"slack_token", "secret", regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`)},
		{"private_key_block", "secret", regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----`)},
		{"jwt", "secret", regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)},

		// ── injection signatures (cheap heuristics) ─────────────────────
		{"ignore_previous_instructions", "injection",
			regexp.MustCompile(`(?i)ignore\s+(?:all\s+)?previous\s+instructions`)},
		{"you_are_now", "injection",
			regexp.MustCompile(`(?i)you\s+are\s+now\s+(?:a|an|the)`)},
		{"new_system_prompt", "injection",
			regexp.MustCompile(`(?i)(?:new|updated)\s+system\s+(?:prompt|instructions)`)},
	}
}
