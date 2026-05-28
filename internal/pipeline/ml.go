package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agentguard/agentguard/internal/ml"
)

// MLStage runs the heuristic prompt-injection classifier over the
// user-facing payload of a JSON-RPC frame. Blocks at very high
// confidence, flags at medium confidence, passes otherwise.
type MLStage struct {
	c *ml.Classifier
}

// NewMLStage returns a stage backed by a shared classifier cache.
func NewMLStage(c *ml.Classifier) *MLStage {
	if c == nil {
		c = ml.New(4096)
	}
	return &MLStage{c: c}
}

func (s *MLStage) Name() string { return "ml_classify" }

func (s *MLStage) Run(_ context.Context, m *Message) StageResult {
	text := extractText(m.Raw)
	if text == "" {
		return StageResult{Verdict: VerdictSkip, Reason: "no text payload"}
	}
	r := s.c.Classify(text)
	detail := fmt.Sprintf(`{"confidence":%.3f,"label":"%s","features":%d}`,
		r.Confidence, r.Label, len(r.Features))
	switch {
	case r.Confidence >= ml.BlockThreshold:
		return StageResult{Verdict: VerdictBlock, Reason: "ml: " + r.Label, Detail: detail}
	case r.Confidence >= ml.FlagThreshold:
		return StageResult{Verdict: VerdictFlag, Reason: "ml: " + r.Label, Detail: detail}
	default:
		return StageResult{Verdict: VerdictPass}
	}
}

// extractText pulls user-supplied text out of a JSON-RPC frame so the
// classifier sees content not envelope.
func extractText(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var env struct {
		Params struct {
			Arguments map[string]any `json:"arguments"`
			Text      string         `json:"text"`
			Input     string         `json:"input"`
			Prompt    string         `json:"prompt"`
		} `json:"params"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return string(raw)
	}
	var parts []byte
	push := func(s string) {
		if s != "" {
			if len(parts) > 0 {
				parts = append(parts, '\n')
			}
			parts = append(parts, s...)
		}
	}
	push(env.Params.Text)
	push(env.Params.Input)
	push(env.Params.Prompt)
	for _, v := range env.Params.Arguments {
		if s, ok := v.(string); ok {
			push(s)
		}
	}
	if len(parts) == 0 {
		return string(raw)
	}
	return string(parts)
}
