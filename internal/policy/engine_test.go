package policy

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEngineBlocksOnExactToolMatch(t *testing.T) {
	e := NewEngine([]Rule{
		{ID: "r1", Tool: "delete_repo", Action: ActionBlock},
	})
	d := e.Evaluate(Input{Tool: "delete_repo", Direction: Outbound})
	require.True(t, d.Matched)
	require.Equal(t, ActionBlock, d.Action)
	require.Equal(t, "r1", d.RuleID)
}

func TestEngineSkipsOnNoMatch(t *testing.T) {
	e := NewEngine([]Rule{
		{ID: "r1", Tool: "delete_repo", Action: ActionBlock},
	})
	d := e.Evaluate(Input{Tool: "echo"})
	require.False(t, d.Matched)
}

func TestEngineFirstTerminalWins(t *testing.T) {
	e := NewEngine([]Rule{
		{ID: "first", Action: ActionFlag},
		{ID: "second", Action: ActionBlock},
		{ID: "third", Action: ActionAllow},
	})
	d := e.Evaluate(Input{})
	require.Equal(t, ActionBlock, d.Action)
	require.Equal(t, "second", d.RuleID)
}

func TestEngineFlagsAccumulateUntilTerminal(t *testing.T) {
	e := NewEngine([]Rule{
		{ID: "f1", Action: ActionFlag},
		{ID: "f2", Action: ActionFlag},
	})
	d := e.Evaluate(Input{})
	require.True(t, d.Matched)
	require.Equal(t, ActionFlag, d.Action)
	require.Equal(t, "f2", d.RuleID, "last flag wins when no terminal fires")
}

func TestEngineContentRegex(t *testing.T) {
	re := regexp.MustCompile(`rm\s+-rf`)
	e := NewEngine([]Rule{
		{ID: "rmrf", ContentMatch: re, Action: ActionBlock},
	})
	require.Equal(t, ActionBlock, e.Evaluate(Input{Raw: []byte("rm -rf /")}).Action)
	require.False(t, e.Evaluate(Input{Raw: []byte("ls /")}).Matched)
}
