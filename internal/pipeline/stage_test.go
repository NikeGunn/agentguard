package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeStage struct {
	name string
	out  StageResult
}

func (f fakeStage) Name() string                            { return f.name }
func (f fakeStage) Run(context.Context, *Message) StageResult { return f.out }

func TestEmptyChainPasses(t *testing.T) {
	c := NewChain()
	tr, v := c.Run(context.Background(), &Message{})
	require.Empty(t, tr)
	require.Equal(t, VerdictPass, v)
}

func TestChainShortCircuitsOnBlock(t *testing.T) {
	c := NewChain(
		fakeStage{name: "a", out: StageResult{Verdict: VerdictPass}},
		fakeStage{name: "b", out: StageResult{Verdict: VerdictBlock, Reason: "nope"}},
		fakeStage{name: "c", out: StageResult{Verdict: VerdictPass}},
	)
	tr, v := c.Run(context.Background(), &Message{})
	require.Equal(t, VerdictBlock, v)
	require.Len(t, tr, 2)
	require.Equal(t, "nope", tr[1].Result.Reason)
}

func TestChainFlagIsNonFatal(t *testing.T) {
	c := NewChain(
		fakeStage{name: "a", out: StageResult{Verdict: VerdictFlag}},
		fakeStage{name: "b", out: StageResult{Verdict: VerdictPass}},
	)
	tr, v := c.Run(context.Background(), &Message{})
	require.Equal(t, VerdictFlag, v)
	require.Len(t, tr, 2)
}

func TestMLStageSkipsForNow(t *testing.T) {
	r := MLStage{}.Run(context.Background(), &Message{})
	require.Equal(t, VerdictSkip, r.Verdict)
}
