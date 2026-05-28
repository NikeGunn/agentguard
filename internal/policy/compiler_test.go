package policy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompilePackHappyPath(t *testing.T) {
	src := []byte(`
name: test
version: 1
rules:
  - id: r1
    match:
      tool: echo
      direction: outbound
    action: block
  - id: r2
    match:
      content_regex: 'AKIA[0-9A-Z]{16}'
    action: redact
`)
	pf, rules, err := CompilePack(src)
	require.NoError(t, err)
	require.Equal(t, "test", pf.Name)
	require.Len(t, rules, 2)
	require.Equal(t, ActionBlock, rules[0].Action)
	require.Equal(t, Outbound, rules[0].Direction)
	require.NotNil(t, rules[1].ContentMatch)
}

func TestCompilePackRejectsUnknownAction(t *testing.T) {
	_, _, err := CompilePack([]byte(`
name: bad
version: 1
rules:
  - action: doomscroll
`))
	require.Error(t, err)
}

func TestCompilePackRejectsBadRegex(t *testing.T) {
	_, _, err := CompilePack([]byte(`
name: bad
version: 1
rules:
  - id: r
    match:
      content_regex: '['
    action: block
`))
	require.Error(t, err)
}

func TestLoadBuiltinDefault(t *testing.T) {
	pf, rules, err := LoadBuiltin("default")
	require.NoError(t, err)
	require.Equal(t, "default", pf.Name)
	require.NotEmpty(t, rules)
}

func TestListBuiltin(t *testing.T) {
	names, err := ListBuiltin()
	require.NoError(t, err)
	require.Contains(t, names, "default")
	require.Contains(t, names, "strict")
}
