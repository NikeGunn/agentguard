package agentdetect

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// CodexDetector finds OpenAI's Codex CLI which uses TOML for its config.
// MCP servers live under an [mcp_servers.<name>] table.
type CodexDetector struct{}

func (CodexDetector) Kind() Kind          { return KindCodex }
func (CodexDetector) DisplayName() string { return "Codex CLI" }

type codexTOMLFile struct {
	MCPServers map[string]codexTOMLEntry `toml:"mcp_servers"`
}

type codexTOMLEntry struct {
	Command string            `toml:"command"`
	Args    []string          `toml:"args"`
	URL     string            `toml:"url"`
	Env     map[string]string `toml:"env"`
}

func (CodexDetector) Detect(home string) (*Detection, error) {
	path := firstExisting([]string{
		joinHome(home, ".codex", "config.toml"),
	})
	if path == "" {
		return nil, ErrNotInstalled
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var parsed codexTOMLFile
	if _, err := toml.Decode(string(data), &parsed); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	names := make([]string, 0, len(parsed.MCPServers))
	for n := range parsed.MCPServers {
		names = append(names, n)
	}
	sortStrings(names)
	entries := make([]MCPServerEntry, 0, len(parsed.MCPServers))
	for _, n := range names {
		e := parsed.MCPServers[n]
		entries = append(entries, MCPServerEntry{
			Name: n, Command: e.Command, Args: e.Args,
			URL: e.URL, Env: e.Env,
		})
	}
	return &Detection{
		Kind:        KindCodex,
		DisplayName: "Codex CLI",
		ConfigPath:  path,
		Format:      FormatTOML,
		Servers:     entries,
	}, nil
}
