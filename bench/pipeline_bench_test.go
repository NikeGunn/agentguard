// pipeline_bench_test.go — measures the milestone-2 cheap-path chain on a
// realistic clean frame. This is the number the CI 10%-regression gate
// (requirements §5.1) watches as later milestones add stages.
package bench

import (
	"context"
	"testing"

	"github.com/agentguard/agentguard/internal/pipeline"
	"github.com/agentguard/agentguard/internal/policy"
)

func buildM2Chain(b *testing.B) *pipeline.Chain {
	b.Helper()
	_, rules, err := policy.LoadBuiltin("default")
	if err != nil {
		b.Fatal(err)
	}
	return pipeline.NewChain(
		&pipeline.TransportGuard{RatePerSecond: 1e9, MaxBytes: 1 << 20},
		pipeline.SchemaValidator{},
		&pipeline.PolicyStage{Engine: policy.NewEngine(rules)},
		pipeline.NewContentScanner(),
	)
}

func BenchmarkM2ChainCleanFrame(b *testing.B) {
	chain := buildM2Chain(b)
	msg := &pipeline.Message{
		SessionID:  "bench",
		ServerName: "mock",
		ToolName:   "echo",
		Method:     "tools/call",
		Direction:  pipeline.Outbound,
		Raw:        []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"message":"hello world"}}}` + "\n"),
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset Raw each iteration: a Transform on a previous iter could
		// otherwise mutate msg.Raw and skew the measurement.
		msg.Raw = []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"message":"hello world"}}}` + "\n")
		_, _ = chain.Run(ctx, msg)
	}
}

func BenchmarkM2ChainSecretFrame(b *testing.B) {
	chain := buildM2Chain(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := &pipeline.Message{
			SessionID: "bench",
			Direction: pipeline.Inbound,
			Raw:       []byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"AKIAIOSFODNN7EXAMPLE"}]}}` + "\n"),
		}
		_, _ = chain.Run(ctx, msg)
	}
}
