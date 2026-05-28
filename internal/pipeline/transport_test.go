package pipeline

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransportGuardAllowsUnderRate(t *testing.T) {
	g := &TransportGuard{RatePerSecond: 100, MaxBytes: 1 << 20}
	for i := 0; i < 50; i++ {
		r := g.Run(context.Background(), &Message{SessionID: "s1", Raw: []byte("{}\n")})
		require.Equal(t, VerdictPass, r.Verdict, "iter %d", i)
	}
}

func TestTransportGuardBlocksWhenBurstExhausted(t *testing.T) {
	g := &TransportGuard{RatePerSecond: 10, Burst: 3}
	var lastReason string
	hit := 0
	for i := 0; i < 10; i++ {
		r := g.Run(context.Background(), &Message{SessionID: "s1", Raw: []byte("{}")})
		if r.Verdict == VerdictBlock {
			hit++
			lastReason = r.Reason
		}
	}
	require.Positive(t, hit, "expected at least one block once burst was drained")
	require.Equal(t, "rate_limited", lastReason)
}

func TestTransportGuardFrameTooLarge(t *testing.T) {
	g := &TransportGuard{RatePerSecond: 100, MaxBytes: 10}
	r := g.Run(context.Background(), &Message{SessionID: "s1", Raw: []byte(strings.Repeat("x", 20))})
	require.Equal(t, VerdictBlock, r.Verdict)
	require.Equal(t, "frame_too_large", r.Reason)
}
