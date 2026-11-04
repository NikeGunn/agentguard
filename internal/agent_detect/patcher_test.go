package agentdetect

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatchJSONRewritesStdioEntries(t *testing.T) {
	home := t.TempDir()
	cfgPath := writeFile(t, home, ".claude.json", `{
  "model": "sonnet",
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {"GITHUB_TOKEN": "xxx"}
    },
    "filesystem": {
      "command": "node",
      "args": ["fs.js"]
    }
  }
}`)
	d, err := ClaudeCodeDetector{}.Detect(home)
	require.NoError(t, err)
	res, err := Apply(d, PatchOptions{AgentguardBinary: "/opt/agentguard"})
	require.NoError(t, err)
	require.Equal(t, 2, res.RewrittenCount)
	require.Equal(t, 0, res.UnchangedCount)

	// Re-detect and inspect the rewritten file.
	d2, err := ClaudeCodeDetector{}.Detect(home)
	require.NoError(t, err)
	for _, s := range d2.Servers {
		require.Equal(t, "/opt/agentguard", s.Command, "%s not rewritten", s.Name)
		require.Equal(t, "wrap", s.Args[0])
		require.Equal(t, "--upstream-name", s.Args[1])
		require.Equal(t, s.Name, s.Args[2])
		require.Equal(t, "--", s.Args[3])
	}
	// Non-MCP top-level keys preserved.
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	require.Contains(t, raw, "model")
}

func TestPatchIsIdempotent(t *testing.T) {
	home := t.TempDir()
	writeFile(t, home, ".claude.json", `{
  "mcpServers": {"x": {"command": "node", "args": ["x.js"]}}
}`)
	d, _ := ClaudeCodeDetector{}.Detect(home)
	r1, err := Apply(d, PatchOptions{AgentguardBinary: "/bin/agentguard"})
	require.NoError(t, err)
	require.Equal(t, 1, r1.RewrittenCount)

	d2, _ := ClaudeCodeDetector{}.Detect(home)
	r2, err := Apply(d2, PatchOptions{AgentguardBinary: "/bin/agentguard"})
	require.NoError(t, err)
	require.Equal(t, 0, r2.RewrittenCount, "second Apply should be no-op")
	require.Equal(t, 1, r2.UnchangedCount)
}

func TestBackupIsByteIdenticalAndRestoreRoundtrips(t *testing.T) {
	home := t.TempDir()
	original := `{
  "model": "sonnet",
  "mcpServers": {
    "github": {"command": "npx", "args": ["-y", "srv"]}
  }
}
`
	cfgPath := writeFile(t, home, ".claude.json", original)
	originalSum := sha256.Sum256([]byte(original))

	d, _ := ClaudeCodeDetector{}.Detect(home)
	_, err := Apply(d, PatchOptions{AgentguardBinary: "/bin/agentguard"})
	require.NoError(t, err)

	// Backup byte-identical to the original.
	bk, err := os.ReadFile(cfgPath + BackupSuffix)
	require.NoError(t, err)
	require.Equal(t, original, string(bk))

	// Restore puts the original back, and the post-restore hash matches.
	require.NoError(t, Restore(cfgPath, hex.EncodeToString(originalSum[:])))
	restored, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.Equal(t, original, string(restored))
	// Backup file removed after Restore.
	_, err = os.Stat(cfgPath + BackupSuffix)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestPatchTOMLRewritesCodex(t *testing.T) {
	home := t.TempDir()
	writeFile(t, home, ".codex/config.toml", `
model = "gpt-5"

[mcp_servers.github]
command = "npx"
args = ["-y", "srv"]

[mcp_servers.fs]
command = "node"
args = ["fs.js"]
`)
	d, err := CodexDetector{}.Detect(home)
	require.NoError(t, err)
	res, err := Apply(d, PatchOptions{AgentguardBinary: "agentguard"})
	require.NoError(t, err)
	require.Equal(t, 2, res.RewrittenCount)

	// Re-detect post-patch.
	d2, err := CodexDetector{}.Detect(home)
	require.NoError(t, err)
	for _, s := range d2.Servers {
		require.Equal(t, "agentguard", s.Command)
		require.Equal(t, "wrap", s.Args[0])
	}
}

func TestPatchSkipsHTTPEntries(t *testing.T) {
	home := t.TempDir()
	writeFile(t, home, ".claude.json", `{
  "mcpServers": {
    "remote": {"url": "https://mcp.example.com/sse"},
    "local":  {"command": "node", "args": ["x.js"]}
  }
}`)
	d, _ := ClaudeCodeDetector{}.Detect(home)
	res, err := Apply(d, PatchOptions{AgentguardBinary: "/bin/agentguard"})
	require.NoError(t, err)
	require.Equal(t, 1, res.RewrittenCount)
	require.Equal(t, 1, res.UnchangedCount)
}

func TestPatchJSONKeepsFlatTransportFields(t *testing.T) {
	home := t.TempDir()
	writeFile(t, home, ".claude.json", `{
  "mcpServers": {
    "string": {
      "transport": "stdio",
      "command": "node",
      "args": ["string.js"]
    },
    "object": {
      "transport": {"type": "stdio"},
      "command": "node",
      "args": ["object.js"]
    }
  }
}`)
	d, err := ClaudeCodeDetector{}.Detect(home)
	require.NoError(t, err)
	require.Len(t, d.Servers, 2)
	res, err := Apply(d, PatchOptions{AgentguardBinary: "/opt/agentguard"})
	require.NoError(t, err)
	require.Equal(t, 2, res.RewrittenCount)

	d2, err := ClaudeCodeDetector{}.Detect(home)
	require.NoError(t, err)
	for _, server := range d2.Servers {
		require.Equal(t, "/opt/agentguard", server.Command)
	}

	data, err := os.ReadFile(d.ConfigPath)
	require.NoError(t, err)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	var servers map[string]map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["mcpServers"], &servers))
	require.JSONEq(t, `"stdio"`, string(servers["string"]["transport"]))
	require.JSONEq(t, `{"type":"stdio"}`, string(servers["object"]["transport"]))
}

func TestPatchJSONRewritesClineNestedTransport(t *testing.T) {
	home := t.TempDir()
	writeFile(t, home, ".cline/data/settings/cline_mcp_settings.json", `{
  "mcpServers": {
    "filesystem": {
      "transport": {
        "type": "stdio",
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem"],
        "env": {"HOME": "/tmp"}
      },
      "disabled": false,
      "metadata": {"source": "user"}
    },
    "remote": {
      "transport": {"type": "streamableHttp", "url": "https://mcp.example.com"}
    }
  }
}`)
	d, err := ClineDetector{}.Detect(home)
	require.NoError(t, err)
	res, err := Apply(d, PatchOptions{AgentguardBinary: "/opt/agentguard"})
	require.NoError(t, err)
	require.Equal(t, 1, res.RewrittenCount)
	require.Equal(t, 1, res.UnchangedCount)

	d2, err := ClineDetector{}.Detect(home)
	require.NoError(t, err)
	require.Len(t, d2.Servers, 2)
	for _, server := range d2.Servers {
		if server.Name == "filesystem" {
			require.Equal(t, "/opt/agentguard", server.Command)
			require.Equal(t, []string{
				"wrap", "--upstream-name", "filesystem", "--", "npx", "-y",
				"@modelcontextprotocol/server-filesystem",
			}, server.Args)
		}
	}
	res2, err := Apply(d2, PatchOptions{AgentguardBinary: "/opt/agentguard"})
	require.NoError(t, err)
	require.Equal(t, 0, res2.RewrittenCount)
	require.Equal(t, 2, res2.UnchangedCount)

	data, err := os.ReadFile(d.ConfigPath)
	require.NoError(t, err)
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	var servers map[string]map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["mcpServers"], &servers))
	require.Contains(t, servers["filesystem"], "disabled")
	require.Contains(t, servers["filesystem"], "metadata")
	require.JSONEq(t, `false`, string(servers["filesystem"]["disabled"]))
	require.JSONEq(t, `{"source":"user"}`, string(servers["filesystem"]["metadata"]))
	var transport map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(servers["filesystem"]["transport"], &transport))
	require.JSONEq(t, `"stdio"`, string(transport["type"]))
	require.JSONEq(t, `{"HOME":"/tmp"}`, string(transport["env"]))
}

func TestPatchSkipServers(t *testing.T) {
	home := t.TempDir()
	writeFile(t, home, ".claude.json", `{
  "mcpServers": {"a": {"command":"node","args":["a.js"]}, "b": {"command":"node","args":["b.js"]}}
}`)
	d, _ := ClaudeCodeDetector{}.Detect(home)
	res, err := Apply(d, PatchOptions{AgentguardBinary: "/bin/agentguard", SkipServers: SkipSet([]string{"a"})})
	require.NoError(t, err)
	require.Equal(t, 1, res.RewrittenCount)
	require.Equal(t, 1, res.UnchangedCount)

	// Filename helper kept happy for tests that don't need it.
	_ = filepath.Base
}
