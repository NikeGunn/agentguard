package agentdetect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeFile writes data to a file inside dir, creating intermediate
// directories. Tests use this to fake a real home tree.
func writeFile(t *testing.T, dir, rel, data string) string {
	t.Helper()
	full := filepath.Join(dir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o700))
	require.NoError(t, os.WriteFile(full, []byte(data), 0o600))
	return full
}

func TestClaudeCodeDetectsClaudeJson(t *testing.T) {
	home := t.TempDir()
	writeFile(t, home, ".claude.json", `{
  "mcpServers": {
    "github": { "command": "npx", "args": ["-y", "@modelcontextprotocol/server-github"] }
  }
}`)
	d, err := ClaudeCodeDetector{}.Detect(home)
	require.NoError(t, err)
	require.Equal(t, KindClaudeCode, d.Kind)
	require.Equal(t, FormatJSON, d.Format)
	require.Len(t, d.Servers, 1)
	require.Equal(t, "github", d.Servers[0].Name)
	require.Equal(t, "npx", d.Servers[0].Command)
}

func TestClaudeCodeNotInstalled(t *testing.T) {
	_, err := ClaudeCodeDetector{}.Detect(t.TempDir())
	require.ErrorIs(t, err, ErrNotInstalled)
}

func TestCursorDetects(t *testing.T) {
	home := t.TempDir()
	writeFile(t, home, ".cursor/mcp.json", `{"mcpServers":{"fs":{"command":"npx","args":["-y","mcp-fs"]}}}`)
	d, err := CursorDetector{}.Detect(home)
	require.NoError(t, err)
	require.Len(t, d.Servers, 1)
	require.Equal(t, "fs", d.Servers[0].Name)
}

func TestCodexDetectsTOML(t *testing.T) {
	home := t.TempDir()
	writeFile(t, home, ".codex/config.toml", `
[mcp_servers.github]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-github"]

[mcp_servers.filesystem]
command = "node"
args = ["fs.js"]
`)
	d, err := CodexDetector{}.Detect(home)
	require.NoError(t, err)
	require.Equal(t, FormatTOML, d.Format)
	require.Len(t, d.Servers, 2)
	names := []string{d.Servers[0].Name, d.Servers[1].Name}
	require.Contains(t, names, "github")
	require.Contains(t, names, "filesystem")
}

func TestDetectAllFindsTwo(t *testing.T) {
	home := t.TempDir()
	writeFile(t, home, ".claude.json", `{"mcpServers":{}}`)
	writeFile(t, home, ".cursor/mcp.json", `{"mcpServers":{}}`)
	got, errs := DetectAll(home)
	require.Empty(t, errs)
	require.Len(t, got, 2)
}
