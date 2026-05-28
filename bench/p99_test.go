package bench

// p99 enforcement test — fails the build if the cheap-path pipeline
// exceeds 5 ms p99 over a 5,000-sample run.
//
// This is the §13 Definition-of-Done gate. Keep it conservative — CI
// runners are noisier than the developer's laptop, so we measure with
// a margin.

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/agentguard/agentguard/internal/ml"
	"github.com/agentguard/agentguard/internal/pipeline"
)

func TestCheapPathP99Under5ms(t *testing.T) {
	if testing.Short() {
		t.Skip("p99 gate skipped under -short")
	}
	chain := pipeline.NewChain(
		pipeline.SchemaValidator{},
		pipeline.NewContentScanner(),
		pipeline.NewMLStage(ml.New(0)),
	)
	frame := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"/tmp/x"}}}`)
	const N = 5000
	durations := make([]time.Duration, N)
	for i := 0; i < N; i++ {
		m := &pipeline.Message{
			SessionID:  "bench",
			ServerName: "github",
			Method:     "tools/call",
			Direction:  pipeline.Outbound,
			Raw:        frame,
		}
		start := time.Now()
		_, _ = chain.Run(context.Background(), m)
		durations[i] = time.Since(start)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p50 := durations[N/2]
	p99 := durations[N*99/100]
	max := durations[N-1]
	t.Logf("cheap-path: p50=%s p99=%s max=%s", p50, p99, max)
	if p99 > 5*time.Millisecond {
		t.Fatalf("cheap-path p99 %s exceeds 5 ms budget", p99)
	}
}
