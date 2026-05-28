package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPackListShowsBuiltins(t *testing.T) {
	cmd := newPackCmd()
	cmd.SetArgs([]string{"list"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	require.NoError(t, cmd.Execute())
	out := buf.String()
	require.Contains(t, out, "Built-in packs")
	require.Contains(t, out, "default")
	require.Contains(t, out, "strict")
}

func TestPackVerifyBuiltin(t *testing.T) {
	cmd := newPackCmd()
	cmd.SetArgs([]string{"verify", "default"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	require.NoError(t, cmd.Execute())
	require.True(t, strings.HasPrefix(buf.String(), "OK"))
}
