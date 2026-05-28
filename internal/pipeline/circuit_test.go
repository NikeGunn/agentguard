package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCircuitClosedByDefault(t *testing.T) {
	c := NewCircuitBreakerStage(DefaultCircuit())
	r := c.Run(context.Background(), &Message{SessionID: "s1"})
	require.Equal(t, VerdictPass, r.Verdict)
	require.Equal(t, CircuitClosed, c.State("s1"))
}

func TestCircuitTripsOnRepeatedBlocks(t *testing.T) {
	c := NewCircuitBreakerStage(CircuitConfig{
		Window: time.Minute, MaxBlocks: 2, MaxErrors: 99, Cooldown: time.Minute,
	})
	for i := 0; i < 3; i++ {
		c.Record("s", VerdictBlock)
	}
	require.Equal(t, CircuitOpen, c.State("s"))

	r := c.Run(context.Background(), &Message{SessionID: "s"})
	require.Equal(t, VerdictBlock, r.Verdict)
	require.Contains(t, r.Reason, "circuit open")
}

func TestCircuitHalfOpenAfterCooldown(t *testing.T) {
	c := NewCircuitBreakerStage(CircuitConfig{
		Window: time.Minute, MaxBlocks: 1, MaxErrors: 99, Cooldown: time.Second,
	})
	now := time.Now()
	c.now = func() time.Time { return now }
	c.Record("s", VerdictBlock)
	c.Record("s", VerdictBlock)
	require.Equal(t, CircuitOpen, c.State("s"))

	// advance past cooldown
	c.now = func() time.Time { return now.Add(2 * time.Second) }
	r := c.Run(context.Background(), &Message{SessionID: "s"})
	require.Equal(t, VerdictPass, r.Verdict)
	require.Equal(t, CircuitHalfOpen, c.State("s"))

	// a clean message recloses
	c.Record("s", VerdictPass)
	require.Equal(t, CircuitClosed, c.State("s"))
}

func TestCircuitSkipsWithoutSessionID(t *testing.T) {
	c := NewCircuitBreakerStage(DefaultCircuit())
	r := c.Run(context.Background(), &Message{})
	require.Equal(t, VerdictPass, r.Verdict)
}
