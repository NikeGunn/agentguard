package pipeline

import "context"

// MLStage is the stub for the ONNX-backed prompt-injection classifier.
//
// TODO milestone 5: load the bundled DeBERTa-v3-small ONNX model, cache
// classifications by content hash, and return Block when confidence >= 0.85.
type MLStage struct{}

func (MLStage) Name() string { return "ml_classify" }

func (MLStage) Run(_ context.Context, _ *Message) StageResult {
	return StageResult{Verdict: VerdictSkip, Reason: "ml stage not implemented (milestone 5)"}
}
